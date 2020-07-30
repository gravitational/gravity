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

package cli

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/rpc"
	pb "github.com/gravitational/gravity/lib/rpc/proto"
	rpcserver "github.com/gravitational/gravity/lib/rpc/server"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/system/service"
	"github.com/gravitational/gravity/lib/systemservice"
	"github.com/gravitational/gravity/lib/update"
	clusterupdate "github.com/gravitational/gravity/lib/update/cluster"
	"github.com/gravitational/gravity/lib/utils"
	"github.com/gravitational/gravity/tool/common"

	"github.com/buger/goterm"
	"github.com/cenkalti/backoff"
	teleclient "github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/trace"
	"github.com/gravitational/version"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/credentials"
)

func rpcAgentInstall(env *localenv.LocalEnvironment, args []string) error {
	gravityPath, err := os.Executable()
	if err != nil {
		return trace.Wrap(err, "failed to determine gravity executable path")
	}

	return trace.Wrap(service.ReinstallSimpleService(
		defaults.GravityRPCAgentServiceName,
		append([]string{gravityPath, "--debug", "agent", "run"}, args...)))
}

// rpcAgentRun runs a local agent executing the function specified with optional args
func rpcAgentRun(localEnv, updateEnv *localenv.LocalEnvironment, args []string) error {
	agent, err := newAgent()
	if err != nil {
		return trace.Wrap(err)
	}
	if len(args) == 0 {
		return trace.Wrap(agent.Serve())
	}
	agentFunc, exists := agentFunctions[args[0]]
	if !exists {
		return trace.NotFound("no such function %q", args[0])
	}
	if err := runAgentFunction(localEnv, updateEnv, agent, agentFunc, args); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(service.Disable(systemservice.DisableServiceRequest{
		Name: defaults.GravityRPCAgentServiceName,
	}))
}

func runAgentFunction(
	localEnv, updateEnv *localenv.LocalEnvironment,
	agent rpcserver.Server,
	agentFunc agentFunc,
	args []string,
) error {
	ctx, cancel := context.WithCancel(context.Background())
	errC := make(chan error, 2)
	f := func() error {
		handler, args := args[0], args[1:]
		log.WithField("handler", handler).Info("Execute.")
		return trace.Wrap(agentFunc(ctx, localEnv, updateEnv, args))
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), defaults.ShutdownTimeout)
		defer cancel()
		if err := agent.Stop(ctx); err != nil {
			log.Warnf("Failed to stop agent: %v.", err)
		}
	}()
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		select {
		case errC <- agent.Serve():
		case <-ctx.Done():
		}
		wg.Done()
	}()
	go func() {
		select {
		case errC <- f():
		case <-ctx.Done():
		}
		wg.Done()
	}()
	err := <-errC
	cancel()
	wg.Wait()
	return trace.Wrap(err)
}

func newAgent() (rpcserver.Server, error) {
	secretsDir, err := fsm.AgentSecretsDir()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	serverCreds, clientCreds, err := rpc.Credentials(secretsDir)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	serverAddr := fmt.Sprintf(":%v", defaults.GravityRPCAgentPort)
	listener, err := net.Listen("tcp4", serverAddr)
	if err != nil {
		return nil, trace.Wrap(err, "failed to bind to %v", serverAddr)
	}

	config := rpcserver.Config{
		Credentials: rpcserver.Credentials{
			Server: serverCreds,
			Client: clientCreds,
		},
		Listener: listener,
	}

	server, err := rpcserver.New(config)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	log.Infof("Starting RPC agent on %v.", listener.Addr().String())

	return server, nil
}

type agentFunc func(ctx context.Context, localEnv, upgradeEnv *localenv.LocalEnvironment, args []string) error

var agentFunctions map[string]agentFunc = map[string]agentFunc{
	constants.RPCAgentUpgradeFunction:  executeAutomaticUpgrade,
	constants.RPCAgentSyncPlanFunction: executeSyncOperationPlan,
}

