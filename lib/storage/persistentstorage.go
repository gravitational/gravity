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
	"strings"
	"time"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"

	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"gopkg.in/yaml.v2"
)

// PersistentStorage represents persistent storage configuration resource.
type PersistentStorage interface {
	// Resource provides common resource methods.
	services.Resource
	// CheckAndSetDefaults validates the object and sets defaults.
	CheckAndSetDefaults() error
	// GetMountExcludes returns mount points to exclude when discovering devices.
	GetMountExcludes() []string
	// GetVendorIncludes returns vendor names to include when discovering devices.
	GetVendorIncludes() []string
	// GetVendorExcludes returns vendor names to exclude when discovering devices.
	GetVendorExcludes() []string
	// GetDeviceIncludes returns device names to include when discovering devices.
	GetDeviceIncludes() []string
	// GetDeviceExcludes returns device names to exclude when discovering devices.
	GetDeviceExcludes() []string
}

// NewPersistentStorage creates a new persistent storage resource from the
// provided spec.
func NewPersistentStorage(spec PersistentStorageSpecV1) PersistentStorage {
	return &PersistentStorageV1{
		Kind:    KindPersistentStorage,
		Version: services.V1,
		Metadata: services.Metadata{
			Name:      KindPersistentStorage,
			Namespace: defaults.Namespace,
		},
		Spec: spec,
	}
}

// PersistentStorageFromNDMConfig creates a new persistent storage resource
// from the provided Node Device Manager configuration.
func PersistentStorageFromNDMConfig(c *NDMConfig) PersistentStorage {
	return NewPersistentStorage(PersistentStorageSpecV1{
		OpenEBS: OpenEBS{
			Filters: OpenEBSFilters{
				MountPoints: OpenEBSFilter{
					Exclude: c.MountExcludes(),
				},
				Vendors: OpenEBSFilter{
					Exclude: c.VendorExcludes(),
					Include: c.VendorIncludes(),
				},
				Devices: OpenEBSFilter{
					Exclude: c.DeviceExcludes(),
					Include: c.DeviceIncludes(),
				},
			},
		},
	})
}

// DefaultPersistentStorage returns a new default persistent storage resource.
func DefaultPersistentStorage() PersistentStorage {
	ps := &PersistentStorageV1{
		Kind:    KindPersistentStorage,
		Version: services.V1,
	}
	//nolint:errcheck
	ps.CheckAndSetDefaults()
	return ps
}

// PersistentStorageV1 represents a persistent storage resource.
type PersistentStorageV1 struct {
	// Kind is the resource kind, always PersistentStorage.
	Kind string `json:"kind"`
	// Version is the resource version.
	Version string `json:"version"`
	// Metadata is the resource metadata.
	Metadata services.Metadata `json:"metadata"`
	// Spec is the resource spec.
	Spec PersistentStorageSpecV1 `json:"spec"`
}

// PersistentStorageSpecV1 is persistent storage resource spec.
type PersistentStorageSpecV1 struct {
	// OpenEBS contains OpenEBS configuration.
	OpenEBS OpenEBS `json:"openebs"`
}

// OpenEBS represents OpenEBS configuration.
type OpenEBS struct {
	// Filters is a list of filters OpenEBS will use when discovering devices.
	Filters OpenEBSFilters `json:"filters"`
}

// OpenEBSFilters is a list of filters OpenEBS will use when discovering devices.
type OpenEBSFilters struct {
	// MountPoints filters devices based on directory mount points.
	MountPoints OpenEBSFilter `json:"mountPoints"`
	// Vendors filters devices based on their vendor names.
	Vendors OpenEBSFilter `json:"vendors"`
	// Devices filters devices based on their names.
	Devices OpenEBSFilter `json:"devices"`
}

// OpenEBSFilter represents a single filter type.
type OpenEBSFilter struct {
	// Include defines filters to include when discovering devices.
	Include []string `json:"include,omitempty"`
	// Exclude defines filters to exclude when discovering devices.
	Exclude []string `json:"exclude,omitempty"`
}

