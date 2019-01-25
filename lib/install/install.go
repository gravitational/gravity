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
	"math/rand"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/color"
	appservice "github.com/gravitational/gravity/lib/app"
	cloudaws "github.com/gravitational/gravity/lib/cloudprovider/aws"
	cloudgce "github.com/gravitational/gravity/lib/cloudprovider/gce"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/httplib"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/modules"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/process"
	"github.com/gravitational/gravity/lib/rpc"
	pb "github.com/gravitational/gravity/lib/rpc/proto"
	rpcserver "github.com/gravitational/gravity/lib/rpc/server"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/state"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/systeminfo"
	"github.com/gravitational/gravity/lib/utils"

	gcemeta "cloud.google.com/go/compute/metadata"
	"github.com/docker/docker/pkg/namesgenerator"
	teleutils "github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/kardianos/osext"
	log "github.com/sirupsen/logrus"
)

// UpsertSystem account creates or updates system account used for all installs
func UpsertSystemAccount(operator ops.Operator) (*ops.Account, error) {
	accounts, err := operator.GetAccounts()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for i := range accounts {
		if accounts[i].Org == defaults.SystemAccountOrg {
			log.Debugf("found account: %v", accounts[i])
			return &accounts[i], nil
		}
	}
	account, err := operator.CreateAccount(ops.NewAccountRequest{
		ID:  defaults.SystemAccountID,
		Org: defaults.SystemAccountOrg,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	log.Debugf("created new account: %v", account)
	return account, nil
}

// Installer is a CLI client that manages installation process
type Installer struct {
	// Config is the installer configuration
	Config
	// FieldLogger is used for structured logging
	log.FieldLogger
	// AccountID is the wizard system account ID
	AccountID string
	// AppPackage is the application being installed
	AppPackage loc.Locator
	// Token is the generated unique install token
	Token storage.InstallToken
	// Operator is the ops service authenticated with wizard process
	Operator ops.Operator
	// Apps is the app service authenticated with wizard process
	Apps appservice.Applications
	// Packages is the pack service authenticated with wizard process
	Packages pack.PackageService
	// OperationKey is the key of the running operation
	OperationKey ops.SiteOperationKey
	// Cluster is the cluster that is being installed
	Cluster *ops.Site
	// engine allows to customize installer behavior
	engine Engine
	// flavor stores the selected install flavor
	flavor *schema.Flavor
	// agentReport stores the last agent report
	agentReport *ops.AgentReport
}

// SetFlavor sets the flavor that will be installed
func (i *Installer) SetFlavor(flavor *schema.Flavor) {
	i.flavor = flavor
}

// Engine defines interface for installer customization
type Engine interface {
	// NewClusterRequest constructs a request to create a new cluster
	NewClusterRequest() ops.NewSiteRequest
	// GetOperationPlan builds a plan for the provided operation
	GetOperationPlan(ops.Site, ops.SiteOperation) (*storage.OperationPlan, error)
	// GetFSM returns the installer FSM engine
	GetFSM() (*fsm.FSM, error)
	// OnPlanComplete is called when install plan finishes execution
	OnPlanComplete(fsm *fsm.FSM, fsmErr error)
	// Cleanup performs post-installation cleanups
	Cleanup(ops.ProgressEntry) error
}

// Config is installer configuration
type Config struct {
	// Context controls state of the installer, e.g. it can be cancelled
	Context context.Context
	// Cancel allows to cancel the context above
	Cancel context.CancelFunc
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
	Flavor string
	// Role is server role
	Role string
	// AppPackage is the application being installed
	AppPackage *loc.Locator
	// Resources is a file with resource specs
	Resources []byte
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
	// Mode is the installation mode (wizard or CLI or via Ops Center)
	Mode string
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
	// LocalApps is the machine-local apps service
	LocalApps appservice.Applications
	// LocalBackend is the machine-local backend
	LocalBackend storage.Backend
	// Manual disables automatic phase execution when set to true
	Manual bool
	// ServiceUser specifies the user to use as a service user in planet
	// and for unprivileged kubernetes services
	ServiceUser systeminfo.User
	// GCENodeTags defines the VM instance tags on GCE
	GCENodeTags []string
	// NewProcess is used to launch gravity API server process
	NewProcess process.NewGravityProcess
	// Silent allows installer to output its progress
	localenv.Silent
}

// CheckAndSetDefaults checks the parameters and autodetects some defaults
func (c *Config) CheckAndSetDefaults() (err error) {
	if c.Context == nil {
		return trace.BadParameter("missing Context")
	}
	if c.EventsC == nil {
		return trace.BadParameter("missing EventsC")
	}
	if c.AdvertiseAddr == "" {
		return trace.BadParameter("missing AdvertiseAddr")
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
	c.CloudProvider, err = ValidateCloudProvider(c.CloudProvider)
	if err != nil {
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
	if c.AppPackage == nil {
		return trace.BadParameter("missing AppPackage")
	}
	if c.SiteDomain == "" {
		rand.Seed(time.Now().UnixNano())
		c.SiteDomain = fmt.Sprintf(
			"%v%d",
			strings.Replace(namesgenerator.GetRandomName(0), "_", "", -1),
			rand.Intn(10000))
	}
	if c.VxlanPort <= 0 || c.VxlanPort > 65535 {
		return trace.BadParameter("invalid vxlan port: must be in range 1-65535")
	}
	if err := c.validateCloudConfig(); err != nil {
		return trace.Wrap(err)
	}
	if c.NewProcess == nil {
		c.NewProcess = process.NewProcess
	}
	if c.DNSConfig.IsEmpty() {
		c.DNSConfig = storage.DefaultDNSConfig
	}
	return nil
}

func (c *Config) validateCloudConfig() error {
	if c.CloudProvider != schema.ProviderGCE {
		return nil
	}
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

// Init creates a new installer and initializes various services it will
// need based on the provided config
func Init(ctx context.Context, cfg Config) (*Installer, error) {
	err := cfg.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	wizard, err := localenv.LoginWizard(fmt.Sprintf("https://%v",
		cfg.Process.Config().Pack.GetAddr().Addr))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	account, err := getSystemAccount(wizard.Operator)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	token, err := generateInstallToken(wizard.Operator, account.ID, cfg.Token)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	installer := &Installer{
		Config:      cfg,
		FieldLogger: log.WithField(trace.Component, "installer"),
		AccountID:   account.ID,
		AppPackage:  *cfg.AppPackage,
		Token:       *token,
		Operator:    wizard.Operator,
		Apps:        wizard.Apps,
		Packages:    wizard.Packages,
	}
	// set the installer engine to itself by default, and external
	// implementations will be able to override it via SetEngine
	installer.SetEngine(installer)
	// initialize the machine's local state to make sure everything's
	// ready for the installation
	err = installer.bootstrap(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// install signal handler
	installer.watchSignals()
	return installer, nil
}

// SetEngine sets the installer engine
func (i *Installer) SetEngine(engine Engine) {
	i.engine = engine
}

// Start starts the installer in the appropriate mode according to its configuration
func (i *Installer) Start() error {
	i.PrintStep("Installing application %v:%v", i.AppPackage.Name, i.AppPackage.Version)
	switch i.Mode {
	case constants.InstallModeCLI:
		i.PrintStep("Starting non-interactive install")
		if err := i.StartCLIInstall(); err != nil {
			return trace.Wrap(err)
		}
	case constants.InstallModeInteractive:
		i.PrintStep("Starting web UI install wizard")
		i.printURL(i.Process.Config().Pack.GetAddr().Addr)
		if err := i.StartInteractiveInstall(); err != nil {
			return trace.Wrap(err)
		}
	default:
		return trace.BadParameter("unknown installer mode %q", i.Mode)
	}
	return nil
}

// printURL prints the URL that installer can be reached at via browser
// in interactive mode to stdout
func (i *Installer) printURL(advertiseAddr string) {
	url := fmt.Sprintf("https://%v/web/installer/new/%v/%v/%v?install_token=%v",
		advertiseAddr,
		i.AppPackage.Repository,
		i.AppPackage.Name,
		i.AppPackage.Version,
		i.Token.Token)
	i.Infof("Installer URL: %v.", url)
	rule := strings.Repeat("-", 100)
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "%v\n", rule)
	fmt.Fprintf(&buf, "%v\n", rule)
	fmt.Fprintf(&buf, "OPEN THIS IN BROWSER: %v\n", url)
	fmt.Fprintf(&buf, "%v\n", rule)
	fmt.Fprintf(&buf, "%v\n", rule)
	fmt.Printf(buf.String())
}

// Stop releases resources allocated by the installer
// and shuts down the agent cluster.
func (i *Installer) Stop(ctx context.Context) error {
	if err := i.Process.AgentService().StopAgents(ctx, i.OperationKey); err != nil {
		return trace.Wrap(err, "failed to stop agents")
	}
	return nil
}

// Wait waits for the install to complete, performs post-installation
// cleanups and shuts down the installer
func (i *Installer) Wait() error {
	for {
		select {
		case <-i.Context.Done():
			return trace.Wrap(i.Context.Err())
		case event := <-i.EventsC:
			if event.Error != nil {
				return trace.Wrap(event.Error)
			}
			progress := event.Progress
			if progress.Message != "" {
				i.PrintStep(progress.Message)
			}
			switch progress.State {
			case ops.ProgressStateCompleted, ops.ProgressStateFailed:
				defer func() {
					select {
					case <-i.Context.Done():
					case <-time.After(10 * time.Second):
						// let agents query the progress before main process exits
					}
				}()
				errCleanup := i.engine.Cleanup(*progress)
				if errCleanup != nil {
					i.Warnf("Installer cleanup failed: %v.",
						trace.DebugReport(errCleanup))
				}
				if progress.State == ops.ProgressStateCompleted {
					i.PrintStep(color.GreenString("Installation succeeded in %v",
						i.timeSinceBeginning(i.OperationKey)))
					if i.Mode == constants.InstallModeInteractive {
						i.printf("\nInstaller process will keep running so the installation can be finished by\n" +
							"completing necessary post-install actions in the installer UI if the installed\n" +
							"application requires it.\n")
						i.printf(color.YellowString("\nOnce no longer needed, press Ctrl-C to shutdown this process.\n"))
						return wait(i.Context, i.Cancel, i.Process)
					}
					return nil
				} else {
					i.PrintStep(color.RedString("Installation failed in %v, "+
						"check %v and %v for details", i.timeSinceBeginning(i.OperationKey), i.UserLogFile, i.SystemLogFile))
					i.printf("\nInstaller process will keep running so you can inspect the operation plan using\n" +
						"`gravity plan` command, see what failed and continue plan execution manually\n" +
						"using `gravity install --phase=<phase-id>` command after fixing the problem.\n")
					i.printf(color.YellowString("\nOnce no longer needed, press Ctrl-C to shutdown this process.\n"))
					return wait(i.Context, i.Cancel, i.Process)
				}
			}
		}
	}
}

func (i *Installer) PrintStep(format string, args ...interface{}) {
	i.printf("%v\t%v\n", time.Now().UTC().Format(constants.HumanDateFormatSeconds),
		fmt.Sprintf(format, args...))
}

func (i *Installer) printf(format string, args ...interface{}) {
	i.Silent.Printf(format, args...)
}

// timeSinceBeginning returns formatted operation duration
func (i *Installer) timeSinceBeginning(key ops.SiteOperationKey) string {
	operation, err := i.Operator.GetSiteOperation(key)
	if err != nil {
		i.Errorf("Failed to retrieve operation: %v.", trace.DebugReport(err))
		return "<unknown>"
	}
	return fmt.Sprintf("%v", time.Since(operation.Created))
}

func getSystemAccount(operator ops.Operator) (account *ops.Account, err error) {
	if err = utils.RetryOnNetworkError(defaults.RetryInterval, defaults.RetryAttempts, func() error {
		account, err = UpsertSystemAccount(operator)
		return trace.Wrap(err)
	}); err != nil {
		return nil, trace.Wrap(err)
	}
	return account, nil
}

func CheckAddr(addr string) error {
	ifaces, err := systeminfo.NetworkInterfaces()
	if err != nil {
		return trace.Wrap(err)
	}
	if len(ifaces) == 0 {
		return trace.NotFound("no network interfaces detected")
	}
	availableAddrs := make([]string, 0, len(ifaces))
	for _, iface := range ifaces {
		if iface.IPv4 == addr {
			return nil
		}
		availableAddrs = append(availableAddrs, iface.IPv4)
	}
	return trace.BadParameter(
		"%v matches none of the available addresses %v",
		addr, strings.Join(availableAddrs, ", "))
}

func generateInstallToken(service ops.Operator, accountID, installToken string) (*storage.InstallToken, error) {
	token, err := service.CreateInstallToken(
		ops.NewInstallTokenRequest{
			AccountID: accountID,
			UserType:  storage.AdminUser,
			UserEmail: defaults.WizardUser,
			Token:     installToken,
		},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return token, nil
}

// GetApp finds the user app in the provided service and returns it
func GetApp(service appservice.Applications) (*appservice.Application, error) {
	apps, err := service.ListApps(appservice.ListAppsRequest{
		Repository: defaults.SystemAccountOrg,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(apps) == 0 {
		return nil, trace.NotFound("no application to install")
	}
	installApp := apps[0]
	// deps is a map of packages that appear as dependencies
	deps := make(map[loc.Locator]struct{})
	for _, app := range apps {
		for _, dep := range app.Manifest.Dependencies.Apps {
			deps[dep.Locator] = struct{}{}
		}
		base := app.Manifest.Base()
		if base != nil {
			deps[*base] = struct{}{}
		}
	}
	for _, app := range apps {
		if _, exists := deps[app.Package]; !exists {
			// Use the app that is not a dependency to any other app as one to install
			installApp = app
			break
		}
	}
	return &installApp, nil
}

// GetAppPackage finds the user app in the provided service and returns its locator
func GetAppPackage(service appservice.Applications) (*loc.Locator, error) {
	app, err := GetApp(service)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &app.Package, nil
}

// GetOrCreateServiceUser returns the user to use for container services.
//
// If the specified ID is empty, a new service user and service group
// (named defaults.ServiceUser/defaults.ServiceGroup) will be created
// with system-allocated IDs.
//
// If the specified ID is not empty, the user is expected to exist.
func GetOrCreateServiceUser(uid, gid string) (user *systeminfo.User, err error) {
	user, err = GetServiceUser(uid)
	if err == nil {
		log.Infof("System user exists: %s.", user)
		return user, nil
	}
	if !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	// Create a new user
	user, err = systeminfo.NewUser(defaults.ServiceUser, defaults.ServiceUserGroup, uid, gid)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	log.Infof("Created system user: %s.", user)
	return user, nil
}

// GetServiceUser retrieves the service user by ID.
// If the specified user ID is empty, the function looks up the user by name
// (defaults.ServiceUser).
func GetServiceUser(uid string) (user *systeminfo.User, err error) {
	if uid != "" {
		var id int
		id, err = strconv.Atoi(uid)
		if err != nil {
			return nil, trace.BadParameter("expected a numeric user ID: %v (%v)", uid, err)
		}
		user, err = systeminfo.LookupUserByUID(id)
		if err != nil && !trace.IsNotFound(err) {
			return nil, trace.Wrap(err, "failed to lookup user by ID %v", id)
		}
	} else {
		user, err = systeminfo.LookupUserByName(defaults.ServiceUser)
		if err != nil && !trace.IsNotFound(err) {
			return nil, trace.Wrap(err,
				"failed to lookup user by name %q and none was specified on command line",
				defaults.ServiceUser)
		}
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return user, nil
}

// EnsureServiceUserAndBinary ensures that the specified user exists on host
// (creating it if it does not).
// It also installs the system binary into the proper binary location
// depending on the OS distribution.
// Returns the new or existing service user as a result
func EnsureServiceUserAndBinary(userID, groupID string) (*systeminfo.User, error) {
	uid, err := strconv.Atoi(userID)
	if err != nil {
		return nil, trace.Wrap(err, "invalid numeric user ID %q for cluster", userID)
	}

	gid, err := strconv.Atoi(groupID)
	if err != nil {
		return nil, trace.Wrap(err, "invalid numeric group ID %q for cluster", groupID)
	}

	user, err := GetOrCreateServiceUser(userID, groupID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = installBinary(uid, gid)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return user, nil
}

// ValidateCloudProvider validates the value of the specified cloud provider.
// If no cloud provider has been specified, the provider is autodetected.
func ValidateCloudProvider(cloudProvider string) (provider string, err error) {
	switch cloudProvider {
	case schema.ProviderAWS, schema.ProvisionerAWSTerraform:
		if !cloudaws.IsRunningOnAWS() {
			return "", trace.BadParameter("cloud provider %q was specified "+
				"but the process does not appear to be running on an AWS "+
				"instance", cloudProvider)
		}
		return schema.ProviderAWS, nil
	case schema.ProviderGCE:
		if !gcemeta.OnGCE() {
			return "", trace.BadParameter("cloud provider %q was specified "+
				"but the process does not appear to be running on a GCE "+
				"instance", cloudProvider)
		}
		return schema.ProviderGCE, nil
	case ops.ProviderGeneric, schema.ProvisionerOnPrem:
		return schema.ProviderOnPrem, nil
	case "":
		// Detect cloud provider
		if cloudaws.IsRunningOnAWS() {
			log.Info("Detected AWS cloud provider.")
			return schema.ProviderAWS, nil
		}
		if gcemeta.OnGCE() {
			log.Info("Detected GCE cloud provider.")
			return schema.ProviderGCE, nil
		}
		log.Info("Detected onprem installation.")
		return schema.ProviderOnPrem, nil
	default:
		return "", trace.BadParameter("unsupported cloud provider %q, "+
			"supported are: %v", cloudProvider, schema.SupportedProviders)
	}
}

// FetchCloudMetadata fetches the metadata for the specified cloud provider
func FetchCloudMetadata(cloudProvider string, config *pb.RuntimeConfig) error {
	var docLink string
	switch cloudProvider {
	case schema.ProviderAWS:
		docLink = "https://gravitational.com/telekube/docs/installation/#aws-credentials-iam-policy"
	case schema.ProviderGCE:
		docLink = "https://gravitational.com/telekube/docs/installation/#installing-on-google-compute-engine"
	default:
		return nil
	}
	metadata, err := rpcserver.GetCloudMetadata(cloudProvider)
	if err != nil {
		log.Error(trace.DebugReport(err))
		return trace.BadParameter("integration with the %v cloud provider has been "+
			"turned on but an attempt to fetch the instance metadata failed with "+
			"the following error: %q.\nCheck the documentation to see the required "+
			"instance permissions (%v) or turn off cloud integration by providing "+
			"--cloud-provider=generic flag", strings.ToUpper(cloudProvider), err, docLink)
	}
	config.CloudMetadata = metadata
	return nil
}

// installBinary places the system binary into the proper binary directory
// depending on the distribution.
// The specified uid/gid pair is used to set user/group permissions on the
// resulting binary
func installBinary(uid, gid int) (err error) {
	for _, targetPath := range state.GravityBinPaths {
		err = tryInstallBinary(targetPath, uid, gid)
		if err == nil {
			break
		}
	}
	if err != nil {
		return trace.Wrap(err, "failed to install gravity binary in any of %v",
			state.GravityBinPaths)
	}
	return nil
}

func tryInstallBinary(targetPath string, uid, gid int) error {
	path, err := osext.Executable()
	if err != nil {
		return trace.Wrap(err, "failed to determine path to binary")
	}
	err = os.MkdirAll(filepath.Dir(targetPath), defaults.SharedDirMask)
	if err != nil {
		return trace.Wrap(err)
	}
	err = utils.CopyFileWithPerms(targetPath, path, defaults.SharedExecutableMask)
	if err != nil {
		return trace.Wrap(err)
	}
	err = os.Chown(targetPath, uid, gid)
	if err != nil {
		return trace.Wrap(trace.ConvertSystemError(err),
			"failed to change ownership on %v", targetPath)
	}
	log.Infof("Installed gravity binary: %v.", targetPath)
	return nil
}

func StartAgent(agentURL string, config rpcserver.PeerConfig, log log.FieldLogger) (*rpcserver.PeerServer, error) {
	log.Debugf("Starting agent: %v.", agentURL)
	u, err := url.ParseRequestURI(agentURL)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	serverAddr, err := teleutils.ParseAddr("tcp://" + u.Host)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	config.RuntimeConfig.Token = u.Query().Get(httplib.AccessTokenQueryParam)
	agent, err := rpcserver.NewPeer(config, serverAddr.Addr, log)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return agent, nil
}

func LoadRPCCredentials(ctx context.Context, packages pack.PackageService, log log.FieldLogger) (*rpcserver.Credentials, error) {
	err := exportRPCCredentials(ctx, packages, log)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	serverCreds, clientCreds, err := rpc.Credentials(defaults.RPCAgentSecretsDir)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &rpcserver.Credentials{
		Server: serverCreds,
		Client: clientCreds,
	}, nil
}

func exportRPCCredentials(ctx context.Context, packages pack.PackageService, log log.FieldLogger) error {
	// retry several times to account for possible transient errors, for
	// example if the target package service is still starting up.
	// Another case would be if joins are started before an installer process
	// in Ops Center-based workflow, in which case the initial package requests
	// will fail with "bad user name or password" and need to be retried.
	//
	// FIXME: this will also mask all other possibly terminal failures (file permission
	// issues, etc.) and will keep the command blocked for the whole interval.
	// Get rid of retry or use a better error classification.
	err := utils.Retry(defaults.RetryInterval, defaults.RetryAttempts, func() error {
		err := pack.Unpack(packages, loc.RPCSecrets,
			defaults.RPCAgentSecretsDir, nil)
		return trace.Wrap(err)
	})
	if err != nil {
		return trace.Wrap(err, "failed to unpack RPC credentials")
	}
	log.Debug("RPC credentials unpacked.")
	return nil
}

func wait(ctx context.Context, cancel context.CancelFunc, p process.GravityProcess) error {
	errC := make(chan error, 1)
	go func() {
		err := p.Wait()
		cancel()
		errC <- err
	}()
	select {
	case err := <-errC:
		return trace.Wrap(err)
	case <-ctx.Done():
		return trace.Wrap(ctx.Err())
	}
}
