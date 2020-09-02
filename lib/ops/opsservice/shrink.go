/*
Copyright 2018 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package opsservice

import (
	"context"
	"fmt"
	"os"

	"github.com/gravitational/gravity/lib/clients"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/users"
	"github.com/gravitational/gravity/lib/utils"

	teleservices "github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
	"github.com/pborman/uuid"
	log "github.com/sirupsen/logrus"
)

// createShrinkOperation initiates shrink operation and starts it immediately
func (s *site) createShrinkOperation(context context.Context, req ops.CreateSiteShrinkOperationRequest) (*ops.SiteOperationKey, error) {
	log.Infof("createShrinkOperation: req=%#v", req)

	cluster, err := s.service.GetSite(s.key)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	server, err := s.validateShrinkRequest(req, *cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	op := &ops.SiteOperation{
		ID:          uuid.New(),
		AccountID:   s.key.AccountID,
		SiteDomain:  s.key.SiteDomain,
		Type:        ops.OperationShrink,
		Created:     s.clock().UtcNow(),
		CreatedBy:   storage.UserFromContext(context),
		Updated:     s.clock().UtcNow(),
		State:       ops.OperationStateShrinkInProgress,
		Provisioner: server.Provisioner,
	}

	ctx, err := s.newOperationContext(*op)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer ctx.Close()

	err = s.updateRequestVars(ctx, &req.Variables, op)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = s.service.setCloudProviderFromRequest(s.key, op.Provisioner, &req.Variables)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	op.Shrink = &storage.ShrinkOperationState{
		Servers:     []storage.Server{*server},
		Force:       req.Force,
		Vars:        req.Variables,
		NodeRemoved: req.NodeRemoved,
	}
	op.Shrink.Vars.System.ClusterName = s.key.SiteDomain

	// make sure the provided keys are valid
	if isAWSProvisioner(op.Provisioner) {
		// when shrinking via command line (using leave/remove), AWS credentials are not
		// provided so skip their validation - terraform will retrieve the keys from AWS
		// metadata API automatically
		aws := s.cloudProvider().(*aws)
		if aws.accessKey != "" || aws.secretKey != "" {
			err = s.verifyPermissionsAWS(ctx)
			if err != nil {
				return nil, trace.BadParameter("invalid AWS credentials")
			}
		}
	}

	key, err := s.getOperationGroup().createSiteOperationWithOptions(*op,
		createOperationOptions{force: req.Force})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	s.reportProgress(ctx, ops.ProgressEntry{
		State:      ops.ProgressStateInProgress,
		Completion: 0,
		Message:    "initializing the operation",
	})

	err = s.executeOperation(*key, s.shrinkOperationStart)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return key, nil
}

func (s *site) validateShrinkRequest(req ops.CreateSiteShrinkOperationRequest, cluster ops.Site) (*storage.Server, error) {
	serverName := req.Servers[0]
	if len(cluster.ClusterState.Servers) == 1 {
		return nil, trace.BadParameter(
			"cannot shrink 1-node cluster, use --force flag to uninstall")
	}

	server, err := cluster.ClusterState.FindServer(serverName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// check to make sure the server exists and can be found
	servers, err := s.getAllTeleportServers()
	if err != nil {
		return nil, trace.Wrap(err, "failed to query teleport servers")
	}

	masters := servers.getWithLabels(labels{schema.ServiceLabelRole: string(schema.ServiceRoleMaster)})
	if len(masters) == 0 {
		return nil, trace.NotFound("no master servers found")
	}
	if len(masters) == 1 && masters[0].GetLabels()[ops.Hostname] == server.Hostname {
		return nil, trace.BadParameter("cannot remove the last master server")
	}

	teleserver := servers.getWithLabels(labels{ops.Hostname: server.Hostname})
	if len(teleserver) == 0 {
		if !req.Force {
			return nil, trace.BadParameter(
				"node %q is offline, add --force flag to force removal", serverName)
		}
		log.Warnf("Node %q is offline, forcing removal.", serverName)
	}

	return server, nil
}

// shrinkOperationStart kicks off actuall uninstall process:
// deprovisions servers, deletes packages
func (s *site) shrinkOperationStart(ctx *operationContext) (err error) {
	state := ctx.operation.Shrink
	ctx.serversToRemove = state.Servers
	force := state.Force

	cluster, err := s.service.GetSite(s.key)
	if err != nil {
		return trace.Wrap(err)
	}

	server, err := cluster.ClusterState.FindServer(state.Servers[0].Hostname)
	if err != nil {
		return trace.Wrap(err)
	}

	// if the node is the gravity site leader (i.e. the process that is executing this code)
	// is running on is being removed, give up the leadership so another process will pick up
	// and resume the operation
	if server.AdvertiseIP == os.Getenv(constants.EnvPodIP) {
		ctx.RecordInfo("this node is being removed, stepping down")
		s.leader().StepDown()
		return nil
	}

	// if the operation was resumed, cloud provider might not be set
	if s.service.getCloudProvider(s.key) == nil {
		err = s.service.setCloudProviderFromRequest(
			s.key, ctx.operation.Provisioner, &ctx.operation.Shrink.Vars)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	serverName := server.Hostname
	logger := ctx.WithField("server", serverName)

	if force {
		ctx.RecordInfo("forcing %q removal", serverName)
	} else {
		ctx.RecordInfo("starting %q removal", serverName)
	}

	// shrink uses a couple of runners for the following purposes:
	//  * teleport master runner is used to execute system commands that remove
	//    the node from k8s, database, etc.
	//  * agent runner runs on the removed node and is used to perform system
	//    uninstall on it (if the node is online)
	var masterRunner, agentRunner *serverRunner

	masterRunner, err = s.pickShrinkMasterRunner(ctx, *server)
	if err != nil {
		return trace.Wrap(err)
	}
	logger.Infof("Selected %v (%v) as master runner.",
		masterRunner.server.HostName(),
		masterRunner.server.Address())

	// determine whether the node being removed is online and, if so, launch
	// a shrink agent on it
	online := false
	if !state.NodeRemoved {
		_, err := s.getTeleportServerNoRetry(ops.Hostname, serverName)
		if err != nil {
			logger.WithError(err).Warn("Node is offline.")
		} else {
			agentRunner, err = s.launchAgent(ctx, *server)
			if err != nil {
				if !force {
					return trace.Wrap(err)
				}
				logger.WithError(err).Warn("Failed to launch agent.")
			} else {
				online = true
			}
		}
	}

	opKey := ctx.key()

	// schedule some clean up actions to run regardless of the outcome of the operation
	defer func() {
		// erase cloud provider info for this site which may contain sensitive information
		// such as API keys
		s.service.deleteCloudProvider(s.key)
		// stop running shrink agent
		err := s.agentService().StopAgents(context.TODO(), opKey)
		if err != nil {
			logger.WithError(err).Warn("Failed to stop shrink agent.")
		}
	}()

	if online {
		ctx.RecordInfo("node %q is online", serverName)
	} else {
		ctx.RecordInfo("node %q is offline", serverName)
	}

	s.reportProgress(ctx, ops.ProgressEntry{
		State:      ops.ProgressStateInProgress,
		Completion: 10,
		Message:    "unregistering the node",
	})

	if err = s.unlabelNode(*server, masterRunner); err != nil {
		if !force {
			return trace.Wrap(err, "failed to unregister the node")
		}
		logger.WithError(err).Warn("Failed to unregister the node, force continue.")
	}

	if s.app.Manifest.HasHook(schema.HookNodeRemoving) {
		s.reportProgress(ctx, ops.ProgressEntry{
			State:      ops.ProgressStateInProgress,
			Completion: 20,
			Message:    "running pre-removal hooks",
		})

		if err = s.runHook(ctx, schema.HookNodeRemoving); err != nil {
			if !force {
				return trace.Wrap(err, "failed to run %v hook", schema.HookNodeRemoving)
			}
			logger.WithError(err).WithField("hook", schema.HookNodeRemoving).Warn("Failed to run hook, force continue.")
		}
	}

	s.reportProgress(ctx, ops.ProgressEntry{
		State:      ops.ProgressStateInProgress,
		Completion: 30,
		Message:    "removing the node from the cluster",
	})

	// if the node is online, it needs to leave the serf cluster to
	// prevent joining back
	if online {
		err = s.serfNodeLeave(agentRunner)
		if err != nil {
			if !force {
				return trace.Wrap(err, "failed to remove the node from the serf cluster")
			}
			logger.WithError(err).Warn("Failed to remove node from serf cluster.")
		}
	}

	// delete the Kubernetes node and force-leave its serf member
	if err = s.removeNodeFromCluster(*server, masterRunner); err != nil {
		if !force {
			return trace.Wrap(err, "failed to remove the node from the cluster")
		}
		logger.WithError(err).Warn("Failed to remove node from the cluster, force continue.")
	}

	s.reportProgress(ctx, ops.ProgressEntry{
		State:      ops.ProgressStateInProgress,
		Completion: 40,
		Message:    "removing the node from the database",
	})

	// remove etcd member
	err = s.removeFromEtcd(context.TODO(), ctx, *server, cluster.ClusterState.Servers.Masters())
	// the node may be an etcd proxy and not a full member of the etcd cluster
	if err != nil && !trace.IsNotFound(err) {
		if !force {
			return trace.Wrap(err, "failed to remove the node from the database")
		}
		logger.WithError(err).Warn("Failed to remove the node from the database, force continue.")
	}

	if online {
		s.reportProgress(ctx, ops.ProgressEntry{
			State:      ops.ProgressStateInProgress,
			Completion: 50,
			Message:    "uninstalling the system software",
		})

		if err = s.uninstallSystem(ctx, agentRunner); err != nil {
			logger.WithError(err).Warn("Failed to uninstall the system software.")
		}
	}

	if isAWSProvisioner(ctx.operation.Provisioner) {
		if !s.app.Manifest.HasHook(schema.HookNodesDeprovision) {
			return trace.BadParameter("%v hook is not defined",
				schema.HookNodesDeprovision)
		}
		logger.Info("Using nodes deprovisioning hook.")
		err := s.runNodesDeprovisionHook(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
		ctx.RecordInfo("nodes have been successfully deprovisioned")
	}

	if s.app.Manifest.HasHook(schema.HookNodeRemoved) {
		s.reportProgress(ctx, ops.ProgressEntry{
			State:      ops.ProgressStateInProgress,
			Completion: 80,
			Message:    "running post-removal hooks",
		})

		if err = s.runHook(ctx, schema.HookNodeRemoved); err != nil {
			if !force {
				return trace.Wrap(err, "failed to run %v hook", schema.HookNodeRemoved)
			}
			logger.WithError(err).WithField("hook", schema.HookNodeRemoved).Warn("Failed to run hook, force continue.")
		}
	}

	s.reportProgress(ctx, ops.ProgressEntry{
		State:      ops.ProgressStateInProgress,
		Completion: 85,
		Message:    "cleaning up packages",
	})

	provisionedServer := &ProvisionedServer{Server: *server}
	if err = s.deletePackages(provisionedServer); err != nil {
		if !force {
			return trace.Wrap(err, "failed to clean up packages")
		}
		logger.WithError(err).Warn("Failed to clean up packages, force continue.")
	}

	s.reportProgress(ctx, ops.ProgressEntry{
		State:      ops.ProgressStateInProgress,
		Completion: 90,
		Message:    "waiting for operation to complete",
	})

	if err = s.waitForServerToDisappear(serverName); err != nil {
		logger.WithError(err).Warn("Failed to wait for server to disappear.")
	}

	if err = s.removeClusterStateServers([]string{server.Hostname}); err != nil {
		return trace.Wrap(err)
	}

	_, err = s.compareAndSwapOperationState(context.TODO(), swap{
		key:            opKey,
		expectedStates: []string{ops.OperationStateShrinkInProgress},
		newOpState:     ops.OperationStateCompleted,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	s.reportProgress(ctx, ops.ProgressEntry{
		State:      ops.ProgressStateCompleted,
		Completion: constants.Completed,
		Message:    fmt.Sprintf("%v removed", serverName),
	})

	return nil
}

func (s *site) pickShrinkMasterRunner(ctx *operationContext, removedServer storage.Server) (*serverRunner, error) {
	masters, err := s.getTeleportServers(schema.ServiceLabelRole, string(schema.ServiceRoleMaster))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Pick any master server except the one that's being removed.
	for _, master := range masters {
		if master.IP != removedServer.AdvertiseIP {
			return &serverRunner{
				&master, &teleportRunner{ctx, s.domainName, s.teleport()},
			}, nil
		}
	}
	return nil, trace.NotFound("%v is being removed and no more master nodes are available to execute the operation",
		removedServer)
}

func (s *site) waitForServerToDisappear(hostname string) error {
	requireServerIsGone := func(domain string, servers []teleservices.Server) error {
		for _, server := range servers {
			labels := server.GetLabels()
			if labels[ops.Hostname] == hostname {
				return trace.AlreadyExists("server %v is not yet removed", hostname)
			}
		}
		return nil
	}

	log.Debug("waiting for server to disappear")
	// wait until the node is removed from the backend
	_, err := s.getTeleportServersWithTimeout(
		nil,
		defaults.TeleportServerQueryTimeout,
		defaults.RetryInterval,
		defaults.RetryLessAttempts,
		requireServerIsGone,
	)
	return trace.Wrap(err)
}

func (s *site) removeFromEtcd(ctx context.Context, opCtx *operationContext, server storage.Server, masters []storage.Server) error {
	peerURL := server.EtcdPeerURL()
	logger := opCtx.WithField("peer", peerURL)
	logger.Info("Remove peer from etcd cluster.")
	b := utils.NewExponentialBackOff(defaults.EtcdRemoveMemberTimeout)
	return utils.RetryTransient(ctx, b, func() error {
		client, err := clients.DefaultEtcdMembers()
		if err != nil {
			return trace.Wrap(err)
		}
		members, err := client.List(ctx)
		logger.WithField("peers", members).Info("Etcd members.")
		if err != nil {
			return trace.Wrap(err)
		}
		member := utils.EtcdHasMember(members, peerURL)
		if member == nil {
			logger.Info("Peer not found.")
			return nil
		}
		err = client.Remove(ctx, member.ID)
		logger.WithError(err).Info("Removed etcd peer.")
		return trace.Wrap(err)
	})
}

func (s *site) uninstallSystem(ctx *operationContext, runner *serverRunner) error {
	commands := [][]string{
		s.gravityCommand("system", "uninstall", "--confirm", "--no-uninstall-service"),
	}

	for _, command := range commands {
		out, err := runner.Run(command...)
		if err != nil {
			ctx.WithError(err).WithFields(log.Fields{
				"command": command,
				"output":  string(out),
			}).Warn("Failed to run.")
		}
	}

	return nil
}

func (s *site) launchAgent(ctx *operationContext, server storage.Server) (*serverRunner, error) {
	teleportServer, err := s.getTeleportServer(ops.Hostname, server.Hostname)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	teleportRunner := &serverRunner{
		server: teleportServer,
		runner: &teleportRunner{ctx, s.domainName, s.teleport()},
	}

	tokenID, err := s.createShrinkAgentToken(ctx.operation.ID)
	if err != nil {
		return nil, trace.Wrap(err, "failed to create shrink agent token")
	}

	serverAddr := s.service.cfg.Agents.ServerAddr()
	command := []string{
		"ops", "agent", s.packages().PortalURL(),
		"--advertise-addr", server.AdvertiseIP,
		"--server-addr", serverAddr,
		"--token", tokenID,
		"--vars", fmt.Sprintf("%v:%v", ops.AgentMode, ops.AgentModeShrink),
		"--service-uid", s.uid(),
		"--service-gid", s.gid(),
		"--service-name", defaults.GravityRPCAgentServiceName,
		"--cloud-provider", s.provider,
	}
	out, err := teleportRunner.Run(s.gravityCommand(command...)...)
	if err != nil {
		return nil, trace.Wrap(err, "failed to start shrink agent: %s", out)
	}

	agentReport, err := s.waitForAgents(context.TODO(), ctx)
	if err != nil {
		return nil, trace.Wrap(err, "failed to wait for shrink agent")
	}

	info := agentReport.Servers[0]
	return &serverRunner{
		server: agentServer{
			AdvertiseIP: info.AdvertiseAddr,
			Hostname:    info.GetHostname(),
		},
		runner: &agentRunner{ctx, s.agentService()},
	}, nil
}

func (s *site) createShrinkAgentToken(operationID string) (tokenID string, err error) {
	token, err := users.CryptoRandomToken(defaults.ProvisioningTokenBytes)
	if err != nil {
		return "", trace.Wrap(err)
	}
	_, err = s.users().CreateProvisioningToken(storage.ProvisioningToken{
		Token:       token,
		AccountID:   s.key.AccountID,
		SiteDomain:  s.key.SiteDomain,
		Type:        storage.ProvisioningTokenTypeInstall,
		Expires:     s.clock().UtcNow().Add(defaults.InstallTokenTTL),
		OperationID: operationID,
		UserEmail:   s.agentUserEmail(),
	})
	if err != nil {
		return "", trace.Wrap(err)
	}
	return token, nil
}

// deletePackages removes stale packages generated for the specified server
// from the cluster package service after the server had been removed.
func (s *site) deletePackages(server *ProvisionedServer) error {
	serverPackages, err := s.serverPackages(server)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, pkg := range serverPackages {
		err = s.packages().DeletePackage(pkg)
		if err != nil && !trace.IsNotFound(err) {
			return trace.Wrap(err, "failed to delete package").AddField("package", pkg)
		}
	}
	return nil
}

func (s *site) serverPackages(server *ProvisionedServer) ([]loc.Locator, error) {
	var packages []loc.Locator
	err := pack.ForeachPackage(s.packages(), func(env pack.PackageEnvelope) error {
		if env.HasLabel(pack.AdvertiseIPLabel, server.AdvertiseIP) {
			packages = append(packages, env.Locator)
			return nil
		}
		if s.isTeleportMasterConfigPackageFor(server, env.Locator) ||
			s.isTeleportNodeConfigPackageFor(server, env.Locator) ||
			s.isPlanetConfigPackageFor(server, env.Locator) ||
			s.isPlanetSecretsPackageFor(server, env.Locator) {
			packages = append(packages, env.Locator)
		}
		return nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return packages, nil
}

// unlabelNode deletes server profile labels from k8s node
func (s *site) unlabelNode(server storage.Server, runner *serverRunner) error {
	profile, err := s.app.Manifest.NodeProfiles.ByName(server.Role)
	if err != nil {
		return trace.Wrap(err)
	}

	var labelFlags []string
	for label := range profile.Labels {
		labelFlags = append(labelFlags, fmt.Sprintf("%s-", label))
	}

	command := s.planetEnterCommand(defaults.KubectlBin, "label", "nodes",
		fmt.Sprintf("-l=%v=%v", "kubernetes.io/hostname", server.KubeNodeID()))
	command = append(command, labelFlags...)

	err = utils.Retry(defaults.RetryInterval, defaults.RetryAttempts, func() error {
		_, err := runner.Run(command...)
		return trace.Wrap(err)
	})

	return trace.Wrap(err)
}

func (s *site) removeNodeFromCluster(server storage.Server, runner *serverRunner) (err error) {
	provisionedServer := ProvisionedServer{Server: server}
	commands := [][]string{
		s.planetEnterCommand(
			defaults.KubectlBin, "delete", "nodes", "--ignore-not-found=true",
			fmt.Sprintf("-l=%v=%v", "kubernetes.io/hostname", server.KubeNodeID())),
		// Issue `serf force-leave -prune` from the master node to immediately
		// evict the member from the serf cluster.
		s.planetEnterCommand(defaults.SerfBin, "force-leave", "-prune", provisionedServer.AgentName(s.domainName)),
	}

	err = utils.Retry(defaults.RetryInterval, defaults.RetryAttempts, func() error {
		for _, command := range commands {
			out, err := runner.Run(command...)
			if err != nil {
				return trace.Wrap(err, "command %q failed: %s", command, out)
			}
		}
		return nil
	})

	return trace.Wrap(err)
}

// serfNodeLeave removes the node specified with runner from the serf cluster
// by issuing a `serf leave` from the node itself.
func (s *site) serfNodeLeave(runner *serverRunner) error {
	// Issue `serf leave` from the node to remove the node from the serf cluster
	command := s.planetEnterCommand(defaults.SerfBin, "leave")
	err := utils.Retry(defaults.RetryInterval, defaults.RetryLessAttempts, func() error {
		out, err := runner.Run(command...)
		if err != nil {
			return trace.Wrap(err, "command %q failed: %s", command, out)
		}
		return nil
	})
	return trace.Wrap(err)
}

func (s *site) isTeleportMasterConfigPackageFor(server *ProvisionedServer, loc loc.Locator) bool {
	configPackage := s.teleportMasterConfigPackage(server)
	return configPackage.Name == loc.Name && configPackage.Repository == loc.Repository
}

func (s *site) isTeleportNodeConfigPackageFor(server *ProvisionedServer, loc loc.Locator) bool {
	configPackage := s.teleportNodeConfigPackage(server)
	return configPackage.Name == loc.Name && configPackage.Repository == loc.Repository
}

func (s *site) isPlanetConfigPackageFor(server *ProvisionedServer, loc loc.Locator) bool {
	// Version omitted on purpose since only repository/name are used for comparison
	configPackage := s.planetConfigPackage(server, "")
	return configPackage.Name == loc.Name && configPackage.Repository == loc.Repository
}

func (s *site) isPlanetSecretsPackageFor(server *ProvisionedServer, loc loc.Locator) bool {
	// Version omitted on purpose since only repository/name are used for comparison
	configPackage := s.planetSecretsPackage(server, "")
	return configPackage.Name == loc.Name && configPackage.Repository == loc.Repository
}
