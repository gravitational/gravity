package phases

import (
	"context"

	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/rigging"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

// NewServices returns a new services step implementation
func NewServices(params fsm.ExecutorParams, client corev1.CoreV1Interface, logger log.FieldLogger) (*Services, error) {
	step := Services{
		FieldLogger:       logger,
		client:            client,
		serviceName:       params.Phase.Data.Update.ClusterConfig.DNSServiceName,
		workerServiceName: params.Phase.Data.Update.ClusterConfig.DNSWorkerServiceName,
	}
	for _, service := range params.Phase.Data.Update.ClusterConfig.Services {
		if !isDNSService(service) {
			step.services = append(step.services, service)
			continue
		}
		if service.Name == dnsServiceName {
			step.dnsService = service
		} else {
			step.dnsWorkerService = service
		}
	}
	return &step, nil
}

// Execute resets the clusterIP for all the cluster services of type ClusterIP
// except DNS services.
// It renames the existing DNS services to keep them available for nodes that have not
// been upgraded to the new service subnet so the Pods scheduled on these nodes can still
// resolve cluster addresses using the old DNS service
func (r *Services) Execute(context.Context) error {
	if err := r.renameDNSServices(); err != nil {
		return trace.Wrap(err)
	}
	if err := r.resetServices(); err != nil {
		return trace.Wrap(err)
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

// Services implements the services step for the cluster configuration upgrade operation
type Services struct {
	log.FieldLogger
	client            corev1.CoreV1Interface
	serviceName       string
	workerServiceName string
	dnsService        v1.Service
	dnsWorkerService  v1.Service
	services          []v1.Service
}

func (r *Services) renameDNSServices() error {
	if err := r.renameService(r.dnsService, r.serviceName); err != nil {
		return trace.Wrap(err)
	}
	if err := r.renameService(r.dnsWorkerService, r.workerServiceName); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (r *Services) renameService(service v1.Service, newName string) error {
	services := r.client.Services(service.Namespace)
	err := services.Delete(service.Name, &metav1.DeleteOptions{})
	err = rigging.ConvertError(err)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	service.ResourceVersion = "0"
	service.Name = newName
	_, err = services.Create(&service)
	if err != nil {
		return rigging.ConvertError(err)
	}
	return nil
}

func (r *Services) resetServices() error {
	for _, service := range r.services {
		logger := r.WithField("service", formatMeta(&service))
		services := r.client.Services(service.Namespace)
		err := services.Delete(service.Name, &metav1.DeleteOptions{})
		err = rigging.ConvertError(err)
		if err != nil && !trace.IsNotFound(err) {
			return err
		}
		logger.Info("Recreate service with empty cluster IP.")
		// Let Kubernetes allocate cluster IP
		service.Spec.ClusterIP = ""
		_, err = services.Create(&service)
		if err != nil {
			return rigging.ConvertError(err)
		}
	}
	return nil
}

func isDNSService(service v1.Service) bool {
	return !(utils.StringInSlice(dnsServices, service.Name) && service.Namespace == metav1.NamespaceSystem)
}

func formatMeta(obj runtime.Object) string {
	return obj.GetObjectKind().GroupVersionKind().String()
}

var dnsServices = []string{
	dnsServiceName,
	dnsWorkerServiceName,
}

const (
	dnsServiceName       = "kube-dns"
	dnsWorkerServiceName = "kube-dns-worker"
)
