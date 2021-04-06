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

package opsservice

import (
	"context"

	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/davecgh/go-spew/spew"
	"github.com/gravitational/trace"
	"github.com/pborman/uuid"
)

// CreateClusterReconfigureOperation creates a new cluster reconfiguration operation.
func (o *Operator) CreateClusterReconfigureOperation(ctx context.Context, req ops.CreateClusterReconfigureOperationRequest) (*ops.SiteOperationKey, error) {
	o.Info(spew.NewDefaultConfig().Sdump(req))

	if err := req.Check(); err != nil {
		return nil, trace.Wrap(err)
	}

	cluster, err := o.openSite(req.SiteKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	operation := ops.SiteOperation{
		ID:            uuid.New(),
		AccountID:     req.AccountID,
		SiteDomain:    req.SiteDomain,
		Type:          ops.OperationReconfigure,
		Created:       cluster.clock().UtcNow(),
		CreatedBy:     storage.UserFromContext(ctx),
		Updated:       cluster.clock().UtcNow(),
		State:         ops.OperationReconfigureInProgress,
		Servers:       req.Servers,
		InstallExpand: req.InstallExpand,
		Reconfigure: &storage.ReconfigureOperationState{
			AdvertiseAddr: req.AdvertiseAddr,
		},
	}

	_, err = cluster.newProvisioningToken(operation)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	key, err := cluster.getOperationGroup().createSiteOperation(operation)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return key, nil
}
