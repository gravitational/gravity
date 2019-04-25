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

package expand

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/app/client"
	"github.com/gravitational/gravity/lib/checks"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/httplib"
	"github.com/gravitational/gravity/lib/install"
	"github.com/gravitational/gravity/lib/install/server"
	"github.com/gravitational/gravity/lib/localenv"
	validationpb "github.com/gravitational/gravity/lib/network/validation/proto"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/ops/events"
	"github.com/gravitational/gravity/lib/ops/opsclient"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/pack/webpack"
	pb "github.com/gravitational/gravity/lib/rpc/proto"
	rpcserver "github.com/gravitational/gravity/lib/rpc/server"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/cenkalti/backoff"
	"github.com/gravitational/coordinate/leader"
	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// NewPeer returns new cluster peer client
func NewPeer(ctx context.Context, config PeerConfig) (*Peer, error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	localCtx, cancel := context.WithCancel(ctx)
	server := server.New(ctx)
	peer := &Peer{
		PeerConfig: config,
		ctx:        localCtx,
		cancel:     cancel,
		server:     server,
	}
	return peer, nil
}

// Peer is a client that manages joining the cluster
type Peer struct {
	PeerConfig
	// ctx defines the local peer context used to cancel internal operation
	ctx    context.Context
	cancel context.CancelFunc
	// agentDoneCh is the agent's done channel.
	// The channel is only set after the agent has been started
	agentDoneCh <-chan struct{}
	// agent is this peer's RPC agent
	agent *rpcserver.PeerServer
	// server is the gRPC installer server
	server *server.Server
}

// Serve starts the server
func (p *Peer) Serve(listener net.Listener) error {
	p.server.Run(func() {
		watchReconnects(p.ctx, p.cancel, p.WatchCh, p.FieldLogger)
	})
	return trace.Wrap(p.server.Serve(p, listener))
}

// Stop shuts down this RPC agent
func (p *Peer) Stop(ctx context.Context) error {
	p.Info("Stop.")
	p.server.Stop(ctx)
	return nil
}

// Abort aborts this RPC agent
func (p *Peer) Abort(ctx context.Context) error {
	p.Info("Abort.")
	p.server.Interrupt(ctx)
	return nil
}

// Shutdown shuts down this peer.
// Implements server.Executor
func (p *Peer) Shutdown(ctx context.Context) error {
	p.Info("Shutdown.")
	return trace.Wrap(p.stop(ctx))
}

