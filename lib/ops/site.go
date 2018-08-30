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

package ops

import (
	"fmt"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	teleservices "github.com/gravitational/teleport/lib/services"
)

// IsInstalledState takes a site state and returns true/false depending on whether
// this state represents one of "installed" states
func IsInstalledState(siteState string) bool {
	notInstalledStates := []string{
		SiteStateNotInstalled,
		SiteStateFailed,
		SiteStateInstalling,
		SiteStateUninstalling,
	}
	return !utils.StringInSlice(notInstalledStates, siteState)
}

// NewClusterFromSite creates cluster resource from Site object
func NewClusterFromSite(site Site) *storage.ClusterV2 {
	spec := site.ClusterState.ClusterNodeSpec()
	cluster := &storage.ClusterV2{
		Kind:    storage.KindCluster,
		Version: teleservices.V2,
		Metadata: teleservices.Metadata{
			Name:      site.Domain,
			Namespace: defaults.Namespace,
			Labels:    site.Labels,
		},
		Spec: storage.ClusterSpecV2{
			App:      fmt.Sprintf("%v:%v", site.App.Package.Name, site.App.Package.Version),
			Provider: site.Provider,
			Nodes:    spec,
			Status:   site.State,
		},
	}
	return cluster
}
