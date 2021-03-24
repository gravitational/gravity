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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	v1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	apiregistration "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset"
)

// NewAPIServiceControl returns new instance of APIService updater
func NewAPIServiceControl(config APIServiceConfig) (*APIServiceControl, error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &APIServiceControl{
		APIServiceConfig: config,
		FieldLogger: log.WithFields(log.Fields{
			"apiService": formatMeta(config.ObjectMeta),
		}),
	}, nil
}

// APIServiceConfig  is a APIService control configuration
type APIServiceConfig struct {
	// APIService is the parsed APIService resource
	*v1.APIService
	// Client is Kube aggregator clientset
	Client *apiregistration.Clientset
}

// CheckAndSetDefaults validates the config
func (c *APIServiceConfig) CheckAndSetDefaults() error {
	if c.APIService == nil {
		return trace.BadParameter("missing parameter APIService")
	}
	if c.Client == nil {
		return trace.BadParameter("missing parameter Client")
	}
	updateTypeMetaAPIService(c.APIService)
	return nil
}

// APIServiceControl a controller for APIService resources
type APIServiceControl struct {
	APIServiceConfig
	log.FieldLogger
}

func (c *APIServiceControl) Delete(ctx context.Context, cascade bool) error {
	c.Infof("delete %v", formatMeta(c.APIService.ObjectMeta))

	err := c.Client.ApiregistrationV1().APIServices().Delete(ctx, c.APIService.Name, metav1.DeleteOptions{})
	return ConvertError(err)
}

func (c *APIServiceControl) Upsert(ctx context.Context) error {
	c.Infof("upsert %v", formatMeta(c.APIService.ObjectMeta))

	client := c.Client.ApiregistrationV1().APIServices()
	c.APIService.UID = ""
	c.APIService.SelfLink = ""
	c.APIService.ResourceVersion = ""
	currentAPIService, err := client.Get(ctx, c.APIService.Name, metav1.GetOptions{})
	err = ConvertError(err)
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		_, err = client.Create(ctx, c.APIService, metav1.CreateOptions{})
		return ConvertError(err)
	}

	if checkCustomerManagedResource(currentAPIService.Annotations) {
		c.WithField("apiservice", formatMeta(c.APIService.ObjectMeta)).Info("Skipping update since object is customer managed.")
		return nil
	}

	c.APIService.ResourceVersion = currentAPIService.ResourceVersion
	_, err = client.Update(ctx, c.APIService, metav1.UpdateOptions{})
	return ConvertError(err)
}

func (c *APIServiceControl) Status(ctx context.Context) error {
	client := c.Client.ApiregistrationV1().APIServices()
	_, err := client.Get(ctx, c.APIService.Name, metav1.GetOptions{})
	return ConvertError(err)
}

func updateTypeMetaAPIService(s *v1.APIService) {
	s.Kind = KindAPIService
	if s.APIVersion == "" {
		s.APIVersion = apiregistrationv1.SchemeGroupVersion.String()
	}
}
