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
	"net"
	"net/http"

	"github.com/gravitational/gravity/e/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/httplib"
	"github.com/gravitational/gravity/lib/sni"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
)

func (p *Process) initMux(ctx context.Context) (err error) {
	p.Info("Initializing mux.")

	publicMux, agentsMux := p.getMux()

	switch p.trafficSplitType() {
	case notOpsCenter:
		// return here as we don't need to start SNI updater in non Ops Center mode
		return trace.Wrap(p.ServeLocal(ctx, httplib.GRPCHandlerFunc(
			p.AgentServer(), publicMux), p.Config().Pack.ListenAddr.Addr))
	case sameHostPort:
		err = p.sameHostPort(ctx, publicMux)
	case diffHostSamePort:
		err = p.diffHostSamePort(ctx, publicMux, agentsMux)
	case sameHostDiffPort, diffHostDiffPort:
		err = p.diffPort(ctx, publicMux, agentsMux)
	}
	if err != nil {
		return trace.Wrap(err)
	}

	go p.startSNIUpdater(ctx)
	return nil
}

// sameHostPort initializes local listeners and SNI routers for the scenario
// when all Ops Center traffic is served on the same host/port, for example
// in case the endpoints look like this:
//
//   public_advertise_addr: ops.example.com:443
//
// In this scenario, there is a single local listener that serves all traffic
// and a single SNI mux that routes to this listener.
func (p *Process) sameHostPort(ctx context.Context, mux *httprouter.Router) error {
	p.Info("Traffic split type: not split.")
	host, _ := utils.SplitHostPort(p.Config().Pack.GetAddr().Addr, "")
	return p.initListenersAndSNI(ctx, trafficSplitConfig{
		publicHandler: httplib.GRPCHandlerFunc(p.AgentServer(), mux),
		publicMux: sni.ListenConfig{
			ListenAddr: p.Config().Pack.ListenAddr.String(),
			Frontends: []sni.Frontend{
				{
					Host: host,
					Name: host,
					Dial: func() (net.Conn, error) {
						return net.Dial("tcp", defaults.LocalPublicAddr)
					},
					Default: true,
				},
			},
		},
	})
}

// diffHostSamePort initializes local listeners and SNI routers for the scenario
// when cluster agents traffic should be served on a different hostname than
// user traffic but on the same port, for example in case the endpoints look
// like this:
//
//   public_advertise_addr: ops.example.com:443
//   agents_advertise_addr: ops-api.example.com:443
//
// In this scenario there are 2 local listeners for user and agents traffic
// and a single SNI mux that routes traffic to the appropriate listener
// based on the SNI hostname.
func (p *Process) diffHostSamePort(ctx context.Context, publicMux, agentsMux *httprouter.Router) error {
	p.Info("Traffic split type: diff host, same port.")
	publicHost, _ := utils.SplitHostPort(p.Config().Pack.GetPublicAddr().Addr, "")
	agentsHost, _ := utils.SplitHostPort(p.Config().Pack.GetAddr().Addr, "")
	return p.initListenersAndSNI(ctx, trafficSplitConfig{
		publicHandler: publicMux,
		agentsHandler: httplib.GRPCHandlerFunc(p.AgentServer(), agentsMux),
		publicMux: sni.ListenConfig{
			ListenAddr: p.Config().Pack.ListenAddr.String(),
			Frontends: []sni.Frontend{
				{
					Host: publicHost,
					Name: publicHost,
					Dial: func() (net.Conn, error) {
						return net.Dial("tcp", defaults.LocalPublicAddr)
					},
					Default: true,
				},
				{
					Host: agentsHost,
					Name: agentsHost,
					Dial: func() (net.Conn, error) {
						return net.Dial("tcp", defaults.LocalAgentsAddr)
					},
				},
				{
					// this frontend is required so all gravity CLI commands
					// that talk to gravity-site service keep working
					Host: defaults.GravityServiceHost,
					Name: defaults.GravityServiceHost,
					Dial: func() (net.Conn, error) {
						return net.Dial("tcp", defaults.LocalAgentsAddr)
					},
				},
			},
		},
	})
}

// diffPort initializes local listeners and SNI routers for the scenario when
// user and cluster agents traffic should be served on different ports, for
// example in case the endpoints look like this:
//
//   public_advertise_addr: ops.example.com:443
//   agents_advertise_addr: ops.example.com:444
//
// In this scenario there are 2 local listeners for user and agents traffic
// and a two SNI muxes that route traffic to respective listeners.
func (p *Process) diffPort(ctx context.Context, publicMux, agentsMux *httprouter.Router) error {
	p.Info("Traffic split type: diff port.")
	publicHost, _ := utils.SplitHostPort(p.Config().Pack.GetPublicAddr().Addr, "")
	agentsHost, _ := utils.SplitHostPort(p.Config().Pack.GetAddr().Addr, "")
	return p.initListenersAndSNI(ctx, trafficSplitConfig{
		publicHandler: publicMux,
		publicMux: sni.ListenConfig{
			ListenAddr: p.Config().Pack.PublicListenAddr.String(),
			Frontends: []sni.Frontend{
				{
					Host: publicHost,
					Name: publicHost,
					Dial: func() (net.Conn, error) {
						return net.Dial("tcp", defaults.LocalPublicAddr)
					},
					Default: true,
				},
			},
		},
		agentsHandler: httplib.GRPCHandlerFunc(p.AgentServer(), agentsMux),
		agentsMux: &sni.ListenConfig{
			ListenAddr: p.Config().Pack.ListenAddr.String(),
			Frontends: []sni.Frontend{
				{
					Host: agentsHost,
					Name: agentsHost,
					Dial: func() (net.Conn, error) {
						return net.Dial("tcp", defaults.LocalAgentsAddr)
					},
					Default: true,
				},
			},
		},
	})
}

