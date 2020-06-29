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
	serviceSubnet, err := configure.ParseCIDR(params.Phase.Data.Update.ClusterConfig.ServiceSubnet)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Kubernetes defaults to the first IP in service range for the kubernetes service
	// See https://github.com/kubernetes/kubernetes/blob/v1.15.12/pkg/master/services.go#L41
	apiServerServiceIP := serviceSubnet.FirstIP().String()
	step := Services{
		FieldLogger:        logger,
		client:             client,
		apiServerServiceIP: apiServerServiceIP,
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
	if err := r.removeDNSServices(ctx); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(r.removeKubernetesService(ctx))
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
	client             corev1.CoreV1Interface
	dnsWorkerService   v1.Service
	services           []v1.Service
	subnet             configure.CIDR
	apiServerServiceIP string
}

func (r *Services) removeKubernetesService(ctx context.Context) error {
	r.Info("Remove kubernetes service.")
	err := removeService(ctx, kubernetesService, &metav1.DeleteOptions{},
		r.client.Services(metav1.NamespaceDefault))
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
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
		err = rigging.ConvertError(err)
		if err != nil && !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		logger.Info("Recreate service with empty cluster IP.")
		// Let Kubernetes allocate cluster IP
		service.Spec.ClusterIP = ""
		service.ResourceVersion = r.allocateServiceIP(service)
		_, err = services.Create(&service)
		if err != nil {
			return rigging.ConvertError(err)
		}
	}
	return nil
}

func (r *Services) allocateServiceIP(service v1.Service) (addr string) {
	if isKubernetesService(service) {
		return r.apiServerServiceIP
	}
	// Let Kubernetes allocate the IP
	return "0"
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
