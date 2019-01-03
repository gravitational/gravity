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
	"fmt"
	"strings"
	"time"

	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/trace"
)

// PhaseParams is a set of parameters for a single phase execution
type PhaseParams struct {
	// PhaseID is the ID of the phase to execute
	PhaseID string
	// Force allows to force phase execution
	Force bool
	// Timeout is phase execution timeout
	Timeout time.Duration
	// Complete marks operation complete
	Complete bool
	// SkipVersionCheck overrides the verification of binary version compatibility
	SkipVersionCheck bool
}

func executePhase(localEnv, updateEnv, joinEnv *localenv.LocalEnvironment, operationID string, params PhaseParams) error {
	return dispatchOperation(localEnv, updateEnv, joinEnv, operationID,
		dispatchExecutePhase(localEnv, updateEnv, joinEnv, params))
}

func rollbackPhase(localEnv, updateEnv, joinEnv *localenv.LocalEnvironment, operationID string, params PhaseParams) error {
	return dispatchOperation(localEnv, updateEnv, joinEnv, operationID,
		dispatchRollbackPhase(localEnv, updateEnv, joinEnv, params))
}

func dispatchExecutePhase(localEnv, updateEnv, joinEnv *localenv.LocalEnvironment, params PhaseParams) dispatchFunc {
	return func(op ops.SiteOperation) error {
		switch op.Type {
		case ops.OperationInstall:
			return executeInstallPhase(localEnv, params)
		case ops.OperationExpand:
			return executeJoinPhase(localEnv, joinEnv, params)
		case ops.OperationUpdate:
			return executeUpgradePhase(localEnv, updateEnv, params)
		case ops.OperationUpdateEnvars:
			return updateEnvarsPhase(localEnv, updateEnv, params)
		case ops.OperationGarbageCollect:
			return garbageCollectPhase(localEnv, params)
		default:
			return trace.BadParameter("operation type %q does not support plan execution", op.Type)
		}
	}
}

func dispatchRollbackPhase(localEnv, updateEnv, joinEnv *localenv.LocalEnvironment, params PhaseParams) dispatchFunc {
	return func(op ops.SiteOperation) error {
		switch op.Type {
		case ops.OperationInstall:
			return rollbackInstallPhase(localEnv, params)
		case ops.OperationExpand:
			return rollbackJoinPhase(localEnv, joinEnv, params)
		case ops.OperationUpdate:
			return rollbackUpgradePhase(localEnv, updateEnv, params)
		case ops.OperationUpdateEnvars:
			return rollbackEnvarsPhase(localEnv, updateEnv, params)
		default:
			return trace.BadParameter("operation type %q does not support plan rollback", op.Type)
		}
	}
}

func dispatchOperation(localEnv, updateEnv, joinEnv *localenv.LocalEnvironment, operationID string, dispatch dispatchFunc) error {
	operations := getBackendOperations(localEnv, updateEnv, joinEnv)
	if len(operations) == 0 {
		return trace.NotFound("no operation found")
	}

	op, err := getActiveOperation(operations, operationID)
	if err != nil {
		log.WithError(err).Warn("Failed to find an active operation, will fall back to last.")
	}
	if op == nil {
		if len(operations) != 1 {
			return trace.BadParameter("multiple operations found: \n%v\n, please specify operation with --operation-id",
				formatOperations(operations))
		}
		op = &operations[0]
	}
	if operationID != "" && op.ID != operationID {
		return trace.NotFound("no operation with ID %q found", operationID)
	}

	err = dispatch(*op)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

type dispatchFunc func(ops.SiteOperation) error

// getBackendOperations returns the list of operation from the specified backends
func getBackendOperations(localEnv, updateEnv, joinEnv *localenv.LocalEnvironment) (result []ops.SiteOperation) {
	isOngoingInstallOperation := func(op ops.SiteOperation) bool {
		return op.Type == ops.OperationInstall && !op.IsCompleted()
	}
	clusterEnv, err := localEnv.NewClusterEnvironmentWithTimeout(1 * time.Second)
	if err != nil {
		log.WithError(err).Debug("Failed to create cluster environment.")
	}
	// operationID -> operation
	operations := make(map[string]ops.SiteOperation)
	var clusterOperation *ops.SiteOperation
	if clusterEnv != nil {
		op, err := storage.GetLastOperation(clusterEnv.Backend)
		if err == nil {
			clusterOperation = (*ops.SiteOperation)(op)
			operations[op.ID] = *clusterOperation
		} else {
			log.WithError(err).Debug("Failed to query last cluster operation.")
		}
	}

	if updateEnv != nil {
		op, err := storage.GetLastOperation(updateEnv.Backend)
		if err == nil {
			if _, exists := operations[op.ID]; !exists {
				operations[op.ID] = (ops.SiteOperation)(*op)
			}
		} else {
			log.WithError(err).Debug("Failed to query update operation.")
		}
	}

	if joinEnv != nil {
		op, err := storage.GetLastOperation(joinEnv.Backend)
		if err == nil {
			operations[op.ID] = (ops.SiteOperation)(*op)
		} else {
			log.WithError(err).Debug("Failed to query expand operation.")
		}
	}

	// Only fetch installer operation as long as no cluster operation was created
	// or the install operation is ongoing
	if clusterOperation == nil || isOngoingInstallOperation(*clusterOperation) {
		wizardEnv, err := localenv.NewRemoteEnvironment()
		if err != nil {
			log.WithError(err).Debug("Failed to create wizard environment.")
		}
		if wizardEnv != nil && wizardEnv.Operator != nil {
			op, err := ops.GetWizardOperation(wizardEnv.Operator)
			if err == nil {
				operations[op.ID] = (ops.SiteOperation)(*op)
			} else {
				log.WithError(err).Debug("Failed to query install operation.")
			}
		}
	}

	for _, op := range operations {
		result = append(result, op)
	}
	return result
}

func getActiveOperation(operations []ops.SiteOperation, operationID string) (*ops.SiteOperation, error) {
	if operationID != "" {
		return getOperationWithID(operations, operationID)
	}
	for _, op := range operations {
		if !op.IsCompleted() {
			return &op, nil
		}
	}
	return nil, trace.NotFound("no active operations found")
}

func getOperationWithID(operations []ops.SiteOperation, id string) (*ops.SiteOperation, error) {
	for _, op := range operations {
		if op.ID == id {
			return &op, nil
		}
	}
	return nil, trace.NotFound("no operation with ID %v found", id)
}

func formatOperations(operations []ops.SiteOperation) string {
	var formats []string
	for _, op := range operations {
		formats = append(formats, fmt.Sprintf("operation(id=%v, type=%v)", op.ID, op.Type))
	}
	return strings.Join(formats, "\n")
}
