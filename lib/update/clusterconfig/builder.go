package clusterconfig

import (
	"fmt"

	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/update"
	"github.com/gravitational/gravity/lib/update/clusterconfig/phases"
	"github.com/gravitational/gravity/lib/update/internal/rollingupdate"
	libphase "github.com/gravitational/gravity/lib/update/internal/rollingupdate/phases"
	"github.com/gravitational/trace"

	utilrand "k8s.io/apimachinery/pkg/util/rand"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

func newBuilder(app loc.Locator) *builder {
	return &builder{
		Builder: rollingupdate.Builder{
			App: app,
		},
	}
}

func newBuilderWithServices(app loc.Locator, client corev1.CoreV1Interface) (*builder, error) {
	services, err := collectServices(client)
	if err != nil {
		return nil, trace.Wrap(err)
	}
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
		Data: &storage.OperationPhaseData{
			Update: &storage.UpdateOperationData{
				ClusterConfig: &storage.ClusterConfigData{
					DNSServiceName:       r.Builder.CustomUpdate.Data.Update.ClusterConfig.DNSServiceName,
					DNSWorkerServiceName: r.Builder.CustomUpdate.Data.Update.ClusterConfig.DNSWorkerServiceName,
					Services:             r.Builder.CustomUpdate.Data.Update.ClusterConfig.Services,
				},
			},
		},
	}
}

func (r builder) fini(desc string) update.Phase {
	return update.Phase{
		ID:          "fini",
		Executor:    phases.FiniPhase,
		Description: desc,
		Data: &storage.OperationPhaseData{
			Update: &storage.UpdateOperationData{
				ClusterConfig: &storage.ClusterConfigData{
					DNSServiceName:       r.Builder.CustomUpdate.Data.Update.ClusterConfig.DNSServiceName,
					DNSWorkerServiceName: r.Builder.CustomUpdate.Data.Update.ClusterConfig.DNSWorkerServiceName,
					Services:             r.Builder.CustomUpdate.Data.Update.ClusterConfig.Services,
				},
			},
		},
	}
}

type builder struct {
	rollingupdate.Builder
}
