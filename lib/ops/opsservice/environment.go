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
	"encoding/json"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/kubernetes"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/rigging"
	"github.com/gravitational/trace"
	"github.com/pborman/uuid"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

// CreateUpdateEnvarsOperation creates a new operation to update cluster environment variables
func (o *Operator) CreateUpdateEnvarsOperation(ctx context.Context, r ops.CreateUpdateEnvarsOperationRequest) (*ops.SiteOperationKey, error) {
	err := r.ClusterKey.Check()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cluster, err := o.openSite(r.ClusterKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	env, err := o.getClusterEnvironment()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	key, err := cluster.createUpdateEnvarsOperation(ctx, r, env)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return key, nil
}

// GetClusterEnvironmentVariables retrieves the cluster environment variables
func (o *Operator) GetClusterEnvironmentVariables(key ops.SiteKey) (env storage.EnvironmentVariables, err error) {
	client, err := o.GetKubeClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	configmap, err := client.CoreV1().ConfigMaps(defaults.KubeSystemNamespace).
		Get(context.TODO(), constants.ClusterEnvironmentMap, metav1.GetOptions{})
	err = rigging.ConvertError(err)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	var data map[string]string
	if configmap != nil {
		data = configmap.Data
	}
	env = storage.NewEnvironment(data)
	return env, nil
}

func (o *Operator) getClusterEnvironment() (env map[string]string, err error) {
	client, err := o.GetKubeClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	configmap, err := client.CoreV1().ConfigMaps(defaults.KubeSystemNamespace).
		Get(context.TODO(), constants.ClusterEnvironmentMap, metav1.GetOptions{})
	err = rigging.ConvertError(err)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	return configmap.Data, nil
}

// UpdateClusterEnvironmentVariables updates the cluster runtime environment variables
// from the specified request
func (o *Operator) UpdateClusterEnvironmentVariables(req ops.UpdateClusterEnvironRequest) error {
	client, err := o.GetKubeClient()
	if err != nil {
		return trace.Wrap(err)
	}
	configmaps := client.CoreV1().ConfigMaps(defaults.KubeSystemNamespace)
	configmap, err := getOrCreateEnvironmentConfigMap(configmaps)
	if err != nil {
		return trace.Wrap(err)
	}
	var previousKeyValues []byte
	if len(configmap.Data) != 0 {
		var err error
		previousKeyValues, err = json.Marshal(configmap.Data)
		if err != nil {
			return trace.Wrap(err, "failed to marshal previous key/values")
		}
		if configmap.Annotations == nil {
			configmap.Annotations = make(map[string]string)
		}
		configmap.Annotations[constants.PreviousKeyValuesAnnotationKey] = string(previousKeyValues)
	}
	configmap.Data = req.Env
	err = kubernetes.Retry(context.TODO(), func() error {
		_, err := configmaps.Update(context.TODO(), configmap, metav1.UpdateOptions{})
		return trace.Wrap(err)
	})
	return trace.Wrap(err)
}

// NewEnvironmentConfigMap creates the backing ConfigMap to host cluster runtime environment variables
func NewEnvironmentConfigMap(data map[string]string) *v1.ConfigMap {
	return &v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       constants.KindConfigMap,
			APIVersion: metav1.SchemeGroupVersion.Version,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.ClusterEnvironmentMap,
			Namespace: defaults.KubeSystemNamespace,
		},
		Data: data,
	}
}

// createUpdateEnvarsOperation creates a new operation to update cluster environment variables
func (s *site) createUpdateEnvarsOperation(ctx context.Context, req ops.CreateUpdateEnvarsOperationRequest, prevEnv map[string]string) (*ops.SiteOperationKey, error) {
	op := ops.SiteOperation{
		ID:         uuid.New(),
		AccountID:  s.key.AccountID,
		SiteDomain: s.key.SiteDomain,
		Type:       ops.OperationUpdateRuntimeEnviron,
		Created:    s.clock().UtcNow(),
		CreatedBy:  storage.UserFromContext(ctx),
		Updated:    s.clock().UtcNow(),
		State:      ops.OperationUpdateRuntimeEnvironInProgress,
		UpdateEnviron: &storage.UpdateEnvarsOperationState{
			PrevEnv: prevEnv,
			Env:     req.Env,
		},
	}
	key, err := s.getOperationGroup().createSiteOperation(op)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return key, nil
}

func getOrCreateEnvironmentConfigMap(client corev1.ConfigMapInterface) (configmap *v1.ConfigMap, err error) {
	configmap, err = client.Get(context.TODO(), constants.ClusterEnvironmentMap, metav1.GetOptions{})
	if err != nil {
		if !trace.IsNotFound(rigging.ConvertError(err)) {
			return nil, trace.Wrap(err)
		}
		err = rigging.ConvertError(err)
	}
	if err == nil {
		return configmap, nil
	}
	configmap = NewEnvironmentConfigMap(nil)
	configmap, err = client.Create(context.TODO(), configmap, metav1.CreateOptions{})
	if err != nil {
		return nil, trace.Wrap(rigging.ConvertError(err))
	}
	return configmap, nil
}
