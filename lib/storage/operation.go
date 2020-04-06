/*
Copyright 2020 Gravitational, Inc.

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

	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

// Operation represents a single cluster operation.
type Operation interface {
	// Resource provides common resource methods.
	services.Resource
	// CheckAndSetDefaults validates the object and sets defaults.
	CheckAndSetDefaults() error
	// GetType returns the operation type.
	GetType() string
	// GetCreates returns the operation created timestamp.
	GetCreated() time.Time
	// GetUpdated returns the operation last updated timestamp.
	GetUpdated() time.Time
	// GetState returns the operation state.
	GetState() string
	// GetInstall returns install operation data.
	GetInstall() *OperationInstall
	// GetExpand returns expand operation data.
	GetExpand() *OperationExpand
	// GetShrink returns shrink operation data.
	GetShrink() *OperationShrink
	// GetUpgrade returns upgrade operation data.
	GetUpgrade() *OperationUpgrade
	// GetUpdateEnviron returns environment update operation data.
	GetUpdateEnviron() *OperationUpdateEnviron
	// GetUpdateConfig returns runtime configuration update operation data.
	GetUpdateConfig() *OperationUpdateConfig
	// GetReconfigure returns reconfigure operation data.
	GetReconfigure() *OperationReconfigure
}

// OperationV2 is the operation resource spec.
type OperationV2 struct {
	// Kind is the operation resource kind.
	Kind string `json:"kind"`
	// Version is the operation resource version.
	Version string `json:"version"`
	// Metadata is the operation metadata.
	Metadata services.Metadata `json:"metadata"`
	// Spec is the operation spec.
	Spec OperationSpecV2 `json:"spec"`
}

type OperationSpecV2 struct {
	// Type is the operation type.
	Type string `json:"type"`
	// Created is when the operation was created.
	Created time.Time `json:"created"`
	// Updated is when the operation was last updated.
	Updated time.Time `json:"updated"`
	// State is the operation state.
	State string `json:"state"`
	// Install is install operation data.
	Install *OperationInstall `json:"install,omitempty"`
	// Expand is expand operation data.
	Expand *OperationExpand `json:"expand,omitempty"`
	// Shrink is shrink operation data.
	Shrink *OperationShrink `json:"shrink,omitempty"`
	// Upgrade is upgrade operation data.
	Upgrade *OperationUpgrade `json:"upgrade,omitempty"`
	// UpdateEnviron is environment update operation data.
	UpdateEnviron *OperationUpdateEnviron `json:"updateEnviron,omitempty"`
	// UpdateConfig is runtime configuration update operation data.
	UpdateConfig *OperationUpdateConfig `json:"updateConfig,omitempty"`
	// Reconfigure is advertise IP reconfiguration operation data.
	Reconfigure *OperationReconfigure `json:"reconfigure,omitempty"`
}

type OperationNode struct {
	// IP is the node advertise IP address.
	IP string `json:"ip"`
	// Hostname is the node hostname.
	Hostname string `json:"hostname"`
	// Role is the node role.
	Role string `json:"role"`
}

type OperationInstall struct {
	// Nodes is a list of nodes participating in installation.
	Nodes []OperationNode `json:"nodes"`
}

type OperationExpand struct {
	// Node is the joining node.
	Node OperationNode `json:"node"`
}

type OperationShrink struct {
	// Node is the node that's leaving.
	Node OperationNode `json:"node"`
}

type OperationUpgrade struct {
	// Package is the upgrade package.
	Package string `json:"package"`
}

type OperationUpdateEnviron struct {
	// Env is the new environment.
	Env map[string]string `json:"env"`
}

type OperationUpdateConfig struct {
	// Config is the new runtime config.
	Config []byte `json:"config"`
}

type OperationReconfigure struct {
	// IP is the new advertise IP address.
	IP string `json:"ip"`
}

// GetName returns operation id.
func (o *OperationV2) GetName() string {
	return o.Metadata.Name
}

// SetName sets operation id.
func (o *OperationV2) SetName(id string) {
	o.Metadata.Name = id
}

// GetMetadata returns operation metadata.
func (o *OperationV2) GetMetadata() services.Metadata {
	return o.Metadata
}

// SetExpiry sets the resource expiration time.
func (o *OperationV2) SetExpiry(expires time.Time) {
	o.Metadata.SetExpiry(expires)
}

// Expires returns the resource expiration time.
func (o *OperationV2) Expiry() time.Time {
	return o.Metadata.Expiry()
}

// SetTTL sets the resource ttl.
func (o *OperationV2) SetTTL(clock clockwork.Clock, ttl time.Duration) {
	o.Metadata.SetTTL(clock, ttl)
}

// GetType returns the operation type.
func (o *OperationV2) GetType() string {
	return o.Spec.Type
}

// GetCreates returns the operation created timestamp.
func (o *OperationV2) GetCreated() time.Time {
	return o.Spec.Created
}

// GetUpdated returns the operation last updated timestamp.
func (o *OperationV2) GetUpdated() time.Time {
	return o.Spec.Updated
}

// GetState returns the operation state.
func (o *OperationV2) GetState() string {
	return o.Spec.State
}

// GetInstall returns install operation data.
func (o *OperationV2) GetInstall() *OperationInstall {
	return o.Spec.Install
}

// GetExpand returns expand operation data.
func (o *OperationV2) GetExpand() *OperationExpand {
	return o.Spec.Expand
}

// GetShrink returns shrink operation data.
func (o *OperationV2) GetShrink() *OperationShrink {
	return o.Spec.Shrink
}

// GetUpgrade returns upgrade operation data.
func (o *OperationV2) GetUpgrade() *OperationUpgrade {
	return o.Spec.Upgrade
}

// GetUpdateEnviron returns environment update operation data.
func (o *OperationV2) GetUpdateEnviron() *OperationUpdateEnviron {
	return o.Spec.UpdateEnviron
}

// GetUpdateConfig returns runtime configuration update operation data.
func (o *OperationV2) GetUpdateConfig() *OperationUpdateConfig {
	return o.Spec.UpdateConfig
}

// GetReconfigure returns reconfigure operation data.
func (o *OperationV2) GetReconfigure() *OperationReconfigure {
	return o.Spec.Reconfigure
}

// CheckAndSetDefaults validates operation resource and sets defaults.
func (o *OperationV2) CheckAndSetDefaults() error {
	if o.Metadata.Name == "" {
		return trace.BadParameter("operation name can't be empty")
	}
	if o.Spec.Type == "" {
		return trace.BadParameter("operation type can't be empty")
	}
	if o.Spec.State == "" {
		return trace.BadParameter("operation state can't be empty")
	}
	return nil
}

// UnmarshalOperation unmarshals operation resource from json.
func UnmarshalOperation(data []byte) (Operation, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("empty operation resource data")
	}
	jsonData, err := utils.ToJSON(data)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var h services.ResourceHeader
	err = json.Unmarshal(jsonData, &h)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch h.Version {
	case services.V2:
		var operation OperationV2
		err := utils.UnmarshalWithSchema(GetOperationSchema(), &operation, jsonData)
		if err != nil {
			return nil, trace.BadParameter(err.Error())
		}
		//nolint:errcheck
		operation.Metadata.CheckAndSetDefaults()
		return &operation, nil
	}
	return nil, trace.BadParameter(
		"operation resource version %q is not supported", h.Version)
}

// MarshalOperation marshals operation resource as json.
func MarshalOperation(operation Operation, opts ...services.MarshalOption) ([]byte, error) {
	return json.Marshal(operation)
}

// GetOperationSchema returns a cluster operation schema.
func GetOperationSchema() string {
	return fmt.Sprintf(services.V2SchemaTemplate, services.MetadataSchema,
		OperationSpecV2Schema, "")
}

// OperationSpecV2Schema is the operation json schema.
var OperationSpecV2Schema = fmt.Sprintf(`{
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "type": {"type": "string"},
    "created": {"type": "string"},
    "updated": {"type": "string"},
    "install": {
      "type": "object",
      "additionalProperties": false,
      "properties": {
        "nodes": {
          "type": "array",
          "items": %[0]v
        }
      }
    },
    "expand": {
      "type": "object",
      "additionalProperties": false,
      "properties": {
        "node": %[0]v
      }
    },
    "shrink": {
      "type": "object",
      "additionalProperties": false,
      "properties": {
        "node": %[0]v
      }
    },
    "upgrade": {
      "type": "object",
      "additionalProperties": false,
      "properties": {
        "package": {"type": "string"}
      }
    },
    "updateEnviron": {
      "type": "object",
      "additionalProperties": false,
      "properties": {
        "env": {"type": "object"}
      }
    },
    "updateConfig": {
      "type": "object",
      "additionalProperties": false,
      "properties": {
        "config": {"type": "string"}
      }
    },
    "reconfigure": {
      "type": "object",
      "additionalProperties": false,
      "properties": {
        "ip": {"type": "string"}
      }
    }
  }
}`, OperationNodeSchema)

// OperationNodeSchema is a single operation node json schema.
var OperationNodeSchema = `{
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "ip": {"type": "string"},
    "hostname": {"type": "string"},
    "role": {"type": "string"}
  }
}
`
