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
	"math/rand"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gravitational/gravity/lib/app"
	appservice "github.com/gravitational/gravity/lib/app"
	autoscaleaws "github.com/gravitational/gravity/lib/autoscale/aws"
	"github.com/gravitational/gravity/lib/checks"
	awscloud "github.com/gravitational/gravity/lib/cloudprovider/aws"
	cloudaws "github.com/gravitational/gravity/lib/cloudprovider/aws"
	cloudgce "github.com/gravitational/gravity/lib/cloudprovider/gce"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/expand"
	"github.com/gravitational/gravity/lib/httplib"
	"github.com/gravitational/gravity/lib/install"
	installerclient "github.com/gravitational/gravity/lib/install/client"
	installpb "github.com/gravitational/gravity/lib/install/proto"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/modules"
	validationpb "github.com/gravitational/gravity/lib/network/validation/proto"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/ops/opsclient"
	"github.com/gravitational/gravity/lib/ops/resources"
	"github.com/gravitational/gravity/lib/pack/localpack"
	"github.com/gravitational/gravity/lib/process"
	"github.com/gravitational/gravity/lib/processconfig"
	"github.com/gravitational/gravity/lib/report"
	"github.com/gravitational/gravity/lib/rpc/proto"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/state"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/storage/clusterconfig"
	"github.com/gravitational/gravity/lib/system/environ"
	libselinux "github.com/gravitational/gravity/lib/system/selinux"
	"github.com/gravitational/gravity/lib/system/signals"
	"github.com/gravitational/gravity/lib/systeminfo"
	"github.com/gravitational/gravity/lib/systemservice"
	"github.com/gravitational/gravity/lib/users"
	"github.com/gravitational/gravity/lib/utils"
	"github.com/gravitational/gravity/lib/utils/helm"
	"github.com/gravitational/satellite/monitoring"
	"github.com/opencontainers/selinux/go-selinux"

	gcemeta "cloud.google.com/go/compute/metadata"
	"github.com/cenkalti/backoff"
	"github.com/docker/docker/pkg/namesgenerator"
	"github.com/fatih/color"
	"github.com/gravitational/configure"
	teledefaults "github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
)

// InstallConfig defines the configuration for the install command
type InstallConfig struct {
	logrus.FieldLogger
	// AdvertiseAddr is advertise address of this server.
	// Also specifies the address advertised as wizard service endpoint
	AdvertiseAddr string
	// Token is install token
	Token string
	// CloudProvider is optional cloud provider
	CloudProvider string
	// StateDir is directory with local installer state
	StateDir string
	// SystemStateDir specifies the custom state directory.
	// If specified, will affect the local file contexts generated
	// when SELinux configuration is bootstrapped
	SystemStateDir string
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
	AppPackage string
	// RuntimeResources specifies optional Kubernetes resources to create
	// If specified, will be combined with Resources
	RuntimeResources []runtime.Object
	// ClusterResources specifies optional cluster resources to create
	// If specified, will be combined with Resources
	// TODO(dmitri): externalize the ClusterConfiguration resource and create
	// default provider-specific cloud-config on Gravity side
	ClusterResources []storage.UnknownResource
	// SystemDevice is a device for gravity data
	SystemDevice string
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
	// LocalPackages is the machine-local package service
	LocalPackages *localpack.PackageServer
	// LocalApps is the machine-local apps service
	LocalApps appservice.Applications
	// LocalBackend is the machine-local backend
	LocalBackend storage.Backend
	// GCENodeTags defines the VM instance tags on GCE
	GCENodeTags []string
	// LocalClusterClient is a factory for creating client to the installed cluster
	LocalClusterClient func(...httplib.ClientOption) (*opsclient.Client, error)
	// Mode specifies the installer mode
	Mode string
	// DNSHosts is a list of DNS host overrides
	DNSHosts []string
	// DNSZones is a list of DNS zone overrides
	DNSZones []string
	// ResourcesPath is the additional Kubernetes resources to create
	ResourcesPath string
	// ServiceUID is the ID of the service user as configured externally
	ServiceUID string
	// ServiceGID is the ID of the service group as configured externally
	ServiceGID string
	// Remote specifies whether the installer executes the operation remotely
	// (i.e. installer node will not be part of cluster)
	Remote bool
	// Printer specifies the output for progress messages
	utils.Printer
	// ProcessConfig specifies the Gravity process configuration
	ProcessConfig *processconfig.Config
	// ServiceUser is the computed service user
	ServiceUser *systeminfo.User
	// SELinux specifies whether the installer runs with SELinux support.
	// This makes the installer run in its own domain
	SELinux bool
	// FromService specifies whether the process runs in service mode
	FromService bool
	// AcceptEULA allows to auto-accept end-user license agreement.
	AcceptEULA bool
	// writeStateDir is the directory where installer stores state for the duration
	// of the operation
	writeStateDir string
	// Values are helm values in marshaled yaml format
	Values []byte
}

// NewReconfigureConfig creates config for the reconfigure operation.
//
// Reconfiguration is very similar to initial installation so the install
// config is reused.
func NewReconfigureConfig(env *localenv.LocalEnvironment, g *Application) (*InstallConfig, error) {
	// The installer is using the existing state directory in order to be able
	// to use existing application packages.
	stateDir, err := state.GetStateDir()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &InstallConfig{
		Insecure:           *g.Insecure,
		StateDir:           state.GravityLocalDir(stateDir),
		UserLogFile:        *g.UserLogFile,
		SystemLogFile:      *g.SystemLogFile,
		AdvertiseAddr:      *g.StartCmd.AdvertiseAddr,
		FromService:        *g.StartCmd.FromService,
		LocalPackages:      env.Packages,
		LocalApps:          env.Apps,
		LocalBackend:       env.Backend,
		LocalClusterClient: env.SiteOperator,
		Mode:               constants.InstallModeCLI,
		Printer:            env,
	}, nil
}

