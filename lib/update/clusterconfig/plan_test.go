/*
Copyright 2019 Gravitational, Inc.

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

package clusterconfig

import (
	"testing"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/compare"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/storage/clusterconfig"
	libphase "github.com/gravitational/gravity/lib/update/internal/rollingupdate/phases"

	. "gopkg.in/check.v1"
	v1 "k8s.io/api/core/v1"
)

func TestFSM(t *testing.T) { TestingT(t) }

type S struct{}

var _ = Suite(&S{})

func (S) TestSingleNodePlan(c *C) {
	operation := ops.SiteOperation{
		ID:         "1",
		AccountID:  "0",
		Type:       ops.OperationUpdateConfig,
		SiteDomain: "cluster",
	}
	servers := []storage.Server{
		{Hostname: "node-1", Role: "node", ClusterRole: string(schema.ServiceRoleMaster)},
	}
	runtimeLoc := loc.Locator{Repository: "foo", Name: "runtime", Version: "0.0.1"}
	app := app.Application{
		Package: loc.MustParseLocator("gravitational.io/app:0.0.1"),
		Manifest: schema.Manifest{
			NodeProfiles: schema.NodeProfiles{
				{
					Name:        "node",
					ServiceRole: "master",
				},
			},
			SystemOptions: &schema.SystemOptions{
				Dependencies: schema.SystemDependencies{
					Runtime: &schema.Dependency{Locator: runtimeLoc},
				},
			},
		},
	}
	clusterConfig := clusterconfig.NewEmpty()
	var services []v1.Service

	plan, err := newOperationPlan(app, storage.DefaultDNSConfig, testOperator, operation, clusterConfig, servers, services)
	c.Assert(err, IsNil)
	c.Assert(plan, compare.DeepEquals, &storage.OperationPlan{
		OperationID:   operation.ID,
		OperationType: operation.Type,
		AccountID:     operation.AccountID,
		ClusterName:   operation.SiteDomain,
		Servers:       servers,
		DNSConfig:     storage.DefaultDNSConfig,
		Phases: []storage.OperationPhase{
			{
				ID:          "/update-config",
				Executor:    libphase.UpdateConfig,
				Description: `Update runtime configuration`,
				Data: &storage.OperationPhaseData{
					Package: &app.Package,
					Update: &storage.UpdateOperationData{
						Servers: []storage.UpdateServer{
							{
								Server: servers[0],
								Runtime: storage.RuntimePackage{
									Installed: runtimeLoc,
									Update: &storage.RuntimeUpdate{
										Package:       runtimeLoc,
										ConfigPackage: testOperator.runtimeConfigPackage,
									},
								},
							},
						},
					},
				},
			},
			{
				ID:          "/masters",
				Description: "Update cluster configuration",
				Phases: []storage.OperationPhase{
					{
						ID:          "/masters/node-1",
						Description: `Update configuration on node "node-1"`,
						Phases: []storage.OperationPhase{
							{
								ID:          "/masters/node-1/drain",
								Executor:    libphase.Drain,
								Description: `Drain node "node-1"`,
								Data: &storage.OperationPhaseData{
									Server: &servers[0],
								},
							},
							{
								ID:          "/masters/node-1/restart",
								Executor:    libphase.RestartContainer,
								Description: `Restart container on node "node-1"`,
								Data: &storage.OperationPhaseData{
									ExecServer: &servers[0],
									Package:    &app.Package,
									Update: &storage.UpdateOperationData{
										Servers: []storage.UpdateServer{
											{
												Server: servers[0],
												Runtime: storage.RuntimePackage{
													Installed: runtimeLoc,
													Update: &storage.RuntimeUpdate{
														Package:       runtimeLoc,
														ConfigPackage: testOperator.runtimeConfigPackage,
													},
												},
											},
										},
									},
								},
								Requires: []string{"/masters/node-1/drain"},
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
						},
					},
				},
				Requires: []string{"/update-config"},
			},
		},
	})
}

func (S) TestMultiNodePlan(c *C) {
	operation := ops.SiteOperation{
		ID:         "1",
		AccountID:  "0",
		Type:       ops.OperationUpdateConfig,
		SiteDomain: "cluster",
	}
	servers := []storage.Server{
		{Hostname: "node-1", Role: "node", ClusterRole: string(schema.ServiceRoleMaster)},
		{Hostname: "node-2", Role: "knode", ClusterRole: string(schema.ServiceRoleNode)},
		{Hostname: "node-3", Role: "node", ClusterRole: string(schema.ServiceRoleMaster)},
	}
	runtimeLoc := loc.Locator{Repository: "foo", Name: "runtime", Version: "0.0.1"}
	app := app.Application{
		Package: loc.MustParseLocator("gravitational.io/app:0.0.1"),
		Manifest: schema.Manifest{
			NodeProfiles: schema.NodeProfiles{
				{
					Name:        "node",
					ServiceRole: "master",
				},
				{
					Name:        "knode",
					ServiceRole: "node",
				},
			},
			SystemOptions: &schema.SystemOptions{
				Dependencies: schema.SystemDependencies{
					Runtime: &schema.Dependency{Locator: runtimeLoc},
				},
			},
		},
	}
	clusterConfig := clusterconfig.NewEmpty()
	var services []v1.Service

	plan, err := newOperationPlan(app, storage.DefaultDNSConfig, testOperator, operation, clusterConfig, servers, services)
	c.Assert(err, IsNil)
	c.Assert(plan, compare.DeepEquals, &storage.OperationPlan{
		OperationID:   operation.ID,
		OperationType: operation.Type,
		AccountID:     operation.AccountID,
		ClusterName:   operation.SiteDomain,
		Servers:       servers,
		DNSConfig:     storage.DefaultDNSConfig,
		Phases: []storage.OperationPhase{
			{
				ID:          "/update-config",
				Executor:    libphase.UpdateConfig,
				Description: `Update runtime configuration`,
				Data: &storage.OperationPhaseData{
					Package: &app.Package,
					Update: &storage.UpdateOperationData{
						Servers: []storage.UpdateServer{
							{
								Server: servers[0],
								Runtime: storage.RuntimePackage{
									Installed: runtimeLoc,
									Update: &storage.RuntimeUpdate{
										Package:       runtimeLoc,
										ConfigPackage: testOperator.runtimeConfigPackage,
									},
								},
							},
							{
								Server: servers[2],
								Runtime: storage.RuntimePackage{
									Installed: runtimeLoc,
									Update: &storage.RuntimeUpdate{
										Package:       runtimeLoc,
										ConfigPackage: testOperator.runtimeConfigPackage,
									},
								},
							},
						},
					},
				},
			},
			{
				ID:          "/masters",
				Description: "Update cluster configuration",
				Phases: []storage.OperationPhase{
					{
						ID:          "/masters/node-1",
						Description: `Update configuration on node "node-1"`,
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
								ID:          "/masters/node-1/restart",
								Executor:    libphase.RestartContainer,
								Description: `Restart container on node "node-1"`,
								Data: &storage.OperationPhaseData{
									ExecServer: &servers[0],
									Package:    &app.Package,
									Update: &storage.UpdateOperationData{
										Servers: []storage.UpdateServer{
											{
												Server: servers[0],
												Runtime: storage.RuntimePackage{
													Installed: runtimeLoc,
													Update: &storage.RuntimeUpdate{
														Package:       runtimeLoc,
														ConfigPackage: testOperator.runtimeConfigPackage,
													},
												},
											},
										},
									},
								},
								Requires: []string{"/masters/node-1/drain"},
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
								Requires: []string{"/masters/node-1/restart"},
							},
							{
								ID:          "/masters/node-1/taint",
								Executor:    libphase.Taint,
								Description: `Taint node "node-1"`,
								Data: &storage.OperationPhaseData{
									Server: &servers[0],
								},
								Requires: []string{"/masters/node-1/elect"},
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
						},
					},
					{
						ID:          "/masters/node-3",
						Description: `Update configuration on node "node-3"`,
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
								ID:          "/masters/node-3/restart",
								Executor:    libphase.RestartContainer,
								Description: `Restart container on node "node-3"`,
								Data: &storage.OperationPhaseData{
									ExecServer: &servers[2],
									Package:    &app.Package,
									Update: &storage.UpdateOperationData{
										Servers: []storage.UpdateServer{
											{
												Server: servers[2],
												Runtime: storage.RuntimePackage{
													Installed: runtimeLoc,
													Update: &storage.RuntimeUpdate{
														Package:       runtimeLoc,
														ConfigPackage: testOperator.runtimeConfigPackage,
													},
												},
											},
										},
									},
								},
								Requires: []string{"/masters/node-3/drain"},
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
				Requires: []string{"/update-config"},
			},
		},
	})
}

func (S) TestBuildsPlanWithNodes(c *C) {
	operation := ops.SiteOperation{
		ID:         "1",
		AccountID:  "0",
		Type:       ops.OperationUpdateConfig,
		SiteDomain: "cluster",
	}
	servers := []storage.Server{
		{Hostname: "node-1", Role: "node", ClusterRole: string(schema.ServiceRoleMaster)},
		{Hostname: "node-2", Role: "knode", ClusterRole: string(schema.ServiceRoleNode)},
	}
	runtimeLoc := loc.Locator{Repository: "foo", Name: "runtime", Version: "0.0.1"}
	app := app.Application{
		Package: loc.MustParseLocator("gravitational.io/app:0.0.1"),
		Manifest: schema.Manifest{
			NodeProfiles: schema.NodeProfiles{
				{
					Name:        "node",
					ServiceRole: "master",
				},
				{
					Name:        "knode",
					ServiceRole: "node",
				},
			},
			SystemOptions: &schema.SystemOptions{
				Dependencies: schema.SystemDependencies{
					Runtime: &schema.Dependency{Locator: runtimeLoc},
				},
			},
		},
	}
	clusterConfig := clusterconfig.NewEmpty()
	clusterConfig.Spec.ComponentConfigs.Kubelet = &clusterconfig.Kubelet{
		Config: []byte(`apiVersion: v1
kind: KubeletConfiguration
address: "0.0.0.0"`),
	}
	// FIXME: populate services
	var services []v1.Service

	plan, err := newOperationPlan(app, storage.DefaultDNSConfig, testOperator, operation, clusterConfig, servers, services)
	c.Assert(err, IsNil)
	c.Assert(plan, compare.DeepEquals, &storage.OperationPlan{
		OperationID:   operation.ID,
		OperationType: operation.Type,
		AccountID:     operation.AccountID,
		ClusterName:   operation.SiteDomain,
		Servers:       servers,
		DNSConfig:     storage.DefaultDNSConfig,
		Phases: []storage.OperationPhase{
			{
				ID:          "/update-config",
				Executor:    libphase.UpdateConfig,
				Description: `Update runtime configuration`,
				Data: &storage.OperationPhaseData{
					Package: &app.Package,
					Update: &storage.UpdateOperationData{
						Servers: []storage.UpdateServer{
							{
								Server: servers[0],
								Runtime: storage.RuntimePackage{
									Installed: runtimeLoc,
									Update: &storage.RuntimeUpdate{
										Package:       runtimeLoc,
										ConfigPackage: testOperator.runtimeConfigPackage,
									},
								},
							},
							{
								Server: servers[1],
								Runtime: storage.RuntimePackage{
									Installed: runtimeLoc,
									Update: &storage.RuntimeUpdate{
										Package:       runtimeLoc,
										ConfigPackage: testOperator.runtimeConfigPackage,
									},
								},
							},
						},
					},
				},
			},
			{
				ID:          "/masters",
				Description: "Update cluster configuration",
				Phases: []storage.OperationPhase{
					{
						ID:          "/masters/node-1",
						Description: `Update configuration on node "node-1"`,
						Phases: []storage.OperationPhase{
							{
								ID:          "/masters/node-1/drain",
								Executor:    libphase.Drain,
								Description: `Drain node "node-1"`,
								Data: &storage.OperationPhaseData{
									Server: &servers[0],
								},
							},
							{
								ID:          "/masters/node-1/restart",
								Executor:    libphase.RestartContainer,
								Description: `Restart container on node "node-1"`,
								Data: &storage.OperationPhaseData{
									ExecServer: &servers[0],
									Package:    &app.Package,
									Update: &storage.UpdateOperationData{
										Servers: []storage.UpdateServer{
											{
												Server: servers[0],
												Runtime: storage.RuntimePackage{
													Installed: runtimeLoc,
													Update: &storage.RuntimeUpdate{
														Package:       runtimeLoc,
														ConfigPackage: testOperator.runtimeConfigPackage,
													},
												},
											},
										},
									},
								},
								Requires: []string{"/masters/node-1/drain"},
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
						},
					},
				},
				Requires: []string{"/update-config"},
			},
			{
				ID:          "/nodes",
				Description: "Update cluster configuration",
				Phases: []storage.OperationPhase{
					{
						ID:          "/nodes/node-2",
						Description: `Update configuration on node "node-2"`,
						Phases: []storage.OperationPhase{
							{
								ID:          "/nodes/node-2/drain",
								Executor:    libphase.Drain,
								Description: `Drain node "node-2"`,
								Data: &storage.OperationPhaseData{
									Server:     &servers[1],
									ExecServer: &servers[0],
								},
							},
							{
								ID:          "/nodes/node-2/restart",
								Executor:    libphase.RestartContainer,
								Description: `Restart container on node "node-2"`,
								Data: &storage.OperationPhaseData{
									ExecServer: &servers[1],
									Package:    &app.Package,
									Update: &storage.UpdateOperationData{
										Servers: []storage.UpdateServer{
											{
												Server: servers[1],
												Runtime: storage.RuntimePackage{
													Installed: runtimeLoc,
													Update: &storage.RuntimeUpdate{
														Package:       runtimeLoc,
														ConfigPackage: testOperator.runtimeConfigPackage,
													},
												},
											},
										},
									},
								},
								Requires: []string{"/nodes/node-2/drain"},
							},
							{
								ID:          "/nodes/node-2/taint",
								Executor:    libphase.Taint,
								Description: `Taint node "node-2"`,
								Data: &storage.OperationPhaseData{
									Server:     &servers[1],
									ExecServer: &servers[0],
								},
								Requires: []string{"/nodes/node-2/restart"},
							},
							{
								ID:          "/nodes/node-2/uncordon",
								Executor:    libphase.Uncordon,
								Description: `Uncordon node "node-2"`,
								Data: &storage.OperationPhaseData{
									Server:     &servers[1],
									ExecServer: &servers[0],
								},
								Requires: []string{"/nodes/node-2/taint"},
							},
							{
								ID:          "/nodes/node-2/endpoints",
								Executor:    libphase.Endpoints,
								Description: `Wait for endpoints on node "node-2"`,
								Data: &storage.OperationPhaseData{
									Server:     &servers[1],
									ExecServer: &servers[0],
								},
								Requires: []string{"/nodes/node-2/uncordon"},
							},
							{
								ID:          "/nodes/node-2/untaint",
								Executor:    libphase.Untaint,
								Description: `Remove taint from node "node-2"`,
								Data: &storage.OperationPhaseData{
									Server:     &servers[1],
									ExecServer: &servers[0],
								},
								Requires: []string{"/nodes/node-2/endpoints"},
							},
						},
					},
				},
				Requires: []string{"/update-config", "/masters"},
			},
		},
	})
}

func (r testRotator) RotateSecrets(ops.RotateSecretsRequest) (*ops.RotatePackageResponse, error) {
	return &ops.RotatePackageResponse{Locator: r.secretsPackage}, nil
}

func (r testRotator) RotatePlanetConfig(ops.RotatePlanetConfigRequest) (*ops.RotatePackageResponse, error) {
	return &ops.RotatePackageResponse{Locator: r.runtimeConfigPackage}, nil
}

var testOperator = testRotator{
	secretsPackage: loc.Locator{
		Repository: "gravitational.io",
		Name:       "planet-secrets",
		Version:    "0.0.1",
	},
	runtimeConfigPackage: loc.Locator{
		Repository: "gravitational.io",
		Name:       "planet-config",
		Version:    "0.0.1",
	},
}

type testRotator struct {
	secretsPackage       loc.Locator
	runtimeConfigPackage loc.Locator
}
