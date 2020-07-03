package phases

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gravitational/gravity/lib/network/ipallocator"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/cenkalti/backoff"
	"github.com/gravitational/rigging"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

func removeService(ctx context.Context, name string, opts *metav1.DeleteOptions, services corev1.ServiceInterface) error {
	return utils.RetryTransient(ctx, newOperationBackoff(), func() error {
		err := services.Delete(name, opts)
		if err != nil && !errors.IsNotFound(err) {
			return rigging.ConvertError(err)
		}
		return nil
	})
}

func createServiceFromTemplate(ctx context.Context, service v1.Service, services corev1.ServiceInterface, logger log.FieldLogger) error {
	logger.Info("Recreate service with original cluster IP.")
	return utils.RetryTransient(ctx, newOperationBackoff(), func() error {
		service.ResourceVersion = "0"
		_, err := services.Create(&service)
		if err != nil && !errors.IsAlreadyExists(err) {
			return rigging.ConvertError(err)
		}
		return nil
	})
}

func createServiceWithClusterIP(ctx context.Context, service v1.Service, alloc *ipallocator.Range, services corev1.ServiceInterface, logger log.FieldLogger) error {
	ip, err := alloc.AllocateNext()
	if err != nil {
		return trace.Wrap(err, "failed to allocate service IP")
	}
	return utils.RetryWithInterval(ctx, newOperationBackoff(), func() error {
		service.Spec.ClusterIP = ip.String()
		logger.WithField("cluster-ip", service.Spec.ClusterIP).Info("Recreate service with cluster IP.")
		service.ResourceVersion = "0"
		_, err = services.Create(&service)
		if err == nil || errors.IsAlreadyExists(err) {
			return nil
		}
		logger.WithField("status-error", dumpStatusError(err)).Info("Service create error.")
		switch {
		case utils.IsTransientClusterError(err), isIPRangeMistmatchError(err):
			// Fall-through
		case isIPAlreadyAllocatedError(err):
			alloc.Release(ip)
			ip, err = alloc.AllocateNext()
			if err != nil {
				return &backoff.PermanentError{Err: err}
			}
			// Fall-through
		default:
			return &backoff.PermanentError{Err: err}
		}
		// Retry on transient errors
		return rigging.ConvertError(err)
	})
}

// isIPRangeMistmatchError detects whether the given error indicates that the suggested cluster IP
// is from an unexpected service IP range. This can happen as long as the apiserver's repair
// step did not commit the new service IP range configuration to the store (eg etcd)
func isIPRangeMistmatchError(err error) bool {
	switch err := err.(type) {
	case *errors.StatusError:
		return err.ErrStatus.Status == "Failure" && statusHasCause(err.ErrStatus,
			"spec.clusterIP", "provided range does not match the current range")
	}
	return false
}

// isIPAlreadyAllocatedError detects whether the given error indicates that the specified
// cluster IP is already allocated.
// This can happen since we are not syncing the IP allocation with the apiserver
func isIPAlreadyAllocatedError(err error) bool {
	switch err := err.(type) {
	case *errors.StatusError:
		return err.ErrStatus.Status == "Failure" && statusHasCause(err.ErrStatus,
			"spec.clusterIP", "provided IP is already allocated")
	}
	return false
}

func statusHasCause(status metav1.Status, field, messagePattern string) bool {
	if status.Details == nil {
		return false
	}
	for _, cause := range status.Details.Causes {
		if cause.Field == field && strings.Contains(cause.Message, messagePattern) {
			return true
		}
	}
	return false
}

func dumpStatusError(err error) string {
	switch err := err.(type) {
	case *errors.StatusError:
		return fmt.Sprintf("Error: %#v, Details: %#v", err, err.ErrStatus.Details)
	}
	return fmt.Sprintf("%#v", err)
}

func newOperationBackoff() backoff.BackOff {
	return utils.NewExponentialBackOff(5 * time.Minute)
}

func isSpecialService(service v1.Service) bool {
	return isHeadlessService(service) || isKubernetesService(service) || isDNSService(service)
}

func isHeadlessService(service v1.Service) bool {
	return service.Spec.ClusterIP == headlessServiceClusterIP
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

	headlessServiceClusterIP = "None"
)
