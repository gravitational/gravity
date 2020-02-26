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
	"net/url"
	"strings"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/install"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/pack"
	rpcserver "github.com/gravitational/gravity/lib/rpc/server"
	"github.com/gravitational/gravity/lib/state"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/system/environ"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// init initializes the peer after a successful connect
func (p *Peer) init(ctx operationContext) error {
	err := p.initEnviron(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	return p.startAgent(ctx)
}

func (p *Peer) initEnviron(ctx operationContext) error {
	if err := p.clearLogins(); err != nil {
		return trace.Wrap(err)
	}
	if err := p.logIntoPeer(); err != nil {
		return trace.Wrap(err)
	}
	if err := p.configureStateDirectory(); err != nil {
		return trace.Wrap(err)
	}
	if err := p.ensureServiceUserAndBinary(ctx); err != nil {
		return trace.Wrap(err)
	}
	if err := p.downloadFio(ctx); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// clearLogins removes all login entries from the local join backend
func (p *Peer) clearLogins() error {
	entries, err := p.JoinBackend.GetLoginEntries()
	if err != nil {
		return trace.Wrap(err)
	}
	for _, entry := range entries {
		err := p.JoinBackend.DeleteLoginEntry(entry.OpsCenterURL)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// logIntoPeer creates a login entry for the peer's peer in the local join backend
func (p *Peer) logIntoPeer() error {
	_, err := p.JoinBackend.UpsertLoginEntry(storage.LoginEntry{
		OpsCenterURL: formatClusterURL(p.Peers[0]),
		Password:     p.Token,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// configureStateDirectory configures local gravity state directory
func (p *Peer) configureStateDirectory() error {
	stateDir, err := state.GetStateDir()
	if err != nil {
		return trace.Wrap(err)
	}
	err = environ.ConfigureStateDirectory(stateDir, p.SystemDevice)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// ensureServiceUserAndBinary makes sure specified service user exists and installs gravity binary
func (p *Peer) ensureServiceUserAndBinary(ctx operationContext) error {
	_, err := install.EnsureServiceUserAndBinary(ctx.Cluster.ServiceUser.UID, ctx.Cluster.ServiceUser.GID)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// downloadFio downloads fio binary from the installer's package service.
func (p *Peer) downloadFio(ctx operationContext) error {
	locator, err := ctx.Cluster.App.Manifest.Dependencies.ByName(constants.FioPackage)
	if err != nil {
		return trace.Wrap(err)
	}
	path, err := state.InStateDir(constants.FioBin)
	if err != nil {
		return trace.Wrap(err)
	}
	err = pack.ExportExecutable(ctx.Packages, *locator, path, defaults.GravityFileLabel)
	if err != nil {
		return trace.Wrap(err)
	}
	p.Infof("Exported fio v%v to %v.", locator.Version, path)
	return nil
}

// syncOperationPlan synchronizes operation and plan data to the local join backend
func (p *Peer) syncOperationPlan(ctx operationContext) error {
	err := p.syncOperation(ctx.Operator, ctx.Cluster, ctx.Operation.Key())
	if err != nil {
		return trace.Wrap(err)
	}
	plan, err := ctx.Operator.GetOperationPlan(ctx.Operation.Key())
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = p.JoinBackend.CreateOperationPlan(*plan)
	if err != nil {
		return trace.Wrap(err)
	}
	p.Debug("Synchronized operation plan to the local backend.")
	return nil
}

// syncOperation synchronizes operation-related data to the local join backend
func (p *Peer) syncOperation(operator ops.Operator, cluster ops.Site, operationKey ops.SiteOperationKey) error {
	// sync cluster
	err := p.JoinBackend.DeleteSite(cluster.Domain)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	_, err = p.JoinBackend.CreateSite(ops.ConvertOpsSite(cluster))
	if err != nil {
		return trace.Wrap(err)
	}
	// sync operation
	operation, err := operator.GetSiteOperation(operationKey)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = p.JoinBackend.CreateSiteOperation(storage.SiteOperation(*operation))
	return trace.Wrap(err)
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
func (p *Peer) newAgent(ctx operationContext) (*rpcserver.PeerServer, error) {
	peerAddr, token, err := getPeerAddrAndToken(ctx, p.Role)
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
		CloudProvider: ctx.Cluster.Provider,
		ServerAddr:    peerAddr,
		Credentials:   ctx.Creds,
		RuntimeConfig: p.RuntimeConfig,
		WatchCh:       p.WatchCh,
		StopHandler:   p.server.ManualStop,
		AbortHandler:  p.server.Interrupted,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return agent, nil
}

// getPeerAddrAndToken returns the peer address and token for the specified role
func getPeerAddrAndToken(ctx operationContext, role string) (peerAddr, token string, err error) {
	peerAddr = ctx.Peer
	if strings.HasPrefix(peerAddr, "http") { // peer may be an URL
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
