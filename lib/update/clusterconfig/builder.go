package clusterconfig

import (
	"fmt"

	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/update"
	"github.com/gravitational/gravity/lib/update/clusterconfig/phases"
	"github.com/gravitational/gravity/lib/update/internal/rollingupdate"
	libphase "github.com/gravitational/gravity/lib/update/internal/rollingupdate/phases"

	v1 "k8s.io/api/core/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
)

func newBuilder(app loc.Locator) *builder {
	return &builder{
		Builder: rollingupdate.Builder{
			App: app,
		},
	}
}

func newBuilderWithServices(app loc.Locator, services []v1.Service, serviceCIDR string) (*builder, error) {
	suffix := utilrand.String(4)
	return &builder{
		Builder: rollingupdate.Builder{
			App: app,
			CustomUpdate: &update.Phase{
				ID:          "services",
				Executor:    libphase.Custom,
				Description: "Reset services",
				Data: &storage.OperationPhaseData{
					Update: &storage.UpdateOperationData{
						ClusterConfig: &storage.ClusterConfigData{
							DNSServiceName:       fmt.Sprintf("kube-dns-%v", suffix),
							DNSWorkerServiceName: fmt.Sprintf("kube-dns-worker-%v", suffix),
							ServiceCIDR:          serviceCIDR,
							Services:             services,
						},
					},
				},
			},
		},
	}, nil
}

func (r builder) init(desc string) update.Phase {
	return update.Phase{
		ID:          "init",
		Executor:    phases.InitPhase,
		Description: desc,
		Data:        r.Builder.CustomUpdate.Data,
	}
}

func (r builder) fini(desc string) update.Phase {
	return update.Phase{
		ID:          "fini",
		Executor:    phases.FiniPhase,
		Description: desc,
		Data:        r.Builder.CustomUpdate.Data,
	}
}

type builder struct {
	rollingupdate.Builder
}
