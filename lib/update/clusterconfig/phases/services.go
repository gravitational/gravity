package phases

import (
	"context"
	"fmt"

	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/configure"
	"github.com/gravitational/rigging"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

// NewServices returns a new services step implementation
func NewServices(params fsm.ExecutorParams, client corev1.CoreV1Interface, logger log.FieldLogger) (*Services, error) {
	step := Services{
		FieldLogger: logger,
		client:      client,
	}
	for _, service := range params.Phase.Data.Update.ClusterConfig.Services {
		if !isDNSService(service) && !isKubernetesService(service) {
			logger.WithField("service", fmt.Sprintf("%#v", service)).Info("Found a generic service.")
			step.services = append(step.services, service)
			continue
		}
	}
	logger.WithField("step", fmt.Sprintf("%#v", step)).Info("New services step.")
	return &step, nil
}

// Execute resets the clusterIP for all the cluster services of type ClusterIP
// except DNS services.
// It renames the existing DNS services to keep them available for nodes that have not
// been upgraded to the new service subnet so the Pods scheduled on these nodes can still
// resolve cluster addresses using the old DNS service
func (r *Services) Execute(ctx context.Context) error {
	return trace.Wrap(r.resetServices(ctx))
}

// Rollback reverts the DNS/kubernetes services created in the new service subnet
func (r *Services) Rollback(ctx context.Context) error {
	return trace.Wrap(r.removeDNSServices(ctx))
}

// PreCheck is no-op for this phase
func (*Services) PreCheck(context.Context) error {
	return nil
}

// PostCheck is no-op for this phase
func (*Services) PostCheck(context.Context) error {
	return nil
}

// Services implements the services step for the cluster configuration upgrade operation
type Services struct {
	log.FieldLogger
	client           corev1.CoreV1Interface
	dnsWorkerService v1.Service
	services         []v1.Service
	subnet           configure.CIDR
}

func (r *Services) removeDNSServices(ctx context.Context) error {
	services := r.client.Services(metav1.NamespaceSystem)
	for _, service := range dnsServices {
		err := removeService(ctx, service, &metav1.DeleteOptions{}, services)
		if err != nil && !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (r *Services) resetServices(ctx context.Context) error {
	for _, service := range r.services {
		logger := r.WithField("service", formatMeta(service.ObjectMeta))
		services := r.client.Services(service.Namespace)
		logger.Info("Remove service.")
		err := removeService(ctx, service.Name, &metav1.DeleteOptions{}, services)
		if err != nil && !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		// Let kubernetes allocate the new IP
		service.Spec.ClusterIP = ""
		logger.WithField("cluster-ip", service.Spec.ClusterIP).Info("Recreate service with new cluster IP.")
		service.ResourceVersion = "0"
		_, err = services.Create(&service)
		if err != nil {
			return rigging.ConvertError(err)
		}
	}
	return nil
}

func isKubernetesService(service v1.Service) bool {
	return service.Name == kubernetesService && service.Namespace == metav1.NamespaceDefault
}

func isDNSService(service v1.Service) bool {
	return utils.StringInSlice(dnsServices, service.Name) && service.Namespace == metav1.NamespaceSystem
}

func formatMeta(meta metav1.ObjectMeta) string {
	return fmt.Sprintf("%v/%v", meta.Namespace, meta.Name)
}

var dnsServices = []string{
	dnsServiceName,
	dnsWorkerServiceName,
}

const (
	dnsServiceName       = "kube-dns"
	dnsWorkerServiceName = "kube-dns-worker"
	kubernetesService    = "kubernetes"
)
