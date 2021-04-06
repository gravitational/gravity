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
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/httplib"
	"github.com/gravitational/gravity/lib/install"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/pack/webpack"
	"github.com/gravitational/gravity/lib/processconfig"
	rpcserver "github.com/gravitational/gravity/lib/rpc/server"
	"github.com/gravitational/gravity/lib/state"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/systeminfo"
	"github.com/gravitational/gravity/lib/utils"
	"github.com/gravitational/gravity/tool/common"

	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

// LocalEnvironmentFactory defines an interface for creating operation-specific environments
type LocalEnvironmentFactory interface {
	// NewLocalEnv creates a new default environment.
	// It will use the location pointer file to find the location of the custom state
	// directory if available and will fall back to defaults.GravityDir otherwise.
	// All other environments are located under this common root directory
	NewLocalEnv() (*localenv.LocalEnvironment, error)
	// TODO(dmitri): generalize operation environment under a single
	// NewOperationEnv API
	// NewUpdateEnv creates a new environment for update operations
	NewUpdateEnv() (*localenv.LocalEnvironment, error)
	// NewJoinEnv creates a new environment for join operations
	NewJoinEnv(stateDir string) (*localenv.LocalEnvironment, error)
}

// NewLocalEnv returns an instance of the local environment.
func (g *Application) NewLocalEnv() (env *localenv.LocalEnvironment, err error) {
	localStateDir, err := getLocalStateDir(*g.StateDir)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return g.getEnv(localStateDir)
}

// NewInstallEnv returns an instance of the local environment for commands that
// initialize cluster environment (i.e. install or join).
func (g *Application) NewInstallEnv() (env *localenv.LocalEnvironment, err error) {
	stateDir := *g.StateDir
	if stateDir == "" {
		stateDir = defaults.LocalGravityDir
	} else {
		stateDir = filepath.Join(stateDir, defaults.LocalDir)
	}
	return g.getEnvWithArgs(localenv.LocalEnvironmentArgs{
		StateDir:         stateDir,
		Insecure:         *g.Insecure,
		Silent:           localenv.Silent(*g.Silent),
		Debug:            *g.Debug,
		EtcdRetryTimeout: *g.EtcdRetryTimeout,
		// Use DNS configuration from installer command line.
		// TODO(dmitri): setting this will only be useful for the install operation
		// as in this case the DNS coniguration will first be set in local state during
		// boostrapping step and the application service that is created based on this
		// setting would have otherwise pointed to the legacy DNS configuration which is
		// incorrect.
		// This is rather a workaround - proper solution will be more involved and will have
		// the application service using the kubernetes client (and hence the DNS config
		// to resolve the names) only for hooks.
		DNS:      localenv.DNSConfig(g.InstallCmd.DNSConfig()),
		Reporter: common.ProgressReporter(*g.Silent),
	})
}

// NewUpdateEnv returns an instance of the local environment that is used
// only for updates
func (g *Application) NewUpdateEnv() (*localenv.LocalEnvironment, error) {
	dir, err := state.GetStateDir()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return g.getEnv(state.GravityUpdateDir(dir))
}

// NewJoinEnv returns an instance of local environment where join-specific data is stored
func (g *Application) NewJoinEnv(stateDir string) (*localenv.LocalEnvironment, error) {
	const failImmediatelyIfLocked = -1
	err := os.MkdirAll(stateDir, defaults.SharedDirMask)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	return g.getEnvWithArgs(localenv.LocalEnvironmentArgs{
		StateDir:         stateDir,
		Insecure:         *g.Insecure,
		Silent:           localenv.Silent(*g.Silent),
		Debug:            *g.Debug,
		EtcdRetryTimeout: *g.EtcdRetryTimeout,
		BoltOpenTimeout:  failImmediatelyIfLocked,
		Reporter:         common.ProgressReporter(*g.Silent),
	})
}

func (g *Application) getEnv(stateDir string) (*localenv.LocalEnvironment, error) {
	return g.getEnvWithArgs(localenv.LocalEnvironmentArgs{
		StateDir:         stateDir,
		Insecure:         *g.Insecure,
		Silent:           localenv.Silent(*g.Silent),
		Debug:            *g.Debug,
		EtcdRetryTimeout: *g.EtcdRetryTimeout,
		Reporter:         common.ProgressReporter(*g.Silent),
	})
}

