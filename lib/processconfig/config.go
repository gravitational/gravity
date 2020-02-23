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

package processconfig

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/helm"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/modules"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/storage/keyval"
	"github.com/gravitational/gravity/lib/systeminfo"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/configure"
	telecfg "github.com/gravitational/teleport/lib/config"
	teleservices "github.com/gravitational/teleport/lib/services"
	teleutils "github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// ReadConfig reads gravity OpsCenter or Site configuration directory
// if path is empty, it looks in default locations.
func ReadConfig(configDir string) (*Config, *telecfg.FileConfig, error) {
	var searchPaths []string
	// if configDir is explicitly set, use only this as a search path
	if configDir != "" {
		searchPaths = []string{configDir}
	} else {
		searchPaths = defaults.GravityConfigDirs
	}

	log.Debugf("got search paths: %v", searchPaths)
	for _, path := range searchPaths {
		log.Debugf("look up configs in %v", path)

		gravityName := filepath.Join(path, defaults.GravityYAMLFile)
		data, err := teleutils.ReadPath(gravityName)
		if err != nil {
			if !trace.IsNotFound(err) && !trace.IsAccessDenied(err) {
				return nil, nil, trace.Wrap(err)
			}
			log.Debugf("%v not found in search path", gravityName)
			continue
		}

		var cfg Config
		if err := configure.ParseYAML(data, &cfg, configure.EnableTemplating()); err != nil {
			return nil, nil, trace.Wrap(err)
		}
		if err := configure.ParseEnv(&cfg); err != nil {
			return nil, nil, trace.Wrap(err)
		}

		teleportName := filepath.Join(path, "teleport.yaml")
		data, err = teleutils.ReadPath(teleportName)
		if err != nil {
			if !trace.IsNotFound(err) && !trace.IsAccessDenied(err) {
				return nil, nil, trace.Wrap(err)
			}
			log.Debugf("%v not found in search path", teleportName)
			continue
		}

		teleportCfg, err := telecfg.ReadConfig(bytes.NewBuffer(data))
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		if err := MergeConfigFromEnv(&cfg); err != nil {
			return nil, nil, trace.Wrap(err)
		}
		if err := MergeTeleConfigFromEnv(teleportCfg); err != nil {
			return nil, nil, trace.Wrap(err)
		}
		return &cfg, teleportCfg, nil
	}

	return nil, nil, trace.NotFound("no configuration found in directories %v", searchPaths)
}

// Config is a gravity specific file config
type Config struct {
	Hostname string `yaml:"hostname"`

	// Mode specifies operation mode, either opscenter or site
	Mode string `yaml:"mode"`

	// Profile specifies performance profiling settings
	Profile ProfileConfig `yaml:"profile"`

	// Devmode defines a development mode that takes several shortcuts in favor
	// of simplicity.
	// In this mode the following things are different:
	//  - web assets are assumed in certain static locations (and not as a package)
	//  - SSL traffic uses self-signed certificates
	Devmode bool `yaml:"devmode"`

	// ClusterName is used in wizard mode to indicate the name of the cluster
	// that is being installed
	ClusterName string `yaml:"-"`

	// WebAssetsDir defines the location of the web assets.
	// If unspecified, defaults to:
	//   * web/dist (relative to current directory, when Devmode == true)
	//   * GravityWebAssetsDir
	WebAssetsDir string `yaml:"web_assets_dir"`

	// DataDir is a directory with local database and package files
	DataDir string `yaml:"data_dir"`

	// HealthAddr provides HTTP API for health and readiness checks
	HealthAddr teleutils.NetAddr `yaml:"health_addr"`

	// BackendType is a type of storage backend
	BackendType string `yaml:"backend_type"`

	// ETCD provides etcd config options
	ETCD keyval.ETCDConfig `yaml:"etcd"`

	// OpsCenter provides settings for OpsCenter
	OpsCenter OpsCenterConfig `yaml:"ops"`

	// Pack provides settings for package service
	Pack PackageServiceConfig `yaml:"pack"`

	// Charts is Helm chart repository configuration.
	Charts ChartsConfig `yaml:"charts"`

	// Users list allows to add registered users to the application
	// e.g. application admins, what is handy for development purposes
	Users Users `yaml:"users"`

	// InstallLogFiles is a list of install log files with user
	// facing diagnostic logs
	InstallLogFiles []string `yaml:"install_log_files"`

	// ImportDir specifies optional directory with bootstrap data.
	//
	// An instance of gravity working in site mode will use this location
	// to bootstrap itself with the data from Ops Center
	ImportDir string `yaml:"-"`

	// ServiceUser specifies the service user to use for wizard-based installation.
	ServiceUser *systeminfo.User `yaml:"-"`

	// InstallToken specifies the authentication token for the install operation
	InstallToken string `yaml:"-"`
}

