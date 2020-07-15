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
	"github.com/gravitational/gravity/lib/pack/localpack"
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
		// Account for agent exit or agent reconnect failure
		errC:        make(chan error, 3),
		exitC:       make(chan error, 1),
		execC:       make(chan *installpb.ExecuteRequest),
		execDoneC:   make(chan install.ExecResult, 1),
		closeC:      make(chan closeResponse),
		connectC:    make(chan connectResult, 1),
		connectingC: make(chan struct{}),
	}
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
	// errC is signaled when agent aborts or exits
	errC chan error
	// exitC is signaled when the service exits
	exitC chan error
	// server is the gRPC installer server
	server     *server.Server
	dispatcher dispatcher.EventDispatcher
	// closeC is a channel to explicitly signal end of operation.
	closeC chan closeResponse
	// execC relays execution requests to the executor loop from outside
	execC chan *installpb.ExecuteRequest
	// execDoneC is signaled by the executor loop to let the client-facing gRPC API
	// know when to stop expecting events and exit
	execDoneC chan install.ExecResult
	// wg is a wait group used to ensure completion of internal processes
	wg sync.WaitGroup
	// connectC receives the results of connecting to either wizard
	// or cluster controller
	connectC chan connectResult
	// connectingC is closed once the connect loop starts running
	connectingC chan struct{}
	// connectOnce enables the execute loop to start the connect loop
	// only on the first execute request
	connectOnce sync.Once
}

// Run runs the peer operation
func (p *Peer) Run(listener net.Listener) error {
	errC := make(chan error, 1)
	go func() {
		errC <- p.server.Run(p, listener)
	}()
	var err error
	select {
	case err = <-errC:
	case err = <-p.exitC:
	}
	if err != nil {
		p.sendClientErrorResponse(err)
	}
	// Stopping is on best-effort basis, the client will be trying to stop the service
	// if notified
	p.WithField("exit-error", err).Info("Exit with error.")
	p.stop()
	if err != nil {
		if installpb.IsAbortedError(err) {
			if err := p.leave(); err != nil {
				p.WithError(err).Warn("Failed to leave cluster.")
			}
		} else if !installpb.IsCompletedError(err) {
			if err := p.fail(err.Error()); err != nil {
				p.WithError(err).Warn("Failed to mark operation as failed.")
			}
		}
	}
	return installpb.WrapServiceError(err)
}

// Stop shuts down this RPC agent
// Implements signals.Stopper
func (p *Peer) Stop(ctx context.Context) error {
	p.Info("Peer Stop.")
	return p.server.ManualStop(ctx, false)
}

// Execute executes the peer operation (join or just serving an agent).
// Implements server.Executor
func (p *Peer) Execute(req *installpb.ExecuteRequest, stream installpb.Agent_ExecuteServer) (err error) {
	p.WithField("req", req).Info("Execute.")
	if req.HasSpecificPhase() {
		// Execute the operation step with a new dispatcher
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
		case result := <-p.execDoneC:
			if result.Error != nil {
				// Phase finished with an error.
				// See https://github.com/grpc/grpc-go/blob/v1.22.0/codes/codes.go#L78
				return status.Error(codes.Aborted, install.FormatAbortError(result.Error))
			}
			if result.CompletionEvent != nil {
				err := stream.Send(result.CompletionEvent.AsProgressResponse())
				if err != nil {
					return err
				}
			}
			return nil
		}
	}
}

