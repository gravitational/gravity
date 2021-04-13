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
		if shouldManageService(service) {
			utils.LoggerWithService(service, logger).Debug("Found a service.")
			step.services = append(step.services, service)
			continue
		}
	}
	return &step, nil
}

// Execute resets the clusterIP for all the cluster services of type ClusterIP
// except services it does not need to handle/manage (eg kubernetes api server service
// and DNS/headless services)
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

// Services implements the services step for the cluster configuration upgrade operation.
// On the happy path, its job is to recreate the services of type clusterIP with the address from
// the new service CIDR.
// During rollback, it will remove the temporary DNS services created as part of the configuration
// update
type Services struct {
	log.FieldLogger
	client   corev1.CoreV1Interface
	services []v1.Service
	alloc    *ipallocator.Range
}

func (r *Services) removeDNSServices(ctx context.Context) error {
	services := r.client.Services(metav1.NamespaceSystem)
	for _, service := range dnsServices {
		err := removeService(ctx, service, metav1.DeleteOptions{}, services)
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
		err := removeService(ctx, service.Name, metav1.DeleteOptions{}, services)
		if err != nil {
			return trace.Wrap(err)
		}
		if err := createServiceWithClusterIP(ctx, service, r.alloc, services, logger); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}
