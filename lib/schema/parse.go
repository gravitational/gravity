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

package schema

import (
	"bytes"
	"encoding/json"
	"io/ioutil"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	schemadefaults "github.com/gravitational/gravity/lib/schema/defaults"
	v1 "github.com/gravitational/gravity/lib/schema/v1"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/coreos/go-semver/semver"
	"github.com/ghodss/yaml"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// ParseManifestYAML parses the provided data as an application manifest
// and validates it
func ParseManifestYAML(data []byte) (*Manifest, error) {
	manifest, err := parseManifestYAML(data)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err = CheckAndSetDefaults(manifest); err != nil {
		return nil, trace.Wrap(err)
	}
	return manifest, nil
}

// ParseManifestYAMLNoValidate parses the provided data as an application manifest
// and does not run any validation checks on it
func ParseManifestYAMLNoValidate(bytesYAML []byte) (*Manifest, error) {
	manifest, err := parseManifestYAML(bytesYAML)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return manifest, nil
}

// ParseManifest parses manifest file at the specified path
func ParseManifest(path string) (*Manifest, error) {
	manifestBytes, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	manifest, err := ParseManifestYAMLNoValidate(manifestBytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return manifest, nil
}

func parseManifestYAML(data []byte) (*Manifest, error) {
	bytes, err := yaml.YAMLToJSON(data)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var manifest Manifest
	if err := json.Unmarshal(bytes, &manifest); err != nil {
		return nil, trace.Wrap(err)
	}
	return &manifest, nil
}

// CheckAndSetDefaults verifies that the specified manifest does not have conflicting
// or errorneous attributes set
func CheckAndSetDefaults(manifest *Manifest) error {
	var errors []error

	err := checkMetadata(manifest.Metadata)
	if err != nil {
		errors = append(errors, trace.Wrap(err))
	}

	// the rest of the checks apply only to user apps
	// TODO Do specific checks for Cluster VS Application
	switch manifest.Kind {
	case KindBundle, KindCluster, KindApplication:
	default:
		return trace.NewAggregate(errors...)
	}

	// make sure that if node profiles are defined, there's at least one flavor
	if len(manifest.NodeProfiles) > 0 && len(manifest.FlavorNames()) == 0 {
		errors = append(errors, trace.BadParameter(
			"at least one flavor is required when node profiles are defined"))
	}

	if manifest.Installer != nil {
		err = checkFlavors(manifest.Installer.Flavors, manifest.NodeProfiles)
		if err != nil {
			errors = append(errors, trace.Wrap(err))
		}
	}

	for _, profile := range manifest.NodeProfiles {
		err = checkProfile(profile)
		if err != nil {
			errors = append(errors, trace.Wrap(err))
		}

		err = checkDocker(manifest.Docker(profile))
		if err != nil {
			errors = append(errors, trace.Wrap(err))
		}
	}

	if manifest.WebConfig != "" {
		err = checkWebConfig(manifest.WebConfig)
		if err != nil {
			errors = append(errors, trace.Wrap(err))
		}
	}

	if manifest.Hooks != nil && (manifest.Hooks.ClusterProvision != nil ||
		manifest.Hooks.ClusterDeprovision != nil ||
		manifest.Hooks.NodesProvision != nil ||
		manifest.Hooks.NodesDeprovision != nil) {

		if manifest.Hooks.ClusterProvision == nil {
			errors = append(errors,
				trace.BadParameter("specify clusterProvision hook when using custom provisioning"))
		}
		if manifest.Hooks.ClusterDeprovision == nil {
			errors = append(errors,
				trace.BadParameter("specify clusterDeprovision hook when using custom provisioning"))
		}
		if manifest.Hooks.NodesProvision == nil {
			errors = append(errors,
				trace.BadParameter("specify nodesProvision hook when using custom provisioning"))
		}
		if manifest.Hooks.NodesDeprovision == nil {
			errors = append(errors,
				trace.BadParameter("specify nodesDeprovision hook when using custom provisioning"))
		}
	}

	for i, nodeProfile := range manifest.NodeProfiles {
		for j := range nodeProfile.Requirements.Volumes {
			if err := manifest.NodeProfiles[i].Requirements.Volumes[j].CheckAndSetDefaults(); err != nil {
				errors = append(errors, err)
			}
		}
	}

	if manifest.SystemOptions != nil {
		if manifest.SystemOptions.Runtime == nil {
			errors = append(errors, trace.NotFound("no runtime application defined"))
		}
	}

	if len(errors) > 0 {
		return trace.NewAggregate(errors...)
	}

	return SetDefaults(manifest)
}

// SetDefaults enforces defaults on fields that require a value
// but have not been specified on the given manifest.
func SetDefaults(manifest *Manifest) error {
	setProviderDefaults(manifest)

	// if there are no profiles/flavors defined, inject default ones
	if len(manifest.NodeProfiles) == 0 && len(manifest.FlavorNames()) == 0 {
		manifest.NodeProfiles = NodeProfiles{defaultNodeProfile}
		if manifest.Installer == nil {
			manifest.Installer = &Installer{}
		}
		manifest.Installer.Flavors.Items = []Flavor{defaultFlavor}
	}

	for i := range manifest.NodeProfiles {
		if manifest.NodeProfiles[i].Labels[constants.NodeLabel] == constants.True {
			manifest.NodeProfiles[i].ServiceRole = ServiceRoleNode
		} else if manifest.NodeProfiles[i].Labels[constants.MasterLabel] == constants.True {
			manifest.NodeProfiles[i].ServiceRole = ServiceRoleMaster
		}
	}
	return nil
}

func setProviderDefaults(manifest *Manifest) {
	if manifest.Providers == nil {
		manifest.Providers = &Providers{}
	}
	if manifest.Providers.AWS.Networking.Type == "" {
		manifest.Providers.AWS.Networking.Type = NetworkingAWSVPC
	}
	if manifest.Providers.Generic.Networking.Type == "" {
		manifest.Providers.Generic.Networking.Type = NetworkingFlannel
	}
}

// checkMetadata performs some sanity checks on manifest metadata
func checkMetadata(metadata Metadata) error {
	var errors []error

	// make sure the app name is acceptable
	err := utils.CheckName(metadata.Name)
	if err != nil {
		errors = append(errors, trace.Wrap(err))
	}

	// make sure the version is correct semver
	_, err = semver.NewVersion(metadata.ResourceVersion)
	if err != nil {
		errors = append(errors, trace.Wrap(
			err, "app version must be in semver format, got %q", metadata.ResourceVersion))
	}

	// repository must be set to gravitational.io, otherwise things don't work
	if metadata.Repository != defaults.SystemAccountOrg {
		errors = append(errors, trace.BadParameter(
			"repository must be equal to %q", defaults.SystemAccountOrg))
	}

	return trace.NewAggregate(errors...)
}

// checkFlavors performs some sanity checks on flavors
func checkFlavors(flavors Flavors, profiles NodeProfiles) error {
	var errors []error

	// make sure the flavors refer to correct profile names and the count is correct
	for _, flavor := range flavors.Items {
		for _, node := range flavor.Nodes {
			_, err := profiles.ByName(node.Profile)
			if err != nil {
				errors = append(errors, trace.BadParameter(
					"flavor %q refers to undefined profile %q", flavor.Name, node.Profile))
			}
			if node.Count < 0 {
				errors = append(errors, trace.BadParameter(
					"flavor %q has negative count for profile %q", flavor.Name, node.Profile))
			}
		}
	}

	return trace.NewAggregate(errors...)
}

// checkProfile performs some sanity checks on node profile
func checkProfile(profile NodeProfile) error {
	var errors []error

	err := checkRequirements(profile.Requirements)
	if err != nil {
		errors = append(errors, trace.Wrap(err))
	}

	if profile.ExpandPolicy != "" {
		if profile.ExpandPolicy != ExpandPolicyFixed && profile.ExpandPolicy != ExpandPolicyFixedInstance {
			errors = append(errors, trace.BadParameter("supported expand policies are %q and %q, got: %q",
				ExpandPolicyFixed, ExpandPolicyFixedInstance, profile.ExpandPolicy))
		}
	}

	return trace.NewAggregate(errors...)
}

// checkRequirements performs some sanity checks on node requirements
func checkRequirements(reqs Requirements) error {
	var errors []error

	if reqs.CPU.Max != 0 && reqs.CPU.Max < reqs.CPU.Min {
		errors = append(errors, trace.BadParameter(
			"max CPU (%v) is less than min CPU (%v)", reqs.CPU.Max, reqs.CPU.Min))
	}

	if reqs.RAM.Max != 0 && reqs.RAM.Max.Bytes() < reqs.RAM.Min.Bytes() {
		errors = append(errors, trace.BadParameter(
			"max RAM (%v) is less than min RAM (%v)", reqs.RAM.Max, reqs.RAM.Min))
	}

	for _, device := range reqs.Devices {
		errors = append(errors, device.Check())
	}

	return trace.NewAggregate(errors...)
}

// checkWebConfig makes sure that the provided web config is a valid JSON
func checkWebConfig(webConfig string) error {
	o := make(map[string]interface{})
	err := json.Unmarshal([]byte(webConfig), &o)
	if err != nil {
		return trace.BadParameter("webConfig is not a valid JSON")
	}
	return nil
}

// checkDocker makes sure that the provided docker configuration is correct
func checkDocker(docker Docker) error {
	if !utils.StringInSlice(constants.DockerSupportedDrivers, docker.StorageDriver) {
		return trace.BadParameter("unrecognized docker storage driver %q, supported are: %v",
			docker.StorageDriver, constants.DockerSupportedDrivers)
	}
	return nil
}

// UnmarshalJSON implements encoding/json#Unmarshaler
func (m *Manifest) UnmarshalJSON(data []byte) error {
	var header Header
	if err := json.Unmarshal(data, &header); err != nil {
		return trace.Wrap(err, "failed to unmarshal manifest header")
	}

	switch header.APIVersion {
	case APIVersionV2, APIVersionV2Cluster, APIVersionV2App:
		if err := schema.Validate(bytes.NewReader(data)); err != nil {
			log.WithError(err).Warn("Failed to validate manifest against schema.")
			return trace.Wrap(err, "failed to validate manifest")
		}

		// Use type alias to avoid infinite recursion during unmarshaling
		type serializableManifest Manifest

		var manifest serializableManifest
		if err := json.Unmarshal(data, &manifest); err != nil {
			log.Warnf("Failed to unmarshal manifest: %v.", err)
			return trace.Wrap(err, "failed to unmarshal manifest")
		}

		if err := schemadefaults.Apply(&manifest, schema); err != nil {
			return trace.Wrap(err, "failed to set manifest defaults")
		}

		*m = Manifest(manifest)
	case APIVersionV1:
		manifestV1, err := v1.UnmarshalJSON(data)
		if err != nil {
			return trace.Wrap(err, "failed to parse v1 manifest")
		}

		converted, err := convertV1ToV2(*manifestV1)
		if err != nil {
			return trace.Wrap(err, "failed to convert manifest")
		}
		*m = *converted
	default:
		return trace.BadParameter("unknown manifest API version: %v", header.APIVersion)
	}

	return nil
}

// MustParseManifestYAML parser the provided manifest data and panics in case of parsing error
func MustParseManifestYAML(data []byte) Manifest {
	manifest, err := ParseManifestYAMLNoValidate(data)
	if err != nil {
		panic(err)
	}
	return *manifest
}

var (
	// defaultNodeProfile gets injected into manifest if it's missing both node profiles
	// and flavors
	defaultNodeProfile = NodeProfile{
		Name:        "node",
		Description: "Default node",
	}

	// defaultFlavor gets injected into manifest if it's missing both node profiles and
	// flavors
	defaultFlavor = Flavor{
		Name: "default",
		Nodes: []FlavorNode{
			{
				Profile: "node",
				Count:   1,
			},
		},
	}
)
