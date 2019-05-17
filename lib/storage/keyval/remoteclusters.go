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
	"encoding/json"

	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
)

// CreateRemoteCluster saves the provided RemoteCluster resource
func (b *backend) CreateRemoteCluster(rc services.RemoteCluster) error {
	data, err := json.Marshal(rc)
	if err != nil {
		return trace.Wrap(err)
	}
	ttl := b.ttl(rc.Expiry())
	err = b.createValBytes(b.key(remoteClustersP, rc.GetName()), []byte(data), ttl)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetRemoteClusters returns a list of remote clusters
func (b *backend) GetRemoteClusters(opts ...services.MarshalOption) ([]services.RemoteCluster, error) {
	keys, err := b.getKeys(b.key(remoteClustersP))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clusters := make([]services.RemoteCluster, 0, len(keys))
	for _, key := range keys {
		data, err := b.getValBytes(b.key(remoteClustersP, key))
		if err != nil {
			if trace.IsNotFound(err) {
				continue
			}
			return nil, trace.Wrap(err)
		}
		cluster, err := services.UnmarshalRemoteCluster(data)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		clusters = append(clusters, cluster)
	}
	return clusters, nil
}

// GetRemoteCluster returns a remote cluster by name
func (b *backend) GetRemoteCluster(clusterName string) (services.RemoteCluster, error) {
	data, err := b.getValBytes(b.key(remoteClustersP, clusterName))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("remote cluster %q is not found", clusterName)
		}
		return nil, trace.Wrap(err)
	}
	return services.UnmarshalRemoteCluster(data)
}

// DeleteRemoteCluster deletes remote cluster by name
func (b *backend) DeleteRemoteCluster(clusterName string) error {
	return b.deleteKey(b.key(remoteClustersP, clusterName))

}

// DeleteAllRemoteClusters deletes all remote clusters
func (b *backend) DeleteAllRemoteClusters() error {
	err := b.deleteDir(b.key(remoteClustersP))
	if trace.IsNotFound(err) {
		return nil
	}
	return trace.Wrap(err)
}
