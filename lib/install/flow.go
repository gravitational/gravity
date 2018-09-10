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

package install

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/gravitational/gravity/lib/checks"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	validationpb "github.com/gravitational/gravity/lib/network/validation/proto"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/rpc"
	pb "github.com/gravitational/gravity/lib/rpc/proto"
	rpcserver "github.com/gravitational/gravity/lib/rpc/server"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/cenkalti/backoff"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// StartInteractiveInstall starts installation that was initiated in
// wizard mode
func (i *Installer) StartInteractiveInstall() error {
	i.send(Event{Progress: &ops.ProgressEntry{
		Message: "Waiting for the operation to start",
	}})
	ticker := backoff.NewTicker(backoff.NewConstantBackOff(1 * time.Second))
	defer ticker.Stop()
	for {
		select {
		case <-i.Context.Done():
			i.Warnf("Context is closing: %v.", i.Context.Err())
			return nil
		case tm := <-ticker.C:
			if tm.IsZero() {
				return trace.ConnectionProblem(nil, "timeout")
			}
			sites, err := i.Operator.GetSites(i.AccountID)
			if err != nil {
				i.Warnf("Failed to get sites: %v.", trace.DebugReport(err))
				continue
			}
			if len(sites) == 0 {
				i.Info("No sites created yet.")
				continue
			}
			i.Cluster = &sites[0]
			// set site domain set by user, otherwise we will attempt
			// to generate new cluster
			i.SiteDomain = i.Cluster.Domain
			operations, err := i.Operator.GetSiteOperations(i.Cluster.Key())
			if err != nil {
				i.Warnf("Failed to get operations: %v.", trace.DebugReport(err))
				continue
			}
			if len(operations) == 0 {
				i.Info("No operations created yet.")
				continue
			}
			op := operations[0]
			i.OperationKey = ops.SiteOperationKey{
				AccountID:   i.AccountID,
				SiteDomain:  i.Cluster.Domain,
				OperationID: op.ID,
			}
			i.Infof("Found operation key: %v.", i.OperationKey)
			if op.State != ops.OperationStateReady {
				i.Infof("Operation %v is not ready yet.", op.ID)
				continue
			}
			err = i.StartOperation()
			if err != nil {
				return trace.Wrap(err, "failed to kick off installation")
			}
			go i.PollProgress(nil)
			return nil
		}
	}
}

// NewClusterRequest constructs a request to create a new cluster
func (i *Installer) NewClusterRequest() ops.NewSiteRequest {
	return ops.NewSiteRequest{
		AppPackage:   i.AppPackage.String(),
		AccountID:    i.AccountID,
		Email:        fmt.Sprintf("installer@%v", i.SiteDomain),
		Provider:     i.CloudProvider,
		DomainName:   i.SiteDomain,
		Resources:    i.Resources,
		InstallToken: i.Config.Token,
		ServiceUser: storage.OSUser{
			Name: i.Config.ServiceUser.Name,
			UID:  strconv.Itoa(i.Config.ServiceUser.UID),
			GID:  strconv.Itoa(i.Config.ServiceUser.GID),
		},
		CloudConfig: storage.CloudConfig{
			GCENodeTags: i.Config.GCENodeTags,
		},
		DNSOverrides: i.DNSOverrides,
		DNSConfig:    i.DNSConfig,
	}
}

