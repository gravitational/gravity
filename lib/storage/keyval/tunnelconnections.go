package keyval

import (
	teleservices "github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
)

// UpsertTunnelConnection upserts tunnel connection
func (b *backend) UpsertTunnelConnection(conn teleservices.TunnelConnection) error {
	bytes, err := teleservices.MarshalTunnelConnection(conn)
	if err != nil {
		return trace.Wrap(err)
	}
	err = b.upsertValBytes(b.key(tunnelConnectionsP, conn.GetClusterName(),
		conn.GetName()), bytes, b.ttl(conn.Expiry()))
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetTunnelConnection returns a single tunnel connection
func (b *backend) GetTunnelConnection(clusterName, connName string) (teleservices.TunnelConnection, error) {
	bytes, err := b.getValBytes(b.key(tunnelConnectionsP, clusterName, connName))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("tunnel connection %v/%v not found",
				clusterName, connName)
		}
		return nil, trace.Wrap(err)
	}
	conn, err := teleservices.UnmarshalTunnelConnection(bytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return conn, nil
}

// GetTunnelConnections returns tunnel connections for a given cluster
func (b *backend) GetTunnelConnections(clusterName string) ([]teleservices.TunnelConnection, error) {
	names, err := b.getKeys(b.key(tunnelConnectionsP, clusterName))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var conns []teleservices.TunnelConnection
	for _, name := range names {
		conn, err := b.GetTunnelConnection(clusterName, name)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		conns = append(conns, conn)
	}
	return conns, nil
}

// GetAllTunnelConnections returns all tunnel connections
func (b *backend) GetAllTunnelConnections() ([]teleservices.TunnelConnection, error) {
	names, err := b.getKeys(b.key(tunnelConnectionsP))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var conns []teleservices.TunnelConnection
	for _, name := range names {
		clusterConns, err := b.GetTunnelConnections(name)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		conns = append(conns, clusterConns...)
	}
	return conns, nil
}

// DeleteTunnelConnection deletes tunnel connection by name
func (b *backend) DeleteTunnelConnection(clusterName string, connName string) error {
	err := b.deleteKey(b.key(tunnelConnectionsP, clusterName, connName))
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("tunnel connection %v/%v not found",
				clusterName, connName)
		}
		return trace.Wrap(err)
	}
	return nil
}

// DeleteTunnelConnections deletes all tunnel connections for cluster
func (b *backend) DeleteTunnelConnections(clusterName string) error {
	err := b.deleteDir(b.key(tunnelConnectionsP, clusterName))
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	return nil
}

// DeleteAllTunnelConnections deletes all tunnel connections
func (b *backend) DeleteAllTunnelConnections() error {
	err := b.deleteDir(b.key(tunnelConnectionsP))
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	return nil
}
