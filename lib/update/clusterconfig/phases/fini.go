package phases

import (
	"context"

	"github.com/gravitational/gravity/lib/fsm"

	"github.com/gravitational/rigging"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

// NewFini returns a new fini step implementation
func NewFini(params fsm.ExecutorParams, client corev1.CoreV1Interface, logger log.FieldLogger) (*Fini, error) {
	return &Fini{
		FieldLogger:       logger,
		client:            client,
		serviceName:       params.Phase.Data.Update.ClusterConfig.DNSServiceName,
		workerServiceName: params.Phase.Data.Update.ClusterConfig.DNSWorkerServiceName,
	}, nil
}

// Execute renames the new dns services so they persist and removes the old services
func (r *Fini) Execute(context.Context) error {
	services := r.client.Services(metav1.NamespaceSystem)
	err := services.Delete(r.serviceName, &metav1.DeleteOptions{})
	err = rigging.ConvertError(err)
	if err != nil && !trace.IsNotFound(err) {
		return err
	}
	err = services.Delete(r.workerServiceName, &metav1.DeleteOptions{})
	err = rigging.ConvertError(err)
	if err != nil && !trace.IsNotFound(err) {
		return err
	}
	return nil
}

// Rollback is no-op for this phase
func (r *Fini) Rollback(context.Context) error {
	return nil
}

// PreCheck is no-op for this phase
func (*Fini) PreCheck(context.Context) error {
	return nil
}

// PostCheck is no-op for this phase
func (*Fini) PostCheck(context.Context) error {
	return nil
}

// Fini implements the fini step for the cluster configuration upgrade operation
type Fini struct {
	log.FieldLogger
	client            corev1.CoreV1Interface
	serviceName       string
	workerServiceName string
}
