/*
Copyright 2020 Gravitational, Inc.

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
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/update/clusterconfig/phases"
	libbuilder "github.com/gravitational/gravity/lib/update/internal/builder"
	"github.com/gravitational/gravity/lib/update/internal/rollingupdate"
	libphase "github.com/gravitational/gravity/lib/update/internal/rollingupdate/phases"
)

func newBuilder(app loc.Locator) *builder {
	return &builder{
		Builder: rollingupdate.Builder{
			App: app,
		},
	}
}

func newBuilderWithServices(config planConfig) (*builder, error) {
	return &builder{
		Builder: rollingupdate.Builder{
			App: config.app.Package,
			CustomUpdate: &storage.OperationPhase{
				ID:          "services",
				Executor:    libphase.Custom,
				Description: "Reset services",
				Data: &storage.OperationPhaseData{
					Update: &storage.UpdateOperationData{
						ClusterConfig: &storage.ClusterConfigData{
							ServiceSuffix: config.serviceSuffix,
							ServiceCIDR:   config.clusterConfig.GetGlobalConfig().ServiceCIDR,
							Services:      config.services,
						},
					},
				},
			},
		},
	}, nil
}

func (r builder) init(desc string) *libbuilder.Phase {
	return libbuilder.NewPhase(storage.OperationPhase{
		ID:          "init",
		Executor:    phases.InitPhase,
		Description: desc,
		Data:        r.Builder.CustomUpdate.Data,
	})
}

func (r builder) fini(desc string) *libbuilder.Phase {
	return libbuilder.NewPhase(storage.OperationPhase{
		ID:          "fini",
		Executor:    phases.FiniPhase,
		Description: desc,
		Data:        r.Builder.CustomUpdate.Data,
	})
}

type builder struct {
	rollingupdate.Builder
}