type deployOptions struct {
	// leaderArgs is additional arguments to the leader agent
	leaderArgs string
	// nodeArgs is additional arguments to the regular agent
	nodeArgs string
	// version specifies the version of the agent to be deployed
	version string
	// hostname specifies the hostname of the node to deploy the agent on
	hostname string
}

func rpcAgentDeploy(localEnv *localenv.LocalEnvironment, options deployOptions) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaults.AgentDeployTimeout)
	defer cancel()
	_, err := rpcAgentDeployHelper(ctx, localEnv, options)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func rpcAgentDeployHelper(ctx context.Context, localEnv *localenv.LocalEnvironment, options deployOptions) (credentials.TransportCredentials, error) {
	localEnv.PrintStep("Deploying agents on the cluster nodes")

	clusterEnv, err := localEnv.NewClusterEnvironment()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	operator, err := localEnv.SiteOperator()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cluster, err := operator.GetLocalSite()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	teleportClient, err := localEnv.TeleportClient(constants.Localhost)
	if err != nil {
		return nil, trace.Wrap(err, "failed to create a teleport client")
	}

	proxy, err := teleportClient.ConnectToProxy(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "failed to connect to teleport proxy")
	}

	// If version is not specified in the request, use the current build version
	if options.version == "" {
		options.version = version.Get().Version
	}

	req := deployAgentsRequest{
		clusterState: cluster.ClusterState,
		cluster:      *cluster,
		clusterEnv:   clusterEnv,
		proxy:        proxy,
		leaderParams: options.leaderArgs,
		nodeParams:   options.nodeArgs,
		version:      options.version,
	}

	// If hostname is specified in the options, deploy agent only on specified node
	if options.hostname != "" {
		server, err := req.clusterState.FindServer(options.hostname)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		req.servers = append(req.servers, *server)
	} else {
		req.servers = cluster.ClusterState.Servers
	}

	// Force this node to be the operation leader
	req.leader, err = findLocalServer(cluster.ClusterState.Servers)
	if err != nil {
		log.WithError(err).Warn("Failed to determine local node.")
		return nil, trace.Wrap(err, "failed to find local node in cluster state.\n"+
			"Make sure you start the operation from one of the cluster master nodes.")
	}

	localCtx, cancel := context.WithTimeout(ctx, defaults.AgentDeployTimeout)
	defer cancel()

	return deployAgents(localCtx, localEnv, req)
}

// verifyNode verifies that we can connect to the teleport node
func verifyNode(ctx context.Context, server rpc.DeployServer, proxy *teleclient.ProxyClient) error {
	client, err := proxy.ConnectToNode(ctx, server.NodeAddr, defaults.SSHUser, false)
	if err != nil {
		return trace.Wrap(err)
	}
	client.Close()
	return nil
}

func verifyCluster(ctx context.Context, req deployAgentsRequest) (servers []rpc.DeployServer, err error) {
	var missing []string
	servers = make([]rpc.DeployServer, 0, len(servers))

	for _, server := range req.servers {
		deployServer := rpc.NewDeployServer(server)

		// do a quick check to make sure we can connect to the teleport node
		if err := verifyNode(ctx, deployServer, req.proxy); err != nil {
			log.WithError(err).Errorf("Failed to connect to teleport on node %v.",
				deployServer)
			missing = append(missing, server.Hostname)
			continue
		}

		log.Infof("Successfully connected to teleport on node %v (%v).",
			server.Hostname, deployServer.NodeAddr)
		servers = append(servers, deployServer)
	}
	if len(missing) != 0 {
		return nil, trace.NotFound(teleportUnavailableMessage,
			strings.Join(missing, ", "), getTeleportVersion(req.cluster.App.Manifest))
	}

	return servers, nil
}