// Apply updates the config with the data found from the cluster/operation.
func (c *InstallConfig) Apply(cluster storage.Site, operation storage.SiteOperation) {
	c.SiteDomain = cluster.Domain
	c.AppPackage = cluster.App.Locator().String()
	c.CloudProvider = cluster.Provider
	c.PodCIDR = operation.Vars().OnPrem.PodCIDR
	c.ServiceCIDR = operation.Vars().OnPrem.ServiceCIDR
	c.VxlanPort = operation.Vars().OnPrem.VxlanPort
	c.ServiceUID = cluster.ServiceUser.UID
	c.ServiceGID = cluster.ServiceUser.GID
}

// NewInstallConfig creates install config from the passed CLI args and flags
func NewInstallConfig(env *localenv.LocalEnvironment, g *Application) (*InstallConfig, error) {
	mode := *g.InstallCmd.Mode
	if *g.InstallCmd.Wizard {
		// this is obsolete parameter but take it into account in
		// case somebody is still using it
		mode = constants.InstallModeInteractive
	}
	values, err := helm.Vals(*g.InstallCmd.Values, *g.InstallCmd.Set, nil, nil, "", "", "")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &InstallConfig{
		Insecure:       *g.Insecure,
		SystemStateDir: *g.StateDir,
		StateDir:       *g.InstallCmd.Path,
		UserLogFile:    *g.UserLogFile,
		SystemLogFile:  *g.SystemLogFile,
		AdvertiseAddr:  *g.InstallCmd.AdvertiseAddr,
		Token:          *g.InstallCmd.Token,
		CloudProvider:  *g.InstallCmd.CloudProvider,
		SiteDomain:     *g.InstallCmd.Cluster,
		Role:           *g.InstallCmd.Role,
		SystemDevice:   *g.InstallCmd.SystemDevice,
		Mounts:         *g.InstallCmd.Mounts,
		PodCIDR:        *g.InstallCmd.PodCIDR,
		ServiceCIDR:    *g.InstallCmd.ServiceCIDR,
		VxlanPort:      *g.InstallCmd.VxlanPort,
		Docker: storage.DockerConfig{
			StorageDriver: g.InstallCmd.DockerStorageDriver.value,
			Args:          *g.InstallCmd.DockerArgs,
		},
		DNSConfig:          g.InstallCmd.DNSConfig(),
		GCENodeTags:        *g.InstallCmd.GCENodeTags,
		LocalPackages:      env.Packages,
		LocalApps:          env.Apps,
		LocalBackend:       env.Backend,
		LocalClusterClient: env.SiteOperator,
		Mode:               mode,
		ServiceUID:         *g.InstallCmd.ServiceUID,
		ServiceGID:         *g.InstallCmd.ServiceGID,
		AppPackage:         *g.InstallCmd.App,
		ResourcesPath:      *g.InstallCmd.ResourcesPath,
		DNSHosts:           *g.InstallCmd.DNSHosts,
		DNSZones:           *g.InstallCmd.DNSZones,
		Flavor:             *g.InstallCmd.Flavor,
		Remote:             *g.InstallCmd.Remote,
		SELinux:            *g.InstallCmd.SELinux,
		FromService:        *g.InstallCmd.FromService,
		AcceptEULA:         *g.InstallCmd.AcceptEULA,
		Values:             values,
		Printer:            env,
	}, nil
}

