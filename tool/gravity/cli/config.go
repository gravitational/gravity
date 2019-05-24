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
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/gravitational/gravity/lib/app"
	appservice "github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/expand"
	"github.com/gravitational/gravity/lib/install"
	installerclient "github.com/gravitational/gravity/lib/install/client"
	installpb "github.com/gravitational/gravity/lib/install/proto"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/modules"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/ops/opsclient"
	"github.com/gravitational/gravity/lib/ops/resources"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/process"
	"github.com/gravitational/gravity/lib/processconfig"
	"github.com/gravitational/gravity/lib/rpc/proto"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/state"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/storage/clusterconfig"
	"github.com/gravitational/gravity/lib/system/environ"
	"github.com/gravitational/gravity/lib/system/signals"
	"github.com/gravitational/gravity/lib/systeminfo"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/docker/docker/pkg/namesgenerator"
	teleutils "github.com/gravitational/teleport/lib/utils"
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
	// LocalPackages is the machine-local package service
	LocalPackages pack.PackageService
	// LocalApps is the machine-local apps service
	LocalApps appservice.Applications
	// LocalBackend is the machine-local backend
	LocalBackend storage.Backend
	// GCENodeTags defines the VM instance tags on GCE
	GCENodeTags []string
	// LocalClusterClient is a factory for creating client to the installed cluster
	LocalClusterClient func() (*opsclient.Client, error)
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
	// ExcludeHostFromCluster specifies whether the host should not be part of the cluster
	ExcludeHostFromCluster bool
	// Printer specifies the output for progress messages
	utils.Printer
	// ProcessConfig specifies the Gravity process configuration
	ProcessConfig *processconfig.Config
	// ServiceUser is the computed service user
	ServiceUser *systeminfo.User
	// FromService specifies whether the process runs in service mode
	FromService bool
	// writeStateDir is the directory where installer stores state for the duration
	// of the operation
	writeStateDir string
}

