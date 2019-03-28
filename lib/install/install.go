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

package install

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/color"
	appservice "github.com/gravitational/gravity/lib/app"
	cloudgce "github.com/gravitational/gravity/lib/cloudprovider/gce"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/modules"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/ops/opsclient"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/process"
	"github.com/gravitational/gravity/lib/rpc"
	rpcserver "github.com/gravitational/gravity/lib/rpc/server"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/status"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/systeminfo"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
)

// New creates a new installer and initializes various services it will
// need based on the provided config
func New(ctx context.Context, config Config) (*Installer, error) {
	err := config.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	wizard, err := localenv.LoginWizard(fmt.Sprintf("https://%v",
		config.Process.Config().Pack.GetAddr().Addr))
	if err != nil {
		return trace.Wrap(err)
	}
	err = upsertSystemAccount(ctx, wizard.Operator)
	if err != nil {
		return trace.Wrap(err)
	}
	token, err := generateInstallToken(wizard.Operator, config.Token)
	if err != nil && !trace.IsAlreadyExists(err) {
		return nil, trace.Wrap(err)
	}
	return &Installer{
		Config:   config,
		Token:    *token,
		Operator: wizard.Operator,
		App:      wizard.Apps,
		Packages: wizard.Packages,
	}, nil
}

// Execute executes the installation using the specified engine
func (i *Installer) Execute(ctx context.Context, engine Engine) error {
	if err := i.upsertAdminUser(); err != nil {
		return trace.Wrap(err)
	}
	if err := engine.Execute(ctx, *i); err != nil {
		return trace.Wrap(err)
	}
	if err := i.finalize(); err != nil {
		i.WithError(err).Warn("Failed to finalize the install.")
	}
	return nil
}

// Stop releases resources allocated by the installer
// and shuts down the agent cluster.
func (i *Installer) Stop(ctx context.Context) error {
	if err := i.Process.AgentService().StopAgents(ctx, i.OperationKey); err != nil {
		return trace.Wrap(err, "failed to stop agents")
	}
	return nil
}

// NewAgent creates a new installer agent
func (i *Installer) NewAgent(agentURL string) (rpcserver.Server, error) {
	listener, err := net.Listen("tcp", defaults.GravityRPCAgentAddr(i.AdvertiseAddr))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	serverCreds, clientCreds, err := rpc.Credentials(defaults.RPCAgentSecretsDir)
	if err != nil {
		listener.Close()
		return nil, trace.Wrap(err)
	}
	var mounts []*pb.Mount
	for name, source := range i.Mounts {
		mounts = append(mounts, &pb.Mount{Name: name, Source: source})
	}
	runtimeConfig := pb.RuntimeConfig{
		SystemDevice: i.SystemDevice,
		DockerDevice: i.DockerDevice,
		Role:         i.Role,
		Mounts:       mounts,
	}
	if err = FetchCloudMetadata(i.CloudProvider, &runtimeConfig); err != nil {
		return nil, trace.Wrap(err)
	}
	config := rpcserver.PeerConfig{
		Config: rpcserver.Config{
			Listener: listener,
			Credentials: rpcserver.Credentials{
				Server: serverCreds,
				Client: clientCreds,
			},
			RuntimeConfig: runtimeConfig,
		},
	}
	agent, err := NewAgent(agentURL, config, i)
	if err != nil {
		listener.Close()
		return nil, trace.Wrap(err)
	}
	return agent, nil
}

func (i *Installer) PrintPostInstallBanner() {
	var buf bytes.Buffer
	i.printEndpoints(&buf)
	if m, ok := modules.Get().(modules.Messager); ok {
		fmt.Fprintf("\n%v\n", m.PostInstallMessage())
	}
	i.Printer.PrintProgress(buf.String())
}

func (i *Installer) printEndpoints(w io.Writer) {
	status, err := i.getClusterStatus()
	if err != nil {
		i.WithError(err).Error("Failed to collect cluster status.")
		return
	}
	fmt.Fprintln(w)
	status.Cluster.Endpoints.Cluster.WriteTo(w)
	fmt.Fprintln(w)
	status.Cluster.Endpoints.Applications.WriteTo(w)
}

