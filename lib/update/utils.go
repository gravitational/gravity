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

package update

import (
	appservice "github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// GetOperationPlan returns an up-to-date operation plan
func GetOperationPlan(b storage.Backend) (*storage.OperationPlan, error) {
	op, err := storage.GetLastOperation(b)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	plan, err := b.GetOperationPlan(op.SiteDomain, op.ID)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}

	if plan == nil {
		return nil, trace.NotFound(
			"%q does not have a plan, use 'gravity plan --init' to initialize it", op.Type)
	}

	changelog, err := b.GetOperationPlanChangelog(op.SiteDomain, op.ID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	plan = fsm.ResolvePlan(*plan, changelog)
	return plan, nil
}

// OverrideDockerConfig updates given config with values from overrideConfig where necessary
func OverrideDockerConfig(config *storage.DockerConfig, overrideConfig storage.DockerConfig) {
	if overrideConfig.StorageDriver != "" {
		config.StorageDriver = overrideConfig.StorageDriver
	}
	if len(overrideConfig.Args) != 0 {
		config.Args = overrideConfig.Args
	}
}

// DockerConfigFromSchema converts the specified Docker schema to storage configuration format
func DockerConfigFromSchema(dockerSchema *schema.Docker) (config storage.DockerConfig) {
	if dockerSchema == nil {
		return config
	}
	return DockerConfigFromSchemaValue(*dockerSchema)
}

// DockerConfigFromSchemaValue converts the specified Docker schema to storage configuration format
func DockerConfigFromSchemaValue(dockerSchema schema.Docker) (config storage.DockerConfig) {
	return storage.DockerConfig{
		StorageDriver: dockerSchema.StorageDriver,
		Args:          dockerSchema.Args,
	}
}

// planetNeedsUpdate returns true if the planet version in the update application is
// greater than in the installed one for the specified node profile
func planetNeedsUpdate(profile string, installed, update appservice.Application) (needsUpdate bool, err error) {
	installedProfile, err := installed.Manifest.NodeProfiles.ByName(profile)
	if err != nil {
		return false, trace.Wrap(err)
	}

	updateProfile, err := update.Manifest.NodeProfiles.ByName(profile)
	if err != nil {
		return false, trace.Wrap(err)
	}

	updateRuntimePackage, err := update.Manifest.RuntimePackage(*updateProfile)
	if err != nil {
		return false, trace.Wrap(err)
	}

	updateVersion, err := updateRuntimePackage.SemVer()
	if err != nil {
		return false, trace.Wrap(err)
	}

	runtimePackage, err := installed.Manifest.RuntimePackage(*installedProfile)
	if err != nil && !trace.IsNotFound(err) {
		return false, trace.Wrap(err)
	}
	if err != nil {
		runtimePackage, err = installed.Manifest.Dependencies.ByName(loc.LegacyPlanetMaster.Name)
		if err != nil {
			logrus.Warnf("Failed to fetch the runtime package: %v.", err)
			return false, trace.NotFound("runtime package not found")
		}
	}

	version, err := runtimePackage.SemVer()
	if err != nil {
		return false, trace.Wrap(err)
	}

	logrus.Debugf("Runtime installed: %v, runtime to update to: %v.", runtimePackage, updateRuntimePackage)
	updateNewer := updateVersion.Compare(*version) > 0
	return updateNewer, nil
}
