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

package opsservice

import (
	"context"
	"net"
	"time"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/utils"
	teleservices "github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"

	log "github.com/sirupsen/logrus"
)

func (s *site) validateRemoteAccess(req ops.ValidateRemoteAccessRequest) (resp *ops.ValidateRemoteAccessResponse, err error) {
	servers, err := s.getTeleportServersWithTimeout(
		req.NodeLabels,
		defaults.TeleportServerQueryTimeout,
		defaults.RetryInterval,
		defaults.RetryLessAttempts,
		queryReturnsAtLeastOneServer)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	runner := &teleportRunner{
		FieldLogger:          log.WithField(trace.Component, "teleport-runner"),
		domainName:           s.domainName,
		TeleportProxyService: s.teleport(),
	}
	var results []ops.NodeResponse
	for _, node := range servers {
		server, err := newTeleportServer(node)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		out, err := runner.Run(server, defaults.ValidateCommand)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		results = append(results, ops.NodeResponse{Output: out, Name: node.GetName()})
	}

	return &ops.ValidateRemoteAccessResponse{
		Results: results,
	}, nil
}

// teleportServer captures a subset of attributes of the remote server
// managed by an instance of teleport
type teleportServer struct {
	// Addr specifies the remote address as host:port
	Addr string
	// IP specifies just the IP of the server
	IP string
	// Hostname specifies the remote server's hostname
	Hostname string
	// Labels lists the set of both static and dynamic node labels
	Labels map[string]string
}

func newTeleportServer(server teleservices.Server) (*teleportServer, error) {
	serverIP, _, err := net.SplitHostPort(server.GetAddr())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &teleportServer{
		Addr:     server.GetAddr(),
		IP:       serverIP,
		Hostname: server.GetHostname(),
		Labels:   server.GetAllLabels(),
	}, nil
}

// Address returns the address this server is accessible on
// Address implements remoteServer.Address
func (t *teleportServer) Address() string { return t.Addr }

// HostName returns the hostname of this server.
// HostName implements remoteServer.HostName
func (t *teleportServer) HostName() string { return t.Labels[ops.Hostname] }

// Debug provides a reference to the specified server useful for logging
// Debug implements remoteServer.Debug
func (t *teleportServer) Debug() string { return t.Addr }

func (s *site) getTeleportServerNoRetry(labelName, labelValue string) (server *teleportServer, err error) {
	const noRetry = 1
	labels := map[string]string{labelName: labelValue}
	servers, err := s.getTeleportServersWithTimeout(
		labels,
		defaults.TeleportServerQueryTimeout,
		defaults.RetryInterval,
		noRetry,
		queryReturnsAtLeastOneServer)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return newTeleportServer(servers[0])
}

// getAllTeleportServers queries all teleport servers in a retry loop
func (s *site) getAllTeleportServers() (teleservers, error) {
	anyServers := func(string, []teleservices.Server) error {
		return nil
	}
	return s.getTeleportServersWithTimeout(nil, defaults.TeleportServerQueryTimeout,
		defaults.RetryInterval, defaults.RetryLessAttempts, anyServers)
}

// getTeleportServers returns all servers matching provided label
func (s *site) getTeleportServers(labelName, labelValue string) (result []teleportServer, err error) {
	servers, err := s.getTeleportServersWithTimeout(
		map[string]string{labelName: labelValue},
		defaults.TeleportServerQueryTimeout,
		defaults.RetryInterval,
		defaults.RetryLessAttempts,
		queryReturnsAtLeastOneServer)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, server := range servers {
		teleportServer, err := newTeleportServer(server)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		result = append(result, *teleportServer)
	}
	return result, nil
}

// getTeleportServer queries the teleport server with the specified label in a retry loop
func (s *site) getTeleportServer(labelName, labelValue string) (server *teleportServer, err error) {
	servers, err := s.getTeleportServers(labelName, labelValue)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(servers) == 0 {
		return nil, trace.NotFound("no teleport servers matching %v=%v",
			labelName, labelValue)
	}
	return &servers[0], nil
}

// getTeleportServerWithTimeout queries the teleport server with the specified label in a retry loop.
// timeout specifies the timeout used in a single query attempt, retryInterval is the frequency of retries
// and retryAttempts specifies the total number of attempts to make
//nolint:unparam // TODO: timeout is always defaults.TeleportServerQueryTimeout, retryInterval is always defaults.RetryInterval
func (s *site) getTeleportServersWithTimeout(labels map[string]string, timeout, retryInterval time.Duration,
	retryAttempts int, check func(string, []teleservices.Server) error) (teleservers, error) {

	var servers []teleservices.Server
	err := utils.Retry(retryInterval, retryAttempts, func() (err error) {
		ctx, cancel := context.WithTimeout(context.TODO(), timeout)
		defer cancel()
		servers, err = s.teleport().GetServers(ctx, s.domainName, labels)
		if err != nil {
			return trace.Wrap(err, "failed to query servers")
		}
		err = check(s.domainName, servers)
		return trace.Wrap(err)
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return teleservers(servers), nil
}

// atLeastOneServer is a server query condition that enforces that the result
// contain at least a single server item
func queryReturnsAtLeastOneServer(domainName string, servers []teleservices.Server) error {
	if len(servers) == 0 {
		return trace.NotFound("no servers found for %q", domainName)
	}
	return nil
}

func (r teleservers) getWithLabels(labels labels) (result teleservers) {
	result = make(teleservers, 0, len(r))
L:
	for _, server := range r {
		serverLabels := server.GetLabels()
		for name, value := range labels {
			if serverValue := serverLabels[name]; serverValue != value {
				continue L
			}
		}
		result = append(result, server)
	}
	return result
}

type teleservers []teleservices.Server

type labels map[string]string
