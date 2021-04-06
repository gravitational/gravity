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

	teleservices "github.com/gravitational/teleport/lib/services"
	teleutils "github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

// Endpoints represents a resource that allows to customize advertise addresses
// used for user and cluster communication
type Endpoints interface {
	// Resource provides common resource methods
	teleservices.Resource
	// CheckAndSetDefaults makes sure the resource is valid
	CheckAndSetDefaults() error
	// GetPublicAddr returns the public advertise addr
	GetPublicAddr() string
	// GetAgentsAddr returns the agents advertise addr
	GetAgentsAddr() string
}

// NewEndpoints creates a new endpoints resource from the provided spec
func NewEndpoints(spec EndpointsSpecV2) Endpoints {
	return &EndpointsV2{
		Kind:    KindEndpoints,
		Version: teleservices.V2,
		Metadata: teleservices.Metadata{
			Name:      KindEndpoints,
			Namespace: defaults.Namespace,
		},
		Spec: spec,
	}
}

// EndpointsV2 represents the endpoints resource
type EndpointsV2 struct {
	// Kind is the resource kind
	Kind string `json:"kind"`
	// Version is the resource version
	Version string `json:"version"`
	// Metadata is the resource metadata
	Metadata teleservices.Metadata `json:"metadata"`
	// Spec is the resource spec
	Spec EndpointsSpecV2 `json:"spec"`
}

// EndpointsSpecV2 is the endpoints resource spec
type EndpointsSpecV2 struct {
	// PublicAddr is the Ops Center endpoint for user traffic
	PublicAddr string `json:"public_advertise_addr"`
	// AgentsAddr is the Ops Center endpoint for cluster traffic
	AgentsAddr string `json:"agents_advertise_addr"`
}

// GetName returns the resource name
func (e *EndpointsV2) GetName() string {
	return e.Metadata.Name
}

// SetName sets the resource name
func (e *EndpointsV2) SetName(name string) {
	e.Metadata.Name = name
}

// GetMetadata returns the resource metadata
func (e *EndpointsV2) GetMetadata() teleservices.Metadata {
	return e.Metadata
}

// SetExpiry sets the resource expiration time
func (e *EndpointsV2) SetExpiry(expires time.Time) {
	e.Metadata.SetExpiry(expires)
}

// Expires returns the resource expiration time
func (e *EndpointsV2) Expiry() time.Time {
	return e.Metadata.Expiry()
}

// SetTTL sets the resource TTL
func (e *EndpointsV2) SetTTL(clock clockwork.Clock, ttl time.Duration) {
	e.Metadata.SetTTL(clock, ttl)
}

// GetPublicAddr returns the public advertise address
func (e *EndpointsV2) GetPublicAddr() string {
	return e.Spec.PublicAddr
}

// GetAgentsAddr returns the agents advertise address
func (e *EndpointsV2) GetAgentsAddr() string {
	if e.Spec.AgentsAddr != "" {
		return e.Spec.AgentsAddr
	}
	return e.Spec.PublicAddr
}

// CheckAndSetDefaults checks validity of all parameters and sets defaults
func (e *EndpointsV2) CheckAndSetDefaults() error {
	if e.Metadata.Name == "" {
		e.Metadata.Name = KindEndpoints
	}
	if e.Spec.PublicAddr == "" {
		return trace.BadParameter("missing parameter 'public_advertise_addr'")
	}
	return nil
}

// UnmarshalEndpoints unmarshals the endpoints resource from JSON
func UnmarshalEndpoints(data []byte) (Endpoints, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing endpoints data")
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
		var e EndpointsV2
		err := teleutils.UnmarshalWithSchema(GetEndpointsSchema(), &e, jsonData)
		if err != nil {
			return nil, trace.BadParameter(err.Error())
		}
		//nolint:errcheck
		e.Metadata.CheckAndSetDefaults()
		return &e, nil
	}
	return nil, trace.BadParameter(
		"%v resource version %q is not supported", KindEndpoints, h.Version)
}

// MarshalEndpoints marshals the endpoints resource to JSON
func MarshalEndpoints(endpoints Endpoints, opts ...teleservices.MarshalOption) ([]byte, error) {
	return json.Marshal(endpoints)
}

// EndpointsSpecV2Schema is the endpoints resource JSON schema
const EndpointsSpecV2Schema = `{
  "type": "object",
  "additionalProperties": false,
  "required": ["public_advertise_addr"],
  "properties": {
    "public_advertise_addr": {"type": "string"},
    "agents_advertise_addr": {"type": "string"}
  }
}`

// GetEndpointsSchema returns the endpoints resource schema
func GetEndpointsSchema() string {
	return fmt.Sprintf(teleservices.V2SchemaTemplate, teleservices.MetadataSchema,
		EndpointsSpecV2Schema, "")
}