// CheckAndSetDefaults validates the configuration object and populates default values
func (i *InstallConfig) CheckAndSetDefaults() (err error) {
	if i.FieldLogger == nil {
		i.FieldLogger = log.WithField(trace.Component, "installer")
	}
	if i.StateDir == "" {
		i.StateDir = filepath.Dir(utils.Exe.Path)
		i.WithField("dir", i.StateDir).Info("Set installer read state directory.")
	}
	i.writeStateDir, err = state.GravityInstallDir()
	if err != nil {
		return trace.Wrap(err)
	}
	if err := os.MkdirAll(i.writeStateDir, defaults.SharedDirMask); err != nil {
		return trace.ConvertSystemError(err)
	}
	i.WithField("dir", i.writeStateDir).Info("Set installer write state directory.")
	isDir, err := utils.IsDirectory(i.StateDir)
	if !isDir {
		return trace.BadParameter("specified state path %v is not a directory",
			i.StateDir)
	}
	if err != nil {
		if trace.IsAccessDenied(err) {
			return trace.Wrap(err, "access denied to the specified state "+
				"directory %v", i.StateDir)
		}
		if trace.IsNotFound(err) {
			return trace.Wrap(err, "specified state directory %v is not found",
				i.StateDir)
		}
		return trace.Wrap(err)
	}
	if i.Token != "" {
		if len(i.Token) < teledefaults.MinPasswordLength {
			return trace.BadParameter("install token is too short, min length is %v",
				teledefaults.MinPasswordLength)
		}
		if len(i.Token) > teledefaults.MaxPasswordLength {
			return trace.BadParameter("install token is too long, max length is %v",
				teledefaults.MaxPasswordLength)
		}
	} else {
		if i.Token, err = newRandomInstallTokenText(); err != nil {
			return trace.Wrap(err)
		}
		i.WithField("token", i.Token).Info("Generated install token.")
	}
	if i.VxlanPort == 0 {
		i.VxlanPort = defaults.VxlanPort
	}
	if err := i.validateDNSConfig(); err != nil {
		return trace.Wrap(err)
	}
	if i.AdvertiseAddr == "" {
		i.AdvertiseAddr, err = selectAdvertiseAddr()
		if err != nil {
			return trace.Wrap(err)
		}
	}
	if err := checkLocalAddr(i.AdvertiseAddr); err != nil {
		return trace.Wrap(err)
	}
	i.WithField("addr", i.AdvertiseAddr).Info("Set advertise address.")
	if err := i.Docker.Check(); err != nil {
		return trace.Wrap(err)
	}
	if i.VxlanPort < 1 || i.VxlanPort > 65535 {
		return trace.BadParameter("invalid vxlan port: must be in range 1-65535")
	}
	if !utils.StringInSlice(modules.Get().InstallModes(), i.Mode) {
		return trace.BadParameter("invalid mode %q", i.Mode)
	}
	i.ServiceUser, err = install.GetOrCreateServiceUser(i.ServiceUID, i.ServiceGID)
	if err != nil {
		return trace.Wrap(err)
	}
	err = i.validateApplicationDir()
	if err != nil {
		return trace.Wrap(err, "failed to validate installer directory. "+
			"Make sure you're running the installer from the directory with the contents "+
			"of the installer tarball")
	}
	if i.DNSConfig.IsEmpty() {
		i.DNSConfig = storage.DefaultDNSConfig
	}
	err = i.checkEULA()
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// checkEULA asks the user to accept the end-user license agreement if one
// is required.
func (i *InstallConfig) checkEULA() error {
	if i.Mode != constants.InstallModeCLI || i.FromService {
		return nil
	}
	app, err := i.getApp()
	if err != nil {
		return trace.Wrap(err)
	}
	eula := app.Manifest.EULA()
	if eula == "" {
		return nil
	}
	if i.AcceptEULA {
		i.Info("EULA was auto-accepted due to --accept-eula flag set.")
		return nil
	}
	i.Printer.Println(eula)
	confirmed, err := confirmWithTitle("Do you accept the end-user license agreement?")
	if err != nil {
		return trace.Wrap(err)
	}
	if !confirmed {
		i.Warn("User did not accept EULA.")
		return trace.BadParameter("end-user license agreement was not accepted")
	}
	i.Info("User accepted EULA.")
	return nil
}

// NewProcessConfig returns new gravity process configuration for this configuration object
func (i *InstallConfig) NewProcessConfig() (*processconfig.Config, error) {
	config, err := install.NewProcessConfig(install.ProcessConfig{
		AdvertiseAddr: i.AdvertiseAddr,
		StateDir:      i.StateDir,
		WriteStateDir: i.writeStateDir,
		LogFile:       i.UserLogFile,
		ServiceUser:   *i.ServiceUser,
		ClusterName:   i.SiteDomain,
		Devmode:       i.Insecure,
		Token:         i.Token,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return config, nil
}

// NewInstallerConfig returns new installer configuration for this configuration object
func (i *InstallConfig) NewInstallerConfig(
	env *localenv.LocalEnvironment,
	wizard *localenv.RemoteEnvironment,
	process process.GravityProcess,
	validator resources.Validator,
) (*install.Config, error) {
	var kubernetesResources []runtime.Object
	var gravityResources []storage.UnknownResource
	if i.ResourcesPath != "" {
		var err error
		kubernetesResources, gravityResources, err = i.splitResources(validator)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	app, err := i.getApp()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	dnsOverrides, err := i.getDNSOverrides()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	gravityResources, err = i.updateClusterConfig(gravityResources)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	flavor, err := getFlavor(i.Flavor, app.Manifest, i.FieldLogger)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	i.Role, err = validateRole(i.Role, *flavor, app.Manifest.NodeProfiles, i.FieldLogger)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !i.Remote {
		if err := i.validateCloudConfig(app.Manifest); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	token, err := generateInstallToken(wizard.Operator, i.Token)
	if err != nil && !trace.IsAlreadyExists(err) {
		return nil, trace.Wrap(err)
	}
	if err := upsertSystemAccount(wizard.Operator); err != nil {
		return nil, trace.Wrap(err)
	}
	if i.SiteDomain == "" {
		i.SiteDomain = generateClusterName()
	}
	return &install.Config{
		FieldLogger:        i.FieldLogger,
		AdvertiseAddr:      i.AdvertiseAddr,
		LocalPackages:      i.LocalPackages,
		LocalApps:          i.LocalApps,
		LocalBackend:       i.LocalBackend,
		Printer:            i.Printer,
		SiteDomain:         i.SiteDomain,
		StateDir:           i.StateDir,
		WriteStateDir:      i.writeStateDir,
		UserLogFile:        i.UserLogFile,
		CloudProvider:      i.CloudProvider,
		GCENodeTags:        i.GCENodeTags,
		SystemDevice:       i.SystemDevice,
		Mounts:             i.Mounts,
		DNSConfig:          i.DNSConfig,
		PodCIDR:            i.PodCIDR,
		ServiceCIDR:        i.ServiceCIDR,
		VxlanPort:          i.VxlanPort,
		Docker:             i.Docker,
		Insecure:           i.Insecure,
		LocalClusterClient: i.LocalClusterClient,
		Role:               i.Role,
		ServiceUser:        *i.ServiceUser,
		Token:              *token,
		App:                app,
		Flavor:             flavor,
		DNSOverrides:       *dnsOverrides,
		RuntimeResources:   kubernetesResources,
		ClusterResources:   gravityResources,
		Process:            process,
		Apps:               wizard.Apps,
		Packages:           wizard.Packages,
		Operator:           wizard.Operator,
		LocalAgent:         !i.Remote,
		Values:             i.Values,
		SELinux:            i.SELinux,
	}, nil

}

// RunLocalChecks executes host-local preflight checks for this configuration
func (i *InstallConfig) RunLocalChecks() error {
	if i.Mode == constants.InstallModeInteractive {
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	interrupt := signals.WatchTerminationSignals(ctx, cancel, utils.DiscardPrinter)
	defer interrupt.Close()

	app, err := i.getApp()
	if err != nil {
		return trace.Wrap(err)
	}
	flavor, err := getFlavor(i.Flavor, app.Manifest, i.FieldLogger)
	if err != nil {
		return trace.Wrap(err)
	}
	role, err := validateRole(i.Role, *flavor, app.Manifest.NodeProfiles, i.FieldLogger)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(checks.RunLocalChecks(ctx, checks.LocalChecksRequest{
		Manifest: app.Manifest,
		Role:     role,
		Docker:   i.Docker,
		Options: &validationpb.ValidateOptions{
			VxlanPort: int32(i.VxlanPort),
			DnsAddrs:  i.DNSConfig.Addrs,
			DnsPort:   int32(i.DNSConfig.Port),
		},
		AutoFix: true,
	}))
}

// BootstrapSELinux configures SELinux on a node prior to installation
func (i *InstallConfig) BootstrapSELinux(ctx context.Context, printer utils.Printer) error {
	logger := log.WithField(trace.Component, "selinux")
	if !i.shouldBootstrapSELinux() {
		if !i.FromService {
			logger.Info("SELinux disabled with configuration.")
		}
		return nil
	}
	metadata, err := monitoring.GetOSRelease()
	if err != nil {
		return trace.Wrap(err)
	}
	if !selinux.GetEnabled() {
		logger.Info("SELinux not enabled on host.")
		i.SELinux = false
	} else if !libselinux.IsSystemSupported(metadata.ID) {
		logger.WithField("id", metadata.ID).Info("Distribution not supported.")
		i.SELinux = false
	}
	return BootstrapSELinuxAndRespawn(ctx, libselinux.BootstrapConfig{
		StateDir: i.StateDir,
		OS:       metadata,
	}, printer)
}

func (i *InstallConfig) shouldBootstrapSELinux() bool {
	return i.SELinux && !(i.FromService || i.Mode == constants.InstallModeInteractive || i.Remote)
}

func (i *InstallConfig) validateApplicationDir() error {
	_, err := i.getApp()
	return trace.Wrap(err)
}

// getApp returns the application package for this installer
func (i *InstallConfig) getApp() (app *app.Application, err error) {
	env, err := localenv.NewLocalEnvironment(localenv.LocalEnvironmentArgs{
		StateDir:        i.StateDir,
		ReadonlyBackend: true,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer env.Close()
	if i.AppPackage != "" {
		loc, err := loc.MakeLocator(i.AppPackage)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		app, err = env.Apps.GetApp(*loc)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return app, nil
	}
	app, err = install.GetApp(env.Apps)
	if err != nil {
		i.WithError(err).Warn("Failed to find application package.")
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("specified state directory %v does not "+
				"contain application data, please provide a path to the "+
				"unpacked installer tarball or specify an application "+
				"package via --app flag", i.StateDir)
		}
		return nil, trace.Wrap(err)
	}
	return app, nil
}

// getDNSOverrides converts DNS overrides specified on CLI to the storage format
func (i *InstallConfig) getDNSOverrides() (*storage.DNSOverrides, error) {
	overrides := &storage.DNSOverrides{
		Hosts: make(map[string]string),
		Zones: make(map[string][]string),
	}
	for _, hostOverride := range i.DNSHosts {
		host, ip, err := utils.ParseHostOverride(hostOverride)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		overrides.Hosts[host] = ip
	}
	for _, zoneOverride := range i.DNSZones {
		zone, nameserver, err := utils.ParseZoneOverride(zoneOverride)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		overrides.Zones[zone] = append(overrides.Zones[zone], nameserver)
	}
	return overrides, nil
}

// splitResources validates the resources specified in ResourcePath
// using the given validator and splits them into Kubernetes and Gravity-specific
func (i *InstallConfig) splitResources(validator resources.Validator) (runtimeResources []runtime.Object, clusterResources []storage.UnknownResource, err error) {
	if i.ResourcesPath == "" {
		return nil, nil, trace.NotFound("no resources provided")
	}
	rc, err := utils.ReaderForPath(i.ResourcesPath)
	if err != nil {
		return nil, nil, trace.Wrap(err, "failed to read resources")
	}
	defer rc.Close()
	// TODO(dmitri): validate kubernetes resources as well
	runtimeResources, clusterResources, err = resources.Split(rc)
	if err != nil {
		return nil, nil, trace.BadParameter("failed to validate %q: %v", i.ResourcesPath, err)
	}
	for _, res := range clusterResources {
		i.WithField("resource", res.ResourceHeader).Info("Validating.")
		if err := validator.Validate(res); err != nil {
			return nil, nil, trace.Wrap(err, "resource %q is invalid", res.Kind)
		}
	}
	return runtimeResources, clusterResources, nil
}

func (i *InstallConfig) updateClusterConfig(resources []storage.UnknownResource) (updated []storage.UnknownResource, err error) {
	var clusterConfig *storage.UnknownResource
	updated = resources[:0]
	for _, res := range resources {
		if res.Kind == storage.KindClusterConfiguration {
			clusterConfig = &res
			continue
		}
		updated = append(updated, res)
	}
	if clusterConfig == nil && i.CloudProvider == "" {
		// Return the resources unchanged
		return resources, nil
	}
	var config clusterconfig.Interface
	if clusterConfig == nil {
		config = clusterconfig.New(clusterconfig.Spec{
			Global: &clusterconfig.Global{CloudProvider: i.CloudProvider},
		})
	} else {
		config, err = clusterconfig.Unmarshal(clusterConfig.Raw)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	if config := config.GetGlobalConfig(); config != nil {
		if config.CloudProvider != "" {
			i.CloudProvider = config.CloudProvider
		}
	}
	// Serialize the cluster configuration and add to resources
	configResource, err := clusterconfig.ToUnknown(config)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	updated = append(updated, *configResource)
	return updated, nil
}

func (i *InstallConfig) validateDNSConfig() error {
	blocks, err := utils.LocalIPNetworks()
	if err != nil {
		return trace.Wrap(err)
	}
	for _, addr := range i.DNSConfig.Addrs {
		ip := net.ParseIP(addr)
		if !validateIP(blocks, ip) {
			return trace.BadParameter(
				"IP address %v does not belong to any local IP network", addr)
		}
	}
	return nil
}

func (i *InstallConfig) validateCloudConfig(manifest schema.Manifest) (err error) {
	i.CloudProvider, err = i.validateOrDetectCloudProvider(i.CloudProvider, manifest)
	if err != nil {
		return trace.Wrap(err)
	}
	if i.CloudProvider != schema.ProviderGCE {
		return nil
	}
	if i.SiteDomain == "" {
		return nil
	}
	// TODO(dmitri): skip validations if user provided custom cloud configuration
	if err := cloudgce.ValidateTag(i.SiteDomain); err != nil {
		log.WithError(err).Warnf("Failed to validate cluster name %v as node tag on GCE.", i.SiteDomain)
		if len(i.GCENodeTags) == 0 {
			return trace.BadParameter("specified cluster name %q does "+
				"not conform to GCE tag value specification "+
				"and no node tags have been specified.\n"+
				"Either provide a conforming cluster name or use --gce-node-tag "+
				"to specify the node tag explicitly.\n"+
				"See https://cloud.google.com/vpc/docs/add-remove-network-tags for details.", i.SiteDomain)
		}
	}
	var errors []error
	for _, tag := range i.GCENodeTags {
		if err := cloudgce.ValidateTag(tag); err != nil {
			errors = append(errors, trace.Wrap(err, "failed to validate tag %q", tag))
		}
	}
	if len(errors) != 0 {
		return trace.NewAggregate(errors...)
	}
	// Use cluster name as node tag
	if len(i.GCENodeTags) == 0 {
		i.GCENodeTags = append(i.GCENodeTags, i.SiteDomain)
	}
	return nil
}

// NewWizardConfig returns new configuration for the interactive installer
func NewWizardConfig(env *localenv.LocalEnvironment, g *Application) (*InstallConfig, error) {
	values, err := helm.Vals(*g.WizardCmd.Values, *g.WizardCmd.Set, nil, nil, "", "", "")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &InstallConfig{
		Mode:               constants.InstallModeInteractive,
		Insecure:           *g.Insecure,
		UserLogFile:        *g.UserLogFile,
		StateDir:           *g.WizardCmd.Path,
		SystemLogFile:      *g.SystemLogFile,
		ServiceUID:         *g.WizardCmd.ServiceUID,
		ServiceGID:         *g.WizardCmd.ServiceGID,
		AdvertiseAddr:      *g.WizardCmd.AdvertiseAddr,
		Token:              *g.WizardCmd.Token,
		FromService:        *g.WizardCmd.FromService,
		Remote:             true,
		Printer:            env,
		LocalPackages:      env.Packages,
		LocalApps:          env.Apps,
		LocalBackend:       env.Backend,
		LocalClusterClient: env.SiteOperator,
		Values:             values,
	}, nil
}

// JoinConfig describes command line configuration of the join command
type JoinConfig struct {
	// SystemLogFile is gravity-system log file path
	SystemLogFile string
	// UserLogFile is gravity-install log file path
	UserLogFile string
	// AdvertiseAddr is the advertise IP for the joining node
	AdvertiseAddr string
	// ServerAddr is the installer RPC server address as host:port
	ServerAddr string
	// PeerAddrs is the list of peers to try connecting to
	PeerAddrs string
	// Token is the join token
	Token string
	// Role is the joining node profile
	Role string
	// SystemDevice is device for gravity data
	SystemDevice string
	// Mounts is a list of additional mounts
	Mounts map[string]string
	// CloudProvider is the node cloud provider
	CloudProvider string
	// Manual turns on manual plan execution mode
	Manual bool
	// Phase is the plan phase to execute
	Phase string
	// OperationID is ID of existing expand operation
	OperationID string
	// SELinux specifies whether the installer runs with SELinux support.
	// This makes the installer run in its own domain
	SELinux bool
	// FromService specifies whether the process runs in service mode
	FromService bool
	// SkipWizard specifies to the join agents that this join request is not too a wizard,
	// and as such wizard connectivity should be skipped
	SkipWizard bool
	// SystemStateDir specifies the custom state directory.
	// If specified, will affect the local file contexts generated
	// when SELinux configuration is bootstrapped
	SystemStateDir string
}

// NewJoinConfig populates join configuration from the provided CLI application
func NewJoinConfig(g *Application) JoinConfig {
	return JoinConfig{
		SystemLogFile:  *g.SystemLogFile,
		UserLogFile:    *g.UserLogFile,
		PeerAddrs:      *g.JoinCmd.PeerAddr,
		AdvertiseAddr:  *g.JoinCmd.AdvertiseAddr,
		ServerAddr:     *g.JoinCmd.ServerAddr,
		Token:          *g.JoinCmd.Token,
		Role:           *g.JoinCmd.Role,
		SystemDevice:   *g.JoinCmd.SystemDevice,
		Mounts:         *g.JoinCmd.Mounts,
		OperationID:    *g.JoinCmd.OperationID,
		SELinux:        *g.JoinCmd.SELinux,
		FromService:    *g.JoinCmd.FromService,
		SystemStateDir: *g.StateDir,
	}
}

// CheckAndSetDefaults validates the configuration and sets default values
func (j *JoinConfig) CheckAndSetDefaults() (err error) {
	if j.AdvertiseAddr == "" {
		j.AdvertiseAddr, err = selectAdvertiseAddr()
		if err != nil {
			return trace.Wrap(err)
		}
	}
	if err := checkLocalAddr(j.AdvertiseAddr); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// NewPeerConfig converts the CLI join configuration to peer configuration
func (j *JoinConfig) NewPeerConfig(env, joinEnv *localenv.LocalEnvironment) (config *expand.PeerConfig, err error) {
	peers, err := j.GetPeers()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &expand.PeerConfig{
		Peers:              peers,
		AdvertiseAddr:      j.AdvertiseAddr,
		ServerAddr:         j.ServerAddr,
		RuntimeConfig:      j.GetRuntimeConfig(),
		DebugMode:          env.Debug,
		Insecure:           env.Insecure,
		LocalBackend:       env.Backend,
		LocalApps:          env.Apps,
		LocalPackages:      env.Packages,
		LocalClusterClient: env.SiteOperator,
		JoinBackend:        joinEnv.Backend,
		StateDir:           joinEnv.StateDir,
		OperationID:        j.OperationID,
		SkipWizard:         j.SkipWizard,
	}, nil
}

// GetPeers returns a list of peers parsed from the peers CLI argument
func (j *JoinConfig) GetPeers() ([]string, error) {
	peers, err := utils.ParseAddrList(j.PeerAddrs)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(peers) == 0 {
		return nil, trace.BadParameter("peers list can't be empty")
	}
	return peers, nil
}

// GetRuntimeConfig returns the RPC agent runtime configuration
func (j *JoinConfig) GetRuntimeConfig() proto.RuntimeConfig {
	return proto.RuntimeConfig{
		Token:        j.Token,
		Role:         j.Role,
		SystemDevice: j.SystemDevice,
		Mounts:       convertMounts(j.Mounts),
		SELinux:      j.SELinux,
	}
}

// bootstrapSELinux configures SELinux on a node prior to join
func (j *JoinConfig) bootstrapSELinux(ctx context.Context, printer utils.Printer) error {
	logger := log.WithField(trace.Component, "selinux")
	if !j.shouldBootstrapSELinux() {
		if !j.FromService {
			logger.Info("SELinux disabled with configuration.")
		}
		return nil
	}
	metadata, err := monitoring.GetOSRelease()
	if err != nil {
		return trace.Wrap(err)
	}
	if !selinux.GetEnabled() {
		logger.Info("SELinux not enabled on host.")
		j.SELinux = false
	} else if !libselinux.IsSystemSupported(metadata.ID) {
		logger.WithField("id", metadata.ID).Info("Distribution not supported.")
		j.SELinux = false
	}
	return BootstrapSELinuxAndRespawn(ctx, libselinux.BootstrapConfig{
		StateDir: j.SystemStateDir,
		OS:       metadata,
	}, printer)
}

func (j *JoinConfig) shouldBootstrapSELinux() bool {
	return j.SELinux && !j.FromService
}

func (r *removeConfig) checkAndSetDefaults() error {
	if r.server == "" {
		return trace.BadParameter("server flag is required")
	}
	return nil
}

type removeConfig struct {
	server    string
	force     bool
	confirmed bool
}

func (j *autojoinConfig) newJoinConfig() JoinConfig {
	return JoinConfig{
		SystemLogFile: j.systemLogFile,
		UserLogFile:   j.userLogFile,
		Role:          j.role,
		SystemDevice:  j.systemDevice,
		Mounts:        j.mounts,
		AdvertiseAddr: j.advertiseAddr,
		PeerAddrs:     j.serviceURL,
		Token:         j.token,
		FromService:   j.fromService,
		SELinux:       j.seLinux,
		// Autojoin can only join an existing cluster, so skip attempts to use the wizard
		SkipWizard: true,
	}
}

func (r *autojoinConfig) checkAndSetDefaults() error {
	if r.advertiseAddr == "" {
		return trace.BadParameter("advertise address is required")
	}
	if err := checkLocalAddr(r.advertiseAddr); err != nil {
		return trace.Wrap(err)
	}
	if r.serviceURL == "" {
		return trace.BadParameter("service URL is required")
	}
	if r.token == "" {
		return trace.BadParameter("token is required")
	}
	return nil
}

// bootstrapSELinux configures SELinux on a node prior to join
func (j *autojoinConfig) bootstrapSELinux(ctx context.Context, printer utils.Printer) error {
	logger := log.WithField(trace.Component, "selinux")
	if !j.shouldBootstrapSELinux() {
		if !j.fromService {
			logger.Info("SELinux disabled with configuration.")
		}
		return nil
	}
	metadata, err := monitoring.GetOSRelease()
	if err != nil {
		return trace.Wrap(err)
	}
	if !selinux.GetEnabled() {
		logger.Info("SELinux not enabled on host.")
		j.seLinux = false
	} else if !libselinux.IsSystemSupported(metadata.ID) {
		logger.WithField("id", metadata.ID).Info("Distribution not supported.")
		j.seLinux = false
	}
	return BootstrapSELinuxAndRespawn(ctx, libselinux.BootstrapConfig{
		OS: metadata,
	}, printer)
}

func (j *autojoinConfig) shouldBootstrapSELinux() bool {
	return j.seLinux && !j.fromService
}

type autojoinConfig struct {
	systemLogFile string
	userLogFile   string
	clusterName   string
	role          string
	systemDevice  string
	mounts        map[string]string
	fromService   bool
	serviceURL    string
	advertiseAddr string
	token         string
	seLinux       bool
}

func (r *agentConfig) newServiceArgs(gravityPath string) (args []string) {
	args = []string{gravityPath, "--debug", "ops", "agent", r.packageAddr,
		"--advertise-addr", r.advertiseAddr,
		"--server-addr", r.serverAddr,
		"--token", r.token,
		"--service-uid", r.serviceUID,
		"--service-gid", r.serviceGID,
	}
	if r.systemLogFile != "" {
		args = append(args, "--system-log-file", r.systemLogFile)
	}
	if r.userLogFile != "" {
		args = append(args, "--log-file", r.userLogFile)
	}
	if r.cloudProvider != "" {
		args = append(args, "--cloud-provider", r.cloudProvider)
	}
	if len(r.vars) != 0 {
		args = append(args, "--vars", r.vars.String())
	}
	return args
}

func (r *agentConfig) checkAndSetDefaults() (err error) {
	if r.serviceUID == "" {
		return trace.BadParameter("service user ID is required")
	}
	if r.serviceGID == "" {
		return trace.BadParameter("service group ID is required")
	}
	if r.packageAddr == "" {
		return trace.BadParameter("package service address is required")
	}
	if r.advertiseAddr == "" {
		return trace.BadParameter("advertise address is required")
	}
	if r.serverAddr == "" {
		return trace.BadParameter("server address is required")
	}
	if r.token == "" {
		return trace.BadParameter("token is required")
	}
	if r.cloudProvider == "" {
		return trace.BadParameter("cloud provider is required")
	}
	if r.serviceName != "" {
		r.serviceName = systemservice.FullServiceName(r.serviceName)
	}
	return nil
}

type agentConfig struct {
	systemLogFile string
	serviceName   string
	userLogFile   string
	advertiseAddr string
	serverAddr    string
	packageAddr   string
	token         string
	vars          configure.KeyVal
	serviceUID    string
	serviceGID    string
	cloudProvider string
}

func retryUpdateJoinConfigFromCloudMetadata(ctx context.Context, config *autojoinConfig) error {
	// Security: Insecure is OK here because we're only testing reachability to the cluster
	client := httplib.GetClient(true,
		httplib.WithTimeout(5*time.Second),
		httplib.WithInsecure())

	b := backoff.NewConstantBackOff(15 * time.Second)
	f := func() error {
		err := updateJoinConfigFromCloudMetadata(ctx, config)
		if err != nil && !trace.IsRetryError(err) {
			return &backoff.PermanentError{Err: err}
		}

		// Test that the serviceURL is reachable
		// When re-installing a cluster into AWS, the serviceURL can point to an old cluster
		// until the new cluster overwrites the SSM values. So test the ServiceURL is reachable before using it.
		req, err := http.NewRequest("GET", config.serviceURL, nil)
		if err != nil {
			return trace.Wrap(err)
		}
		// TODO(Knisbet) replace with NewRequestWithContext when on golang 1.13
		req = req.WithContext(ctx)
		_, err = client.Do(req)
		if err != nil {
			return trace.Wrap(err, "Waiting for service URL to become available.").
				AddField("service_url", config.serviceURL)
		}

		return trace.Wrap(err)
	}
	return trace.Wrap(utils.RetryWithInterval(ctx, b, f))
}

func updateJoinConfigFromCloudMetadata(ctx context.Context, config *autojoinConfig) error {
	instance, err := cloudaws.NewLocalInstance()
	if err != nil {
		log.WithError(err).Warn("Failed to fetch instance metadata on AWS.")
		return trace.BadParameter("autojoin only supports AWS")
	}
	config.advertiseAddr = instance.PrivateIP

	autoscaler, err := autoscaleaws.New(autoscaleaws.Config{
		ClusterName: config.clusterName,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	config.serviceURL, err = autoscaler.GetServiceURL(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	config.token, err = autoscaler.GetJoinToken(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func convertMounts(mounts map[string]string) (result []*proto.Mount) {
	result = make([]*proto.Mount, 0, len(mounts))
	for name, source := range mounts {
		result = append(result, &proto.Mount{Name: name, Source: source})
	}
	return result
}

func getFlavor(name string, manifest schema.Manifest, logger logrus.FieldLogger) (flavor *schema.Flavor, err error) {
	flavors := manifest.Installer.Flavors
	if len(flavors.Items) == 0 {
		return nil, trace.NotFound("no install flavors defined in manifest")
	}
	if name == "" {
		if flavors.Default != "" {
			name = flavors.Default
			logger.WithField("flavor", name).Info("Flavor is not set, picking default flavor.")
		} else {
			name = flavors.Items[0].Name
			logger.WithField("flavor", name).Info("Flavor is not set, picking first flavor.")
		}
	}
	flavor = manifest.FindFlavor(name)
	if flavor == nil {
		return nil, trace.NotFound("install flavor %q not found", name)
	}
	return flavor, nil
}

func validateRole(role string, flavor schema.Flavor, profiles schema.NodeProfiles, logger logrus.FieldLogger) (string, error) {
	if role == "" {
		for _, node := range flavor.Nodes {
			role = node.Profile
			logger.WithField("role", role).Info("No server profile specified, picking the first.")
			break
		}
	}
	for _, profile := range profiles {
		if profile.Name == role {
			return role, nil
		}
	}
	return "", trace.NotFound("server role %q not found", role)
}

func validateIP(blocks []net.IPNet, ip net.IP) bool {
	for _, block := range blocks {
		if block.Contains(ip) {
			return true
		}
	}
	return false
}

func newRandomInstallTokenText() (token string, err error) {
	token, err = users.CryptoRandomToken(defaults.InstallTokenBytes)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return token, nil
}

func generateInstallToken(operator ops.Operator, installToken string) (*storage.InstallToken, error) {
	token, err := operator.CreateInstallToken(
		ops.NewInstallTokenRequest{
			AccountID: defaults.SystemAccountID,
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

func generateClusterName() string {
	rand.Seed(time.Now().UnixNano())
	return fmt.Sprintf(
		"%v%d",
		strings.Replace(namesgenerator.GetRandomName(0), "_", "", -1),
		rand.Intn(10000))
}

// AborterForMode returns the Aborter implementation specific to given installation mode
func AborterForMode(mode string, env *localenv.LocalEnvironment) func(context.Context) error {
	switch mode {
	case constants.InstallModeInteractive:
		return installerInteractiveUninstallSystem(env)
	default:
		return installerAbortOperation(env)
	}
}

// installerAbortOperation implements the clean up phase when the installer service
// is explicitly interrupted by user
func installerAbortOperation(env *localenv.LocalEnvironment) func(context.Context) error {
	return func(ctx context.Context) error {
		logger := log.WithField(trace.Component, "installer:abort")
		logger.Info("Leaving cluster.")
		stateDir, err := state.GravityInstallDir()
		if err != nil {
			return trace.Wrap(err)
		}
		serviceName, err := environ.GetServicePath(stateDir)
		if err == nil {
			logger := logger.WithField("service", serviceName)
			logger.Info("Uninstalling service.")
			if err := environ.UninstallService(serviceName); err != nil {
				logger.WithError(err).Warn("Failed to uninstall service.")
			}
		}
		logger.Info("Uninstalling system.")
		if err := environ.UninstallSystem(ctx, utils.DiscardPrinter, logger); err != nil {
			logger.WithError(err).Warn("Failed to uninstall system.")
		}
		logger.Info("System uninstalled.")
		return nil
	}
}

// installerInteractiveUninstallSystem implements the clean up phase when the interactive installer service
// is explicitly interrupted by user
func installerInteractiveUninstallSystem(env *localenv.LocalEnvironment) func(context.Context) error {
	return func(ctx context.Context) error {
		logger := log.WithFields(logrus.Fields{
			trace.Component: "installer:abort",
			"service":       defaults.GravityRPCInstallerServiceName,
		})
		logger.Info("Uninstalling service.")
		if err := environ.UninstallService(defaults.GravityRPCInstallerServiceName); err != nil {
			logger.WithError(err).Warn("Failed to uninstall service.")
		}
		if err := environ.CleanupOperationState(utils.DiscardPrinter, logger); err != nil {
			logger.WithError(err).Warn("Failed to clean up operation state.")
		}
		logger.Info("System uninstalled.")
		return nil
	}
}

// InstallerCompleteOperation implements the clean up phase when the installer service
// shuts down after a sucessfully completed operation
func InstallerCompleteOperation(env *localenv.LocalEnvironment) installerclient.CompletionHandler {
	return func(ctx context.Context, status installpb.ProgressResponse_Status) error {
		logger := log.WithField(trace.Component, "installer:cleanup")
		if status == installpb.StatusCompletedPending {
			// Wait for explicit interrupt signal before cleaning up
			env.PrintStep(postInstallInteractiveBanner)
			signals.WaitFor(os.Interrupt)
		}
		return trace.Wrap(installerCleanup(logger))
	}
}

// InstallerCleanup uninstalls the services and cleans up operation state
func InstallerCleanup() error {
	return installerCleanup(log.WithField(trace.Component, "installer:cleanup"))
}

// InstallerGenerateLocalReport creates a host-local debug report in the specified file
func InstallerGenerateLocalReport(env *localenv.LocalEnvironment) func(context.Context, string) error {
	return func(ctx context.Context, path string) error {
		return systemReport(env, report.AllFilters, true, path, time.Duration(0))
	}
}

func installerCleanup(logger logrus.FieldLogger) error {
	stateDir, err := state.GravityInstallDir()
	if err != nil {
		return trace.Wrap(err)
	}
	serviceName, err := environ.GetServicePath(stateDir)
	logger.WithField("service", serviceName).Info("Uninstalling service.")
	if err == nil {
		if err := environ.UninstallService(serviceName); err != nil {
			logger.WithError(err).Warn("Failed to uninstall agent services.")
		}
	}
	if err := environ.CleanupOperationState(utils.DiscardPrinter, logger); err != nil {
		logger.WithError(err).Warn("Failed to clean up operation state.")
	}
	return nil
}

// checkLocalAddr verifies that addr specifies one of the local interfaces
func checkLocalAddr(addr string) error {
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

// selectAdvertiseAddr selects an advertise address from one of the host's interfaces
func selectAdvertiseAddr() (string, error) {
	// otherwise, try to pick an address among machine's interfaces
	addr, err := utils.PickAdvertiseIP()
	if err != nil {
		return "", trace.Wrap(err, "could not pick advertise address among "+
			"the host's network interfaces, please set the advertise address "+
			"via --advertise-addr flag")
	}
	return addr, nil
}

// validateOrDetectCloudProvider validates the value of the specified cloud provider.
// If no cloud provider has been specified, the provider is autodetected.
func (i *InstallConfig) validateOrDetectCloudProvider(cloudProvider string, manifest schema.Manifest) (provider string, err error) {
	// If cloud provider wasn't explicitly specified on the CLI, see if there
	// is a default one specified in the manifest.
	if cloudProvider != "" {
		log.Infof("Will use provider %q specified on the CLI.", cloudProvider)
	} else {
		cloudProvider = manifest.DefaultProvider()
		if cloudProvider != "" {
			log.Infof("Will use default provider %q from manifest.", cloudProvider)
		}
	}
	switch cloudProvider {
	case schema.ProviderAWS, schema.ProvisionerAWSTerraform:
		runningOnAWS, err := awscloud.IsRunningOnAWS()
		if err != nil {
			// TODO: fallthrough instead of failing to keep backwards compat
			log.WithError(err).Warn("Failed to determine whether running on AWS.")
		}
		if !runningOnAWS {
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
		log.Info("Will auto-detect provider.")
		// Detect cloud provider
		runningOnAWS, err := awscloud.IsRunningOnAWS()
		if err != nil {
			// TODO: fallthrough instead of failing to keep backwards compat
			log.WithError(err).Warn("Failed to determine whether running on AWS.")
		}
		if runningOnAWS {
			log.Info("Detected AWS cloud provider.")
			return schema.ProviderAWS, nil
		}
		if gcemeta.OnGCE() {
			log.Info("Detected GCE cloud provider.")
			return schema.ProviderGCE, nil
		}
		log.Info("No cloud provider detected, will use generic.")
		return schema.ProviderOnPrem, nil
	default:
		return "", trace.BadParameter("unsupported cloud provider %q, "+
			"supported are: %v", cloudProvider, schema.SupportedProviders)
	}
}

func upsertSystemAccount(operator ops.Operator) error {
	_, err := ops.UpsertSystemAccount(operator)
	return trace.Wrap(err)
}

var postInstallInteractiveBanner = color.YellowString(`
Installer process will keep running so the installation can be finished by
completing necessary post-install actions in the installer UI if the installed
application requires it.
After completing the installation, press Ctrl+C to finish the operation.`)
