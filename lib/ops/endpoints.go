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
	"fmt"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
)

// ClusterEndpoints contains system cluster endpoints such as Teleport
// proxy address or cluster control panel URL.
type ClusterEndpoints struct {
	// Internal contains internal cluster endpoints.
	Internal clusterEndpoints
	// Public contains public cluster endpoints.
	Public clusterEndpoints
}

// AuthGateways returns all auth gateway endpoints.
func (e ClusterEndpoints) AuthGateways() []string {
	if len(e.Public.AuthGateways) > 0 {
		return e.Public.AuthGateways
	}
	return e.Internal.AuthGateways
}

// AuthGateway returns a single auth gateway endpoint.
func (e ClusterEndpoints) AuthGateway() string {
	gateways := e.AuthGateways()
	if len(gateways) > 0 {
		return gateways[0]
	}
	return ""
}

// ManagementURLs returns all cluster management URLs.
func (e ClusterEndpoints) ManagementURLs() []string {
	if len(e.Public.ManagementURLs) > 0 {
		return e.Public.ManagementURLs
	}
	return e.Internal.ManagementURLs
}

// clusterEndpoints combines various types of cluster endpoints.
type clusterEndpoints struct {
	// AuthGateways is a list of Teleport proxy addresses.
	AuthGateways []string
	// ManagementURLs is a list of URLs pointing to cluster dashboard.
	ManagementURLs []string
}

// GetClusterEndpoints returns system endpoints for the specified cluster.
func GetClusterEndpoints(operator Operator, key SiteKey) (*ClusterEndpoints, error) {
	cluster, err := operator.GetSite(key)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	authGateway, err := operator.GetAuthGateway(key)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Internal endpoints point directly to master nodes.
	var internal clusterEndpoints
	for _, master := range cluster.Masters() {
		internal.AuthGateways = append(internal.AuthGateways,
			fmt.Sprintf("%v:%v", master.AdvertiseIP, defaults.GravitySiteNodePort))
		internal.ManagementURLs = append(internal.ManagementURLs,
			fmt.Sprintf("https://%v:%v", master.AdvertiseIP, defaults.GravitySiteNodePort))
	}
	// Public endpoints are configured via auth gateway resource.
	var public clusterEndpoints
	for _, address := range authGateway.GetWebPublicAddrs() {
		public.AuthGateways = append(public.AuthGateways,
			utils.EnsurePort(address, defaults.HTTPSPort))
		public.ManagementURLs = append(public.ManagementURLs,
			fmt.Sprintf("https://%v", utils.EnsurePort(address, defaults.HTTPSPort)))
	}
	return &ClusterEndpoints{
		Internal: internal,
		Public:   public,
	}, nil
}
