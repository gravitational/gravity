/*
Copyright 2018 Gravitational, Inc.

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

package opsservice

import (
	"context"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/ops/events"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/rigging"
	"github.com/gravitational/trace"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

// GetSMTPConfig returns the cluster SMTP configuration
func (o *Operator) GetSMTPConfig(key ops.SiteKey) (storage.SMTPConfig, error) {
	client, err := o.GetKubeClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	data, err := getSMTPConfig(client.CoreV1().Secrets(defaults.MonitoringNamespace))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	config, err := storage.UnmarshalSMTPConfig(data)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return config, nil
}

// UpdateSMTPConfig updates the cluster SMTP configuration
func (o *Operator) UpdateSMTPConfig(ctx context.Context, key ops.SiteKey, config storage.SMTPConfig) error {
	client, err := o.GetKubeClient()
	if err != nil {
		return trace.Wrap(err)
	}
	err = updateSMTPConfig(client.CoreV1().Secrets(defaults.MonitoringNamespace), config)
	if err != nil {
		return trace.Wrap(err)
	}
	events.Emit(ctx, o, events.SMTPConfigCreated)
	return nil
}

// DeleteSMTPConfig deletes the cluster SMTP configuration
func (o *Operator) DeleteSMTPConfig(ctx context.Context, key ops.SiteKey) error {
	client, err := o.GetKubeClient()
	if err != nil {
		return trace.Wrap(err)
	}

	err = rigging.ConvertError(client.CoreV1().Secrets(defaults.MonitoringNamespace).
		Delete(ctx, constants.SMTPSecret, metav1.DeleteOptions{}))
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("no SMTP configuration found")
		}
		return trace.Wrap(err)
	}

	events.Emit(ctx, o, events.SMTPConfigDeleted)
	return nil
}

func getSMTPConfig(client corev1.SecretInterface) ([]byte, error) {
	secret, err := client.Get(context.TODO(), constants.SMTPSecret, metav1.GetOptions{})
	err = rigging.ConvertError(err)
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("no SMTP configuration found")
		}
		return nil, trace.Wrap(err)
	}

	data, ok := secret.Data[constants.ResourceSpecKey]
	if !ok {
		return nil, trace.NotFound("no SMTP configuration found")
	}

	return data, nil
}

func updateSMTPConfig(client corev1.SecretInterface, config storage.SMTPConfig) error {
	bytes, err := storage.MarshalSMTPConfig(config)
	if err != nil {
		return trace.Wrap(err)
	}

	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.SMTPSecret,
			Namespace: defaults.MonitoringNamespace,
			Labels: map[string]string{
				// Update SMTP configuration for monitoring
				constants.MonitoringType: constants.MonitoringTypeSMTP,
			},
		},
		Data: map[string][]byte{
			constants.ResourceSpecKey: bytes,
		},
		Type: v1.SecretTypeOpaque,
	}

	_, err = client.Create(context.TODO(), secret, metav1.CreateOptions{})
	err = rigging.ConvertError(err)
	if err == nil {
		return nil
	}

	if !trace.IsAlreadyExists(err) {
		return trace.Wrap(err)
	}

	_, err = client.Update(context.TODO(), secret, metav1.UpdateOptions{})
	return trace.Wrap(rigging.ConvertError(err))
}
