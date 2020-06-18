package phases

import (
	"context"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/storage/clusterconfig"

	"github.com/gravitational/rigging"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

// NewServices returns a new services step implementation
func NewServices(params fsm.ExecutorParams, logger log.FieldLogger) (*Services, error) {
	// TODO
	return &Services{
		FieldLogger: logger,
	}, nil
}

// Execute creates alias DNS services in the new service subnet
func (r *Services) Execute(context.Context) error {
	services := r.client.Services(metav1.NamespaceSystem)
	_, err := services.Create(newDNSService(r.serviceName, dnsServiceSelector, r.serviceIP))
	err = rigging.ConvertError(err)
	if err != nil && !trace.IsAlreadyExists(err) {
		return err
	}
	_, err = services.Create(newDNSService(r.workerServiceName, dnsWorkerServiceSelector, r.workerServiceIP))
	err = rigging.ConvertError(err)
	if err != nil && !trace.IsAlreadyExists(err) {
		return err
	}
	return nil
}

// Rollback removes the alias DNS services created as part of this step
func (r *Services) Rollback(context.Context) error {
	services := r.client.Services(metav1.NamespaceSystem)
	err := services.Delete(r.serviceName, nil)
	err = rigging.ConvertError(err)
	if err != nil && !trace.IsNotFound(err) {
		return rigging.ConvertError(err)
	}
	err = services.Delete(r.workerServiceName, nil)
	err = rigging.ConvertError(err)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	return nil
}

// PreCheck is no-op for this phase
func (*Services) PreCheck(context.Context) error {
	return nil
}

// PostCheck is no-op for this phase
func (*Services) PostCheck(context.Context) error {
	return nil
}

// Services implements the init step for the cluster configuration upgrade operation
type Services struct {
	log.FieldLogger
	client            corev1.CoreV1Client
	serviceName       string
	serviceIP         string
	workerServiceName string
	workerServiceIP   string
	config            clusterconfig.Interface
}

func newDNSService(name, selector, clusterIP string) *v1.Service {
	return &v1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       constants.KindService,
			APIVersion: constants.ServiceAPIVersion,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: metav1.NamespaceSystem,
			Labels: map[string]string{
				"k8s-app":                       selector,
				"kubernetes.io/cluster-service": "true",
				"kubernetes.io/name":            "CoreDNS",
			},
		},
		Spec: v1.ServiceSpec{
			Type: v1.ServiceTypeLoadBalancer,
			Selector: map[string]string{
				"k8s-app": selector,
			},
			Ports: []v1.ServicePort{
				{
					Name:     "dns",
					Protocol: "udp",
					Port:     53,
				},
				{
					Name:     "dns-tcp",
					Protocol: "tcp",
					Port:     53,
				},
			},
			ClusterIP: clusterIP,
		},
	}
}

const (
	dnsServiceSelector       = "kube-dns"
	dnsWorkerServiceSelector = "kube-dns-worker"
)
