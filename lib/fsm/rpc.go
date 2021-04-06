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

package fsm

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"sync"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/rpc"
	rpcclient "github.com/gravitational/gravity/lib/rpc/client"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/state"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/systeminfo"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/credentials"
)

// AgentSecretsDir returns the location of agent credentials
func AgentSecretsDir() (string, error) {
	stateDir, err := state.GetStateDir()
	if err != nil {
		return "", trace.Wrap(err)
	}

	secretsDir := filepath.Join(
		state.GravityRPCAgentDir(stateDir), defaults.SecretsDir)
	return secretsDir, nil
}

// CheckServer determines if the specified server is a local machine
// or has an agent running. Returns an error if the server cannot be
// used to execute a command (neither is a local machine nor has an agent running).
func (f *FSM) CheckServer(ctx context.Context, server storage.Server) error {
	can, err := canExecuteOnServer(ctx, server, f.Runner, f.FieldLogger)
	if err != nil {
		return trace.Wrap(err)
	}

	if can == CanRunLocally || can == CanRunRemotely {
		return nil
	}

	return trace.NotFound("no agent is running on %q, please execute this command on that node locally", serverName(server))
}

// Remote allows to invoke remote commands
type Remote interface {
	// CheckServer determines if the specified server is a local machine
	// or has an agent running. Returns an error if the server cannot be
	// used to execute a command (neither is a local machine nor has an agent running).
	CheckServer(context.Context, storage.Server) error
}

// NewAgentRunner creates a new RemoteRunner that uses a cluster of agents
// to run remote commands
func NewAgentRunner(creds credentials.TransportCredentials) *agentRunner {
	return &agentRunner{
		FieldLogger: logrus.WithField(trace.Component, "fsm:remote"),
		agentCache: &agentCache{
			creds:   creds,
			clients: make(map[string]rpcclient.Client),
		},
	}
}

// Run executes a command on the remote server
// Implements rpc.RemoteRunner
func (r *agentRunner) Run(ctx context.Context, server storage.Server, args ...string) error {
	logger := r.WithFields(logrus.Fields{
		"gravity": args,
		"server":  serverName(server),
	})

	canRun, err := canExecuteOnServer(ctx, server, r, logger)
	if err != nil {
		return trace.Wrap(err)
	}

	switch canRun {
	case ShouldRunRemotely:
		return trace.Errorf("no agent is running on %s, please execute this command on that node", serverName(server))
	case CanRunLocally:
		logger.Debug("Executing locally.")
		out, err := RunCommand(append([]string{constants.GravityBin}, args...))
		if err != nil {
			logger.Warnf("Failed to execute gravity command %q: %s (%v).",
				args, out, trace.DebugReport(err))
		}
		return trace.Wrap(err, "failed to execute gravity command %q: %s", args, out)
	case CanRunRemotely:
		logger.WithField("server", server).Debug("Dialing...")
		agent, err := r.GetClient(ctx, server.AdvertiseIP)
		if err != nil {
			return trace.Wrap(err, "failed to execute gravity command %v on remote node %v",
				args, serverName(server))
		}
		logger.Debug("Executing remotely: ", args)
		var stderr bytes.Buffer
		err = agent.GravityCommand(ctx, logger, nil, &stderr, args...)
		return trace.Wrap(err, "failed to execute command %v: %s", args, stderr.String())
	default:
		return trace.Errorf("internal error, canExecute=%v", canRun)
	}
}

// CanExecute verifies if it can execute remote commands on server
func (r *agentRunner) CanExecute(ctx context.Context, server storage.Server) error {
	_, err := r.GetClient(ctx, server.AdvertiseIP)
	return trace.Wrap(err)
}

type agentRunner struct {
	logrus.FieldLogger
	// agentCache provides access to RPC agents
	*agentCache
}

func canExecuteOnServer(ctx context.Context, server storage.Server, runner rpc.RemoteRunner, log logrus.FieldLogger) (ExecutionCheck, error) {
	err := systeminfo.HasInterface(server.AdvertiseIP)
	if err == nil {
		return CanRunLocally, nil
	}

	if !trace.IsNotFound(err) {
		return ExecutionCheckUndefined, trace.Wrap(err)
	}

	ctx, cancel := context.WithTimeout(ctx, defaults.DialTimeout)
	defer cancel()
	err = runner.CanExecute(ctx, server)
	if err == nil {
		return CanRunRemotely, nil
	}

	log.WithFields(logrus.Fields{
		"error":  err,
		"server": server,
	}).Warn("Failed to connect to the remote agent.")
	return ShouldRunRemotely, nil
}

func serverName(server storage.Server) string {
	return fmt.Sprintf("%s/%s", server.Hostname, server.AdvertiseIP)
}

type ExecutionCheck int

const (
	CanRunLocally = ExecutionCheck(iota)
	CanRunRemotely
	ShouldRunRemotely
	ExecutionCheckUndefined
)

// Close closes this cache by closing all existing clients
func (r *agentCache) Close() error {
	for _, clt := range r.clients {
		clt.Close()
	}
	return nil
}

// GetClient returns a new agent client
func (r *agentCache) GetClient(ctx context.Context, addr string) (clt rpcclient.Client, err error) {
	addr = rpc.AgentAddr(addr)
	r.Lock()
	clt = r.clients[addr]
	r.Unlock()
	if clt != nil {
		return clt, nil
	}

	clt, err = rpcclient.New(ctx, rpcclient.Config{ServerAddr: addr, Credentials: r.creds})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	r.Lock()
	r.clients[addr] = clt
	r.Unlock()
	return clt, nil
}

// Implements remote
type agentCache struct {
	creds credentials.TransportCredentials
	sync.Mutex
	clients map[string]rpcclient.Client
}

// RunCommand executes the provided command locally and returns its output
func RunCommand(args []string) ([]byte, error) {
	logrus.Debugf("Executing command: %v.", args)
	command := exec.Command(args[0], args[1:]...)
	var buf bytes.Buffer
	err := utils.Exec(command, &buf)
	return buf.Bytes(), trace.Wrap(err)
}

// CheckMasterServer makes sure this method is executed on a master server
func CheckMasterServer(servers []storage.Server) error {
	var thisServer storage.Server
	for _, server := range servers {
		err := systeminfo.HasInterface(server.AdvertiseIP)
		if err != nil && !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		if trace.IsNotFound(err) {
			continue
		}
		thisServer = server
		break
	}
	if !IsMasterServer(thisServer) {
		return trace.BadParameter(
			"this phase must be executed from a master node")
	}
	return nil
}

// IsMasterServer returns true if the provided service has a master cluster role
func IsMasterServer(server storage.Server) bool {
	return server.ClusterRole == string(schema.ServiceRoleMaster)
}

// GetClientCredentials returns the RPC credentials for an update operation
func GetClientCredentials() (credentials.TransportCredentials, error) {
	secretsDir, err := AgentSecretsDir()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	creds, err := rpc.ClientCredentials(secretsDir)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return creds, nil
}
