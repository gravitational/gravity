/*
Copyright 2019 Gravitational, Inc.

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
	"fmt"
	"time"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/ops/events"
	"github.com/gravitational/gravity/lib/storage"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	appsv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"

	"github.com/gravitational/rigging"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// GetPersistentStorage retrieves the current persistent storage configuration.
func (o *Operator) GetPersistentStorage(ctx context.Context, key ops.SiteKey) (storage.PersistentStorage, error) {
	if o.cfg.OpenEBS == nil {
		return nil, trace.BadParameter("persistent storage is not configured")
	}
	ndmConfig, err := o.cfg.OpenEBS.GetNDMConfig()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return storage.PersistentStorageFromNDMConfig(ndmConfig), nil
}

// UpdatePersistentStorage updates cluster persistent storage configuration.
func (o *Operator) UpdatePersistentStorage(ctx context.Context, req ops.UpdatePersistentStorageRequest) error {
	if o.cfg.OpenEBS == nil {
		return trace.BadParameter("persistent storage is not configured")
	}
	ndmConfig, err := o.cfg.OpenEBS.GetNDMConfig()
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	if trace.IsNotFound(err) {
		ndmConfig = storage.DefaultNDMConfig()
	}
	ndmConfig.Apply(req.Resource)
	err = o.cfg.OpenEBS.UpdateNDMConfig(ndmConfig)
	if err != nil {
		return trace.Wrap(err)
	}
	events.Emit(ctx, o, events.PersistentStorageUpdated)
	err = o.cfg.OpenEBS.RestartNDM()
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// OpenEBSControl provides interface for managing OpenEBS in the cluster.
type OpenEBSControl interface {
	// GetNDMConfig returns node device manager configuration.
	GetNDMConfig() (*storage.NDMConfig, error)
	// UpdateNDMConfig updates node device manager configuration.
	UpdateNDMConfig(*storage.NDMConfig) error
	// RestartNDM restarts node device manager pods.
	RestartNDM() error
}

type openEBSControl struct {
	// cm is config map interface in the configured namespace.
	cm corev1.ConfigMapInterface
	// ds is daemon set interface in the configured namespace.
	ds appsv1.DaemonSetInterface
	// cmName is the node device manager config map name.
	cmName string
	// dsName if the node device manager daemon set name.
	dsName string
	// FieldLogger is used for logging.
	logrus.FieldLogger
}

// OpenEBSConfig is the OpenEBS controller configuration.
type OpenEBSConfig struct {
	// Client is the Kubernetes client.
	Client *kubernetes.Clientset
	// Namespace is the namespace where OpenEBS components reside.
	Namespace string
	// ConfigMapName is the name of the config map with node device manager configuration.
	ConfigMapName string
	// DaemonSetName is the name of the node device manager daemon set.
	DaemonSetName string
}

// CheckAndSetDefaults validates the config and sets defaults.
func (c *OpenEBSConfig) CheckAndSetDefaults() error {
	if c.Client == nil {
		return trace.BadParameter("missing Kubernetes client")
	}
	if c.Namespace == "" {
		c.Namespace = defaults.OpenEBSNamespace
	}
	if c.ConfigMapName == "" {
		c.ConfigMapName = constants.OpenEBSNDMConfigMap
	}
	if c.DaemonSetName == "" {
		c.DaemonSetName = constants.OpenEBSNDMDaemonSet
	}
	return nil
}

// NewOpenEBSContol returns a new OpenEBS controller for the provided client.
func NewOpenEBSControl(config OpenEBSConfig) (*openEBSControl, error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &openEBSControl{
		FieldLogger: logrus.WithField(trace.Component, "openebs"),
		cm:          config.Client.CoreV1().ConfigMaps(config.Namespace),
		ds:          config.Client.AppsV1().DaemonSets(config.Namespace),
		cmName:      config.ConfigMapName,
		dsName:      config.DaemonSetName,
	}, nil
}

// GetNDMConfig returns node device manager configuration.
func (c *openEBSControl) GetNDMConfig() (*storage.NDMConfig, error) {
	cm, err := c.cm.Get(context.TODO(), c.cmName, metav1.GetOptions{})
	if err != nil {
		return nil, rigging.ConvertError(err)
	}
	config, err := storage.NDMConfigFromConfigMap(cm)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return config, nil
}

// UpdateNDMConfig updates node device manager configuration.
func (c *openEBSControl) UpdateNDMConfig(config *storage.NDMConfig) error {
	cm, err := config.ToConfigMap()
	if err != nil {
		return trace.Wrap(err)
	}
	c.Infof("Updating NDM config map: %v.", cm.Data)
	_, err = c.cm.Update(context.TODO(), cm, metav1.UpdateOptions{})
	if err != nil {
		return rigging.ConvertError(err)
	}
	return nil
}

// RestartNDM restarts node device manager pods.
func (c *openEBSControl) RestartNDM() error {
	c.Info("Restarting NDM daemon set.")
	_, err := c.ds.Patch(context.TODO(), c.dsName, types.StrategicMergePatchType,
		formatRestartPatch(), metav1.PatchOptions{})
	if err != nil {
		return rigging.ConvertError(err)
	}
	return nil
}

// formatRestartPatch returns the patch that sets the restartedAt annotation
// on the daemon set object.
func formatRestartPatch() []byte {
	return []byte(fmt.Sprintf(`{"spec":{"template":{"metadata":{"annotations":{"gravity-site.gravitational.io/restartedAt":"%s"}}}}}`,
		time.Now().Format(constants.HumanDateFormatSeconds)))
}
