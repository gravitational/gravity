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

package rpc

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/fatih/color"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/state"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	teleclient "github.com/gravitational/teleport/lib/client"
	teledefaults "github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// DeployAgentsRequest defines the extent of configuration
// necessary to deploy agents on the local cluster.
type DeployAgentsRequest struct {
	// GravityPackage specifies the gravity binary package to use
	// as the main process
	GravityPackage loc.Locator

	// ClusterState is the cluster state
	ClusterState storage.ClusterState

	// Servers lists the servers to deploy
	Servers []DeployServer

	// SecretsPackage specifies the package with RPC credentials
	SecretsPackage loc.Locator

	// Proxy telekube proxy for remote execution
	Proxy *teleclient.ProxyClient

	// FieldLogger defines the logger to use
	logrus.FieldLogger

	// LeaderParams defines which parameters to pass to the leader agent process.
	// The leader agent specifies the agent that executes an operation.
	LeaderParams string

	// Leader is the node where the leader agent should be launched
	//
	// If not set, the first master node will serve as a leader
	Leader *storage.Server

	// NodeParams defines which parameters to pass to the regular agent process.
	NodeParams string

	// Progress is the progress reporter.
	Progress utils.Progress
}

// CheckAndSetDefaults validates the request to deploy agents and sets defaults.
func (r *DeployAgentsRequest) CheckAndSetDefaults() error {
	// if the leader node was explicitly passed, make sure
	// it is present among the deploy nodes
	if r.Leader != nil && len(r.LeaderParams) != 0 {
		leaderPresent := false
		for _, node := range r.Servers {
			if node.AdvertiseIP == r.Leader.AdvertiseIP {
				leaderPresent = true
				break
			}
		}
		if !leaderPresent {
			return trace.NotFound("requested leader node %v was not found among deploy servers: %v",
				r.Leader.AdvertiseIP, r.Servers)
		}
	}
	if r.Progress == nil {
		r.Progress = utils.DiscardProgress
	}
	return nil
}

// canBeLeader returns true if the provided node can run leader agent
func (r DeployAgentsRequest) canBeLeader(node DeployServer) bool {
	// if there are no leader-specific parameters, there is no leader agent
	if len(r.LeaderParams) == 0 {
		return false
	}
	// if no specific leader node was requested, any master will do
	if r.Leader == nil {
		return node.Role == schema.ServiceRoleMaster
	}
	// otherwise see if this is the requested leader node
	return r.Leader.AdvertiseIP == node.AdvertiseIP
}

// DeployAgents uses teleport to discover cluster nodes, distribute and run RPC agents
// across the local cluster.
// One of the master nodes is selected to control the automatic update operation specified
// with req.LeaderParams.
func DeployAgents(ctx context.Context, req DeployAgentsRequest) error {
	if err := req.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	errors := make(chan error, len(req.Servers))
	leaderProcessScheduled := false
	for _, server := range req.Servers {
		leaderProcess := false
		if !leaderProcessScheduled && req.canBeLeader(server) {
			leaderProcess = true
			leaderProcessScheduled = true
			req.WithField("args", req.LeaderParams).
				Infof("Master process will run on node %v/%v.",
					server.Hostname, server.NodeAddr)
		}

		// determine the server's state directory
		stateServer, err := req.ClusterState.FindServerByIP(server.AdvertiseIP)
		if err != nil {
			return trace.Wrap(err)
		}

		logger := req.WithFields(server.fields())

		go func(nodeAddr string, leader bool) {
			// Try a few times to account for possible network glitches.
			err := utils.RetryOnNetworkError(defaults.RetryInterval, defaults.RetryLessAttempts, func() error {
				if err := deployAgentOnNode(ctx, req, *stateServer, nodeAddr, leader, req.SecretsPackage.String()); err != nil {
					logger.WithError(err).Warn("Failed to deploy agent.")
					return trace.Wrap(err)
				}
				req.Progress.Print(color.GreenString("Deployed agent on %v (%v)", stateServer.Hostname, stateServer.AdvertiseIP))
				logger.Info("Agent deployed.")
				return nil
			})
			errors <- trace.Wrap(err, "failed to deploy agent on %v", stateServer.AdvertiseIP)
		}(server.NodeAddr, leaderProcess)
	}

	err := utils.CollectErrors(ctx, errors)
	if err != nil {
		return trace.Wrap(err)
	}

	if !leaderProcessScheduled && len(req.LeaderParams) > 0 {
		return trace.NotFound("No nodes with %s=%s were found while scheduling agents, requested operation %q is not running.",
			schema.ServiceLabelRole, schema.ServiceRoleMaster, req.LeaderParams)
	}

	req.Println("Agents deployed.")
	return nil
}

