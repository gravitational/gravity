/*
Copyright 2019 Gravitational, Inc.

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
package installer

import (
	"context"
	"net"
	"time"

	"github.com/gravitational/gravity/lib/state"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	grpc "google.golang.org/grpc"
)

// NewClient returns a new client using the specified socket file path
func NewClient(ctx context.Context, socketPath string, logger log.FieldLogger, opts ...grpc.DialOption) (AgentClient, error) {
	dialOptions := []grpc.DialOption{
		// Don't use TLS, as we communicate over domain sockets
		grpc.WithInsecure(),
		// Retry every second after failure
		grpc.WithBackoffMaxDelay(1 * time.Second),
		grpc.WithBlock(),
		grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
			conn, err := (&net.Dialer{}).DialContext(ctx, "unix", socketPath)
			logger.WithFields(log.Fields{
				log.ErrorKey: err,
				"addr":       socketPath,
			}).Debug("Connect to installer service.")
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return conn, nil
		}),
	}
	dialOptions = append(dialOptions, opts...)
	conn, err := grpc.DialContext(ctx, "unix:///installer.sock", dialOptions...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	client := NewAgentClient(conn)
	return client, nil
}

// SocketPath returns the default path to the installer service socket
func SocketPath() (path string, err error) {
	return state.GravityInstallDir("installer.sock")
}
