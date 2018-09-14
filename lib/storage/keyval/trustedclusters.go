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
