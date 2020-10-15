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
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	installerclient "github.com/gravitational/gravity/lib/install/client"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/state"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/system/signals"
	"github.com/gravitational/gravity/lib/utils"
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
	params.PhaseID = fsm.RootPhase
	err := executePhase(localEnv, environ, params)
	if err == nil {
		return nil
	}
	if !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	if opErr, ok := trace.Unwrap(err).(operationNotFound); ok {
		if opErr.existing {
			return trace.Wrap(opErr)
		}
	}
	log.WithError(err).Warn("No operation found - will attempt to restart installation (resume join).")
	return trace.Wrap(restartInstallOrJoin(localEnv))
}

// executePhase executes a phase for the operation specified with params
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
		return executeJoinPhaseForOperation(localEnv, params, op)
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

// setPhase sets the specified phase state without executing it.
func setPhase(env *localenv.LocalEnvironment, environ LocalEnvironmentFactory, params SetPhaseParams) error {
	operation, err := getActiveOperation(env, environ, params.OperationID)
	if err != nil {
		return trace.Wrap(err)
	}
	op := operation.SiteOperation
	switch op.Type {
	case ops.OperationInstall, ops.OperationExpand:
		err = setPhaseFromService(env, params, op)
	case ops.OperationUpdate:
		err = setUpdatePhaseForOperation(env, environ, params, op)
	case ops.OperationUpdateRuntimeEnviron:
		err = setEnvironPhaseForOperation(env, environ, params, op)
	case ops.OperationUpdateConfig:
		err = setConfigPhaseForOperation(env, environ, params, op)
	case ops.OperationGarbageCollect:
		err = setGarbageCollectPhaseForOperation(env, params, op)
	default:
		return trace.BadParameter("operation type %q does not support setting phase state", op.Type)
	}
	if err != nil {
		return trace.Wrap(err)
	}
	env.PrintStep("Set phase %v to %v state", params.PhaseID, params.State)
	return nil
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
	op, err := getActiveOperation(localEnv, environ, params.OperationID)
	if err != nil {
		return trace.Wrap(err)
	}
	switch op.Type {
	case ops.OperationUpdate, ops.OperationUpdateRuntimeEnviron, ops.OperationUpdateConfig:
	default:
		return trace.BadParameter(unsupportedRollbackWarning, op.TypeString())
	}

	if err := verifyOrDeployAgents(localEnv); err != nil {
		// Continue operation in case gravity-site or etcd is down. In these
		// cases the agent status may not be retrievable.
		log.WithError(err).Warn("Failed to verify or deploy agents.")
	}

	if !confirmed && !params.DryRun {
		localEnv.Printf(planRollbackWarning, operationList{*op}.formatTable())
		if err := enforceConfirmation("Proceed?"); err != nil {
			return trace.Wrap(err)
		}
	}
	params.PhaseID = fsm.RootPhase
	switch op.Type {
	case ops.OperationUpdate:
		err = rollbackUpdatePhaseForOperation(localEnv, environ, params, op.SiteOperation)
	case ops.OperationUpdateRuntimeEnviron:
		err = rollbackEnvironPhaseForOperation(localEnv, environ, params, op.SiteOperation)
	case ops.OperationUpdateConfig:
		err = rollbackConfigPhaseForOperation(localEnv, environ, params, op.SiteOperation)
	default:
		return trace.BadParameter(unsupportedRollbackWarning, op.TypeString())
	}
	if err != nil {
		return trace.Wrap(err)
	}
	// Make sure to reset the cluster state after the operation has been
	// fully rolled back.
	if !params.DryRun {
		return completeOperationPlanForOperation(localEnv, environ, *op)
	}
	return nil
}

// rollbackPhase rolls back a phase for the operation specified with params
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
		return rollbackJoinPhaseForOperation(localEnv, params, op)
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
	err = completeOperationPlanForOperation(localEnv, environ, *operation)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func completeOperationPlanForOperation(localEnv *localenv.LocalEnvironment, environ LocalEnvironmentFactory, op clusterOperation) (err error) {
	switch op.Type {
	case ops.OperationInstall:
		err = completeInstallPlanForOperation(localEnv, op.SiteOperation)
	case ops.OperationExpand:
		err = completeJoinPlanForOperation(localEnv, op.SiteOperation)
	case ops.OperationUpdate:
		err = completeUpdatePlanForOperation(localEnv, environ, op)
	case ops.OperationUpdateRuntimeEnviron:
		err = completeEnvironPlanForOperation(localEnv, environ, op.SiteOperation)
	case ops.OperationUpdateConfig:
		err = completeConfigPlanForOperation(localEnv, environ, op.SiteOperation)
	default:
		return completeClusterOperationPlan(localEnv, op.SiteOperation)
	}
	if op.Type != ops.OperationInstall && trace.IsNotFound(err) {
		log.WithError(err).Warn("Failed to complete operation from service.")
		return completeClusterOperationPlan(localEnv, op.SiteOperation)
	}
	return trace.Wrap(err)
}

