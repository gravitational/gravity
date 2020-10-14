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
func NewClient(ctx context.Context, config ClientConfig) (AgentClient, error) {
	if err := config.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	dialOptions := []grpc.DialOption{
		// Don't use TLS, as we communicate over domain sockets
		grpc.WithInsecure(),
		// Retry every second after failure
		grpc.WithBackoffMaxDelay(1 * time.Second),
		grpc.WithBlock(),
		grpc.FailOnNonTempDialError(true),
		grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
			if err := config.ShouldReconnectService(); err != nil {
				// Fast path: service has failed or has been uninstalled
				// Note, the error is not trace.Wrapped on purpose - it needs
				// to retain the fact that it's terminal
				return nil, serviceError(trace.UserMessage(err))
			}
			conn, err := (&net.Dialer{}).DialContext(ctx, "unix", config.SocketPath)
			config.WithFields(log.Fields{
				log.ErrorKey: err,
				"addr":       config.SocketPath,
			}).Debug("Connect to installer service.")
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return conn, nil
		}),
	}
	dialOptions = append(dialOptions, config.DialOptions...)
	conn, err := grpc.DialContext(ctx, "unix:///installer.sock", dialOptions...)
	if err != nil {
		if wrappedErr, ok := trace.Unwrap(err).(wrappedError); ok {
			return nil, trace.Wrap(wrappedErr.Origin())
		}
		return nil, trace.Wrap(err)
	}
	client := NewAgentClient(conn)
	return client, nil
}

func (r *ClientConfig) checkAndSetDefaults() error {
	if r.SocketPath == "" {
		return trace.BadParameter("socket path is required")
	}
	if r.FieldLogger == nil {
		r.FieldLogger = log.WithField(trace.Component, "proto:client")
	}
	if r.ShouldReconnectService == nil {
		// Reconnect always by default
		r.ShouldReconnectService = func() error { return nil }
	}
	return nil
}

// ClientConfig describes client configuration
type ClientConfig struct {
	// FieldLogger specifies the logger
	log.FieldLogger
	// SocketPath specifies the path to the service's socket file
	SocketPath string
	// ShouldReconnectService determines if the service should be reconnected.
	// If this returns an error, the internal dialer will cancel all attempts to reconnect
	ShouldReconnectService func() error
	// DialOptions specifies additional gRPC dial options
	DialOptions []grpc.DialOption
}

// SocketPath returns the default path to the installer service socket
func SocketPath() (path string) {
	return state.GravityInstallDir("installer.sock")
}

// Temporary returns false for this error.
// This indicates to the gRPC dialing logic that the error is not retryable
func (serviceError) Temporary() bool {
	return false
}

// Error returns the error text
func (r serviceError) Error() string {
	return string(r)
}

type serviceError string

// wrappedError models the functionality of an gRPC error that wraps another error.
// It exists as gRPC package does not expose the interface itself
type wrappedError interface {
	// Origin returns the original error
	Origin() error
}
