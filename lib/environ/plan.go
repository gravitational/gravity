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

package environ

import (
	"github.com/gravitational/gravity/lib/environ/internal/fsm"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/trace"
)

func (r *Updater) getOrCreateOperationPlan() (plan *storage.OperationPlan, err error) {
	plan, err = r.Operator.GetOperationPlan(r.Operation.Key())
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}

	if trace.IsNotFound(err) {
		plan, err = fsm.NewOperationPlan(*r.Operation, r.Servers)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		err = r.Operator.CreateOperationPlan(r.Operation.Key(), *plan)
		if err != nil {
			if trace.IsNotFound(err) {
				return nil, trace.NotImplemented(
					"cluster operator does not implement the API required for update environment variables. " +
						"Please make sure you're running the command on a compatible cluster.")
			}
			return nil, trace.Wrap(err)
		}
	}

	return plan, nil
}
