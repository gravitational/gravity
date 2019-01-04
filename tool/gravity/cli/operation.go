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
	"github.com/sirupsen/logrus"
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
	op, err := getActiveOperation(localEnv, updateEnv, joinEnv, operationID)
	if err != nil {
		return trace.Wrap(err)
	}
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

func rollbackPhase(localEnv, updateEnv, joinEnv *localenv.LocalEnvironment, operationID string, params PhaseParams) error {
	op, err := getActiveOperation(localEnv, updateEnv, joinEnv, operationID)
	if err != nil {
		return trace.Wrap(err)
	}
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

func getLastOperation(localEnv, updateEnv, joinEnv *localenv.LocalEnvironment, operationID string) (*ops.SiteOperation, error) {
	operations := getBackendOperations(localEnv, updateEnv, joinEnv, operationID)
	if len(operations) == 0 {
		if operationID != "" {
			return nil, trace.NotFound("no operation with ID %v found", operationID)
		}
		return nil, trace.NotFound("no operation found")
	}
	op, err := getActiveOperationFromList(operations)
	if err != nil {
		log.WithError(err).Warn("Failed to find active operation, will fall back to last completed.")
	}
	if op == nil {
		if len(operations) != 1 {
			return nil, trace.BadParameter("multiple operations found: \n%v\n, please specify operation with --operation-id",
				formatOperations(operations))
		}
		op = &operations[0]
	}
	return op, nil
}

func getActiveOperation(localEnv, updateEnv, joinEnv *localenv.LocalEnvironment, operationID string) (*ops.SiteOperation, error) {
	operations := getBackendOperations(localEnv, updateEnv, joinEnv, operationID)
	if len(operations) == 0 {
		if operationID != "" {
			return nil, trace.NotFound("no operation with ID %v found", operationID)
		}
		return nil, trace.NotFound("no operation found")
	}
	op, err := getActiveOperationFromList(operations)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return op, nil
}

// getBackendOperations returns the list of operation from the specified backends
func getBackendOperations(localEnv, updateEnv, joinEnv *localenv.LocalEnvironment, operationID string) (result []ops.SiteOperation) {
	// operationID -> operation
	operations := make(map[string]ops.SiteOperation)
	var clusterOperation *ops.SiteOperation
	isOngoingInstallOperation := func() bool {
		return clusterOperation == nil ||
			(clusterOperation.Type == ops.OperationInstall && !clusterOperation.IsCompleted())
	}
	getBackendOperation := func(backend storage.Backend, ctx string) *ops.SiteOperation {
		op, err := storage.GetLastOperation(backend)
		if err == nil {
			if _, exists := operations[op.ID]; !exists {
				operations[op.ID] = (ops.SiteOperation)(*op)
			}
		} else {
			log.WithFields(logrus.Fields{
				"context":       ctx,
				logrus.ErrorKey: err,
			}).Debug("Failed to query operation.")
		}
		return (*ops.SiteOperation)(op)
	}
	clusterEnv, err := localEnv.NewClusterEnvironmentWithTimeout(1 * time.Second)
	if err != nil {
		log.WithError(err).Debug("Failed to create cluster environment.")
	}
	if clusterEnv != nil {
		clusterOperation = getBackendOperation(clusterEnv.Backend, "cluster")
	}
	if updateEnv != nil {
		getBackendOperation(updateEnv.Backend, "update")
	}
	if joinEnv != nil {
		getBackendOperation(joinEnv.Backend, "expand")
	}
	// Only fetch operation from remote (install) environment if the install operation is ongoing
	// or we failed to fetch the operation details from the cluster
	if isOngoingInstallOperation() {
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
		if operationID == "" || operationID == op.ID {
			result = append(result, op)
		}
	}
	return result
}

func getActiveOperationFromList(operations []ops.SiteOperation) (*ops.SiteOperation, error) {
	for _, op := range operations {
		if !op.IsCompleted() {
			return &op, nil
		}
	}
	return nil, trace.NotFound("no active operations found")
}

func formatOperations(operations []ops.SiteOperation) string {
	var formats []string
	for _, op := range operations {
		formats = append(formats, fmt.Sprintf("operation(id=%v, type=%v)", op.ID, op.Type))
	}
	return strings.Join(formats, "\n")
}