// ExecuteOperation executes the peer operation (join or just serving an agent).
// Implements server.Executor
func (p *Peer) ExecuteOperation() (err error) {
	p.Info("ExecuteOperation.")
	if err := p.init(); err != nil {
		return trace.Wrap(err)
	}
	if err := p.run(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// AbortOperation aborts the installation and cleans up the operation state.
// Implements server.Executor
func (p *Peer) AbortOperation(ctx context.Context) error {
	p.Info("AbortOperation.")
	var errors []error
	if err := p.stop(ctx); err != nil {
		errors = append(errors, err)
	}
	p.server.WaitForOperation()
	if err := p.AbortHandler(ctx); err != nil {
		errors = append(errors, err)
	}
	return trace.NewAggregate(errors...)
}

// printStep publishes a progress entry described with (format, args) tuple to the client
func (p *Peer) printStep(format string, args ...interface{}) error {
	event := server.Event{Progress: &ops.ProgressEntry{Message: fmt.Sprintf(format, args...)}}
	return p.server.Send(event)
}

// PeerConfig is for peers joining the cluster
type PeerConfig struct {
	// Peers is a list of peer addresses
	Peers []string
	// AdvertiseAddr is advertise addr of this node
	AdvertiseAddr string
	// ServerAddr is optional address of the agent server.
	// It will be derived from agent instructions if unspecified
	ServerAddr string
	// CloudProvider is the node cloud provider
	CloudProvider string
	// WatchCh is channel that relays peer reconnect events
	WatchCh chan rpcserver.WatchEvent
	// RuntimeConfig is peer's runtime configuration
	pb.RuntimeConfig
	// FieldLogger is used for logging
	log.FieldLogger
	// DebugMode turns on FSM debug mode
	DebugMode bool
	// Insecure turns on FSM insecure mode
	Insecure bool
	// LocalBackend is local backend of the joining node
	LocalBackend storage.Backend
	// LocalApps is local apps service of the joining node
	LocalApps app.Applications
	// LocalPackages is local package service of the joining node
	LocalPackages pack.PackageService
	// JoinBackend is the local backend where join-specific operation data is stored
	JoinBackend storage.Backend
	// OperationID is the ID of existing join operation created via UI
	OperationID string
	// StateDir defines where peer will store operation-specific data
	StateDir string
	// AbortHandler specifies the handler for aborting the installation
	AbortHandler func(context.Context) error
}

// CheckAndSetDefaults checks the parameters and autodetects some defaults
func (c *PeerConfig) CheckAndSetDefaults() (err error) {
	if len(c.Peers) == 0 {
		return trace.BadParameter("missing Peers")
	}
	if c.AdvertiseAddr == "" {
		return trace.BadParameter("missing AdvertiseAddr")
	}
	if err := install.CheckAddr(c.AdvertiseAddr); err != nil {
		return trace.Wrap(err)
	}
	if c.Token == "" {
		return trace.BadParameter("missing Token")
	}
	if c.LocalBackend == nil {
		return trace.BadParameter("missing LocalBackned")
	}
	if c.LocalApps == nil {
		return trace.BadParameter("missing LocalApps")
	}
	if c.LocalPackages == nil {
		return trace.BadParameter("missing LocalPackages")
	}
	if c.JoinBackend == nil {
		return trace.BadParameter("missing JoinBackend")
	}
	if c.StateDir == "" {
		return trace.BadParameter("missing StateDir")
	}
	if c.AbortHandler == nil {
		return trace.BadParameter("missing AbortHandler")
	}
	c.CloudProvider, err = install.ValidateCloudProvider(c.CloudProvider)
	if err != nil {
		return trace.Wrap(err)
	}
	if c.FieldLogger == nil {
		c.FieldLogger = log.WithField(trace.Component, "peer")
	}
	if c.WatchCh == nil {
		c.WatchCh = make(chan rpcserver.WatchEvent, 1)
	}
	return nil
}

// init initializes the peer
func (p *Peer) init() error {
	if err := p.bootstrap(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// stop stops peer operation
func (p *Peer) stop(ctx context.Context) error {
	p.cancel()
	var errors []error
	if p.agent != nil {
		p.Info("Shut down agent.")
		err := p.agent.Stop(ctx)
		if err != nil {
			errors = append(errors, err)
		}
	}
	return trace.NewAggregate(errors...)
}

func (p *Peer) dialCluster(addr string) (*operationContext, error) {
	targetURL := formatClusterURL(addr)
	httpClient := httplib.GetClient(true)
	operator, err := opsclient.NewBearerClient(targetURL, p.Token, opsclient.HTTPClient(httpClient))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	packages, err := webpack.NewBearerClient(targetURL, p.Token, roundtrip.HTTPClient(httpClient))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	apps, err := client.NewBearerClient(targetURL, p.Token, client.HTTPClient(httpClient))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cluster, err := operator.GetLocalSite()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = p.checkAndSetServerProfile(cluster.App)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	installOp, _, err := ops.GetInstallOperation(cluster.Key(), operator)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = p.runLocalChecks(*cluster, *installOp)
	if err != nil {
		return nil, utils.Abort(err) // stop retrying on failed checks
	}
	var operation *ops.SiteOperation
	if p.OperationID == "" {
		operation, err = p.createExpandOperation(operator, *cluster)
	} else {
		operation, err = p.getExpandOperation(operator, *cluster)
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}
	creds, err := install.LoadRPCCredentials(p.ctx, packages, p.FieldLogger)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	peerURL, err := url.Parse(targetURL)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &operationContext{
		Operator:  operator,
		Packages:  packages,
		Apps:      apps,
		Peer:      peerURL.Host,
		Operation: *operation,
		Cluster:   *cluster,
		Creds:     *creds,
	}, nil
}

// createExpandOperation creates a new expand operation
func (p *Peer) createExpandOperation(operator ops.Operator, cluster ops.Site) (*ops.SiteOperation, error) {
	key, err := operator.CreateSiteExpandOperation(p.ctx, ops.CreateSiteExpandOperationRequest{
		AccountID:   cluster.AccountID,
		SiteDomain:  cluster.Domain,
		Provisioner: schema.ProvisionerOnPrem,
		Servers:     map[string]int{p.Role: 1},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = operator.SetOperationState(*key, ops.SetOperationStateRequest{
		State: ops.OperationStateReady,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	operation, err := operator.GetSiteOperation(*key)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return operation, nil
}

// getExpandOperation returns existing expand operation created via UI
func (p *Peer) getExpandOperation(operator ops.Operator, cluster ops.Site) (*ops.SiteOperation, error) {
	operation, err := operator.GetSiteOperation(ops.SiteOperationKey{
		AccountID:   cluster.AccountID,
		SiteDomain:  cluster.Domain,
		OperationID: p.OperationID,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return operation, nil
}

// dialWizard connects to a wizard
func (p *Peer) dialWizard(addr string) (*operationContext, error) {
	env, err := localenv.NewRemoteEnvironment()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	entry, err := env.LoginWizard(addr)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cluster, operation, err := p.validateWizardState(env.Operator)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = p.checkAndSetServerProfile(cluster.App)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = p.runLocalChecks(*cluster, *operation)
	if err != nil {
		return nil, utils.Abort(err) // stop retrying on failed checks
	}
	creds, err := install.LoadRPCCredentials(p.ctx, env.Packages, p.FieldLogger)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	peerURL, err := url.Parse(entry.OpsCenterURL)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &operationContext{
		Operator:  env.Operator,
		Packages:  env.Packages,
		Apps:      env.Apps,
		Peer:      peerURL.Host,
		Operation: *operation,
		Cluster:   *cluster,
		Creds:     *creds,
	}, nil
}

func (p *Peer) checkAndSetServerProfile(app ops.Application) error {
	if p.Role == "" {
		for _, profile := range app.Manifest.NodeProfiles {
			p.Role = profile.Name
			p.Infof("no role specified, picking %q", p.Role)
			break
		}
	}
	for _, profile := range app.Manifest.NodeProfiles {
		if profile.Name == p.Role {
			return nil
		}
	}
	return utils.Abort(trace.BadParameter(
		"specified node role %q is not defined in the application manifest", p.Role))
}

// runLocalChecks makes sure node satisfies system requirements
func (p *Peer) runLocalChecks(cluster ops.Site, installOperation ops.SiteOperation) error {
	return checks.RunLocalChecks(p.ctx, checks.LocalChecksRequest{
		Manifest: cluster.App.Manifest,
		Role:     p.Role,
		Docker:   cluster.ClusterState.Docker,
		Options: &validationpb.ValidateOptions{
			VxlanPort: int32(installOperation.GetVars().OnPrem.VxlanPort),
			DnsAddrs:  cluster.DNSConfig.Addrs,
			DnsPort:   int32(cluster.DNSConfig.Port),
		},
		AutoFix: true,
	})
}

// operationContext describes the active install/expand operation.
// Used by peers to add new nodes for install/expand and poll progress
// of the operation.
type operationContext struct {
	// Operator is the ops service of cluster or installer
	Operator ops.Operator
	// Packages is the pack service of cluster or installer
	Packages pack.PackageService
	// Apps is the apps service of cluster or installer
	Apps app.Applications
	// Peer is the IP:port of the peer this peer has joined to
	Peer string
	// Operation is the expand operation this peer is executing
	Operation ops.SiteOperation
	// Cluster is the cluster this peer is joining to
	Cluster ops.Site
	// Creds is the RPC agent credentials
	Creds rpcserver.Credentials
}

// connect dials to either a running wizard OpsCenter or a local gravity cluster.
// For wizard, if the dial succeeds, it will join the active installation and return
// an operation context of the active install operation.
//
// For a local gravity cluster, it will attempt to start the expand operation
// and will return an operation context wrapping a new expand operation.
func (p *Peer) connect() (*operationContext, error) {
	ticker := backoff.NewTicker(leader.NewUnlimitedExponentialBackOff())
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			ctx, err := p.tryConnect()
			if err != nil {
				// join token is incorrect, fail immediately and report to user
				if trace.IsAccessDenied(err) {
					return nil, trace.AccessDenied("access denied: bad secret token")
				}
				if err, ok := trace.Unwrap(err).(*utils.AbortRetry); ok {
					return nil, trace.BadParameter(err.OriginalError())
				}
				// most of the time errors are expected, like another operation
				// is in progress, so just retry until we connect (or timeout)
				continue
			}
			return ctx, nil
		case <-p.ctx.Done():
			return nil, trace.Wrap(p.ctx.Err())
		}
	}
}

func (p *Peer) tryConnect() (op *operationContext, err error) {
	p.printStep("Connecting to cluster")
	for _, addr := range p.Peers {
		p.WithField("peer", addr).Debug("Trying peer.")
		op, err = p.dialWizard(addr)
		if err == nil {
			p.WithField("addr", op.Peer).Debug("Connected to wizard.")
			p.printStep("Connected to installer at %v", addr)
			return op, nil
		}
		p.WithError(err).Info("Failed connecting to wizard.")
		if utils.IsAbortError(err) {
			return nil, trace.Wrap(err)
		}
		// already exists error is returned when there's an ongoing install
		// operation, do not attempt to dial the cluster until it completes
		if trace.IsAlreadyExists(err) {
			p.printStep("Waiting for the install operation to finish")
			return nil, trace.Wrap(err)
		}

		op, err = p.dialCluster(addr)
		if err == nil {
			p.WithField("addr", op.Peer).Debug("Connected to cluster.")
			p.printStep("Connected to existing cluster at %v", addr)
			return op, nil
		}
		p.WithError(err).Info("Failed connecting to cluster.")
		if utils.IsAbortError(err) {
			return nil, trace.Wrap(err)
		}
		if trace.IsCompareFailed(err) {
			p.printStep("Waiting for another operation to finish at %v", addr)
		}
	}
	return op, trace.Wrap(err)
}

// getAgent creates an RPC agent instance that, once started, will connect
// to its peer which can be either installer process or existing cluster
func (p *Peer) getAgent(opCtx operationContext) (*rpcserver.PeerServer, error) {
	peerAddr, token, err := getPeerAddrAndToken(opCtx, p.Role)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	p.RuntimeConfig.Token = token
	agent, err := install.NewAgent(p.ctx, install.AgentConfig{
		FieldLogger:   p.FieldLogger,
		AdvertiseAddr: p.AdvertiseAddr,
		ServerAddr:    peerAddr,
		Credentials:   opCtx.Creds,
		RuntimeConfig: p.RuntimeConfig,
		WatchCh:       p.WatchCh,
		AbortHandler:  p.AbortHandler,
	})
	if err != nil {
		if agent != nil {
			agent.Stop(p.ctx)
		}
		return nil, trace.Wrap(err)
	}
	return agent, nil
}

func (p *Peer) run() error {
	ctx, err := p.connect()
	if err != nil {
		return trace.Wrap(err)
	}
	p.WithField("context", fmt.Sprintf("%+v", *ctx)).Info("Connected.")

	p.server.RunProgressLoop(ctx.Operator, ctx.Operation.Key())

	err = p.ensureServiceUserAndBinary(*ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	err = p.checkAndSetServerProfile(ctx.Cluster.App)
	if err != nil {
		return trace.Wrap(err)
	}

	// schedule cleanup function in case anything goes wrong before
	// the operation can start
	defer func() {
		if err == nil {
			return
		}
		p.WithError(err).Warn("Peer is exiting with error.")
		stopCtx, cancel := context.WithTimeout(p.ctx, defaults.AgentStopTimeout)
		defer cancel()
		p.Warn("Stopping peer.")
		if err := p.Stop(stopCtx); err != nil {
			p.WithError(err).Warn("Failed to stop peer.")
		}
		// in case of join via CLI the operation has already been created
		// above but the agent failed to connect so we're deleting the
		// operation because from user's perspective it hasn't started
		//
		// in case of join via UI the peer is joining to the existing
		// operation created via UI so we're not touching it and the
		// user can cancel it in the UI
		if p.OperationID == "" { // operation ID is given in UI usecase
			p.WithField("op", ctx.Operation).Warn("Cleaning up unstarted operation.")
			if err := ctx.Operator.DeleteSiteOperation(ctx.Operation.Key()); err != nil {
				p.WithError(err).Warn("Failed to delete unstarted operation.")
			}
		}
	}()

	p.agent, err = p.getAgent(*ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	p.agentDoneCh = p.agent.Done()

	if ctx.Operation.Type != ops.OperationExpand {
		return trace.Wrap(p.agent.Serve())
	}

	p.server.Run(func() {
		if err := p.agent.Serve(); err != nil {
			p.WithError(err).Warn("Agent failed.")
		}
	})
	return trace.Wrap(p.startExpandOperation(*ctx))
}

// waitForOperation blocks until the join operation is ready
func (p *Peer) waitForOperation(ctx operationContext) error {
	ticker := backoff.NewTicker(backoff.NewConstantBackOff(1 * time.Second))
	defer ticker.Stop()
	log := p.WithField(constants.FieldOperationID, ctx.Operation.ID)
	log.Debug("Waiting for the operation to become ready.")
	for {
		select {
		case <-ticker.C:
			operation, err := ctx.Operator.GetSiteOperation(ctx.Operation.Key())
			if err != nil {
				return trace.Wrap(err)
			}
			if operation.State != ops.OperationStateReady {
				log.Info("Operation is not ready yet.")
				continue
			}
			log.Info("Operation is ready!")
			return nil
		case <-p.ctx.Done():
			return trace.Wrap(p.ctx.Err())
		}
	}
}

func (p *Peer) waitForAgents(ctx operationContext) error {
	ticker := backoff.NewTicker(&backoff.ExponentialBackOff{
		InitialInterval: time.Second,
		Multiplier:      1.5,
		MaxInterval:     10 * time.Second,
		MaxElapsedTime:  5 * time.Minute,
		Clock:           backoff.SystemClock,
	})
	defer ticker.Stop()
	log := p.WithField(constants.FieldOperationID, ctx.Operation.ID)
	log.Debug("Waiting for the agent to join.")
	for {
		select {
		case tm := <-ticker.C:
			if tm.IsZero() {
				return trace.ConnectionProblem(nil, "timed out waiting for agents to join")
			}
			report, err := ctx.Operator.GetSiteExpandOperationAgentReport(ctx.Operation.Key())
			if err != nil {
				log.WithError(err).Warn("Failed to query agent report.")
				continue
			}
			if len(report.Servers) == 0 {
				log.Debug("Agent hasn't joined yet.")
				continue
			}
			op, err := ctx.Operator.GetSiteOperation(ctx.Operation.Key())
			if err != nil {
				log.Warningf("%v", err)
				continue
			}
			req, err := install.GetServerUpdateRequest(*op, report.Servers)
			if err != nil {
				log.WithError(err).Warn("Failed to create server update request.")
				continue
			}
			err = ctx.Operator.UpdateExpandOperationState(ctx.Operation.Key(), *req)
			if err != nil {
				return trace.Wrap(err)
			}
			log.WithField("report", report).Info("Installation can proceed!")
			return nil
		case <-p.ctx.Done():
			return trace.Wrap(p.ctx.Err())
		}
	}
}

func (p *Peer) startExpandOperation(ctx operationContext) error {
	err := p.waitForOperation(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	err = p.waitForAgents(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	err = p.initOperationPlan(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	err = p.syncOperation(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	err = p.emitAuditEvent(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	/*
		if p.Manual {
			p.Silent.Println(`Operation was started in manual mode
		Inspect the operation plan using "gravity plan" and execute plan phases manually on this node using "gravity join --phase=<phase-id>"
		After all phases have completed successfully, complete the operation using "gravity join --complete" and shutdown this process using Ctrl-C`)
			return nil
		}
	*/
	fsm, err := p.getFSM(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	// FIXME: DiscardProgress -> send to client
	fsmErr := fsm.ExecutePlan(p.ctx, utils.DiscardProgress)
	if fsmErr != nil {
		p.WithError(fsmErr).Warn("Failed to execute plan.")
	}
	err = fsm.Complete(fsmErr)
	if err != nil {
		return trace.Wrap(err, "failed to complete operation")
	}
	return nil
}

// emitAuditEvent sends expand operation start event to the cluster audit log.
func (p *Peer) emitAuditEvent(ctx operationContext) error {
	operation, err := ctx.Operator.GetSiteOperation(ctx.Operation.Key())
	if err != nil {
		return trace.Wrap(err)
	}
	events.Emit(p.ctx, ctx.Operator, events.OperationExpandStart,
		events.FieldsForOperation(*operation))
	return nil
}

func (p *Peer) getFSM(ctx operationContext) (*fsm.FSM, error) {
	return NewFSM(FSMConfig{
		Operator:      ctx.Operator,
		OperationKey:  ctx.Operation.Key(),
		Apps:          ctx.Apps,
		Packages:      ctx.Packages,
		LocalBackend:  p.LocalBackend,
		LocalApps:     p.LocalApps,
		LocalPackages: p.LocalPackages,
		JoinBackend:   p.JoinBackend,
		Credentials:   ctx.Creds.Client,
		DebugMode:     p.DebugMode,
		Insecure:      p.Insecure,
	})
}

func (p *Peer) validateWizardState(operator ops.Operator) (*ops.Site, *ops.SiteOperation, error) {
	accounts, err := operator.GetAccounts()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	if len(accounts) == 0 {
		return nil, nil, trace.NotFound("no accounts created yet")
	}
	account := accounts[0]
	clusters, err := operator.GetSites(account.ID)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	if len(clusters) == 0 {
		return nil, nil, trace.NotFound("no sites created yet")
	}
	cluster := clusters[0]

	operation, progress, err := ops.GetInstallOperation(cluster.Key(), operator)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	if progress.IsCompleted() && operation.State == ops.OperationStateCompleted {
		return nil, nil, trace.BadParameter("installation already completed")
	}

	switch operation.State {
	case ops.OperationStateInstallInitiated, ops.OperationStateInstallProvisioning, ops.OperationStateFailed:
		// Consider these states for resuming the installation
		// (including failed that puts the operation into manual mode)
	default:
		return nil, nil, trace.AlreadyExists("operation %#v is in progress",
			operation)
	}
	if len(operation.InstallExpand.Profiles) == 0 {
		return nil, nil,
			trace.ConnectionProblem(nil, "no server profiles selected yet")
	}

	if operation.State == ops.OperationStateFailed {
		// Cannot validate the agents for a failed operation
		// that has been placed into manual mode
		return &cluster, operation, nil
	}

	// FIXME: this is not friends with the option when the node running the installer
	// is not to be part of the cluster
	//
	// // unless installing via UI, do not join until there's at least one other
	// // agent, this way we can make sure that the agent on the installer node
	// // (the one that runs "install") joins first
	// // This is important to avoid the case when the joining agent joins a single
	// // node install by mistake
	// if p.OperationID == "" { // operation ID is given in UI usecase
	// 	report, err := operator.GetSiteInstallOperationAgentReport(operation.Key())
	// 	if err != nil {
	// 		return nil, nil, trace.Wrap(err)
	// 	}
	// 	if len(report.Servers) == 0 {
	// 		return nil, nil, trace.NotFound("no other agents joined yet")
	// 	}
	// }

	return &cluster, operation, nil
}

// FIXME: reconnect forever
func watchReconnects(ctx context.Context, cancel context.CancelFunc, watchCh <-chan rpcserver.WatchEvent, logger log.FieldLogger) {
	for {
		select {
		case event := <-watchCh:
			if event.Error == nil {
				continue
			}
			logger.WithFields(log.Fields{
				log.ErrorKey: event.Error,
				"peer":       event.Peer,
			}).Warn("Failed to reconnect, will abort.")
			cancel()
			return
		case <-ctx.Done():
			return
		}
	}
}

// getPeerAddrAndToken returns the peer address and token for the specified role
func getPeerAddrAndToken(ctx operationContext, role string) (peerAddr, token string, err error) {
	peerAddr = ctx.Peer
	if strings.Contains(peerAddr, "http") { // peer may be an URL
		peerURL, err := url.Parse(ctx.Peer)
		if err != nil {
			return "", "", trace.Wrap(err)
		}
		peerAddr = peerURL.Host
	}
	instructions, ok := ctx.Operation.InstallExpand.Agents[role]
	if !ok {
		return "", "", trace.BadParameter("no agent instructions for role %q: %v",
			role, ctx.Operation.InstallExpand)
	}
	return peerAddr, instructions.Token, nil
}

// formatClusterURL returns cluster API URL from the provided peer addr which
// can be either IP address or a URL (in which case it is returned as-is)
func formatClusterURL(addr string) string {
	if strings.Contains(addr, "http") {
		return addr
	}
	return fmt.Sprintf("https://%v:%v", addr, defaults.GravitySiteNodePort)
}
