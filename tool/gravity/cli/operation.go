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
	"context"
	"sort"
	"strings"
	"time"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	installerclient "github.com/gravitational/gravity/lib/install/client"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/system/signals"

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

func (r PhaseParams) isResume() bool {
	return r.PhaseID == fsm.RootPhase
}

// SetPhaseParams contains parameters for setting phase state.
type SetPhaseParams struct {
	// OperationID is an optional ID of the operation the phase belongs to.
	OperationID string
	// PhaseID is ID of the phase to set the state.
	PhaseID string
	// State is the new phase state.
	State string
}

// resumeOperation resumes the operation specified with params
func resumeOperation(localEnv *localenv.LocalEnvironment, environ LocalEnvironmentFactory, params PhaseParams) error {
	err := executePhase(localEnv, environ, PhaseParams{
		PhaseID:          fsm.RootPhase,
		Force:            params.Force,
		Timeout:          params.Timeout,
		SkipVersionCheck: params.SkipVersionCheck,
		OperationID:      params.OperationID,
	})
	if err == nil {
		return nil
	}
	if !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	log.WithError(err).Warn("No operation found - will attempt to restart installation (resume join).")
	return trace.Wrap(restartInstallOrJoin(localEnv))
}

// executePhase executes a phase for the operation specified with params
func executePhase(localEnv *localenv.LocalEnvironment, environ LocalEnvironmentFactory, params PhaseParams) error {
	operations, err := getActiveOperations(localEnv, environ, params.OperationID)
	if err != nil {
		return trace.Wrap(err)
	}
	if len(operations) != 1 {
		log.WithField("operations", oplist(operations)).Warn("Multiple operations found.")
		return trace.BadParameter("multiple operations found. Please specify operation with --operation-id")
	}
	op := operations[0]
	switch op.Type {
	case ops.OperationInstall:
		return executeInstallPhase(localEnv, params, op)
	case ops.OperationExpand:
		return executeJoinPhase(localEnv, params, op)
	case ops.OperationUpdate:
		return executeUpdatePhase(localEnv, environ, params, op)
	case ops.OperationUpdateRuntimeEnviron:
		return executeEnvironPhase(localEnv, environ, params, op)
	case ops.OperationUpdateConfig:
		return executeConfigPhase(localEnv, environ, params, op)
	case ops.OperationGarbageCollect:
		return executeGarbageCollectPhase(localEnv, params, op)
	default:
		return trace.BadParameter("operation type %q does not support plan execution", op.Type)
	}
}

// setPhase sets the specified phase state without executing it.
func setPhase(env *localenv.LocalEnvironment, environ LocalEnvironmentFactory, params SetPhaseParams) error {
	operations, err := getActiveOperations(env, environ, params.OperationID)
	if err != nil {
		return trace.Wrap(err)
	}
	if len(operations) != 1 {
		log.WithField("operations", oplist(operations)).Warn("Multiple operations found.")
		return trace.BadParameter("multiple operations found. Please specify operation with --operation-id")
	}
	op := operations[0]
	switch op.Type {
	case ops.OperationInstall, ops.OperationExpand:
		err = setPhaseFromService(env, params, op)
	case ops.OperationUpdate:
		err = setUpdatePhase(env, environ, params, op)
	case ops.OperationUpdateRuntimeEnviron:
		err = setEnvironPhase(env, environ, params, op)
	case ops.OperationUpdateConfig:
		err = setConfigPhase(env, environ, params, op)
	case ops.OperationGarbageCollect:
		err = setGarbageCollectPhase(env, params, op)
	default:
		return trace.BadParameter("operation type %q does not support setting phase state", op.Type)
	}
	if err != nil {
		return trace.Wrap(err)
	}
	env.PrintStep("Set phase %v to %v state", params.PhaseID, params.State)
	return nil
}

