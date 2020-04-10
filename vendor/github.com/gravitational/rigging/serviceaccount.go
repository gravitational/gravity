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

// NewServiceAccountControl returns a new instance of the ServiceAccount controller
func NewServiceAccountControl(config ServiceAccountConfig) (*ServiceAccountControl, error) {
	err := config.checkAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &ServiceAccountControl{
		ServiceAccountConfig: config,
		FieldLogger: log.WithFields(log.Fields{
			"service_account": formatMeta(config.ObjectMeta),
		}),
	}, nil
}

// ServiceAccountConfig defines controller configuration
type ServiceAccountConfig struct {
	// ServiceAccount is the existing service account
	*v1.ServiceAccount
	// Client is k8s client
	Client *kubernetes.Clientset
}

func (c *ServiceAccountConfig) checkAndSetDefaults() error {
	if c.ServiceAccount == nil {
		return trace.BadParameter("missing parameter ServiceAccount")
	}
	if c.Client == nil {
		return trace.BadParameter("missing parameter Client")
	}
	updateTypeMetaServiceAccount(c.ServiceAccount)
	return nil
}

// ServiceAccountControl is a service accounts controller,
// adds various operations, like delete, status check and update
type ServiceAccountControl struct {
	ServiceAccountConfig
	log.FieldLogger
}

func (c *ServiceAccountControl) Delete(ctx context.Context, cascade bool) error {
	c.Infof("delete %v", formatMeta(c.ObjectMeta))

	err := c.Client.CoreV1().ServiceAccounts(c.Namespace).Delete(c.Name, nil)
	return ConvertError(err)
}

func (c *ServiceAccountControl) Upsert(ctx context.Context) error {
	c.Infof("upsert %v", formatMeta(c.ObjectMeta))

	accounts := c.Client.CoreV1().ServiceAccounts(c.Namespace)
	c.UID = ""
	c.SelfLink = ""
	c.ResourceVersion = ""
	_, err := accounts.Get(c.Name, metav1.GetOptions{})
	err = ConvertError(err)
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		_, err = accounts.Create(c.ServiceAccount)
		return ConvertError(err)
	}
	_, err = accounts.Update(c.ServiceAccount)
	return ConvertError(err)
}

func (c *ServiceAccountControl) Status() error {
	accounts := c.Client.CoreV1().ServiceAccounts(c.Namespace)
	_, err := accounts.Get(c.Name, metav1.GetOptions{})
	return ConvertError(err)
}

func updateTypeMetaServiceAccount(r *v1.ServiceAccount) {
	r.Kind = KindServiceAccount
	if r.APIVersion == "" {
		r.APIVersion = v1.SchemeGroupVersion.String()
	}
}
