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
	"strings"
	"time"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/teleport"
	teleservices "github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

// createExpandOperation initiates expand operation
func (s *site) createExpandOperation(req ops.CreateSiteExpandOperationRequest) (*ops.SiteOperationKey, error) {
	log.Debugf("createExpandOperation(%#v)", req)

	profiles := make(map[string]storage.ServerProfile)
	for role, count := range req.Servers {
		profile, err := s.app.Manifest.NodeProfiles.ByName(role)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		profiles[role] = storage.ServerProfile{
			Description: profile.Description,
			Labels:      profile.Labels,
			ServiceRole: string(profile.ServiceRole),
			Request: storage.ServerProfileRequest{
				Count: count,
			},
		}
	}
	return s.createInstallExpandOperation(
		ops.OperationExpand, ops.OperationStateExpandInitiated, req.Provisioner,
		req.Variables, profiles)
}

func (s *site) getSiteOperation(operationID string) (*ops.SiteOperation, error) {
	op, err := s.backend().GetSiteOperation(s.key.SiteDomain, operationID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return (*ops.SiteOperation)(op), nil
}

// expandOperationStart kicks off actuall expansion process:
// resource provisioning, package configuration and deployment
func (s *site) expandOperationStart(ctx *operationContext) error {
	op, err := s.compareAndSwapOperationState(swap{
		key: ctx.key(),
		expectedStates: []string{
			ops.OperationStateExpandInitiated,
			ops.OperationStateExpandPrechecks,
		},
		newOpState: ops.OperationStateExpandProvisioning,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	if isAWSProvisioner(op.Provisioner) {
		if !s.app.Manifest.HasHook(schema.HookNodesProvision) {
			return trace.NotFound("%v hook is not defined",
				schema.HookNodesProvision)
		}
		ctx.Infof("Using nodes provisioning hook.")
		err := s.runNodesProvisionHook(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
		ctx.RecordInfo("Infrastructure has been successfully provisioned.")
	}

	s.reportProgress(ctx, ops.ProgressEntry{
		State:   ops.ProgressStateInProgress,
		Message: "Waiting for the provisioned node to come up",
	})

	_, err = s.waitForAgents(context.TODO(), ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	s.reportProgress(ctx, ops.ProgressEntry{
		State:   ops.ProgressStateInProgress,
		Message: "The node is up",
	})

	op, err = s.compareAndSwapOperationState(swap{
		key:            ctx.key(),
		expectedStates: []string{ops.OperationStateExpandProvisioning},
		newOpState:     ops.OperationStateReady,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	err = s.waitForOperation(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil

	// TODO REMOVE THIS ⮟⮟⮟⮟⮟⮟⮟⮟

	labels := map[string]string{schema.ServiceLabelRole: string(schema.ServiceRoleMaster)}
	masters, err := s.teleport().GetServers(context.TODO(), s.domainName, labels)
	if err != nil {
		return trace.Wrap(err)
	}
	if len(masters) == 0 {
		return trace.NotFound("no master server found")
	}

	ctx.provisionedServers, err = s.loadProvisionedServers(op.Servers, len(masters), ctx.Entry)
	if err != nil {
		return trace.Wrap(err)
	}

	master, err := newTeleportServer(masters[0])
	if err != nil {
		return trace.Wrap(err)
	}

	ctx.Infof("[EXPAND] with servers: %v", ctx.provisionedServers)

	if s.app.Manifest.HasHook(schema.HookNodeAdding) {
		s.reportProgress(ctx, ops.ProgressEntry{
			State:      ops.ProgressStateInProgress,
			Completion: 20,
			Message:    "running pre-expand hooks",
		})

		err = s.runHook(ctx, schema.HookNodeAdding)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	servers := make([]remoteServer, 0, len(ctx.provisionedServers))
	for _, server := range ctx.provisionedServers {
		servers = append(servers, server)
	}

	if err := s.addClusterStateServers(op.Servers); err != nil {
		return trace.Wrap(err)
	}

	err = s.executeOnServers(context.TODO(), servers,
		func(c context.Context, server remoteServer) error {
			return trace.Wrap(s.addServer(ctx, server.(*ProvisionedServer), master, len(masters)))
		})
	if err != nil {
		return trace.Wrap(err)
	}

	if s.app.Manifest.HasHook(schema.HookNodeAdded) {
		s.reportProgress(ctx, ops.ProgressEntry{
			State:      ops.ProgressStateInProgress,
			Completion: 80,
			Message:    "Running post-expand hooks",
		})

		err = s.runHook(ctx, schema.HookNodeAdded)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	err = s.agentService().StopAgents(context.TODO(), ctx.key())
	if err != nil {
		return trace.Wrap(err)
	}

	// erase cloud provider info for this site which may contain sensitive information
	// such as API keys
	s.service.deleteCloudProvider(s.key)

	_, err = s.compareAndSwapOperationState(swap{
		key:            ctx.key(),
		expectedStates: []string{ops.OperationStateExpandDeploying},
		newOpState:     ops.OperationStateCompleted,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	s.reportProgress(ctx, ops.ProgressEntry{
		State:      ops.ProgressStateCompleted,
		Completion: constants.Completed,
		Message:    "expand completed",
	})

	return nil
}

func (s *site) validateExpand(op *ops.SiteOperation, req *ops.OperationUpdateRequest) error {
	if op.Provisioner == schema.ProvisionerOnPrem {
		if len(req.Servers) > 1 {
			return trace.BadParameter(
				"can only add one node at a time, stop agents on %v extra node(-s)", len(req.Servers)-1)
		} else if len(req.Servers) == 0 {
			return trace.BadParameter(
				"no servers provided, run agent command on the node you want to join")
		}
	}
	for role, _ := range req.Profiles {
		profile, err := s.app.Manifest.NodeProfiles.ByName(role)
		if err != nil {
			return trace.Wrap(err)
		}
		if profile.ExpandPolicy == schema.ExpandPolicyFixed {
			return trace.BadParameter(
				"server profile %q does not allow expansion", role)
		}
	}

	labels := map[string]string{
		schema.ServiceLabelRole: string(schema.ServiceRoleMaster),
	}
	masters, err := s.teleport().GetServers(context.TODO(), s.domainName, labels)
	if err != nil {
		return trace.Wrap(err)
	}

	err = setClusterRoles(req.Servers, *s.app, len(masters))
	return trace.Wrap(err)
}

func (s *site) getTeleportSecrets() (*teleportSecrets, error) {
	withPrivateKey := true
	authorities, err := s.teleport().CertAuthorities(withPrivateKey)
	if err != nil {
		return nil, trace.Wrap(err, "failed to query cert authorities")
	}

	var hostPrivateKey, userPrivateKey []byte
	for _, ca := range authorities {
		if len(ca.GetSigningKeys()) == 0 {
			log.Errorf("no signing key of type %v", ca.GetType())
			continue
		}
		switch ca.GetType() {
		case teleservices.HostCA:
			hostPrivateKey = ca.GetSigningKeys()[0]
		case teleservices.UserCA:
			userPrivateKey = ca.GetSigningKeys()[0]
		}
	}

	var errors []error
	if hostPrivateKey == nil {
		errors = append(errors, trace.NotFound("host CA not found"))
	}
	if userPrivateKey == nil {
		errors = append(errors, trace.NotFound("user CA not found"))
	}
	if len(errors) > 0 {
		return nil, trace.NewAggregate(errors...)
	}

	hostKey, err := ssh.ParsePrivateKey(hostPrivateKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	hostPublicKey := ssh.MarshalAuthorizedKey(hostKey.PublicKey())

	userKey, err := ssh.ParsePrivateKey(userPrivateKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	userPublicKey := ssh.MarshalAuthorizedKey(userKey.PublicKey())

	return &teleportSecrets{
		HostCAPrivateKey: hostPrivateKey,
		HostCAPublicKey:  hostPublicKey,
		UserCAPrivateKey: userPrivateKey,
		UserCAPublicKey:  userPublicKey,
	}, nil
}

func (s *site) addServer(ctx *operationContext, provServer *ProvisionedServer, master *teleportServer, numMasters int) error {
	s.reportProgress(ctx, ops.ProgressEntry{
		State:      ops.ProgressStateInProgress,
		Completion: 40,
		Message:    "Configuring system packages",
	})

	runner := &teleportRunner{ctx, s.domainName, s.teleport()}
	serverRunner := &serverRunner{master, runner}

	// get teleport signing key
	teleportCA, err := s.getTeleportSecrets()
	if err != nil {
		return trace.Wrap(err)
	}

	var out []byte
	command := s.etcdctlCommand("member", "list")
	err = utils.Retry(defaults.RetryInterval, defaults.RetryLessAttempts, func() error {
		out, err = runner.Run(master, command...)
		return trace.Wrap(err)
	})
	if err != nil {
		ctx.RecordError("failed listing system service etcd members")
		return trace.Wrap(err)
	}

	initialCluster, err := utils.EtcdInitialCluster(string(out))
	if err != nil {
		ctx.RecordError("failed configuring system service etcd")
		return trace.Wrap(err)
	}

	etcd := etcdConfig{
		initialCluster:      initialCluster,
		initialClusterState: etcdExistingCluster,
		proxyMode:           etcdProxyOn,
	}

	secretsPackage, err := s.planetSecretsPackage(provServer)
	if err != nil {
		return trace.Wrap(err)
	}

	planetPackage, err := s.app.Manifest.RuntimePackage(provServer.Profile)
	if err != nil {
		return trace.Wrap(err)
	}

	configPackage, err := s.planetConfigPackage(provServer, planetPackage.Version)
	if err != nil {
		return trace.Wrap(err)
	}

	docker, err := s.selectDockerConfig(ctx.operation, provServer.Role, s.app.Manifest)
	if err != nil {
		return trace.Wrap(err)
	}

	config := planetConfig{
		etcd:          etcd,
		docker:        *docker,
		planetPackage: *planetPackage,
		configPackage: *configPackage,
	}

	if provServer.IsMaster() {
		if err := s.configureTeleportMaster(ctx, teleportCA, provServer); err != nil {
			ctx.RecordError("failed configuring system service teleport")
			return trace.Wrap(err)
		}

		masterParams := planetMasterParams{
			master:            provServer,
			secretsPackage:    secretsPackage,
			serviceSubnetCIDR: ctx.operation.InstallExpand.Subnets.Service,
		}
		// if we have a connection to ops center set up, configure
		// SNI host so opscenter can dial in
		trustedCluster, err := storage.GetTrustedCluster(s.backend())
		if err == nil {
			masterParams.sniHost = trustedCluster.GetSNIHost()
		}
		if err := s.configurePlanetMasterSecrets(masterParams); err != nil {
			ctx.RecordError("failed configuring system service planet")
			return trace.Wrap(err)
		}

		config.master = masterConfig{
			electionEnabled: false,
			addr:            s.teleport().GetPlanetLeaderIP(),
		}
		err = s.configurePlanetMaster(provServer, ctx.operation, config,
			*secretsPackage, *configPackage)
		if err != nil {
			ctx.RecordError("failed configuring system service planet")
			return trace.Wrap(err)
		}
	} else {
		if err = s.configurePlanetNodeSecrets(provServer, secretsPackage); err != nil {
			ctx.RecordError("failed configuring system service planet")
			return trace.Wrap(err)
		}

		err = s.configurePlanetNode(
			provServer,
			ctx.operation,
			config,
			*secretsPackage, *configPackage)
		if err != nil {
			ctx.RecordError("failed configuring system service planet")
			return trace.Wrap(err)
		}
	}

	// Add other packages necessary for installation
	provServer.PackageSet.AddPackage(
		s.gravityPackage, map[string]string{pack.InstalledLabel: pack.InstalledLabel})

	err = s.configureUserApp(provServer)
	if err != nil {
		return trace.Wrap(err)
	}

	err = s.configureTeleportKeyPair(teleportCA, provServer, teleport.RoleNode)
	if err != nil {
		return trace.Wrap(err)
	}

	err = s.configureTeleportNode(ctx, master.IP, provServer)
	if err != nil {
		return trace.Wrap(err)
	}

	err = s.setupRemoteEnvironment(ctx, provServer, s.agentRunner(ctx))
	if err != nil {
		return trace.Wrap(err)
	}

	// check status
	command = s.gravityCommand("planet", "status", "--", "--local")
	var node *teleportServer
	err = utils.Retry(defaults.RetryInterval, defaults.RetryAttempts, func() error {
		node, err = s.getTeleportServer(ops.AdvertiseIP, provServer.AdvertiseIP)
		if err != nil {
			return trace.Wrap(err)
		}
		out, err = runner.Run(node, command...)
		if err != nil {
			return trace.Wrap(err)
		}
		ctx.Infof("check status got output: '%v'", string(out))
		if err := checkRunning(out); err != nil {
			return trace.Wrap(err)
		}
		return nil
	})
	if err != nil {
		ctx.RecordError("system service planet failed to start")
		return trace.Wrap(err)
	}

	s.reportProgress(ctx, ops.ProgressEntry{
		State:      ops.ProgressStateInProgress,
		Completion: 60,
		Message:    "Registering with Kubernetes",
	})

	err = s.updateUsers(ctx, provServer)
	if err != nil {
		log.Warnf("failed to update host users, kubectl might not work on host: %v",
			trace.DebugReport(err))
	}

	// wait until the new node has registered with the Kubernetes cluster
	command = s.planetEnterCommand(defaults.KubectlBin,
		"get", "nodes", "-l", fmt.Sprintf("%v=%v", defaults.KubernetesHostnameLabel, provServer.KubeNodeID()))

	if err = utils.Retry(defaults.RetryInterval, defaults.RetryAttempts, func() error {
		out, err := serverRunner.Run(command...)
		if err != nil {
			return trace.Wrap(err)
		}
		// empty output means the node hasn't registered yet (not visible via kubectl)
		if len(strings.TrimSpace(string(out))) == 0 {
			return trace.Errorf("node %q has not registered yet", provServer.KubeNodeID())
		}
		return nil
	}); err != nil {
		ctx.RecordError("kubernetes node %v failed to join the cluster", provServer.KubeNodeID())
		return trace.Wrap(err)
	}

	// once we've passed a certain amount of nodes in the cluster, new etcd servers
	// stay in proxy mode
	if provServer.IsMaster() {
		if err := s.promoteEtcd(ctx, runner, master, node, provServer); err != nil {
			ctx.RecordError("failed promoting etcd node %v to member", provServer.KubeNodeID())
			return trace.Wrap(err)
		}
		// TODO: configure the master node already with election enabled
		err = utils.Retry(defaults.RetryInterval, defaults.RetryAttempts, func() error {
			err := s.resumeLeaderElection(serverRunner, provServer)
			if err != nil {
				return trace.Wrap(err)
			}
			return nil
		})
		if err != nil {
			ctx.RecordError("failed to resume leader election")
			return trace.Wrap(err)
		}
	}

	s.reportProgress(ctx, ops.ProgressEntry{
		State:      ops.ProgressStateInProgress,
		Completion: 70,
		Message:    "Updating Kubernetes node labels",
	})

	// once it has registered, label it appropriately
	command = s.planetEnterCommand(
		defaults.KubectlBin,
		fmt.Sprintf("--kubeconfig=%v", constants.PrivilegedKubeconfig),
		"label", "--overwrite=true", "nodes", "-l",
		fmt.Sprintf("%v=%v", defaults.KubernetesHostnameLabel, provServer.KubeNodeID()),
		fmt.Sprintf("%v=%v", defaults.KubernetesAdvertiseIPLabel, provServer.AdvertiseIP))
	command = append(command, provServer.Profile.LabelValues()...)
	if err = utils.Retry(5*time.Second, 10, func() error {
		_, err := serverRunner.Run(command...)
		return trace.Wrap(err)
	}); err != nil {
		ctx.RecordError("failed to set kubernetes node %v labels", provServer.KubeNodeID())
		return trace.Wrap(err)
	}

	// once it has registered, taint it appropriately
	if len(provServer.Profile.TaintValues()) != 0 {
		command = s.planetEnterCommand(
			defaults.KubectlBin,
			fmt.Sprintf("--kubeconfig=%v", constants.PrivilegedKubeconfig),
			"taint", "--overwrite=true", "nodes", "-l",
			fmt.Sprintf("%v=%v", defaults.KubernetesHostnameLabel, provServer.KubeNodeID()))
		command = append(command, provServer.Profile.TaintValues()...)
		err = utils.Retry(5*time.Second, 10, func() error {
			_, err := serverRunner.Run(command...)
			return trace.Wrap(err)
		})
		if err != nil {
			ctx.RecordError("failed to set kubernetes node %v taints", provServer.KubeNodeID())
			return trace.Wrap(err)
		}
	}

	ctx.RecordInfo("node %v has joined the cluster", provServer.KubeNodeID())
	return nil
}

// promoteEtcd adds a new provisioned server as a member to the existing etcd cluster
func (s *site) promoteEtcd(ctx *operationContext, runner *teleportRunner, master, node *teleportServer, provServer *ProvisionedServer) error {
	memberName := provServer.EtcdMemberName(s.domainName)

	command := s.etcdctlCommand(
		"member", "add", memberName, fmt.Sprintf("https://%v:%v", provServer.AdvertiseIP, etcdPeerPort))

	out, err := runner.Run(master, command...)
	if err != nil {
		ctx.RecordError("failed to add etcd member %v to the cluster", memberName)
		return trace.Wrap(err)
	}
	ctx.Infof("added a new etcd member: %v", string(out))

	name, initialCluster, initialClusterState, err := utils.EtcdParseAddMember(string(out))
	if err != nil {
		return trace.Wrap(err)
	}

	command = s.planetEnterCommand(
		defaults.PlanetBin, "etcd", "promote", "--name", name, "--initial-cluster", initialCluster,
		"--initial-cluster-state", initialClusterState)
	out, err = runner.Run(node, command...)
	if err != nil {
		ctx.RecordError("failed to promote etcd member %v", memberName)
		return trace.Wrap(err, "failed to promote etcd node: %s", out)
	}
	ctx.Infof("promoted etcd proxy to a full member: %v", string(out))

	return nil
}
