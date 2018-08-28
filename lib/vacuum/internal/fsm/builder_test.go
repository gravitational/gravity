package fsm

import (
	"testing"

	"github.com/gravitational/gravity/lib/compare"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"

	. "gopkg.in/check.v1"
)

func TestFSM(t *testing.T) { TestingT(t) }

type S struct{}

var _ = Suite(&S{})

func (S) TestSingleNodePlan(c *C) {
	operation := ops.SiteOperation{
		ID:         "1",
		AccountID:  "0",
		Type:       ops.OperationGarbageCollect,
		SiteDomain: "cluster",
	}
	servers := []storage.Server{
		{Hostname: "node-1", ClusterRole: string(schema.ServiceRoleMaster)},
	}

	plan, err := NewOperationPlan(operation, servers)
	c.Assert(err, IsNil)
	c.Assert(plan, compare.DeepEquals, &storage.OperationPlan{
		OperationID:   operation.ID,
		OperationType: operation.Type,
		AccountID:     operation.AccountID,
		ClusterName:   operation.SiteDomain,
		Servers:       servers,
		Phases: []storage.OperationPhase{
			{
				ID:          "/registry",
				Description: "Prune unused docker images",
				Phases: []storage.OperationPhase{
					{
						ID:          "/registry/node-1",
						Description: `Prune unused docker images on node "node-1"`,
						Data: &storage.OperationPhaseData{
							Server: &servers[0],
						},
					},
				},
			},
			{
				ID:          "/packages",
				Description: "Prune unused packages",
				Phases: []storage.OperationPhase{
					{
						ID:          "/packages/cluster",
						Description: `Prune unused cluster packages`,
					},
					{
						ID:          "/packages/node-1",
						Description: `Prune unused packages on node "node-1"`,
						Data: &storage.OperationPhaseData{
							Server: &servers[0],
						},
					},
				},
			},
			{
				ID:          "/journal",
				Description: "Prune obsolete systemd journal directories",
				Phases: []storage.OperationPhase{
					{
						ID:          "/journal/node-1",
						Description: `Prune journal directories on node "node-1"`,
						Data: &storage.OperationPhaseData{
							Server: &servers[0],
						},
					},
				},
			},
		},
	})
}

func (S) TestMultiNodePlan(c *C) {
	operation := ops.SiteOperation{
		ID:         "1",
		AccountID:  "0",
		Type:       ops.OperationGarbageCollect,
		SiteDomain: "cluster",
	}
	servers := []storage.Server{
		{Hostname: "node-1", ClusterRole: string(schema.ServiceRoleMaster)},
		{Hostname: "node-2", ClusterRole: string(schema.ServiceRoleNode)},
		{Hostname: "node-3", ClusterRole: string(schema.ServiceRoleMaster)},
	}

	plan, err := NewOperationPlan(operation, servers)
	c.Assert(err, IsNil)
	c.Assert(plan, compare.DeepEquals, &storage.OperationPlan{
		OperationID:   operation.ID,
		OperationType: operation.Type,
		AccountID:     operation.AccountID,
		ClusterName:   operation.SiteDomain,
		Servers:       servers,
		Phases: []storage.OperationPhase{
			{
				ID:          "/registry",
				Description: "Prune unused docker images",
				Phases: []storage.OperationPhase{
					{
						ID:          "/registry/node-1",
						Description: `Prune unused docker images on node "node-1"`,
						Data: &storage.OperationPhaseData{
							Server: &servers[0],
						},
					},
					{
						ID:          "/registry/node-3",
						Description: `Prune unused docker images on node "node-3"`,
						Data: &storage.OperationPhaseData{
							Server: &servers[2],
						},
						Requires: []string{"/registry/node-1"},
					},
				},
			},
			{
				ID:          "/packages",
				Description: "Prune unused packages",
				Phases: []storage.OperationPhase{
					{
						ID:          "/packages/cluster",
						Description: `Prune unused cluster packages`,
					},
					{
						ID:          "/packages/node-1",
						Description: `Prune unused packages on node "node-1"`,
						Data: &storage.OperationPhaseData{
							Server: &servers[0],
						},
					},
					{
						ID:          "/packages/node-2",
						Description: `Prune unused packages on node "node-2"`,
						Data: &storage.OperationPhaseData{
							Server: &servers[1],
						},
					},
					{
						ID:          "/packages/node-3",
						Description: `Prune unused packages on node "node-3"`,
						Data: &storage.OperationPhaseData{
							Server: &servers[2],
						},
					},
				},
			},
			{
				ID:          "/journal",
				Description: "Prune obsolete systemd journal directories",
				Phases: []storage.OperationPhase{
					{
						ID:          "/journal/node-1",
						Description: `Prune journal directories on node "node-1"`,
						Data: &storage.OperationPhaseData{
							Server: &servers[0],
						},
					},
					{
						ID:          "/journal/node-2",
						Description: `Prune journal directories on node "node-2"`,
						Data: &storage.OperationPhaseData{
							Server: &servers[1],
						},
					},
					{
						ID:          "/journal/node-3",
						Description: `Prune journal directories on node "node-3"`,
						Data: &storage.OperationPhaseData{
							Server: &servers[2],
						},
					},
				},
			},
		},
	})
}
