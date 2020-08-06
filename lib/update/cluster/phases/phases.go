/*
Copyright 2020 Gravitational, Inc.

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

package phases

import "github.com/gravitational/gravity/lib/ops"

// PackageRotator defines the subset of the operator to generate
// new configuration packages
type PackageRotator interface {
	// RotateSecrets generates a new secrets package for the specified request
	RotateSecrets(ops.RotateSecretsRequest) (*ops.RotatePackageResponse, error)
	// RotatePlanetConfig generates a new planet configuration package for the specified request
	RotatePlanetConfig(ops.RotatePlanetConfigRequest) (*ops.RotatePackageResponse, error)
	// RotateTeleportConfig generates new teleport configuration packages for the specified request
	RotateTeleportConfig(ops.RotateTeleportConfigRequest) (
		masterConfig, nodeConfig *ops.RotatePackageResponse,
		err error,
	)
}
