package ui

import (
	"fmt"
	"strings"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/trace"

	yaml "github.com/ghodss/yaml"
	teleservices "github.com/gravitational/teleport/lib/services"
)

// ConfigCollection is a collection of ConfigItems
type ConfigCollection struct {
	// Items is a slice of ConfigItems
	Items []ConfigItem `json:"items"`
}

// ConfigItem is UI representation of the resource
type ConfigItem struct {
	// ID is a resource ID which is a composed value based on kind and name
	ID string `json:"id"`
	// Kind is a resource kind
	Kind string `json:"kind"`
	// Name is a resource name
	Name string `json:"name"`
	// DisplayName is a resource display name
	DisplayName string `json:"displayName"`
	// Content is resource yaml content
	Content string `json:"content"`
}

// ConvertRoles creates UI objects for Roles
func ConvertRoles(roles []teleservices.Role) ([]ConfigItem, error) {
	configItems := []ConfigItem{}
	for _, role := range roles {
		if isSystemRole(role) {
			continue
		}

		item, err := NewConfigItem(teleservices.KindRole, role.GetName(), role)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		configItems = append(configItems, *item)
	}

	return configItems, nil
}

// ConvertTrustedClusters creates UI objects for Cluster
func ConvertTrustedClusters(clusters []teleservices.TrustedCluster) ([]ConfigItem, error) {
	configItems := []ConfigItem{}
	for _, cluster := range clusters {
		item, err := NewConfigItem(teleservices.KindTrustedCluster, cluster.GetName(), cluster)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		configItems = append(configItems, *item)
	}

	return configItems, nil
}

// ConvertGithubConnectors creates UI Github connector objects
func ConvertGithubConnectors(connectors []teleservices.GithubConnector) ([]ConfigItem, error) {
	configItems := []ConfigItem{}
	for _, conn := range connectors {
		item, err := NewConfigItem(teleservices.KindGithubConnector, conn.GetName(), conn)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		configItems = append(configItems, *item)
	}

	return configItems, nil
}

// ConvertOIDCConnectors creates UI objects for OIDC connectors
func ConvertOIDCConnectors(connectors []teleservices.OIDCConnector) ([]ConfigItem, error) {
	configItems := []ConfigItem{}
	for _, oidc := range connectors {
		item, err := NewConfigItem(teleservices.KindOIDCConnector, oidc.GetName(), oidc)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		configItems = append(configItems, *item)
	}

	return configItems, nil
}

// ConvertSAMLConnectors creates UI objects for SAML connectors
func ConvertSAMLConnectors(connectors []teleservices.SAMLConnector) ([]ConfigItem, error) {
	configItems := []ConfigItem{}
	for _, saml := range connectors {
		item, err := NewConfigItem(teleservices.KindSAMLConnector, saml.GetName(), saml)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		configItems = append(configItems, *item)
	}

	return configItems, nil
}

// ConvertLogForwarders creates UI objects for Log Forwarders
func ConvertLogForwarders(logForwarders []storage.LogForwarder) ([]ConfigItem, error) {
	configItems := []ConfigItem{}
	for _, logFwrd := range logForwarders {
		item, err := NewConfigItem(storage.KindLogForwarder, logFwrd.GetName(), logFwrd)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		configItems = append(configItems, *item)
	}

	return configItems, nil
}

// NewConfigItem creates UI objects for resource
func NewConfigItem(kind string, name string, resource interface{}) (*ConfigItem, error) {
	data, err := yaml.Marshal(resource)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &ConfigItem{
		ID:          fmt.Sprintf("%v:%v", kind, name),
		Kind:        kind,
		Name:        name,
		DisplayName: name,
		Content:     string(data[:]),
	}, nil

}

// isSystemRole returns true for built-in role
func isSystemRole(role teleservices.Role) bool {
	if strings.HasPrefix(role.GetName(), "ca:") {
		return true
	}

	// allow UI to see @teleadmin role
	if role.GetName() == constants.RoleAdmin {
		return false
	}

	return role.GetMetadata().Labels[constants.SystemLabel] == constants.True
}
