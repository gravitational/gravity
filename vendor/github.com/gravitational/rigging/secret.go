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

// NewSecretControl returns new instance of Secret updater
func NewSecretControl(config SecretConfig) (*SecretControl, error) {
	err := config.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var rc *v1.Secret
	if config.Secret != nil {
		rc = config.Secret
	} else {
		rc, err = ParseSecret(config.Reader)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	rc.Kind = KindSecret
	return &SecretControl{
		SecretConfig: config,
		secret:       *rc,
		Entry: log.WithFields(log.Fields{
			"secret": formatMeta(rc.ObjectMeta),
		}),
	}, nil
}

// SecretConfig  is a Secret control configuration
type SecretConfig struct {
	// Reader with daemon set to update, will be used if present
	Reader io.Reader
	// Secret is already parsed daemon set, will be used if present
	Secret *v1.Secret
	// Client is k8s client
	Client *kubernetes.Clientset
}

func (c *SecretConfig) CheckAndSetDefaults() error {
	if c.Reader == nil && c.Secret == nil {
		return trace.BadParameter("missing parameter Reader or Secret")
	}
	if c.Client == nil {
		return trace.BadParameter("missing parameter Client")
	}
	return nil
}

// SecretControl is a daemon set controller,
// adds various operations, like delete, status check and update
type SecretControl struct {
	SecretConfig
	secret v1.Secret
	*log.Entry
}

func (c *SecretControl) Delete(ctx context.Context, cascade bool) error {
	c.Infof("delete %v", formatMeta(c.secret.ObjectMeta))

	err := c.Client.Core().Secrets(c.secret.Namespace).Delete(c.secret.Name, nil)
	return ConvertError(err)
}

func (c *SecretControl) Upsert(ctx context.Context) error {
	c.Infof("upsert %v", formatMeta(c.secret.ObjectMeta))

	secrets := c.Client.Core().Secrets(c.secret.Namespace)
	c.secret.UID = ""
	c.secret.SelfLink = ""
	c.secret.ResourceVersion = ""
	_, err := secrets.Get(c.secret.Name, metav1.GetOptions{})
	err = ConvertError(err)
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		_, err = secrets.Create(&c.secret)
		return ConvertError(err)
	}
	_, err = secrets.Update(&c.secret)
	return ConvertError(err)
}

func (c *SecretControl) Status() error {
	secrets := c.Client.Core().Secrets(c.secret.Namespace)
	_, err := secrets.Get(c.secret.Name, metav1.GetOptions{})
	return ConvertError(err)
}