func (g *Application) getEnvWithArgs(args localenv.LocalEnvironmentArgs) (*localenv.LocalEnvironment, error) {
	if *g.StateDir != defaults.LocalGravityDir {
		args.LocalKeyStoreDir = *g.StateDir
	}
	// set insecure in devmode so we won't need to use
	// --insecure flag all the time
	cfg, _, err := processconfig.ReadConfig("")
	if err == nil && cfg.Devmode {
		args.Insecure = true
	}
	return localenv.NewLocalEnvironment(args)
}

// ConfigureNoProxy configures the current process to not use any configured HTTP proxy when connecting to any
// destination by IP address, or a domain with a suffix of .local. Gravity internally connects to nodes by IP address,
// and by queries to kubernetes using the .local suffix. The side effect is, connections towards the internet by IP
// address and not a configured domain name will not be able to invoke a proxy. This should be a reasonable tradeoff,
// because with a cluster that changes over time, it's difficult for us to accuratly detect what IP addresses need to
// have no_proxy set.
func ConfigureNoProxy() {
	// The golang HTTP proxy env variable detection only uses the first detected http proxy env variable
	// so we need to grab both to make sure we edit the correct one.
	// https://github.com/golang/net/blob/c21de06aaf072cea07f3a65d6970e5c7d8b6cd6d/http/httpproxy/proxy.go#L91-L107
	proxy := map[string]string{
		"NO_PROXY": os.Getenv("NO_PROXY"),
		"no_proxy": os.Getenv("no_proxy"),
	}

	for k, v := range proxy {
		if len(v) != 0 {
			os.Setenv(k, strings.Join([]string{v, "0.0.0.0/0", ".local"}, ","))
			return
		}
	}

	os.Setenv("NO_PROXY", strings.Join([]string{"0.0.0.0/0", ".local"}, ","))
}

func getLocalStateDir(stateDir string) (localStateDir string, err error) {
	if stateDir != "" {
		// If state directory has been explicitly specified on command line,
		// use it
		return stateDir, nil
	}
	stateDir, err = state.GetStateDir()
	if err != nil {
		return "", trace.Wrap(err)
	}
	return filepath.Join(stateDir, defaults.LocalDir), nil
}

// findLocalServer searches the provided cluster's state for the server that matches the one
// the current command is being executed from
func findLocalServer(servers storage.Servers) (*storage.Server, error) {
	// collect the machines's IP addresses and search by them
	ifaces, err := systeminfo.NetworkInterfaces()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(ifaces) == 0 {
		return nil, trace.NotFound("no network interfaces found")
	}

	var ips []string
	for _, iface := range ifaces {
		ips = append(ips, iface.IPv4)
	}

	server, err := findServer(servers, ips)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return server, nil
}

// findServer searches the provided cluster's state for a server that matches one of the provided
// tokens, where a token can be the server's advertise IP, hostname, cloud specific InstanceID or AWS internal DNS name
func findServer(servers storage.Servers, tokens []string) (*storage.Server, error) {
	for _, server := range servers {
		for _, token := range tokens {
			if token == "" {
				continue
			}
			switch token {
			case server.AdvertiseIP, server.Hostname, server.Nodename, server.InstanceID:
				return &server, nil
			}
		}
	}
	return nil, trace.NotFound("no server matching %v found among registered cluster nodes",
		tokens)
}

func isCancelledError(err error) bool {
	if err == nil {
		return false
	}
	return trace.IsCompareFailed(err) && strings.Contains(err.Error(), "cancelled")
}

func watchReconnects(ctx context.Context, cancel context.CancelFunc, watchCh <-chan rpcserver.WatchEvent) {
	go func() {
		for event := range watchCh {
			if event.Error == nil {
				continue
			}
			log.Warnf("Failed to reconnect to %v: %v.", event.Peer, event.Error)
			cancel()
			return
		}
	}()
}

func loadRPCCredentials(ctx context.Context, addr, token string) (*rpcserver.Credentials, error) {
	// Assume addr to be a complete address if it's prefixed with `http`
	if !strings.Contains(addr, "http") {
		host, port := utils.SplitHostPort(addr, strconv.Itoa(defaults.GravitySiteNodePort))
		addr = fmt.Sprintf("https://%v:%v", host, port)
	}
	httpClient := roundtrip.HTTPClient(httplib.GetClient(true))
	packages, err := webpack.NewBearerClient(addr, token, httpClient)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	creds, err := install.LoadRPCCredentials(ctx, packages)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return creds, nil
}

func parseArgs(args []string) (*kingpin.ParseContext, error) {
	app := kingpin.New("gravity", "")
	app.Terminate(func(int) {})
	app.Writer(ioutil.Discard)
	return RegisterCommands(app).ParseContext(args)
}
