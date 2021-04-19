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

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/kubernetes/typed/admissionregistration/v1"
)

// NewValidatingWebhookConfigurationControl creates control service for
// ValidatingWebhookConfiguration resouces.
func NewValidatingWebhookConfigurationControl(config ValidatingWebhookConfigurationConfig) (*ValidatingWebhookConfigurationControl, error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &ValidatingWebhookConfigurationControl{
		ValidatingWebhookConfigurationConfig: config,
		FieldLogger: log.WithFields(log.Fields{
			"validating_webhook_configuration": formatMeta(config.ObjectMeta),
		}),
	}, nil
}

// ValidatingWebhookConfigurationConfig is the control configuration.
type ValidatingWebhookConfigurationConfig struct {
	*admissionregistrationv1.ValidatingWebhookConfiguration
	Client *kubernetes.Clientset
}

// CheckAndSetDefauls validates the config and sets defaults.
func (c *ValidatingWebhookConfigurationConfig) CheckAndSetDefaults() error {
	if c.ValidatingWebhookConfiguration == nil {
		return trace.BadParameter("missing parameter ValidatingWebhookConfiguration")
	}
	if c.Client == nil {
		return trace.BadParameter("missing parameter Client")
	}
	updateTypeMetaValidatingWebhookConfiguration(c.ValidatingWebhookConfiguration)
	return nil
}

// ValidatingWebhookConfigurationControl is the control service for
// ValidatingWebhookConfiguration resources.
type ValidatingWebhookConfigurationControl struct {
	ValidatingWebhookConfigurationConfig
	log.FieldLogger
}

// Delete deletes the resource.
func (c *ValidatingWebhookConfigurationControl) Delete(ctx context.Context, cascade bool) error {
	c.Infof("delete %v", formatMeta(c.ObjectMeta))
	err := c.client().Delete(ctx, c.Name, metav1.DeleteOptions{})
	return ConvertError(err)
}

// Upsert creates or updates the resource.
func (c *ValidatingWebhookConfigurationControl) Upsert(ctx context.Context) error {
	c.Infof("upsert %v", formatMeta(c.ObjectMeta))
	c.UID = ""
	c.SelfLink = ""
	c.ResourceVersion = ""
	currentWebhook, err := c.client().Get(ctx, c.Name, metav1.GetOptions{})
	if err != nil {
		if !trace.IsNotFound(ConvertError(err)) {
			return trace.Wrap(err)
		}
		_, err = c.client().Create(ctx, c.ValidatingWebhookConfiguration, metav1.CreateOptions{})
		return ConvertError(err)
	}

	if checkCustomerManagedResource(currentWebhook.Annotations) {
		c.WithField("webhook", formatMeta(c.ObjectMeta)).Info("Skipping update since object is customer managed.")
		return nil
	}

	c.ResourceVersion = currentWebhook.ResourceVersion
	_, err = c.client().Update(ctx, c.ValidatingWebhookConfiguration, metav1.UpdateOptions{})
	return ConvertError(err)
}

// Status checks whether the resource exists.
func (c *ValidatingWebhookConfigurationControl) Status(ctx context.Context) error {
	_, err := c.client().Get(ctx, c.Name, metav1.GetOptions{})
	return ConvertError(err)
}

func (c *ValidatingWebhookConfigurationControl) client() v1.ValidatingWebhookConfigurationInterface {
	return c.Client.AdmissionregistrationV1().ValidatingWebhookConfigurations()
}

// NewMutatingWebhookConfigurationControl creates control service for
// MutatingWebhookConfiguration resouces.
func NewMutatingWebhookConfigurationControl(config MutatingWebhookConfigurationConfig) (*MutatingWebhookConfigurationControl, error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &MutatingWebhookConfigurationControl{
		MutatingWebhookConfigurationConfig: config,
		FieldLogger: log.WithFields(log.Fields{
			"mutating_webhook_configuration": formatMeta(config.ObjectMeta),
		}),
	}, nil
}

// MutatingWebhookConfigurationConfig is the control configuration.
type MutatingWebhookConfigurationConfig struct {
	*admissionregistrationv1.MutatingWebhookConfiguration
	Client *kubernetes.Clientset
}

// CheckAndSetDefauls validates the config and sets defaults.
func (c *MutatingWebhookConfigurationConfig) CheckAndSetDefaults() error {
	if c.MutatingWebhookConfiguration == nil {
		return trace.BadParameter("missing parameter MutatingWebhookConfiguration")
	}
	if c.Client == nil {
		return trace.BadParameter("missing parameter Client")
	}
	updateTypeMetaMutatingWebhookConfiguration(c.MutatingWebhookConfiguration)
	return nil
}

// MutatingWebhookConfigurationControl is the control service for
// MutatingWebhookConfiguration resources.
type MutatingWebhookConfigurationControl struct {
	MutatingWebhookConfigurationConfig
	log.FieldLogger
}

// Delete deletes the resource.
func (c *MutatingWebhookConfigurationControl) Delete(ctx context.Context, cascade bool) error {
	c.Infof("delete %v", formatMeta(c.ObjectMeta))
	err := c.client().Delete(ctx, c.Name, metav1.DeleteOptions{})
	return ConvertError(err)
}

// Upsert creates or updates the resource.
func (c *MutatingWebhookConfigurationControl) Upsert(ctx context.Context) error {
	c.Infof("upsert %v", formatMeta(c.ObjectMeta))
	c.UID = ""
	c.SelfLink = ""
	c.ResourceVersion = ""
	currentWebhook, err := c.client().Get(ctx, c.Name, metav1.GetOptions{})
	if err != nil {
		if !trace.IsNotFound(ConvertError(err)) {
			return trace.Wrap(err)
		}
		_, err = c.client().Create(ctx, c.MutatingWebhookConfiguration, metav1.CreateOptions{})
		return ConvertError(err)
	}

	if checkCustomerManagedResource(currentWebhook.Annotations) {
		c.WithField("mutatingwebhook", formatMeta(c.ObjectMeta)).Info("Skipping update since object is customer managed.")
		return nil
	}

	c.ResourceVersion = currentWebhook.ResourceVersion
	_, err = c.client().Update(ctx, c.MutatingWebhookConfiguration, metav1.UpdateOptions{})
	return ConvertError(err)
}

// Status checks whether the resource exists.
func (c *MutatingWebhookConfigurationControl) Status(ctx context.Context) error {
	_, err := c.client().Get(ctx, c.Name, metav1.GetOptions{})
	return ConvertError(err)
}

func (c *MutatingWebhookConfigurationControl) client() v1.MutatingWebhookConfigurationInterface {
	return c.Client.AdmissionregistrationV1().MutatingWebhookConfigurations()
}

func updateTypeMetaValidatingWebhookConfiguration(webhook *admissionregistrationv1.ValidatingWebhookConfiguration) {
	webhook.Kind = KindValidatingWebhookConfiguration
	if webhook.APIVersion == "" {
		webhook.APIVersion = admissionregistrationv1.SchemeGroupVersion.String()
	}
}

func updateTypeMetaMutatingWebhookConfiguration(webhook *admissionregistrationv1.MutatingWebhookConfiguration) {
	webhook.Kind = KindMutatingWebhookConfiguration
	if webhook.APIVersion == "" {
		webhook.APIVersion = admissionregistrationv1.SchemeGroupVersion.String()
	}
}