// SetPhase sets phase state without executing it.
func (p *Peer) SetPhase(req *installpb.SetStateRequest) error {
	p.WithField("req", req).Info("Set phase.")
	ctx, err := p.tryConnect(req.OperationID())
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
func (p *Peer) Complete(ctx context.Context, opKey ops.SiteOperationKey) error {
	p.WithField("key", opKey).Info("Complete.")
	opCtx, err := p.tryConnect(opKey.OperationID)
	if err != nil {
		return trace.Wrap(err)
	}
	machine, err := p.getFSM(*opCtx)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(machine.Complete(ctx, trace.Errorf("completed manually")))
}

// HandleCompleted completes the operation by dispatching a completion event to the client.
// Implements server.Completer
func (p *Peer) HandleCompleted(ctx context.Context) error {
	p.Debug("Completion signaled.")
	if p.sendClientCloseResponse(installpb.CompleteEvent) {
		p.Debug("Client notified about completion.")
	}
	p.exitWithError(installpb.ErrCompleted)
	return nil
}

// HandleAborted completes the operation by dispatching an abort event to the client.
// Implements server.Completer
func (p *Peer) HandleAborted(ctx context.Context) error {
	p.Debug("Abort signaled.")
	if p.sendClientCloseResponse(installpb.AbortEvent) {
		p.Debug("Client notified about abort.")
	}
	p.exitWithError(installpb.ErrAborted)
	return nil
}

// HandleStopped shuts down the server
// Implements server.Completer
func (p *Peer) HandleStopped(context.Context) error {
	p.Debug("Stop signaled.")
	p.exitWithError(context.Canceled)
	return nil
}

// PeerConfig defines the configuration for a peer joining the cluster
type PeerConfig struct {
	// Peers is a list of peer addresses
	Peers []string
	// AdvertiseAddr is advertise address of this node
	AdvertiseAddr string
	// ServerAddr is optional address of the agent server.
	// It will be derived from agent instructions if unspecified
	ServerAddr string
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
	LocalPackages *localpack.PackageServer
	// LocalClusterClient is a factory for creating a client to the installed cluster
	LocalClusterClient func(...httplib.ClientOption) (*opsclient.Client, error)
	// JoinBackend is the local backend where join-specific operation data is stored
	JoinBackend storage.Backend
	// OperationID is the ID of existing join operation created via UI
	OperationID string
	// StateDir defines where peer will store operation-specific data
	StateDir string
	// SkipWizard specifies to the peer agents that the peer is not a wizard
	// and attempts to contact the wizard should be skipped
	SkipWizard bool
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
	p.connectOnce.Do(func() {
		close(p.connectingC)
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			ctx, err := p.connectLoop()
			if err == nil {
				ctx.agent, err = p.init(*ctx)
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
	})
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
				status, err := p.execute(req)
				select {
				case <-p.execDoneC:
					// Empty the result channel
				default:
				}
				if err != nil {
					p.WithFields(log.Fields{
						log.ErrorKey: err,
						"status":     status,
						"req":        req,
					}).Warn("Failed to execute.")
					p.execDoneC <- install.ExecResult{Error: err}
					if installpb.IsFailedPreconditionError(err) {
						p.exitWithError(err)
						return
					}
				} else {
					var result install.ExecResult
					if status.IsCompleted() {
						result.CompletionEvent = p.newCompletionEvent()
					}
					p.execDoneC <- result
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
		err := install.ProgressPoller{
			FieldLogger:  p.FieldLogger,
			Operator:     ctx.Operator,
			OperationKey: ctx.Operation.Key(),
			Dispatcher:   p.dispatcher,
		}.Run(p.ctx)
		if err != nil {
			p.Warnf("Failed to stop progress poller: %v.", err)
		}
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
func (p *Peer) execute(req *installpb.ExecuteRequest) (dispatcher.Status, error) {
	p.WithField("req", req).Info("Execute.")
	p.startConnectLoop()
	opCtx, err := p.operationContext(p.ctx)
	if err != nil {
		return dispatcher.StatusUnknown, trace.Wrap(err)
	}
	if req.Phase != nil {
		return p.executePhase(p.ctx, *opCtx, *req.Phase, p.dispatcher)
	}
	return p.run(*opCtx)
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
	opCtx, err := p.operationContext(stream.Context())
	if err != nil {
		return trace.Wrap(err)
	}
	dispatcher := direct.New()
	defer dispatcher.Close()
	errC := make(chan error, 1)
	go func() {
		_, err := p.executePhase(stream.Context(), *opCtx, *req.Phase, dispatcher)
		errC <- err
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
}

func (p *Peer) executePhase(ctx context.Context, opCtx operationContext, phase installpb.Phase, disp dispatcher.EventDispatcher) (dispatcher.Status, error) {
	if phase.IsResume() && !opCtx.isExpand() {
		return p.agentLoop(opCtx)
	}
	if phase.IsResume() {
		err := p.resumeExpandOperation(ctx, opCtx, phase, disp)
		if err != nil {
			return dispatcher.StatusUnknown, trace.Wrap(err)
		}
		return dispatcher.StatusCompleted, nil
	}
	err := p.executeSinglePhase(ctx, opCtx, phase, disp)
	return dispatcher.StatusUnknown, trace.Wrap(err)
}

func (p *Peer) resumeExpandOperation(ctx context.Context, opCtx operationContext, phase installpb.Phase, disp dispatcher.EventDispatcher) error {
	progressReporter := dispatcher.NewProgressReporter(ctx, disp, phaseTitle(phase))
	defer progressReporter.Stop()
	machine, err := p.getFSM(opCtx)
	if err != nil {
		return trace.Wrap(err)
	}
	err = p.ensureExpandOperationState(opCtx)
	if err != nil {
		return trace.Wrap(err)
	}
	return install.ExecuteOperation(ctx, machine, progressReporter, p.FieldLogger)
}

func (p *Peer) executeSinglePhase(ctx context.Context, opCtx operationContext, phase installpb.Phase, disp dispatcher.EventDispatcher) error {
	machine, err := p.getFSM(opCtx)
	if err != nil {
		return trace.Wrap(err)
	}
	progressReporter := dispatcher.NewProgressReporter(ctx, disp, phaseTitle(phase))
	defer progressReporter.Stop()
	params := fsm.Params{
		PhaseID:  phase.ID,
		Force:    phase.Force,
		Progress: progressReporter,
	}
	if phase.Rollback {
		return machine.RollbackPhase(ctx, params)
	}
	return machine.ExecutePhase(ctx, params)
}

// printStep publishes a progress entry described with (format, args) tuple to the client
func (p *Peer) printStep(format string, args ...interface{}) {
	event := dispatcher.Event{Progress: &ops.ProgressEntry{Message: fmt.Sprintf(format, args...)}}
	p.dispatcher.Send(event)
}

// dialWizard connects to the wizard process with the specified address
func (p *Peer) dialWizard(addr string) (*operationContext, error) {
	env, err := localenv.NewRemoteEnvironment()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	entry, err := env.LoginWizard(addr, p.Token)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	peerURL, err := url.Parse(entry.OpsCenterURL)
	if err != nil {
		return nil, utils.Abort(err)
	}
	cluster, operation, err := p.validateWizardState(env.Operator)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = p.checkAndSetServerProfile(cluster.App)
	if err != nil {
		return nil, utils.Abort(err)
	}
	creds, err := install.LoadRPCCredentials(p.ctx, env.Packages)
	if err != nil {
		return nil, utils.Abort(err)
	}
	ctx := operationContext{
		Operator:  env.Operator,
		Packages:  env.Packages,
		Apps:      env.Apps,
		Peer:      peerURL.Host,
		Operation: *operation,
		Cluster:   *cluster,
		Creds:     *creds,
	}
	if shouldRunLocalChecks(ctx) {
		err = p.runLocalChecks(ctx)
		if err != nil {
			return nil, utils.Abort(err)
		}
	}
	return &ctx, nil
}

// dialCluster connects to the cluster controller with the specified address.
// operationID specifies optional existing expand operation ID
func (p *Peer) dialCluster(addr, operationID string) (*operationContext, error) {
	ctx, err := p.connectCluster(addr)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = p.checkAndSetServerProfile(ctx.Cluster.App)
	if err != nil {
		return nil, utils.Abort(err)
	}
	if ctx.hasOperation() {
		if shouldRunLocalChecks(*ctx) {
			err = p.runLocalChecks(*ctx)
			if err != nil {
				return nil, utils.Abort(err)
			}
		}
		return ctx, nil
	}
	operation, err := p.getOrCreateExpandOperation(ctx.Operator, ctx.Cluster, operationID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ctx.Operation = *operation
	if shouldRunLocalChecks(*ctx) {
		err = p.runLocalChecks(*ctx)
		if err != nil {
			return nil, utils.Abort(err)
		}
	}
	return ctx, nil
}

func (p *Peer) connectCluster(addr string) (*operationContext, error) {
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
	creds, err := install.LoadRPCCredentials(p.ctx, packages)
	if err != nil {
		return nil, utils.Abort(err)
	}
	peerURL, err := url.Parse(targetURL)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &operationContext{
		Operator: operator,
		Packages: packages,
		Apps:     apps,
		Peer:     peerURL.Host,
		Cluster:  *cluster,
		Creds:    *creds,
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

func (p *Peer) getOrCreateExpandOperation(operator ops.Operator, cluster ops.Site, operationID string) (*ops.SiteOperation, error) {
	if operationID != "" {
		operation, err := p.getExpandOperation(operator, cluster, operationID)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return operation, nil
	}
	operation, err := ops.GetExpandOperation(p.JoinBackend)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	if err == nil {
		return operation, nil
	}
	operation, err = p.createExpandOperation(operator, cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = p.syncOperation(operator, cluster, operation.Key())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return operation, nil
}

func (p *Peer) runLocalChecks(ctx operationContext) error {
	installOperation, _, err := ops.GetInstallOperation(ctx.Cluster.Key(), ctx.Operator)
	if err != nil {
		return trace.Wrap(err)
	}
	return checks.RunLocalChecks(p.ctx, checks.LocalChecksRequest{
		Manifest: ctx.Cluster.App.Manifest,
		Role:     p.Role,
		Docker:   ctx.Cluster.ClusterState.Docker,
		Options: &validationpb.ValidateOptions{
			VxlanPort: int32(installOperation.GetVars().OnPrem.VxlanPort),
			DnsAddrs:  ctx.Cluster.DNSConfig.Addrs,
			DnsPort:   int32(ctx.Cluster.DNSConfig.Port),
		},
		AutoFix: true,
	})
}

func (r operationContext) hasOperation() bool {
	return r.Operation.Type != ""
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
	// agent specifies the agent instance active during the operation.
	agent *rpcserver.PeerServer
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
					log.WithError(err).Warn("Access denied during connect.")
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

func (p *Peer) stop() {
	ctx, cancel := context.WithTimeout(context.Background(), defaults.ShutdownTimeout)
	defer cancel()
	if err := p.shutdownAgent(ctx); err != nil {
		p.WithError(err).Warn("Failed to shut down agent.")
	}
	p.cancel()
	p.wg.Wait()
	p.dispatcher.Close()
	p.server.Stop(ctx)
}

func (p *Peer) shutdownAgent(ctx context.Context) error {
	opCtx, err := p.maybeOperationContext(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	if opCtx == nil || opCtx.agent == nil {
		return nil
	}
	p.Info("Stop peer agent.")
	err = opCtx.agent.Stop(ctx)
	<-opCtx.agent.Done()
	return trace.Wrap(err)
}

func (p *Peer) tryConnect(operationID string) (ctx *operationContext, err error) {
	p.printStep("Connecting to cluster")
	for _, addr := range p.Peers {
		p.WithField("peer", addr).Debug("Dialing peer.")
		if !p.SkipWizard {
			ctx, err = p.dialWizard(addr)
			if err == nil {
				p.WithField("addr", ctx.Peer).Debug("Connected to wizard.")
				p.printStep("Connected to installer at %v", addr)
				return ctx, nil
			}
			if !utils.IsConnectionResetError(err) {
				p.WithError(err).Warn("Failed connecting to wizard.")
			}
			if isTerminalError(err) {
				return nil, utils.Abort(err)
			}
			// already exists error is returned when there's an ongoing install
			// operation, do not attempt to dial the cluster until it completes
			if trace.IsAlreadyExists(err) {
				p.printStep("Waiting for the install operation to finish")
				return nil, trace.Wrap(err)
			}
		}
		ctx, err = p.dialCluster(addr, operationID)
		if err == nil {
			p.WithField("addr", ctx.Peer).Debug("Connected to cluster.")
			p.printStep("Connected to existing cluster at %v", addr)
			return ctx, nil
		}
		p.WithError(err).Warn("Failed connecting to cluster.")
		if isTerminalError(err) {
			return nil, utils.Abort(err)
		}
		if trace.IsCompareFailed(err) {
			p.Warnf("Waiting for precondition to create expand operation: %v.", err)
			if utils.IsClusterDegradedError(err) {
				p.printStep("Cluster is degraded, waiting for it to become healthy")
			} else {
				p.printStep("Waiting for another operation to complete at %v", addr)
			}
		}
	}
	return ctx, trace.Wrap(err)
}

func (p *Peer) run(ctx operationContext) (dispatcher.Status, error) {
	if ctx.Operation.Type == ops.OperationInstall {
		return p.agentLoop(ctx)
	}
	err := p.executeExpandOperation(ctx)
	if err != nil {
		return dispatcher.StatusUnknown, trace.Wrap(err)
	}
	return dispatcher.StatusCompleted, nil
}

func (p *Peer) agentLoop(ctx operationContext) (dispatcher.Status, error) {
	p.startProgressLoop(ctx)
	<-p.ctx.Done()
	return dispatcher.StatusUnknown, nil
}

func (p *Peer) executeExpandOperation(ctx operationContext) error {
	err := p.ensureExpandOperationState(ctx)
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
	err = fsm.Complete(p.ctx, fsmErr)
	if err != nil {
		return trace.Wrap(err, "failed to complete operation")
	}
	return trace.Wrap(fsmErr)
}

func (p *Peer) ensureExpandOperationState(ctx operationContext) error {
	err := p.waitForOperation(ctx.Operator, ctx.Operation)
	if err != nil {
		return trace.Wrap(err)
	}
	err = p.waitForAgents(ctx.Operator, ctx.Operation)
	if err != nil {
		return trace.Wrap(err)
	}
	err = p.initOperationPlan(ctx)
	if err != nil && !trace.IsAlreadyExists(err) {
		return trace.Wrap(err)
	}
	err = p.syncOperationPlan(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
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
	operation, err := operator.GetSiteOperation(*key)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = operator.SetOperationState(p.ctx, *key, ops.SetOperationStateRequest{
		State: ops.OperationStateReady,
	})
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

// waitForOperation blocks until the join operation is ready
func (p *Peer) waitForOperation(operator ops.Operator, operation ops.SiteOperation) error {
	ticker := backoff.NewTicker(backoff.NewConstantBackOff(1 * time.Second))
	defer ticker.Stop()
	log := p.WithField(constants.FieldOperationID, operation.ID)
	log.Debug("Waiting for the operation to become ready.")
	for {
		select {
		case <-ticker.C:
			operation, err := operator.GetSiteOperation(operation.Key())
			if err != nil {
				return trace.Wrap(err)
			}
			logger := log.WithField("state", operation.State)
			ready, err := isExpandOperationReady(operation.State)
			if err == nil && !ready {
				logger.Info("Operation is not ready yet.")
				continue
			}
			log.Debug("Operation is ready.")
			return nil
		case <-p.ctx.Done():
			return trace.Wrap(p.ctx.Err())
		}
	}
}

func (p *Peer) leave() error {
	p.Info("Leave cluster.")
	ctx, cancel := context.WithTimeout(context.Background(), defaults.NodeLeaveTimeout)
	defer cancel()
	if err := p.failOperation(ctx, "aborted"); err != nil {
		p.WithError(err).Warn("Failed to mark the operation as failed.")
	}
	return p.createShrinkOperation(ctx)
}

func (p *Peer) fail(message string) error {
	p.Debug("Mark operation as failed.")
	ctx, cancel := context.WithTimeout(context.Background(), defaults.NodeLeaveTimeout)
	defer cancel()
	return p.failOperation(ctx, message)
}

func (p *Peer) failOperation(ctx context.Context, message string) error {
	opCtx, err := p.maybeOperationContext(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	if opCtx == nil {
		// No operation to fail
		return nil
	}
	return ops.FailOperation(ctx, opCtx.Operation.Key(), opCtx.Operator, message)
}

func (p *Peer) createShrinkOperation(ctx context.Context) error {
	opCtx, err := p.operationContext(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	// Refresh operation from cluster
	operation, err := opCtx.Operator.GetSiteOperation(opCtx.Operation.Key())
	if err != nil {
		return trace.Wrap(err)
	}
	if len(operation.Servers) == 0 {
		p.WithField("operation", operation.String()).Warn("Failed to create shrink operation - no servers in state.")
		return nil
	}
	_, err = opCtx.Operator.CreateSiteShrinkOperation(ctx,
		ops.CreateSiteShrinkOperationRequest{
			AccountID:  opCtx.Cluster.AccountID,
			SiteDomain: opCtx.Cluster.Domain,
			Servers:    []string{operation.Servers[0].Hostname},
			Force:      true,
			// Have cluster avoid triggering remote node cleanup
			NodeRemoved: true,
		},
	)
	return trace.Wrap(err)
}

func (p *Peer) waitForAgents(operator ops.Operator, operation ops.SiteOperation) error {
	ticker := backoff.NewTicker(&backoff.ExponentialBackOff{
		InitialInterval: time.Second,
		Multiplier:      1.5,
		MaxInterval:     10 * time.Second,
		MaxElapsedTime:  5 * time.Minute,
		Clock:           backoff.SystemClock,
	})
	defer ticker.Stop()
	log := p.WithField(constants.FieldOperationID, operation.ID)
	log.Debug("Waiting for agent to join.")
	for {
		select {
		case tm := <-ticker.C:
			if tm.IsZero() {
				return trace.ConnectionProblem(nil, "timed out waiting for agents to join")
			}
			report, err := operator.GetSiteExpandOperationAgentReport(p.ctx, operation.Key())
			if err != nil {
				log.WithError(err).Warn("Failed to query agent report.")
				continue
			}
			if len(report.Servers) == 0 {
				log.Debug("Agent hasn't joined yet.")
				continue
			}
			op, err := operator.GetSiteOperation(operation.Key())
			if err != nil {
				log.WithError(err).Warn("Failed to query operation.")
				continue
			}
			if shouldUpdateExpandOperationState(op.State) {
				req, err := install.GetServerUpdateRequest(*op, report.Servers)
				if err != nil {
					log.WithError(err).Warn("Failed to create server update request.")
					continue
				}
				err = operator.UpdateExpandOperationState(operation.Key(), *req)
				if err != nil {
					return trace.Wrap(err)
				}
			}
			log.WithField("report", report).Debug("Installation can proceed.")
			return nil
		case <-p.ctx.Done():
			return trace.Wrap(p.ctx.Err())
		}
	}
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

func (p *Peer) maybeOperationContext(ctx context.Context) (*operationContext, error) {
	select {
	case <-p.connectingC:
		return p.operationContext(ctx)
	default:
		return nil, nil
	}
}

func (p *Peer) operationContext(ctx context.Context) (*operationContext, error) {
	select {
	case result := <-p.connectC:
		p.connectC <- result
		if result.err != nil {
			return nil, result.err
		}
		return result.operationContext, nil
	case <-ctx.Done():
		return nil, trace.Wrap(ctx.Err())
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
		ops.OperationStateInstallDeploying,
		ops.OperationStateFailed:
		// Consider these states for resuming the installation
		// (including failed that puts the operation into manual mode).
		// Be careful about including unrelated peers though
		if len(operation.Servers) == 0 || peerPartOfInstallState(p.AdvertiseAddr, operation.Servers) {
			break
		}
		fallthrough
	default:
		return nil, nil, trace.AlreadyExists("operation %v is in progress",
			operation)
	}

	if len(operation.InstallExpand.Profiles) == 0 {
		return nil, nil,
			trace.ConnectionProblem(nil, "no server profiles selected yet")
	}

	return &cluster, operation, nil
}

func (p *Peer) newCompletionEvent() *dispatcher.Event {
	return &dispatcher.Event{
		Progress: &ops.ProgressEntry{
			Message:    "Operation completed",
			Completion: constants.Completed,
		},
		// Set the completion status
		Status: dispatcher.StatusCompleted,
	}
}

func (p *Peer) sendClientErrorResponse(err error) bool {
	message := err.Error()
	s, ok := status.FromError(trace.Unwrap(err))
	if ok {
		message = s.Message()
	}
	return p.sendClientCloseResponse(&installpb.ProgressResponse{
		Error: &installpb.Error{Message: message},
	})
}

func (p *Peer) sendClientCloseResponse(resp *installpb.ProgressResponse) bool {
	doneC := make(chan struct{})
	select {
	case p.closeC <- closeResponse{doneC: doneC, resp: resp}:
		// If receiver is available, wait for completion
		<-doneC
		return true
	default:
		// Do not block otherwise
		return false
	}
}

func (p *Peer) exitWithError(err error) {
	select {
	case p.exitC <- err:
	case <-p.ctx.Done():
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

// formatClusterURL returns cluster API URL from the provided peer addr which
// can be either IP address or a URL (in which case it is returned as-is)
func formatClusterURL(addr string) string {
	if strings.HasPrefix(addr, "http") {
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

func shouldRunLocalChecks(ctx operationContext) bool {
	if !ctx.hasOperation() {
		return true
	}
	switch ctx.Operation.State {
	case ops.OperationStateExpandInitiated,
		ops.OperationStateExpandProvisioning,
		ops.OperationStateInstallInitiated,
		ops.OperationStateInstallProvisioning,
		ops.OperationStateReady:
		// Keep this in sync with opsservice#updateOperationState
		return true
	default:
		return false
	}
}

func shouldUpdateExpandOperationState(state string) bool {
	switch state {
	case ops.OperationStateExpandInitiated,
		ops.OperationStateExpandProvisioning,
		ops.OperationStateReady:
		// Keep this in sync with opsservice#updateOperationState
		return true
	default:
		return false
	}
}

func isExpandOperationReady(state string) (bool, error) {
	switch state {
	case ops.OperationStateReady, ops.OperationStateExpandPrechecks:
		return true, nil
	case ops.OperationStateExpandInitiated,
		ops.OperationStateExpandProvisioning:
		return false, nil
	default:
		return false, trace.BadParameter("unexpected operation state: %q", state)
	}
}

func peerPartOfInstallState(addr string, servers storage.Servers) bool {
	return servers.FindByIP(addr) != nil
}

type connectResult struct {
	*operationContext
	err error
}

type closeResponse struct {
	doneC chan struct{}
	resp  *installpb.ProgressResponse
}
