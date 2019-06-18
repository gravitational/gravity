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
	"github.com/gravitational/gravity/lib/install"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/state"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/system/environ"

	"github.com/gravitational/trace"
)

// init initializes the peer after a successful connect
func (p *Peer) init(ctx operationContext) error {
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
	if err := p.startAgent(ctx); err != nil {
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

// syncOperation synchronizes operation-related data to the local join backend
func (p *Peer) syncOperation(ctx operationContext) error {
	// sync cluster
	err := p.JoinBackend.DeleteSite(ctx.Cluster.Domain)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	_, err = p.JoinBackend.CreateSite(ops.ConvertOpsSite(ctx.Cluster))
	if err != nil {
		return trace.Wrap(err)
	}
	// sync operation
	operation, err := ctx.Operator.GetSiteOperation(ctx.Operation.Key())
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = p.JoinBackend.CreateSiteOperation(storage.SiteOperation(*operation))
	if err != nil {
		return trace.Wrap(err)
	}
	// sync operation plan
	plan, err := ctx.Operator.GetOperationPlan(ctx.Operation.Key())
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = p.JoinBackend.CreateOperationPlan(*plan)
	if err != nil {
		return trace.Wrap(err)
	}
	p.Debug("Synchronized operation to the local backend.")
	return nil
}
