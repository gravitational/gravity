/*
Copyright 2020 Gravitational, Inc.

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

package phases

import (
	"context"
	"time"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/teleport/lib/services"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// NewState returns executor that updates the cluster state.
//
// This phase patches the cluster state with updated node/cluster information
// and also removes old certificate authorities to make sure they are
// regenerated when teleport auth server starts up.
func NewState(p fsm.ExecutorParams, operator ops.Operator) (fsm.PhaseExecutor, error) {
	logger := &fsm.Logger{
		FieldLogger: logrus.WithField(constants.FieldPhase, p.Phase.ID),
		Key:         p.Key(),
		Operator:    operator,
		Server:      p.Phase.Data.Server,
	}
	operation, err := operator.GetSiteOperation(p.Key())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &stateExecutor{
		FieldLogger:    logger,
		ExecutorParams: p,
		Operator:       operator,
		Operation:      *operation,
		Server:         *p.Phase.Data.Server,
	}, nil
}

type stateExecutor struct {
	// FieldLogger is used for logging.
	logrus.FieldLogger
	// ExecutorParams are common executor parameters.
	fsm.ExecutorParams
	// Operator is the installer operator.
	Operator ops.Operator
	// Operation is the current reconfigure operation.
	Operation ops.SiteOperation
	// Server is the server undergoing the reconfigure operation.
	Server storage.Server
}

// Execute updates the server information in the cluster state.
func (p *stateExecutor) Execute(ctx context.Context) error {
	p.Progress.NextStep("Updating cluster state")
	clusterEnv, err := localenv.NewClusterEnvironment()
	if err != nil {
		return trace.Wrap(err)
	}
	if err := p.updateNode(clusterEnv.Backend); err != nil {
		return trace.Wrap(err)
	}
	if err := p.createOperation(clusterEnv.Backend); err != nil {
		return trace.Wrap(err)
	}
	if err := p.removeAuthorities(clusterEnv.Backend); err != nil {
		return trace.Wrap(err)
	}
	if err := p.updateTokens(clusterEnv.Backend, clusterEnv.Operator); err != nil {
		return trace.Wrap(err)
	}
	if err := p.updatePeers(clusterEnv.Backend); err != nil {
		return trace.Wrap(err)
	}
	if err := p.updateBlobs(clusterEnv.Backend); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (p *stateExecutor) updateNode(backend storage.Backend) error {
	cluster, err := backend.GetLocalSite(defaults.SystemAccountID)
	if err != nil {
		return trace.Wrap(err)
	}
	cluster.ClusterState.Servers = storage.Servers{p.Server}
	_, err = backend.UpdateSite(*cluster)
	if err != nil {
		return trace.Wrap(err)
	}
	p.Debug("Updated node in the cluster state.")
	return nil
}

func (p *stateExecutor) updateTokens(backend storage.Backend, operator ops.Operator) error {
	existingToken, err := operator.GetExpandToken(p.Key().SiteKey())
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	if existingToken != nil {
		err = backend.DeleteProvisioningToken(existingToken.Token)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	token, err := p.Operator.GetExpandToken(p.Key().SiteKey())
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = backend.CreateProvisioningToken(*token)
	if err != nil && !trace.IsAlreadyExists(err) {
		return trace.Wrap(err)
	}
	p.Debug("Updated provisioning tokens.")
	return nil
}

func (p *stateExecutor) updatePeers(backend storage.Backend) error {
	peers, err := backend.GetPeers()
	if err != nil {
		return trace.Wrap(err)
	}
	for _, peer := range peers {
		if peer.ID != p.Server.AdvertiseIP {
			err := backend.DeletePeer(peer.ID)
			if err != nil {
				return trace.Wrap(err)
			}
			p.Debugf("Deleted peer %v.", peer)
		}
	}
	return nil
}

func (p *stateExecutor) updateBlobs(backend storage.Backend) error {
	blobs, err := backend.GetObjects()
	if err != nil {
		return trace.Wrap(err)
	}
	for _, blob := range blobs {
		peers, err := backend.GetObjectPeers(blob)
		if err != nil {
			return trace.Wrap(err)
		}
		err = backend.DeleteObjectPeers(blob, peers)
		if err != nil {
			return trace.Wrap(err)
		}
		err = backend.UpsertObjectPeers(blob, []string{p.Server.AdvertiseIP}, 0)
		if err != nil {
			return trace.Wrap(err)
		}
		p.Debugf("Updated peers for object %v.", blob)
	}
	return nil
}

func (p *stateExecutor) removeAuthorities(backend storage.Backend) error {
	for _, authType := range []services.CertAuthType{services.HostCA, services.UserCA} {
		authID := services.CertAuthID{
			Type:       authType,
			DomainName: p.Plan.ClusterName,
		}
		err := backend.DeleteCertAuthority(authID)
		if err != nil && !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		p.Debugf("Removed %s from the cluster state.", authID)
	}
	return nil
}

func (p *stateExecutor) createOperation(backend storage.Backend) error {
	operation := storage.SiteOperation(p.Operation)
	operation.State = ops.OperationStateCompleted
	_, err := backend.CreateSiteOperation(operation)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = backend.CreateProgressEntry(storage.ProgressEntry{
		SiteDomain:  operation.SiteDomain,
		OperationID: operation.ID,
		Created:     time.Now().UTC(),
		Completion:  constants.Completed,
		State:       ops.ProgressStateCompleted,
		Message:     "Operation has completed",
	})
	if err != nil {
		return trace.Wrap(err)
	}
	plan := p.Plan
	fsm.MarkCompleted(&plan)
	_, err = backend.CreateOperationPlan(plan)
	if err != nil {
		return trace.Wrap(err)
	}
	p.Debug("Created operation and plan in the cluster state.")
	return nil
}

// Rollback is no-op for this phase.
func (*stateExecutor) Rollback(ctx context.Context) error {
	return nil
}

// PreCheck is no-op for this phase.
func (*stateExecutor) PreCheck(ctx context.Context) error {
	return nil
}

// PostCheck is no-op for this phase.
func (*stateExecutor) PostCheck(ctx context.Context) error {
	return nil
}
