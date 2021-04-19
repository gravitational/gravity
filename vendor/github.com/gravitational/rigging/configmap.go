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

// NewConfigMapControl returns new instance of ConfigMap updater
func NewConfigMapControl(config ConfigMapConfig) (*ConfigMapControl, error) {
	err := config.checkAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &ConfigMapControl{
		ConfigMapConfig: config,
		Entry: log.WithFields(log.Fields{
			"configMap": formatMeta(config.ObjectMeta),
		}),
	}, nil
}

// ConfigMapConfig  is a ConfigMap control configuration
type ConfigMapConfig struct {
	// ConfigMap is already parsed daemon set, will be used if present
	*v1.ConfigMap
	// Client is k8s client
	Client *kubernetes.Clientset
}

func (c *ConfigMapConfig) checkAndSetDefaults() error {
	if c.ConfigMap == nil {
		return trace.BadParameter("missing parameter ConfigMap")
	}
	if c.Client == nil {
		return trace.BadParameter("missing parameter Client")
	}
	updateTypeMetaConfigMap(c.ConfigMap)
	return nil
}

// ConfigMapControl is a daemon set controller,
// adds various operations, like delete, status check and update
type ConfigMapControl struct {
	ConfigMapConfig
	*log.Entry
}

func (c *ConfigMapControl) Delete(ctx context.Context, cascade bool) error {
	c.Infof("delete %v", formatMeta(c.ConfigMap.ObjectMeta))

	err := c.Client.CoreV1().ConfigMaps(c.ConfigMap.Namespace).Delete(ctx, c.ConfigMap.Name, metav1.DeleteOptions{})
	return ConvertError(err)
}

func (c *ConfigMapControl) Upsert(ctx context.Context) error {
	c.Infof("upsert %v", formatMeta(c.ConfigMap.ObjectMeta))

	configMaps := c.Client.CoreV1().ConfigMaps(c.ConfigMap.Namespace)
	c.ConfigMap.UID = ""
	c.ConfigMap.SelfLink = ""
	c.ConfigMap.ResourceVersion = ""
	currentConfigMap, err := configMaps.Get(ctx, c.ConfigMap.Name, metav1.GetOptions{})
	err = ConvertError(err)
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		_, err = configMaps.Create(ctx, c.ConfigMap, metav1.CreateOptions{})
		return ConvertError(err)
	}

	if checkCustomerManagedResource(currentConfigMap.Annotations) {
		c.WithField("configmap", formatMeta(c.ConfigMap.ObjectMeta)).Info("Skipping update since object is customer managed.")
		return nil
	}

	_, err = configMaps.Update(ctx, c.ConfigMap, metav1.UpdateOptions{})
	return ConvertError(err)
}

func (c *ConfigMapControl) Status(ctx context.Context) error {
	configMaps := c.Client.CoreV1().ConfigMaps(c.ConfigMap.Namespace)
	_, err := configMaps.Get(ctx, c.ConfigMap.Name, metav1.GetOptions{})
	return ConvertError(err)
}

func updateTypeMetaConfigMap(r *v1.ConfigMap) {
	r.Kind = KindConfigMap
	if r.APIVersion == "" {
		r.APIVersion = v1.SchemeGroupVersion.String()
	}
}