func getTeleportVersion(manifest schema.Manifest) string {
	teleportPackage, err := manifest.Dependencies.ByName(constants.TeleportPackage)
	if err == nil {
		return teleportPackage.Version
	}
	return "<version>"
}

const (
	// teleportUnavailableMessage is displayed when some Teleport nodes are
	// unavailable during agents deployment.
	teleportUnavailableMessage = `Teleport is unavailable on the following cluster nodes: %[1]s.

Please check the status and logs of Teleport systemd service on the specified
nodes and make sure it's running:

systemctl status gravity__gravitational.io__teleport__%[2]v
journalctl -u gravity__gravitational.io__teleport__%[2]v --no-pager

After fixing the issue, "./gravity status" can be used to confirm the status of
Teleport on each node using "remote access" field.

Once all Teleport nodes are running, launch the upgrade again.
`
)

func upsertRPCCredentialsPackage(
	servers []rpc.DeployServer,
	packages pack.PackageService,
	clusterName string,
	packageTemplate loc.Locator) (secretsPackage *loc.Locator, err error) {
	hosts := make([]string, 0, len(servers))
	for _, server := range servers {
		hosts = append(hosts, strings.Split(server.NodeAddr, ":")[0])
	}

	archive, err := rpc.GenerateAgentCredentials(hosts, clusterName, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	secretsPackage, err = rpc.GenerateAgentCredentialsPackage(packages, packageTemplate, archive)
	if err != nil && !trace.IsAlreadyExists(err) {
		return nil, trace.Wrap(err)
	}
	return secretsPackage, nil
}

func deployAgents(ctx context.Context, env *localenv.LocalEnvironment, req deployAgentsRequest) (credentials.TransportCredentials, error) {
	deployReq, err := newDeployAgentsRequest(ctx, env, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = rpc.DeployAgents(ctx, *deployReq)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clientCreds, err := getClientCredentials(ctx, req.clusterEnv.ClusterPackages, deployReq.SecretsPackage)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return clientCreds, nil
}

// newDeployAgentsRequest creates a new request to deploy agents on the local cluster
func newDeployAgentsRequest(ctx context.Context, env *localenv.LocalEnvironment, req deployAgentsRequest) (*rpc.DeployAgentsRequest, error) {
	servers, err := verifyCluster(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	gravityPackage := loc.Locator{
		Repository: defaults.SystemAccountOrg,
		Name:       constants.GravityPackage,
		Version:    req.version,
	}

	secretsPackageTemplate := loc.Locator{
		Repository: req.cluster.Domain,
		Version:    req.version,
	}

	secretsPackage, err := upsertRPCCredentialsPackage(
		servers, req.clusterEnv.ClusterPackages, req.cluster.Domain, secretsPackageTemplate)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &rpc.DeployAgentsRequest{
		Proxy:          req.proxy,
		ClusterState:   req.clusterState,
		Servers:        servers,
		SecretsPackage: *secretsPackage,
		GravityPackage: gravityPackage,
		FieldLogger:    logrus.WithField(trace.Component, "rpc:deploy"),
		LeaderParams:   req.leaderParams,
		Leader:         req.leader,
		NodeParams:     req.nodeParams,
		Progress:       utils.NewProgress(ctx, "", 0, bool(env.Silent)),
	}, nil
}

func getClientCredentials(ctx context.Context, packages pack.PackageService, secretsPackage loc.Locator) (credentials.TransportCredentials, error) {
	var r io.Reader
	ctx, cancel := defaults.WithTimeout(ctx)
	defer cancel()
	err := utils.RetryWithInterval(ctx, utils.NewUnlimitedExponentialBackOff(), func() (err error) {
		_, r, err = packages.ReadPackage(secretsPackage)
		if err != nil {
			if utils.IsPathError(err) {
				log.Debugf("Package %v has not been replicated yet, will retry.", secretsPackage)
				return trace.Wrap(err)
			}
			return &backoff.PermanentError{Err: err}
		}
		return nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tlsArchive, err := utils.ReadTLSArchive(r)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clientCreds, err := rpc.ClientCredentialsFromKeyPairs(
		*tlsArchive[pb.Client], *tlsArchive[pb.CA])
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return clientCreds, nil
}

func rpcAgentShutdown(env *localenv.LocalEnvironment) error {
	env.PrintStep("Shutting down the agents")
	creds, err := fsm.GetClientCredentials()
	if err != nil {
		return trace.Wrap(err)
	}
	runner := fsm.NewAgentRunner(creds)
	err = clusterupdate.ShutdownClusterAgents(context.TODO(), runner)
	return trace.Wrap(err)
}

// rpcAgentStatus requests the gravity agent status from all members of the
// cluster, then writes the information to stdout.
// If an agent fails to return a status response, the agent will be considered
// `Offline` and will display an empty version column.
func rpcAgentStatus(env *localenv.LocalEnvironment) error {
	env.PrintStep("Collecting RPC agent status")

	operator, err := env.SiteOperator()
	if err != nil {
		return trace.Wrap(err)
	}

	creds, err := fsm.GetClientCredentials()
	if err != nil {
		return trace.Wrap(err)
	}

	cluster, err := operator.GetLocalSite()
	if err != nil {
		return trace.Wrap(err)
	}

	timeout, err := utils.GetenvDuration(constants.AgentStatusTimeoutEnvVar)
	if err != nil {
		timeout = defaults.AgentStatusTimeout
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	statusList := rpc.CollectAgentStatus(ctx, cluster.ClusterState.Servers, fsm.NewAgentRunner(creds))

	var errs []error

	t := goterm.NewTable(0, 10, 5, ' ', 0)
	common.PrintTableHeader(t, []string{"Hostname", "Address", "Status", "Version"})
	for _, status := range statusList {
		fmt.Fprintf(t, "%s\t%s\t%s\t%s\n", status.Hostname, status.Address, status.Status, status.Version)
		if status.Error != nil {
			log.WithError(status.Error).Debugf("Failed to collect agent status on %s.", status.Address)
			errs = append(errs, status.Error)
		}
	}
	env.Println(t.String())

	if len(errs) > 0 {
		log.Warn("Some agents are offline.")
		return trace.BadParameter("some agents are offline")
	}

	return nil
}

func executeAutomaticUpgrade(ctx context.Context, localEnv, upgradeEnv *localenv.LocalEnvironment, args []string) error {
	return trace.Wrap(clusterupdate.AutomaticUpgrade(ctx, localEnv, upgradeEnv))
}

func executeSyncOperationPlan(ctx context.Context, localEnv, updateEnv *localenv.LocalEnvironment, args []string) error {
	clusterEnv, err := localEnv.NewClusterEnvironment()
	if err != nil {
		return trace.Wrap(err)
	}
	operation, err := storage.GetLastOperation(clusterEnv.Backend)
	if err != nil {
		return trace.Wrap(err)
	}
	plan, err := clusterEnv.Backend.GetOperationPlan(operation.SiteDomain, operation.ID)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(update.SyncOperationPlan(clusterEnv.Backend, updateEnv.Backend, *plan, *operation))
}

func getGravityPackage() loc.Locator {
	ver := version.Get()
	return loc.Locator{
		Repository: defaults.SystemAccountOrg,
		Name:       constants.GravityPackage,
		Version:    strings.Split(ver.Version, "+")[0],
	}
}

type deployAgentsRequest struct {
	clusterEnv   *localenv.ClusterEnvironment
	clusterState storage.ClusterState
	cluster      ops.Site
	proxy        *teleclient.ProxyClient
	leader       *storage.Server
	// servers specifies the list of servers to deploy agents on
	servers      storage.Servers
	leaderParams string
	nodeParams   string
	// version specifies the version of the gravity agent to deploy
	version string
}
