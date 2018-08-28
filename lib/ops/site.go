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