func (cfg *Config) CheckAndSetDefaults() error {
	if len(cfg.DataDir) == 0 {
		return trace.BadParameter("empty DataDir")
	}
	if cfg.Mode == "" {
		cfg.Mode = constants.ComponentSite
	}
	if !utils.StringInSlice(modules.Get().ProcessModes(), cfg.Mode) {
		return trace.BadParameter("unsupported process mode: %q", cfg.Mode)
	}

	if err := os.MkdirAll(cfg.DataDir, defaults.SharedDirMask); err != nil {
		return trace.Wrap(err)
	}
	if len(cfg.Pack.ReadDir) != 0 {
		if err := os.MkdirAll(cfg.Pack.ReadDir, defaults.SharedDirMask); err != nil {
			return trace.Wrap(err)
		}
	}

	if cfg.Pack.GetAddr().Addr == "" {
		return trace.BadParameter("missing pack service advertise address")
	}

	if cfg.HealthAddr.IsEmpty() {
		cfg.HealthAddr = teleutils.NetAddr{
			AddrNetwork: "tcp",
			Addr:        defaults.HealthListenAddr,
		}
	}

	switch cfg.BackendType {
	case "", constants.BoltBackend:
		cfg.BackendType = constants.BoltBackend
	case constants.ETCDBackend:
		if err := cfg.ETCD.Check(); err != nil {
			log.Errorf("error reading config: %#v", cfg.ETCD)
			return trace.Wrap(err)
		}
	default:
		return trace.BadParameter("unsupported backend type: %v", cfg.BackendType)
	}

	// Set default service user if unspecified
	if cfg.ServiceUser == nil {
		cfg.ServiceUser = systeminfo.DefaultServiceUser()
	}

	if err := cfg.Charts.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (cfg Config) CreateBackend() (backend storage.Backend, err error) {
	switch cfg.BackendType {
	case constants.BoltBackend:
		log.Debug("using bolt backend")
		backend, err = keyval.NewBolt(keyval.BoltConfig{
			Path:  filepath.Join(cfg.DataDir, defaults.GravityDBFile),
			Multi: true,
		})
	case constants.ETCDBackend:
		log.Debug("using ETCD backend")
		backend, err = keyval.NewETCD(cfg.ETCD)
	}
	return backend, trace.Wrap(err)
}

func (cfg Config) ProcessID() string {
	id := os.Getenv(constants.EnvPodIP)
	if id == "" {
		// ':' is not handled well by HTTP protocol (e.g. basic auth gets confused)
		id = strings.Replace(cfg.Pack.GetAddr().Addr, ":", "_", -1)
	}
	return id
}

// WizardAddr returns the address of the wizard endpoint
func (cfg Config) WizardAddr() (addr string) {
	return fmt.Sprintf("https://%v", cfg.Pack.GetAddr().Addr)
}

// ProfileConfig is a profile configuration
type ProfileConfig struct {
	// HTTPEndpoint is HTTP profile endpoint
	HTTPEndpoint string `yaml:"http_endpoint"`
	// OutputDir is where profiler will put samples
	OutputDir string `yaml:"output_dir"`
}

type Identity struct {
	Email       string `yaml:"email" json:"email"`
	ConnectorID string `yaml:"connector_id" json:"connector_id"`
}

func (i *Identity) Parse() (*teleservices.ExternalIdentity, error) {
	out := &teleservices.ExternalIdentity{Username: i.Email, ConnectorID: i.ConnectorID}
	if err := out.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	return out, nil
}

// User provides
type User struct {
	Owner      bool       `yaml:"owner" json:"owner"`
	Password   string     `yaml:"password" json:"password"`
	Type       string     `yaml:"type" json:"type"`
	Org        string     `yaml:"org" json:"org"`
	Identities []Identity `yaml:"identities" json:"identities"`
	Email      string     `yaml:"email" json:"email"`
	Roles      []string   `yaml:"roles" json:"roles"`
	Tokens     []string   `yaml:"tokens" json:"tokens"`
}

func (u *User) ParsedIdentities() ([]teleservices.ExternalIdentity, error) {
	out := make([]teleservices.ExternalIdentity, len(u.Identities))
	for i, id := range u.Identities {
		parsed, err := id.Parse()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out[i] = *parsed
	}
	return out, nil
}

// PackageServiceConfig provides settings for package service
type PackageServiceConfig struct {
	// ListenAddr provides HTTP API for package service
	ListenAddr teleutils.NetAddr `yaml:"listen_addr"`
	// PublicListenAddr is the listen address for a server that serves
	// user traffic such as UI, will be used only if user and cluster
	// traffic are separated in Ops Center mode
	PublicListenAddr teleutils.NetAddr `yaml:"public_listen_addr"`

	// AdvertiseAddr is the address advertised to clients
	AdvertiseAddr teleutils.NetAddr `yaml:"advertise_addr" env:"ADVERTISE_ADDR"`
	// PublicAdvertiseAddr is the advertise address for user traffic such
	// as UI
	PublicAdvertiseAddr teleutils.NetAddr `yaml:"public_advertise_addr"`

	// ReadDir is an optional directory with extra packages
	ReadDir string `yaml:"read_dir"`
}

// PeerAddr returns peer address of the package service instance
func (p *PackageServiceConfig) PeerAddr() (*teleutils.NetAddr, error) {
	podIP := os.Getenv(constants.EnvPodIP)
	if podIP == "" {
		addr := p.GetAddr()
		log.Infof("PodIP is not found, falling back to advertise addr: %v", addr)
		return &addr, nil
	}
	_, port, err := net.SplitHostPort(p.ListenAddr.Addr)
	if err != nil {
		return nil, trace.BadParameter("failed to parse port in %v", p.ListenAddr)
	}
	return &teleutils.NetAddr{
		AddrNetwork: p.ListenAddr.AddrNetwork,
		Addr:        fmt.Sprintf("%v:%v", podIP, port),
	}, nil
}

func (p *PackageServiceConfig) GetAddr() teleutils.NetAddr {
	if p.AdvertiseAddr.IsEmpty() {
		return p.ListenAddr
	}
	return p.AdvertiseAddr
}

func (p *PackageServiceConfig) GetPublicAddr() teleutils.NetAddr {
	if p.PublicAdvertiseAddr.IsEmpty() {
		return p.GetAddr()
	}
	return p.PublicAdvertiseAddr
}

// Charts defines Helm charts repository configuration.
type ChartsConfig struct {
	// Backend is the chart repository backend.
	//
	// Currently only cluster-local backend is supported (i.e. repository
	// index file and charts are stored in the cluster's database/object
	// storage). Other backends, such as S3, will/can be added later.
	Backend string `yaml:"backend"`
}

// CheckAndSetDefaults validates chart repository configuration.
func (c *ChartsConfig) CheckAndSetDefaults() error {
	switch c.Backend {
	case helm.BackendLocal:
	case "":
		c.Backend = helm.BackendLocal
	default:
		return trace.BadParameter("unsupported chart repository backend %q, only %q is currently supported",
			c.Backend, helm.BackendLocal)
	}
	return nil
}

// OpsCenterConfig provides settings for access and installation portal
type OpsCenterConfig struct {
	// SeedConfig defines optional configuration to apply on OpsCenter start
	SeedConfig *ops.SeedConfig `yaml:"seed_config"`
}

type Locators []loc.Locator

func (l *Locators) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var locators []string
	if err := unmarshal(&locators); err != nil {
		return trace.Wrap(err)
	}
	out := make([]loc.Locator, len(locators))
	for i, val := range locators {
		loc, err := loc.ParseLocator(val)
		if err != nil {
			return trace.Wrap(err, val)
		}
		out[i] = *loc
	}
	*l = out
	return nil
}

type Users []User

func (u *Users) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var users []User
	if err := unmarshal(&users); err != nil {
		return trace.Wrap(err)
	}
	out := make([]User, len(users))
	for i, user := range users {
		if user.Email == "" {
			return trace.BadParameter("missing User.Email")
		}
		if user.Org == "" {
			return trace.BadParameter("missing User.Org")
		}
		if user.Type == "" {
			return trace.BadParameter("missing User.Type")
		}
		out[i] = user
	}
	*u = out
	return nil
}