// GetName returns the resource name.
func (ps *PersistentStorageV1) GetName() string {
	return ps.Metadata.Name
}

// SetName sets the resource name.
func (ps *PersistentStorageV1) SetName(name string) {
	ps.Metadata.Name = name
}

// GetMetadata returns the resource metadata.
func (ps *PersistentStorageV1) GetMetadata() services.Metadata {
	return ps.Metadata
}

// SetExpiry sets the resource expiration time.
func (ps *PersistentStorageV1) SetExpiry(expires time.Time) {
	ps.Metadata.SetExpiry(expires)
}

// Expiry returns the resource expiration time.
func (ps *PersistentStorageV1) Expiry() time.Time {
	return ps.Metadata.Expiry()
}

// SetTTL sets the resource TTL.
func (ps *PersistentStorageV1) SetTTL(clock clockwork.Clock, ttl time.Duration) {
	ps.Metadata.SetTTL(clock, ttl)
}

// GetMountExcludes returns mount points to exclude when discovering devices.
func (ps *PersistentStorageV1) GetMountExcludes() []string {
	return ps.Spec.OpenEBS.Filters.MountPoints.Exclude
}

// GetVendorIncludes returns vendor names to include when discovering devices.
func (ps *PersistentStorageV1) GetVendorIncludes() []string {
	return ps.Spec.OpenEBS.Filters.Vendors.Include
}

// GetVendorExcludes returns vendor names to exclude when discovering devices.
func (ps *PersistentStorageV1) GetVendorExcludes() []string {
	return ps.Spec.OpenEBS.Filters.Vendors.Exclude
}

// GetDeviceIncludes returns device names to include when discovering devices.
func (ps *PersistentStorageV1) GetDeviceIncludes() []string {
	return ps.Spec.OpenEBS.Filters.Devices.Include
}

// GetDeviceExcludes returns device names to exclude when discovering devices.
func (ps *PersistentStorageV1) GetDeviceExcludes() []string {
	return ps.Spec.OpenEBS.Filters.Devices.Exclude
}

// CheckAndSetDefaults validates the resources and sets defaults.
func (ps *PersistentStorageV1) CheckAndSetDefaults() error {
	if ps.Metadata.Name == "" {
		ps.Metadata.Name = KindPersistentStorage
	}
	if err := ps.Metadata.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	// Make sure to always include default excludes for paths/devices/vendors
	// to prevent OpenEBS from discovering things it should not.
	ps.Spec.OpenEBS.Filters.MountPoints.Exclude = utils.Deduplicate(
		append(defaultExcludeMounts, ps.Spec.OpenEBS.Filters.MountPoints.Exclude...))
	ps.Spec.OpenEBS.Filters.Vendors.Exclude = utils.Deduplicate(
		append(defaultExcludeVendors, ps.Spec.OpenEBS.Filters.Vendors.Exclude...))
	ps.Spec.OpenEBS.Filters.Devices.Exclude = utils.Deduplicate(
		append(defaultExcludeDevices, ps.Spec.OpenEBS.Filters.Devices.Exclude...))
	return nil
}

// UnmarshalPersistentStorage unmarshals provided data into persistent storage resource.
func UnmarshalPersistentStorage(data []byte) (PersistentStorage, error) {
	jsonData, err := utils.ToJSON(data)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var header services.ResourceHeader
	if err := json.Unmarshal(jsonData, &header); err != nil {
		return nil, trace.Wrap(err)
	}
	switch header.Version {
	case services.V1:
		var ps PersistentStorageV1
		err := utils.UnmarshalWithSchema(GetPersistentStorageSchema(), &ps, jsonData)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if err := ps.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}
		return &ps, nil
	}
	return nil, trace.BadParameter("%v resource version %q is not supported",
		KindPersistentStorage, header.Version)
}

