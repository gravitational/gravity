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
		SiteStateReconfiguring,
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

// ConvertOpsSite converts ops.Site to storage.Site
func ConvertOpsSite(in Site) storage.Site {
	cluster := storage.Site{
		AccountID: in.AccountID,
		Domain:    in.Domain,
		Created:   in.Created,
		CreatedBy: in.CreatedBy,
		State:     in.State,
		Reason:    in.Reason,
		Provider:  in.Provider,
		Local:     in.Local,
		App: storage.Package{
			Repository:    in.App.PackageEnvelope.Locator.Repository,
			Name:          in.App.PackageEnvelope.Locator.Name,
			Version:       in.App.PackageEnvelope.Locator.Version,
			SHA512:        in.App.PackageEnvelope.SHA512,
			SizeBytes:     int(in.App.PackageEnvelope.SizeBytes),
			Created:       in.App.PackageEnvelope.Created,
			CreatedBy:     in.App.PackageEnvelope.CreatedBy,
			RuntimeLabels: in.App.PackageEnvelope.RuntimeLabels,
			Type:          in.App.PackageEnvelope.Type,
			Hidden:        in.App.PackageEnvelope.Hidden,
			Encrypted:     in.App.PackageEnvelope.Encrypted,
			Manifest:      in.App.PackageEnvelope.Manifest,
		},
		Resources:       in.Resources,
		Labels:          in.Labels,
		Location:        in.Location,
		Flavor:          in.Flavor,
		UpdateInterval:  in.UpdateInterval,
		NextUpdateCheck: in.NextUpdateCheck,
		ClusterState:    in.ClusterState,
		ServiceUser:     in.ServiceUser,
		CloudConfig:     in.CloudConfig,
		DNSOverrides:    in.DNSOverrides,
		DNSConfig:       in.DNSConfig,
	}
	if in.License != nil {
		cluster.License = in.License.Raw
	}
	if in.DNSConfig.IsEmpty() {
		cluster.DNSConfig = storage.DefaultDNSConfig
	}
	return cluster
}
