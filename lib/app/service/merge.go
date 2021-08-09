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

package service

import (
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
)

// mergeManifests combines the manifest of the specified application with
// the base manifest and augments the application manifest so that it contains
// all the details of a complete manifest.
func mergeManifests(target *schema.Manifest, source schema.Manifest) error {
	mergeDependencies(&target.Dependencies, source.Dependencies)
	if err := mergeRuntime(target, source); err != nil {
		return trace.Wrap(err)
	}
	target.Endpoints = mergeEndpoints(target.Endpoints, source.Endpoints)

	if source.Installer != nil {
		if target.Installer == nil {
			target.Installer = &schema.Installer{}
		}
		mergeInstaller(target.Installer, source.Installer)
	}

	if source.Providers != nil {
		if target.Providers == nil {
			target.Providers = &schema.Providers{}
		}
		mergeProviders(target.Providers, *source.Providers)
	}

	return nil
}

func mergeRuntime(target *schema.Manifest, source schema.Manifest) error {
	if target.SystemOptions == nil {
		target.SystemOptions = &schema.SystemOptions{}
	}

	if source.SystemOptions == nil || source.SystemOptions.Dependencies.Runtime == nil {
		_, err := source.Dependencies.ByName(loc.LegacyPlanetMaster.Name,
			loc.LegacyPlanetNode.Name)
		if err != nil {
			return trace.NotFound("no runtime package specified in manifest for %s", source.Locator())
		}
		// TODO: ideally now, that the manifest requires a runtime package, the version of
		// the manifest schema needs to be bumped to account for this.
		// Then, it would be natural to convert one to the other.
		return nil
	}

	sourceRuntime := source.SystemOptions.Dependencies.Runtime
	if target.SystemOptions.Dependencies.Runtime == nil {
		target.SystemOptions.Dependencies.Runtime = sourceRuntime
	}

	return nil
}

func mergeInstaller(target *schema.Installer, source *schema.Installer) {
	if len(target.SetupEndpoints) == 0 {
		target.SetupEndpoints = append(target.SetupEndpoints, source.SetupEndpoints...)
	}
}

// mergeEndpoints merges the Endpoints of two manifests. Any Endpoints with the same
// name are overwritten by the values in the target.
func mergeEndpoints(target []schema.Endpoint, source []schema.Endpoint) []schema.Endpoint {
	merged := make(map[string]schema.Endpoint)

	for _, sourceEndpoint := range source {
		merged[sourceEndpoint.Name] = sourceEndpoint
	}

	for _, targetEndpoint := range target {
		merged[targetEndpoint.Name] = targetEndpoint
	}

	newTarget := make([]schema.Endpoint, 0, len(merged))
	for _, v := range merged {
		newTarget = append(newTarget, v)
	}

	return newTarget
}

func mergeDependencies(target *schema.Dependencies, source schema.Dependencies) {
	target.Packages = append(source.Packages, target.Packages...)
	target.Apps = append(source.Apps, target.Apps...)
}

func mergeProviders(target *schema.Providers, source schema.Providers) {
	mergeAWS(&target.AWS, source.AWS)
	mergeGeneric(&target.Generic, source.Generic)
}

func mergeAWS(target *schema.AWS, source schema.AWS) {
	mergeNetworking(&target.Networking, source.Networking)
	mergeIAMPolicy(&target.IAMPolicy, source.IAMPolicy)
}

func mergeGeneric(target *schema.Generic, source schema.Generic) {
	mergeNetworking(&target.Networking, source.Networking)
}

func mergeNetworking(target *schema.Networking, source schema.Networking) {
	if target.Type == "" {
		target.Type = source.Type
	}
}

func mergeIAMPolicy(target *schema.IAMPolicy, source schema.IAMPolicy) {
	if target.Version == "" {
		target.Version = source.Version
	}

	actions := utils.NewStringSetFromSlice(target.Actions)
	actions.AddSlice(source.Actions)
	target.Actions = actions.Slice()
}
