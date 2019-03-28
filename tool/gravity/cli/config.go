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
	"net"
	"os"
	"path/filepath"
	"strconv"

	appservice "github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/expand"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/install"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/ops/resources"
	"github.com/gravitational/gravity/lib/processconfig"
	"github.com/gravitational/gravity/lib/rpc/proto"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/storage/clusterconfig"
	"github.com/gravitational/gravity/lib/utils"

	teleutils "github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
)

// InstallConfig defines the configuration for the install command
type InstallConfig struct {
	logrus.FieldLogger
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
	// AppPackage is the application package to install
	AppPackage string
	// ResourcesPath is the additional Kubernetes resources to create
	ResourcesPath string
	// ServiceUID is the ID of the service user as configured externally
	ServiceUID string
	// ServiceGID is the ID of the service group as configured externally
	ServiceGID string
	// Flavor specifies the installation flavor to use
	Flavor string
	// ExcludeHostFromCluster specifies whether the host should not be part of the cluster
	ExcludeHostFromCluster bool
	// Printer specifies the output for progress messages
	utils.Printer
	// ProcessConfig specifies the Gravity process configuration
	ProcessConfig *processconfig.Config
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
		DNSConfig: g.InstallCmd.DNSConfig(),
		// FIXME
		// Manual:             *g.InstallCmd.Manual,
		GCENodeTags:        *g.InstallCmd.GCENodeTags,
		LocalPackages:      env.Packages,
		LocalApps:          env.Apps,
		LocalBackend:       env.Backend,
		LocalClusterClient: env.SiteOperator,

		Mode:          mode,
		ServiceUID:    *g.InstallCmd.ServiceUID,
		ServiceGID:    *g.InstallCmd.ServiceGID,
		AppPackage:    *g.InstallCmd.App,
		ResourcesPath: *g.InstallCmd.ResourcesPath,
		DNSHosts:      *g.InstallCmd.DNSHosts,
		DNSZones:      *g.InstallCmd.DNSZones,
		Flavor:        *g.InstallCmd.Flavor,
		Printer:       env.Silent,
	}
}

