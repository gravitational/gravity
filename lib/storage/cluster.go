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
	"time"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"

	"github.com/gravitational/license"
	teleservices "github.com/gravitational/teleport/lib/services"
	teleutils "github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

// Cluster contains a set of permissions or settings
type Cluster interface {
	// Resource provides common resource methods
	teleservices.Resource
	// CheckAndSetDefaults makes sure the cluster is valid
	CheckAndSetDefaults() error
	// SetApp sets the cluster app
	SetApp(string)
	// GetApp returns the cluster app
	GetApp() string
	// SetResources sets additional Kubernetes resources
	SetResources(string)
	// GetResources returns additional Kubernetes resources
	GetResources() string
	// SetLicense sets the cluster license
	SetLicense(string)
	// GetLicense returns the cluster license
	GetLicense() string
	// GetStatus returns cluster status
	GetStatus() string
	// GetProvider returns cluster provider
	GetProvider() string
	// GetAWSRegion returns region
	GetRegion() string
	// GetNodes returns cluster nodes
	GetNodes() []ClusterNodeSpecV2
}

// NewClusterFromSite returns new cluster from stored site
func NewClusterFromSite(site *Site) Cluster {
	spec := site.ClusterState.ClusterNodeSpec()
	return &ClusterV2{
		Kind:    KindCluster,
		Version: teleservices.V2,
		Metadata: teleservices.Metadata{
			Name:      site.Domain,
			Namespace: defaults.Namespace,
			Labels:    site.Labels,
		},
		Spec: ClusterSpecV2{
			App:       fmt.Sprintf("%v:%v", site.App.Name, site.App.Version),
			Provider:  site.Provider,
			Nodes:     spec,
			Resources: string(site.Resources),
			License:   site.License,
			Status:    site.State,
		},
	}
}

// NewCluster returns instance of the new cluster
func NewCluster(name string) Cluster {
	return &ClusterV2{
		Kind:    KindCluster,
		Version: teleservices.V2,
		Metadata: teleservices.Metadata{
			Name:      name,
			Namespace: defaults.Namespace,
		},
		Spec: ClusterSpecV2{},
	}
}

// ClusterV2 represents cluster resource specification
type ClusterV2 struct {
	// Kind is a resource kind - always resource
	Kind string `json:"kind"`
	// Version is a resource version
	Version string `json:"version"`
	// Metadata is cluster metadata
	Metadata teleservices.Metadata `json:"metadata"`
	// Spec contains cluster specification
	Spec ClusterSpecV2 `json:"spec"`
}

// GetName returns cluster name and is a shortcut for GetMetadata().Name
func (t *ClusterV2) GetName() string {
	return t.Metadata.Name
}

// SetName sets cluster name
func (c *ClusterV2) SetName(name string) {
	c.Metadata.Name = name
}

// GetMetadata returns cluster metadata
func (c *ClusterV2) GetMetadata() teleservices.Metadata {
	return c.Metadata
}

// SetExpiry sets cluster expiration time
func (c *ClusterV2) SetExpiry(expires time.Time) {
	c.Metadata.SetExpiry(expires)
}

// Expires returns cluster expiration time
func (c *ClusterV2) Expiry() time.Time {
	return c.Metadata.Expiry()
}

// SetTTL sets Expires header using realtime clock
func (c *ClusterV2) SetTTL(clock clockwork.Clock, ttl time.Duration) {
	c.Metadata.SetTTL(clock, ttl)
}

// GetApp returns the cluster app
func (c *ClusterV2) GetApp() string {
	return c.Spec.App
}

// SetApp sets the cluster application
func (c *ClusterV2) SetApp(app string) {
	c.Spec.App = app
}

// SetResources sets additional Kubernetes resources
func (c *ClusterV2) SetResources(resources string) {
	c.Spec.Resources = resources
}

// GetResources returns additional Kubernetes resources
func (c *ClusterV2) GetResources() string {
	return c.Spec.Resources
}

// SetLicense sets the cluster license
func (c *ClusterV2) SetLicense(license string) {
	c.Spec.License = license
}

// GetLicense returns the cluster license
func (c *ClusterV2) GetLicense() string {
	return c.Spec.License
}

// GetStatus returns cluster status
func (c *ClusterV2) GetStatus() string {
	return c.Spec.Status
}

// GetProvider returns cluster provider
func (c *ClusterV2) GetProvider() string {
	return c.Spec.Provider
}

// GetAWSRegion returns region
func (c *ClusterV2) GetRegion() string {
	if c.Spec.AWS != nil {
		return c.Spec.AWS.Region
	}
	return ""
}

// GetNodes returns cluster nodes
func (c *ClusterV2) GetNodes() []ClusterNodeSpecV2 {
	return c.Spec.Nodes
}

