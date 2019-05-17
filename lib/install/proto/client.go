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
	"path/filepath"
	"time"

	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/state"

	"github.com/gogo/protobuf/types"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	grpc "google.golang.org/grpc"
)

// NewClient returns a new client using the specified state directory
// to look for socket file
func NewClient(ctx context.Context, socketPath string, logger log.FieldLogger, opts ...grpc.DialOption) (AgentClient, error) {
	type result struct {
		*grpc.ClientConn
		error
	}
	resultC := make(chan result, 1)
	go func() {
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
		conn, err := grpc.Dial("unix:///installer.sock", dialOptions...)
		resultC <- result{ClientConn: conn, error: err}
	}()
	for {
		select {
		case result := <-resultC:
			if result.error != nil {
				return nil, trace.Wrap(result.error)
			}
			client := NewAgentClient(result.ClientConn)
			return client, nil
		case <-ctx.Done():
			logger.WithError(ctx.Err()).Warn("Failed to connect.")
			return nil, trace.Wrap(ctx.Err())
		}
	}
}

// SocketPath returns the default path to the installer service socket
func SocketPath() (path string) {
	return filepath.Join(state.GravityInstallDir(), "installer.sock")
}

// KeyFromProto converts the specified operation key to internal format
func KeyFromProto(key *OperationKey) ops.SiteOperationKey {
	return ops.SiteOperationKey{
		AccountID:   key.AccountID,
		SiteDomain:  key.ClusterName,
		OperationID: key.ID,
	}
}

// KeyToProto converts the specified operation key to proto format
func KeyToProto(key ops.SiteOperationKey) *OperationKey {
	return &OperationKey{
		AccountID:   key.AccountID,
		ClusterName: key.SiteDomain,
		ID:          key.OperationID,
	}
}

// Empty defines the empty RPC message
var Empty = &types.Empty{}
