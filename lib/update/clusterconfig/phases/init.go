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

	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/rigging"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

// NewInit returns a new init step implementation
func NewInit(params fsm.ExecutorParams, client corev1.CoreV1Interface, logger log.FieldLogger) (*Init, error) {
	step := Init{
		FieldLogger: logger,
		client:      client,
		suffix:      serviceSuffix(params.Phase.Data.Update.ClusterConfig.ServiceSuffix),
	}
	for _, service := range params.Phase.Data.Update.ClusterConfig.Services {
		if shouldManageService(service) {
			utils.LoggerWithService(service, logger).Debug("Found a service.")
			step.services = append(step.services, service)
			continue
		}
		if service.Name == dnsServiceName {
			step.dnsService = service
		} else if service.Name == dnsWorkerServiceName {
			step.dnsWorkerService = service
		}
	}
	return &step, nil
}

// Execute renames the existing DNS services to keep them available for nodes that have not
// been upgraded to the new service subnet so the Pods scheduled on these nodes can still
// resolve cluster addresses using the old DNS service
func (r *Init) Execute(ctx context.Context) error {
	return trace.Wrap(r.renameDNSServices(ctx))
}

// Rollback resets the services to their original values
func (r *Init) Rollback(ctx context.Context) error {
	if err := r.resetDNSServices(ctx); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(r.recreateServices(ctx))
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
	client corev1.CoreV1Interface
	// suffix specifies the temporary (operation-bound) DNS service suffix
	suffix serviceSuffix
	// dnsService references the original DNS service
	dnsService v1.Service
	// dnsWorkerService references the original DNS worker service
	dnsWorkerService v1.Service
	// services lists all other cluster services except DNS and kubernetes services
	services []v1.Service
}

func (r *Init) renameDNSServices(ctx context.Context) error {
	if err := r.renameService(ctx, r.dnsService, r.suffix.serviceName()); err != nil {
		return trace.Wrap(err)
	}
	if err := r.renameService(ctx, r.dnsWorkerService, r.suffix.workerServiceName()); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (r *Init) renameService(ctx context.Context, service v1.Service, newName string) error {
	r.WithField("service", utils.FormatMeta(service.ObjectMeta)).Info("Rename service.")
	services := r.client.Services(service.Namespace)
	existingName := service.Name
	service.Name = newName
	if err := r.recreateService(ctx, existingName, service, services); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// resetDNSServices will rename the old DNS services back to their original names
func (r *Init) resetDNSServices(ctx context.Context) error {
	if err := r.renameService(ctx, r.dnsService, dnsServiceName); err != nil {
		return trace.Wrap(err)
	}
	if err := r.renameService(ctx, r.dnsWorkerService, dnsWorkerServiceName); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (r *Init) recreateServices(ctx context.Context) error {
	for _, service := range r.services {
		services := r.client.Services(service.Namespace)
		if err := r.recreateService(ctx, service.Name, service, services); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (r *Init) recreateService(ctx context.Context, name string, service v1.Service, services corev1.ServiceInterface) error {
	if name != service.Name {
		r.WithField("old", name).WithField("new", service.Name).Info("Rename service.")
	} else {
		r.WithField("service", utils.FormatMeta(service.ObjectMeta)).Info("Recreate service.")
	}
	if err := removeService(ctx, name, metav1.DeleteOptions{}, services); err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err, "failed to delete service: %v/%v", service.Namespace, name)
	}
	service.ResourceVersion = "0"
	if err := createServiceFromTemplate(ctx, service, services, r.FieldLogger); err != nil {
		return trace.Wrap(rigging.ConvertError(err),
			"failed to create service: %v", utils.FormatMeta(service.ObjectMeta))
	}
	return nil
}
