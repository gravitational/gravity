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

package gravity

import (
	"fmt"
	"io"
	"strings"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"
	"github.com/gravitational/gravity/tool/common"

	"github.com/buger/goterm"
	teleservices "github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
)

type roleCollection struct {
	roles []teleservices.Role
}

// Resources returns the resources collection in the generic format
func (c *roleCollection) Resources() (resources []teleservices.UnknownResource, err error) {
	for _, item := range c.roles {
		resource, err := utils.ToUnknownResource(item)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		resources = append(resources, *resource)
	}
	return resources, nil
}

// WriteText serializes collection in human-friendly text format
func (c *roleCollection) WriteText(w io.Writer) error {
	t := goterm.NewTable(0, 10, 5, ' ', 0)
	common.PrintTableHeader(t, []string{"Name"})
	for _, role := range c.filterRoles() {
		fmt.Fprintf(t, "%v\n", role.GetName())
	}
	_, err := io.WriteString(w, t.String())
	return trace.Wrap(err)
}

// WriteJSON serializes collection into JSON format
func (c *roleCollection) WriteJSON(w io.Writer) error {
	return utils.WriteJSON(c, w)
}

func (c *roleCollection) filterRoles() []teleservices.Role {
	var roles []teleservices.Role
	for i := range c.roles {
		role := c.roles[i]
		if !shouldShowRole(role) {
			continue
		}
		roles = append(roles, role)
	}
	return roles
}

func (c *roleCollection) ToMarshal() interface{} {
	roles := c.filterRoles()
	// if originally provided just one role and have filtered roles
	// return just one role, not a list
	if len(roles) == 1 {
		return roles[0]
	}
	return roles
}

// WriteYAML serializes collection into YAML format
func (c *roleCollection) WriteYAML(w io.Writer) error {
	return utils.WriteYAML(c, w)
}

// shouldShowRole returns true if the provided role should be displayed to user
func shouldShowRole(role teleservices.Role) bool {
	if role.GetName() == constants.RoleAdmin {
		return true
	}
	if strings.HasPrefix(role.GetName(), "ca:") {
		return false
	}
	return true
}

type trustedClusterCollection struct {
	clusters []storage.TrustedCluster
}

// Resources returns the resources collection in the generic format
func (c *trustedClusterCollection) Resources() (resources []teleservices.UnknownResource, err error) {
	for _, item := range c.clusters {
		resource, err := utils.ToUnknownResource(item)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		resources = append(resources, *resource)
	}
	return resources, nil
}

// WriteText serializes collection in human-friendly text format
func (c *trustedClusterCollection) WriteText(w io.Writer) error {
	t := goterm.NewTable(0, 10, 5, ' ', 0)
	common.PrintTableHeader(t, []string{
		"Name", "Enabled", "Pull Updates", "Reverse Tunnel Address", "Proxy Address"})
	for _, cluster := range c.filterClusters() {
		fmt.Fprintf(t, "%v\t%v\t%v\t%v\t%v\n",
			cluster.GetName(),
			cluster.GetEnabled(),
			cluster.GetPullUpdates(),
			cluster.GetReverseTunnelAddress(),
			cluster.GetProxyAddress())
	}
	_, err := io.WriteString(w, t.String())
	return trace.Wrap(err)
}

// WriteJSON serializes collection into JSON format
func (c *trustedClusterCollection) WriteJSON(w io.Writer) error {
	return utils.WriteJSON(c, w)
}

// WriteYAML serializes collection into YAML format
func (c *trustedClusterCollection) WriteYAML(w io.Writer) error {
	return utils.WriteYAML(c, w)
}

func (c *trustedClusterCollection) filterClusters() []storage.TrustedCluster {
	var clusters []storage.TrustedCluster
	for _, c := range c.clusters {
		if !c.GetWizard() {
			clusters = append(clusters, c)
		}
	}
	return clusters
}

func (c *trustedClusterCollection) ToMarshal() interface{} {
	clusters := c.filterClusters()
	if len(clusters) == 1 {
		return clusters[0]
	}
	return clusters
}

type endpointsCollection struct {
	endpoints storage.Endpoints
}

// Resources returns the resources collection in the generic format
func (c *endpointsCollection) Resources() (resources []teleservices.UnknownResource, err error) {
	resource, err := utils.ToUnknownResource(c.endpoints)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return []teleservices.UnknownResource{*resource}, nil
}

// WriteText serializes collection in human-friendly text format
func (c *endpointsCollection) WriteText(w io.Writer) error {
	t := goterm.NewTable(0, 10, 5, ' ', 0)
	common.PrintTableHeader(t, []string{
		"Public Advertise Address",
		"Agents Advertise Address",
	})
	fmt.Fprintf(t, "%v\t%v\n",
		c.endpoints.GetPublicAddr(),
		c.endpoints.GetAgentsAddr(),
	)
	_, err := io.WriteString(w, t.String())
	return trace.Wrap(err)
}

// WriteJSON serializes collection into JSON format
func (c *endpointsCollection) WriteJSON(w io.Writer) error {
	return utils.WriteJSON(c, w)
}

