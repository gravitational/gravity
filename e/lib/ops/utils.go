// Copyright 2021 Gravitational Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ops

import (
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/trace"
)

// GetTrustedCluster returns a trusted cluster representing the Ops Center
// the specified site is connected to, currently only 1 is supported
func GetTrustedCluster(key ops.SiteKey, operator Operator) (storage.TrustedCluster, error) {
	clusters, err := operator.GetTrustedClusters(key)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, cluster := range clusters {
		if !cluster.GetWizard() {
			return cluster, nil
		}
	}
	return nil, trace.NotFound("trusted cluster for %v not found", key)
}

// GetWizardTrustedCluster returns a trusted cluster representing the wizard
// Ops Center the specified site is connected to
func GetWizardTrustedCluster(key ops.SiteKey, operator Operator) (storage.TrustedCluster, error) {
	clusters, err := operator.GetTrustedClusters(key)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, cluster := range clusters {
		if cluster.GetWizard() {
			return cluster, nil
		}
	}
	return nil, trace.NotFound("wizard trusted cluster for %v not found", key)
}
