package vacuum

import (
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/vacuum/internal/fsm"

	"github.com/gravitational/trace"
)

func (r *Collector) getOrCreateOperationPlan() (plan *storage.OperationPlan, err error) {
	plan, err = r.Operator.GetOperationPlan(r.Operation.Key())
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}

	if trace.IsNotFound(err) {
		plan, err = fsm.NewOperationPlan(*r.Operation, r.Servers)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		err = r.Operator.CreateOperationPlan(r.Operation.Key(), *plan)
		if err != nil {
			if trace.IsNotFound(err) {
				return nil, trace.NotImplemented(
					"cluster operator does not implement the API required for garbage collection. " +
						"Please make sure you're running the command on a compatible cluster.")
			}
			return nil, trace.Wrap(err)
		}
	}

	return plan, nil
}
