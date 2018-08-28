// This package implements compatibilty layer to bridge previous provider/provisioner
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
