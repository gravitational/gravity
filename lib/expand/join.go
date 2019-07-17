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
	"sync"
	"time"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/app/client"
	"github.com/gravitational/gravity/lib/checks"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/httplib"
	"github.com/gravitational/gravity/lib/install"
	"github.com/gravitational/gravity/lib/install/dispatcher"
	"github.com/gravitational/gravity/lib/install/dispatcher/buffered"
	"github.com/gravitational/gravity/lib/install/dispatcher/direct"
	installpb "github.com/gravitational/gravity/lib/install/proto"
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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// NewPeer returns new cluster peer client
func NewPeer(config PeerConfig) (*Peer, error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(utils.NewFailedPreconditionError(err))
	}
	server := server.New()
	dispatcher := buffered.New()
	ctx, cancel := context.WithCancel(context.Background())
	peer := &Peer{
		PeerConfig: config,
		ctx:        ctx,
		cancel:     cancel,
		server:     server,
		dispatcher: dispatcher,
		// Account for server exit, agent exit or agent reconnect failure
		errC:      make(chan error, 4),
		execC:     make(chan *installpb.ExecuteRequest),
		execDoneC: make(chan error, 1),
		closeC:    make(chan closeResponse),
		connectC:  make(chan connectResult, 1),
	}
	peer.startConnectLoop()
	peer.startExecuteLoop()
	peer.startReconnectWatchLoop()
	return peer, nil
}

// Peer is a client that manages joining the cluster
type Peer struct {
	PeerConfig
	// ctx controls the lifetime of interal peer processes
	ctx context.Context
	// cancel cancels internal operation
	cancel context.CancelFunc
	// errC is signaled when either the service or agent is aborted
	errC chan error
	// connectC receives the results of connecting to either wizard
	// or cluster controller
	connectC chan connectResult
	// server is the gRPC installer server
	server     *server.Server
	dispatcher dispatcher.EventDispatcher

	// closeC is a channel to explicitly signal end of operation.
	closeC chan closeResponse

	// execC relays execution requests to the executor loop from outside
	execC chan *installpb.ExecuteRequest
	// execDoneC is signaled by the executor loop to let the client-facing gRPC API
	// know when to stop expecting events and exit
	execDoneC chan error

	// wg is a wait group used to ensure completion of internal processes
	wg sync.WaitGroup
}

// Run runs the peer operation
func (p *Peer) Run(listener net.Listener) error {
	go func() {
		p.errC <- p.server.Run(p, listener)
	}()
	err := <-p.errC
	// Stopping is on best-effort basis, the client will be trying to stop the service
	// if notified
	p.stop()
	return installpb.WrapServiceError(err)
}

// Stop shuts down this RPC agent
// Implements signals.Stopper
func (p *Peer) Stop(ctx context.Context) error {
	p.Info("Stop service.")
	p.server.Stopped(ctx, false)
	return nil
}

// Execute executes the peer operation (join or just serving an agent).
// Implements server.Executor
func (p *Peer) Execute(req *installpb.ExecuteRequest, stream installpb.Agent_ExecuteServer) (err error) {
	p.WithField("req", req).Info("Execute.")
	if req.HasSpecificPhase() {
		// Execute the operation with a new dispatcher
		return p.executeConcurrentStep(req, stream)
	}
	p.submit(req)
	for {
		select {
		case event := <-p.dispatcher.Chan():
			err := stream.Send(event)
			if err != nil {
				return err
			}
		case req := <-p.closeC:
			err := stream.Send(req.resp)
			close(req.doneC)
			if err != nil {
				return err
			}
		case err := <-p.execDoneC:
			if err != nil {
				// Phase finished with an error.
				if installpb.IsRPCError(err) {
					return trace.Unwrap(err)
				}
				// Flag a failed execution (i.e. intermediate) step with codes.Aborted
				// instead of codes.FailedPrecondition as a better suiting error code.
				// See this for reference:
				//
				// FailedPrecondition indicates operation was rejected because the
				// system is not in a state required for the operation's execution.
				// For example, directory to be deleted may be non-empty, an rmdir
				// operation is applied to a non-directory, etc.
				//
				// A litmus test that may help a service implementor in deciding
				// between FailedPrecondition, Aborted, and Unavailable:
				//  (a) Use Unavailable if the client can retry just the failing call.
				//  (b) Use Aborted if the client should retry at a higher-level
				//      (e.g., restarting a read-modify-write sequence).
				//  (c) Use FailedPrecondition if the client should not retry until
				//      the system state has been explicitly fixed. E.g., if an "rmdir"
				//      fails because the directory is non-empty, FailedPrecondition
				//      should be returned since the client should not retry unless
				//      they have first fixed up the directory by deleting files from it.
				//  (d) Use FailedPrecondition if the client performs conditional
				//      REST Get/Update/Delete on a resource and the resource on the
				//      server does not match the condition. E.g., conflicting
				//      read-modify-write on the same resource.
				return status.Error(codes.Aborted, err.Error())
			}
			return nil
		}
	}
	return nil
}

