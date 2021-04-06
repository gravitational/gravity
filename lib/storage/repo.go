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

	teledefaults "github.com/gravitational/teleport/lib/defaults"
	teleservices "github.com/gravitational/teleport/lib/services"
	teleutils "github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

// Repository is a repository resource
type Repository interface {
	// Resource provides common resource methods
	teleservices.Resource
}

// NewRepository returns new repository object from repo name
func NewRepository(name string) *RepositoryV2 {
	return &RepositoryV2{
		Kind:    KindRepository,
		Version: teleservices.V2,
		Metadata: teleservices.Metadata{
			Name:      name,
			Namespace: teledefaults.Namespace,
		},
	}
}

// RepositoryV2 represents repository resource specification
type RepositoryV2 struct {
	// Kind is a resource kind - always resource
	Kind string `json:"kind"`
	// Version is a resource version
	Version string `json:"version"`
	// Metadata is cluster metadata
	Metadata teleservices.Metadata `json:"metadata"`
	// Spec is repository specification
	Spec struct{} `json:"spec"`
}

// GetName returns cluster name and is a shortcut for GetMetadata().Name
func (t *RepositoryV2) GetName() string {
	return t.Metadata.Name
}

// SetName sets cluster name
func (c *RepositoryV2) SetName(name string) {
	c.Metadata.Name = name
}

// GetMetadata returns cluster metadata
func (c *RepositoryV2) GetMetadata() teleservices.Metadata {
	return c.Metadata
}

// SetExpiry sets cluster expiration time
func (c *RepositoryV2) SetExpiry(expires time.Time) {
	c.Metadata.SetExpiry(expires)
}

// Expires returns cluster expiration time
func (c *RepositoryV2) Expiry() time.Time {
	return c.Metadata.Expiry()
}

// SetTTL sets Expires header using realtime clock
func (c *RepositoryV2) SetTTL(clock clockwork.Clock, ttl time.Duration) {
	c.Metadata.SetTTL(clock, ttl)
}

// RepositoryV1 is a collection of packages
type RepositoryV1 struct {
	// Name is a unique repository name, usually domain name, e.g. example.com
	Name string

	// Expires sets expiry for this repository and all packages
	// inside this repository
	Expires time.Time
}

// String returns human readable representation of the repository
func (r RepositoryV1) String() string {
	return fmt.Sprintf(
		"repository(name=%v)", r.Name)
}

// V2 returns V2 version of Repository resource
func (r *RepositoryV1) V2() *RepositoryV2 {
	new := NewRepository(r.Name)
	if !r.Expires.IsZero() {
		new.SetExpiry(r.Expires)
	}
	//nolint:errcheck
	new.Metadata.CheckAndSetDefaults()
	return new
}

// RepositorySpecV2Schema is JSON schema for repository spec
const RepositorySpecV2Schema = `{
  "type": "object",
  "additionalProperties": false,
  "properties": {}
}`

// GetRepositorySchema returns V2 schema of the repository
func GetRepositorySchema() string {
	return fmt.Sprintf(teleservices.V2SchemaTemplate, teleservices.MetadataSchema,
		RepositorySpecV2Schema, "")
}

// UnmarshalRepository unmarshals repository from JSON
func UnmarshalRepository(data []byte) (Repository, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing repository data")
	}
	jsonData, err := teleutils.ToJSON(data)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var h teleservices.ResourceHeader
	err = json.Unmarshal(jsonData, &h)
	if err != nil {
		h.Version = teleservices.V2
	}

	switch h.Version {
	case teleservices.V2:
		var repository RepositoryV2
		if err := teleutils.UnmarshalWithSchema(GetRepositorySchema(), &repository, jsonData); err != nil {
			return nil, trace.BadParameter(err.Error())
		}
		// we are ignoring error from this function on purpose here
		//nolint:errcheck
		repository.Metadata.CheckAndSetDefaults()
		return &repository, nil
	case "":
		var repository RepositoryV1
		err := json.Unmarshal(jsonData, &repository)
		if err != nil {
			return nil, trace.BadParameter(err.Error())
		}
		return repository.V2(), nil
	}

	return nil, trace.BadParameter("repository version %q is not supported", h.Version)
}

// MarshalRepository marshalls repository into JSON
func MarshalRepository(r Repository, opts ...teleservices.MarshalOption) ([]byte, error) {
	return json.Marshal(r)
}