// getClusterStatus collects status of the installer cluster.
func (i *Installer) getClusterStatus() (*status.Status, error) {
	clusterOperator, err := localenv.ClusterOperator()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cluster, err := clusterOperator.GetLocalSite()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	status, err := status.FromCluster(i.Context, clusterOperator, *cluster, "")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return status, nil
}

// upsertAdminAgent creates an admin agent for the cluster being installed
func (i *Installer) upsertAdminAgent() error {
	agent, err := i.Process.UsersService().CreateClusterAdminAgent(i.SiteDomain,
		storage.NewUser(storage.ClusterAdminAgent(i.SiteDomain), storage.UserSpecV2{
			AccountID: defaults.SystemAccountID,
		}))
	if err != nil && !trace.IsAlreadyExists(err) {
		return trace.Wrap(err)
	}
	i.WithField("agent", agent).Info("Created cluster agent.")
	return nil
}

func (i *Installer) finalize() error {
	if err := i.uploadInstallLog(); err != nil {
		errors = append(errors, err)
	}
	if err := i.completeFinalInstallStep(); err != nil {
		errors = append(errors, err)
	}
	if err := i.emitAuditEvents(); err != nil {
		errors = append(errors, err)
	}
	return trace.NewAggregate(errors...)
}

// completeFinalInstallStep marks the final install step as completed unless
// the application has a custom install step - in which case it does nothing
// because it will be completed by user later
func (i *Installer) completeFinalInstallStep() error {
	// see if the app defines custom install step
	application, err := i.Apps.GetApp(i.AppPackage)
	if err != nil {
		return trace.Wrap(err)
	}
	// if app has a custom setup endpoint, user will complete it
	if application.Manifest.SetupEndpoint() != nil {
		return nil
	}
	// determine delay for removing connection from installed cluster to this
	// installer process - in case of interactive installs, we can not remove
	// the link right away because it is used to tunnel final install step
	var delay time.Duration
	if i.Mode == constants.InstallModeInteractive {
		delay = defaults.WizardLinkTTL
	}
	req := ops.CompleteFinalInstallStepRequest{
		AccountID:           defaults.SystemAccountID,
		SiteDomain:          i.SiteDomain,
		WizardConnectionTTL: delay,
	}
	i.Debugf("Completing final install step: %s.", req)
	if err := i.Operator.CompleteFinalInstallStep(req); err != nil {
		return trace.Wrap(err, "failed to complete final install step")
	}
	return nil
}

// uploadInstallLog uploads user-facing operation log to the installed cluster
func (i *Installer) uploadInstallLog() error {
	file, err := os.Open(i.UserLogFile)
	if err != nil {
		return trace.Wrap(err)
	}
	defer file.Close()
	err = i.Operator.StreamOperationLogs(i.OperationKey, file)
	if err != nil {
		return trace.Wrap(err, "failed to upload install log")
	}
	i.Debug("Uploaded install log to the cluster.")
	return nil
}

// emitAuditEvents sends the install operation's start/finish
// events to the installed cluster's audit log.
func (i *Installer) emitAuditEvents() error {
	operation, err := i.Operator.GetSiteOperation(i.OperationKey)
	if err != nil {
		return trace.Wrap(err)
	}
	operator, err := localenv.ClusterOperator()
	if err != nil {
		return trace.Wrap(err)
	}
	fields := events.FieldsForOperation(*operation)
	events.Emit(i.Context, operator, events.OperationStarted, fields.WithField(
		events.FieldTime, operation.Created))
	events.Emit(i.Context, operator, events.OperationCompleted, fields)
	return nil
}

// Installer manages the installation process
type Installer struct {
	// Config specifies the configuration for the install operation
	Config
	// Operator specifies the wizard's operator service
	Operator *opsclient.Client
	// Apps specifies the wizard's application service
	Apps appservice.Applications
	// Packages specifies the wizard's packageservice
	Packages pack.PackageService
}