// SetPhase sets phase state without executing it.
func (p *Peer) SetPhase(req *installpb.SetStateRequest) error {
	p.WithField("req", req).Info("Set phase.")
	ctx, err := p.tryConnect(req.OperationKey().OperationID)
	if err != nil {
		return trace.Wrap(err)
	}
	machine, err := p.getFSM(*ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	return machine.ChangePhaseState(p.ctx, fsm.StateChange{
		Phase: req.Phase.ID,
		State: req.State,
	})
}

// Complete manually completes the operation given with opKey.
// Implements server.Executor
func (p *Peer) Complete(opKey ops.SiteOperationKey) error {
	p.WithField("key", opKey).Info("Complete.")
	ctx, err := p.tryConnect(opKey.OperationID)
	if err != nil {
		return trace.Wrap(err)
	}
	machine, err := p.getFSM(*ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(machine.Complete(trace.Errorf("completed manually")))
}

// HandleCompleted completes the operation by dispatching a completion event to the client.
// Implements server.Completer
func (p *Peer) HandleCompleted(ctx context.Context) error {
	p.Info("Completion signaled.")
	if p.sendCloseResponse(installpb.CompleteEvent) {
		p.Info("Client notified about completion.")
	}
	p.errC <- installpb.ErrCompleted
	return nil
}

// HandleAborted completes the operation by dispatching an abort event to the client.
// Implements server.Completer
func (p *Peer) HandleAborted(ctx context.Context) error {
	p.Info("Abort signaled.")
	if p.sendCloseResponse(installpb.AbortEvent) {
		p.Info("Client notified about abort.")
	}
	p.errC <- installpb.ErrAborted
	return nil
}

// HandleStopped shuts down the server
// Implements server.Completer
func (p *Peer) HandleStopped(context.Context) error {
	p.Info("Stop signaled.")
	p.errC <- nil
	return nil
}

// PeerConfig defines the configuration for a peer joining the cluster
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
	// LocalClusterClient is a factory for creating a client to the installed cluster
	LocalClusterClient func() (*opsclient.Client, error)
	// JoinBackend is the local backend where join-specific operation data is stored
	JoinBackend storage.Backend
	// OperationID is the ID of existing join operation created via UI
	OperationID string
	// StateDir defines where peer will store operation-specific data
	StateDir string
}

// CheckAndSetDefaults checks the parameters and autodetects some defaults
func (c *PeerConfig) CheckAndSetDefaults() (err error) {
	if len(c.Peers) == 0 {
		return trace.BadParameter("missing Peers")
	}
	if c.AdvertiseAddr == "" {
		return trace.BadParameter("missing AdvertiseAddr")
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
	if c.LocalClusterClient == nil {
		return trace.BadParameter("missing LocalClusterClient")
	}
	if c.JoinBackend == nil {
		return trace.BadParameter("missing JoinBackend")
	}
	if c.StateDir == "" {
		return trace.BadParameter("missing StateDir")
	}
	if c.FieldLogger == nil {
		c.FieldLogger = log.WithFields(log.Fields{
			trace.Component: "peer",
			"addr":          c.AdvertiseAddr,
		})
	}
	if c.WatchCh == nil {
		c.WatchCh = make(chan rpcserver.WatchEvent, 1)
	}
	return nil
}

func (p *Peer) startConnectLoop() {
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		ctx, err := p.connectLoop()
		if err == nil {
			err = p.init(*ctx)
		}
		if err != nil {
			// Consider failure to connect/init a terminal error.
			// This will prevent the service from automatically restarting.
			// It can be restarted manually though (i.e. after correcting the configuration)
			err = status.Error(codes.FailedPrecondition, trace.UserMessage(err))
		}
		select {
		case p.connectC <- connectResult{
			operationContext: ctx,
			err:              err,
		}:
		case <-p.ctx.Done():
		}
	}()
}

// startExecuteLoop starts a loop that services the channel to handle
// step execute requests.
// The step execution is blocking - i.e. when there's an active operation in flight,
// the subsequent requests are blocked
func (p *Peer) startExecuteLoop() {
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		for {
			select {
			case req := <-p.execC:
				err := p.execute(req)
				select {
				case <-p.execDoneC:
					// Empty the result channel
				default:
				}
				if err != nil {
					p.WithFields(log.Fields{
						log.ErrorKey: err,
						"req":        req,
					}).Warn("Failed to execute.")
				}
				p.execDoneC <- err
				if installpb.IsFailedPreconditionError(err) {
					p.errC <- err
					return
				}
			case <-p.ctx.Done():
				return
			}
		}
	}()
}

