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
	"strings"
	"time"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/utils"

	teleservices "github.com/gravitational/teleport/lib/services"
	teleutils "github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

// EnvironmentVariables defines the cluster runtime environment variables resource.
// It allows to override runtime environment variables on each node in the cluster.
// There is only a single instance of the resource in a cluster
type EnvironmentVariables interface {
	// Resource provides common resource methods
	teleservices.Resource
	// CheckAndSetDefaults validates this resource and sets defaults
	CheckAndSetDefaults() error
	// GetKeyValues returns the values of environment variables from this resource
	GetKeyValues() map[string]string
}

// NewEnvironment creates a new instance of the resource
func NewEnvironment(kvs map[string]string) *EnvironmentV1 {
	return &EnvironmentV1{
		Kind:    KindRuntimeEnvironment,
		Version: "v1",
		Metadata: teleservices.Metadata{
			Name:      constants.ClusterEnvironmentMap,
			Namespace: defaults.KubeSystemNamespace,
		},
		Spec: EnvironmentSpec{
			KeyValues: kvs,
		},
	}
}

// EnvironmentV1 describes the cluster runtime environment variables resource
type EnvironmentV1 struct {
	// Kind is a resource kind
	Kind string `json:"kind"`
	// Version is a resource version
	Version string `json:"version"`
	// Metadata specifies resource metadata
	Metadata teleservices.Metadata `json:"metadata"`
	// Spec defines the resource
	Spec EnvironmentSpec `json:"spec"`
}

// GetName returns the name of the resource name
func (r *EnvironmentV1) GetName() string {
	return r.Metadata.Name
}

// SetName resets the resource name to the specified value
func (r *EnvironmentV1) SetName(name string) {
	r.Metadata.Name = name
}

// GetMetadata returns resource metadata
func (r *EnvironmentV1) GetMetadata() teleservices.Metadata {
	return r.Metadata
}

// SetExpiry resets expiration time to the specified value
func (r *EnvironmentV1) SetExpiry(expires time.Time) {
	r.Metadata.SetExpiry(expires)
}

// Expires returns expiration time
func (r *EnvironmentV1) Expiry() time.Time {
	return r.Metadata.Expiry()
}

// SetTTL resets the resources's time to live to the specified value
// using given clock implementation
func (r *EnvironmentV1) SetTTL(clock clockwork.Clock, ttl time.Duration) {
	r.Metadata.SetTTL(clock, ttl)
}

// GetKeyValues returns the values of environment variables from this resource
func (r *EnvironmentV1) GetKeyValues() map[string]string {
	return r.Spec.KeyValues
}

// CheckAndSetDefaults validates this resource and sets defaults
func (r *EnvironmentV1) CheckAndSetDefaults() error {
	var errors []error
	for _, env := range []string{
		constants.HTTPProxyEnvVar, strings.ToLower(constants.HTTPProxyEnvVar),
		constants.HTTPSProxyEnvVar, strings.ToLower(constants.HTTPSProxyEnvVar),
	} {
		if err := r.checkProxy(env); err != nil {
			errors = append(errors, err)
		}
	}
	return trace.NewAggregate(errors...)
}

// checkProxy verifies the specified environment variable (HTTP_PROXY or
// HTTPS_PROXY) is a valid proxy URL.
func (r *EnvironmentV1) checkProxy(env string) error {
	httpProxy := r.getVariable(env)
	if httpProxy == "" {
		return nil
	}
	if _, err := utils.ParseProxy(httpProxy); err != nil {
		return trace.Wrap(err, "failed to parse %v", env)
	}
	return nil
}

// getVariable returns value of the specified environment variable or an empty string.
func (r *EnvironmentV1) getVariable(key string) string {
	for k, v := range r.GetKeyValues() {
		if k == key {
			return v
		}
	}
	return ""
}

// UnmarshalEnvironmentVariables unmarshals the resource from YAML/JSON given with data
func UnmarshalEnvironmentVariables(data []byte) (EnvironmentVariables, error) {
	if len(data) == 0 {
		return &EnvironmentV1{}, nil
	}
	jsonData, err := teleutils.ToJSON(data)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var hdr teleservices.ResourceHeader
	err = json.Unmarshal(jsonData, &hdr)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch hdr.Version {
	case "v1":
		var env EnvironmentV1
		err := teleutils.UnmarshalWithSchema(GetEnvironmentSpecSchema(), &env, jsonData)
		if err != nil {
			return nil, trace.BadParameter(err.Error())
		}
		// Set namespace explicitly - schema default is ignored in json.Unmarshal
		// as teleservices.Metadata.Namespace is missing the json serialization tag
		env.Metadata.Namespace = defaults.KubeSystemNamespace
		env.Metadata.Name = constants.ClusterEnvironmentMap
		if env.Metadata.Expires != nil {
			teleutils.UTC(env.Metadata.Expires)
		}
		return &env, nil
	}
	return nil, trace.BadParameter(
		"%v resource version %q is not supported", KindRuntimeEnvironment, hdr.Version)
}

// MarshalEnvironment marshals this resource as JSON
func MarshalEnvironment(env EnvironmentVariables, opts ...teleservices.MarshalOption) ([]byte, error) {
	return json.Marshal(env)
}

// EnvironmentSpec defines the environment variable resource
type EnvironmentSpec struct {
	// KeyValues specifies the environment
	KeyValues map[string]string `json:"data"`
}

// EnvironmentSpecSchema is JSON schema for the cluster runtime environment variables resource
const EnvironmentSpecSchema = `{
  "type": "object",
  "additionalProperties": false,
  "required": ["kind", "spec", "version"],
  "properties": {
    "kind": {"type": "string"},
    "version": {"type": "string", "default": "v1"},
    "metadata": {
      "default": {},
      "type": "object",
      "additionalProperties": false,
      "properties": {
        "name": {"type": "string", "default": "%v"},
        "namespace": {"type": "string", "default": "%v"},
        "description": {"type": "string"},
        "expires": {"type": "string"},
        "labels": {
          "type": "object",
          "patternProperties": {
             "^[a-zA-Z/.0-9_]$":  {"type": "string"}
          }
        }
      }
    },
    "spec": {
      "type": "object",
      "additionalProperties": false,
      "required": ["data"],
      "properties": {
        "data": {"type": ["object", "null"]}
      }
    }
  }
}`

// GetEnvironmentSpecSchema returns the formatted JSON schema for the cluster runtime environment
// variables resource
func GetEnvironmentSpecSchema() string {
	return fmt.Sprintf(EnvironmentSpecSchema,
		constants.ClusterEnvironmentMap, defaults.KubeSystemNamespace)
}
