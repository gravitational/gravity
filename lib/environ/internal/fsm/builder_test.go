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
		Type:       ops.OperationUpdateEnvars,
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
				ID:          "/masters",
				Description: "Update cluster environment variables",
				Phases: []storage.OperationPhase{
					{
						ID:          "/masters/node-1",
						Description: `Update environment variables on node "node-1"`,
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
		Type:       ops.OperationUpdateEnvars,
		SiteDomain: "cluster",
	}
	servers := []storage.Server{
		{Hostname: "node-1", ClusterRole: string(schema.ServiceRoleMaster)},
		{Hostname: "node-2", ClusterRole: string(schema.ServiceRoleNode)},
		{Hostname: "node-3", ClusterRole: string(schema.ServiceRoleMaster)},
		{Hostname: "node-4", ClusterRole: string(schema.ServiceRoleNode)},
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
				ID:          "/masters",
				Description: "Update cluster environment variables",
				Phases: []storage.OperationPhase{
					{
						ID:          "/masters/node-1",
						Description: `Update environment variables on node "node-1"`,
						Data: &storage.OperationPhaseData{
							Server: &servers[0],
						},
					},
					{
						ID:          "/masters/node-3",
						Description: `Update environment variables on node "node-3"`,
						Data: &storage.OperationPhaseData{
							Server: &servers[2],
						},
						Requires: []string{"/masters/node-1"},
					},
				},
			},
			{
				ID:          "/nodes",
				Description: "Update cluster environment variables",
				Phases: []storage.OperationPhase{
					{
						ID:          "/nodes/node-2",
						Description: `Update environment variables on node "node-2"`,
						Data: &storage.OperationPhaseData{
							Server: &servers[1],
						},
					},
					{
						ID:          "/nodes/node-4",
						Description: `Update environment variables on node "node-4"`,
						Data: &storage.OperationPhaseData{
							Server: &servers[3],
						},
						Requires: []string{"/nodes/node-2"},
					},
				},
				Requires: []string{"/masters"},
			},
		},
	})
}
