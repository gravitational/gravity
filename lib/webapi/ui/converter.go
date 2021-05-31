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

package ui

import (
	"github.com/gravitational/gravity/lib/storage"

	teleservices "github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
)

// Converter provides methods for converting objects between backend
// and UI representation
type Converter interface {
	// ToConfigItems converts the provided resources to their UI representations
	ToConfigItems([]teleservices.UnknownResource) ([]ConfigItem, error)
}

type converter struct{}

// NewConverter returns a new converter instance
func NewConverter() Converter {
	return &converter{}
}

// ToConfigItems converts the provided resources to their UI representations
func (c *converter) ToConfigItems(resources []teleservices.UnknownResource) ([]ConfigItem, error) {
	var items []ConfigItem
	for _, resource := range resources {
		var item *ConfigItem
		switch resource.Kind {
		case teleservices.KindRole:
			role, err := teleservices.GetRoleMarshaler().UnmarshalRole(resource.Raw)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			if isSystemRole(role) {
				continue // skip this resource
			}
			item, err = NewConfigItem(resource.Kind, role.GetName(), role)
			if err != nil {
				return nil, trace.Wrap(err)
			}
		case teleservices.KindOIDCConnector:
			connector, err := teleservices.GetOIDCConnectorMarshaler().UnmarshalOIDCConnector(resource.Raw)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			item, err = NewConfigItem(resource.Kind, connector.GetName(), connector)
			if err != nil {
				return nil, trace.Wrap(err)
			}
		case teleservices.KindSAMLConnector:
			connector, err := teleservices.GetSAMLConnectorMarshaler().UnmarshalSAMLConnector(resource.Raw)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			item, err = NewConfigItem(resource.Kind, connector.GetName(), connector)
			if err != nil {
				return nil, trace.Wrap(err)
			}
		case teleservices.KindGithubConnector:
			connector, err := teleservices.GetGithubConnectorMarshaler().Unmarshal(resource.Raw)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			item, err = NewConfigItem(resource.Kind, connector.GetName(), connector)
			if err != nil {
				return nil, trace.Wrap(err)
			}
		case storage.KindLogForwarder:
			forwarder, err := storage.GetLogForwarderMarshaler().Unmarshal(resource.Raw)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			item, err = NewConfigItem(resource.Kind, forwarder.GetName(), forwarder)
			if err != nil {
				return nil, trace.Wrap(err)
			}
		case "":
			return nil, trace.BadParameter("resource kind is empty")
		default:
			return nil, trace.BadParameter("resource kind %q is not supported",
				resource.Kind)
		}
		items = append(items, *item)
	}
	return items, nil
}
