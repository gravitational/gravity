// Copyright 2021 Gravitational Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package modules

import (
	"strings"

	"github.com/gravitational/gravity/e/lib/constants"
	ossconstants "github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/modules"
	"github.com/gravitational/gravity/lib/rpc/proto"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/version"
	"helm.sh/helm/v3/pkg/chartutil"
)

// init installs modules that provide custom behavior for the
// enterprise compared to the open-source version
func init() {
	modules.Set(&enterpriseModules{})
	modules.SetResources(&enterpriseResources{})
}

type enterpriseModules struct{}

// ProcessModes returns a list of modes gravity process can run in
func (m *enterpriseModules) ProcessModes() []string {
	return []string{
		ossconstants.ComponentSite,
		ossconstants.ComponentInstaller,
		constants.ComponentOpsCenter,
	}
}

// InstallModes returns a list of modes gravity install supports
func (m *enterpriseModules) InstallModes() []string {
	return []string{
		ossconstants.InstallModeInteractive,
		ossconstants.InstallModeCLI,
	}
}

// DefaultAuthPreference returns default auth preference based on run mode
func (m *enterpriseModules) DefaultAuthPreference(processMode string) (services.AuthPreference, error) {
	if processMode == constants.ComponentOpsCenter {
		return services.NewAuthPreference(
			services.AuthPreferenceSpecV2{
				Type:         teleport.OIDC,
				SecondFactor: teleport.OTP,
			})
	}
	return services.NewAuthPreference(
		services.AuthPreferenceSpecV2{
			Type:         teleport.Local,
			SecondFactor: teleport.OFF,
		})
}

// ProxyFeatures returns additional features Teleport proxy supports based on process mode
func (m *enterpriseModules) ProxyFeatures(processMode string) []string {
	if processMode == constants.ComponentOpsCenter {
		return []string{
			client.FeatureDocker,
			client.FeatureHelm,
		}
	}
	return nil
}

// SupportedConnectors returns a list of supported auth connector kinds
func (m *enterpriseModules) SupportedConnectors() []string {
	return []string{
		services.KindOIDCConnector,
		services.KindSAMLConnector,
		services.KindGithubConnector,
	}
}

// Version returns the gravity version
func (m *enterpriseModules) Version() proto.Version {
	ver := version.Get()
	return proto.Version{
		Edition:   "enterprise",
		Version:   ver.Version,
		GitCommit: ver.GitCommit,
		Helm:      chartutil.DefaultCapabilities.HelmVersion.Version,
	}
}

// TeleRepository returns the default repository for tele package cache
func (m *enterpriseModules) TeleRepository() string {
	return defaults.DistributionOpsCenter
}

type enterpriseResources struct{}

// SupportedResources returns a list of resources that can be created/viewed
func (*enterpriseResources) SupportedResources() []string {
	return SupportedResources
}

// SupportedResourcesToRemove returns a list of resources that can be removed
func (*enterpriseResources) SupportedResourcesToRemove() []string {
	return SupportedResourcesToRemove
}

// CanonicalKind translates the specified kind to canonical form.
// Returns an empty string if no canonical form exists
func (*enterpriseResources) CanonicalKind(kind string) string {
	return CanonicalKind(kind)
}

// CanonicalKind translates the specified kind to canonical form.
// Returns kind unmodified if no canonical form exists
func CanonicalKind(kind string) string {
	switch strings.ToLower(kind) {
	case services.KindRole, "roles":
		return services.KindRole
	case services.KindOIDCConnector:
		return services.KindOIDCConnector
	case services.KindSAMLConnector:
		return services.KindSAMLConnector
	case services.KindTrustedCluster, "trustedcluster", "trustedclusters":
		return services.KindTrustedCluster
	default:
		return storage.CanonicalKind(kind)
	}
}

var (
	// SupportedResources is a list of all OSS and enterprise resources
	// supported by "gravity resource create/get" subcommands
	SupportedResources = append(
		storage.SupportedGravityResources,
		services.KindRole,
		services.KindOIDCConnector,
		services.KindSAMLConnector,
		services.KindTrustedCluster,
		storage.KindEndpoints)

	// SupportedResourcesToRemove is a list of all OSS and enterprise
	// resources supported by "gravity resource rm" subcommand
	SupportedResourcesToRemove = append(
		storage.SupportedGravityResourcesToRemove,
		services.KindRole,
		services.KindOIDCConnector,
		services.KindSAMLConnector,
		services.KindTrustedCluster)
)
