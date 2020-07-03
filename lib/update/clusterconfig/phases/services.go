package phases

import (
	"context"
	"net"

	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/network/ipallocator"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

// NewServices returns a new services step implementation
func NewServices(params fsm.ExecutorParams, client corev1.CoreV1Interface, logger log.FieldLogger) (*Services, error) {
	serviceCIDR := params.Phase.Data.Update.ClusterConfig.ServiceCIDR
	_, ipNet, err := net.ParseCIDR(serviceCIDR)
	if err != nil {
		return nil, trace.Wrap(err, "invalid service subnet: %q", serviceCIDR)
	}
	step := Services{
		FieldLogger: logger,
		client:      client,
		alloc:       ipallocator.NewAllocatorCIDRRange(ipNet),
	}
	for _, service := range params.Phase.Data.Update.ClusterConfig.Services {
		if !isSpecialService(service) {
			utils.WithService(service, logger).Debug("Found a service.")
			step.services = append(step.services, service)
			continue
		}
	}
	return &step, nil
}

// Execute resets the clusterIP for all the cluster services of type ClusterIP
// except services it does not need to handle/manage (eg kubernetes api server service
// and DNS/headless services).
// It renames the existing DNS services to keep them available for nodes that have not
// been upgraded to the new service subnet so the Pods scheduled on these nodes can still
// resolve cluster addresses using the old DNS service
func (r *Services) Execute(ctx context.Context) error {
	return trace.Wrap(r.resetServices(ctx))
}

// Rollback removes the temporary DNS services created in the new service subnet
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
	alloc            *ipallocator.Range
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
		logger := r.WithField("service", utils.FormatMeta(service.ObjectMeta))
		services := r.client.Services(service.Namespace)
		logger.Info("Remove service.")
		err := removeService(ctx, service.Name, &metav1.DeleteOptions{}, services)
		if err != nil {
			return trace.Wrap(err)
		}
		if err := createServiceWithClusterIP(ctx, service, r.alloc, services, logger); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}
