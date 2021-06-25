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
)

// ResumeShrink resumes the started shrink operation if the node being shrunk gave up
// its leadership
func (o *Operator) ResumeShrink(key ops.SiteKey) (*ops.SiteOperationKey, error) {
	site, err := o.openSite(ops.SiteKey{AccountID: key.AccountID, SiteDomain: key.SiteDomain})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	opKey, err := site.resumeShrink()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return opKey, nil
}

func (s *site) resumeShrink() (*ops.SiteOperationKey, error) {
	s.Debug("Resume shrink operation.")

	cluster, err := s.service.GetSite(s.key)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if cluster.State != ops.SiteStateShrinking {
		return nil, trace.NotFound("cluster is not shrinking")
	}

	op, err := ops.GetLastShrinkOperation(s.key, s.service)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	key := op.Key()

	if op.State != ops.OperationStateShrinkInProgress {
		return nil, trace.NotFound("shrink operation is not in progress: %v", op)
	}

	s.WithField("op", op.String()).Debug("Resume shrink operation.")

	ctx, err := s.newOperationContext(*op)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = s.executeOperationWithContext(ctx, s.shrinkOperationStart)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &key, nil
}
