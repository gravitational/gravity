// +build !go1.8,go1.10

package inprocess

import "net"

// netPipe creates an in-process full-duplex network connection.
// It exists for compatibility with older versions of the standard
// library that did not provide a complete net.Conn implementation
// for in-process network pipe.
func netPipe() (net.Conn, net.Conn) {
	return net.Pipe()
}
