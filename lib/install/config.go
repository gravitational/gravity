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
package install

import (
	"context"
	"fmt"
	"strconv"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/install/engine"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/ops/opsclient"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/pack/localpack"
	"github.com/gravitational/gravity/lib/process"
	pb "github.com/gravitational/gravity/lib/rpc/proto"
	rpcserver "github.com/gravitational/gravity/lib/rpc/server"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/systeminfo"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
)

// NewFSMConfig returns state machine configiration
func NewFSMConfig(operator ops.Operator, operationKey ops.SiteOperationKey, config Config) FSMConfig {
	fsmConfig := FSMConfig{
		Operator:           operator,
		OperationKey:       operationKey,
		Packages:           config.Packages,
		Apps:               config.Apps,
		LocalPackages:      config.LocalPackages,
		LocalApps:          config.LocalApps,
		LocalBackend:       config.LocalBackend,
		LocalClusterClient: config.LocalClusterClient,
		Insecure:           config.Insecure,
		UserLogFile:        config.UserLogFile,
		ReportProgress:     true,
	}
	fsmConfig.Spec = FSMSpec(fsmConfig)
	return fsmConfig
}

// GetWizardAddr returns the advertise address of the wizard process
func (c *Config) GetWizardAddr() (addr string) {
	return c.Process.Config().WizardAddr()
}

// Config is installer configuration
type Config struct {
	// FieldLogger is used for logging
	log.FieldLogger
	// Printer specifies the output sink for progress messages
	utils.Printer
	// AdvertiseAddr is advertise address of this server
	AdvertiseAddr string
	// Token specifies the agent validation token used during the operation
	Token storage.InstallToken
	// CloudProvider is optional cloud provider
	CloudProvider string
	// StateDir is directory with local installer state
	StateDir string
	// WriteStateDir is installer write layer
	WriteStateDir string
	// UserLogFile is the log file where user-facing operation logs go
	UserLogFile string
	// SiteDomain is the name of the cluster
	SiteDomain string
	// Flavor is installation flavor
	Flavor schema.Flavor
	// Role is server role
	Role string
	// App is the application being installed
	App app.Application
	// RuntimeResources specifies optional Kubernetes resources to create
	RuntimeResources []runtime.Object
	// ClusterResources specifies optional cluster resources to create
	// TODO(dmitri): externalize the ClusterConfiguration resource and create
	// default provider-specific cloud-config on Gravity side
	ClusterResources []storage.UnknownResource
	// SystemDevice is a device for gravity data
	SystemDevice string
	// Mounts is a list of mount points (name -> source pairs)
	Mounts map[string]string
	// DNSOverrides contains installer node DNS overrides
	DNSOverrides storage.DNSOverrides
	// VxlanPort is the overlay network port
	VxlanPort int
	// DNSConfig overrides the local cluster DNS configuration
	DNSConfig storage.DNSConfig
	// Docker specifies docker configuration
	Docker storage.DockerConfig
	// Insecure allows to turn off cert validation
	Insecure bool
	// Process is the gravity process running inside the installer
	Process process.GravityProcess
	// LocalPackages is the machine-local package service
	LocalPackages *localpack.PackageServer
	// LocalApps is the machine-local application service
	LocalApps app.Applications
	// LocalBackend is the machine-local backend
	LocalBackend storage.Backend
	// ServiceUser specifies the user to use as a service user in planet
	// and for unprivileged kubernetes services
	ServiceUser systeminfo.User
	// GCENodeTags specifies additional VM instance tags on GCE
	GCENodeTags []string
	// LocalClusterClient is a factory for creating client to the installed cluster
	LocalClusterClient func() (*opsclient.Client, error)
	// Operator specifies the wizard's operator service
	Operator ops.Operator
	// Apps specifies the wizard's application service
	Apps app.Applications
	// Packages specifies the wizard's package service
	Packages pack.PackageService
	// LocalAgent specifies whether the installer will also run an agent
	LocalAgent bool
}

// checkAndSetDefaults checks the parameters and autodetects some defaults
func (c *Config) checkAndSetDefaults() (err error) {
	if c.AdvertiseAddr == "" {
		return trace.BadParameter("missing AdvertiseAddr")
	}
	if c.LocalClusterClient == nil {
		return trace.BadParameter("missing LocalClusterClient")
	}
	if c.Apps == nil {
		return trace.BadParameter("missing Apps")
	}
	if c.Packages == nil {
		return trace.BadParameter("missing Packages")
	}
	if c.Operator == nil {
		return trace.BadParameter("missing Operator")
	}
	if c.Process == nil {
		return trace.BadParameter("missing Process")
	}
	if c.LocalPackages == nil {
		return trace.BadParameter("missing LocalPackages")
	}
	if c.LocalApps == nil {
		return trace.BadParameter("missing LocalApps")
	}
	if c.LocalBackend == nil {
		return trace.BadParameter("missing LocalBackend")
	}
	if c.App.Package.IsEmpty() {
		return trace.BadParameter("missing App")
	}
	if c.DNSConfig.IsEmpty() {
		return trace.BadParameter("missing DNSConfig")
	}
	return nil
}

