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

package webapi

import (
	"bytes"
	"strconv"
	"text/template"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/ops"

	"github.com/gravitational/trace"
)

// webClusterInfo encapsulates basic information about cluster such as
// management endpoints and status used by the control panel.
type webClusterInfo struct {
	// ClusterState is the current cluster state.
	ClusterState string `json:"clusterState"`
	// PublicURLs is the advertised public cluster URLs set via auth gateway resource.
	PublicURLs []string `json:"publicURL"`
	// InternalURLs is a list of internal cluster management URLs.
	InternalURLs []string `json:"internalURLs"`
	// AuthGateways is the cluster's authentication gateway addresses.
	AuthGateways []string `json:"authGateways"`
	// MasterNodes is a list of cluster's master nodes.
	MasterNodes []string `json:"masterNodes"`
	// GravityURL is the URL to download gravity binary from the cluster.
	GravityURL string `json:"gravityURL"`
}

// getClusterInfo collects information for the specified cluster.
func getClusterInfo(operator ops.Operator, cluster ops.Site) (*webClusterInfo, error) {
	endpoints, err := ops.GetClusterEndpoints(operator, cluster.Key())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	masterNode, err := cluster.FirstMaster()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var gravityURL bytes.Buffer
	if err := gravityURLTpl.Execute(&gravityURL, map[string]string{
		"node": masterNode.AdvertiseIP,
		"port": strconv.Itoa(defaults.GravitySiteNodePort),
	}); err != nil {
		return nil, trace.Wrap(err)
	}
	return &webClusterInfo{
		ClusterState: cluster.State,
		PublicURLs:   endpoints.Public.ManagementURLs,
		InternalURLs: endpoints.Internal.ManagementURLs,
		AuthGateways: endpoints.AuthGateways(),
		MasterNodes:  cluster.Masters().MasterIPs(),
		GravityURL:   gravityURL.String(),
	}, nil
}

var (
	// gravityURLTpl is the template of the URL to download gravity binary.
	gravityURLTpl = template.Must(template.New("gravityURL").Parse(
		`https://{{.node}}:{{.port}}/portal/v1/gravity`))
)
