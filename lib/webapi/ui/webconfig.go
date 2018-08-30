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
	teleui "github.com/gravitational/teleport/lib/web/ui"
)

// NewOIDCAuthProvider creates AuthProvider of OIDC type
func NewOIDCAuthProvider(name string, displayName string) teleui.WebConfigAuthProvider {
	return teleui.WebConfigAuthProvider{
		Type:        teleui.WebConfigAuthProviderOIDCType,
		WebAPIURL:   teleui.WebConfigAuthProviderOIDCURL,
		Name:        name,
		DisplayName: displayName,
	}
}

// NewSAMLAuthProvider creates AuthProvider of SAML type
func NewSAMLAuthProvider(name string, displayName string) teleui.WebConfigAuthProvider {
	return teleui.WebConfigAuthProvider{
		Type:        teleui.WebConfigAuthProviderSAMLType,
		WebAPIURL:   teleui.WebConfigAuthProviderSAMLURL,
		Name:        name,
		DisplayName: displayName,
	}
}

// NewGithubAuthProvider creates AuthProvider of Github type
func NewGithubAuthProvider(name string, displayName string) teleui.WebConfigAuthProvider {
	return teleui.WebConfigAuthProvider{
		Type:        teleui.WebConfigAuthProviderGitHubType,
		WebAPIURL:   teleui.WebConfigAuthProviderGitHubURL,
		Name:        name,
		DisplayName: displayName,
	}
}

// WebConfig contains various UI customizations (served as config.js)
type WebConfig struct {
	// SystemInfo contains system information
	SystemInfo struct {
		// Wizard indicates whether gravity has been launched in the wizard (aka standalone installer) mode
		Wizard bool `json:"wizard"`
		// ClusterName is the name of the local cluster (if running inside k8s)
		ClusterName string `json:"clusterName,omitempty"`
	} `json:"systemInfo"`
	// Auth customizes login screen
	Auth teleui.WebConfigAuthSettings `json:"auth"`
	// User specifies which user features are enabled
	User struct {
		// Login is configuration for the login screen
		Login struct {
			// HeaderText is the title above login form
			HeaderText string `json:"headerText,omitempty"`
		} `json:"login"`
		// Logo is the logo to display on the login screen
		Logo string `json:"logo,omitempty"`
	} `json:"user"`
	// Routes defines web app routes
	Routes struct {
		// DefaultEntry is the default web app entry point
		DefaultEntry string `json:"defaultEntry,omitempty"`
	} `json:"routes"`
	Modules struct {
		OpsCenter struct {
			Features struct {
				LicenseGenerator struct {
					Enabled bool `json:"enabled"`
				} `json:"licenseGenerator"`
			} `json:"features"`
		} `json:"opsCenter"`
	} `json:"modules"`
}