// MarshalPersistentStorage marshals persistent storage resource into a json.
func MarshalPersistentStorage(ps PersistentStorage, opts ...services.MarshalOption) ([]byte, error) {
	return json.Marshal(ps)
}

// GetPersistentStorageSchema returns the full persistent storage resource schema.
func GetPersistentStorageSchema() string {
	return fmt.Sprintf(services.V2SchemaTemplate, MetadataSchema,
		PersistentStorageSpecV1Schema, "")
}

// PersistentStorageSpecV1Schema is the persistent storage resource spec schema.
var PersistentStorageSpecV1Schema = `{
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "openebs": {
      "type": "object",
      "additionalProperties": false,
      "properties": {
        "filters": {
          "type": "object",
          "additionalProperties": false,
          "properties": {
            "mountPoints": {
              "type": "object",
              "additionalProperties": false,
              "properties": {
                "exclude": {"type": "array", "items": {"type": "string"}}
              }
            },
            "vendors": {
              "type": "object",
              "additionalProperties": false,
              "properties": {
                "include": {"type": "array", "items": {"type": "string"}},
                "exclude": {"type": "array", "items": {"type": "string"}}
              }
            },
            "devices": {
              "type": "object",
              "additionalProperties": false,
              "properties": {
                "include": {"type": "array", "items": {"type": "string"}},
                "exclude": {"type": "array", "items": {"type": "string"}}
              }
            }
          }
        }
      }
    }
  }
}`

// NDMConfig represents the OpenEBS Node Device Manager configuration.
type NDMConfig struct {
	// ProbeConfigs contains probes NDM performs when discovering devices.
	ProbeConfigs []*NDMProbe `yaml:"probeconfigs"`
	// FilterConfigs contains filters NDM considers when discovering devices.
	FilterConfigs []*NDMFilter `yaml:"filterconfigs"`
}

func (c *NDMConfig) getFilter(key string) *NDMFilter {
	for _, filter := range c.FilterConfigs {
		if filter.Key == key {
			return filter
		}
	}
	return &NDMFilter{}
}

// MountExcludes returns mount exclude filter.
func (c *NDMConfig) MountExcludes() []string {
	return strings.Split(c.getFilter("os-disk-exclude-filter").Exclude, ",")
}

// SetMountExcludes sets mount exclude filter.
func (c *NDMConfig) SetMountExcludes(excludes []string) {
	c.getFilter("os-disk-exclude-filter").Exclude = strings.Join(excludes, ",")
}

// VendorExcludes returns vendor exclude filter.
func (c *NDMConfig) VendorExcludes() []string {
	return strings.Split(c.getFilter("vendor-filter").Exclude, ",")
}

// SetVendorExcludes sets vendor exclude filter.
func (c *NDMConfig) SetVendorExcludes(excludes []string) {
	c.getFilter("vendor-filter").Exclude = strings.Join(excludes, ",")
}

// VendorIncludes returns vendor include filter.
func (c *NDMConfig) VendorIncludes() []string {
	return strings.Split(c.getFilter("vendor-filter").Include, ",")
}

// SetVendorIncludes sets vendor include filter.
func (c *NDMConfig) SetVendorIncludes(includes []string) {
	c.getFilter("vendor-filter").Include = strings.Join(includes, ",")
}

// DeviceExcludes returns device exclude filter.
func (c *NDMConfig) DeviceExcludes() []string {
	return strings.Split(c.getFilter("path-filter").Exclude, ",")
}

// SetDeviceExcludes sets device exclude filter.
func (c *NDMConfig) SetDeviceExcludes(excludes []string) {
	c.getFilter("path-filter").Exclude = strings.Join(excludes, ",")
}

// DeviceIncludes returns device include filter.
func (c *NDMConfig) DeviceIncludes() []string {
	return strings.Split(c.getFilter("path-filter").Include, ",")
}

// SetDeviceIncludes sets device include filter.
func (c *NDMConfig) SetDeviceIncludes(includes []string) {
	c.getFilter("path-filter").Include = strings.Join(includes, ",")
}

