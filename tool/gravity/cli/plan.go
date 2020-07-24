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
	"os"
	"time"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/storage"
	libenviron "github.com/gravitational/gravity/lib/system/environ"
	"github.com/gravitational/gravity/lib/update"
	clusterupdate "github.com/gravitational/gravity/lib/update/cluster"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/fatih/color"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

func initUpdateOperationPlan(localEnv, updateEnv *localenv.LocalEnvironment) error {
	ctx := context.TODO()
	clusterEnv, err := localEnv.NewClusterEnvironment()
	if err != nil {
		return trace.Wrap(err)
	}
	if clusterEnv.Client == nil {
		return trace.BadParameter("this operation can only be executed on one of the master nodes")
	}
	cluster, err := clusterEnv.Operator.GetLocalSite()
	if err != nil {
		return trace.Wrap(err)
	}
	operation, _, err := ops.GetLastOperation(cluster.Key(), clusterEnv.Operator)
	if err != nil {
		return trace.Wrap(err)
	}
	leader, err := findLocalServer(cluster.ClusterState.Servers)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = clusterupdate.InitOperationPlan(ctx, localEnv, updateEnv, clusterEnv, operation.Key(), leader)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(err)
}

func displayOperationPlan(localEnv *localenv.LocalEnvironment, environ LocalEnvironmentFactory, operationID string, format constants.Format) error {
	op, err := getLastOperation(localEnv, environ, operationID)
	if err != nil {
		if trace.IsNotFound(err) {
			message := noOperationStateNoClusterStateBanner
			if err := libenviron.ValidateNoPackageState(localEnv.Packages, localEnv.StateDir); err != nil {
				message = NoOperationStateBanner
			}
			return trace.NotFound(message)
		}
		return trace.Wrap(err)
	}
	if isInvalidOperation(*op) {
		return trace.BadParameter(invalidOperationBanner, op.String(), op.ID)
	}
	if op.IsCompleted() && op.hasPlan {
		return displayClusterOperationPlan(localEnv, op.Key(), format)
	}
	switch op.Type {
	case ops.OperationInstall, ops.OperationReconfigure:
		err = displayInstallOperationPlan(op.Key(), format)
	case ops.OperationExpand:
		err = displayExpandOperationPlan(environ, op.Key(), format)
	case ops.OperationUpdate:
		err = displayUpdateOperationPlan(localEnv, environ, op.Key(), format)
	case ops.OperationUpdateRuntimeEnviron:
		err = displayUpdateOperationPlan(localEnv, environ, op.Key(), format)
	case ops.OperationUpdateConfig:
		err = displayUpdateOperationPlan(localEnv, environ, op.Key(), format)
	case ops.OperationGarbageCollect:
		err = displayClusterOperationPlan(localEnv, op.Key(), format)
	default:
		return trace.BadParameter("cannot display plan for %q operation as it does not support plans", op.TypeString())
	}
	if err != nil && trace.IsNotFound(err) {
		// Fallback to cluster plan
		return displayClusterOperationPlan(localEnv, op.Key(), format)
	}
	return trace.Wrap(err)
}

func displayClusterOperationPlan(env *localenv.LocalEnvironment, opKey ops.SiteOperationKey, format constants.Format) error {
	clusterEnv, err := env.NewClusterEnvironment()
	if err != nil {
		return trace.Wrap(err)
	}
	plan, err := clusterEnv.Operator.GetOperationPlan(opKey)
	if err != nil {
		return trace.Wrap(err)
	}
	err = outputPlan(*plan, format)
	return trace.Wrap(err)
}

