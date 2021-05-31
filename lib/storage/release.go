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

package storage

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"helm.sh/helm/v3/pkg/release"
)

// Release represents a single instance of a running application.
type Release interface {
	// Resource provides base resource methods.
	services.Resource
	// GetChartName returns the name of the deployed chart.
	GetChartName() string
	// GetChartVersion returns the deployed chart version.
	GetChartVersion() string
	// GetChartIcon returns the chart application icon.
	GetChartIcon() string
	// SetChartIcon sets the chart application icon.
	SetChartIcon(string)
	// GetChart returns the full chart name that includes version.
	GetChart() string
	// GetAppVersion returns the application version (may be empty).
	GetAppVersion() string
	// GetNamespace returns namespace where chart is deployed.
	GetNamespace() string
	// GetStatus returns the release deployment status.
	GetStatus() string
	// GetRevision returns the release revision number.
	GetRevision() int
	// GetUpdated returns the release last updated timestamp.
	GetUpdated() time.Time
	// GetLocator returns locator of the corresponding application package.
	GetLocator() loc.Locator
}

// NewRelease creates a new release resource from the provided Helm release.
func NewRelease(release *release.Release) (Release, error) {
	if err := verifyRelease(release); err != nil {
		return nil, trace.Wrap(err, "release is missing relevant fields")
	}

	return &ReleaseV1{
		Kind:    KindRelease,
		Version: services.V1,
		Metadata: services.Metadata{
			Name:        release.Name,
			Description: release.Chart.Metadata.Description,
		},
		Spec: ReleaseSpecV1{
			ChartName:    release.Chart.Metadata.Name,
			ChartVersion: release.Chart.Metadata.Version,
			AppVersion:   release.Chart.Metadata.AppVersion,
			Namespace:    release.Namespace,
		},
		Status: ReleaseStatusV1{
			Status:   release.Info.Status.String(),
			Revision: release.Version,
			Updated:  release.Info.LastDeployed.Time,
		},
	}, nil
}

// verifyRelease returns an error if any relevant fields are missing.
func verifyRelease(release *release.Release) error {
	if release == nil {
		return trace.BadParameter("release is nil")
	}
	if release.Chart == nil {
		return trace.BadParameter("chart is nil")
	}
	if release.Chart.Metadata == nil {
		return trace.BadParameter("metadata is nil")
	}
	if release.Info == nil {
		return trace.BadParameter("info is nil")
	}
	return nil
}

// ReleaseV1 defines the release resource.
type ReleaseV1 struct {
	// Kind is the resource kind, always "release" for this resource.
	Kind string `json:"kind"`
	// Version is the resource version, always "v1" for this resource.
	Version string `json:"version"`
	// Metadata is the resource metadata.
	Metadata services.Metadata `json:"metadata"`
	// Spec is the release spec.
	Spec ReleaseSpecV1 `json:"spec"`
	// Status provides runtime information about release.
	Status ReleaseStatusV1 `json:"status"`
}

// ReleaseSpecV1 defines release resource spec.
type ReleaseSpecV1 struct {
	// ChartName is the name of the deployed chart.
	ChartName string `json:"chart_name"`
	// ChartVersion is the deployed chart version.
	ChartVersion string `json:"chart_version"`
	// ChartIcon is the chart application icon.
	ChartIcon string `json:"chart_icon,omitempty"`
	// AppVersion is the application version (may be empty).
	AppVersion string `json:"app_version"`
	// Namespace is the namespace where release is deployed.
	//
	// TODO: This field is a part of spec rather than metadata because
	// Teleport resources are single-namespace at the moment and namespace
	// field from metadata is never exposed.
	Namespace string `json:"namespace"`
}

// ReleaseStatusV1 provides runtime information about release.
type ReleaseStatusV1 struct {
	// Status is the release deployment status.
	Status string `json:"status"`
	// Revision is the release revision number.
	Revision int `json:"revision"`
	// Updated is the release last updated timestamp.
	Updated time.Time `json:"updated"`
}