// Check checks validity of all parameters and sets defaults
func (c *ClusterV2) CheckAndSetDefaults() error {
	if c.Metadata.Name == "" {
		return trace.BadParameter("parameter metadata.name can't be empty")
	}
	if c.Spec.App == "" {
		return trace.BadParameter("parameter spec.app can't be empty")
	}
	if _, err := loc.ParseLocator(defaults.SystemAccountOrg + "/" + c.Spec.App); err != nil {
		return trace.Wrap(err, "failed to parse spec.app parameter, has to be valid app name with version, e.g. gravity:4.14.0")
	}
	if c.Spec.Provider == "" {
		return trace.BadParameter("parameter spec.provider can't be empty")
	}
	if len(c.Spec.Nodes) == 0 {
		return trace.BadParameter("parameter spec.nodes can't be empty")
	}
	nodes := make(map[string]struct{})
	for _, node := range c.Spec.Nodes {
		if _, ok := nodes[node.Profile]; ok {
			return trace.BadParameter("parameter spec.nodes is invalid: profile %q appears more than once", node.Profile)
		}
		nodes[node.Profile] = struct{}{}
		if node.InstanceType == "" {
			return trace.BadParameter("profile %q is invalid: instanceType can't be empty", node.Profile)
		}
		if node.Count == 0 {
			return trace.BadParameter("profile %q is invalid: count can't be 0", node.Profile)
		}
	}
	if c.Spec.License != "" {
		if _, err := license.ParseLicense(c.Spec.License); err != nil {
			return trace.Wrap(err, "failed to parse spec.license parameter")
		}
	}
	return nil
}

// UnmarshalCluster unmarshals cluster from JSON
func UnmarshalCluster(data []byte) (Cluster, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing cluster data")
	}
	jsonData, err := teleutils.ToJSON(data)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var h teleservices.ResourceHeader
	err = json.Unmarshal(jsonData, &h)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch h.Version {
	case teleservices.V2:
		var t ClusterV2
		err := teleutils.UnmarshalWithSchema(GetClusterSchema(), &t, jsonData)
		if err != nil {
			return nil, trace.BadParameter(err.Error())
		}
		//nolint:errcheck
		t.Metadata.CheckAndSetDefaults()
		return &t, nil
	}
	return nil, trace.BadParameter(
		"cluster resource version %q is not supported", h.Version)
}

// MarshalCluster marshals cluster into JSON
func MarshalCluster(cluster Cluster, opts ...teleservices.MarshalOption) ([]byte, error) {
	return json.Marshal(cluster)
}

// ClusterSpecV2 is cluster V2 specification
type ClusterSpecV2 struct {
	// App is an application name
	App string `json:"app"`
	// Provider is a cloud provider name
	Provider string `json:"provider"`
	// AWS is AWS provider specification, used when provider is set to aws
	AWS *ClusterAWSProviderSpecV2 `json:"aws"`
	// Nodes is a list of node profiles with amount to create/update and instance types
	Nodes []ClusterNodeSpecV2 `json:"nodes"`
	// Resources is additional Kubernetes resources
	Resources string `json:"resources"`
	// License is the cluster license
	License string `json:"license"`
	// Status is a cluster status, initialized for existing clusters only
	Status string `json:"status,omitempty"`
}

// ClusterAWSProviderSpecV2 is AWS provider specification
type ClusterAWSProviderSpecV2 struct {
	// Region is AWS region
	Region string `json:"region"`
	// VPC is VPC ID
	VPC string `json:"vpc,omitempty"`
	// KeyName is SSH key name
	KeyName string `json:"keyName"`
}

// ClusterNodeSpecV2 is a spec of cluster node provisioned via AWS
type ClusterNodeSpecV2 struct {
	// Profile is server profile
	Profile string `json:"profile"`
	// InstanceType is instance type to use
	InstanceType string `json:"instanceType"`
	// Count is count of instances
	Count int `json:"count"`
}

// ClusterV2Schema is JSON schema for server
const ClusterSpecV2Schema = `{
  "type": "object",
  "additionalProperties": false,
  "required": ["provider", "nodes"],
  "properties": {
    "app": {"type": "string"},
    "provider": {"type": "string"},
    "aws": {
      "type": "object",
      "additionalProperties": false,
      "required": ["region", "keyName"],
      "properties": {
        "region": {"type": "string"},
        "vpc": {"type": "string"},
        "keyName": {"type": "string"}
      }
    },
    "nodes": {
      "type": "array",
      "items": {
        "type": "object",
        "additionalProperties": false,
        "required": ["profile", "instanceType", "count"],
        "properties": {
          "profile": {"type": "string"},
          "count": {"type": "integer"},
          "instanceType": {"type": "string"}
        }
      }
    },
    "resources": {"type": "string"},
    "license": {"type": "string"},
    "status": {
      "type": "string"
    }
  }
}`

// GetClusterSchema returns cluster schema for V2 resource
func GetClusterSchema() string {
	return fmt.Sprintf(teleservices.V2SchemaTemplate, teleservices.MetadataSchema,
		ClusterSpecV2Schema, "")
}
