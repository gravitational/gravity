package phases

import (
	"context"

	"github.com/gravitational/gravity/lib/fsm"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

// NewFini returns a new fini step implementation
func NewFini(params fsm.ExecutorParams, client corev1.CoreV1Interface, logger log.FieldLogger) (*Fini, error) {
	return &Fini{
		FieldLogger: logger,
		client:      client,
		suffix:      serviceSuffix(params.Phase.Data.Update.ClusterConfig.ServiceSuffix),
	}, nil
}

// Execute renames the new DNS services so they persist and removes the old services
func (r *Fini) Execute(ctx context.Context) error {
	services := r.client.Services(metav1.NamespaceSystem)
	for _, service := range []string{r.suffix.serviceName(), r.suffix.workerServiceName()} {
		if err := removeService(ctx, service, &metav1.DeleteOptions{}, services); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// Rollback removes the DNS services so they will be reset by agents on their way back
func (r *Fini) Rollback(ctx context.Context) error {
	services := r.client.Services(metav1.NamespaceSystem)
	for _, service := range []string{dnsServiceName, dnsWorkerServiceName} {
		if err := removeService(ctx, service, &metav1.DeleteOptions{}, services); err != nil {
			return trace.Wrap(err)
		}
	}
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
	client corev1.CoreV1Interface
	suffix serviceSuffix
}
