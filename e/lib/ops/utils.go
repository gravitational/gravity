package ops

import (
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/trace"
)

// GetTrustedCluster returns a trusted cluster representing the Ops Center
// the specified site is connected to, currently only 1 is supported
func GetTrustedCluster(key ops.SiteKey, operator Operator) (storage.TrustedCluster, error) {
	clusters, err := operator.GetTrustedClusters(key)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, cluster := range clusters {
		if !cluster.GetWizard() {
			return cluster, nil
		}
	}
	return nil, trace.NotFound("trusted cluster for %v not found", key)
}

// GetWizardTrustedCluster returns a trusted cluster representing the wizard
// Ops Center the specified site is connected to
func GetWizardTrustedCluster(key ops.SiteKey, operator Operator) (storage.TrustedCluster, error) {
	clusters, err := operator.GetTrustedClusters(key)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, cluster := range clusters {
		if cluster.GetWizard() {
			return cluster, nil
		}
	}
	return nil, trace.NotFound("wizard trusted cluster for %v not found", key)
}
