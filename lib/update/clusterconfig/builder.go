package clusterconfig

import (
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/update"
	"github.com/gravitational/gravity/lib/update/clusterconfig/phases"
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
			CustomUpdate: &update.Phase{
				ID:          "services",
				Executor:    libphase.Custom,
				Description: "Reset services",
				Data: &storage.OperationPhaseData{
					Update: &storage.UpdateOperationData{
						ClusterConfig: &storage.ClusterConfigData{
							ServiceSuffix: config.serviceSuffix,
							ServiceCIDR:   config.serviceCIDR,
							Services:      config.services,
						},
					},
				},
			},
		},
	}, nil
}

func (r builder) init(desc string) *update.Phase {
	return &update.Phase{
		ID:          "init",
		Executor:    phases.InitPhase,
		Description: desc,
		Data:        r.Builder.CustomUpdate.Data,
	}
}

func (r builder) fini(desc string) *update.Phase {
	return &update.Phase{
		ID:          "fini",
		Executor:    phases.FiniPhase,
		Description: desc,
		Data:        r.Builder.CustomUpdate.Data,
	}
}

type builder struct {
	rollingupdate.Builder
}
