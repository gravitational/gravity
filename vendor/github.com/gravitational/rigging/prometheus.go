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

// NewPrometheusControl returns new instance of Prometheus updater
func NewPrometheusControl(config PrometheusConfig) (*PrometheusControl, error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &PrometheusControl{
		PrometheusConfig: config,
		FieldLogger: log.WithFields(log.Fields{
			"prometheus": formatMeta(config.ObjectMeta),
		}),
	}, nil
}

// PrometheusConfig  is a Prometheus control configuration
type PrometheusConfig struct {
	// Prometheus is the parsed Prometheus resource
	*monitoringv1.Prometheus
	// Client is monitoring API client
	Client *monitoring.Clientset
}

// CheckAndSetDefaults validates the config
func (c *PrometheusConfig) CheckAndSetDefaults() error {
	if c.Prometheus == nil {
		return trace.BadParameter("missing parameter Prometheus")
	}
	if c.Client == nil {
		return trace.BadParameter("missing parameter Client")
	}
	updateTypeMetaPrometheus(c.Prometheus)
	return nil
}

// PrometheusControl a controller for Prometheus resources
type PrometheusControl struct {
	PrometheusConfig
	log.FieldLogger
}

func (c *PrometheusControl) Delete(ctx context.Context, cascade bool) error {
	c.Infof("delete %v", formatMeta(c.Prometheus.ObjectMeta))

	err := c.Client.MonitoringV1().Prometheuses(c.Prometheus.Namespace).Delete(ctx, c.Prometheus.Name, metav1.DeleteOptions{})
	return ConvertError(err)
}

func (c *PrometheusControl) Upsert(ctx context.Context) error {
	c.Infof("upsert %v", formatMeta(c.Prometheus.ObjectMeta))

	client := c.Client.MonitoringV1().Prometheuses(c.Prometheus.Namespace)
	c.Prometheus.UID = ""
	c.Prometheus.SelfLink = ""
	c.Prometheus.ResourceVersion = ""
	currentPrometheus, err := client.Get(ctx, c.Prometheus.Name, metav1.GetOptions{})
	err = ConvertError(err)
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		_, err = client.Create(ctx, c.Prometheus, metav1.CreateOptions{})
		return ConvertError(err)
	}

	if checkCustomerManagedResource(currentPrometheus.Annotations) {
		c.WithField("prometheus", formatMeta(c.ObjectMeta)).Info("Skipping update since object is customer managed.")
		return nil
	}

	c.Prometheus.ResourceVersion = currentPrometheus.ResourceVersion
	_, err = client.Update(ctx, c.Prometheus, metav1.UpdateOptions{})
	return ConvertError(err)
}

func (c *PrometheusControl) Status(ctx context.Context) error {
	client := c.Client.MonitoringV1().Prometheuses(c.Prometheus.Namespace)
	_, err := client.Get(ctx, c.Prometheus.Name, metav1.GetOptions{})
	return ConvertError(err)
}

func updateTypeMetaPrometheus(p *monitoringv1.Prometheus) {
	p.Kind = KindPrometheus
	if p.APIVersion == "" {
		p.APIVersion = monitoringv1.SchemeGroupVersion.String()
	}
}
