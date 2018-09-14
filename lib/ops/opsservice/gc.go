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

package opsservice

import (
	"github.com/gravitational/gravity/lib/ops"

	"github.com/gravitational/trace"
	"github.com/pborman/uuid"
)

// createGarbageCollectOperation creates a new garbage collection operation in the cluster
func (s *site) createGarbageCollectOperation(req ops.CreateClusterGarbageCollectOperationRequest) (*ops.SiteOperationKey, error) {
	_, err := ops.GetCompletedInstallOperation(s.key, s.service)
	if err != nil {
		return nil, trace.Wrap(err, "garbage collection can only be started on an installed cluster")
	}

	op := ops.SiteOperation{
		ID:         uuid.New(),
		AccountID:  s.key.AccountID,
		SiteDomain: s.key.SiteDomain,
		Type:       ops.OperationGarbageCollect,
		Created:    s.clock().UtcNow(),
		Updated:    s.clock().UtcNow(),
		State:      ops.OperationGarbageCollectInProgress,
	}

	key, err := s.getOperationGroup().createSiteOperation(op)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return key, nil
}
