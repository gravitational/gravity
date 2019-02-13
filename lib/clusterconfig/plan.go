/*
Copyright 2019 Gravitational, Inc.

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

package clusterconfig

import (
	"github.com/gravitational/gravity/lib/clusterconfig/internal/fsm"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/trace"
)

// NewOperationPlan creates a new operation plan for the specified operation
func NewOperationPlan(operator ops.Operator, operation ops.SiteOperation, servers []storage.Server) (plan *storage.OperationPlan, err error) {
	cluster, err := operator.GetLocalSite()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	plan, err = fsm.NewOperationPlan(cluster.App.Package, cluster.DNSConfig, operation, servers)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = operator.CreateOperationPlan(operation.Key(), *plan)
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotImplemented(
				"cluster operator does not implement the API required to update cluster configuration. " +
					"Please make sure you're running the command on a compatible cluster.")
		}
		return nil, trace.Wrap(err)
	}
	return plan, nil
}

func getOrCreateOperationPlan(operator ops.Operator, operation ops.SiteOperation, servers []storage.Server) (plan *storage.OperationPlan, err error) {
	plan, err = operator.GetOperationPlan(operation.Key())
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	if trace.IsNotFound(err) {
		plan, err = NewOperationPlan(operator, operation, servers)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return plan, nil
}
