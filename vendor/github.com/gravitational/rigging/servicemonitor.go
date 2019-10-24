// Copyright 2019 Gravitational Inc.
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

	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	monitoring "github.com/coreos/prometheus-operator/pkg/client/versioned"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewServiceMonitorControl returns new instance of ServiceMonitor updater
func NewServiceMonitorControl(config ServiceMonitorConfig) (*ServiceMonitorControl, error) {
	err := config.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &ServiceMonitorControl{
		ServiceMonitorConfig: config,
		FieldLogger: log.WithFields(log.Fields{
			"serviceMonitor": formatMeta(config.ObjectMeta),
		}),
	}, nil
}

// ServiceMonitorConfig  is a ServiceMonitor control configuration
type ServiceMonitorConfig struct {
	// ServiceMonitor is the parsed ServiceMonitor resource
	*monitoringv1.ServiceMonitor
	// Client is monitoring API client
	Client *monitoring.Clientset
}

// CheckAndSetDefaults validates the config
func (c *ServiceMonitorConfig) CheckAndSetDefaults() error {
	if c.ServiceMonitor == nil {
		return trace.BadParameter("missing parameter ServiceMonitor")
	}
	if c.Client == nil {
		return trace.BadParameter("missing parameter Client")
	}
	updateTypeMetaServiceMonitor(c.ServiceMonitor)
	return nil
}

// ServiceMonitorControl a controller for ServiceMonitor resources
type ServiceMonitorControl struct {
	ServiceMonitorConfig
	log.FieldLogger
}

func (c *ServiceMonitorControl) Delete(ctx context.Context, cascade bool) error {
	c.Infof("delete %v", formatMeta(c.ServiceMonitor.ObjectMeta))

	err := c.Client.MonitoringV1().ServiceMonitors(c.ServiceMonitor.Namespace).Delete(c.ServiceMonitor.Name, nil)
	return ConvertError(err)
}

func (c *ServiceMonitorControl) Upsert(ctx context.Context) error {
	c.Infof("upsert %v", formatMeta(c.ServiceMonitor.ObjectMeta))

	serviceMonitorsClient := c.Client.MonitoringV1().ServiceMonitors(c.ServiceMonitor.Namespace)
	c.ServiceMonitor.UID = ""
	c.ServiceMonitor.SelfLink = ""
	c.ServiceMonitor.ResourceVersion = ""
	currentServiceMonitor, err := serviceMonitorsClient.Get(c.ServiceMonitor.Name, metav1.GetOptions{})
	err = ConvertError(err)
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		_, err = serviceMonitorsClient.Create(c.ServiceMonitor)
		return ConvertError(err)
	}
	c.ServiceMonitor.ResourceVersion = currentServiceMonitor.ResourceVersion
	_, err = serviceMonitorsClient.Update(c.ServiceMonitor)
	return ConvertError(err)
}

func (c *ServiceMonitorControl) Status() error {
	client := c.Client.MonitoringV1().ServiceMonitors(c.ServiceMonitor.Namespace)
	_, err := client.Get(c.ServiceMonitor.Name, metav1.GetOptions{})
	return ConvertError(err)
}

func updateTypeMetaServiceMonitor(monitor *monitoringv1.ServiceMonitor) {
	monitor.Kind = KindServiceMonitor
	if monitor.APIVersion == "" {
		monitor.APIVersion = monitoringv1.SchemeGroupVersion.String()
	}
}
