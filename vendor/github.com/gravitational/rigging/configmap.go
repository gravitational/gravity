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

// NewConfigMapControl returns new instance of ConfigMap updater
func NewConfigMapControl(config ConfigMapConfig) (*ConfigMapControl, error) {
	err := config.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var rc *v1.ConfigMap
	if config.ConfigMap != nil {
		rc = config.ConfigMap
	} else {
		rc, err = ParseConfigMap(config.Reader)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	rc.Kind = KindConfigMap
	return &ConfigMapControl{
		ConfigMapConfig: config,
		configMap:       *rc,
		Entry: log.WithFields(log.Fields{
			"configMap": formatMeta(rc.ObjectMeta),
		}),
	}, nil
}

// ConfigMapConfig  is a ConfigMap control configuration
type ConfigMapConfig struct {
	// Reader with daemon set to update, will be used if present
	Reader io.Reader
	// ConfigMap is already parsed daemon set, will be used if present
	ConfigMap *v1.ConfigMap
	// Client is k8s client
	Client *kubernetes.Clientset
}

func (c *ConfigMapConfig) CheckAndSetDefaults() error {
	if c.Reader == nil && c.ConfigMap == nil {
		return trace.BadParameter("missing parameter Reader or ConfigMap")
	}
	if c.Client == nil {
		return trace.BadParameter("missing parameter Client")
	}
	return nil
}

// ConfigMapControl is a daemon set controller,
// adds various operations, like delete, status check and update
type ConfigMapControl struct {
	ConfigMapConfig
	configMap v1.ConfigMap
	*log.Entry
}

func (c *ConfigMapControl) Delete(ctx context.Context, cascade bool) error {
	c.Infof("delete %v", formatMeta(c.configMap.ObjectMeta))

	err := c.Client.Core().ConfigMaps(c.configMap.Namespace).Delete(c.configMap.Name, nil)
	return ConvertError(err)
}

func (c *ConfigMapControl) Upsert(ctx context.Context) error {
	c.Infof("upsert %v", formatMeta(c.configMap.ObjectMeta))

	configMaps := c.Client.Core().ConfigMaps(c.configMap.Namespace)
	c.configMap.UID = ""
	c.configMap.SelfLink = ""
	c.configMap.ResourceVersion = ""
	_, err := configMaps.Get(c.configMap.Name, metav1.GetOptions{})
	err = ConvertError(err)
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		_, err = configMaps.Create(&c.configMap)
		return ConvertError(err)
	}
	_, err = configMaps.Update(&c.configMap)
	return ConvertError(err)
}

func (c *ConfigMapControl) Status() error {
	configMaps := c.Client.Core().ConfigMaps(c.configMap.Namespace)
	_, err := configMaps.Get(c.configMap.Name, metav1.GetOptions{})
	return ConvertError(err)
}
