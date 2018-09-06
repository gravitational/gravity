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
	"strings"
	"time"

	"github.com/gravitational/gravity/lib/checks"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/httplib"
	"github.com/gravitational/gravity/lib/localenv"
	validationpb "github.com/gravitational/gravity/lib/network/validation/proto"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/ops/opsclient"
	"github.com/gravitational/gravity/lib/pack/webpack"
	pb "github.com/gravitational/gravity/lib/rpc/proto"
	rpcserver "github.com/gravitational/gravity/lib/rpc/server"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/cenkalti/backoff"
	"github.com/gravitational/coordinate/leader"
	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// PeerConfig is for peers joining the cluster
type PeerConfig struct {
	// Peers is a list of peer addresses
	Peers []string
	// Context controls state of the peer, e.g. it can be cancelled
	Context context.Context
	// AdvertiseAddr is advertise addr of this node
	AdvertiseAddr string
	// ServerAddr is optional address of the agent server.
	// It will be derived from agent instructions if unspecified
	ServerAddr string
	// EventsC is channel with events indicating install progress
	EventsC chan Event
	// WatchCh is channel that relays peer reconnect events
	WatchCh chan rpcserver.WatchEvent
	// RuntimeConfig is peer's runtime configuration
	pb.RuntimeConfig
}

// CheckAndSetDefaults checks the parameters and autodetects some defaults
func (c *PeerConfig) CheckAndSetDefaults() error {
	if c.Context == nil {
		return trace.BadParameter("missing Context")
	}
	if len(c.Peers) == 0 {
		return trace.BadParameter("missing Peers")
	}
	if c.AdvertiseAddr == "" {
		return trace.BadParameter("missing AdvertiseAddr")
	}
	if err := checkAddr(c.AdvertiseAddr); err != nil {
		return trace.Wrap(err)
	}
	if c.Token == "" {
		return trace.BadParameter("missing Token")
	}
	if c.EventsC == nil {
		return trace.BadParameter("missing EventsC")
	}
	return nil
}

// Peer is a client that manages joining the cluster
type Peer struct {
	PeerConfig
	log.FieldLogger
	// agentDoneCh is the agent's done channel.
	// Only set after the agent has been started
	agentDoneCh <-chan struct{}
	// agent is this peer's RPC agent
	agent rpcserver.Server
}

// NewPeer returns new cluster peer client
func NewPeer(cfg PeerConfig, log log.FieldLogger) (*Peer, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &Peer{PeerConfig: cfg, FieldLogger: log}, nil
}

func (p *Peer) dialSite(addr string) (*operationContext, error) {
	var targetURL string
	// assume that is URL
	if strings.Contains(addr, "http") {
		targetURL = addr
	} else {
		targetURL = fmt.Sprintf("https://%v:%v", addr, defaults.GravitySiteNodePort)
	}
	httpClient := roundtrip.HTTPClient(httplib.GetClient(true))
	operator, err := opsclient.NewBearerClient(targetURL, p.Token, httpClient)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	packages, err := webpack.NewBearerClient(targetURL, p.Token, httpClient)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cluster, err := operator.GetLocalSite()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := p.checkAndSetServerProfile(cluster.App); err != nil {
		return nil, trace.Wrap(err)
	}
	installOp, _, err := ops.GetInstallOperation(cluster.Key(), operator)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = checks.RunLocalChecks(checks.LocalChecksRequest{
		Context:  p.Context,
		Manifest: cluster.App.Manifest,
		Role:     p.Role,
		Options: &validationpb.ValidateOptions{
			VxlanPort:     int32(installOp.GetVars().OnPrem.VxlanPort),
			DnsListenAddr: installOp.GetVars().OnPrem.DNSListenAddr,
		},
		AutoFix: true,
	})
	if err != nil {
		return nil, utils.Abort(err)
	}

	opReq := ops.CreateSiteExpandOperationRequest{
		SiteDomain: cluster.Domain,
		AccountID:  cluster.AccountID,
		// With CLI install flow we always rely on external provisioner
		Provisioner: schema.ProvisionerOnPrem,
		Servers:     map[string]int{p.Role: 1},
	}
	key, err := operator.CreateSiteExpandOperation(opReq)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	op, err := operator.GetSiteOperation(*key)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	creds, err := loadRPCCredentials(p.Context, packages, p.FieldLogger)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &operationContext{
		Operation: *op,
		Operator:  operator,
		Site:      *cluster,
		Creds:     *creds,
	}, nil
}

