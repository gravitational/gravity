package pack

import "github.com/gravitational/gravity/lib/storage"

// WithLabels adds the specified labels as runtime labels to a package
func WithLabels(labels map[string]string) PackageOption {
	return func(pkg *storage.Package) {
		pkg.RuntimeLabels = labels
	}
}

// WithHidden configures a hidden flag for a package
func WithHidden(hidden bool) PackageOption {
	return func(pkg *storage.Package) {
		pkg.Hidden = hidden
	}
}

// WithEncrypted configures the encryption status of a package
func WithEncrypted(encrypted bool) PackageOption {
	return func(pkg *storage.Package) {
		pkg.Encrypted = encrypted
	}
}

// WithManifest configures application-specific attributes like type and manifest for a package
func WithManifest(packageType string, manifest []byte) PackageOption {
	return func(pkg *storage.Package) {
		pkg.Type = packageType
		pkg.Manifest = manifest
	}
}

// WithCreatedBy configures the package creator
func WithCreatedBy(createdBy string) PackageOption {
	return func(pkg *storage.Package) {
		pkg.CreatedBy = createdBy
	}
}
