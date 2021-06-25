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
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/gravitational/gravity/lib/app/resources"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/modules"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	teleservices "github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
)

// Resources defines methods each specific resource controller should implement
//
// The reason it exists is because gravity and tele CLI tools each support
// their own set of resources.
type Resources interface {
	// Create creates the provided resource
	Create(context.Context, CreateRequest) error
	// GetCollection retrieves a collection of specified resources
	GetCollection(ListRequest) (Collection, error)
	// Remove removes the specified resource
	Remove(context.Context, RemoveRequest) error
}

// Validator is a service to validate resources
type Validator interface {
	// Validate checks whether the specified resource
	// represents a valid resource.
	Validate(storage.UnknownResource) error
}

// Validate checks whether the specified resource
// represents a valid resource.
// Implements Validator
func (r ValidateFunc) Validate(res storage.UnknownResource) error {
	return r(res)
}

// ValidateFunc is a resource validator implemented as a single function
type ValidateFunc func(storage.UnknownResource) error

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
	// SiteKey is the key of the cluster to route request to.
	ops.SiteKey
	// Resource is the resource to create
	Resource teleservices.UnknownResource
	// Upsert is whether to update a resource
	Upsert bool
	// Owner is the user to create resource for
	Owner string
	// Manual defines whether the operation should operate
	// in manual mode.
	// This attribute is operation-specific
	Manual bool
	// Confirmed defines whether the operation has been explicitly approved.
	// This attribute is operation-specific
	Confirmed bool
}

// String returns the request string representation.
func (r CreateRequest) String() string {
	if r.Resource.Kind == storage.KindToken {
		return fmt.Sprintf("CreateResource(Cluster=%v, Kind=%v)",
			r.SiteDomain, r.Resource.Kind)
	}
	return fmt.Sprintf("CreateResource(Cluster=%v, Kind=%v, Name=%v)",
		r.SiteDomain, r.Resource.Kind, r.Resource.Metadata.Name)
}

// Check validates the request
func (r CreateRequest) Check() error {
	if r.Resource.Kind == "" {
		return trace.BadParameter("resource kind is mandatory")
	}
	return nil
}

// ListRequest describes a request to list resources
type ListRequest struct {
	// SiteKey is the key of the cluster to route request to.
	ops.SiteKey
	// Kind is kind of the resource
	Kind string
	// Name is name of the resource
	Name string
	// WithSecrets is whether to display hidden resource fields
	WithSecrets bool
	// User is the resource owner
	User string
}

// String returns the request string representation.
func (r ListRequest) String() string {
	if r.Kind == storage.KindToken {
		return fmt.Sprintf("ListResources(Cluster=%v, Kind=%v)",
			r.SiteDomain, r.Kind)
	}
	return fmt.Sprintf("ListResources(Cluster=%v, Kind=%v, Name=%v)",
		r.SiteDomain, r.Kind, r.Name)
}

// Check validates the request
func (r *ListRequest) Check() error {
	if r.Kind == "" {
		return trace.BadParameter("resource kind is mandatory")
	}
	kind := modules.GetResources().CanonicalKind(r.Kind)
	resources := modules.GetResources().SupportedResources()
	if !utils.StringInSlice(resources, kind) {
		return trace.BadParameter("unknown resource kind %q", r.Kind)
	}
	r.Kind = kind
	return nil
}

// RemoveRequest describes a request to remove a resource
type RemoveRequest struct {
	// SiteKey is the key of the cluster to route request to.
	ops.SiteKey
	// Kind is kind of the resource
	Kind string
	// Name is name of the resource
	Name string
	// Force is whether to suppress not found errors
	Force bool
	// Owner is the resource owner
	Owner string
	// Manual defines whether the operation should operate
	// in manual mode.
	// This attribute is operation-specific
	Manual bool
	// Confirmed defines whether the operation has been explicitly approved.
	// This attribute is operation-specific
	Confirmed bool
}

