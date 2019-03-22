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
	"net"
	"os"
	"path/filepath"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/expand"
	"github.com/gravitational/gravity/lib/install"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/ops/resources"
	"github.com/gravitational/gravity/lib/rpc/proto"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/storage/clusterconfig"
	"github.com/gravitational/gravity/lib/utils"

	teleutils "github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"k8s.io/apimachinery/pkg/runtime"
)

// InstallConfig is the gravity install command configuration
type InstallConfig struct {
	install.Config
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

	// FIXME: unused?
	// License is the cluster License
	// License string
}

// NewInstallConfig creates install config from the passed CLI args and flags
func NewInstallConfig(g *Application) InstallConfig {
	mode := *g.InstallCmd.Mode
	if *g.InstallCmd.Wizard {
		// this is obsolete parameter but take it into account in
		// case somebody is still using it
		mode = constants.InstallModeInteractive
	}

	return InstallConfig{
		Config: install.Config{
			Mode:          mode,
			Insecure:      *g.Insecure,
			StateDir:      *g.InstallCmd.Path,
			UserLogFile:   *g.UserLogFile,
			SystemLogFile: *g.SystemLogFile,
			AdvertiseAddr: *g.InstallCmd.AdvertiseAddr,
			Token:         *g.InstallCmd.Token,
			CloudProvider: *g.InstallCmd.CloudProvider,
			SiteDomain:    *g.InstallCmd.Cluster,
			Flavor:        *g.InstallCmd.Flavor,
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
			DNSConfig:   g.InstallCmd.DNSConfig(),
			Manual:      *g.InstallCmd.Manual,
			GCENodeTags: *g.InstallCmd.GCENodeTags,
		},
		ServiceUID:    *g.InstallCmd.ServiceUID,
		ServiceGID:    *g.InstallCmd.ServiceGID,
		AppPackage:    *g.InstallCmd.App,
		ResourcesPath: *g.InstallCmd.ResourcesPath,
		DNSHosts:      *g.InstallCmd.DNSHosts,
		DNSZones:      *g.InstallCmd.DNSZones,
	}
}

// CheckAndSetDefaults validates the configuration object and populates default values
func (i *InstallConfig) CheckAndSetDefaults() (err error) {
	if i.Config.StateDir == "" {
		if i.Config.StateDir, err = os.Getwd(); err != nil {
			return trace.ConvertSystemError(err)
		}
		log.Infof("Set installer state directory: %v.", i.Config.StateDir)
	}
	if i.Config.WriteStateDir == "" {
		i.Config.WriteStateDir = filepath.Join(os.TempDir(), defaults.WizardStateDir)
		if err := os.MkdirAll(i.Config.WriteStateDir, defaults.SharedDirMask); err != nil {
			return trace.ConvertSystemError(err)
		}
		log.Infof("Installer write layer: %v.", i.Config.WriteStateDir)
	}
	isDir, err := utils.IsDirectory(i.Config.StateDir)
	if !isDir {
		return trace.BadParameter("the specified state path %v is not "+
			"a directory", i.Config.StateDir)
	}
	if err != nil {
		if trace.IsAccessDenied(err) {
			return trace.Wrap(err, "access denied to the specified state "+
				"directory %v", i.Config.StateDir)
		}
		if trace.IsNotFound(err) {
			return trace.Wrap(err, "the specified state directory %v is not "+
				"found", i.Config.StateDir)
		}
		return trace.Wrap(err)
	}
	if i.Config.Token == "" {
		if i.Config.Token, err = teleutils.CryptoRandomHex(6); err != nil {
			return trace.Wrap(err)
		}
		log.Infof("Generated install token: %v.", i.Token)
	}
	serviceUser, err := install.GetOrCreateServiceUser(i.ServiceUID, i.ServiceGID)
	if err != nil {
		return trace.Wrap(err)
	}
	if i.VxlanPort == 0 {
		i.VxlanPort = defaults.VxlanPort
	}
	if err := i.validateDNSConfig(); err != nil {
		return trace.Wrap(err)
	}
	i.Config.ServiceUser = *serviceUser
	return nil
}

// GetAdvertiseAddr return the advertise address provided in the config, or
// asks the user to choose it among the host's interfaces
func (i *InstallConfig) GetAdvertiseAddr() (string, error) {
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

// GetAppPackage returns the application package for this installer
func (i *InstallConfig) GetAppPackage() (*loc.Locator, error) {
	if i.AppPackage != "" {
		return loc.MakeLocator(i.AppPackage)
	}
	env, err := localenv.New(i.StateDir)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer env.Close()
	locator, err := install.GetAppPackage(env.Apps)
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("the specified state dir %v does not "+
				"contain application data, please provide a path to the "+
				"unpacked installer tarball or specify an application "+
				"package via --app flag", i.StateDir)
		}
		return nil, trace.Wrap(err)
	}
	return locator, nil
}

// GetResouces returns additional Kubernetes resources
func (i *InstallConfig) GetResources() ([]byte, error) {
	if i.ResourcesPath == "" {
		return nil, trace.NotFound("no resources provided")
	}
	resources, err := utils.ReadPath(i.ResourcesPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resources, nil
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

// ToInstallerConfig converts CLI config to installer format
func (i *InstallConfig) ToInstallerConfig(env *localenv.LocalEnvironment, validator resources.Validator) (*install.Config, error) {
	advertiseAddr, err := i.GetAdvertiseAddr()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var kubernetesResources []runtime.Object
	var gravityResources []storage.UnknownResource
	if i.ResourcesPath != "" {
		kubernetesResources, gravityResources, err = i.splitResources(validator)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	appPackage, err := i.GetAppPackage()
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
	config := i.Config
	config.AdvertiseAddr = advertiseAddr
	config.AppPackage = appPackage
	config.LocalPackages = env.Packages
	config.LocalApps = env.Apps
	config.LocalBackend = env.Backend
	config.Silent = env.Silent
	config.DNSOverrides = *dnsOverrides
	config.LocalClusterClient = env.SiteOperator
	config.RuntimeResources = kubernetesResources
	config.ClusterResources = gravityResources
	return &config, nil
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
		log.WithField("resource", res.ResourceHeader).Info("Validating.")
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

func validateIP(blocks []net.IPNet, ip net.IP) bool {
	for _, block := range blocks {
		if block.Contains(ip) {
			return true
		}
	}
	return false
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
