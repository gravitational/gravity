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
	"sort"
	"strings"
	"time"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/tool/common"

	"github.com/buger/goterm"
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
	// DryRun allows to only print execute/rollback phases
	DryRun bool
	// Block indicates whether the command should be run in foreground or as a systemd unit
	Block bool
}

func (p PhaseParams) toFSM() fsm.Params {
	return fsm.Params{
		PhaseID: p.PhaseID,
		Force:   p.Force,
		DryRun:  p.DryRun,
	}
}

func executePhase(localEnv *localenv.LocalEnvironment, environ LocalEnvironmentFactory, params PhaseParams) error {
	operation, err := getActiveOperation(localEnv, environ, params.OperationID)
	if err != nil {
		return trace.Wrap(err)
	}
	op := operation.SiteOperation
	switch op.Type {
	case ops.OperationInstall:
		return executeInstallPhaseForOperation(localEnv, params, op)
	case ops.OperationExpand:
		return executeJoinPhaseForOperation(localEnv, environ, params, op)
	case ops.OperationUpdate:
		return executeUpdatePhaseForOperation(localEnv, environ, params, op)
	case ops.OperationUpdateRuntimeEnviron:
		return executeEnvironPhaseForOperation(localEnv, environ, params, op)
	case ops.OperationUpdateConfig:
		return executeConfigPhaseForOperation(localEnv, environ, params, op)
	case ops.OperationGarbageCollect:
		return executeGarbageCollectPhaseForOperation(localEnv, params, op)
	default:
		return trace.BadParameter("operation type %q does not support plan execution", op.Type)
	}
}

const (
	// planRollbackWarning is shown when "gravity rollback" command is launched
	// without --confirm flag.
	planRollbackWarning = `You are about to rollback the following operation:

%v
Consider checking the operation plan and using --dry-run flag first to see which actions will be performed.
You can suppress this warning in future by providing --confirm flag.

`
	// unsupportedRollbackWarning is shown for operations that "gravity rollback"
	// command does not support.
	unsupportedRollbackWarning = `Operation %q does not support automatic rollback.
Please use "gravity plan rollback" command to rollback individual phases.`
)

func rollbackPlan(localEnv *localenv.LocalEnvironment, environ LocalEnvironmentFactory, params PhaseParams, confirmed bool) error {
	operation, err := getActiveOperation(localEnv, environ, params.OperationID)
	if err != nil {
		return trace.Wrap(err)
	}
	op := operation.SiteOperation
	switch op.Type {
	case ops.OperationUpdate, ops.OperationUpdateRuntimeEnviron, ops.OperationUpdateConfig:
	default:
		return trace.BadParameter(unsupportedRollbackWarning, op.TypeString())
	}
	if !confirmed && !params.DryRun {
		localEnv.Printf(planRollbackWarning, operationList([]clusterOperation{*operation}).formatTable())
		if err := enforceConfirmation("Proceed?"); err != nil {
			return trace.Wrap(err)
		}
	}
	params.PhaseID = fsm.RootPhase
	switch op.Type {
	case ops.OperationUpdate:
		err = rollbackUpdatePhaseForOperation(localEnv, environ, params, op)
	case ops.OperationUpdateRuntimeEnviron:
		err = rollbackEnvironPhaseForOperation(localEnv, environ, params, op)
	case ops.OperationUpdateConfig:
		err = rollbackConfigPhaseForOperation(localEnv, environ, params, op)
	default:
		return trace.BadParameter(unsupportedRollbackWarning, op.TypeString())
	}
	if err != nil {
		return trace.Wrap(err)
	}
	// Make sure to reset the cluster state after the operation has been
	// fully rolled back.
	if !params.DryRun {
		return completeOperationPlanForOperation(localEnv, environ, op)
	}
	return nil
}

func rollbackPhase(localEnv *localenv.LocalEnvironment, environ LocalEnvironmentFactory, params PhaseParams) error {
	operation, err := getActiveOperation(localEnv, environ, params.OperationID)
	if err != nil {
		return trace.Wrap(err)
	}
	op := operation.SiteOperation
	switch op.Type {
	case ops.OperationInstall:
		return rollbackInstallPhaseForOperation(localEnv, params, op)
	case ops.OperationExpand:
		return rollbackJoinPhaseForOperation(localEnv, environ, params, op)
	case ops.OperationUpdate:
		return rollbackUpdatePhaseForOperation(localEnv, environ, params, op)
	case ops.OperationUpdateRuntimeEnviron:
		return rollbackEnvironPhaseForOperation(localEnv, environ, params, op)
	case ops.OperationUpdateConfig:
		return rollbackConfigPhaseForOperation(localEnv, environ, params, op)
	default:
		return trace.BadParameter("operation type %q does not support plan rollback", op.Type)
	}
}

