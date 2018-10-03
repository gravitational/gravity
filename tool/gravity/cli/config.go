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
	"io/ioutil"
	"net"
	"os"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/expand"
	"github.com/gravitational/gravity/lib/install"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/process"
	"github.com/gravitational/gravity/lib/rpc/proto"
	rpcserver "github.com/gravitational/gravity/lib/rpc/server"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/systeminfo"
	"github.com/gravitational/gravity/lib/utils"

	teleutils "github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

// InstallConfig is the gravity install command configuration
type InstallConfig struct {
	// Mode is the install mode
	Mode string
	// Insecure allows to turn on certificate validation
	Insecure bool
	// ReadStateDir is the installer state dir
	ReadStateDir string
	// WriteStateDir is the directory where installer writes its state
	WriteStateDir string
	// SystemLogFile is the telekube-system.log file
	SystemLogFile string
	// UserLogFile is the telekube-install.log file
	UserLogFile string
	// AdvertiseAddr is the advertise IP for this node
	AdvertiseAddr string
	// InstallToken is the unique install token
	InstallToken string
	// CloudProvider is the cloud provider
	CloudProvider string
	// License is the cluster License
	License string
	// SiteDomain is the cluster domain name
	SiteDomain string
	// Flavor is the Flavor name to install
	Flavor string
	// Role is this node Role
	Role string
	// ResourcesPath is the additional Kubernetes resources to create
	ResourcesPath string
	// SystemDevice is the block device to use for gravity data
	SystemDevice string
	// DockerDevice is the block device to use for Docker data
	DockerDevice string
	// Mounts is a list of additional app Mounts
	Mounts map[string]string
	// DNSHosts is a list of DNS host overrides
	DNSHosts []string
	// DNSZones is a list of DNS zone overrides
	DNSZones []string
	// DNSConfig is the DNS configuration for planet container DNS.
	DNSConfig storage.DNSConfig
	// PodCIDR is the pod network subnet
	PodCIDR string
	// ServiceCIDR is the service network subnet
	ServiceCIDR string
	// VxlanPort is the overlay network port
	VxlanPort int
	// Docker is the Docker configuration
	Docker storage.DockerConfig
	// Manual allows to execute install plan phases manually
	Manual bool
	// AppPackage is the application package to install
	AppPackage string
	// ServiceUser is the service user configuration
	ServiceUser systeminfo.User
	// ServiceUID is the ID of the service user as configured externally
	ServiceUID string
	// ServiceGID is the ID of the service group as configured externally
	ServiceGID string
	// NodeTags specifies VM instance tags on GCE.
	// Kubernetes uses tags to match instances for load balancing support.
	// By default, the tag is generated based on the cluster name.
	// It can be overridden with this value (i.e. when cluster name does not
	// conform to the GCE tag requirements)
	NodeTags []string
	// NewProcess is used to launch gravity API server process
	NewProcess process.NewGravityProcess
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
		Mode:          mode,
		Insecure:      *g.Insecure,
		ReadStateDir:  *g.InstallCmd.Path,
		UserLogFile:   *g.UserLogFile,
		SystemLogFile: *g.SystemLogFile,
		AdvertiseAddr: *g.InstallCmd.AdvertiseAddr,
		InstallToken:  *g.InstallCmd.Token,
		CloudProvider: *g.InstallCmd.CloudProvider,
		SiteDomain:    *g.InstallCmd.Cluster,
		AppPackage:    *g.InstallCmd.App,
		Flavor:        *g.InstallCmd.Flavor,
		Role:          *g.InstallCmd.Role,
		ResourcesPath: *g.InstallCmd.ResourcesPath,
		SystemDevice:  *g.InstallCmd.SystemDevice,
		DockerDevice:  *g.InstallCmd.DockerDevice,
		Mounts:        *g.InstallCmd.Mounts,
		DNSHosts:      *g.InstallCmd.DNSHosts,
		DNSZones:      *g.InstallCmd.DNSZones,
		PodCIDR:       *g.InstallCmd.PodCIDR,
		ServiceCIDR:   *g.InstallCmd.ServiceCIDR,
		VxlanPort:     *g.InstallCmd.VxlanPort,
		Docker: storage.DockerConfig{
			StorageDriver: g.InstallCmd.DockerStorageDriver.value,
			Args:          *g.InstallCmd.DockerArgs,
		},
		DNSConfig:  g.InstallCmd.DNSConfig(),
		Manual:     *g.InstallCmd.Manual,
		ServiceUID: *g.InstallCmd.ServiceUID,
		ServiceGID: *g.InstallCmd.ServiceGID,
		NodeTags:   *g.InstallCmd.GCENodeTags,
	}
}