// NewFSMFactory returns a new state machine factory
func NewFSMFactory(config Config) engine.FSMFactory {
	return &fsmFactory{Config: config}
}

// NewStateMachine creates a new state machine for the specified operator and operation.
// Implements engine.FSMFactory
func (r *fsmFactory) NewFSM(operator ops.Operator, operationKey ops.SiteOperationKey) (fsm *fsm.FSM, err error) {
	return NewFSM(NewFSMConfig(operator, operationKey, r.Config))
}

type fsmFactory struct {
	Config
}

// NewClusterFactory returns a factory for creating cluster requests
func NewClusterFactory(config Config) engine.ClusterFactory {
	return &clusterFactory{Config: config}
}

// NewCluster returns a new request to create a cluster.
// Implements engine.ClusterFactory
func (r *clusterFactory) NewCluster() ops.NewSiteRequest {
	return ops.NewSiteRequest{
		AppPackage:   r.App.Package.String(),
		AccountID:    defaults.SystemAccountID,
		Email:        fmt.Sprintf("installer@%v", r.SiteDomain),
		Provider:     r.CloudProvider,
		DomainName:   r.SiteDomain,
		Flavor:       r.Flavor.Name,
		InstallToken: r.Token.Token,
		ServiceUser: storage.OSUser{
			Name: r.ServiceUser.Name,
			UID:  strconv.Itoa(r.ServiceUser.UID),
			GID:  strconv.Itoa(r.ServiceUser.GID),
		},
		CloudConfig: storage.CloudConfig{
			GCENodeTags: r.GCENodeTags,
		},
		DNSOverrides: r.DNSOverrides,
		DNSConfig:    r.DNSConfig,
		Docker:       r.Docker,
	}
}

type clusterFactory struct {
	Config
}

func (r *RuntimeConfig) checkAndSetDefaults() error {
	if err := r.Config.checkAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	if r.FSMFactory == nil {
		return trace.BadParameter("FSMFactory is required")
	}
	if r.ClusterFactory == nil {
		return trace.BadParameter("ClusterFactory is required")
	}
	if r.Planner == nil {
		return trace.BadParameter("Planner is required")
	}
	if r.Engine == nil {
		return trace.BadParameter("Engine is required")
	}
	return nil
}

// RuntimeConfig specifies installer configuration not exposed to the engine
type RuntimeConfig struct {
	// Config is the main configuration for the installer
	Config
	// FSMFactory specifies the state machine factory to use
	FSMFactory engine.FSMFactory
	// ClusterFactory specifies the cluster request factory to use
	ClusterFactory engine.ClusterFactory
	// Planner specifies the plan generator
	Planner engine.Planner
	// Engine specifies the installer flow engine
	Engine Engine
}

// getInstallerTrustedCluster returns trusted cluster representing installer process
func (c *Config) getInstallerTrustedCluster() (storage.TrustedCluster, error) {
	seedConfig := c.Process.Config().OpsCenter.SeedConfig
	if seedConfig == nil {
		return nil, trace.NotFound("expected SeedConfig field to be present "+
			"in the Process configuration: %#v", c.Process.Config())
	}
	for _, tc := range seedConfig.TrustedClusters {
		if tc.GetWizard() {
			return tc, nil
		}
	}
	return nil, trace.NotFound("trusted cluster representing this installer "+
		"is not found in the Process configuration: %#v", seedConfig)
}

// newAgent creates a new unstarted installer agent.
// ctx specifies the context for the duration of the method and is not used beyond that.
// Agent can be started with Serve
func newAgent(ctx context.Context, config Config) (*rpcserver.PeerServer, error) {
	creds, err := LoadRPCCredentials(ctx, config.Packages)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var mounts []*pb.Mount
	for name, source := range config.Mounts {
		mounts = append(mounts, &pb.Mount{Name: name, Source: source})
	}
	runtimeConfig := pb.RuntimeConfig{
		SystemDevice: config.SystemDevice,
		Role:         config.Role,
		Mounts:       mounts,
	}
	return NewAgent(AgentConfig{
		FieldLogger:   config.FieldLogger.WithField(trace.Component, "agent:rpc"),
		CloudProvider: config.CloudProvider,
		AdvertiseAddr: config.AdvertiseAddr,
		ServerAddr:    config.Process.Config().Pack.GetAddr().Addr,
		Credentials:   *creds,
		RuntimeConfig: runtimeConfig,
		ReconnectStrategy: &rpcserver.ReconnectStrategy{
			ShouldReconnect: func(err error) error {
				// Reconnect forever
				return err
			},
		},
	})
}
