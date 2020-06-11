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

package fsm

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/tool/common"

	"github.com/gravitational/trace"
	"gopkg.in/yaml.v2"
)

func FormatOperationPlanYAML(w io.Writer, plan storage.OperationPlan) error {
	bytes, err := yaml.Marshal(plan)
	if err != nil {
		return trace.Wrap(err)
	}

	if _, err := w.Write(bytes); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func FormatOperationPlanJSON(w io.Writer, plan storage.OperationPlan) error {
	bytes, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return trace.Wrap(err)
	}

	if _, err := w.Write(bytes); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func FormatOperationPlanText(w io.Writer, plan storage.OperationPlan) {
	var t tabwriter.Writer
	t.Init(w, 0, 10, 5, ' ', 0)
	common.PrintTableHeader(&t, []string{"Phase", "Description", "State", "Node", "Requires", "Updated"})
	for _, phase := range plan.Phases {
		printPhase(&t, phase, 0)
	}
	t.Flush()
}

func printPhase(w io.Writer, phase storage.OperationPhase, indent int) {
	marker := "*"
	if phase.GetState() == storage.OperationPhaseStateInProgress {
		marker = constants.InProgressMark
	} else if phase.GetState() == storage.OperationPhaseStateCompleted {
		marker = constants.SuccessMark
	} else if phase.GetState() == storage.OperationPhaseStateFailed {
		marker = constants.FailureMark
	} else if phase.GetState() == storage.OperationPhaseStateRolledBack {
		marker = constants.RollbackMark
	}
	fmt.Fprintf(w, "%v%v %v\t%v\t%v\t%v\t%v\t%v\n",
		strings.Repeat("  ", indent),
		marker,
		formatName(phase.ID),
		phase.Description,
		formatState(phase.GetState()),
		formatNode(phase),
		formatRequires(phase.Requires),
		formatTimestamp(phase.GetLastUpdateTime()))
	for _, subPhase := range phase.Phases {
		printPhase(w, subPhase, indent+1)
	}
}

// FormatOperationPlanShort formats provided operation plan as text with
// fewer number of columns.
func FormatOperationPlanShort(w io.Writer, plan storage.OperationPlan) {
	var t tabwriter.Writer
	t.Init(w, 0, 10, 5, ' ', 0)
	common.PrintTableHeader(&t, []string{"Phase", "State", "Updated"})
	for _, phase := range plan.Phases {
		printPhaseShort(&t, phase, 0)
	}
	t.Flush()
}

func printPhaseShort(w io.Writer, phase storage.OperationPhase, indent int) {
	marker := "*"
	if phase.GetState() == storage.OperationPhaseStateInProgress {
		marker = constants.InProgressMark
	} else if phase.GetState() == storage.OperationPhaseStateCompleted {
		marker = constants.SuccessMark
	} else if phase.GetState() == storage.OperationPhaseStateFailed || phase.GetState() == storage.OperationPhaseStateRolledBack {
		marker = constants.FailureMark
	}
	fmt.Fprintf(w, "%v%v %v\t%v\t%v\n",
		strings.Repeat("  ", indent),
		marker,
		formatName(phase.ID),
		formatState(phase.GetState()),
		formatTimestamp(phase.GetLastUpdateTime()))
	for _, subPhase := range phase.Phases {
		printPhaseShort(w, subPhase, indent+1)
	}
}

func formatNode(phase storage.OperationPhase) string {
	if phase.Data == nil || phase.Data.ExecServer == nil {
		return "-"
	}
	return phase.Data.ExecServer.AdvertiseIP
}

func formatName(phaseID string) string {
	parts := strings.Split(phaseID, "/")
	return parts[len(parts)-1]
}

func formatRequires(requires []string) string {
	if len(requires) == 0 {
		return "-"
	}
	return strings.Join(requires, ",")
}

func formatTimestamp(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Format(constants.HumanDateFormat)
}

func formatState(state string) string {
	switch state {
	case storage.OperationPhaseStateUnstarted:
		return "Unstarted"
	case storage.OperationPhaseStateInProgress:
		return "In Progress"
	case storage.OperationPhaseStateCompleted:
		return "Completed"
	case storage.OperationPhaseStateFailed:
		return "Failed"
	case storage.OperationPhaseStateRolledBack:
		return "Rolled Back"
	default:
		return "Unknown"
	}
}