// dialWizard connects to a wizard
func (p *Peer) dialWizard(addr string) (*operationContext, error) {
	env, err := localenv.NewRemoteEnvironment()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	_, err = env.LoginWizard(addr)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cluster, operation, err := validateWizardState(env.Operator)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = p.checkAndSetServerProfile(cluster.App)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = checks.RunLocalChecks(checks.LocalChecksRequest{
		Context:  p.Context,
		Manifest: cluster.App.Manifest,
		Role:     p.Role,
		Options: &validationpb.ValidateOptions{
			VxlanPort:     int32(operation.GetVars().OnPrem.VxlanPort),
			DnsListenAddr: operation.GetVars().OnPrem.DNSListenAddr,
		},
		AutoFix: true,
	})
	if err != nil {
		return nil, utils.Abort(err)
	}
	creds, err := loadRPCCredentials(p.Context, env.Packages, p.FieldLogger)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &operationContext{
		Operation: *operation,
		Operator:  env.Operator,
		Site:      *cluster,
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
	return trace.NotFound("server role %q is not found", p.Role)
}

// operationContext describes the active install/expand operation.
// Used by peers to add new nodes for install/expand and poll progress
// of the operation.
type operationContext struct {
	Operator  ops.Operator
	Operation ops.SiteOperation
	Site      ops.Site
	Creds     rpcserver.Credentials
}

// connect dials to either a running wizard OpsCenter or a local gravity cluster.
// For wizard, if the dial succeeds, it will join the active installation and return
// an operation context of the active install operation.
//
// For a local gravity cluster, it will attempt to start the expand operation
// and will return an operation context wrapping a new expand operation.
func (p *Peer) connect() (*operationContext, error) {
	ticker := backoff.NewTicker(leader.NewUnlimitedExponentialBackOff())
	for {
		select {
		case <-p.Context.Done():
			return nil, trace.ConnectionProblem(p.Context.Err(), "context is closing")
		case tm := <-ticker.C:
			if tm.IsZero() {
				return nil, trace.ConnectionProblem(nil, "timeout")
			}
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
		}
	}
}

func (p *Peer) tryConnect() (op *operationContext, err error) {
	p.sendMessage("Connecting to cluster")
	for _, addr := range p.Peers {
		op, err = p.dialWizard(addr)
		if err == nil {
			p.sendMessage("Connected to installer at %v", addr)
			return op, nil
		}
		if utils.IsAbortError(err) {
			return nil, trace.Wrap(err)
		}

		// already exists error is returned when there's an ongoing install operation,
		// do not attempt to dial the site until it fully completes
		if trace.IsAlreadyExists(err) {
			p.sendMessage("Waiting for the install operation to finish")
			return nil, trace.Wrap(err)
		}
		p.Infof("failed connecting to wizard: %v", err)

		op, err = p.dialSite(addr)
		if err == nil {
			p.sendMessage("Connected to existing cluster at %v", addr)
			return op, nil
		}
		if utils.IsAbortError(err) {
			return nil, trace.Wrap(err)
		}

		p.Infof("failed connecting to cluster: %v", err)
		if trace.IsCompareFailed(err) {
			p.sendMessage("Waiting for another operation to finish at %v", addr)
		}
	}
	return op, trace.Wrap(err)
}

func (p *Peer) run() error {
	ctx, err := p.connect()
	if err != nil {
		return trace.Wrap(err)
	}

	serviceUser, err := EnsureServiceUserAndBinary(ctx.Site.ServiceUser.UID, ctx.Site.ServiceUser.GID)
	if err != nil {
		return trace.Wrap(err)
	}
	log.Infof("Service user: %v.", serviceUser)

	installState := ctx.Operation.InstallExpand
	if installState == nil {
		return trace.BadParameter("internal error, no install state for %v", ctx.Operation)
	}
	p.Debugf("Got operation state: %#v.", installState)

	if err := p.checkAndSetServerProfile(ctx.Site.App); err != nil {
		return trace.Wrap(err)
	}

	agentInstructions, ok := installState.Agents[p.Role]
	if !ok {
		return trace.NotFound("agent instructions not found for %v", p.Role)
	}

	listener, err := net.Listen("tcp", defaults.GravityRPCAgentAddr(p.AdvertiseAddr))
	if err != nil {
		return trace.Wrap(err)
	}

	config := rpcserver.PeerConfig{
		Config: rpcserver.Config{
			Listener:      listener,
			Credentials:   ctx.Creds,
			RuntimeConfig: p.RuntimeConfig,
		},
		WatchCh: p.WatchCh,
		ReconnectStrategy: rpcserver.ReconnectStrategy{
			ShouldReconnect: shouldReconnectPeer,
		},
	}
	agent, err := startAgent(agentInstructions.AgentURL, config,
		p.WithField("peer", listener.Addr().String()))
	if err != nil {
		listener.Close()
		return trace.Wrap(err)
	}

	p.agent = agent
	p.agentDoneCh = agent.Done()
	go agent.Serve()

	if ctx.Operation.Type == ops.OperationExpand {
		err = p.startExpandOperation(*ctx)
		if err != nil {
			agent.Stop(p.Context)
			if err := ctx.Operator.DeleteSiteOperation(ctx.Operation.Key()); err != nil {
				p.Errorf("failed to delete operation: %v", trace.DebugReport(err))
			}
			return trace.Wrap(err)
		}
	}

	pollProgress(p.Context, p.send, ctx.Operator, ctx.Operation.Key(), agent.Done())
	return nil
}

// Stop shuts down RPC agent
func (p *Peer) Stop(ctx context.Context) error {
	if p.agent == nil {
		return nil
	}
	return trace.Wrap(p.agent.Stop(ctx))
}

func (p *Peer) waitForAgents(site ops.Site, operator ops.Operator, opKey ops.SiteOperationKey) error {
	ticker := backoff.NewTicker(&backoff.ExponentialBackOff{
		InitialInterval: time.Second,
		Multiplier:      1.5,
		MaxInterval:     10 * time.Second,
		MaxElapsedTime:  5 * time.Minute,
		Clock:           backoff.SystemClock,
	})

	for {
		select {
		case <-p.Context.Done():
			return trace.ConnectionProblem(p.Context.Err(), "context is closing")
		case tm := <-ticker.C:
			if tm.IsZero() {
				return trace.ConnectionProblem(nil, "timed out waiting for agents to join")
			}
			report, err := operator.GetSiteExpandOperationAgentReport(opKey)
			if err != nil {
				p.Warningf("%v", err)
				continue
			}
			if len(report.Servers) == 0 {
				continue
			}
			op, err := operator.GetSiteOperation(opKey)
			if err != nil {
				p.Warningf("%v", err)
				continue
			}
			req, err := getServers(*op, report.Servers)
			if err != nil {
				p.Warningf("%v", err)
				continue
			}
			err = operator.UpdateExpandOperationState(opKey, *req)
			if err != nil {
				return trace.Wrap(err)
			}
			p.Infof("installation can proceed! %v", report)
			return nil
		}
	}
}

func (p *Peer) send(e Event) {
	select {
	case p.EventsC <- e:
	case <-p.Context.Done():
		select {
		case p.EventsC <- Event{Error: trace.ConnectionProblem(p.Context.Err(), "context is closing")}:
		default:
		}
	default:
		p.Warningf("failed to send event, events channel is blocked")
	}
}

// sendMessage sends an event with just a progress message
func (p *Peer) sendMessage(format string, args ...interface{}) {
	p.send(Event{
		Progress: &ops.ProgressEntry{
			Message: fmt.Sprintf(format, args...)},
	})
}

// Start starts non-interactive join process
func (p *Peer) Start() (err error) {
	go func() {
		err := p.run()
		if err != nil {
			p.send(Event{Error: err})
		}
	}()
	return nil
}

// Done returns the done channel for the agent if one has been started.
// If no agent has been started, nil channel is returned
func (p *Peer) Done() <-chan struct{} {
	return p.agentDoneCh
}

func (p *Peer) startExpandOperation(ctx operationContext) error {
	err := p.waitForAgents(ctx.Site, ctx.Operator, ctx.Operation.Key())
	if err != nil {
		return trace.Wrap(err)
	}

	err = ctx.Operator.SiteExpandOperationStart(ctx.Operation.Key())
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func validateWizardState(operator ops.Operator) (*ops.Site, *ops.SiteOperation, error) {
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
	case ops.OperationStateInstallInitiated, ops.OperationStateFailed:
		// Consider these states for resuming the installation
		// (including failed that puts the operation into manual mode)
	default:
		return nil, nil,
			trace.AlreadyExists("operation %v is in progress", operation)
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

	report, err := operator.GetSiteInstallOperationAgentReport(operation.Key())
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	// do not join until there's at least one other agent, this way we can make sure that
	// the agent on the installer node (the one that runs "install") joins first
	if len(report.Servers) == 0 {
		return nil, nil, trace.NotFound("no other agents joined yet")
	}

	return &cluster, operation, nil
}

// shouldReconnectPeer implements the error classification for
// peer connection errors.
// It detects unrecoverable errors and aborts the reconnect attempts.
func shouldReconnectPeer(err error) error {
	if isPeerDeniedError(err.Error()) {
		return &backoff.PermanentError{err}
	}
	return err
}

func isPeerDeniedError(message string) bool {
	return strings.Contains(message, "AccessDenied")
}