// rollbackPhase rolls back a phase for the operation specified with params
func rollbackPhase(localEnv *localenv.LocalEnvironment, environ LocalEnvironmentFactory, params PhaseParams) error {
	operations, err := getActiveOperations(localEnv, environ, params.OperationID)
	if err != nil {
		return trace.Wrap(err)
	}
	if len(operations) != 1 {
		log.WithField("operations", oplist(operations)).Warn("Multiple operations found.")
		return trace.BadParameter("multiple operations found. Please specify operation with --operation-id")
	}
	op := operations[0]
	switch op.Type {
	case ops.OperationInstall:
		return rollbackInstallPhase(localEnv, params, op)
	case ops.OperationExpand:
		return rollbackJoinPhase(localEnv, params, op)
	case ops.OperationUpdate:
		return rollbackUpdatePhase(localEnv, environ, params, op)
	case ops.OperationUpdateRuntimeEnviron:
		return rollbackEnvironPhase(localEnv, environ, params, op)
	case ops.OperationUpdateConfig:
		return rollbackConfigPhase(localEnv, environ, params, op)
	default:
		return trace.BadParameter("operation type %q does not support plan rollback", op.Type)
	}
}

func completeOperationPlan(localEnv *localenv.LocalEnvironment, environ LocalEnvironmentFactory, operationID string) error {
	operations, err := getActiveOperations(localEnv, environ, operationID)
	if err != nil {
		return trace.Wrap(err)
	}
	if len(operations) != 1 {
		log.WithField("operations", oplist(operations)).Warn("Multiple operations found.")
		return trace.BadParameter("multiple operations found. Please specify operation with --operation-id")
	}
	op := operations[0]
	switch op.Type {
	case ops.OperationInstall:
		err = completeInstallPlan(localEnv, op)
	case ops.OperationExpand:
		err = completeJoinPlan(localEnv, op)
	case ops.OperationUpdate:
		err = completeUpdatePlan(localEnv, environ, op)
	case ops.OperationUpdateRuntimeEnviron:
		err = completeEnvironPlan(localEnv, environ, op)
	case ops.OperationUpdateConfig:
		err = completeConfigPlan(localEnv, environ, op)
	default:
		return trace.BadParameter("operation type %q does not support plan completion", op.Type)
	}
	if op.Type != ops.OperationInstall && trace.IsNotFound(err) {
		log.WithError(err).Warn("Failed to complete operation from service.")
		return completeClusterOperationPlan(localEnv, op)
	}
	return trace.Wrap(err)
}

func completeClusterOperationPlan(localEnv *localenv.LocalEnvironment, operation ops.SiteOperation) error {
	clusterEnv, err := localEnv.NewClusterEnvironment()
	if err != nil {
		return trace.Wrap(err)
	}
	plan, err := fsm.GetOperationPlan(clusterEnv.Backend, operation.Key())
	if err != nil {
		return trace.Wrap(err)
	}
	if fsm.IsCompleted(plan) {
		return ops.CompleteOperation(context.TODO(), operation.Key(), clusterEnv.Operator)
	}
	return ops.FailOperation(context.TODO(), operation.Key(), clusterEnv.Operator, "completed manually")
}

func getLastOperations(localEnv *localenv.LocalEnvironment, environ LocalEnvironmentFactory, operationID string) ([]ops.SiteOperation, error) {
	operations, err := getBackendOperations(localEnv, environ, operationID)
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
	return operations, nil
}

