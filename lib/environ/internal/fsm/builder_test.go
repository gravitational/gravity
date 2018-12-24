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
	libphase "github.com/gravitational/gravity/lib/environ/internal/phases"
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
						ID:          "/masters/drain",
						Executor:    libphase.Drain,
						Description: `Drain node "node-1"`,
						Data: &storage.OperationPhaseData{
							Server: &servers[0],
						},
					},
					{
						ID:          "/masters/update-config",
						Executor:    libphase.UpdateConfig,
						Description: `Update runtime configuration on node "node-1"`,
						Data: &storage.OperationPhaseData{
							Server: &servers[0],
						},
						Requires: []string{"/masters/drain"},
					},
					{
						ID:          "/masters/restart",
						Executor:    libphase.RestartContainer,
						Description: `Restart container on node "node-1"`,
						Data: &storage.OperationPhaseData{
							Server: &servers[0],
						},
						Requires: []string{"/masters/update-config"},
					},
					{
						ID:          "/masters/taint",
						Executor:    libphase.Taint,
						Description: `Taint node "node-1"`,
						Data: &storage.OperationPhaseData{
							Server: &servers[0],
						},
						Requires: []string{"/masters/restart"},
					},
					{
						ID:          "/masters/uncordon",
						Executor:    libphase.Uncordon,
						Description: `Uncordon node "node-1"`,
						Data: &storage.OperationPhaseData{
							Server: &servers[0],
						},
						Requires: []string{"/masters/taint"},
					},
					{
						ID:          "/masters/endpoints",
						Executor:    libphase.Endpoints,
						Description: `Wait for endpoints on node "node-1"`,
						Data: &storage.OperationPhaseData{
							Server: &servers[0],
						},
						Requires: []string{"/masters/uncordon"},
					},
					{
						ID:          "/masters/untaint",
						Executor:    libphase.Untaint,
						Description: `Remove taint from node "node-1"`,
						Data: &storage.OperationPhaseData{
							Server: &servers[0],
						},
						Requires: []string{"/masters/endpoints"},
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
						Phases: []storage.OperationPhase{
							{
								ID:          "/masters/node-1/stepdown",
								Executor:    libphase.Elections,
								Description: `Step down "node-1" as Kubernetes leader`,
								Data: &storage.OperationPhaseData{
									Server: &servers[0],
									ElectionChange: &storage.ElectionChange{
										DisableServers: []storage.Server{servers[0]},
									},
								},
							},
							{
								ID:          "/masters/node-1/drain",
								Executor:    libphase.Drain,
								Description: `Drain node "node-1"`,
								Data: &storage.OperationPhaseData{
									Server: &servers[0],
								},
								Requires: []string{"/masters/node-1/stepdown"},
							},
							{
								ID:          "/masters/node-1/update-config",
								Executor:    libphase.UpdateConfig,
								Description: `Update runtime configuration on node "node-1"`,
								Data: &storage.OperationPhaseData{
									Server: &servers[0],
								},
								Requires: []string{"/masters/node-1/drain"},
							},
							{
								ID:          "/masters/node-1/restart",
								Executor:    libphase.RestartContainer,
								Description: `Restart container on node "node-1"`,
								Data: &storage.OperationPhaseData{
									Server: &servers[0],
								},
								Requires: []string{"/masters/node-1/update-config"},
							},

							{
								ID:          "/masters/node-1/taint",
								Executor:    libphase.Taint,
								Description: `Taint node "node-1"`,
								Data: &storage.OperationPhaseData{
									Server: &servers[0],
								},
								Requires: []string{"/masters/node-1/restart"},
							},
							{
								ID:          "/masters/node-1/uncordon",
								Executor:    libphase.Uncordon,
								Description: `Uncordon node "node-1"`,
								Data: &storage.OperationPhaseData{
									Server: &servers[0],
								},
								Requires: []string{"/masters/node-1/taint"},
							},
							{
								ID:          "/masters/node-1/endpoints",
								Executor:    libphase.Endpoints,
								Description: `Wait for endpoints on node "node-1"`,
								Data: &storage.OperationPhaseData{
									Server: &servers[0],
								},
								Requires: []string{"/masters/node-1/uncordon"},
							},
							{
								ID:          "/masters/node-1/untaint",
								Executor:    libphase.Untaint,
								Description: `Remove taint from node "node-1"`,
								Data: &storage.OperationPhaseData{
									Server: &servers[0],
								},
								Requires: []string{"/masters/node-1/endpoints"},
							},
							{
								ID:          "/masters/node-1/elect",
								Executor:    libphase.Elections,
								Description: `Make node "node-1" Kubernetes leader`,
								Data: &storage.OperationPhaseData{
									Server: &servers[0],
									ElectionChange: &storage.ElectionChange{
										EnableServers:  []storage.Server{servers[0]},
										DisableServers: []storage.Server{servers[2]},
									},
								},
								Requires: []string{"/masters/node-1/untaint"},
							},
						},
					},
					{
						ID:          "/masters/node-3",
						Description: `Update environment variables on node "node-3"`,
						Phases: []storage.OperationPhase{
							{
								ID:          "/masters/node-3/drain",
								Executor:    libphase.Drain,
								Description: `Drain node "node-3"`,
								Data: &storage.OperationPhaseData{
									Server: &servers[2],
								},
							},
							{
								ID:          "/masters/node-3/update-config",
								Executor:    libphase.UpdateConfig,
								Description: `Update runtime configuration on node "node-3"`,
								Data: &storage.OperationPhaseData{
									Server: &servers[2],
								},
								Requires: []string{"/masters/node-3/drain"},
							},
							{
								ID:          "/masters/node-3/restart",
								Executor:    libphase.RestartContainer,
								Description: `Restart container on node "node-3"`,
								Data: &storage.OperationPhaseData{
									Server: &servers[2],
								},
								Requires: []string{"/masters/node-3/update-config"},
							},
							{
								ID:          "/masters/node-3/taint",
								Executor:    libphase.Taint,
								Description: `Taint node "node-3"`,
								Data: &storage.OperationPhaseData{
									Server: &servers[2],
								},
								Requires: []string{"/masters/node-3/restart"},
							},
							{
								ID:          "/masters/node-3/uncordon",
								Executor:    libphase.Uncordon,
								Description: `Uncordon node "node-3"`,
								Data: &storage.OperationPhaseData{
									Server: &servers[2],
								},
								Requires: []string{"/masters/node-3/taint"},
							},
							{
								ID:          "/masters/node-3/endpoints",
								Executor:    libphase.Endpoints,
								Description: `Wait for endpoints on node "node-3"`,
								Data: &storage.OperationPhaseData{
									Server: &servers[2],
								},
								Requires: []string{"/masters/node-3/uncordon"},
							},
							{
								ID:          "/masters/node-3/untaint",
								Executor:    libphase.Untaint,
								Description: `Remove taint from node "node-3"`,
								Data: &storage.OperationPhaseData{
									Server: &servers[2],
								},
								Requires: []string{"/masters/node-3/endpoints"},
							},
							{
								ID:          "/masters/node-3/enable-elections",
								Executor:    libphase.Elections,
								Description: `Enable leader election on node "node-3"`,
								Data: &storage.OperationPhaseData{
									Server: &servers[2],
									ElectionChange: &storage.ElectionChange{
										EnableServers: []storage.Server{servers[2]},
									},
								},
								Requires: []string{"/masters/node-3/untaint"},
							},
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
						Phases: []storage.OperationPhase{
							{
								ID:          "/nodes/node-2/drain",
								Executor:    libphase.Drain,
								Description: `Drain node "node-2"`,
								Data: &storage.OperationPhaseData{
									Server: &servers[1],
								},
							},
							{
								ID:          "/nodes/node-2/update-config",
								Executor:    libphase.UpdateConfig,
								Description: `Update runtime configuration on node "node-2"`,
								Data: &storage.OperationPhaseData{
									Server: &servers[1],
								},
								Requires: []string{"/nodes/node-2/drain"},
							},
							{
								ID:          "/nodes/node-2/restart",
								Executor:    libphase.RestartContainer,
								Description: `Restart container on node "node-2"`,
								Data: &storage.OperationPhaseData{
									Server: &servers[1],
								},
								Requires: []string{"/nodes/node-2/update-config"},
							},
							{
								ID:          "/nodes/node-2/taint",
								Executor:    libphase.Taint,
								Description: `Taint node "node-2"`,
								Data: &storage.OperationPhaseData{
									Server: &servers[1],
								},
								Requires: []string{"/nodes/node-2/restart"},
							},
							{
								ID:          "/nodes/node-2/uncordon",
								Executor:    libphase.Uncordon,
								Description: `Uncordon node "node-2"`,
								Data: &storage.OperationPhaseData{
									Server: &servers[1],
								},
								Requires: []string{"/nodes/node-2/taint"},
							},
							{
								ID:          "/nodes/node-2/endpoints",
								Executor:    libphase.Endpoints,
								Description: `Wait for endpoints on node "node-2"`,
								Data: &storage.OperationPhaseData{
									Server: &servers[1],
								},
								Requires: []string{"/nodes/node-2/uncordon"},
							},
							{
								ID:          "/nodes/node-2/untaint",
								Executor:    libphase.Untaint,
								Description: `Remove taint from node "node-2"`,
								Data: &storage.OperationPhaseData{
									Server: &servers[1],
								},
								Requires: []string{"/nodes/node-2/endpoints"},
							},
						},
					},
					{
						ID:          "/nodes/node-4",
						Description: `Update environment variables on node "node-4"`,
						Phases: []storage.OperationPhase{
							{
								ID:          "/nodes/node-4/drain",
								Executor:    libphase.Drain,
								Description: `Drain node "node-4"`,
								Data: &storage.OperationPhaseData{
									Server: &servers[3],
								},
							},
							{
								ID:          "/nodes/node-4/update-config",
								Executor:    libphase.UpdateConfig,
								Description: `Update runtime configuration on node "node-4"`,
								Data: &storage.OperationPhaseData{
									Server: &servers[3],
								},
								Requires: []string{"/nodes/node-4/drain"},
							},
							{
								ID:          "/nodes/node-4/restart",
								Executor:    libphase.RestartContainer,
								Description: `Restart container on node "node-4"`,
								Data: &storage.OperationPhaseData{
									Server: &servers[3],
								},
								Requires: []string{"/nodes/node-4/update-config"},
							},
							{
								ID:          "/nodes/node-4/taint",
								Executor:    libphase.Taint,
								Description: `Taint node "node-4"`,
								Data: &storage.OperationPhaseData{
									Server: &servers[3],
								},
								Requires: []string{"/nodes/node-4/restart"},
							},
							{
								ID:          "/nodes/node-4/uncordon",
								Executor:    libphase.Uncordon,
								Description: `Uncordon node "node-4"`,
								Data: &storage.OperationPhaseData{
									Server: &servers[3],
								},
								Requires: []string{"/nodes/node-4/taint"},
							},
							{
								ID:          "/nodes/node-4/endpoints",
								Executor:    libphase.Endpoints,
								Description: `Wait for endpoints on node "node-4"`,
								Data: &storage.OperationPhaseData{
									Server: &servers[3],
								},
								Requires: []string{"/nodes/node-4/uncordon"},
							},
							{
								ID:          "/nodes/node-4/untaint",
								Executor:    libphase.Untaint,
								Description: `Remove taint from node "node-4"`,
								Data: &storage.OperationPhaseData{
									Server: &servers[3],
								},
								Requires: []string{"/nodes/node-4/endpoints"},
							},
						},
						Requires: []string{"/nodes/node-2"},
					},
				},
				Requires: []string{"/masters"},
			},
		},
	})
}
