package opsservice

import (
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/trace"
)

// ResumeShrink resumes the started shrink operation if the node being shrunk gave up
// its leadership
func (o *Operator) ResumeShrink(key ops.SiteKey) (*ops.SiteOperationKey, error) {
	site, err := o.openSite(ops.SiteKey{AccountID: key.AccountID, SiteDomain: key.SiteDomain})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	opKey, err := site.resumeShrink()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return opKey, nil
}

func (s *site) resumeShrink() (*ops.SiteOperationKey, error) {
	s.Debug("resume shrink operation")

	site, err := s.service.GetSite(s.key)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if site.State != ops.SiteStateShrinking {
		return nil, trace.NotFound("cluster is not shrinking")
	}

	op, err := ops.GetLastShrinkOperation(s.key, s.service)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	key := op.Key()

	if op.State != ops.OperationStateShrinkInProgress {
		return nil, trace.NotFound("shrink operation is not in progress: %v", op)
	}

	s.Debugf("resuming shrink operation: %v", op)

	err = s.executeOperation(key, s.shrinkOperationStart)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &key, nil
}
