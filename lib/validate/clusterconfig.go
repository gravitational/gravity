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

package validate

import (
	"net"
	"strings"

	"github.com/gravitational/gravity/lib/storage/clusterconfig"

	"github.com/gravitational/trace"
)

// ClusterConfiguration validates that `update` can update `existing` without invalidating consistency.
func ClusterConfiguration(existing, update clusterconfig.Interface) error {
	if newGlobalConfig := update.GetGlobalConfig(); !isCloudConfigEmpty(newGlobalConfig) {
		// TODO(dmitri): require cloud provider if cloud-config is being updated
		// This is more a sanity check than a hard requirement so users are explicit about changes
		// in the cloud configuration
		if newGlobalConfig.CloudConfig != "" && newGlobalConfig.CloudProvider == "" {
			return trace.BadParameter("cloud provider is required when updating cloud configuration")
		}
	}
	newGlobalConfig := update.GetGlobalConfig()
	if newGlobalConfig.IsEmpty() {
		return trace.BadParameter("provided cluster configuration is empty")
	}
	globalConfig := existing.GetGlobalConfig()
	if isCloudConfigEmpty(globalConfig) {
		if newGlobalConfig := update.GetGlobalConfig(); !isCloudConfigEmpty(newGlobalConfig) {
			return trace.BadParameter("cannot change cloud configuration: cluster does not have cloud provider configured")
		}
	}
	if newGlobalConfig.CloudProvider != "" && globalConfig.CloudProvider != newGlobalConfig.CloudProvider {
		return trace.BadParameter("changing cloud provider is not supported (%q -> %q)",
			newGlobalConfig.CloudProvider, globalConfig.CloudProvider)
	}
	if globalConfig.CloudProvider == "" && newGlobalConfig.CloudConfig != "" {
		return trace.BadParameter("cannot set cloud configuration: cluster does not have cloud provider configured")
	}

	if err := validateAdmissionPlugins(newGlobalConfig.AdmissionPlugins); err != nil {
		return trace.Wrap(err)
	}

	podCIDRString := globalConfig.PodCIDR
	serviceCIDRString := globalConfig.ServiceCIDR
	podSubnetSizeString := globalConfig.PodSubnetSize

	if newGlobalConfig.PodCIDR != "" {
		_, podCIDR, err := net.ParseCIDR(newGlobalConfig.PodCIDR)
		if err != nil {
			return trace.Wrap(err, "invalid pod subnet: %v", newGlobalConfig.PodCIDR)
		}
		if podCIDR.String() == globalConfig.PodCIDR {
			return trace.BadParameter("specified pod subnet (%v) is the same as existing pod subnet",
				newGlobalConfig.PodCIDR)
		}
		podCIDRString = newGlobalConfig.PodCIDR
	}

	if newGlobalConfig.ServiceCIDR != "" {
		_, serviceCIDR, err := net.ParseCIDR(newGlobalConfig.ServiceCIDR)
		if err != nil {
			return trace.Wrap(err, "invalid service subnet: %v", newGlobalConfig.ServiceCIDR)
		}
		if serviceCIDR.String() == globalConfig.ServiceCIDR {
			return trace.BadParameter("specified service subnet (%v) is the same as existing service subnet",
				newGlobalConfig.ServiceCIDR)
		}
		serviceCIDRString = newGlobalConfig.ServiceCIDR
	}

	if newGlobalConfig.PodSubnetSize != "" {
		podSubnetSizeString = newGlobalConfig.PodSubnetSize
	}

	if err := KubernetesSubnetsFromStrings(podCIDRString, serviceCIDRString, podSubnetSizeString); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func isCloudConfigEmpty(global clusterconfig.Global) bool {
	return global.CloudProvider == "" && global.CloudConfig == ""
}

// validateAdmissionPlugins verifies that the provided Kubernetes admission
// plugins are valid.
func validateAdmissionPlugins(plugins []string) error {
	// https://kubernetes.io/docs/reference/command-line-tools-reference/kube-apiserver/#options
	// List of available Kubernetes admission plugins can be found under
	// --enable-admission-plugins.
	validPlugins := []string{
		"ImagePolicyWebhook",
	}

	searchPlugins := make(map[string]struct{})
	for _, validPlugin := range validPlugins {
		searchPlugins[validPlugin] = struct{}{}
	}

	for _, plugin := range plugins {
		if _, exists := searchPlugins[plugin]; !exists {
			return trace.BadParameter("%s is not a valid plugin. Current valid plugins include: \n\t%v",
				plugin, strings.Join(validPlugins, "\n\t"))
		}
	}

	return nil
}
