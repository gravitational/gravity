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

// This package implements compatibility layer to bridge previous provider/provisioner
// mismatch and as such is discouraged for future use.
package schema

import "github.com/gravitational/trace"

// IsAWSProvider determines if specified provider string refers to AWS provider
func IsAWSProvider(provider string) bool {
	switch provider {
	case ProviderAWS, ProvisionerAWSTerraform:
		return true
	default:
		return false
	}
}

// GetProviderFromProvisioner derives a provider name from the specified
// provisioner.
// It does not try to guess hard enough and supports only basic translation.
// Note, it is always cleaner to set the provider in the request explicitly.
func GetProviderFromProvisioner(provisioner string) (string, error) {
	switch provisioner {
	case ProvisionerAWSTerraform:
		return ProviderAWS, nil
	case ProvisionerOnPrem:
		return ProviderOnPrem, nil
	default:
		return "", trace.BadParameter("unknown provisioner %q", provisioner)
	}
}

// GetProvisionerFromProvider derives a provisioner name from the specified
// provider.
// It does not try to guess hard enough and supports only basic translation.
// Note, it is always cleaner to set the provisioner in the request explicitly.
func GetProvisionerFromProvider(provider string) (string, error) {
	switch provider {
	case ProviderAWS:
		return ProvisionerAWSTerraform, nil
	case ProviderOnPrem:
		return ProviderOnPrem, nil
	default:
		return "", trace.BadParameter("unknown provider %q", provider)
	}
}
