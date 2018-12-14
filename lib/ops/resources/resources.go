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

package resources

import (
	"io"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"

	teleservices "github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
	"k8s.io/apimachinery/pkg/util/yaml"
)

// Resources defines methods each specific resource controller should implement
//
// The reason it exists is because gravity and tele CLI tools each support
// their own set of resources.
type Resources interface {
	// Create creates the provided resource
	Create(CreateRequest) error
	// GetCollection retrieves a collection of specified resources
	GetCollection(ListRequest) (Collection, error)
	// Remove removes the specified resource
	Remove(RemoveRequest) error
}

// ResourceControl allows to create/list/remove resources
//
// A list of supported resources is determined by the specific controller
// it is initialized with.
type ResourceControl struct {
	// Resources is the specific resource controller
	Resources
}

// CreateRequest describes a request to create a resource
type CreateRequest struct {
	// Resource is the resource to create
	Resource teleservices.UnknownResource
	// Upsert is whether to update a resource
	Upsert bool
	// User is the user to create resource for
	User string
}

// ListRequest describes a request to list resources
type ListRequest struct {
	// Kind is kind of the resource
	Kind string
	// Name is name of the resource
	Name string
	// WithSecrets is whether to display hidden resource fields
	WithSecrets bool
	// User is the resource owner
	User string
}

// Check validates the request
func (r ListRequest) Check() error {
	if r.Kind == "" {
		return trace.BadParameter("resource kind is mandatory")
	}
	return nil
}

// RemoveRequest describes a request to remove a resource
type RemoveRequest struct {
	// Kind is kind of the resource
	Kind string
	// Name is name of the resource
	Name string
	// Force is whether to suppress not found errors
	Force bool
	// User is the resource owner
	User string
}

// Check validates the request
func (r RemoveRequest) Check() error {
	if r.Kind == "" {
		return trace.BadParameter("resource kind is mandatory")
	}
	if r.Name == "" {
		return trace.BadParameter("resource name is mandatory")
	}
	return nil
}

// Collection represents printable collection of resources
// that can serialize itself into various format
type Collection interface {
	// WriteText serializes collection in human-friendly text format
	WriteText(w io.Writer) error
	// WriteJSON serializes collection into JSON format
	WriteJSON(w io.Writer) error
	// WriteYAML serializes collection into YAML format
	WriteYAML(w io.Writer) error
	// Resources returns the resources collection in the generic format
	Resources() ([]teleservices.UnknownResource, error)
}

// NewControl creates a new resource control instance
func NewControl(resources Resources) *ResourceControl {
	return &ResourceControl{
		Resources: resources,
	}
}

// Create creates all resources found in the provided data
func (r *ResourceControl) Create(reader io.Reader, upsert bool, user string) (created []teleservices.UnknownResource, err error) {
	decoder := yaml.NewYAMLOrJSONDecoder(reader, defaults.DecoderBufferSize)
	empty := true
	for {
		var raw teleservices.UnknownResource
		err = decoder.Decode(&raw)
		if err != nil {
			break
		}
		empty = false
		err = r.Resources.Create(CreateRequest{
			Resource: raw,
			Upsert:   upsert,
			User:     user,
		})
		if err != nil {
			break
		}
		created = append(created, raw)
	}
	if err != io.EOF {
		return nil, trace.Wrap(err)
	}
	if empty {
		return nil, trace.BadParameter("no resources found, empty input?")
	}
	return created, nil
}

// Get retrieves the specified resource collection and outputs it
func (r *ResourceControl) Get(w io.Writer, kind, name string, withSecrets bool, format constants.Format, user string) error {
	collection, err := r.Resources.GetCollection(ListRequest{
		Kind:        kind,
		Name:        name,
		WithSecrets: withSecrets,
		User:        user,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	switch format {
	case constants.EncodingText:
		return collection.WriteText(w)
	case constants.EncodingJSON:
		return collection.WriteJSON(w)
	case constants.EncodingYAML:
		return collection.WriteYAML(w)
	}
	return trace.BadParameter("unsupported format %q, supported are: %v",
		format, constants.OutputFormats)
}

// Remove removes the specified resource
func (r *ResourceControl) Remove(kind, name string, force bool, user string) error {
	err := r.Resources.Remove(RemoveRequest{
		Kind:  kind,
		Name:  name,
		Force: force,
		User:  user,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}
