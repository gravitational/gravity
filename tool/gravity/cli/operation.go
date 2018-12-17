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
	"time"

	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/ops"

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

func executeOperation(localEnv, updateEnv, joinEnv *localenv.LocalEnvironment, operationID string, params PhaseParams) error {
	if joinEnv != nil && hasExpandOperation(joinEnv) {
		return executeJoinPhase(localEnv, joinEnv, params)
	}
	if hasUpdateOperation(updateEnv) {
		return executeUpgradePhase(localEnv, updateEnv, params)
	}
	op, err := getOperationFromEnv(localEnv, operationID)
	if err != nil {
		return trace.Wrap(err)
	}
	switch op.Type {
	case ops.OperationInstall:
		return executeInstallPhase(localEnv, params)
	case ops.OperationGarbageCollect:
		return garbageCollectPhase(localEnv, params)
	case ops.OperationUpdateEnvars:
		return updateEnvarsPhase(localEnv, params)
	default:
		return trace.BadParameter("operation type %q does not support phase execution",
			op.Type)
	}
	return nil
}

func rollbackOperation(localEnv, updateEnv, joinEnv *localenv.LocalEnvironment, operationID string, params PhaseParams) error {
	if joinEnv != nil && hasExpandOperation(joinEnv) {
		return rollbackJoinPhase(localEnv, joinEnv, params)
	}
	if hasUpdateOperation(updateEnv) {
		return rollbackUpgradePhase(localEnv, updateEnv, params)
	}
	op, err := getOperationFromEnv(localEnv, operationID)
	if err != nil {
		return trace.Wrap(err)
	}
	switch op.Type {
	case ops.OperationInstall:
		return rollbackInstallPhase(localEnv, params)
	case ops.OperationUpdateEnvars:
		return rollbackUpdateEnvarsPhase(localEnv, params)
	default:
		return trace.BadParameter("operation type %q does not support phase rollback",
			op.Type)
	}
	return nil
}

func getOperationFromEnv(localEnv *localenv.LocalEnvironment, operationID string) (*ops.SiteOperation, error) {
	operator, err := localEnv.SiteOperator()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cluster, err := operator.GetLocalSite()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var op *ops.SiteOperation
	if operationID != "" {
		op, err = operator.GetSiteOperation(ops.SiteOperationKey{
			AccountID:   cluster.AccountID,
			SiteDomain:  cluster.Domain,
			OperationID: operationID,
		})
	} else {
		op, _, err = ops.GetLastOperation(cluster.Key(), operator)
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return op, nil
}