// NewInstallConfig creates install config from the passed CLI args and flags
func NewInstallConfig(env *localenv.LocalEnvironment, g *Application) InstallConfig {
	mode := *g.InstallCmd.Mode
	if *g.InstallCmd.Wizard {
		// this is obsolete parameter but take it into account in
		// case somebody is still using it
		mode = constants.InstallModeInteractive
	}
	return InstallConfig{
		Insecure:      *g.Insecure,
		StateDir:      *g.InstallCmd.Path,
		UserLogFile:   *g.UserLogFile,
		SystemLogFile: *g.SystemLogFile,
		AdvertiseAddr: *g.InstallCmd.AdvertiseAddr,
		Token:         *g.InstallCmd.Token,
		CloudProvider: *g.InstallCmd.CloudProvider,
		SiteDomain:    *g.InstallCmd.Cluster,
		Role:          *g.InstallCmd.Role,
		SystemDevice:  *g.InstallCmd.SystemDevice,
		DockerDevice:  *g.InstallCmd.DockerDevice,
		Mounts:        *g.InstallCmd.Mounts,
		PodCIDR:       *g.InstallCmd.PodCIDR,
		ServiceCIDR:   *g.InstallCmd.ServiceCIDR,
		VxlanPort:     *g.InstallCmd.VxlanPort,
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

		Mode:                   mode,
		ServiceUID:             *g.InstallCmd.ServiceUID,
		ServiceGID:             *g.InstallCmd.ServiceGID,
		AppPackage:             *g.InstallCmd.App,
		ResourcesPath:          *g.InstallCmd.ResourcesPath,
		DNSHosts:               *g.InstallCmd.DNSHosts,
		DNSZones:               *g.InstallCmd.DNSZones,
		Flavor:                 *g.InstallCmd.Flavor,
		ExcludeHostFromCluster: *g.InstallCmd.ExcludeHostFromCluster,
		FromService:            *g.InstallCmd.FromService,
		Printer:                env,
	}
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
	i.writeStateDir = state.GravityInstallDir()
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
	if i.Token == "" {
		if i.Token, err = teleutils.CryptoRandomHex(6); err != nil {
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
	advertiseAddr, err := i.getAdvertiseAddr()
	if err != nil {
		return trace.Wrap(err)
	}
	i.AdvertiseAddr = advertiseAddr
	i.WithField("addr", advertiseAddr).Info("Set advertise address.")
	if !utils.StringInSlice(modules.Get().InstallModes(), i.Mode) {
		return trace.BadParameter("invalid mode %q", i.Mode)
	}
	i.ServiceUser, err = install.GetOrCreateServiceUser(i.ServiceUID, i.ServiceGID)
	if err != nil {
		return trace.Wrap(err)
	}
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
		SystemLogFile:      i.SystemLogFile,
		CloudProvider:      i.CloudProvider,
		GCENodeTags:        i.GCENodeTags,
		SystemDevice:       i.SystemDevice,
		DockerDevice:       i.DockerDevice,
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
		AppPackage:         &app.Package,
		Flavor:             flavor,
		DNSOverrides:       *dnsOverrides,
		RuntimeResources:   kubernetesResources,
		ClusterResources:   gravityResources,
		Process:            process,
		Apps:               wizard.Apps,
		Packages:           wizard.Packages,
		Operator:           wizard.Operator,
		LocalAgent:         !i.ExcludeHostFromCluster,
	}, nil

}

// getAdvertiseAddr returns the advertise address to use for the ???
// asks the user to choose it among the host's interfaces
func (i *InstallConfig) getAdvertiseAddr() (string, error) {
	// if it was set explicitly with --advertise-addr flag, use it
	if i.AdvertiseAddr != "" {
		return i.AdvertiseAddr, nil
	}
	// otherwise, try to pick an address among machine's interfaces
	addr, err := utils.PickAdvertiseIP()
	if err != nil {
		return "", trace.Wrap(err, "could not pick advertise address among "+
			"the host's network interfaces, please set the advertise address "+
			"via --advertise-addr flag")
	}
	return addr, nil
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
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("the specified state dir %v does not "+
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

// NewWizardConfig returns new configuration for the interactive installer
func NewWizardConfig(env *localenv.LocalEnvironment, g *Application) InstallConfig {
	return InstallConfig{
		Mode:                   constants.InstallModeInteractive,
		Insecure:               *g.Insecure,
		UserLogFile:            *g.UserLogFile,
		StateDir:               *g.WizardCmd.Path,
		SystemLogFile:          *g.SystemLogFile,
		ServiceUID:             *g.WizardCmd.ServiceUID,
		ServiceGID:             *g.WizardCmd.ServiceGID,
		FromService:            *g.WizardCmd.FromService,
		ExcludeHostFromCluster: true,
		Printer:                env,
		LocalPackages:          env.Packages,
		LocalApps:              env.Apps,
		LocalBackend:           env.Backend,
		LocalClusterClient:     env.SiteOperator,
	}
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
	// DockerDevice is device for docker data
	DockerDevice string
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
	// FromService specifies whether the process runs in service mode
	FromService bool
}

// NewJoinConfig populates join configuration from the provided CLI application
func NewJoinConfig(g *Application) JoinConfig {
	return JoinConfig{
		SystemLogFile: *g.SystemLogFile,
		UserLogFile:   *g.UserLogFile,
		PeerAddrs:     *g.JoinCmd.PeerAddr,
		AdvertiseAddr: *g.JoinCmd.AdvertiseAddr,
		ServerAddr:    *g.JoinCmd.ServerAddr,
		Token:         *g.JoinCmd.Token,
		Role:          *g.JoinCmd.Role,
		SystemDevice:  *g.JoinCmd.SystemDevice,
		DockerDevice:  *g.JoinCmd.DockerDevice,
		Mounts:        *g.JoinCmd.Mounts,
		CloudProvider: *g.JoinCmd.CloudProvider,
		OperationID:   *g.JoinCmd.OperationID,
		FromService:   *g.JoinCmd.FromService,
	}
}

// CheckAndSetDefaults validates the configuration and sets default values
func (j *JoinConfig) CheckAndSetDefaults() (err error) {
	j.CloudProvider, err = install.ValidateCloudProvider(j.CloudProvider)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetAdvertiseAddr return the advertise address provided in the config, or
// picks one among the host's interfaces
func (j *JoinConfig) GetAdvertiseAddr() (string, error) {
	// if it was set explicitly with --advertise-addr flag, use it
	if j.AdvertiseAddr != "" {
		return j.AdvertiseAddr, nil
	}
	// otherwise, try to pick an address among machine's interfaces
	addr, err := utils.PickAdvertiseIP()
	if err != nil {
		return "", trace.Wrap(err, "could not pick advertise address among "+
			"the host's network interfaces, please set the advertise address "+
			"via --advertise-addr flag")
	}
	return addr, nil
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
func (j *JoinConfig) GetRuntimeConfig() (*proto.RuntimeConfig, error) {
	config := &proto.RuntimeConfig{
		Token:        j.Token,
		Role:         j.Role,
		SystemDevice: j.SystemDevice,
		DockerDevice: j.DockerDevice,
		Mounts:       convertMounts(j.Mounts),
	}
	err := install.FetchCloudMetadata(j.CloudProvider, config)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return config, nil
}

// NewPeerConfig converts the CLI join configuration to peer configuration
func (j *JoinConfig) NewPeerConfig(env, joinEnv *localenv.LocalEnvironment) (*expand.PeerConfig, error) {
	advertiseAddr, err := j.GetAdvertiseAddr()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	peers, err := j.GetPeers()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	runtimeConfig, err := j.GetRuntimeConfig()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &expand.PeerConfig{
		Peers:         peers,
		AdvertiseAddr: advertiseAddr,
		ServerAddr:    j.ServerAddr,
		CloudProvider: j.CloudProvider,
		RuntimeConfig: *runtimeConfig,
		DebugMode:     env.Debug,
		Insecure:      env.Insecure,
		LocalBackend:  env.Backend,
		LocalApps:     env.Apps,
		LocalPackages: env.Packages,
		JoinBackend:   joinEnv.Backend,
		StateDir:      joinEnv.StateDir,
		OperationID:   j.OperationID,
	}, nil
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
		return installerUninstallSystem(env)
	}
}

// installerUninstallSystem implements the clean up phase when the installer service
// is explicitly interrupted by user
func installerUninstallSystem(env *localenv.LocalEnvironment) func(context.Context) error {
	return func(ctx context.Context) error {
		logger := log.WithField(trace.Component, "installer:abort")
		logger.Info("Leaving cluster.")
		if err := tryLeave(env, leaveConfig{
			confirmed: true,
			force:     true,
		}); err != nil {
			logger.WithError(err).Warn("Failed to leave cluster.")
		}
		serviceName, err := installerclient.GetServicePath(state.GravityInstallDir())
		if err == nil {
			logger := logger.WithField("service", serviceName)
			logger.Info("Uninstalling service.")
			if err := environ.UninstallService(serviceName); err != nil {
				logger.WithError(err).Warn("Failed to uninstall service.")
			}
		}
		logger.Info("Uninstalling system.")
		if err := environ.UninstallSystem(utils.DiscardPrinter, logger); err != nil {
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
	return func(ctx context.Context, interrupt *signals.InterruptHandler, status installpb.ProgressResponse_Status) error {
		logger := log.WithField(trace.Component, "installer:cleanup")
		if status == installpb.StatusCompletedPending {
			// Wait for explicit interrupt signal before cleaning up
			interrupt.Close()
			env.PrintStep(postInstallInteractiveBanner)
			signals.WaitFor(os.Interrupt)
		}
		serviceName, err := installerclient.GetServicePath(state.GravityInstallDir())
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
}

func upsertSystemAccount(operator ops.Operator) error {
	_, err := ops.UpsertSystemAccount(operator)
	return trace.Wrap(err)
}

var postInstallInteractiveBanner = fmt.Sprintln(`
Installer process will keep running so the installation can be finished by
completing necessary post-install actions in the installer UI if the installed
application requires it.
`, color.YellowString("After completing the installation, press Ctrl+C to finish the operation."),
)