func displayUpdateOperationPlan(localEnv *localenv.LocalEnvironment, environ LocalEnvironmentFactory, opKey ops.SiteOperationKey, format constants.Format) error {
	updateEnv, err := environ.NewUpdateEnv()
	if err != nil {
		return trace.Wrap(err)
	}
	defer updateEnv.Close()
	plan, err := fsm.GetOperationPlan(updateEnv.Backend, opKey)
	if err != nil {
		return trace.Wrap(err)
	}
	reconciledPlan, err := tryReconcilePlan(context.TODO(), localEnv, updateEnv, *plan)
	if err != nil {
		logrus.WithError(err).Warn("Failed to reconcile plan.")
	} else {
		plan = reconciledPlan
	}
	err = outputPlan(*plan, format)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func displayInstallOperationPlan(opKey ops.SiteOperationKey, format constants.Format) error {
	plan, err := getPlanFromWizard(opKey)
	if err == nil {
		log.Debug("Showing install operation plan retrieved from wizard process.")
		return trace.Wrap(outputPlan(*plan, format))
	}
	plan, err = getPlanFromWizardBackend(opKey)
	if err != nil {
		return trace.Wrap(err, "failed to get plan for the install operation.\n"+
			"Make sure you are running 'gravity plan' from the installer node.")
	}
	return trace.Wrap(outputPlan(*plan, format))
}

// displayExpandOperationPlan shows plan of the join operation from the local join backend
func displayExpandOperationPlan(environ LocalEnvironmentFactory, opKey ops.SiteOperationKey, format constants.Format) error {
	joinEnv, err := environ.NewJoinEnv()
	if err != nil {
		return trace.Wrap(err)
	}
	defer joinEnv.Close()
	plan, err := fsm.GetOperationPlan(joinEnv.Backend, opKey)
	if err != nil {
		return trace.Wrap(err)
	}
	log.Debug("Showing join operation plan retrieved from local join backend.")
	return outputPlan(*plan, format)
}

func outputPlan(plan storage.OperationPlan, format constants.Format) (err error) {
	switch format {
	case constants.EncodingYAML:
		err = fsm.FormatOperationPlanYAML(os.Stdout, plan)
	case constants.EncodingJSON:
		err = fsm.FormatOperationPlanJSON(os.Stdout, plan)
	case constants.EncodingText:
		fsm.FormatOperationPlanText(os.Stdout, plan)
		err = explainPlan(plan.Phases)
	case constants.EncodingShort:
		fsm.FormatOperationPlanShort(os.Stdout, plan)
		err = explainPlan(plan.Phases)
	default:
		return trace.BadParameter("unknown output format %q", format)
	}
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func explainPlan(phases []storage.OperationPhase) (err error) {
	for _, phase := range phases {
		if phase.State == storage.OperationPhaseStateFailed {
			if err := outputPhaseError(phase); err != nil {
				log.Warnf("Failed to output phase error: %v.", err)
			}
			return nil
		}
		if err := explainPlan(phase.Phases); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func outputPhaseError(phase storage.OperationPhase) error {
	fmt.Print(color.RedString("The %v phase (%q) has failed", phase.ID, phase.Description))
	if phase.Error != nil {
		var phaseErr trace.TraceErr
		if err := utils.UnmarshalError(phase.Error.Err, &phaseErr); err != nil {
			return trace.Wrap(err, "failed to unmarshal phase error from JSON")
		}
		fmt.Print(color.RedString("\n\t%v\n", phaseErr.Err))
	}
	return nil
}

func tryReconcilePlan(ctx context.Context, localEnv, updateEnv *localenv.LocalEnvironment, plan storage.OperationPlan) (*storage.OperationPlan, error) {
	clusterEnv, err := localEnv.NewClusterEnvironment(localenv.WithEtcdTimeout(1 * time.Second))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	reconciler := update.NewDefaultReconciler(clusterEnv.Backend, updateEnv.Backend,
		plan.ClusterName, plan.OperationID, logrus.WithField("operation-id", plan.OperationID))
	reconciledPlan, err := reconciler.ReconcilePlan(ctx, plan)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return reconciledPlan, nil
}

func getPlanFromWizardBackend(opKey ops.SiteOperationKey) (*storage.OperationPlan, error) {
	wizardEnv, err := localenv.NewLocalWizardEnvironment()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	plan, err := fsm.GetOperationPlan(wizardEnv.Backend, opKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return plan, nil
}

func getPlanFromWizard(opKey ops.SiteOperationKey) (*storage.OperationPlan, error) {
	wizardEnv, err := localenv.NewRemoteEnvironment()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if wizardEnv.Operator == nil {
		return nil, trace.NotFound("no operation plan")
	}
	plan, err := wizardEnv.Operator.GetOperationPlan(opKey)
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound(
				"install operation plan hasn't been initialized yet.")
		}
		return nil, trace.Wrap(err)
	}
	return plan, nil
}

const (
	// NoOperationStateBanner specifies the message for when the operation
	// cannot be retrieved from the installer process and that the operation
	// should be restarted
	NoOperationStateBanner = `no operation found.
This usually means that the installation/join operation has failed to start or was not started.
Clean up the node with 'gravity leave' and start the operation with either 'gravity install' or 'gravity join'.
`
	noOperationStateNoClusterStateBanner = `no operation found.
This usually means that the installation/join operation has failed to start or was not started.
Start the operation with either 'gravity install' or 'gravity join'.
`
	invalidOperationBanner = `%v is invalid.
This usually means that the operation has failed to initialize properly.
You can mark this operation explicitly as failed with 'gravity plan complete --operation-id=%v' so it does not appear active and re-attempt it.
`
)
