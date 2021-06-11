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

package storage

import (
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/utils"

	teleservices "github.com/gravitational/teleport/lib/services"
	teleutils "github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

// TrustedCluster extends Teleport's trusted cluster interface with Gravity
// specific methods
type TrustedCluster interface {
	// TrustedCluster is the base trusted cluster interface from Teleport
	teleservices.TrustedCluster
	// GetSNIHost returns the Ops Center SNI host
	GetSNIHost() string
	// SetSNIHost sets the Ops Center SNI host
	SetSNIHost(string)
	// GetPullUpdates returns true if the cluster pulls updates from Ops Center
	GetPullUpdates() bool
	// SetPullUpdates enables or disables pulling updates from Ops Center
	SetPullUpdates(bool)
	// GetWizard returns true for trusted cluster representing wizard Ops Center
	GetWizard() bool
	// SetWizard marks the trusted cluster as wizard mode or not
	SetWizard(bool)
	// GetSystem returns true if this is a system trusted cluster
	GetSystem() bool
	// SetSystem marks the trusted cluster as a system
	SetSystem(bool)
	// GetRegular returns true if this is a regular Ops Center.
	GetRegular() bool
}

// NewTrustedCluster returns a new trusted cluster from the provided name and spec
func NewTrustedCluster(name string, spec TrustedClusterSpecV2) TrustedCluster {
	return &TrustedClusterV2{
		Kind:    teleservices.KindTrustedCluster,
		Version: teleservices.V2,
		Metadata: teleservices.Metadata{
			Name:      name,
			Namespace: defaults.Namespace,
			Labels:    map[string]string{},
		},
		Spec: spec,
	}
}

// NewTrustedClusterFromLinks creates a trusted cluster from the legacy remote
// support and update links
func NewTrustedClusterFromLinks(remoteLink OpsCenterLink, updateLink *OpsCenterLink) (TrustedCluster, error) {
	opsCenterURL, err := url.ParseRequestURI(remoteLink.APIURL)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	spec := TrustedClusterSpecV2{
		Enabled:              remoteLink.Enabled,
		ProxyAddress:         opsCenterURL.Host,
		ReverseTunnelAddress: remoteLink.RemoteAddr,
		SNIHost:              remoteLink.Hostname,
		Roles:                []string{constants.RoleAdmin},
		Wizard:               remoteLink.Wizard,
	}
	if remoteLink.User != nil {
		spec.Token = remoteLink.User.Token
	}
	if updateLink != nil {
		spec.PullUpdates = updateLink.Enabled
	}
	return NewTrustedCluster(remoteLink.Hostname, spec), nil
}

// TrustedClusterV2 represents a trusted cluster resource
type TrustedClusterV2 struct {
	// Kind is the resource kind, trusted_cluster
	Kind string `json:"kind"`
	// Version is the resource version
	Version string `json:"version"`
	// Metadata is the resource metadata
	Metadata teleservices.Metadata `json:"metadata"`
	// Spec is the trusted cluster spec
	Spec TrustedClusterSpecV2 `json:"spec"`
}

// TrustedClusterSpecV2 represents the trusted cluster spec
type TrustedClusterSpecV2 struct {
	// Enabled indicates whether the trusted cluster is enabled
	Enabled bool `json:"enabled"`
	// Token is a shared authorization token used to connect a remote cluster
	Token string `json:"token"`
	// ProxyAddress is the address of the web proxy server of the cluster to join.
	// If not set, defaults to <metadata.name>:<default web proxy server port>
	ProxyAddress string `json:"web_proxy_addr"`
	// ReverseTunnelAddress is the address of the SSH proxy server of the cluster
	// to join. If not set, defaults to <metadata.name>:<default reverse tunnel port>
	ReverseTunnelAddress string `json:"tunnel_addr"`
	// SNIHost is the Ops Center's public endpoint hostname
	SNIHost string `json:"sni_host"`
	// Roles is a list of roles that users will be assuming when connecting to
	// this cluster
	Roles []string `json:"roles,omitempty"`
	// RoleMap specifies role mappings to remote roles
	RoleMap teleservices.RoleMap `json:"role_map,omitempty"`
	// PullUpdates indicates whether the trusted cluster should pull updates
	PullUpdates bool `json:"pull_updates"`
	// Wizard is true for trusted cluster representing a standalone installer
	// Ops Center
	Wizard bool `json:"wizard,omitempty"`
}

// GetName returns the trusted cluster name
func (c *TrustedClusterV2) GetName() string {
	return c.Metadata.Name
}

// SetName sets the trusted cluster name
func (c *TrustedClusterV2) SetName(name string) {
	c.Metadata.Name = name
}

// Expiry returns the trusted cluster expiration time
func (c *TrustedClusterV2) Expiry() time.Time {
	return c.Metadata.Expiry()
}

// SetExpiry sets the trusted cluster expiration time
func (c *TrustedClusterV2) SetExpiry(expires time.Time) {
	c.Metadata.SetExpiry(expires)
}

// SetTTL sets the trusted cluster TTL
func (c *TrustedClusterV2) SetTTL(clock clockwork.Clock, ttl time.Duration) {
	c.Metadata.SetTTL(clock, ttl)
}

// GetMetadata returns the trusted cluster metadata
func (c *TrustedClusterV2) GetMetadata() teleservices.Metadata {
	return c.Metadata
}

// GetRoleMap returns the cluster role map
func (c *TrustedClusterV2) GetRoleMap() teleservices.RoleMap {
	return c.Spec.RoleMap
}

// SetRoleMap sets the cluster role map
func (c *TrustedClusterV2) SetRoleMap(m teleservices.RoleMap) {
	c.Spec.RoleMap = m
}

// GetRoles returns the cluster roles
func (c *TrustedClusterV2) GetRoles() []string {
	return c.Spec.Roles
}

// SetRoles sets the cluster roles
func (c *TrustedClusterV2) SetRoles(roles []string) {
	c.Spec.Roles = roles
}

// CombinedMapping returns role map combined with roles
func (c *TrustedClusterV2) CombinedMapping() teleservices.RoleMap {
	if len(c.Spec.Roles) != 0 {
		return []teleservices.RoleMapping{{
			Remote: teleservices.Wildcard,
			Local:  c.Spec.Roles,
		}}
	}
	return c.Spec.RoleMap
}

// GetEnabled returns true if the cluster is connected to Ops Center
func (c *TrustedClusterV2) GetEnabled() bool {
	return c.Spec.Enabled
}

// SetEnabled enables or disables Ops Center connection
func (c *TrustedClusterV2) SetEnabled(enabled bool) {
	c.Spec.Enabled = enabled
}

// GetToken returns the authorization and authentication token
func (c *TrustedClusterV2) GetToken() string {
	return c.Spec.Token
}

// SetToken sets the authorization and authentication token
func (c *TrustedClusterV2) SetToken(token string) {
	c.Spec.Token = token
}

// GetProxyAddress returns the address of the proxy server
func (c *TrustedClusterV2) GetProxyAddress() string {
	return c.Spec.ProxyAddress
}

// SetProxyAddress sets the address of the proxy server
func (c *TrustedClusterV2) SetProxyAddress(addr string) {
	c.Spec.ProxyAddress = addr
}

// GetReverseTunnelAddress returns the address of the reverse tunnel
func (c *TrustedClusterV2) GetReverseTunnelAddress() string {
	return c.Spec.ReverseTunnelAddress
}

// SetReverseTunnelAddress sets the address of the reverse tunnel
func (c *TrustedClusterV2) SetReverseTunnelAddress(addr string) {
	c.Spec.ReverseTunnelAddress = addr
}

// GetSNIHost returns the Ops Center SNI host
func (c *TrustedClusterV2) GetSNIHost() string {
	if c.Spec.SNIHost != "" {
		return c.Spec.SNIHost
	}
	host, _ := utils.SplitHostPort(c.Spec.ProxyAddress, "")
	return host
}

// SetSNIHost sets the Ops Center SNI host
func (c *TrustedClusterV2) SetSNIHost(host string) {
	c.Spec.SNIHost = host
}

// CanChangeStateTo checks if the state change is allowed or not. If not,
// returns an error explaining the reason
func (c *TrustedClusterV2) CanChangeStateTo(t teleservices.TrustedCluster) error {
	if c.GetToken() != t.GetToken() {
		return trace.BadParameter(
			"can not update token for existing trusted cluster")
	}
	if c.GetProxyAddress() != t.GetProxyAddress() {
		return trace.BadParameter(
			"can not update proxy address for existing trusted cluster")
	}
	if c.GetReverseTunnelAddress() != t.GetReverseTunnelAddress() {
		return trace.BadParameter(
			"can not update tunnel address for existing trusted cluster")
	}
	if !c.GetRoleMap().Equals(t.GetRoleMap()) {
		return trace.BadParameter(
			"can not update role map for existing trusted cluster")
	}
	return nil
}

// GetPullUpdates returns true if the cluster pulls updates from Ops Center
func (c *TrustedClusterV2) GetPullUpdates() bool {
	return c.Spec.PullUpdates
}

// SetPullUpdates enables or disables pulling updates from Ops Center
func (c *TrustedClusterV2) SetPullUpdates(enabled bool) {
	c.Spec.PullUpdates = enabled
}

// GetWizard returns true for trusted cluster representing wizard Ops Center
func (c *TrustedClusterV2) GetWizard() bool {
	return c.Spec.Wizard
}

// SetWizard marks the trusted cluster as wizard mode or not
func (c *TrustedClusterV2) SetWizard(wizard bool) {
	c.Spec.Wizard = wizard
}

// GetSystem returns true if this is a system trusted cluster
func (c *TrustedClusterV2) GetSystem() bool {
	return c.Metadata.Labels[constants.SystemLabel] == "true"
}

// SetSystem marks the trusted clusters as a system
func (c *TrustedClusterV2) SetSystem(system bool) {
	if system {
		c.Metadata.Labels[constants.SystemLabel] = "true"
	} else {
		delete(c.Metadata.Labels, constants.SystemLabel)
	}
}

// GetRegular returns true if this is a regular Ops Center.
func (c *TrustedClusterV2) GetRegular() bool {
	return !c.GetWizard() && !c.GetSystem()
}

// CheckAndSetDefaults checks the cluster resource and sets some defaults
func (c *TrustedClusterV2) CheckAndSetDefaults() error {
	if err := c.Metadata.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if c.Spec.Token == "" {
		return trace.BadParameter("token can't be empty")
	}
	if c.Spec.ProxyAddress == "" {
		return trace.BadParameter("web_proxy_addr can't be empty")
	}
	if c.Spec.ReverseTunnelAddress == "" {
		return trace.BadParameter("tunnel_addr can't be empty")
	}
	if len(c.Spec.Roles) != 0 && len(c.Spec.RoleMap) != 0 {
		return trace.BadParameter("either roles or role_map should be set, not both")
	}
	if err := c.Spec.RoleMap.Check(); err != nil {
		return trace.Wrap(err)
	}
	if c.Metadata.Labels == nil {
		c.Metadata.Labels = map[string]string{}
	}
	// Fields "roles" and "role_map" are mutually exclusive so only populate
	// default mapping if neither of those is set, otherwise it will lead to
	// incorrect trusted cluster configuration.
	if len(c.Spec.Roles)+len(c.Spec.RoleMap) == 0 {
		c.Spec.RoleMap = teleservices.RoleMap{
			{
				Remote: constants.RoleAdmin,
				Local:  []string{constants.RoleAdmin},
			},
		}
	}
	return nil
}

// String returns a string representation of a trusted cluster
func (c TrustedClusterV2) String() string {
	return fmt.Sprintf("TrustedClusterV2(Name=%v, Enabled=%v, ProxyAddress=%v, ReverseTunnelAddress=%v, SNIHost=%v, Roles=%v, PullUpdates=%v, Wizard=%v, System=%v)",
		c.GetName(), c.GetEnabled(), c.GetProxyAddress(), c.GetReverseTunnelAddress(), c.GetSNIHost(), c.GetRoles(), c.GetPullUpdates(), c.GetWizard(), c.GetSystem())
}

func init() {
	teleservices.SetTrustedClusterMarshaler(&trustedClusterMarshaler{})
}

type trustedClusterMarshaler struct{}

// Marshal marshals the trusted cluster resource to JSON
func (*trustedClusterMarshaler) Marshal(cluster teleservices.TrustedCluster, opts ...teleservices.MarshalOption) ([]byte, error) {
	bytes, err := MarshalTrustedCluster(cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return bytes, nil
}

// Unmarshal unmarshals the trusted cluster resource from JSON bytes
func (*trustedClusterMarshaler) Unmarshal(bytes []byte) (teleservices.TrustedCluster, error) {
	cluster, err := UnmarshalTrustedCluster(bytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return cluster, nil
}

// MarshalTrustedCluster marshals the provided trusted cluster into JSON
func MarshalTrustedCluster(cluster teleservices.TrustedCluster) ([]byte, error) {
	bytes, err := json.Marshal(cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return bytes, nil
}

// UnmarshalTrustedCluster unmarshals the trusted cluster resource from bytes
func UnmarshalTrustedCluster(bytes []byte) (TrustedCluster, error) {
	jsonBytes, err := teleutils.ToJSON(bytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var header teleservices.ResourceHeader
	if err := json.Unmarshal(jsonBytes, &header); err != nil {
		return nil, trace.Wrap(err)
	}
	if header.Kind != teleservices.KindTrustedCluster {
		return nil, trace.BadParameter("invalid kind %q, expected %q",
			header.Kind, teleservices.KindTrustedCluster)
	}
	switch header.Version {
	case teleservices.V2:
		var cluster TrustedClusterV2
		err := teleutils.UnmarshalWithSchema(
			teleservices.GetTrustedClusterSchema(TrustedClusterSpecV2Extension),
			&cluster, bytes)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if err := cluster.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}
		return &cluster, nil
	}
	return nil, trace.BadParameter(
		"trusted cluster resource version %q is not supported", header.Version)
}

const TrustedClusterSpecV2Extension = `
  "sni_host": {"type": "string"},
  "pull_updates": {"type": "boolean"},
  "wizard": {"type": "boolean"}
`
