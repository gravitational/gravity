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
	// PurposeTeleportConfig marks packages with teleport config
	PurposeTeleportConfig = "teleport-config"
	// PurposeMetadata defines a label to use for application packages
	// that represent another package on a remote cluster.
	// A metadata package only contains a metadata block w/o actual contents of the
	// remote counterpart.
	PurposeMetadata = "metadata"
	// PurposeRPCCredentials marks a package as a package with agent RPC credentials
	PurposeRPCCredentials = "rpc-secrets"
)

// RuntimePackageLabels identifies the runtime package
var RuntimePackageLabels = map[string]string{
	PurposeLabel: PurposeRuntime,
}

// InstalledLabels defines a label set for an installed package
var InstalledLabels = map[string]string{
	InstalledLabel: InstalledLabel,
}
