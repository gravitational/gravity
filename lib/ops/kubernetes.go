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

package ops

import (
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MakeClusterInfoMap creates a config map with information about the provided
// cluster that will be made available to all hooks.
func MakeClusterInfoMap(cluster storage.Site) *v1.ConfigMap {
	provider := cluster.Provider
	// The on-prem provider is exposed to the users as 'generic'.
	if provider == schema.ProviderOnPrem {
		provider = schema.ProviderGeneric
	}
	return &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.ClusterInfoMap,
			Namespace: constants.KubeSystemNamespace,
		},
		Data: map[string]string{
			constants.ClusterNameEnv:     cluster.Domain,
			constants.ClusterProviderEnv: provider,
			constants.ClusterFlavorEnv:   cluster.Flavor,
		},
	}
}