// CheckAndSetDefaults validates the configuration object and populates default values
func (i *InstallConfig) CheckAndSetDefaults() (err error) {
	if i.ReadStateDir == "" {
		if i.ReadStateDir, err = os.Getwd(); err != nil {
			return trace.ConvertSystemError(err)
		}
		log.Infof("Set installer state directory: %v.", i.ReadStateDir)
	}
	if i.WriteStateDir == "" {
		if i.WriteStateDir, err = ioutil.TempDir("", "gravity-wizard"); err != nil {
			return trace.ConvertSystemError(err)
		}
		log.Infof("Installer write layer: %v.", i.WriteStateDir)
	}
	isDir, err := utils.IsDirectory(i.ReadStateDir)
	if !isDir {
		return trace.BadParameter("the specified state path %v is not "+
			"a directory", i.ReadStateDir)
	}
	if err != nil {
		if trace.IsAccessDenied(err) {
			return trace.Wrap(err, "access denied to the specified state "+
				"dir %v", i.ReadStateDir)
		}
		if trace.IsNotFound(err) {
			return trace.Wrap(err, "the specified state dir %v is not "+
				"found", i.ReadStateDir)
		}
		return trace.Wrap(err)
	}
	if i.InstallToken == "" {
		if i.InstallToken, err = teleutils.CryptoRandomHex(6); err != nil {
			return trace.Wrap(err)
		}
		log.Infof("Generated install token: %v.", i.InstallToken)
	}
	serviceUser, err := install.GetOrCreateServiceUser(i.ServiceUID, i.ServiceGID)
	if err != nil {
		return trace.Wrap(err)
	}
	if i.VxlanPort == 0 {
		i.VxlanPort = defaults.VxlanPort
	}
	if i.DNSConfig.IsEmpty() {
		i.DNSConfig = storage.DefaultDNSConfig
	}
	if err := i.validateDNSConfig(); err != nil {
		return trace.Wrap(err)
	}
	i.ServiceUser = *serviceUser
	if i.NewProcess == nil {
		i.NewProcess = process.NewProcess
	}
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
		return pack.MakeLocator(i.AppPackage)
	}
	env, err := localenv.New(i.ReadStateDir)
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
				"package via --app flag", i.ReadStateDir)
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
func (i *InstallConfig) ToInstallerConfig(env *localenv.LocalEnvironment) (*install.Config, error) {
	advertiseAddr, err := i.GetAdvertiseAddr()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	resources, err := i.GetResources()
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	appPackage, err := i.GetAppPackage()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	dnsOverrides, err := i.getDNSOverrides()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &install.Config{
		Context:       ctx,
		Cancel:        cancel,
		EventsC:       make(chan install.Event, 100),
		AdvertiseAddr: advertiseAddr,
		Resources:     resources,
		AppPackage:    appPackage,
		LocalPackages: env.Packages,
		LocalApps:     env.Apps,
		LocalBackend:  env.Backend,
		Silent:        env.Silent,
		SiteDomain:    i.SiteDomain,
		StateDir:      i.ReadStateDir,
		WriteStateDir: i.WriteStateDir,
		UserLogFile:   i.UserLogFile,
		SystemLogFile: i.SystemLogFile,
		Token:         i.InstallToken,
		CloudProvider: i.CloudProvider,
		Flavor:        i.Flavor,
		Role:          i.Role,
		SystemDevice:  i.SystemDevice,
		DockerDevice:  i.DockerDevice,
		Mounts:        i.Mounts,
		DNSOverrides:  *dnsOverrides,
		DNSConfig:     i.DNSConfig,
		Mode:          i.Mode,
		PodCIDR:       i.PodCIDR,
		ServiceCIDR:   i.ServiceCIDR,
		VxlanPort:     i.VxlanPort,
		Docker:        i.Docker,
		Insecure:      i.Insecure,
		Manual:        i.Manual,
		ServiceUser:   i.ServiceUser,
		GCENodeTags:   i.NodeTags,
		NewProcess:    i.NewProcess,
	}, nil
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
	// SystemLogFile is telekube-system log file path
	SystemLogFile string
	// UserLogFile is telekube-install log file path
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
	ctx, cancel := context.WithCancel(context.Background())
	return &expand.PeerConfig{
		Context:       ctx,
		Cancel:        cancel,
		Peers:         peers,
		AdvertiseAddr: advertiseAddr,
		ServerAddr:    j.ServerAddr,
		CloudProvider: j.CloudProvider,
		EventsC:       make(chan install.Event, 100),
		WatchCh:       make(chan rpcserver.WatchEvent, 1),
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
