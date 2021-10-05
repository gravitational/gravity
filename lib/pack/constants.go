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

package pack

const (
	// InstalledLabel is used to mark installed packages
	InstalledLabel = "installed"
	// LatestLabel is a pseudo label that allows system to find the latest version
	LatestLabel = "latest"
	// ConfigLabel means that this is a configuration package for another package
	ConfigLabel = "config-package-for"
	// PurposeLabel describes the package purpose
	PurposeLabel = "purpose"
	// AdvertiseIPLabel contains advertise IP of the server the package is for
	AdvertiseIPLabel = "advertise-ip"
	// OperationIDLabel contains ID of the operation the package was configured for
	OperationIDLabel = "operation-id"

	// PurposeCA marks the planet certificate authority package
	PurposeCA = "ca"
	// PurposeExport marks the package with cluster export data
	PurposeExport = "export"
	// PurposeLicense marks the package with cluster license
	PurposeLicense = "license"
	// PurposeResources marks the package with user resources
	PurposeResources = "resources"
	// PurposePlanetSecrets marks packages with planet secrets
	PurposePlanetSecrets = "planet-secrets"
	// PurposePlanetConfig marks packages with planet config
	PurposePlanetConfig = "planet-config"
	// PurposeRuntime marks a package as a runtime container package
	PurposeRuntime = "runtime"
	// PurposeTeleportMasterConfig marks package with teleport master config
	PurposeTeleportMasterConfig = "teleport-master-config"
	// PurposeTeleportNodeConfig marks package with teleport node config
	PurposeTeleportNodeConfig = "teleport-node-config"
	// PurposeLegacyTeleportNodeConfig specifies how the teleport node configuration packages
	// were marked in previous version
	PurposeLegacyTeleportNodeConfig = "teleport-config"
	// PurposeMetadata defines a label to use for application packages
	// that represent another package on a remote cluster.
	// A metadata package only contains a metadata block w/o actual contents of the
	// remote counterpart.
	PurposeMetadata = "metadata"
	// PurposeRPCCredentials marks a package as a package with agent RPC credentials
	PurposeRPCCredentials = "rpc-secrets"
)

var (
	// RuntimePackageLabels identifies the runtime package
	RuntimePackageLabels = map[string]string{
		PurposeLabel: PurposeRuntime,
	}

	// RuntimeSecretsPackageLabels identifies the runtime secrets package
	RuntimeSecretsPackageLabels = map[string]string{
		PurposeLabel: PurposePlanetSecrets,
	}

	// RuntimeConfigPackageLabels identifies the runtime configuration package
	RuntimeConfigPackageLabels = map[string]string{
		PurposeLabel: PurposePlanetConfig,
	}

	// TeleportNodeConfigPackageLabels identifies the teleport node configuration package
	TeleportNodeConfigPackageLabels = map[string]string{
		PurposeLabel: PurposeTeleportNodeConfig,
	}

	// TeleportLegacyNodeConfigPackageLabels identifies the teleport node configuration package in previous versions.
	// TODO(dmitri): remove when no longer supported
	TeleportLegacyNodeConfigPackageLabels = map[string]string{
		PurposeLabel: PurposeLegacyTeleportNodeConfig,
	}

	// TeleportMasterConfigPackageLabels identifies the teleport master configuration package
	TeleportMasterConfigPackageLabels = map[string]string{
		PurposeLabel: PurposeTeleportMasterConfig,
	}

	// InstalledLabels defines a label set for an installed package
	InstalledLabels = map[string]string{
		InstalledLabel: InstalledLabel,
	}
)