// trafficSplitConfig is the configuration that describes which listeners
// should be started and which handlers should be used in Ops Center mode
type trafficSplitConfig struct {
	// publicHandler is the router that provides handlers for user traffic
	publicHandler http.Handler
	// agentsHandler is the router that provides handlers for cluster traffic,
	// may be nil if traffic is not split
	agentsHandler http.Handler
	// publicMux is the SNI router configuration for user traffic
	publicMux sni.ListenConfig
	// agentsMux is the SNI router configuration for cluster traffic,
	// may be nil if traffic is not split
	agentsMux *sni.ListenConfig
}

// initListenerAndMux starts appropriate local listeners and SNI routers
// based on the provided config
func (p *Process) initListenersAndSNI(ctx context.Context, config trafficSplitConfig) (err error) {
	err = p.ServeLocal(ctx, config.publicHandler, defaults.LocalPublicAddr)
	if err != nil {
		return trace.Wrap(err)
	}
	p.Infof("Started local public listener: %v.", defaults.LocalPublicAddr)
	if config.agentsHandler != nil {
		err = p.ServeLocal(ctx, config.agentsHandler, defaults.LocalAgentsAddr)
		if err != nil {
			return trace.Wrap(err)
		}
		p.Infof("Started local agents listener: %v.", defaults.LocalAgentsAddr)
	}
	p.mux, err = sni.Listen(config.publicMux)
	if err != nil {
		return trace.Wrap(err)
	}
	p.Infof("Started SNI listener: %v.", config.publicMux.ListenAddr)
	// if agents mux was specified, start it too
	if config.agentsMux != nil {
		p.agentsMux, err = sni.Listen(*config.agentsMux)
		if err != nil {
			return trace.Wrap(err)
		}
		p.Infof("Started SNI listener: %v.", config.agentsMux.ListenAddr)
	}
	return nil
}

// getMux returns a pair of routers initialized with appropriate handlers
// based on whether the user and cluster traffic is split in the current
// process configuration
//
// The agentsMux may be nil if all traffic will be served by public router.
func (p *Process) getMux() (publicMux *httprouter.Router, agentsMux *httprouter.Router) {
	// regular clusters and standalone install process do not support
	// traffic splitting, so everything is served by the same mux
	if p.Config().Mode != constants.ComponentOpsCenter {
		publicMux = &httprouter.Router{}
		p.addAllHandlers(publicMux)
		return publicMux, nil
	}
	// traffic is not split, everything is served by the same mux
	if p.Config().Pack.GetAddr().Addr == p.Config().Pack.GetPublicAddr().Addr {
		publicMux = &httprouter.Router{}
		p.addAllHandlers(publicMux)
		return publicMux, nil
	}
	// otherwise traffix is split, there are separate muxes for public
	// and agents APIs
	publicMux = &httprouter.Router{}
	p.addPublicHandlers(publicMux)
	agentsMux = &httprouter.Router{}
	p.addAgentsHandlers(agentsMux)
	return publicMux, agentsMux
}

// addPublicHandlers updates the provided mux with handlers required for
// user UI and tele command to work, for the case when public Ops Center
// traffic is split from internal traffic
func (p *Process) addPublicHandlers(mux *httprouter.Router) {
	for _, method := range httplib.Methods {
		mux.Handler(method, "/telekube/*rest", p.Handlers().Apps)
		mux.Handler(method, "/charts/*rest", p.Handlers().Apps)
		if p.Handlers().Web != nil {
			mux.Handler(method, "/web", p.Handlers().Web) // to handle redirect
			mux.Handler(method, "/web/*web", p.Handlers().Web)
		}
		mux.Handler(method, "/proxy/*proxy", http.StripPrefix("/proxy", p.Handlers().WebProxy))
		mux.Handler(method, "/v1/webapi/*webapi", p.Handlers().WebProxy)
		mux.Handler(method, "/portalapi/v1/*portalapi", http.StripPrefix("/portalapi/v1", p.handlers.WebAPI))
		mux.Handler(method, "/sites/*rest", p.Handlers().Proxy)
		// pretty much the whole pack, app and ops services should be
		// public because they are our main APIs at the moment and are
		// used by tools like tele login/build/push/pull/rm/etc
		mux.Handler(method, "/pack/*packages", p.Handlers().Packages)
		mux.Handler(method, "/app/*apps", p.Handlers().Apps)
		mux.Handler(method, "/portal/*portal", p.handlers.Operator)
		mux.Handler(method, "/v2/*rest", p.Handlers().Registry)
	}
	if p.Handlers().Web != nil {
		mux.NotFound = p.Handlers().Web.NotFound
	} else {
		mux.NotFound = p.Handlers().WebAPI.NotFound
	}
}