// GetChartName returns the deployed chart name.
func (r *ReleaseV1) GetChartName() string {
	return r.Spec.ChartName
}

// GetChartVersion returns the deployed chart version.
func (r *ReleaseV1) GetChartVersion() string {
	return r.Spec.ChartVersion
}

// GetChartIcon returns the chart application icon.
func (r *ReleaseV1) GetChartIcon() string {
	return r.Spec.ChartIcon
}

// SetChartIcon sets the chart application icon.
func (r *ReleaseV1) SetChartIcon(val string) {
	r.Spec.ChartIcon = val
}

// GetChart returns the full chart name that includes version.
func (r *ReleaseV1) GetChart() string {
	return fmt.Sprintf("%s-%s", r.Spec.ChartName, r.Spec.ChartVersion)
}

// GetAppVersion returns chart application name.
func (r *ReleaseV1) GetAppVersion() string {
	return r.Spec.AppVersion
}

// GetNamespace returns namespace where chart is deployed.
func (r *ReleaseV1) GetNamespace() string {
	return r.Spec.Namespace
}

// GetStatus returns the release status.
func (r *ReleaseV1) GetStatus() string {
	return r.Status.Status
}

// GetRevision returns the release revision number.
func (r *ReleaseV1) GetRevision() int {
	return r.Status.Revision
}

// GetUpdated returns the release last update timestamp.
func (r *ReleaseV1) GetUpdated() time.Time {
	return r.Status.Updated
}

// GetLocator returns locator of the corresponding application package.
func (r *ReleaseV1) GetLocator() loc.Locator {
	return loc.Locator{
		Repository: defaults.SystemAccountOrg,
		Name:       r.Spec.ChartName,
		Version:    r.Spec.ChartVersion,
	}
}

// GetName returns the resource name.
func (r *ReleaseV1) GetName() string {
	return r.Metadata.Name
}

// SetName sets the resource name.
func (r *ReleaseV1) SetName(name string) {
	r.Metadata.Name = name
}

// GetMetadata returns the resource metadata.
func (r *ReleaseV1) GetMetadata() services.Metadata {
	return r.Metadata
}

// SetExpiry sets the resource expiration time.
func (r *ReleaseV1) SetExpiry(expires time.Time) {
	r.Metadata.SetExpiry(expires)
}

// Expiry returns the resource expiration time.
func (r *ReleaseV1) Expiry() time.Time {
	return r.Metadata.Expiry()
}

// SetTTL sets the resource TTL.
func (r *ReleaseV1) SetTTL(clock clockwork.Clock, ttl time.Duration) {
	r.Metadata.SetTTL(clock, ttl)
}

// MarshalRelease marshals provided release resource to JSON.
func MarshalRelease(release Release, opts ...services.MarshalOption) ([]byte, error) {
	return json.Marshal(release)
}

// UnmarshalRelease unmarshals release resource from the provided data.
func UnmarshalRelease(data []byte) (Release, error) {
	jsonData, err := utils.ToJSON(data)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var header services.ResourceHeader
	err = json.Unmarshal(jsonData, &header)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch header.Version {
	case services.V1:
		var release ReleaseV1
		err := utils.UnmarshalWithSchema(GetReleaseSchema(), &release, jsonData)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &release, nil
	}
	return nil, trace.BadParameter("%v resource version %q is not supported",
		KindRelease, header.Version)
}

// GetReleaseSchema returns the full release resource schema.
func GetReleaseSchema() string {
	return fmt.Sprintf(services.V2SchemaTemplate, services.MetadataSchema,
		ReleaseV1Schema, "")
}

// ReleaseV1Schema defines the release resource schema.
var ReleaseV1Schema = `{
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "chart_name": {"type": "string"},
    "chart_version": {"type": "string"},
    "chart_icon": {"type": "string"},
    "app_version": {"type": "string"},
    "namespace": {"type": "string"}
  }
},
"status": {
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "status": {"type": "string"},
    "revision": {"type": "number"},
    "updated": {"type": "string"}
  }
}`
