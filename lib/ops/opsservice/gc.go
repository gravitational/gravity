package opsservice

import (
	"github.com/gravitational/gravity/lib/ops"

	"github.com/gravitational/trace"
	"github.com/pborman/uuid"
)

// createGarbageCollectOperation creates a new garbage collection operation in the cluster
func (s *site) createGarbageCollectOperation(req ops.CreateClusterGarbageCollectOperationRequest) (*ops.SiteOperationKey, error) {
	_, err := ops.GetCompletedInstallOperation(s.key, s.service)
	if err != nil {
		return nil, trace.Wrap(err, "garbage collection can only be started on an installed cluster")
	}

	op := ops.SiteOperation{
		ID:         uuid.New(),
		AccountID:  s.key.AccountID,
		SiteDomain: s.key.SiteDomain,
		Type:       ops.OperationGarbageCollect,
		Created:    s.clock().UtcNow(),
		Updated:    s.clock().UtcNow(),
		State:      ops.OperationGarbageCollectInProgress,
	}

	key, err := s.getOperationGroup().createSiteOperation(op)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return key, nil
}
