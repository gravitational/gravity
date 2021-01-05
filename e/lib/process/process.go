// Copyright 2021 Gravitational Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package process

import (
	"context"
	"time"

	"github.com/gravitational/gravity/e/lib/constants"
	"github.com/gravitational/gravity/e/lib/ops"
	"github.com/gravitational/gravity/e/lib/ops/handler"
	"github.com/gravitational/gravity/e/lib/ops/router"
	"github.com/gravitational/gravity/e/lib/ops/service"
	"github.com/gravitational/gravity/e/lib/periodic"
	"github.com/gravitational/gravity/e/lib/webapi"
	ossconstants "github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	ossops "github.com/gravitational/gravity/lib/ops"
	ossrouter "github.com/gravitational/gravity/lib/ops/opsroute"
	ossservice "github.com/gravitational/gravity/lib/ops/opsservice"
	"github.com/gravitational/gravity/lib/process"
	"github.com/gravitational/gravity/lib/processconfig"
	"github.com/gravitational/gravity/lib/sni"
	"github.com/gravitational/gravity/lib/utils"

	teleconfig "github.com/gravitational/teleport/lib/config"
	teleservice "github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/trace"
)

// Process extends open-source gravity process with enterprise specific features
type Process struct {
	// Process is the underlying open-source version of the process
	*process.Process
	// mux is the Ops Center SNI mupliplexer serving public traffic
	mux *sni.Mux
	// agentsMux is the Ops Center SNI multiplexer serving cluster agents
	// traffic, only initialized if user/agents traffic is separated with
	// endpoints resource
	agentsMux *sni.Mux
	// handlers contains all web handlers
	handlers *Handlers
	// operatorReadyC is the chan signalled when the operator reference
	// has been initialized. It is never written to
	operatorReadyC chan struct{}
	// operator is the enterprise operator service
	operator ops.Operator
}

// Handlers extends open-source web handlers with enterprise handlers
type Handlers struct {
	// Handlers is the open-source handlers
	*process.Handlers
	// Operator extends the open-source ops service handler
	Operator *handler.WebHandler
	// WebAPI extends the open-source web API handler
	WebAPI *webapi.Handler
}

// NewProcess creates a new instance of enterprise gravity API server
//
// Satisfies process.NewGravityProcess function type.
func NewProcess(ctx context.Context, cfg processconfig.Config, tcfg teleconfig.FileConfig) (process.GravityProcess, error) {
	return New(ctx, cfg, tcfg)
}