func getActiveOperation(localEnv *localenv.LocalEnvironment, environ LocalEnvironmentFactory, operationID string) (*ops.SiteOperation, error) {
	operations, err := getActiveOperations(localEnv, environ, operationID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(operations) != 1 {
		log.WithField("operations", oplist(operations)).Warn("Multiple operations found.")
		return nil, trace.BadParameter("multiple operations found. Please specify operation with --operation-id")
	}
	return &operations[0], nil
}

func getActiveOperations(localEnv *localenv.LocalEnvironment, environ LocalEnvironmentFactory, operationID string) ([]ops.SiteOperation, error) {
	operations, err := getBackendOperations(localEnv, environ, operationID)
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
	return getActiveOperationsFromList(operations)
}

// getBackendOperations returns the list of operation from the specified backends
// in descending order (sorted by creation time)
func getBackendOperations(localEnv *localenv.LocalEnvironment, environ LocalEnvironmentFactory, operationID string) (result []ops.SiteOperation, err error) {
	b := newBackendOperations()
	err = b.List(localEnv, environ)
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

func (r *backendOperations) List(localEnv *localenv.LocalEnvironment, environ LocalEnvironmentFactory) error {
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
	if err := r.listUpdateOperation(environ); err != nil && !trace.IsNotFound(err) {
		log.WithError(err).Warn("Failed to list update operation.")
	}
	if err := r.listJoinOperation(environ); err != nil && !trace.IsNotFound(err) {
		log.WithError(err).Warn("Failed to list join operation.")
	}
	// Only fetch operation from remote (install) environment if the install operation is ongoing
	// or we failed to fetch the operation details from the cluster
	if r.isActiveInstallOperation() {
		if err := r.listInstallOperation(); err != nil {
			return trace.Wrap(err)
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

func (r *backendOperations) getOperationAndUpdateCache(getter operationGetter, logger logrus.FieldLogger) *ops.SiteOperation {
	op, err := getter.getOperation()
	if err == nil {
		// Operation from the backend takes precedence over the existing operation (from cluster state)
		r.operations[op.ID] = (ops.SiteOperation)(*op)
	} else if !trace.IsNotFound(err) {
		logger.WithError(err).Warn("Failed to query operation.")
	}
	return (*ops.SiteOperation)(op)
}

func (r *backendOperations) listUpdateOperation(environ LocalEnvironmentFactory) error {
	env, err := environ.NewUpdateEnv()
	if err != nil {
		return trace.Wrap(err)
	}
	defer env.Close()
	r.getOperationAndUpdateCache(getOperationFromBackend(env.Backend),
		log.WithField("context", "update"))
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
	r.getOperationAndUpdateCache(getOperationFromBackend(env.Backend),
		log.WithField("context", "expand"))
	return nil
}

func (r *backendOperations) listInstallOperation() error {
	if err := ensureInstallerServiceRunning(); err != nil {
		return trace.Wrap(err, "failed to restart installer service")
	}
	wizardEnv, err := localenv.NewRemoteEnvironment()
	if err == nil && wizardEnv.Operator != nil {
		cluster, err := getLocalClusterFromOperator(wizardEnv.Operator)
		if err == nil {
			log.Info("Fetching operation from wizard.")
			r.getOperationAndUpdateCache(getOperationFromOperator(wizardEnv.Operator, cluster.Key()),
				log.WithField("context", "install"))
			return nil
		}
		if trace.IsNotFound(err) {
			// Fail early if not found
			return trace.Wrap(err)
		}
		log.WithError(err).Warn("Failed to connect to wizard.")
	}
	return trace.NotFound("no operation found")
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
	operations       map[string]ops.SiteOperation
	clusterOperation *ops.SiteOperation
}

func getActiveOperationsFromList(operations []ops.SiteOperation) (result []ops.SiteOperation, err error) {
	for _, op := range operations {
		if !op.IsCompleted() {
			result = append(result, op)
		}
	}
	if len(result) == 0 {
		return nil, trace.NotFound("no active operations found")
	}
	return result, nil
}

func (r oplist) String() string {
	var ops []string
	for _, op := range r {
		ops = append(ops, op.String())
	}
	return strings.Join(ops, "\n")
}

type oplist []ops.SiteOperation

func getOperationFromOperator(operator ops.Operator, clusterKey ops.SiteKey) operationGetter {
	return operationGetterFunc(func() (*ops.SiteOperation, error) {
		op, _, err := ops.GetLastOperation(clusterKey, operator)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return op, nil
	})
}

func getOperationFromBackend(backend storage.Backend) operationGetter {
	return operationGetterFunc(func() (*ops.SiteOperation, error) {
		op, err := storage.GetLastOperation(backend)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return (*ops.SiteOperation)(op), nil
	})
}

func getLocalClusterFromOperator(operator ops.Operator) (cluster *ops.Site, err error) {
	// TODO(dmitri): when cluster is created by the wizard, it is not local
	// so resort to look up
	clusters, err := operator.GetSites(defaults.SystemAccountID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	log.WithField("clusters", clusters).Info("Fetched clusters from remote wizard.")
	if len(clusters) == 0 {
		return nil, trace.NotFound("no clusters found")
	}
	if len(clusters) != 1 {
		return nil, trace.BadParameter("expected a single cluster, but found %v", len(clusters))
	}
	return &clusters[0], nil
}

func (r operationGetterFunc) getOperation() (*ops.SiteOperation, error) {
	return r()
}

type operationGetterFunc func() (*ops.SiteOperation, error)

type operationGetter interface {
	getOperation() (*ops.SiteOperation, error)
}

func ensureInstallerServiceRunning() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	interrupt := signals.NewInterruptHandler(ctx, cancel)
	defer interrupt.Close()
	_, err := installerclient.New(context.Background(), installerclient.Config{
		ConnectStrategy:  &installerclient.ResumeStrategy{},
		InterruptHandler: interrupt,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}