// WriteYAML serializes collection into YAML format
func (c *endpointsCollection) WriteYAML(w io.Writer) error {
	return utils.WriteYAML(c, w)
}

func (c *endpointsCollection) ToMarshal() interface{} {
	return c.endpoints
}

type oidcCollection struct {
	connectors []teleservices.OIDCConnector
}

// Resources returns the resources collection in the generic format
func (c *oidcCollection) Resources() (resources []teleservices.UnknownResource, err error) {
	for _, item := range c.connectors {
		resource, err := utils.ToUnknownResource(item)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		resources = append(resources, *resource)
	}
	return resources, nil
}

// WriteText serializes collection in human-friendly text format
func (c *oidcCollection) WriteText(w io.Writer) error {
	t := goterm.NewTable(0, 10, 5, ' ', 0)
	common.PrintTableHeader(t, []string{"Name", "Issuer URL", "Additional Scope"})
	for _, conn := range c.connectors {
		fmt.Fprintf(t, "%v\t%v\t%v\n",
			conn.GetName(),
			conn.GetIssuerURL(),
			strings.Join(conn.GetScope(), ","))
	}
	_, err := io.WriteString(w, t.String())
	return trace.Wrap(err)
}

// WriteJSON serializes collection into JSON format
func (c *oidcCollection) WriteJSON(w io.Writer) error {
	return utils.WriteJSON(c, w)
}

func (c *oidcCollection) ToMarshal() interface{} {
	if len(c.connectors) == 1 {
		return c.connectors[0]
	}
	return c.connectors
}

// WriteYAML serializes collection into YAML format
func (c *oidcCollection) WriteYAML(w io.Writer) error {
	return utils.WriteYAML(c, w)
}

type samlCollection struct {
	connectors []teleservices.SAMLConnector
}

// Resources returns the resources collection in the generic format
func (c *samlCollection) Resources() (resources []teleservices.UnknownResource, err error) {
	for _, item := range c.connectors {
		resource, err := utils.ToUnknownResource(item)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		resources = append(resources, *resource)
	}
	return resources, nil
}

// WriteText serializes collection in human-friendly text format
func (c *samlCollection) WriteText(w io.Writer) error {
	t := goterm.NewTable(0, 10, 5, ' ', 0)
	common.PrintTableHeader(t, []string{"Name", "Issuer", "Attribute Mapping"})
	for _, conn := range c.connectors {
		fmt.Fprintf(t, "%v\t%v\t%v\n",
			conn.GetName(),
			conn.GetIssuer(),
			formatAttributesToRoles(conn.GetAttributesToRoles()))
	}
	_, err := io.WriteString(w, t.String())
	return trace.Wrap(err)
}

func formatAttributesToRoles(mappings []teleservices.AttributeMapping) string {
	var formatted []string
	for _, m := range mappings {
		formatted = append(formatted, fmt.Sprintf("%v/%v -> %v", m.Name,
			m.Value, strings.Join(m.Roles, ",")))
	}
	return strings.Join(formatted, "; ")
}

// WriteJSON serializes collection into JSON format
func (c *samlCollection) WriteJSON(w io.Writer) error {
	return utils.WriteJSON(c, w)
}

func (c *samlCollection) ToMarshal() interface{} {
	if len(c.connectors) == 1 {
		return c.connectors[0]
	}
	return c.connectors
}

// WriteYAML serializes collection into YAML format
func (c *samlCollection) WriteYAML(w io.Writer) error {
	return utils.WriteYAML(c, w)
}

type authConnectorCollection struct {
	connectors []teleservices.Resource
}

// Resources returns the resources collection in the generic format
func (c *authConnectorCollection) Resources() (resources []teleservices.UnknownResource, err error) {
	for _, item := range c.connectors {
		resource, err := utils.ToUnknownResource(item)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		resources = append(resources, *resource)
	}
	return resources, nil
}

// WriteText serializes collection in human-friendly text format
func (c *authConnectorCollection) WriteText(w io.Writer) error {
	connectors, err := c.Resources()
	if err != nil {
		return trace.Wrap(err)
	}
	t := goterm.NewTable(0, 10, 5, ' ', 0)
	common.PrintTableHeader(t, []string{"Kind", "Name"})
	for _, conn := range connectors {
		fmt.Fprintf(t, "%v\t%v\n", conn.Kind, conn.Metadata.Name)
	}
	_, err = io.WriteString(w, t.String())
	return trace.Wrap(err)
}

// WriteJSON serializes collection into JSON format
func (c *authConnectorCollection) WriteJSON(w io.Writer) error {
	return utils.WriteJSON(c, w)
}

func (c *authConnectorCollection) ToMarshal() interface{} {
	if len(c.connectors) == 1 {
		return c.connectors[0]
	}
	return c.connectors
}

// WriteYAML serializes collection into YAML format
func (c *authConnectorCollection) WriteYAML(w io.Writer) error {
	return utils.WriteYAML(c, w)
}
