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
	leader, err := findLocalServer(*cluster)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = clusterupdate.InitOperationPlan(ctx, localEnv, updateEnv, clusterEnv, operation.Key(), leader)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(err)
}

func displayOperationPlan(localEnv, updateEnv, joinEnv *localenv.LocalEnvironment, operationID string, format constants.Format) error {
	op, err := getLastOperation(localEnv, updateEnv, joinEnv, operationID)
	if err != nil {
		return trace.Wrap(err)
	}
	if op.IsCompleted() {
		return displayClusterOperationPlan(localEnv, op.Key(), format)
	}
	switch op.Type {
	case ops.OperationInstall:
		return displayInstallOperationPlan(op.Key(), format)
	case ops.OperationExpand:
		return displayExpandOperationPlan(joinEnv, op.Key(), format)
	case ops.OperationUpdate:
		return displayUpdateOperationPlan(localEnv, updateEnv, op.Key(), format)
	case ops.OperationUpdateRuntimeEnviron:
		return displayUpdateOperationPlan(localEnv, updateEnv, op.Key(), format)
	case ops.OperationUpdateConfig:
		return displayUpdateOperationPlan(localEnv, updateEnv, op.Key(), format)
	case ops.OperationGarbageCollect:
		return displayClusterOperationPlan(localEnv, op.Key(), format)
	default:
		return trace.BadParameter("unknown operation type %q", op.Type)
	}
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

func displayUpdateOperationPlan(localEnv, updateEnv *localenv.LocalEnvironment, opKey ops.SiteOperationKey, format constants.Format) error {
	plan, err := fsm.GetOperationPlan(updateEnv.Backend, opKey.SiteDomain, opKey.OperationID)
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
	wizardEnv, err := localenv.NewRemoteEnvironment()
	if err != nil {
		return trace.Wrap(err)
	}
	if wizardEnv.Operator == nil {
		return trace.NotFound(`could not retrieve install operation plan.

If you have not launched the installation, or it has been started moments ago,
the plan may not be initialized yet.

If the install operation is in progress, please make sure you're invoking
"gravity plan" command from the same directory where "gravity install"
was run.`)
	}
	plan, err := wizardEnv.Operator.GetOperationPlan(opKey)
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound(
				"Install operation plan hasn't been initialized yet.")
		}
		return trace.Wrap(err)
	}
	log.Debug("Showing install operation plan retrieved from wizard process.")
	err = outputPlan(*plan, format)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// displayExpandOperationPlan shows plan of the join operation from the local join backend
func displayExpandOperationPlan(joinEnv *localenv.LocalEnvironment, opKey ops.SiteOperationKey, format constants.Format) error {
	plan, err := fsm.GetOperationPlan(joinEnv.Backend, opKey.SiteDomain, opKey.OperationID)
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
	fmt.Printf(color.RedString("The %v phase (%q) has failed", phase.ID, phase.Description))
	if phase.Error != nil {
		var phaseErr trace.TraceErr
		if err := utils.UnmarshalError(phase.Error.Err, &phaseErr); err != nil {
			return trace.Wrap(err, "failed to unmarshal phase error from JSON")
		}
		fmt.Printf(color.RedString("\n\t%v\n", phaseErr.Err))
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

const recoveryModeWarning = "Failed to retrieve plan from etcd, showing cached plan. If etcd went down as a result of a system upgrade, you can perform a rollback phase. Run 'gravity plan --repair' when etcd connection is restored.\n"
