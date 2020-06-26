package phases

import (
	"context"

	"github.com/gravitational/gravity/lib/fsm"

	"github.com/gravitational/rigging"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

// NewInit returns a new init step implementation
func NewInit(params fsm.ExecutorParams, client corev1.CoreV1Interface, logger log.FieldLogger) (*Init, error) {
	return &Init{
		FieldLogger: logger,
		client:      client,
		services:    params.Phase.Data.Update.ClusterConfig.Services,
	}, nil
}

// Execute is a no-op for this phase
func (r *Init) Execute(context.Context) error {
	return nil
}

// Rollback resets the services to their original values
func (r *Init) Rollback(context.Context) error {
	return trace.Wrap(r.recreateServices())
}

// PreCheck is no-op for this phase
func (*Init) PreCheck(context.Context) error {
	return nil
}

// PostCheck is no-op for this phase
func (*Init) PostCheck(context.Context) error {
	return nil
}

// Init implements the init step for the cluster configuration upgrade operation
type Init struct {
	log.FieldLogger
	client   corev1.CoreV1Interface
	services []v1.Service
	// changeset rigging.Changeset
}

// TODO: use rigging to manage service state
func (r *Init) recreateServices() error {
	// TODO: r.changeset.Revert()
	for _, service := range r.services {
		services := r.client.Services(service.Namespace)
		if err := services.Delete(service.Name, &metav1.DeleteOptions{}); err != nil {
			err = rigging.ConvertError(err)
			if !trace.IsNotFound(err) {
				return trace.Wrap(err, "failed to delete service: %v", formatMeta(&service))
			}
		}
		service.ResourceVersion = "0"
		if _, err := services.Create(&service); err != nil {
			return trace.Wrap(rigging.ConvertError(err),
				"failed to create service: %v", formatMeta(&service))
		}
	}
	return nil
}
