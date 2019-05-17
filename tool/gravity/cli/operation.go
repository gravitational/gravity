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

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
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

// resumeOperation resumes the operation specified with params
func resumeOperation(localEnv *localenv.LocalEnvironment, environ LocalEnvironmentFactory, params PhaseParams) error {
	params.PhaseID = fsm.RootPhase
	err := executePhase(localEnv, environ, params)
	if err == nil {
		return nil
	}
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	// No operation found.
	// Attempt to resume the installation
	return trace.Wrap(ResumeInstall(localEnv))
}

// executePhase executes a phase for the operation specified with params
func executePhase(localEnv *localenv.LocalEnvironment, environ LocalEnvironmentFactory, params PhaseParams) error {
	op, err := getActiveOperation(localEnv, environ, params.OperationID)
	if err != nil {
		return trace.Wrap(err)
	}
	switch op.Type {
	case ops.OperationInstall:
		return executeInstallPhase(localEnv, params, op)
	case ops.OperationExpand:
		return executeJoinPhase(localEnv, params, op)
	case ops.OperationUpdate:
		return executeUpdatePhase(localEnv, environ, params, *op)
	case ops.OperationUpdateRuntimeEnviron:
		return executeEnvironPhase(localEnv, environ, params, *op)
	case ops.OperationUpdateConfig:
		return executeConfigPhase(localEnv, environ, params, *op)
	case ops.OperationGarbageCollect:
		return executeGarbageCollectPhase(localEnv, params, op)
	default:
		return trace.BadParameter("operation type %q does not support plan execution", op.Type)
	}
}

// rollbackPhase rolls back a phase for the operation specified with params
func rollbackPhase(localEnv *localenv.LocalEnvironment, environ LocalEnvironmentFactory, params PhaseParams) error {
	op, err := getActiveOperation(localEnv, environ, params.OperationID)
	if err != nil {
		return trace.Wrap(err)
	}
	switch op.Type {
	case ops.OperationInstall:
		return rollbackInstallPhase(localEnv, params, op)
	case ops.OperationExpand:
		return rollbackJoinPhase(localEnv, params, op)
	case ops.OperationUpdate:
		return rollbackUpdatePhase(localEnv, environ, params, *op)
	case ops.OperationUpdateRuntimeEnviron:
		return rollbackEnvironPhase(localEnv, environ, params, *op)
	case ops.OperationUpdateConfig:
		return rollbackConfigPhase(localEnv, environ, params, *op)
	default:
		return trace.BadParameter("operation type %q does not support plan rollback", op.Type)
	}
}

func completeOperationPlan(localEnv *localenv.LocalEnvironment, environ LocalEnvironmentFactory, operationID string) error {
	op, err := getActiveOperation(localEnv, environ, operationID)
	if err != nil {
		return trace.Wrap(err)
	}
	switch op.Type {
	case ops.OperationInstall:
		return completeInstallPlan(localEnv, op)
	case ops.OperationExpand:
		return completeJoinPlan(localEnv, op)
	case ops.OperationUpdate:
		return completeUpdatePlan(localEnv, environ, *op)
	case ops.OperationUpdateRuntimeEnviron:
		return completeEnvironPlan(localEnv, environ, *op)
	case ops.OperationUpdateConfig:
		return completeConfigPlan(localEnv, environ, *op)
	default:
		return trace.BadParameter("operation type %q does not support plan completion", op.Type)
	}
}

func getLastOperation(localEnv *localenv.LocalEnvironment, environ LocalEnvironmentFactory, operationID string) (*ops.SiteOperation, error) {
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
	if len(operations) == 1 && operationID != "" {
		log.WithField("operation", operations[0]).Debug("Fetched operation by ID.")
		return &operations[0], nil
	}
	if len(operations) != 1 {
		log.Infof("Multiple operations found: \n%v\n, please specify operation with --operation-id.\n"+
			"Displaying the most recent operation.",
			oplist(operations))
	}
	return &operations[0], nil
}

func getActiveOperation(localEnv *localenv.LocalEnvironment, environ LocalEnvironmentFactory, operationID string) (*ops.SiteOperation, error) {
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
	op, err := getActiveOperationFromList(operations)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return op, nil
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
	if err := r.listUpdateOperation(environ); err != nil {
		log.WithError(err).Warn("Failed to list update operation.")
	}
	if err := r.listJoinOperation(environ); err != nil {
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
	} else {
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
	wizardEnv, err := localenv.NewRemoteEnvironment()
	if err == nil && wizardEnv.Operator != nil {
		cluster, err := getLocalClusterFromWizard(wizardEnv.Operator)
		if err == nil {
			log.Info("Fetching operation from wizard.")
			r.getOperationAndUpdateCache(getOperationFromOperator(wizardEnv.Operator, cluster.Key()),
				log.WithField("context", "install"))
			return nil
		}
		log.WithError(err).Warn("Failed to connect to wizard.")
	}
	wizardLocalEnv, err := localenv.NewLocalWizardEnvironment()
	if err != nil {
		return trace.Wrap(err, "failed to read local wizard environment")
	}
	log.Info("Fetching operation directly from wizard backend.")
	r.getOperationAndUpdateCache(getOperationFromBackend(wizardLocalEnv.Backend),
		log.WithField("context", "install"))
	return nil
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

func isActiveOperation(op ops.SiteOperation) bool {
	return op.IsFailed() || !op.IsCompleted()
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

func getLocalClusterFromWizard(operator ops.Operator) (cluster *ops.Site, err error) {
	// TODO(dmitri): I attempted to default to local when creating clusters with wizard
	// but this breaks when the installer needs to tunnel APIs to the installed cluster
	// in which case it uses the difference of local (installed cluster) vs non-local
	// (in wizard state), so resorting to look up
	clusters, err := operator.GetSites(defaults.SystemAccountID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	log.WithField("clusters", clusters).Info("Fetched clusters from remote wizard.")
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