// Engine implements the process of cluster installation
type Engine interface {
	// Execute executes the steps to install a cluster.
	// Config specifies the configuration from command line parameters
	Execute(context.Context, Installer) error
}

// CheckAndSetDefaults checks the parameters and autodetects some defaults
func (c *Config) CheckAndSetDefaults() (err error) {
	if c.AdvertiseAddr == "" {
		return trace.BadParameter("missing AdvertiseAddr")
	}
	if c.LocalClusterClient == nil {
		return trace.BadParameter("missing LocalClusterClient")
	}
	if !utils.StringInSlice(modules.Get().InstallModes(), c.Mode) {
		return trace.BadParameter("invalid Mode %q", c.Mode)
	}
	if err := CheckAddr(c.AdvertiseAddr); err != nil {
		return trace.Wrap(err)
	}
	if err := c.Docker.Check(); err != nil {
		return trace.Wrap(err)
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
	if c.App == nil {
		return trace.BadParameter("missing App")
	}
	if c.Engine == nil {
		return trace.BadParameter("missing Engine")
	}
	if c.VxlanPort < 1 || c.VxlanPort > 65535 {
		return trace.BadParameter("invalid vxlan port: must be in range 1-65535")
	}
	if err := c.validateCloudConfig(); err != nil {
		return trace.Wrap(err)
	}
	if c.SiteDomain == "" {
		c.SiteDomain = generateClusterName()
	}
	if c.DNSConfig.IsEmpty() {
		c.DNSConfig = storage.DefaultDNSConfig
	}
	return nil
}

// NewStateMachine returns a new instance of the installer state machine.
// Implements engine.StateMachineFactory
func (c *Config) NewStateMachine(operator ops.Operator, operationKey ops.SiteOperationKey) (fsm *fsm.FSM, err error) {
	config := install.FSMConfig{
		Operator:           operator,
		OperationKey:       operationKey,
		Packages:           c.Packages,
		Apps:               c.Apps,
		LocalPackages:      c.LocalPackages,
		LocalApps:          c.LocalApps,
		LocalBackend:       c.LocalBackend,
		LocalClusterClient: c.LocalClusterClient,
		Insecure:           c.Insecure,
		UserLogFile:        c.UserLogFile,
		ReportProgress:     true,
	}
	config.FSMSpec = FSMSpec(config)
	return install.NewFSM(config)
}

// NewCluster creates the cluster with the specified operator
// Implements engine.ClusterFactory
func (c *Config) CreateCluster(operator ops.Operator) (fsm *fsm.FSM, err error) {
	req := ossops.NewSiteRequest{
		AppPackage:   c.App.Package.String(),
		AccountID:    c.AccountID,
		Email:        fmt.Sprintf("installer@%v", i.SiteDomain),
		Provider:     c.CloudProvider,
		DomainName:   c.SiteDomain,
		InstallToken: c.Token,
		ServiceUser: storage.OSUser{
			Name: c.ServiceUser.Name,
			UID:  strconv.Itoa(c.ServiceUser.UID),
			GID:  strconv.Itoa(c.ServiceUser.GID),
		},
		CloudConfig: storage.CloudConfig{
			GCENodeTags: c.GCENodeTags,
		},
		DNSOverrides: c.DNSOverrides,
		DNSConfig:    c.DNSConfig,
		Docker:       c.Docker,
	}
	return operator.CreateSite(req)
}

// Config is installer configuration
type Config struct {
	// FieldLogger is used for logging
	log.FieldLogger
	// Printer specifies the output sink for progress messages
	utils.Printer
	// AdvertiseAddr is advertise address of this server
	AdvertiseAddr string
	// Token is install token
	Token string
	// CloudProvider is optional cloud provider
	CloudProvider string
	// StateDir is directory with local installer state
	StateDir string
	// WriteStateDir is installer write layer
	WriteStateDir string
	// UserLogFile is the log file where user-facing operation logs go
	UserLogFile string
	// SystemLogFile is the log file for system logs
	SystemLogFile string
	// SiteDomain is the name of the cluster
	SiteDomain string
	// Flavor is installation flavor
	Flavor *schema.Flavor
	// Role is server role
	Role string
	// App is the application being installed
	App *appservice.Application
	// RuntimeResources specifies optional Kubernetes resources to create
	RuntimeResources []runtime.Object
	// ClusterResources specifies optional cluster resources to create
	// TODO(dmitri): externalize the ClusterConfiguration resource and create
	// default provider-specific cloud-config on Gravity side
	ClusterResources []storage.UnknownResource
	// EventsC is channel with events indicating install progress
	EventsC chan Event
	// SystemDevice is a device for gravity data
	SystemDevice string
	// DockerDevice is a device for docker
	DockerDevice string
	// Mounts is a list of mount points (name -> source pairs)
	Mounts map[string]string
	// DNSOverrides contains installer node DNS overrides
	DNSOverrides storage.DNSOverrides
	// PodCIDR is a pod network CIDR
	PodCIDR string
	// ServiceCIDR is a service network CIDR
	ServiceCIDR string
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
	LocalPackages pack.PackageService
	// LocalApps is the machine-local application service
	LocalApps appservice.Applications
	// LocalBackend is the machine-local backend
	LocalBackend storage.Backend
	// Manual disables automatic phase execution
	// FIXME: is this really necessary?
	// Manual bool
	// ServiceUser specifies the user to use as a service user in planet
	// and for unprivileged kubernetes services
	ServiceUser systeminfo.User
	// GCENodeTags specifies additional VM instance tags on GCE
	GCENodeTags []string
	// LocalClusterClient is a factory for creating client to the installed cluster
	LocalClusterClient func() (*opsclient.Client, error)
}

func (c *Config) validateCloudConfig() (err error) {
	c.CloudProvider, err = ValidateCloudProvider(c.CloudProvider)
	if err != nil {
		return trace.Wrap(err)
	}
	if c.CloudProvider != schema.ProviderGCE {
		return nil
	}
	// TODO(dmitri): skip validations if user provided custom cloud configuration
	if err := cloudgce.ValidateTag(c.SiteDomain); err != nil {
		log.WithError(err).Warnf("Failed to validate cluster name %v as node tag on GCE.", c.SiteDomain)
		if len(c.GCENodeTags) == 0 {
			return trace.BadParameter("specified cluster name %q does "+
				"not conform to GCE tag value specification "+
				"and no node tags have been specified.\n"+
				"Either provide a conforming cluster name or use --gce-node-tag "+
				"to specify the node tag explicitly.\n"+
				"See https://cloud.google.com/vpc/docs/add-remove-network-tags for details.", c.SiteDomain)
		}
	}
	var errors []error
	for _, tag := range c.GCENodeTags {
		if err := cloudgce.ValidateTag(tag); err != nil {
			errors = append(errors, trace.Wrap(err, "failed to validate tag %q", tag))
		}
	}
	if len(errors) != 0 {
		return trace.NewAggregate(errors...)
	}
	// Use cluster name as node tag
	if len(c.GCENodeTags) == 0 {
		c.GCENodeTags = append(c.GCENodeTags, c.SiteDomain)
	}
	return nil
}

func (i *Installer) PrintStep(format string, args ...interface{}) {
	i.Printer.Printf("%v\t%v\n", time.Now().UTC().Format(constants.HumanDateFormatSeconds),
		fmt.Sprintf(format, args...))
}

// timeSinceBeginning returns formatted operation duration
func (i *Installer) timeSinceBeginning(key ops.SiteOperationKey) string {
	operation, err := i.Operator.GetSiteOperation(key)
	if err != nil {
		i.Errorf("Failed to retrieve operation: %v.", trace.DebugReport(err))
		return "<unknown>"
	}
	return time.Since(operation.Created).String()
}