// StartCLIInstall starts non-interactive installation
func (i *Installer) StartCLIInstall() (err error) {
	i.Cluster, err = i.Operator.CreateSite(i.engine.NewClusterRequest())
	if err != nil {
		return trace.Wrap(err)
	}
	i.flavor, err = i.getFlavor()
	if err != nil {
		return trace.Wrap(err)
	}
	err = i.checkAndSetServerProfile()
	if err != nil {
		return trace.Wrap(err)
	}
	err = checks.RunLocalChecks(checks.LocalChecksRequest{
		Context:  i.Context,
		Manifest: i.Cluster.App.Manifest,
		Role:     i.Role,
		Options: &validationpb.ValidateOptions{
			VxlanPort: int32(i.VxlanPort),
			DnsAddrs:  i.DNSConfig.Addrs,
			DnsPort:   int32(i.DNSConfig.Port),
		},
		AutoFix: true,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return i.LaunchOperation(ops.CreateSiteInstallOperationRequest{
		SiteDomain: i.Cluster.Domain,
		AccountID:  i.Cluster.AccountID,
		// With CLI install flow we always rely on external provisioner
		Provisioner: schema.ProvisionerOnPrem,
		Variables: storage.OperationVariables{
			System: storage.SystemVariables{
				Docker: i.Docker,
			},
			OnPrem: storage.OnPremVariables{
				PodCIDR:     i.PodCIDR,
				ServiceCIDR: i.ServiceCIDR,
				VxlanPort:   i.VxlanPort,
				DNS:         i.DNSConfig,
			},
		},
		Profiles: ServerRequirements(*i.flavor),
	})
}

// LaunchOperation creates the install operation according the provided
// request and launches an install agent
func (i *Installer) LaunchOperation(req ops.CreateSiteInstallOperationRequest) error {
	key, err := i.Operator.CreateSiteInstallOperation(req)
	if err != nil {
		return trace.Wrap(err)
	}
	i.OperationKey = *key
	i.Debugf("Got operation key: %v.", i.OperationKey)
	op, err := i.Operator.GetSiteOperation(i.OperationKey)
	if err != nil {
		return trace.Wrap(err)
	}
	if op.InstallExpand == nil {
		return trace.BadParameter("no install state for %v", key)
	}
	i.Debugf("Got operation state: %#v.", op.InstallExpand)
	agentInstructions, ok := op.InstallExpand.Agents[i.Role]
	if !ok {
		return trace.NotFound("agent instructions not found for %v", i.Role)
	}
	go func() {
		err := i.run(agentInstructions.AgentURL)
		if err != nil {
			i.send(Event{Error: err})
		}
	}()
	return nil
}

// StartAgent launches the RPC install agent
func (i *Installer) StartAgent(agentURL string) (rpcserver.Server, error) {
	listener, err := net.Listen("tcp", defaults.GravityRPCAgentAddr(i.AdvertiseAddr))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	serverCreds, clientCreds, err := rpc.Credentials(defaults.RPCAgentSecretsDir)
	if err != nil {
		listener.Close()
		return nil, trace.Wrap(err)
	}

	var mounts []*pb.Mount
	for name, source := range i.Mounts {
		mounts = append(mounts, &pb.Mount{Name: name, Source: source})
	}

	runtimeConfig := pb.RuntimeConfig{
		SystemDevice: i.SystemDevice,
		DockerDevice: i.DockerDevice,
		Role:         i.Role,
		Mounts:       mounts,
	}
	if err = FetchCloudMetadata(i.CloudProvider, &runtimeConfig); err != nil {
		return nil, trace.Wrap(err)
	}

	config := rpcserver.PeerConfig{
		Config: rpcserver.Config{
			Listener: listener,
			Credentials: rpcserver.Credentials{
				Server: serverCreds,
				Client: clientCreds,
			},
			RuntimeConfig: runtimeConfig,
		},
	}
	agent, err := startAgent(agentURL, config, i)
	if err != nil {
		listener.Close()
		return nil, trace.Wrap(err)
	}
	return agent, nil
}

func (i *Installer) run(agentURL string) error {
	agent, err := i.StartAgent(agentURL)
	if err != nil {
		return trace.Wrap(err)
	}

	go agent.Serve()

	err = i.waitForAgents()
	if err != nil {
		return trace.Wrap(err)
	}

	i.Info("Starting installation.")

	err = i.StartOperation()
	if err != nil {
		return trace.Wrap(err, "failed to start installation")
	}

	i.PollProgress(agent.Done())
	return nil
}

// createAdminAgent creates an admin agent for the cluster being installed
func (i *Installer) createAdminAgent() error {
	agent, err := i.Process.UsersService().CreateClusterAdminAgent(i.SiteDomain,
		storage.NewUser(storage.ClusterAdminAgent(i.SiteDomain), storage.UserSpecV2{
			AccountID: defaults.SystemAccountID,
		}))
	if err != nil {
		return trace.Wrap(err)
	}
	i.Infof("Created cluster agent: %v.", agent)
	return nil
}

// StartOperation inializes installation plan, instantiates the install
// FSM engine and launches the operation (plan execution)
func (i *Installer) StartOperation() error {
	i.Info("Starting installation.")
	i.sendMessage("Starting the installation")
	if err := i.createAdminAgent(); err != nil {
		return trace.Wrap(err, "failed to create cluster admin agent")
	}
	if err := i.initOperationPlan(); err != nil {
		return trace.Wrap(err, "failed to initialize install plan")
	}
	// in the manual mode do not launch FSM
	if i.Manual {
		i.Info("Manual install mode, not starting automatic plan execution.")
		i.sendMessage(`Installation was started in manual mode
Inspect the operation plan using "gravity plan" and execute plan phases manually on respective nodes using "gravity install --phase=<phase-id>"
After all phases have completed successfully, shutdown this installer process using Ctrl-C`)
		return nil
	}
	fsm, err := i.engine.GetFSM()
	if err != nil {
		return trace.Wrap(err)
	}
	go i.startFSM(fsm)
	return nil
}

// GetFSM returns the installer FSM engine
func (i *Installer) GetFSM() (*fsm.FSM, error) {
	return NewFSM(FSMConfig{
		OperationKey:   i.OperationKey,
		Packages:       i.Packages,
		Apps:           i.Apps,
		Operator:       i.Operator,
		LocalPackages:  i.LocalPackages,
		LocalApps:      i.LocalApps,
		LocalBackend:   i.LocalBackend,
		Insecure:       i.Insecure,
		UserLogFile:    i.UserLogFile,
		DNSConfig:      i.DNSConfig,
		ReportProgress: true,
	})
}

// startFSM executes operation plan using the provided FSM
func (i *Installer) startFSM(fsm *fsm.FSM) {
	force := false
	err := fsm.ExecutePlan(i.Context, utils.NewNopProgress(), force)
	if err != nil {
		i.Errorf("Failed to execute plan: %v.", trace.DebugReport(err))
	}
	// regardless of the plan execution outcome we need to mark
	// the operation completed (success or fail) so progress
	// monitors (UI or CLI) can act accordingly
	i.engine.OnPlanComplete(fsm, err)
}

// Event represents install event
type Event struct {
	Progress *ops.ProgressEntry
	Error    error
}

func (i *Installer) send(e Event) {
	select {
	case i.EventsC <- e:
	case <-i.Context.Done():
	default:
		i.Warnf("Failed to send event, events channel is blocked.")
	}
}

func (i *Installer) sendMessage(format string, args ...interface{}) {
	i.send(Event{Progress: &ops.ProgressEntry{Message: fmt.Sprintf(format, args...)}})
}

func (i *Installer) PollProgress(agentDoneCh <-chan struct{}) {
	pollProgress(i.Context, i.send, i.Operator, i.OperationKey, agentDoneCh)
}

func (i *Installer) canContinue(servers []checks.ServerInfo) bool {
	profiles := make(map[string]int)
	for _, node := range i.flavor.Nodes {
		profiles[node.Profile] = node.Count
	}
	for _, s := range servers {
		counter := profiles[s.Role]
		counter -= 1
		profiles[s.Role] = counter
	}
	for role, counter := range profiles {
		if counter > 0 {
			i.send(Event{
				Progress: &ops.ProgressEntry{
					Message: fmt.Sprintf(
						"Still waiting for %v nodes of role %q", counter, role),
				},
			})
			log.Infof("Still waiting for %v nodes of role %q.", counter, role)
			return false
		}
	}
	i.send(Event{
		Progress: &ops.ProgressEntry{
			Message: "All agents have connected!",
		},
	})
	log.Info("All agents have connected!")
	return true
}

func (i *Installer) waitForAgents() error {
	ticker := backoff.NewTicker(&backoff.ExponentialBackOff{
		InitialInterval: time.Second,
		Multiplier:      1.0,
		MaxInterval:     time.Second,
		MaxElapsedTime:  5 * time.Minute,
		Clock:           backoff.SystemClock,
	})

	for {
		select {
		case <-i.Context.Done():
			return trace.ConnectionProblem(nil, "context is closing")
		case tm := <-ticker.C:
			if tm.IsZero() {
				return trace.ConnectionProblem(nil, "timed out waiting for agents to join")
			}
			report, err := i.Operator.GetSiteInstallOperationAgentReport(i.OperationKey)
			if err != nil {
				log.Warningf("Failed to get agent report: %v.", err)
				continue
			}
			if !i.canContinue(report.Servers) {
				continue
			}
			log.Infof("Installation can proceed! %v", report)
			err = i.UpdateOperationState()
			if err != nil {
				return trace.Wrap(err)
			}
			return nil
		}
	}
}

// UpdateOperationState updates the operation data according to the agent report
func (i *Installer) UpdateOperationState() error {
	report, err := i.Operator.GetSiteInstallOperationAgentReport(i.OperationKey)
	if err != nil {
		return trace.Wrap(err, "failed to get agent report")
	}
	operation, err := i.Operator.GetSiteOperation(i.OperationKey)
	if err != nil {
		return trace.Wrap(err, "failed to get operation")
	}
	request, err := getServers(*operation, report.Servers)
	if err != nil {
		return trace.Wrap(err, "failed to parse report: %#v", report)
	}
	err = i.Operator.UpdateInstallOperationState(i.OperationKey, *request)
	if err != nil {
		return trace.Wrap(err)
	}
	i.Infof("Updated operation state: %#v.", request)
	return nil
}

func (i *Installer) getFlavor() (*schema.Flavor, error) {
	// pick flavor and server profile
	flavors := i.Cluster.App.Manifest.Installer.Flavors
	if i.Flavor == "" {
		if flavors.Default != "" {
			i.Flavor = flavors.Default
			i.Infof("Flavor is not set, picking default flavor: %q.", i.Flavor)
		} else {
			i.Flavor = flavors.Items[0].Name
			i.Infof("Flavor is not set, picking first flavor: %q.", i.Flavor)
		}
	}
	flavor := i.Cluster.App.Manifest.FindFlavor(i.Flavor)
	if flavor == nil {
		return nil, trace.NotFound("install flavor %q is not found", i.Flavor)
	}
	return flavor, nil
}

func (i *Installer) checkAndSetServerProfile() error {
	if i.Role == "" {
		for _, node := range i.flavor.Nodes {
			i.Role = node.Profile
			i.Infof("No server profile specified, picking the first one: %q.", i.Role)
			break
		}
	}
	for _, profile := range i.Cluster.App.Manifest.NodeProfiles {
		if profile.Name == i.Role {
			return nil
		}
	}
	return trace.NotFound("server role %q is not found", i.Role)
}

func pollProgress(ctx context.Context, send func(Event), operator ops.Operator,
	opKey ops.SiteOperationKey, agentDoneCh <-chan struct{}) {
	ticker := backoff.NewTicker(backoff.NewConstantBackOff(1 * time.Second))
	defer ticker.Stop()
	var progress *ops.ProgressEntry
	var lastProgress *ops.ProgressEntry
	var err error
	var agentClosed bool
	for {
		select {
		case <-ctx.Done():
			send(Event{Error: trace.ConnectionProblem(nil, "context is closing")})
		case <-agentDoneCh:
			log.Debug("Agent shut down.")
			// avoid receiving on closed channel
			agentDoneCh = nil
			agentClosed = true
		case tm := <-ticker.C:
			if tm.IsZero() {
				send(Event{Error: trace.ConnectionProblem(nil, "timeout")})
			}
			progress, err = operator.GetSiteOperationProgress(opKey)
			if err != nil {
				progress = newCompletedProgressEntry()
				log.Warnf("Failed to query operation progress: %v.",
					trace.DebugReport(err))
				if !agentClosed {
					continue
				}
			}
			if lastProgress == nil || !lastProgress.IsEqual(*progress) {
				updateProgress(*progress, send)
			}
			if progress.IsCompleted() {
				return
			}
			lastProgress = progress
		}
	}
}

func getServers(op ops.SiteOperation, servers []checks.ServerInfo) (*ops.OperationUpdateRequest, error) {
	req := ops.OperationUpdateRequest{
		Profiles: make(map[string]storage.ServerProfileRequest),
	}
	for _, serverInfo := range servers {
		if serverInfo.AdvertiseAddr == "" {
			return nil, trace.BadParameter("%v has no advertise_addr specified", serverInfo)
		}
		if serverInfo.Role == "" {
			return nil, trace.BadParameter("%v has no role specified", serverInfo)
		}
		var mounts []storage.Mount
		for _, mount := range serverInfo.Mounts {
			mounts = append(mounts, storage.Mount{Name: mount.Name, Source: mount.Source})
		}
		ip, _ := utils.SplitHostPort(serverInfo.AdvertiseAddr, "")
		server := storage.Server{
			AdvertiseIP: ip,
			Hostname:    serverInfo.GetHostname(),
			Role:        serverInfo.Role,
			OSInfo:      serverInfo.GetOS(),
			Mounts:      mounts,
			User:        serverInfo.GetUser(),
			Provisioner: op.Provisioner,
			Created:     time.Now().UTC(),
		}
		if serverInfo.CloudMetadata != nil {
			server.Nodename = serverInfo.CloudMetadata.NodeName
			server.InstanceType = serverInfo.CloudMetadata.InstanceType
			server.InstanceID = serverInfo.CloudMetadata.InstanceId
		}
		req.Servers = append(req.Servers, server)
		profile := req.Profiles[serverInfo.Role]
		profile.Count += 1
		req.Profiles[serverInfo.Role] = profile
	}
	return &req, nil
}

func newCompletedProgressEntry() *ops.ProgressEntry {
	return &ops.ProgressEntry{
		Completion: constants.Completed,
		State:      ops.ProgressStateCompleted,
	}
}

func updateProgress(progress ops.ProgressEntry, send func(Event)) {
	send(Event{Progress: &progress})
	if progress.State == ops.ProgressStateCompleted {
		log.Info("operation completed")
	}
	if progress.State == ops.ProgressStateFailed {
		log.Info("operation failed")
	}
}

// ServerRequirements computes server requirements based on the selected flavor
func ServerRequirements(flavor schema.Flavor) map[string]storage.ServerProfileRequest {
	result := make(map[string]storage.ServerProfileRequest)
	for _, node := range flavor.Nodes {
		result[node.Profile] = storage.ServerProfileRequest{
			Count: node.Count,
		}
	}
	return result
}