// addAgentsHandlers updates the provided mux with internal API handlers
// required for in-cluster and cluster -> Ops Center communication, for
// the case when public Ops Center traffic is split from internal traffic
func (p *Process) addAgentsHandlers(mux *httprouter.Router) {
	for _, method := range httplib.Methods {
		mux.Handler(method, "/pack/*packages", p.Handlers().Packages)
		mux.Handler(method, "/portal/*portal", p.handlers.Operator)
		mux.Handler(method, "/t/*portal", p.handlers.Operator) // shortener for instructions tokens
		mux.Handler(method, "/app/*apps", p.Handlers().Apps)
		mux.Handler(method, "/objects/*rest", p.Handlers().BLOB)
		mux.Handler(method, "/proxy/*proxy", http.StripPrefix("/proxy", p.Handlers().WebProxy))
		mux.Handler(method, "/v1/webapi/*webapi", p.Handlers().WebProxy)
		mux.HandlerFunc(method, "/readyz", p.ReportReadiness)
		mux.HandlerFunc(method, "/healthz", p.ReportHealth)
	}
}

// addAllHandlers adds all API handlers to the provided mux, for the case when
// traffic is not split into public/internal
func (p *Process) addAllHandlers(mux *httprouter.Router) {
	for _, method := range httplib.Methods {
		if p.Handlers().Web != nil {
			mux.Handler(method, "/web", p.Handlers().Web) // to handle redirect
			mux.Handler(method, "/web/*web", p.Handlers().Web)
		}
		mux.Handler(method, "/proxy/*proxy", http.StripPrefix("/proxy", p.Handlers().WebProxy))
		mux.Handler(method, "/v1/webapi/*webapi", p.Handlers().WebProxy)
		mux.Handler(method, "/portalapi/v1/*portalapi", http.StripPrefix("/portalapi/v1", p.handlers.WebAPI))
		mux.Handler(method, "/sites/*rest", p.Handlers().Proxy)
		mux.Handler(method, "/pack/*packages", p.Handlers().Packages)
		mux.Handler(method, "/portal/*portal", p.handlers.Operator)
		mux.Handler(method, "/t/*portal", p.handlers.Operator) // shortener for instructions tokens
		mux.Handler(method, "/app/*apps", p.Handlers().Apps)
		mux.Handler(method, "/telekube/*rest", p.Handlers().Apps)
		mux.Handler(method, "/charts/*rest", p.Handlers().Apps)
		mux.Handler(method, "/objects/*rest", p.Handlers().BLOB)
		mux.Handler(method, "/v2/*rest", p.Handlers().Registry)
		mux.HandlerFunc(method, "/readyz", p.ReportReadiness)
		mux.HandlerFunc(method, "/healthz", p.ReportHealth)
	}
	if p.Handlers().Web != nil {
		mux.NotFound = p.Handlers().Web.NotFound
	} else {
		mux.NotFound = p.Handlers().WebAPI.NotFound
	}
}

// trafficSplitType returns an appropriate traffic split type based on
// the configured public/agents advertise addresses
func (p *Process) trafficSplitType() trafficSplitType {
	if p.Config().Mode != constants.ComponentOpsCenter {
		return notOpsCenter
	}
	publicHost, publicPort := utils.SplitHostPort(p.Config().Pack.GetPublicAddr().Addr, "")
	agentsHost, agentsPort := utils.SplitHostPort(p.Config().Pack.GetAddr().Addr, "")
	if publicHost == agentsHost {
		if publicPort == agentsPort {
			return sameHostPort
		}
		return sameHostDiffPort
	}
	if publicPort == agentsPort {
		return diffHostSamePort
	}
	return diffHostDiffPort
}

// trafficSplitType defines a type for different cases of how an Ops Center
// serves user vs cluster traffic
type trafficSplitType byte

const (
	// notOpsCenter means that the process is not an Ops Center and traffic
	// should not be split
	notOpsCenter trafficSplitType = iota
	// sameHostPort means that both user and cluster traffic are served
	// over the same hostname and port
	sameHostPort
	// diffHostSamePort means that user and cluster traffic are served
	// on the same port but different hostnames
	diffHostSamePort
	// sameHostDiffPort means that user and cluster traffic are served
	// over the same hostname but different ports
	sameHostDiffPort
	// diffHostDiffPort means that user and cluster traffic are served
	// over different hostnames and ports
	diffHostDiffPort
)
