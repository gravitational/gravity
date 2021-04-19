// Copyright 2016 Gravitational Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package rigging

import (
	"context"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// NewServiceControl returns new instance of Service updater
func NewServiceControl(config ServiceConfig) (*ServiceControl, error) {
	err := config.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &ServiceControl{
		ServiceConfig: config,
		FieldLogger: log.WithFields(log.Fields{
			"service": formatMeta(config.ObjectMeta),
		}),
	}, nil
}

// ServiceConfig  is a Service control configuration
type ServiceConfig struct {
	// Service specifies the existing service
	*v1.Service
	// Client is k8s client
	Client *kubernetes.Clientset
}

func (c *ServiceConfig) CheckAndSetDefaults() error {
	if c.Service == nil {
		return trace.BadParameter("missing parameter Service")
	}
	if c.Client == nil {
		return trace.BadParameter("missing parameter Client")
	}
	updateTypeMetaService(c.Service)
	return nil
}

// ServiceControl is a daemon set controller,
// adds various operations, like delete, status check and update
type ServiceControl struct {
	ServiceConfig
	log.FieldLogger
}

func (c *ServiceControl) Delete(ctx context.Context, cascade bool) error {
	c.Infof("delete %v", formatMeta(c.Service.ObjectMeta))

	err := c.Client.CoreV1().Services(c.Service.Namespace).Delete(ctx, c.Service.Name, metav1.DeleteOptions{})
	return ConvertError(err)
}

func (c *ServiceControl) Upsert(ctx context.Context) error {
	c.Infof("upsert %v", formatMeta(c.Service.ObjectMeta))

	services := c.Client.CoreV1().Services(c.Service.Namespace)
	currentService, err := services.Get(ctx, c.Service.Name, metav1.GetOptions{})
	err = ConvertError(err)
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		c.Service.UID = ""
		c.Service.SelfLink = ""
		c.Service.ResourceVersion = ""
		_, err = services.Create(ctx, c.Service, metav1.CreateOptions{})
		return ConvertError(err)
	}

	if checkCustomerManagedResource(currentService.Annotations) {
		c.WithField("service", formatMeta(c.ObjectMeta)).Info("Skipping update since object is customer managed.")
		return nil
	}

	c.Service.Spec.ClusterIP = currentService.Spec.ClusterIP
	c.Service.ResourceVersion = currentService.ResourceVersion
	_, err = services.Update(ctx, c.Service, metav1.UpdateOptions{})
	return ConvertError(err)
}

func (c *ServiceControl) Status(ctx context.Context) error {
	services := c.Client.CoreV1().Services(c.Service.Namespace)
	_, err := services.Get(ctx, c.Service.Name, metav1.GetOptions{})
	return ConvertError(err)
}

func updateTypeMetaService(r *v1.Service) {
	r.Kind = KindService
	if r.APIVersion == "" {
		r.APIVersion = v1.SchemeGroupVersion.String()
	}
}
