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
	"io"

	log "github.com/sirupsen/logrus"
	"github.com/gravitational/trace"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// NewServiceControl returns new instance of Service updater
func NewServiceControl(config ServiceConfig) (*ServiceControl, error) {
	err := config.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var rc *v1.Service
	if config.Service != nil {
		rc = config.Service
	} else {
		rc, err = ParseService(config.Reader)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	rc.Kind = KindService
	return &ServiceControl{
		ServiceConfig: config,
		service:       *rc,
		Entry: log.WithFields(log.Fields{
			"service": formatMeta(rc.ObjectMeta),
		}),
	}, nil
}

// ServiceConfig  is a Service control configuration
type ServiceConfig struct {
	// Reader with daemon set to update, will be used if present
	Reader io.Reader
	// Service is already parsed daemon set, will be used if present
	Service *v1.Service
	// Client is k8s client
	Client *kubernetes.Clientset
}

func (c *ServiceConfig) CheckAndSetDefaults() error {
	if c.Reader == nil && c.Service == nil {
		return trace.BadParameter("missing parameter Reader or Service")
	}
	if c.Client == nil {
		return trace.BadParameter("missing parameter Client")
	}
	return nil
}

// ServiceControl is a daemon set controller,
// adds various operations, like delete, status check and update
type ServiceControl struct {
	ServiceConfig
	service v1.Service
	*log.Entry
}

func (c *ServiceControl) Delete(ctx context.Context, cascade bool) error {
	c.Infof("delete %v", formatMeta(c.service.ObjectMeta))

	err := c.Client.Core().Services(c.service.Namespace).Delete(c.service.Name, nil)
	return ConvertError(err)
}

func (c *ServiceControl) Upsert(ctx context.Context) error {
	c.Infof("upsert %v", formatMeta(c.service.ObjectMeta))

	services := c.Client.Core().Services(c.service.Namespace)
	currentService, err := services.Get(c.service.Name, metav1.GetOptions{})
	err = ConvertError(err)
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		c.service.UID = ""
		c.service.SelfLink = ""
		c.service.ResourceVersion = ""
		_, err = services.Create(&c.service)
		return ConvertError(err)
	}
	c.service.Spec.ClusterIP = currentService.Spec.ClusterIP
	c.service.ResourceVersion = currentService.ResourceVersion
	_, err = services.Update(&c.service)
	return ConvertError(err)
}

func (c *ServiceControl) Status() error {
	services := c.Client.Core().Services(c.service.Namespace)
	_, err := services.Get(c.service.Name, metav1.GetOptions{})
	return ConvertError(err)
}