// CheckAndSetDefaults validates the configuration object and populates default values
func (i *InstallConfig) CheckAndSetDefaults() (err error) {
	if i.FieldLogger == nil {
		i.FieldLogger = log.WithField(trace.Component, "installer")
	}
	if i.StateDir == "" {
		if i.StateDir, err = os.Getwd(); err != nil {
			return trace.ConvertSystemError(err)
		}
		i.WithField("dir", i.StateDir).Info("Set installer read state directory.")
	}
	if i.WriteStateDir == "" {
		i.WriteStateDir = filepath.Join(os.TempDir(), defaults.WizardStateDir)
		if err := os.MkdirAll(i.WriteStateDir, defaults.SharedDirMask); err != nil {
			return trace.ConvertSystemError(err)
		}
		i.WithField("dir", i.WriteStateDir).Info("Set installer write state directory.")
	}
	isDir, err := utils.IsDirectory(i.StateDir)
	if !isDir {
		return trace.BadParameter("the specified state path %v is not "+
			"a directory", i.StateDir)
	}
	if err != nil {
		if trace.IsAccessDenied(err) {
			return trace.Wrap(err, "access denied to the specified state "+
				"directory %v", i.StateDir)
		}
		if trace.IsNotFound(err) {
			return trace.Wrap(err, "the specified state directory %v is not "+
				"found", i.StateDir)
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
		return nil, trace.Wrap(err)
	}
	r.AdvertiseAddr = advertiseAddr
	dnsOverrides, err := i.getDNSOverrides()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	i.Config.DNSOverrides = *dnsOverrides
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return nil
}

// ToInstallerConfig returns this configuration as install.Config
func (i *InstallConfig) ToInstallerConfig(validator resources.Validator) (*install.Config, error) {
	var kubernetesResources []runtime.Object
	var gravityResources []storage.UnknownResource
	if i.ResourcesPath != "" {
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
	serviceUser, err := install.GetOrCreateServiceUser(i.ServiceUID, i.ServiceGID)
	if err != nil {
		return trace.Wrap(err)
	}
	flavor, err := getFlavor(i.Flavor, app.Manifest, i.FieldLogger)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	role, err := getRole(i.Role, *flavor, app.Manifest.Installer.NodeProfiles)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &install.Config{
		FieldLogger:        i.FieldLogger,
		AdvertiseAddr:      i.AdvertiseAddr,
		App:                *app,
		LocalPackages:      i.Packages,
		LocalApps:          i.Apps,
		LocalBackend:       i.Backend,
		Printer:            i.Printer,
		SiteDomain:         i.SiteDomain,
		StateDir:           i.ReadStateDir,
		WriteStateDir:      i.WriteStateDir,
		UserLogFile:        i.UserLogFile,
		SystemLogFile:      i.SystemLogFile,
		Token:              i.InstallToken,
		CloudProvider:      i.CloudProvider,
		GCENodeTags:        i.NodeTags,
		SystemDevice:       i.SystemDevice,
		DockerDevice:       i.DockerDevice,
		Mounts:             i.Mounts,
		DNSConfig:          i.DNSConfig,
		Mode:               i.Mode,
		PodCIDR:            i.PodCIDR,
		ServiceCIDR:        i.ServiceCIDR,
		VxlanPort:          i.VxlanPort,
		Docker:             i.Docker,
		Insecure:           i.Insecure,
		LocalClusterClient: i.LocalClusterClient,
		// Manual:             i.Manual,
		Flavor:           flavor,
		Role:             role,
		DNSOverrides:     *dnsOverrides,
		ServiceUser:      *serviceUser,
		RuntimeResources: kubernetesResources,
		ClusterResources: gravityResources,
	}, nil
}

// getAdvertiseAddr return the advertise address provided in the config, or
// asks the user to choose it among the host's interfaces
func (i *InstallConfig) getAdvertiseAddr() (string, error) {
	// if it was set explicitly with --advertise-addr flag, use it
	if i.AdvertiseAddr != "" {
		return i.AdvertiseAddr, nil
	}
	// in interactive install mode ask user to choose among host's interfaces
	if i.Mode == constants.InstallModeInteractive {
		return selectNetworkInterface()
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

// getApp returns the application for this installer
func (i *InstallConfig) getApp() (app *appservice.Application, err error) {
	env, err := localenv.New(i.StateDir)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer env.Close()
	if i.AppPackage != "" {
		app, err = env.Apps.GetApp(i.AppPackage)
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

// JoinConfig is the configuration object built from gravity join command args and flags
type JoinConfig struct {
	// SystemLogFile is gravity-system log file path
	SystemLogFile string
	// UserLogFile is gravity-install log file path
	UserLogFile string
	// AdvertiseAddr is the advertise IP for the joining node
	AdvertiseAddr string
	// ServerAddr is the RPC server address
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
	// OperationID is ID of existing join operation
	OperationID string
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
		Manual:        *g.JoinCmd.Manual,
		Phase:         *g.JoinCmd.Phase,
		OperationID:   *g.JoinCmd.OperationID,
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

// ToPeerConfig converts the CLI join configuration to a peer configuration
func (j *JoinConfig) ToPeerConfig(env, joinEnv *localenv.LocalEnvironment) (*expand.PeerConfig, error) {
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
		Silent:        env.Silent,
		DebugMode:     env.Debug,
		Insecure:      env.Insecure,
		LocalBackend:  env.Backend,
		LocalApps:     env.Apps,
		LocalPackages: env.Packages,
		JoinBackend:   joinEnv.Backend,
		Manual:        j.Manual,
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

func getFlavor(name string, manifest schema.Manifest, logger logrus.FieldLogger) (*schema.Flavor, error) {
	if len(manifest.Flavors.Items) == 0 {
		return nil, trace.NotFound("no install flavors defined in manifest")
	}
	if name == "" {
		if manifest.Installer.Flavors.Default != "" {
			name = flavors.Default
			logger.WithField("flavor", name).Info("Flavor is not set, picking default flavor.")
		} else {
			name = manifest.Flavors.Items[0].Name
			logger.WithField("flavor", name).Info("Flavor is not set, picking first flavor.")
		}
	}
	flavor := manifest.FindFlavor(name)
	if flavor == nil {
		return nil, trace.NotFound("install flavor %q not found", name)
	}
	return flavor, nil
}

func getRole(role string, flavor schema.Flavor, profiles schema.NodeProfiles) (role string, err error) {
	if role == "" {
		for _, node := range flavor.Nodes {
			role = node.Profile
			logger.WithField("role", role).Info("No server profile specified, picking the first.")
			break
		}
	}
	for _, profile := range nodeProfiles {
		if profile.Name == role {
			return role, nil
		}
	}
	return "", trace.NotFound("server role %q is not found", role)
}

func validateIP(blocks []net.IPNet, ip net.IP) bool {
	for _, block := range blocks {
		if block.Contains(ip) {
			return true
		}
	}
	return false
}

func upsertSystemAccount(operator ops.Operator) error {
	b := utils.NewUnlimitedExponentialBackOff()
	ctx = defaults.WithTimeout(context.Background())
	if err := utils.RetryTransient(ctx, b, func() (err error) {
		accounts, err := operator.GetAccounts()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		for i := range accounts {
			if accounts[i].Org == defaults.SystemAccountOrg {
				return &accounts[i], nil
			}
		}
		_, err := operator.CreateAccount(ops.NewAccountRequest{
			ID:  defaults.SystemAccountID,
			Org: defaults.SystemAccountOrg,
		})
		return trace.Wrap(err)
		}
	}); err != nil {
		return trace.Wrap(err)
	}
	return nil
}
