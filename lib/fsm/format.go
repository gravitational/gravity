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
	common.PrintTableHeader(&t, []string{"Phase", "Description", "State", "Requires", "Updated"})
	for _, phase := range plan.Phases {
		printPhase(&t, phase, 0)
	}
	t.Flush()
}

func printPhase(w io.Writer, phase storage.OperationPhase, indent int) {
	marker := "*"
	if phase.GetState() == storage.OperationPhaseStateInProgress {
		marker = "→"
	} else if phase.GetState() == storage.OperationPhaseStateCompleted {
		marker = "✓"
	} else if phase.GetState() == storage.OperationPhaseStateFailed || phase.GetState() == storage.OperationPhaseStateRolledBack {
		marker = "⚠"
	}
	fmt.Fprintf(w, "%v%v %v\t%v\t%v\t%v\t%v\n",
		strings.Repeat("  ", indent),
		marker,
		formatName(phase.ID),
		phase.Description,
		formatState(phase.GetState()),
		formatRequires(phase.Requires),
		formatTimestamp(phase.GetLastUpdateTime()))
	for _, subPhase := range phase.Phases {
		printPhase(w, subPhase, indent+1)
	}
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