func completeOperationPlan(localEnv *localenv.LocalEnvironment, environ LocalEnvironmentFactory, operationID string) error {
	operation, err := getActiveOperation(localEnv, environ, operationID)
	if err != nil {
		return trace.Wrap(err)
	}
	err = completeOperationPlanForOperation(localEnv, environ, operation.SiteOperation)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func completeOperationPlanForOperation(localEnv *localenv.LocalEnvironment, environ LocalEnvironmentFactory, op ops.SiteOperation) (err error) {
	switch op.Type {
	case ops.OperationInstall:
		// There's only one install operation
		err = completeInstallPlanForOperation(localEnv, op)
	case ops.OperationExpand:
		err = completeJoinPlanForOperation(localEnv, environ, op)
	case ops.OperationUpdate:
		err = completeUpdatePlanForOperation(localEnv, environ, op)
	case ops.OperationUpdateRuntimeEnviron:
		err = completeEnvironPlanForOperation(localEnv, environ, op)
	case ops.OperationUpdateConfig:
		err = completeConfigPlanForOperation(localEnv, environ, op)
	default:
		return completeClusterOperationPlan(localEnv, op)
	}
	if op.Type != ops.OperationInstall && trace.IsNotFound(err) {
		log.WithError(err).Warn("Failed to complete operation locally, will fallback to cluster operation.")
		return completeClusterOperationPlan(localEnv, op)
	}
	return trace.Wrap(err)
}

func completeClusterOperationPlan(localEnv *localenv.LocalEnvironment, operation ops.SiteOperation) error {
	clusterEnv, err := localEnv.NewClusterEnvironment()
	if err != nil {
		return trace.Wrap(err)
	}
	plan, err := fsm.GetOperationPlan(clusterEnv.Backend, operation.SiteDomain, operation.ID)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	if err == nil && fsm.IsCompleted(plan) {
		return ops.CompleteOperation(operation.Key(), clusterEnv.Operator)
	}
	return ops.FailOperation(operation.Key(), clusterEnv.Operator, "completed manually")
}

// CheckInstallOperationComplete verifies whether there's a completed install operation.
// Returns nil if there is a completed install operation
func CheckInstallOperationComplete(localEnv *localenv.LocalEnvironment) error {
	operations, err := getBackendOperations(localEnv, nil, "")
	if err != nil {
		return trace.Wrap(err)
	}
	log.WithField("operations", operationList(operations).String()).Debug("Fetched backend operations.")
	if len(operations) == 0 {
		return trace.NotFound("no install operation found")
	}
	firstOperation := operations[len(operations)-1]
	if firstOperation.Type == ops.OperationInstall && firstOperation.IsCompleted() {
		return nil
	}
	return trace.NotFound("no install operation found")
}

// getLastOperation returns the last operation found across the specified backends.
// If no operation is found, the returned error will indicate a not found operation
func getLastOperation(localEnv *localenv.LocalEnvironment, environ LocalEnvironmentFactory, operationID string) (*clusterOperation, error) {
	operations, err := getBackendOperations(localEnv, environ, operationID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	log.WithField("operations", operationList(operations).String()).Debug("Fetched backend operations.")
	if len(operations) == 0 {
		if operationID != "" {
			return nil, trace.NotFound("no operation with ID %v found", operationID)
		}
		return nil, trace.NotFound("no operation found")
	}
	return &operations[0], nil
}

func getActiveOperation(localEnv *localenv.LocalEnvironment, environ LocalEnvironmentFactory, operationID string) (*clusterOperation, error) {
	operation, err := getLastOperation(localEnv, environ, operationID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if operation.IsCompleted() {
		return nil, trace.NotFound("no active operation found")
	}
	return operation, nil
}

// getBackendOperations returns the list of operation from the specified backends
// in descending order (sorted by creation time)
func getBackendOperations(localEnv *localenv.LocalEnvironment, environ LocalEnvironmentFactory, operationID string) (result []clusterOperation, err error) {
	b := newBackendOperations()
	b.List(localEnv, environ)
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
		operations: make(map[string]clusterOperation),
	}
}

func (r *backendOperations) List(localEnv *localenv.LocalEnvironment, environ LocalEnvironmentFactory) {
	clusterEnv, err := localEnv.NewClusterEnvironment(localenv.WithEtcdTimeout(1 * time.Second))
	if err != nil {
		log.WithError(err).Debug("Failed to create cluster environment.")
	}
	if clusterEnv != nil {
		err = r.init(clusterEnv.Backend)
		if err != nil {
			log.WithError(err).Debug("Failed to query cluster operations.")
		}
	}
	if environ == nil {
		return
	}
	// List operation from a local state store.
	// This is required in cases when the cluster store is inaccessible (like during upgrades)
	if err := r.listUpdateOperation(environ); err != nil && !trace.IsNotFound(err) {
		log.WithError(err).Warn("Failed to list update operation.")
	}
	if err := r.listJoinOperation(environ); err != nil && !trace.IsNotFound(err) {
		log.WithError(err).Warn("Failed to list expand operation.")
	}
	// Only fetch operation from remote (install) environment if the install operation is ongoing
	// or we failed to fetch the operation details from the cluster
	if r.isActiveInstallOperation() {
		r.listInstallOperation()
	}
}

func (r *backendOperations) init(clusterBackend storage.Backend) error {
	clusterOperations, err := storage.GetOperations(clusterBackend)
	if err != nil {
		return trace.Wrap(err, "failed to query cluster operations")
	}
	if len(clusterOperations) == 0 {
		return nil
	}
	for _, op := range clusterOperations {
		clusterOperation := clusterOperation{
			SiteOperation: (ops.SiteOperation)(op),
		}
		if _, err := clusterBackend.GetOperationPlan(op.SiteDomain, op.ID); err == nil {
			clusterOperation.hasPlan = true
		}
		r.operations[op.ID] = clusterOperation
	}
	latestOperation := r.operations[clusterOperations[0].ID]
	r.clusterOperation = &latestOperation
	return nil
}

func (r *backendOperations) listUpdateOperation(environ LocalEnvironmentFactory) error {
	env, err := environ.NewUpdateEnv()
	if err != nil {
		return trace.Wrap(err)
	}
	defer env.Close()
	r.updateOperationInCache(env.Backend, log.WithField("context", "update"))
	return nil
}

func (r *backendOperations) listJoinOperation(environ LocalEnvironmentFactory) error {
	env, err := environ.NewJoinEnv()
	if err != nil && !trace.IsConnectionProblem(err) {
		return trace.Wrap(err)
	}
	if env == nil {
		// Do not fail for timeout errors.
		// Timeout error means the directory is used by the active installer process
		// which means, it's the installer environment, not joining node's
		return nil
	}
	defer env.Close()
	r.updateOperationInCache(env.Backend, log.WithField("context", "expand"))
	return nil
}

func (r *backendOperations) listInstallOperation() {
	wizardEnv, err := localenv.NewRemoteEnvironment()
	if err != nil {
		log.WithError(err).Warn("Failed to create wizard environment.")
		return
	}
	if wizardEnv.Operator == nil {
		return
	}
	op, err := ops.GetWizardOperation(wizardEnv.Operator)
	if err != nil {
		log.WithError(err).Warn("Failed to query install operation.")
		return
	}
	clusterOperation := clusterOperation{
		SiteOperation: (ops.SiteOperation)(*op),
	}
	if _, err := wizardEnv.Operator.GetOperationPlan(op.Key()); err == nil {
		clusterOperation.hasPlan = true
	}
	log.Debug("Fetched install operation from wizard environment.")
	r.operations[op.ID] = clusterOperation
}

func (r *backendOperations) updateOperationInCache(backend storage.Backend, logger logrus.FieldLogger) {
	op, err := storage.GetLastOperation(backend)
	if err != nil {
		if !trace.IsNotFound(err) {
			logger.WithError(err).Warn("Failed to query operation.")
		}
		return
	}
	clusterOperation := clusterOperation{
		SiteOperation: (ops.SiteOperation)(*op),
	}
	if _, err := backend.GetOperationPlan(op.SiteDomain, op.ID); err == nil {
		clusterOperation.hasPlan = true
	}
	// Operation from the backend takes precedence over the existing operation (from cluster state)
	r.operations[op.ID] = clusterOperation
}

func (r backendOperations) isActiveInstallOperation() bool {
	// Bail out if there's an operation from a local backend and we failed to query
	// cluster operations.
	// It cannot be an install operation as wizard has not been queried yet
	if r.clusterOperation == nil && len(r.operations) != 0 {
		return false
	}
	// Otherwise, consider this to be an install operation if:
	//  - we failed to fetch any operation (either from cluster or local storage)
	//  - we fetched operation(s) from cluster storage and the most recent one is an install operation
	//
	// FIXME: continue using wizard as source of truth as operation state
	// replicated in etcd is reported completed before it actually is
	return r.clusterOperation == nil || (r.clusterOperation.Type == ops.OperationInstall)
}

type backendOperations struct {
	// operations lists currently detected operations.
	// Operations are queried over a variety of backends due to disparity of state storage
	// locations (including cluster state store).
	// Operations found outside the cluster state store (etcd) are considered to be
	// more up-to-date and take precedence.
	operations map[string]clusterOperation
	// clusterOperation stores the first operation found in cluster state store (if any)
	clusterOperation *clusterOperation
}

func isInvalidOperation(op clusterOperation) bool {
	switch op.Type {
	case ops.OperationShrink:
		return false
	default:
		return !op.hasPlan
	}
}

// formatTable formats this operation list as a table
func (r operationList) formatTable() string {
	t := goterm.NewTable(0, 10, 5, ' ', 0)
	common.PrintTableHeader(t, []string{"Type", "ID", "State", "Created"})
	for _, op := range r {
		fmt.Fprintf(t, "%v\t%v\t%v\t%v\n",
			op.Type, op.ID, op.State, op.Created.Format(constants.ShortDateFormat))
	}
	return t.String()
}

func (r operationList) String() string {
	var ops []string
	for _, op := range r {
		ops = append(ops, op.String())
	}
	return strings.Join(ops, "\n")
}

type operationList []clusterOperation

type clusterOperation struct {
	ops.SiteOperation
	hasPlan bool
}
