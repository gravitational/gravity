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

package cli

import (
	"sort"
	"strings"
	"time"

	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// PhaseParams is a set of parameters for a single phase execution
type PhaseParams struct {
	// PhaseID is the ID of the phase to execute
	PhaseID string
	// OperationID specifies the operation to work with.
	// If unspecified, last operation is used.
	// Some commands will require the last operation to also be active
	OperationID string
	// Force allows to force phase execution
	Force bool
	// Timeout is phase execution timeout
	Timeout time.Duration
	// SkipVersionCheck overrides the verification of binary version compatibility
	SkipVersionCheck bool
}

func executePhase(localEnv, updateEnv, joinEnv *localenv.LocalEnvironment, params PhaseParams) error {
	op, err := getActiveOperation(localEnv, updateEnv, joinEnv, params.OperationID)
	if err != nil {
		return trace.Wrap(err)
	}
	switch op.Type {
	case ops.OperationInstall:
		return executeInstallPhase(localEnv, params, op)
	case ops.OperationExpand:
		return executeJoinPhase(localEnv, joinEnv, params, op)
	case ops.OperationUpdate:
		return executeUpdatePhase(localEnv, updateEnv, params, *op)
	case ops.OperationUpdateRuntimeEnviron:
		return executeEnvironPhase(localEnv, updateEnv, params, *op)
	case ops.OperationUpdateConfig:
		return executeConfigPhase(localEnv, updateEnv, params, *op)
	case ops.OperationGarbageCollect:
		return executeGarbageCollectPhase(localEnv, params, op)
	default:
		return trace.BadParameter("operation type %q does not support plan execution", op.Type)
	}
}

func rollbackPhase(localEnv, updateEnv, joinEnv *localenv.LocalEnvironment, params PhaseParams) error {
	op, err := getActiveOperation(localEnv, updateEnv, joinEnv, params.OperationID)
	if err != nil {
		return trace.Wrap(err)
	}
	switch op.Type {
	case ops.OperationInstall:
		return rollbackInstallPhase(localEnv, params, op)
	case ops.OperationExpand:
		return rollbackJoinPhase(localEnv, joinEnv, params, op)
	case ops.OperationUpdate:
		return rollbackUpdatePhase(localEnv, updateEnv, params, *op)
	case ops.OperationUpdateRuntimeEnviron:
		return rollbackEnvironPhase(localEnv, updateEnv, params, *op)
	case ops.OperationUpdateConfig:
		return rollbackConfigPhase(localEnv, updateEnv, params, *op)
	default:
		return trace.BadParameter("operation type %q does not support plan rollback", op.Type)
	}
}

func completeOperationPlan(localEnv, updateEnv, joinEnv *localenv.LocalEnvironment, operationID string) error {
	op, err := getActiveOperation(localEnv, updateEnv, joinEnv, operationID)
	if err != nil {
		return trace.Wrap(err)
	}
	switch op.Type {
	case ops.OperationInstall:
		// There's only one install operation
		return completeInstallPlan(localEnv, op)
	case ops.OperationExpand:
		return completeJoinPlan(localEnv, joinEnv, op)
	case ops.OperationUpdate:
		return completeUpdatePlan(localEnv, updateEnv, *op)
	case ops.OperationUpdateRuntimeEnviron:
		return completeEnvironPlan(localEnv, updateEnv, *op)
	case ops.OperationUpdateConfig:
		return completeConfigPlan(localEnv, updateEnv, *op)
	default:
		return trace.BadParameter("operation type %q does not support plan completion", op.Type)
	}
}

func getLastOperation(localEnv, updateEnv, joinEnv *localenv.LocalEnvironment, operationID string) (*ops.SiteOperation, error) {
	operations, err := getBackendOperations(localEnv, updateEnv, joinEnv, operationID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	log.WithField("operations", oplist(operations).String()).Debug("Fetched backend operations.")
	if len(operations) == 0 {
		if operationID != "" {
			return nil, trace.NotFound("no operation with ID %v found", operationID)
		}
		return nil, trace.NotFound("no operation found")
	}
	if len(operations) == 1 && operationID != "" {
		log.WithField("operation", operations[0]).Debug("Fetched operation by ID.")
		return &operations[0], nil
	}
	log.Infof("Multiple operations found: \n%v\n, please specify operation with --operation-id.\n"+
		"Displaying the most recent operation.",
		oplist(operations))
	return &operations[0], nil
}

func getActiveOperation(localEnv, updateEnv, joinEnv *localenv.LocalEnvironment, operationID string) (*ops.SiteOperation, error) {
	operations, err := getBackendOperations(localEnv, updateEnv, joinEnv, operationID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	log.WithField("operations", oplist(operations).String()).Debug("Fetched backend operations.")
	if len(operations) == 0 {
		if operationID != "" {
			return nil, trace.NotFound("no operation with ID %v found", operationID)
		}
		return nil, trace.NotFound("no operation found")
	}
	if len(operations) == 1 && operationID != "" {
		log.WithField("operation", operations[0]).Debug("Fetched operation by ID.")
		return &operations[0], nil
	}
	op, err := getActiveOperationFromList(operations)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return op, nil
}

// getBackendOperations returns the list of operation from the specified backends
// in descending order (sorted by creation time)
func getBackendOperations(localEnv, updateEnv, joinEnv *localenv.LocalEnvironment, operationID string) (result []ops.SiteOperation, err error) {
	b := newBackendOperations()
	err = b.List(localEnv, updateEnv, joinEnv)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, op := range b.operations {
		if operationID == "" || operationID == op.ID {
			result = append(result, op)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Created.After(result[j].Created)
	})
	return result, nil
}

func newBackendOperations() backendOperations {
	return backendOperations{
		operations: make(map[string]ops.SiteOperation),
	}
}

func (r *backendOperations) List(localEnv, updateEnv, joinEnv *localenv.LocalEnvironment) error {
	clusterEnv, err := localEnv.NewClusterEnvironment(localenv.WithEtcdTimeout(1 * time.Second))
	if err != nil {
		log.WithError(err).Debug("Failed to create cluster environment.")
	}
	if clusterEnv != nil {
		err = r.init(clusterEnv.Backend)
		if err != nil {
			log.WithError(err).Warn("Failed to query cluster operations.")
		}
	}
	if updateEnv != nil {
		r.getOperationAndUpdateCache(updateEnv.Backend, log.WithField("context", "update"))
	}
	if joinEnv != nil {
		r.getOperationAndUpdateCache(joinEnv.Backend, log.WithField("context", "expand"))
	}
	// Only fetch operation from remote (install) environment if the install operation is ongoing
	// or we failed to fetch the operation details from the cluster
	if r.isActiveInstallOperation() {
		wizardEnv, err := localenv.NewRemoteEnvironment()
		if err != nil {
			log.WithError(err).Warn("Failed to create wizard environment.")
		}
		if wizardEnv != nil && wizardEnv.Operator != nil {
			op, err := ops.GetWizardOperation(wizardEnv.Operator)
			if err == nil {
				log.Debug("Fetched install operation from wizard environment.")
				r.operations[op.ID] = (ops.SiteOperation)(*op)
			} else {
				log.WithError(err).Warn("Failed to query install operation.")
			}
		}
	}
	return nil
}

func (r *backendOperations) init(clusterBackend storage.Backend) error {
	clusterOperations, err := storage.GetOperations(clusterBackend)
	if err != nil {
		return trace.Wrap(err, "failed to query cluster operations")
	}
	if len(clusterOperations) == 0 {
		return nil
	}
	// Initialize the operation state from the list of existing cluster operations
	for _, op := range clusterOperations {
		r.operations[op.ID] = (ops.SiteOperation)(op)
	}
	r.clusterOperation = (*ops.SiteOperation)(&clusterOperations[0])
	r.operations[r.clusterOperation.ID] = *r.clusterOperation
	return nil
}

func (r *backendOperations) getOperationAndUpdateCache(backend storage.Backend, logger logrus.FieldLogger) *ops.SiteOperation {
	op, err := storage.GetLastOperation(backend)
	if err == nil {
		// Operation from the backend takes precedence over the existing operation (from cluster state)
		r.operations[op.ID] = (ops.SiteOperation)(*op)
	} else {
		logger.WithError(err).Warn("Failed to query operation.")
	}
	return (*ops.SiteOperation)(op)
}

func (r backendOperations) isActiveInstallOperation() bool {
	// FIXME: continue using wizard as source of truth as operation state
	// replicated in etcd is reported completed before it actually is
	return r.clusterOperation == nil || (r.clusterOperation.Type == ops.OperationInstall)
}

type backendOperations struct {
	operations       map[string]ops.SiteOperation
	clusterOperation *ops.SiteOperation
}

func getActiveOperationFromList(operations []ops.SiteOperation) (*ops.SiteOperation, error) {
	for _, op := range operations {
		if !op.IsCompleted() {
			return &op, nil
		}
	}
	return nil, trace.NotFound("no active operations found")
}

func (r oplist) String() string {
	var ops []string
	for _, op := range r {
		ops = append(ops, op.String())
	}
	return strings.Join(ops, "\n")
}

type oplist []ops.SiteOperation
