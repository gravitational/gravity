package clusterconfig

import (
	"fmt"

	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/update"
	"github.com/gravitational/gravity/lib/update/clusterconfig/phases"
	"github.com/gravitational/gravity/lib/update/internal/rollingupdate"

	utilrand "k8s.io/apimachinery/pkg/util/rand"
)

func newBuilder(app loc.Locator) builder {
	return builder{
		Builder: rollingupdate.Builder{App: app},
	}
}

func (r builder) init(desc string) *update.Phase {
	suffix := utilrand.String(4)
	serviceName := fmt.Sprintf("kube-dns-%v", suffix)
	workerServiceName := fmt.Sprintf("kube-dns-worker-%v", suffix)
	return &update.Phase{
		ID:          "init",
		Executor:    phases.InitPhase,
		Description: desc,
		Data: &storage.OperationPhaseData{
			Update: &storage.UpdateOperationData{
				ClusterConfig: &storage.ClusterConfigData{
					DNSServiceName:       serviceName,
					DNSWorkerServiceName: workerServiceName,
					// Services: services,
				},
			},
		},
	}
}

func (r builder) fini(desc string, init update.Phase) *update.Phase {
	return &update.Phase{
		ID:          "fini",
		Executor:    phases.FiniPhase,
		Description: desc,
		Data: &storage.OperationPhaseData{
			Update: &storage.UpdateOperationData{
				ClusterConfig: &storage.ClusterConfigData{
					DNSServiceName:       init.Data.Update.ClusterConfig.DNSServiceName,
					DNSWorkerServiceName: init.Data.Update.ClusterConfig.DNSWorkerServiceName,
				},
			},
		},
	}
}

type builder struct {
	rollingupdate.Builder
}
