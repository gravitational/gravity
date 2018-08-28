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
