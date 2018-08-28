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
