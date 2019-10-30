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

// NewSecretControl returns new instance of Secret updater
func NewSecretControl(config SecretConfig) (*SecretControl, error) {
	err := config.checkAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &SecretControl{
		SecretConfig: config,
		FieldLogger: log.WithFields(log.Fields{
			"secret": formatMeta(config.Secret.ObjectMeta),
		}),
	}, nil
}

// SecretConfig  is a Secret control configuration
type SecretConfig struct {
	// Secret specifies the existing secret
	*v1.Secret
	// Client is k8s client
	Client *kubernetes.Clientset
}

func (c *SecretConfig) checkAndSetDefaults() error {
	if c.Secret == nil {
		return trace.BadParameter("missing parameter Secret")
	}
	if c.Client == nil {
		return trace.BadParameter("missing parameter Client")
	}
	updateTypeMetaSecret(c.Secret)
	return nil
}

// SecretControl is a daemon set controller,
// adds various operations, like delete, status check and update
type SecretControl struct {
	SecretConfig
	log.FieldLogger
}

func (c *SecretControl) Delete(ctx context.Context, cascade bool) error {
	c.Infof("delete %v", formatMeta(c.Secret.ObjectMeta))

	err := c.Client.CoreV1().Secrets(c.Secret.Namespace).Delete(c.Secret.Name, nil)
	return ConvertError(err)
}

func (c *SecretControl) Upsert(ctx context.Context) error {
	c.Infof("upsert %v", formatMeta(c.Secret.ObjectMeta))

	secrets := c.Client.CoreV1().Secrets(c.Secret.Namespace)
	c.Secret.UID = ""
	c.Secret.SelfLink = ""
	c.Secret.ResourceVersion = ""
	_, err := secrets.Get(c.Secret.Name, metav1.GetOptions{})
	err = ConvertError(err)
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		_, err = secrets.Create(c.Secret)
		return ConvertError(err)
	}
	_, err = secrets.Update(c.Secret)
	return ConvertError(err)
}

func (c *SecretControl) Status() error {
	secrets := c.Client.CoreV1().Secrets(c.Secret.Namespace)
	_, err := secrets.Get(c.Secret.Name, metav1.GetOptions{})
	return ConvertError(err)
}

func updateTypeMetaSecret(r *v1.Secret) {
	r.Kind = KindSecret
	if r.APIVersion == "" {
		r.APIVersion = v1.SchemeGroupVersion.String()
	}
}
