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

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewCustomResourceDefinitionControl returns new instance of CustomResourceDefinition updater
func NewCustomResourceDefinitionControl(
	config CustomResourceDefinitionConfig) (*CustomResourceDefinitionControl, error) {
	err := config.checkAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &CustomResourceDefinitionControl{
		CustomResourceDefinitionConfig: config,
		Entry: log.WithFields(log.Fields{
			"CustomResourceDefinition": formatMeta(config.ObjectMeta),
		}),
	}, nil
}

// CustomResourceDefinitionConfig  is a CustomResourceDefinition control configuration
type CustomResourceDefinitionConfig struct {
	// CustomResourceDefinition is already parsed daemon set, will be used if present
	*v1beta1.CustomResourceDefinition
	// Client is k8s client
	Client *apiextensionsclientset.Clientset
}

func (c *CustomResourceDefinitionConfig) checkAndSetDefaults() error {
	if c.CustomResourceDefinition == nil {
		return trace.BadParameter("missing parameter CustomResourceDefinition")
	}
	if c.Client == nil {
		return trace.BadParameter("missing parameter Client")
	}
	updateTypeMetaCustomResourceDefinition(c.CustomResourceDefinition)
	return nil
}

// CustomResourceDefinitionControl is a daemon set controller,
// adds various operations, like delete, status check and update
type CustomResourceDefinitionControl struct {
	CustomResourceDefinitionConfig
	*log.Entry
}

func (c *CustomResourceDefinitionControl) Delete(ctx context.Context, cascade bool) error {
	c.Infof("delete %v", formatMeta(c.CustomResourceDefinition.ObjectMeta))

	err := c.Client.ApiextensionsV1beta1().CustomResourceDefinitions().Delete(ctx, c.CustomResourceDefinition.Name, metav1.DeleteOptions{})
	return ConvertError(err)
}

func (c *CustomResourceDefinitionControl) Upsert(ctx context.Context) error {
	c.Infof("upsert %v", formatMeta(c.CustomResourceDefinition.ObjectMeta))

	CustomResourceDefinitions := c.Client.ApiextensionsV1beta1().CustomResourceDefinitions()
	c.CustomResourceDefinition.UID = ""
	c.CustomResourceDefinition.SelfLink = ""
	c.CustomResourceDefinition.ResourceVersion = ""
	existing, err := CustomResourceDefinitions.Get(ctx, c.CustomResourceDefinition.Name, metav1.GetOptions{})
	err = ConvertError(err)
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		_, err = CustomResourceDefinitions.Create(ctx, c.CustomResourceDefinition, metav1.CreateOptions{})
		return ConvertError(err)
	}

	if checkCustomerManagedResource(existing.Annotations) {
		c.WithField("customresourcedefinition", formatMeta(c.CustomResourceDefinition.ObjectMeta)).Info("Skipping update since object is customer managed.")
		return nil
	}

	c.CustomResourceDefinition.ResourceVersion = existing.ResourceVersion
	_, err = CustomResourceDefinitions.Update(ctx, c.CustomResourceDefinition, metav1.UpdateOptions{})
	return ConvertError(err)
}

func (c *CustomResourceDefinitionControl) Status(ctx context.Context) error {
	CustomResourceDefinitions := c.Client.ApiextensionsV1beta1().CustomResourceDefinitions()
	_, err := CustomResourceDefinitions.Get(ctx, c.CustomResourceDefinition.Name, metav1.GetOptions{})
	return ConvertError(err)
}

func updateTypeMetaCustomResourceDefinition(r *v1beta1.CustomResourceDefinition) {
	r.Kind = KindCustomResourceDefinition
	if r.APIVersion == "" {
		r.APIVersion = v1.SchemeGroupVersion.String()
	}
}
