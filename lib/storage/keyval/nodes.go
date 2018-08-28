package keyval

import (
	"github.com/gravitational/gravity/lib/storage"

	teleservices "github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
)

func getKind(nodeType string) string {
	switch nodeType {
	case storage.NodeTypeNode:
		return teleservices.KindNode
	case storage.NodeTypeProxy:
		return teleservices.KindProxy
	case storage.NodeTypeAuth:
		return teleservices.KindAuthServer
	}
	return ""
}

func (b *backend) getServers(nodeType string) ([]teleservices.Server, error) {
	ids, err := b.getKeys(b.key(nodesP, nodeType))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	marshaler := teleservices.GetServerMarshaler()
	var out []teleservices.Server
	for _, id := range ids {
		data, err := b.getValBytes(b.key(nodesP, nodeType, id))
		if err != nil {
			if trace.IsNotFound(err) {
				continue
			}
		}
		srv, err := marshaler.UnmarshalServer(data, getKind(nodeType))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out = append(out, srv)
	}
	return out, nil
}

func (b *backend) getNodes(namespace string) ([]teleservices.Server, error) {
	ids, err := b.getKeys(b.key(nodesP, storage.NodeTypeNode, namespace))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	marshaler := teleservices.GetServerMarshaler()
	var out []teleservices.Server
	for _, id := range ids {
		data, err := b.getValBytes(b.key(nodesP, storage.NodeTypeNode, namespace, id))
		if err != nil {
			if trace.IsNotFound(err) {
				continue
			}
		}
		srv, err := marshaler.UnmarshalServer(data, getKind(storage.NodeTypeNode))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out = append(out, srv)
	}
	return out, nil
}

func (b *backend) upsertNode(server teleservices.Server) error {
	data, err := teleservices.GetServerMarshaler().MarshalServer(server)
	if err != nil {
		return trace.Wrap(err)
	}
	err = b.upsertValBytes(b.key(nodesP, storage.NodeTypeNode, server.GetNamespace(), server.GetName()), data, b.ttl(server.Expiry()))
	return trace.Wrap(err)
}

func (b *backend) upsertServer(nodeType string, server teleservices.Server) error {
	data, err := teleservices.GetServerMarshaler().MarshalServer(server)
	if err != nil {
		return trace.Wrap(err)
	}
	err = b.upsertValBytes(b.key(nodesP, nodeType, server.GetName()), data, b.ttl(server.Expiry()))
	return trace.Wrap(err)
}

// GetNodes returns a list of registered servers
func (b *backend) GetNodes(namespace string) ([]teleservices.Server, error) {
	return b.getNodes(namespace)
}

// UpsertNode registers node presence, permanently if ttl is 0 or
// for the specified duration with second resolution if it's >= 1 second
func (b *backend) UpsertNode(server teleservices.Server) error {
	return b.upsertNode(server)
}

// GetAuthServers returns a list of registered servers
func (b *backend) GetAuthServers() ([]teleservices.Server, error) {
	return b.getServers(storage.NodeTypeAuth)
}

// UpsertAuthServer registers auth server presence, permanently if ttl is 0 or
// for the specified duration with second resolution if it's >= 1 second
func (b *backend) UpsertAuthServer(server teleservices.Server) error {
	return b.upsertServer(storage.NodeTypeAuth, server)
}

// UpsertProxy registers proxy server presence, permanently if ttl is 0 or
// for the specified duration with second resolution if it's >= 1 second
func (b *backend) UpsertProxy(server teleservices.Server) error {
	return b.upsertServer(storage.NodeTypeProxy, server)
}

// GetProxies returns a list of registered proxies
func (b *backend) GetProxies() ([]teleservices.Server, error) {
	return b.getServers(storage.NodeTypeProxy)
}

// UpsertReverseTunnel upserts reverse tunnel entry temporarily or permanently
func (b *backend) UpsertReverseTunnel(tunnel teleservices.ReverseTunnel) error {
	data, err := teleservices.GetReverseTunnelMarshaler().MarshalReverseTunnel(tunnel)
	if err != nil {
		return trace.Wrap(err)
	}
	err = b.upsertValBytes(b.key(tunnelsP, tunnel.GetClusterName()), data, b.ttl(tunnel.Expiry()))
	return trace.Wrap(err)
}

// GetReverseTunnels returns a list of registered servers
func (b *backend) GetReverseTunnels() ([]teleservices.ReverseTunnel, error) {
	clusterNames, err := b.getKeys(b.key(tunnelsP))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	marshaler := teleservices.GetReverseTunnelMarshaler()
	var out []teleservices.ReverseTunnel
	for _, clusterName := range clusterNames {
		data, err := b.getValBytes(b.key(tunnelsP, clusterName))
		if err != nil {
			if trace.IsNotFound(err) {
				continue
			}
		}
		tun, err := marshaler.UnmarshalReverseTunnel(data)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out = append(out, tun)
	}
	return out, nil
}

// DeleteReverseTunnel deletes reverse tunnel by cluster name
func (b *backend) DeleteReverseTunnel(clusterName string) error {
	err := b.deleteKey(b.key(tunnelsP, clusterName))
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("tunnel(%v) not found", clusterName)
		}
		return trace.Wrap(err)
	}
	return nil
}

// DeleteAllNodes deletes all nodes
func (b *backend) DeleteAllNodes(namespace string) error {
	err := b.deleteDir(b.key(nodesP, storage.NodeTypeProxy, namespace))
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}
	return nil
}

// DeleteAllProxies deletes all proxies
func (b *backend) DeleteAllProxies() error {
	err := b.deleteDir(b.key(nodesP, storage.NodeTypeProxy))
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}
	return nil
}

// DeleteAllReverseTunnels deletes all reverse tunnels
func (b *backend) DeleteAllReverseTunnels() error {
	err := b.deleteDir(b.key(tunnelsP))
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}
	return nil
}

// UpsertLocalClusterName upserts local domain
func (b *backend) UpsertLocalClusterName(name string) error {
	return b.upsertVal(b.key(localClusterP), name, forever)
}

// GetLocalClusterName return local cluster name
func (b *backend) GetLocalClusterName() (string, error) {
	var localCluster string
	err := b.getVal(b.key(localClusterP), &localCluster)
	return localCluster, err
}
