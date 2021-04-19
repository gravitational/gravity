/*
Copyright 2020 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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

func removeService(ctx context.Context, name string, opts metav1.DeleteOptions, services corev1.ServiceInterface) error {
	return utils.RetryTransient(ctx, newOperationBackoff(), func() error {
		err := services.Delete(ctx, name, opts)
		if err != nil && !errors.IsNotFound(err) {
			return rigging.ConvertError(err)
		}
		return nil
	})
}

func createServiceFromTemplate(ctx context.Context, service v1.Service, services corev1.ServiceInterface, logger log.FieldLogger) error {
	utils.LoggerWithService(service, logger).Debug("Recreate service with original cluster IP.")
	return utils.RetryWithInterval(ctx, newOperationBackoff(), func() error {
		service.ResourceVersion = "0"
		_, err := services.Create(ctx, &service, metav1.CreateOptions{})
		if err == nil || errors.IsAlreadyExists(err) {
			return nil
		}
		switch {
		case utils.IsTransientClusterError(err), isIPRangeMistmatchError(err):
			return rigging.ConvertError(err)
		default:
			return &backoff.PermanentError{Err: err}
		}
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
		_, err = services.Create(ctx, &service, metav1.CreateOptions{})
		if err == nil || errors.IsAlreadyExists(err) {
			return nil
		}
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
		return err.ErrStatus.Status == "Failure" &&
			statusHasCause(err.ErrStatus, "spec.clusterIP",
				"provided range does not match the current range",
				"provided IP is not in the valid range",
			)
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

func statusHasCause(status metav1.Status, field string, messages ...string) bool {
	if status.Details == nil {
		return false
	}
	for _, cause := range status.Details.Causes {
		for _, message := range messages {
			if cause.Field == field && strings.Contains(cause.Message, message) {
				return true
			}
		}
	}
	return false
}

func newOperationBackoff() backoff.BackOff {
	return utils.NewExponentialBackOff(5 * time.Minute)
}

// shouldManageService decides whether the configuration operation should
// manage the given service.
// The operation will manage a service, if it's of type clusterIP and is not a headless service
// (i.e. has a valid IP address assigned from the service subnet). Additionally, DNS services
// as well as the API server service are managed elsewhere and hence excluded
func shouldManageService(service v1.Service) bool {
	return !(utils.IsHeadlessService(service) || utils.IsAPIServerService(service) || isDNSService(service))
}

func isDNSService(service v1.Service) bool {
	return utils.StringInSlice(dnsServices, service.Name) && service.Namespace == metav1.NamespaceSystem
}

func (r serviceSuffix) serviceName() string {
	return fmt.Sprint(dnsServiceName, "-", r)
}

func (r serviceSuffix) workerServiceName() string {
	return fmt.Sprint(dnsWorkerServiceName, "-", r)
}

type serviceSuffix string

var dnsServices = []string{
	dnsServiceName,
	dnsWorkerServiceName,
}

const (
	dnsServiceName       = "kube-dns"
	dnsWorkerServiceName = "kube-dns-worker"
)