// DeployServer describes an agent to deploy on every node during update.
//
// Agents come in two flavors: passive or controller.
// Once an agent cluster has been built, an agent will be selected to
// control the update (i.e. give commands to other agents) if the process is automatic.
type DeployServer struct {
	// Role specifies the server's service role
	Role schema.ServiceRole
	// AdvertiseIP specifies the address the server is available on
	AdvertiseIP string
	// Hostname specifies the server's hostname
	Hostname string
	// NodeAddr is the server's address in teleport context
	NodeAddr string
}

// fields returns log fields for the server.
func (s DeployServer) fields() logrus.Fields {
	return logrus.Fields{"hostname": s.Hostname, "ip": s.AdvertiseIP}
}

// NewDeployServer creates a new instance of DeployServer
func NewDeployServer(node storage.Server) DeployServer {
	return DeployServer{
		Role:        schema.ServiceRole(node.ClusterRole),
		Hostname:    node.Hostname,
		AdvertiseIP: node.AdvertiseIP,
		NodeAddr: fmt.Sprintf("%v:%v", node.AdvertiseIP,
			teledefaults.SSHServerListenPort),
	}
}

func deployAgentOnNode(ctx context.Context, req DeployAgentsRequest, server storage.Server, nodeAddr string, leader bool, secretsPackage string) error {
	nodeClient, err := req.Proxy.ConnectToNode(ctx, nodeAddr, defaults.SSHUser, false)
	if err != nil {
		return trace.Wrap(err, "failed to connect").AddField("node", nodeAddr)
	}
	defer nodeClient.Close()

	stateDir := server.StateDir()
	gravityHostPath := filepath.Join(
		state.GravityRPCAgentDir(stateDir), constants.GravityPackage)
	secretsHostDir := filepath.Join(
		state.GravityRPCAgentDir(stateDir), defaults.SecretsDir)

	var runCmd string
	if leader {
		runCmd = fmt.Sprintf("%s agent --debug install %s",
			gravityHostPath, req.LeaderParams)
	} else {
		runCmd = fmt.Sprintf("%s agent --debug install %s",
			gravityHostPath, req.NodeParams)
	}

	exportFormat := "%s package export --file-mask=%o %s %s --ops-url=%s --insecure"
	exportArgs := []interface{}{
		constants.GravityBin, defaults.SharedExecutableMask,
		req.GravityPackage, gravityHostPath, defaults.GravityServiceURL,
	}
	if server.SELinux {
		exportFormat = "%s package export --file-mask=%o %s %s --ops-url=%s --insecure --file-label=%s"
		exportArgs = append(exportArgs, defaults.GravityFileLabel)
	}
	err = utils.NewSSHCommands(nodeClient.Client).
		C("rm -rf %s", secretsHostDir).
		C("mkdir -p %s", secretsHostDir).
		WithRetries("%s package unpack %s %s --debug --ops-url=%s --insecure",
			constants.GravityBin, secretsPackage, secretsHostDir, defaults.GravityServiceURL).
		IgnoreError("/bin/systemctl stop %s", defaults.GravityRPCAgentServiceName).
		WithRetries(exportFormat, exportArgs...).
		C(runCmd).
		WithLogger(req.WithField("node", nodeAddr)).
		Run(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}
