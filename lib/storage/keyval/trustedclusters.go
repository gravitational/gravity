package keyval

import (
	teleservices "github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
)

func (b *backend) UpsertTrustedCluster(cluster teleservices.TrustedCluster) error {
	bytes, err := teleservices.GetTrustedClusterMarshaler().Marshal(cluster)
	if err != nil {
		return trace.Wrap(err)
	}
	err = b.upsertValBytes(b.key(trustedClustersP, cluster.GetName()),
		bytes, b.ttl(cluster.Expiry()))
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (b *backend) GetTrustedCluster(name string) (teleservices.TrustedCluster, error) {
	bytes, err := b.getValBytes(b.key(trustedClustersP, name))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("trusted cluster %q not found", name)
		}
		return nil, trace.Wrap(err)
	}
	cluster, err := teleservices.GetTrustedClusterMarshaler().Unmarshal(bytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return cluster, nil
}

func (b *backend) GetTrustedClusters() ([]teleservices.TrustedCluster, error) {
	names, err := b.getKeys(b.key(trustedClustersP))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var clusters []teleservices.TrustedCluster
	for _, name := range names {
		cluster, err := b.GetTrustedCluster(name)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		clusters = append(clusters, cluster)
	}
	return clusters, nil
}

func (b *backend) DeleteTrustedCluster(name string) error {
	err := b.deleteKey(b.key(trustedClustersP, name))
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("trusted cluster %q not found", name)
		}
		return trace.Wrap(err)
	}
	return nil
}
