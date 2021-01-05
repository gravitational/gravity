package catalog

import (
	"context"

	"github.com/gravitational/gravity/e/lib/environment"
	"github.com/gravitational/gravity/e/lib/ops"
	"github.com/gravitational/gravity/lib/catalog"

	"github.com/gravitational/trace"
)

// newRemote returns application catalog for the Ops Center this cluster is
// connected to via a trusted cluster.
func newRemote() (catalog.Catalog, error) {
	localOperator, err := environment.ClusterOperator()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	localCluster, err := localOperator.GetLocalSite(context.TODO())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	trustedCluster, err := ops.GetTrustedCluster(localCluster.Key(), localOperator)
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("the cluster is not connected to a Gravity Hub")
		}
		return nil, trace.Wrap(err)
	}
	return catalog.NewRemoteFor(trustedCluster.GetName())
}

func init() {
	catalog.SetRemoteFunc(newRemote)
}
