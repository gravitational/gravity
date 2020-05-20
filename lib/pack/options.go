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

import (
	"github.com/gravitational/gravity/lib/storage"
)

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