func completeClusterOperationPlan(localEnv *localenv.LocalEnvironment, operation ops.SiteOperation) error {
	clusterEnv, err := localEnv.NewClusterEnvironment()
	if err != nil {
		return trace.Wrap(err)
	}
	plan, err := fsm.GetOperationPlan(clusterEnv.Backend, operation.Key())
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	if err == nil && fsm.IsCompleted(plan) {
		return ops.CompleteOperation(context.TODO(), operation.Key(), clusterEnv.Operator)
	}
	return ops.FailOperation(context.TODO(), operation.Key(), clusterEnv.Operator, "completed manually")
}

// getLastOperation returns the last operation found across the specified backends.
// If no operation is found, the returned error will indicate a not found operation
func getLastOperation(localEnv *localenv.LocalEnvironment, environ LocalEnvironmentFactory, operationID string) (*clusterOperation, error) {
	b := newBackendOperations()

	if operationID != "" {
		op := b.GetOperationById(localEnv, environ, operationID)
		if op == nil {
			return nil, newOperationNotFound("no operation with ID %v found", operationID)
		}
	}

	op := b.GetLastOperation(localEnv, environ)
	if op == nil {
		return nil, newOperationNotFound("no operation found")
	}

	return op, nil
}

func getActiveOperation(localEnv *localenv.LocalEnvironment, environ LocalEnvironmentFactory, operationID string) (*clusterOperation, error) {
	operation, err := getLastOperation(localEnv, environ, operationID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if operation.IsCompleted() {
		return nil, operationNotFound{message: "no active operation found", existing: true}
	}
	return operation, nil
}

func newBackendOperations() backendOperations {
	return backendOperations{
		operations: make(map[string]clusterOperation),
	}
}

func (r *backendOperations) getLastOperationFromCluster(localEnv *localenv.LocalEnvironment) (*clusterOperation, error) {
	clusterEnv, err := localEnv.NewClusterEnvironment(localenv.WithEtcdTimeout(1 * time.Second))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if clusterEnv == nil {
		return nil, trace.NotFound("clusterEnv not available")
	}

	sites, err := clusterEnv.Operator.GetSites(defaults.SystemAccountID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(sites) == 0 {
		return nil, trace.NotFound("no clusters found")
	}

	operations, err := clusterEnv.Operator.GetSiteOperations(ops.SiteKey{
		AccountID:  defaults.SystemAccountID,
		SiteDomain: sites[0].Domain,
	}, ops.OperationsFilter{
		Last: true,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(operations) == 0 {
		return nil, trace.NotFound("no operations found")
	}

	plan, err := clusterEnv.Operator.GetOperationPlan(ops.SiteOperationKey{
		AccountID:   defaults.SystemAccountID,
		SiteDomain:  sites[0].Domain,
		OperationID: operations[0].ID,
	})
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err).AddField("operationId", operations[0].ID)
	}

	return &clusterOperation{
		SiteOperation: ops.SiteOperation(operations[0]),
		hasPlan:       plan != nil,
	}, nil
}

func (r *backendOperations) GetLastOperation(localEnv *localenv.LocalEnvironment, environ LocalEnvironmentFactory) *clusterOperation {
	r.importLocal(environ)

	clusterOp, err := r.getLastOperationFromCluster(localEnv)
	if err != nil {
		log.WithError(err).Warn("Failed to request operation from cluster.")
	}

	if len(r.operations) == 0 {
		return clusterOp
	}

	operations := []clusterOperation{}
	for _, v := range r.operations {
		operations = append(operations, v)
	}

	sort.Slice(operations, func(i, j int) bool {
		return operations[i].Created.After(operations[j].Created)
	})

	if clusterOp != nil && clusterOp.Created.After(operations[0].Created) {
		return clusterOp
	}

	return &operations[0]
}

func (r *backendOperations) getOperationByIDFromCluster(localEnv *localenv.LocalEnvironment, operationID string) (*clusterOperation, error) {
	clusterEnv, err := localEnv.NewClusterEnvironment(localenv.WithEtcdTimeout(1 * time.Second))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if clusterEnv == nil {
		return nil, trace.NotFound("clusterEnv not available")
	}

	sites, err := clusterEnv.Operator.GetSites(defaults.SystemAccountID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(sites) == 0 {
		return nil, trace.NotFound("no clusters found")
	}

	operation, err := clusterEnv.Operator.GetSiteOperation(ops.SiteOperationKey{
		AccountID:   defaults.SystemAccountID,
		SiteDomain:  sites[0].Domain,
		OperationID: operationID,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	plan, err := clusterEnv.Operator.GetOperationPlan(ops.SiteOperationKey{
		AccountID:   defaults.SystemAccountID,
		SiteDomain:  sites[0].Domain,
		OperationID: operation.ID,
	})
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err).AddField("operationId", operation.ID)
	}

	return &clusterOperation{
		SiteOperation: *operation,
		hasPlan:       plan != nil,
	}, nil
}

func (r *backendOperations) GetOperationById(localEnv *localenv.LocalEnvironment, environ LocalEnvironmentFactory, operationId string) *clusterOperation {
	r.importLocal(environ)

	if op, ok := r.operations[operationId]; ok {
		return &op
	}

	clusterOp, err := r.getOperationByIDFromCluster(localEnv, operationId)
	if err != nil {
		log.WithError(err).Warn("Failed to request operation from cluster.")
	}

	return clusterOp
}

func (r *backendOperations) importLocal(environ LocalEnvironmentFactory) {
	if environ == nil {
		return
	}
	// List operation from a local state store.
	// This is required in cases when the cluster store is inaccessible (like during upgrades)
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
			log.WithError(err).Warn("Failed to list install operation.")
		}
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
	// Initialize the operation state from the list of existing cluster operations.
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
	r.updateOperationInCache(getOperationFromBackend(env.Backend),
		log.WithField("context", "update"))
	return nil
}

func (r *backendOperations) listJoinOperation(environ LocalEnvironmentFactory) error {
	// TODO(dmitri): assumes local state inside process's working directory
	env, err := environ.NewJoinEnv(state.GravityInstallDir())
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
	r.updateOperationInCache(getOperationFromBackend(env.Backend),
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
			r.updateOperationInCache(getOperationFromOperator(wizardEnv.Operator, cluster.Key()),
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

func (r *backendOperations) updateOperationInCache(getter operationGetter, logger logrus.FieldLogger) {
	op, err := getter.getLastOperation()
	if err != nil {
		if !trace.IsNotFound(err) {
			logger.WithError(err).Warn("Failed to query operation.")
		}
		return
	}
	clusterOperation := clusterOperation{
		SiteOperation: (ops.SiteOperation)(*op),
	}
	if _, err := getter.getOperationPlan(op.Key()); err == nil {
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

func getOperationFromOperator(operator ops.Operator, clusterKey ops.SiteKey) operationGetter {
	return operatorGetter{
		operator:   operator,
		clusterKey: clusterKey,
	}
}

func (r operatorGetter) getLastOperation() (*ops.SiteOperation, error) {
	op, _, err := ops.GetLastOperation(r.clusterKey, r.operator)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return op, nil
}

func (r operatorGetter) getOperationPlan(key ops.SiteOperationKey) (*storage.OperationPlan, error) {
	plan, err := r.operator.GetOperationPlan(key)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return plan, nil
}

type operatorGetter struct {
	operator   ops.Operator
	clusterKey ops.SiteKey
}

func getOperationFromBackend(backend storage.Backend) operationGetter {
	return backendGetter{
		backend: backend,
	}
}

func (r backendGetter) getLastOperation() (*ops.SiteOperation, error) {
	op, err := storage.GetLastOperation(r.backend)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return (*ops.SiteOperation)(op), nil
}

func (r backendGetter) getOperationPlan(key ops.SiteOperationKey) (*storage.OperationPlan, error) {
	plan, err := r.backend.GetOperationPlan(key.SiteDomain, key.OperationID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return plan, nil
}

type backendGetter struct {
	backend storage.Backend
}

func getLocalClusterFromOperator(operator ops.Operator) (cluster *ops.Site, err error) {
	// TODO(dmitri): when cluster is created by the wizard, it is not local
	// so resort to looking it up
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

type operationGetter interface {
	getLastOperation() (*ops.SiteOperation, error)
	getOperationPlan(key ops.SiteOperationKey) (*storage.OperationPlan, error)
}

func ensureInstallerServiceRunning() error {
	strategy, err := newResumeStrategy(utils.Exe.WorkingDir)
	if err != nil {
		return trace.Wrap(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	interrupt := signals.NewInterruptHandler(ctx, cancel)
	defer interrupt.Close()
	_, err = installerclient.New(context.Background(), installerclient.Config{
		ConnectStrategy:  strategy,
		InterruptHandler: interrupt,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func newOperationNotFound(format string, args ...interface{}) operationNotFound {
	return operationNotFound{
		message: fmt.Sprintf(format, args...),
	}
}

// Error returns the text representation of this error.
// Implement error
func (r operationNotFound) Error() string {
	if r.message != "" {
		return r.message
	}
	return "no operation found"
}

// IsNotFoundError indicates that this is a not found error type.
// Implements trace.IsNotFoundError
func (r operationNotFound) IsNotFoundError() bool {
	return true
}

type operationNotFound struct {
	message string
	// existing indicates whether an operation exists but is not active
	existing bool
}