// startReconnectWatchLoop starts a loop to watch agent reconnect updates
// and fail if the reconnects fail due to a terminal error.
// Whenever the reconnects fail with a terminal error, this will send the
// corresponding error on p.errC and exit.
// The lifetime is bounded by the peer-internal context
func (p *Peer) startReconnectWatchLoop() {
	p.wg.Add(1)
	go func() {
		watchReconnects(p.ctx, p.errC, p.WatchCh, p.FieldLogger)
		p.wg.Done()
	}()
}

// startProgressLoop starts a new progress watch and dispatch loop.
// The loop exits once the completion progress message has been received.
// The lifetime is bounded by the peer-internal context
func (p *Peer) startProgressLoop(ctx operationContext) {
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		install.ProgressPoller{
			FieldLogger:  p.FieldLogger,
			Operator:     ctx.Operator,
			OperationKey: ctx.Operation.Key(),
			Dispatcher:   p.dispatcher,
		}.Run(p.ctx)
	}()
}

// submit submits the specified request for execution.
// Returns true if the request has actually started an operation
func (p *Peer) submit(req *installpb.ExecuteRequest) bool {
	select {
	case p.execC <- req:
		return true
	default:
		// Another operation is already in flight
		return false
	}
}

// execute executes either the complete operation or a single phase specified with req
func (p *Peer) execute(req *installpb.ExecuteRequest) (err error) {
	p.WithField("req", req).Info("Execute.")
	opCtx, err := p.operationContext()
	if err != nil {
		return trace.Wrap(err)
	}
	if req.Phase != nil {
		return trace.Wrap(p.executePhase(p.ctx, *opCtx, *req.Phase, p.dispatcher))
	}
	return trace.Wrap(p.run(*opCtx))
}

// executeConcurrentStep executes the phase given with req concurrently
// with the running operation.
// The running operation in this case is the install agent service loop
// which is the focus of resume (from the user's point of view),
// while individual phases are required to advance the installer state machine.
// Each concurrent step corresponds to a plan execute command requested
// remotely by the installer process
func (p *Peer) executeConcurrentStep(req *installpb.ExecuteRequest, stream installpb.Agent_ExecuteServer) error {
	p.WithField("req", req).Info("Executing phase concurrently.")
	opCtx, err := p.operationContext()
	if err != nil {
		return trace.Wrap(err)
	}
	dispatcher := direct.New()
	defer dispatcher.Close()
	errC := make(chan error, 1)
	go func() {
		errC <- p.executePhase(stream.Context(), *opCtx, *req.Phase, dispatcher)
	}()
	for {
		select {
		case event := <-dispatcher.Chan():
			err := stream.Send(event)
			if err != nil {
				return trace.Wrap(err)
			}
		case err := <-errC:
			return trace.Wrap(err)
		}
	}
	return nil
}

func (p *Peer) executePhase(ctx context.Context, opCtx operationContext, phase installpb.Phase, disp dispatcher.EventDispatcher) error {
	if phase.IsResume() && !opCtx.isExpand() {
		return trace.Wrap(p.run(opCtx))
	}
	machine, err := p.getFSM(opCtx)
	if err != nil {
		return trace.Wrap(err)
	}
	progressReporter := dispatcher.NewProgressReporter(ctx, disp, phaseTitle(phase))
	defer progressReporter.Stop()
	if phase.IsResume() {
		return trace.Wrap(install.ExecuteOperation(ctx, machine,
			progressReporter, p.FieldLogger))
	}
	params := fsm.Params{
		PhaseID:  phase.ID,
		Force:    phase.Force,
		Progress: progressReporter,
	}
	if phase.Rollback {
		return trace.Wrap(machine.RollbackPhase(ctx, params))
	}
	return trace.Wrap(machine.ExecutePhase(ctx, params))
}