// New returns a new uninitialized instance of an enterprise process
func New(ctx context.Context, cfg processconfig.Config, tcfg teleconfig.FileConfig) (*Process, error) {
	ossProcess, err := process.New(ctx, cfg, tcfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &Process{
		Process:        ossProcess,
		operatorReadyC: make(chan struct{}, 1),
	}, nil
}

// Init initializes the enterprise gravity process, it can then be started
// using Start
func (p *Process) Init() error {
	// register enterprise-specific cluster services, this should be done
	// before initializing the open-source process
	p.registerClusterServices()
	// init the open-source process
	if err := p.Process.Init(p.Context()); err != nil {
		return trace.Wrap(err)
	}
	// the OSS operator can be either a local operator or a router
	switch o := p.Operator().(type) {
	case *ossservice.Operator:
		p.operator = service.New(o)
	case *ossrouter.Router:
		p.operator = router.New(o, service.New(o.Local))
	default:
		return trace.BadParameter("unexpected type: %T", p.Operator())
	}
	close(p.operatorReadyC)
	p.handlers = &Handlers{
		Handlers: p.Handlers(),
		Operator: handler.NewWebHandler(p.Handlers().Operator, p.operator),
		WebAPI:   webapi.NewHandler(p.Handlers().WebAPI, p.operator),
	}
	// some actions should be taken when running inside Kubernetes
	if p.KubeClient() != nil {
		err := p.runMigrations(p.KubeClient())
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// Start starts all process services
func (p *Process) Start() (err error) {
	p.TeleportProcess, err = teleservice.NewTeleport(p.TeleportConfig())
	if err != nil {
		return trace.Wrap(err)
	}
	p.RegisterFunc("gravity.service", func() (err error) {
		defer p.BroadcastEvent(teleservice.Event{
			Name:    ossconstants.ServiceStartedEvent,
			Payload: &process.ServiceStartedEvent{Error: err},
		})
		if err = p.Init(); err != nil {
			return trace.Wrap(err)
		}
		if err = p.Serve(); err != nil {
			return trace.Wrap(err)
		}
		return nil
	})
	return p.TeleportProcess.Start()
}

// Serve starts serving all process web services
func (p *Process) Serve() error {
	err := p.initMux(p.Context())
	if err != nil {
		return trace.Wrap(err)
	}
	err = p.ServeHealth()
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// runUpdater runs cluster update poller and checker.
// Must be called in a goroutine
func (p *Process) runUpdater(ctx context.Context) {
	if !p.waitForOperator(ctx) {
		return
	}
	var cluster *ossops.Site
	err := utils.RetryWithInterval(ctx, utils.NewUnlimitedExponentialBackOff(), func() (err error) {
		cluster, err = p.operator.GetLocalSite(ctx)
		if err != nil {
			return trace.Wrap(err, "failed to get local cluster")
		}
		p.Info("Starting periodic updates.")
		err = p.operator.StartPeriodicUpdates(cluster.Key())
		if err != nil && !trace.IsNotFound(err) {
			return trace.Wrap(err, "failed to start periodic updates")
		}
		return nil
	})
	if err != nil {
		// unreachable unless the context is cancelled
		p.WithError(err).Warn("Failed to start periodic updates.")
		return
	}
	<-ctx.Done()
	p.Info("Stopping periodic updates.")
	err = p.operator.StopPeriodicUpdates(cluster.Key())
	if err != nil {
		p.WithError(err).Warn("Failed to stop periodic updates.")
	}
}

// runLicenseChecker runs a periodic license checker; should be run in a goroutine
func (p *Process) runLicenseChecker(ctx context.Context) {
	if !p.waitForOperator(ctx) {
		return
	}
	p.Info("Starting license checker.")
	ticker := time.NewTicker(defaults.LicenseCheckInterval)
	defer ticker.Stop()
	localCtx := context.WithValue(ctx, ossconstants.UserContext,
		constants.ServiceLicenseChecker)
	for {
		select {
		case <-ticker.C:
			cluster, err := p.operator.GetLocalSite(ctx)
			if err != nil {
				p.WithError(err).Warn("Failed to get local cluster.")
				continue
			}
			err = p.operator.CheckSiteLicense(localCtx, cluster.Key())
			if err != nil {
				p.WithError(err).Warn("License check failed.")
				continue
			}
		case <-ctx.Done():
			p.Info("Stopping license checker.")
			return
		}
	}
}

// startStateChecker launches a goroutine that monitors installed clusters and
// transitions them to offline state and back based on whether they maintain
// reverse tunnel to OpsCenter or not
func (p *Process) startStateChecker(ctx context.Context) {
	if !p.waitForOperator(ctx) {
		return
	}
	p.Info("Starting state checker.")
	periodic.StartStateChecker(periodic.StateCheckerConfig{
		Context:  ctx,
		Backend:  p.Backend(),
		Operator: p.Operator(),
		Packages: p.Packages(),
		Tunnel:   p.ReverseTunnel(),
	})
}

// registerClusterServices registers enterprise-specific subroutines
// that will be running inside active gravity process leader
func (p *Process) registerClusterServices() {
	// updater checks for new versions of apps and downloads them
	p.RegisterClusterService(p.runUpdater)
	// license checker makes sure the installed cluster license is valid
	p.RegisterClusterService(p.runLicenseChecker)
	// state checker periodically checks connected clusters
	if p.Config().Mode == constants.ComponentOpsCenter {
		p.RegisterClusterService(p.startStateChecker)
	}
	// older trusted clusters may need to be reconnected
	// TODO Remove after 5.4.0 LTS release
	p.RegisterClusterService(p.maybeReconnectTrustedClusters)
}

// waitForOperator blocks until this process operator has been initialized
func (p *Process) waitForOperator(ctx context.Context) bool {
	select {
	case <-p.operatorReadyC:
		return true
	case <-ctx.Done():
		return false
	}
}
