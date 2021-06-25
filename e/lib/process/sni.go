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
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/sni"
	"github.com/gravitational/gravity/lib/utils"
	"github.com/gravitational/gravity/lib/webapi"

	"github.com/gravitational/trace"
)

// startSNIUpdater starts a process to reflect changes to SNI routes in the handler
func (p *Process) startSNIUpdater(ctx context.Context) {
	p.Infof("Starting SNI updater.")
	if err := p.updateSNIRoutes(ctx); err != nil {
		p.Errorf("Failed updating SNI routes: %v.", trace.DebugReport(err))
	}
	ticker := time.NewTicker(defaults.CheckForUpdatesInterval)
	for {
		select {
		case <-ticker.C:
			if err := p.updateSNIRoutes(ctx); err != nil {
				p.Errorf("Failed updating SNI routes: %v.",
					trace.DebugReport(err))
			}
		case <-ctx.Done():
			p.Infof("Stopping SNI updater.")
			ticker.Stop()
			return
		}
	}
}

func (p *Process) updateSNIRoutes(context.Context) error {
	accounts, err := p.operator.GetAccounts()
	if err != nil {
		return trace.Wrap(err)
	}
	sites := make(map[string]struct{})
	for _, a := range accounts {
		out, err := p.operator.GetSites(a.ID)
		if err != nil {
			return trace.Wrap(err)
		}
		for i := range out {
			sites[out[i].Domain] = struct{}{}
		}
	}

	sniHost := p.APIAdvertiseHost()
	sniPublicHost := p.WebAdvertiseHost()

	addFrontend := func(siteDomain string) error {
		targetHost := strings.Join([]string{siteDomain, sniPublicHost}, ".")
		if p.mux.HasFrontend(targetHost) {
			return nil
		}
		err := p.mux.AddFrontend(sni.Frontend{
			Host: targetHost,
			Name: siteDomain,
			Dial: func() (net.Conn, error) {
				site, err := p.ReverseTunnel().GetSite(siteDomain)
				if err != nil {
					return nil, trace.Wrap(err)
				}
				return site.Dial(
					webapi.NewTCPAddr("127.0.0.1:3024"),
					webapi.NewTCPAddr(fmt.Sprintf("%v:%v",
						constants.APIServerDomainName,
						defaults.APIServerSecurePort)),
					nil)
			},
		})
		if err != nil {
			return trace.Wrap(err)
		}
		return nil
	}

	for name := range sites {
		if err := addFrontend(name); err != nil {
			return trace.Wrap(err)
		}
	}

	for _, f := range p.mux.ExistingFrontends() {
		keep := []string{sniHost, sniPublicHost, defaults.GravityServiceHost}
		if utils.StringInSlice(keep, f.Host) {
			continue
		}
		if _, ok := sites[f.Name]; !ok {
			if err := p.mux.RemoveFrontend(f.Host); err != nil {
				p.Debugf("Failed to remove frontend: %v.",
					trace.DebugReport(err))
			}
		}
	}

	return nil
}
