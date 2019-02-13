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
	"encoding/json"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/kubernetes"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/storage/clusterconfig"

	"github.com/gravitational/rigging"
	"github.com/gravitational/trace"
	"github.com/pborman/uuid"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

// CreateUpdateConfigOperation creates a new operation to update cluster configuration
func (o *Operator) CreateUpdateConfigOperation(r ops.CreateUpdateConfigOperationRequest) (*ops.SiteOperationKey, error) {
	err := r.ClusterKey.Check()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cluster, err := o.openSite(r.ClusterKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	key, err := cluster.createUpdateConfigOperation(r)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return key, nil
}

// GetClusterConfiguration retrieves the cluster configuration
func (o *Operator) GetClusterConfiguration(key ops.SiteKey) (config clusterconfig.Interface, err error) {
	client, err := o.GetKubeClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	configmap, err := client.CoreV1().ConfigMaps(defaults.KubeSystemNamespace).
		Get(constants.ClusterConfigurationMap, metav1.GetOptions{})
	err = rigging.ConvertError(err)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	spec := configmap.Data["spec"]
	if len(spec) != 0 {
		config, err = clusterconfig.Unmarshal([]byte(spec))
		if err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		config = clusterconfig.Empty()
	}
	return config, nil
}

// NewConfigurationConfigMap creates the backing ConfigMap to host cluster configuration
func NewConfigurationConfigMap(config []byte) *v1.ConfigMap {
	return &v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       constants.KindConfigMap,
			APIVersion: metav1.SchemeGroupVersion.Version,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.ClusterConfigurationMap,
			Namespace: defaults.KubeSystemNamespace,
		},
		Data: map[string]string{
			"spec": string(config),
		},
	}
}

// createUpdateConfigOperation creates a new operation to update cluster configuration
func (s *site) createUpdateConfigOperation(req ops.CreateUpdateConfigOperationRequest) (*ops.SiteOperationKey, error) {
	client, err := s.service.GetKubeClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	configmaps := client.CoreV1().ConfigMaps(defaults.KubeSystemNamespace)
	configmap, err := getOrCreateClusterConfigMap(configmaps)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var previousKeyValues []byte
	if len(configmap.Data) != 0 {
		var err error
		previousKeyValues, err = json.Marshal(configmap.Data)
		if err != nil {
			return nil, trace.Wrap(err, "failed to marshal previous key/values")
		}
		if configmap.Annotations == nil {
			configmap.Annotations = make(map[string]string)
		}
		configmap.Annotations[constants.PreviousKeyValuesAnnotationKey] = string(previousKeyValues)
	}
	op := ops.SiteOperation{
		ID:           uuid.New(),
		AccountID:    s.key.AccountID,
		SiteDomain:   s.key.SiteDomain,
		Type:         ops.OperationUpdateConfig,
		Created:      s.clock().UtcNow(),
		Updated:      s.clock().UtcNow(),
		State:        ops.OperationUpdateConfigInProgress,
		UpdateConfig: &storage.UpdateConfigOperationState{Config: req.Config},
	}
	key, err := s.getOperationGroup().createSiteOperation(op)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	configmap.Data = map[string]string{
		"spec": string(req.Config),
	}
	err = kubernetes.Retry(context.TODO(), func() error {
		_, err := configmaps.Update(configmap)
		return trace.Wrap(err)
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return key, nil
}

func getOrCreateClusterConfigMap(client corev1.ConfigMapInterface) (configmap *v1.ConfigMap, err error) {
	configmap, err = client.Get(constants.ClusterConfigurationMap, metav1.GetOptions{})
	if err != nil {
		if !trace.IsNotFound(rigging.ConvertError(err)) {
			return nil, trace.Wrap(err)
		}
		err = rigging.ConvertError(err)
	}
	if err == nil {
		return configmap, nil
	}
	configmap = NewConfigurationConfigMap(nil)
	configmap, err = client.Create(configmap)
	if err != nil {
		return nil, trace.Wrap(rigging.ConvertError(err))
	}
	return configmap, nil
}
