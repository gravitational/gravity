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

// NewAlertmanagerControl returns new instance of Alertmanager updater
func NewAlertmanagerControl(config AlertmanagerConfig) (*AlertmanagerControl, error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &AlertmanagerControl{
		AlertmanagerConfig: config,
		FieldLogger: log.WithFields(log.Fields{
			"alertManager": formatMeta(config.ObjectMeta),
		}),
	}, nil
}

// AlertmanagerConfig  is a Alertmanager control configuration
type AlertmanagerConfig struct {
	// Alertmanager is the parsed Alertmanager resource
	*monitoringv1.Alertmanager
	// Client is monitoring API client
	Client *monitoring.Clientset
}

// CheckAndSetDefaults validates the config
func (c *AlertmanagerConfig) CheckAndSetDefaults() error {
	if c.Alertmanager == nil {
		return trace.BadParameter("missing parameter Alertmanager")
	}
	if c.Client == nil {
		return trace.BadParameter("missing parameter Client")
	}
	updateTypeMetaAlertmanager(c.Alertmanager)
	return nil
}

// AlertmanagerControl a controller for Alertmanager resources
type AlertmanagerControl struct {
	AlertmanagerConfig
	log.FieldLogger
}

func (c *AlertmanagerControl) Delete(ctx context.Context, cascade bool) error {
	c.Infof("delete %v", formatMeta(c.Alertmanager.ObjectMeta))

	err := c.Client.MonitoringV1().Alertmanagers(c.Alertmanager.Namespace).Delete(ctx, c.Alertmanager.Name, metav1.DeleteOptions{})
	return ConvertError(err)
}

func (c *AlertmanagerControl) Upsert(ctx context.Context) error {
	c.Infof("upsert %v", formatMeta(c.Alertmanager.ObjectMeta))

	client := c.Client.MonitoringV1().Alertmanagers(c.Alertmanager.Namespace)
	c.Alertmanager.UID = ""
	c.Alertmanager.SelfLink = ""
	c.Alertmanager.ResourceVersion = ""
	currentAlertmanager, err := client.Get(ctx, c.Alertmanager.Name, metav1.GetOptions{})
	err = ConvertError(err)
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		_, err = client.Create(ctx, c.Alertmanager, metav1.CreateOptions{})
		return ConvertError(err)
	}

	if checkCustomerManagedResource(currentAlertmanager.Annotations) {
		c.WithField("alertmanager", formatMeta(c.Alertmanager.ObjectMeta)).Info("Skipping update since object is customer managed.")
		return nil
	}

	c.Alertmanager.ResourceVersion = currentAlertmanager.ResourceVersion
	_, err = client.Update(ctx, c.Alertmanager, metav1.UpdateOptions{})
	return ConvertError(err)
}

func (c *AlertmanagerControl) Status(ctx context.Context) error {
	client := c.Client.MonitoringV1().Alertmanagers(c.Alertmanager.Namespace)
	_, err := client.Get(ctx, c.Alertmanager.Name, metav1.GetOptions{})
	return ConvertError(err)
}

func updateTypeMetaAlertmanager(am *monitoringv1.Alertmanager) {
	am.Kind = KindAlertmanager
	if am.APIVersion == "" {
		am.APIVersion = monitoringv1.SchemeGroupVersion.String()
	}
}