// printStep publishes a progress entry described with (format, args) tuple to the client
func (p *Peer) printStep(format string, args ...interface{}) {
	event := dispatcher.Event{Progress: &ops.ProgressEntry{Message: fmt.Sprintf(format, args...)}}
	p.dispatcher.Send(event)
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
func (p *Peer) getExpandOperation(operator ops.Operator, cluster ops.Site, operationID string) (*ops.SiteOperation, error) {
	operation, err := operator.GetSiteOperation(ops.SiteOperationKey{
		AccountID:   cluster.AccountID,
		SiteDomain:  cluster.Domain,
		OperationID: operationID,
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
	entry, err := env.LoginWizard(addr, p.Token)
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
	creds, err := install.LoadRPCCredentials(p.ctx, env.Packages)
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

func (p *Peer) dialCluster(addr, operationID string) (*operationContext, error) {
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
	if operationID == "" {
		operation, err = p.createExpandOperation(operator, *cluster)
	} else {
		operation, err = p.getExpandOperation(operator, *cluster, operationID)
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}
	creds, err := install.LoadRPCCredentials(p.ctx, packages)
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

// connectLoop dials to either a running wizard OpsCenter or a local gravity cluster.
// For wizard, if the dial succeeds, it will join the active installation and return
// an operation context of the active install operation.
//
// For a local gravity cluster, it will attempt to start the expand operation
// and will return an operation context wrapping a new expand operation.
func (p *Peer) connectLoop() (*operationContext, error) {
	ticker := backoff.NewTicker(leader.NewUnlimitedExponentialBackOff())
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			ctx, err := p.tryConnect(p.OperationID)
			if err != nil {
				// join token is incorrect, fail immediately and report to user
				if trace.IsAccessDenied(err) {
					return nil, trace.AccessDenied("bad secret token")
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

func (p *Peer) stop() error {
	p.Info("Stopping peer.")
	ctx, cancel := context.WithTimeout(context.Background(), defaults.ShutdownTimeout)
	defer cancel()
	p.cancel()
	p.wg.Wait()
	p.dispatcher.Close()
	p.server.Stop(ctx)
	return nil
}

func (p *Peer) tryConnect(operationID string) (op *operationContext, err error) {
	p.printStep("Connecting to cluster")
	for _, addr := range p.Peers {
		p.WithField("peer", addr).Debug("Dialing peer.")
		op, err = p.dialWizard(addr)
		if err == nil {
			p.WithField("addr", op.Peer).Debug("Connected to wizard.")
			p.printStep("Connected to installer at %v", addr)
			return op, nil
		}
		p.WithError(err).Info("Failed connecting to wizard.")
		if isTerminalError(err) {
			return nil, trace.Wrap(err)
		}
		// already exists error is returned when there's an ongoing install
		// operation, do not attempt to dial the cluster until it completes
		if trace.IsAlreadyExists(err) {
			p.printStep("Waiting for the install operation to finish")
			return nil, trace.Wrap(err)
		}
		op, err = p.dialCluster(addr, operationID)
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

// startAgent starts a new RPC agent using the specified operation context.
// The agent will signal p.errC once it has terminated
func (p *Peer) startAgent(ctx operationContext) error {
	agent, err := p.newAgent(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	go func() {
		p.errC <- agent.Serve()
	}()
	return nil
}

// newAgent returns an instance of the RPC agent to handle remote calls
func (p *Peer) newAgent(opCtx operationContext) (*rpcserver.PeerServer, error) {
	peerAddr, token, err := getPeerAddrAndToken(opCtx, p.Role)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	p.RuntimeConfig.Token = token
	agent, err := install.NewAgent(install.AgentConfig{
		FieldLogger: p.WithFields(log.Fields{
			trace.Component: "rpc:peer",
			"addr":          p.AdvertiseAddr,
		}),
		AdvertiseAddr: p.AdvertiseAddr,
		CloudProvider: p.CloudProvider,
		ServerAddr:    peerAddr,
		Credentials:   opCtx.Creds,
		RuntimeConfig: p.RuntimeConfig,
		WatchCh:       p.WatchCh,
		StopHandler:   p.server.Stopped,
		AbortHandler:  p.server.Interrupted,
	})
	if err != nil {

		return nil, trace.Wrap(err)
	}
	return agent, nil
}

func (p *Peer) run(ctx operationContext) error {
	err := p.checkAndSetServerProfile(ctx.Cluster.App)
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

	p.startProgressLoop(ctx)
	if ctx.Operation.Type != ops.OperationExpand {
		<-p.ctx.Done()
		return nil
	}

	return trace.Wrap(p.executeExpandOperation(ctx))
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
	log.Debug("Waiting for agent to join.")
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
			log.WithField("report", report).Info("Installation can proceed.")
			return nil
		case <-p.ctx.Done():
			return trace.Wrap(p.ctx.Err())
		}
	}
}

func (p *Peer) executeExpandOperation(ctx operationContext) error {
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
	fsm, err := p.getFSM(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	progress := dispatcher.NewProgressReporter(p.ctx, p.dispatcher, "Executing expand operation")
	defer progress.Stop()
	fsmErr := fsm.ExecutePlan(p.ctx, progress)
	if fsmErr != nil {
		p.WithError(fsmErr).Warn("Failed to execute plan.")
	}
	err = fsm.Complete(fsmErr)
	if err != nil {
		return trace.Wrap(err, "failed to complete operation")
	}
	p.sendCompletionEvent()
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

func (p *Peer) operationContext() (*operationContext, error) {
	select {
	case result := <-p.connectC:
		p.connectC <- result
		if result.err != nil {
			return nil, result.err
		}
		return result.operationContext, nil
	case <-p.ctx.Done():
		return nil, trace.Wrap(p.ctx.Err())
	}
}

func (p *Peer) getFSM(ctx operationContext) (*fsm.FSM, error) {
	if ctx.Operation.Type == ops.OperationInstall {
		return p.newInstallFSM(ctx)
	}
	return p.newJoinFSM(ctx)
}

func (p *Peer) newInstallFSM(ctx operationContext) (*fsm.FSM, error) {
	return install.NewFSM(install.FSMConfig{
		Operator:           ctx.Operator,
		OperationKey:       ctx.Operation.Key(),
		Packages:           ctx.Packages,
		Apps:               ctx.Apps,
		LocalClusterClient: p.LocalClusterClient,
		LocalBackend:       p.LocalBackend,
		LocalApps:          p.LocalApps,
		LocalPackages:      p.LocalPackages,
		Insecure:           p.Insecure,
	})
}

func (p *Peer) newJoinFSM(ctx operationContext) (*fsm.FSM, error) {
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
	case ops.OperationStateInstallInitiated,
		ops.OperationStateInstallProvisioning,
		ops.OperationStateInstallPrechecks,
		ops.OperationStateFailed:
		// Consider these states for resuming the installation
		// (including failed that puts the operation into manual mode)
	default:
		return nil, nil, trace.AlreadyExists("operation %v is in progress",
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
	return &cluster, operation, nil
}

func (p *Peer) sendCompletionEvent() {
	event := dispatcher.Event{
		Progress: &ops.ProgressEntry{
			Message:    "Operation completed",
			Completion: constants.Completed,
		},
		// Set the completion status
		Status: dispatcher.StatusCompleted,
	}
	p.dispatcher.Send(event)
}

func (p *Peer) sendError(err error) {
	p.dispatcher.Send(dispatcher.Event{
		Error: err,
	})
}

func (p *Peer) sendCloseResponse(resp *installpb.ProgressResponse) bool {
	doneC := make(chan struct{})
	select {
	case p.closeC <- closeResponse{doneC: doneC, resp: resp}:
		// If receiver is available, wait for completion
		<-doneC
		return true
	default:
		// Do not block if otherwise
		return false
	}
}

func (r operationContext) isExpand() bool {
	return r.Operation.Type == ops.OperationExpand
}

func watchReconnects(ctx context.Context, errC chan<- error, watchCh <-chan rpcserver.WatchEvent, logger log.FieldLogger) {
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
			errC <- event.Error
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

func isTerminalError(err error) bool {
	return utils.IsAbortError(err) || trace.IsAccessDenied(err)
}

func phaseTitle(phase installpb.Phase) string {
	if phase.IsResume() {
		return "Resuming operation"
	}
	return fmt.Sprintf("Executing phase %v", phase.ID)
}

type connectResult struct {
	*operationContext
	err error
}

type closeResponse struct {
	doneC chan struct{}
	resp  *installpb.ProgressResponse
}