// DefaultNDMConfig returns a default NDM config.
func DefaultNDMConfig() *NDMConfig {
	return &NDMConfig{
		ProbeConfigs: []*NDMProbe{
			{Name: "udev probe", Key: udevProbe, State: true},
			{Name: "searchest probe", Key: searchestProbe, State: false},
			{Name: "smart probe", Key: smartProbe, State: true},
		},
		FilterConfigs: []*NDMFilter{
			{
				Name:    "os disk exclude filter",
				Key:     osDiskFilter,
				State:   true,
				Exclude: strings.Join(defaultExcludeMounts, ","),
			},
			{
				Name:    "vendor filter",
				Key:     vendorFilter,
				State:   true,
				Exclude: strings.Join(defaultExcludeVendors, ","),
			},
			{
				Name:    "path filter",
				Key:     pathFilter,
				State:   true,
				Exclude: strings.Join(defaultExcludeDevices, ","),
			},
		},
	}
}

const (
	udevProbe      = "udev-probe"
	searchestProbe = "searchest-probe"
	smartProbe     = "smart-probe"
	osDiskFilter   = "os-disk-exclude-filter"
	vendorFilter   = "vendor-filter"
	pathFilter     = "path-filter"
)

// NDMConfigFromConfigMap creates NDM config from the provided config map.
func NDMConfigFromConfigMap(cm *v1.ConfigMap) (*NDMConfig, error) {
	data := cm.Data["node-disk-manager.config"]
	if len(data) == 0 {
		return nil, trace.BadParameter("config map %v does not contain node disk manager configuration", cm.Name)
	}
	var config NDMConfig
	if err := yaml.Unmarshal([]byte(data), &config); err != nil {
		return nil, trace.Wrap(err)
	}
	return &config, nil
}

// Apply applies parameters from the provided resource to this configuration.
func (c *NDMConfig) Apply(ps PersistentStorage) {
	c.SetMountExcludes(ps.GetMountExcludes())
	c.SetVendorIncludes(ps.GetVendorIncludes())
	c.SetVendorExcludes(ps.GetVendorExcludes())
	c.SetDeviceIncludes(ps.GetDeviceIncludes())
	c.SetDeviceExcludes(ps.GetDeviceExcludes())
}

// ToConfigMap creates a config map from this NDM config.
func (c *NDMConfig) ToConfigMap() (*v1.ConfigMap, error) {
	data, err := yaml.Marshal(c)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       constants.KindConfigMap,
			APIVersion: metav1.SchemeGroupVersion.Version,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.OpenEBSNDMConfigMap,
			Namespace: defaults.OpenEBSNamespace,
			Labels: map[string]string{
				"openebs.io/component-name": "ndm-config",
			},
		},
		Data: map[string]string{
			"node-disk-manager.config": string(data),
		},
	}, nil
}

// NDMProbe represents a single NDM probe configuration.
type NDMProbe struct {
	// Name is the probe name.
	Name string `yaml:"name"`
	// Key is the probe id.
	Key string `yaml:"key"`
	// State is the probe state (enabled/disabled).
	State bool `yaml:"state"`
}

// NDMFilter represents a single NDM filter.
type NDMFilter struct {
	// Name is the filter name.
	Name string `yaml:"name"`
	// Key is the filter id.
	Key string `yaml:"key"`
	// State is the filter state (enabled/disabled).
	State bool `yaml:"state"`
	// Include is a list of includes for this filter.
	Include string `yaml:"include,omitempty"`
	// Exclude is a list of excludes for this filter.
	Exclude string `yaml:"exclude,omitempty"`
}

var (
	defaultExcludeMounts  = []string{"/", "/etc/hosts", "/boot"}
	defaultExcludeVendors = []string{"CLOUDBYT", "OpenEBS"}
	defaultExcludeDevices = []string{"loop", "/dev/fd0", "/dev/sr0", "/dev/ram", "/dev/dm-", "/dev/md"}
)