// String returns the request string representation.
func (r RemoveRequest) String() string {
	if r.Kind == storage.KindToken {
		return fmt.Sprintf("RemoveResource(Cluster=%v, Kind=%v)",
			r.SiteDomain, r.Kind)
	}
	return fmt.Sprintf("RemoveResource(Cluster=%v, Kind=%v, Name=%v)",
		r.SiteDomain, r.Kind, r.Name)
}

// Check validates the request
func (r *RemoveRequest) Check() error {
	if r.Kind == "" {
		return trace.BadParameter("resource kind is mandatory")
	}
	kind := modules.GetResources().CanonicalKind(r.Kind)
	resources := modules.GetResources().SupportedResourcesToRemove()
	if !utils.StringInSlice(resources, kind) {
		return trace.BadParameter("unknown resource kind %q", r.Kind)
	}
	switch kind {
	case storage.KindAlertTarget:
	case storage.KindSMTPConfig:
	case storage.KindRuntimeEnvironment:
	case storage.KindClusterConfiguration:
	case storage.KindPersistentStorage:
	default:
		if r.Name == "" {
			return trace.BadParameter("resource name is mandatory")
		}
	}
	r.Kind = kind
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
func (r *ResourceControl) Create(ctx context.Context, reader io.Reader, req CreateRequest) (err error) {
	err = ForEach(reader, func(res storage.UnknownResource) error {
		req.Resource = teleservices.UnknownResource{
			ResourceHeader: res.ResourceHeader,
			Raw:            res.Raw,
		}
		return trace.Wrap(r.Resources.Create(ctx, req))
	})
	return trace.Wrap(err)
}

// Get retrieves the specified resource collection and outputs it
func (r *ResourceControl) Get(w io.Writer, req ListRequest, format constants.Format) error {
	collection, err := r.Resources.GetCollection(req)
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
	default:
		return trace.BadParameter("unsupported format %q, supported are: %v",
			format, constants.OutputFormats)
	}
}

// Remove removes the specified resource
func (r *ResourceControl) Remove(ctx context.Context, req RemoveRequest) error {
	err := r.Resources.Remove(ctx, req)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// Split interprets the given reader r as a list of resources and splits
// them in two groups: Kubernetes and Gravity resources
func Split(r io.Reader) (kubernetesResources []runtime.Object, gravityResources []storage.UnknownResource, err error) {
	err = ForEach(r, func(resource storage.UnknownResource) error {
		if isKubernetesResource(resource) {
			// reinterpret as a Kubernetes resource
			var unknown resources.Unknown
			if err := json.Unmarshal(resource.Raw, &unknown); err != nil {
				return trace.Wrap(err)
			}
			kubernetesResources = append(kubernetesResources, &unknown)
		} else {
			gravityResources = append(gravityResources, resource)
		}
		return nil
	})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return kubernetesResources, gravityResources, nil
}

// ForEach interprets the given reader r as a collection of Gravity resources
// and invokes the specified handler for each resource in the list.
// Returns the first encountered error
func ForEach(r io.Reader, handler ResourceFunc) (err error) {
	decoder := yaml.NewYAMLOrJSONDecoder(r, defaults.DecoderBufferSize)
	for err == nil || utils.IsAbortError(err) {
		var resource storage.UnknownResource
		err = decoder.Decode(&resource)
		if err != nil {
			break
		}
		resource.Kind = modules.GetResources().CanonicalKind(resource.Kind)
		err = handler(resource)
	}
	if err == io.EOF {
		err = nil
	}
	if origErr, ok := trace.Unwrap(err).(*utils.AbortRetry); ok {
		err = origErr.Err
	}
	return trace.Wrap(err)
}

// ResourceFunc is a callback that operates on a Gravity resource
type ResourceFunc func(storage.UnknownResource) error

func isKubernetesResource(resource storage.UnknownResource) bool {
	return resource.Version == ""
}
