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

package opsservice

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gravitational/gravity/lib/clients"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/devicemapper"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/storage/clusterconfig"
	"github.com/gravitational/gravity/lib/transfer"
	"github.com/gravitational/gravity/lib/utils"

	teleetcd "github.com/gravitational/teleport/lib/backend/etcdbk"
	telecfg "github.com/gravitational/teleport/lib/config"
	teleservices "github.com/gravitational/teleport/lib/services"
	teleutils "github.com/gravitational/teleport/lib/utils"

	"github.com/cloudflare/cfssl/csr"
	"github.com/gravitational/configure"
	"github.com/gravitational/license/authority"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

// shouldUseInsecure returns true if the commands talking to this ops service API
// such as curl should use --insecure flag
func (s *site) shouldUseInsecure() bool {
	return s.service.cfg.Devmode || s.service.cfg.Local || s.service.cfg.Wizard
}

const (
	etcdProxyOff        = "off"
	etcdProxyOn         = "on"
	etcdNewCluster      = "new"
	etcdExistingCluster = "existing"
	etcdPeerPort        = 2380
	etcdEndpointPort    = 2379
)

// Configure packages configures packages for the specified install operation
func (o *Operator) ConfigurePackages(req ops.ConfigurePackagesRequest) error {
	log.WithField("req", req).Info("Configuring packages.")
	operation, err := o.GetSiteOperation(req.SiteOperationKey)
	if err != nil {
		return trace.Wrap(err)
	}
	switch operation.Type {
	case ops.OperationInstall, ops.OperationExpand:
	default:
		return trace.BadParameter("expected install or expand operation, got: %v",
			operation)
	}
	site, err := o.openSite(req.ClusterKey())
	if err != nil {
		return trace.Wrap(err)
	}
	ctx, err := site.newOperationContext(*operation)
	if err != nil {
		return trace.Wrap(err)
	}
	ctx.provisionedServers, err = site.loadProvisionedServers(
		operation.Servers, 0, ctx.Entry)
	if err != nil {
		return trace.Wrap(err)
	}
	// cluster state servers need to be added before configuring packages so
	// they make it into the site export package
	err = site.addClusterStateServers(operation.Servers)
	if err != nil {
		return trace.Wrap(err)
	}

	if operation.Type == ops.OperationInstall {
		err = site.configurePackages(ctx, req)
	} else {
		err = site.configureExpandPackages(context.TODO(), ctx)
	}
	if err != nil {
		// remove cluster state servers on error so the operation can be retried
		errRemove := site.removeClusterStateServers(storage.Hostnames(operation.Servers))
		if errRemove != nil {
			o.Errorf("Failed to remove cluster state servers: %v.",
				trace.DebugReport(errRemove))
		}
		return trace.Wrap(err)
	}

	return nil
}

// getEtcdConfig returns etcd configuration for the provided server
func (s *site) getEtcdConfig(ctx context.Context, opCtx *operationContext, server *ProvisionedServer) (*etcdConfig, error) {
	etcdClient, err := clients.DefaultEtcdMembers()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	members, err := etcdClient.List(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	initialCluster := []string{opCtx.provisionedServers.InitialCluster(s.domainName)}
	// add existing members
	for _, member := range members {
		address, err := utils.URLHostname(member.PeerURLs[0])
		if err != nil {
			return nil, trace.Wrap(err)
		}
		initialCluster = append(initialCluster, fmt.Sprintf("%s:%s",
			member.Name, address))
	}
	proxyMode := etcdProxyOff
	if !server.IsMaster() {
		proxyMode = etcdProxyOn
	}
	return &etcdConfig{
		initialCluster:      strings.Join(initialCluster, ","),
		initialClusterState: etcdExistingCluster,
		proxyMode:           proxyMode,
	}, nil
}

func (s *site) getTeleportMasterIPs(ctx context.Context) (ips []string, err error) {
	masters, err := s.getTeleportMasters(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, master := range masters {
		ips = append(ips, master.IP)
	}
	return ips, nil
}

func (s *site) getTeleportMasters(ctx context.Context) (servers []teleportServer, err error) {
	masters, err := s.teleport().GetServers(ctx, s.domainName, map[string]string{
		schema.ServiceLabelRole: string(schema.ServiceRoleMaster),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, master := range masters {
		server, err := newTeleportServer(master)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		servers = append(servers, *server)
	}
	return servers, nil
}

func (s *site) getTeleportMaster(ctx context.Context) (*teleportServer, error) {
	masters, err := s.getTeleportMasters(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(masters) == 0 {
		return nil, trace.NotFound("no master servers found")
	}
	return &masters[0], nil
}

func (s *site) configureExpandPackages(ctx context.Context, opCtx *operationContext) error {
	teleportMasterIPs, err := s.getTeleportMasterIPs(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	provisionedServer := opCtx.provisionedServers[0]
	etcdConfig, err := s.getEtcdConfig(ctx, opCtx, provisionedServer)
	if err != nil {
		return trace.Wrap(err)
	}
	planetPackage, err := s.app.Manifest.RuntimePackage(provisionedServer.Profile)
	if err != nil {
		return trace.Wrap(err)
	}
	secretsPackage := s.planetSecretsPackage(provisionedServer, planetPackage.Version)
	configPackage := s.planetConfigPackage(provisionedServer, planetPackage.Version)
	env, err := s.service.GetClusterEnvironmentVariables(s.key)
	if err != nil {
		return trace.Wrap(err)
	}
	config, err := s.service.GetClusterConfiguration(s.key)
	if err != nil {
		return trace.Wrap(err)
	}
	planetConfig := planetConfig{
		server:        *provisionedServer,
		installExpand: opCtx.operation,
		etcd:          *etcdConfig,
		docker:        s.dockerConfig(),
		planetPackage: *planetPackage,
		configPackage: configPackage,
		manifest:      s.app.Manifest,
		env:           env.GetKeyValues(),
		config:        config,
	}
	if provisionedServer.IsMaster() {
		err := s.configureTeleportMaster(opCtx, provisionedServer)
		if err != nil && !trace.IsAlreadyExists(err) {
			return trace.Wrap(err)
		}
		masterParams := planetMasterParams{
			master:         provisionedServer,
			secretsPackage: &secretsPackage,
			serviceSubnet:  planetConfig.serviceSubnet(),
		}
		// if we have connection to an Ops Center set up, configure
		// SNI host so it can dial in
		trustedCluster, err := storage.GetTrustedCluster(s.backend())
		if err == nil {
			masterParams.sniHost = trustedCluster.GetSNIHost()
		}
		err = s.configurePlanetMasterSecrets(opCtx, masterParams)
		if err != nil && !trace.IsAlreadyExists(err) {
			return trace.Wrap(err)
		}
		planetConfig.master = masterConfig{
			electionEnabled: false,
			addr:            s.teleport().GetPlanetLeaderIP(),
		}
		err = s.configurePlanetMaster(planetConfig, configPackage)
		if err != nil && !trace.IsAlreadyExists(err) {
			return trace.Wrap(err)
		}
		// Teleport nodes on masters prefer their local auth server
		// but will try all other masters if the local gravity-site
		// isn't running.
		err = s.configureTeleportNode(opCtx, append([]string{constants.Localhost}, teleportMasterIPs...),
			provisionedServer)
		if err != nil && !trace.IsAlreadyExists(err) {
			return trace.Wrap(err)
		}
	} else {
		err = s.configurePlanetNodeSecrets(opCtx, provisionedServer, secretsPackage)
		if err != nil && !trace.IsAlreadyExists(err) {
			return trace.Wrap(err)
		}
		err = s.configurePlanetNode(planetConfig, configPackage)
		if err != nil && !trace.IsAlreadyExists(err) {
			return trace.Wrap(err)
		}
		err = s.configureTeleportNode(opCtx, teleportMasterIPs, provisionedServer)
		if err != nil && !trace.IsAlreadyExists(err) {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (s *site) configurePackages(ctx *operationContext, req ops.ConfigurePackagesRequest) error {
	err := s.packages().UpsertRepository(s.siteRepoName(), time.Time{})
	if err != nil {
		return trace.Wrap(err)
	}

	p := ctx.provisionedServers

	if err := s.configurePlanetCertAuthority(ctx); err != nil {
		return trace.Wrap(err)
	}

	err = s.configureRemoteCluster()
	if err != nil && !trace.IsAlreadyExists(err) {
		return trace.Wrap(err)
	}

	err = s.configureSiteExportPackage(ctx)
	if err != nil && !trace.IsAlreadyExists(err) {
		return trace.Wrap(err)
	}

	err = s.configureLicensePackage(ctx)
	if err != nil && !trace.IsAlreadyExists(err) {
		return trace.Wrap(err)
	}

	masters := p.Masters()
	activeMaster := masters[0]
	regularMasterConfig := masterConfig{
		electionEnabled: true,
	}
	suspendedMasterConfig := masterConfig{
		electionEnabled: false,
		addr:            activeMaster.AdvertiseIP,
	}

	etcdConfig := s.prepareEtcdConfig(ctx)

	clusterConfig := clusterconfig.NewEmpty()
	if len(req.Config) != 0 {
		clusterConfig, err = clusterconfig.Unmarshal(req.Config)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	if s.cloudProviderName() != "" {
		clusterConfig.SetCloudProvider(s.cloudProviderName())
	}

	for i, master := range masters {
		planetPackage, err := s.app.Manifest.RuntimePackage(master.Profile)
		if err != nil {
			return trace.Wrap(err)
		}
		secretsPackage := s.planetSecretsPackage(master, planetPackage.Version)
		configPackage := s.planetConfigPackage(master, planetPackage.Version)

		masterConfig := regularMasterConfig
		masterConfig.addr = master.AdvertiseIP
		if i > 0 {
			// Suspend node's election participation until after the installation
			// to avoid multiple nodes competing to be leader before the state
			// has been properly initialized in a single master
			masterConfig = suspendedMasterConfig
		}

		masterEtcdConfig, ok := etcdConfig[master.AdvertiseIP]
		if !ok {
			return trace.NotFound("etcd config not found for %v: %v",
				master, etcdConfig)
		}

		config := planetConfig{
			server:        *master,
			installExpand: ctx.operation,
			etcd:          masterEtcdConfig,
			master:        masterConfig,
			docker:        s.dockerConfig(),
			planetPackage: *planetPackage,
			configPackage: configPackage,
			manifest:      s.app.Manifest,
			env:           req.Env,
			config:        clusterConfig,
		}

		err = s.configurePlanetMasterSecrets(ctx, planetMasterParams{
			master:         master,
			secretsPackage: &secretsPackage,
			serviceSubnet:  config.serviceSubnet(),
			sniHost:        s.service.cfg.SNIHost,
		})
		if err != nil && !trace.IsAlreadyExists(err) {
			return trace.Wrap(err)
		}

		err = s.configurePlanetMaster(config, configPackage)
		if err != nil && !trace.IsAlreadyExists(err) {
			return trace.Wrap(err)
		}

		err = s.configureTeleportMaster(ctx, master)
		if err != nil && !trace.IsAlreadyExists(err) {
			return trace.Wrap(err)
		}

		// Teleport nodes on masters prefer their local auth server
		// but will try all other masters if the local gravity-site
		// isn't running.
		err = s.configureTeleportNode(ctx, append([]string{constants.Localhost}, p.MasterIPs()...),
			master)
		if err != nil && !trace.IsAlreadyExists(err) {
			return trace.Wrap(err)
		}
	}

	for _, node := range p.Nodes() {
		err := s.configureTeleportNode(ctx, p.MasterIPs(), node)
		if err != nil && !trace.IsAlreadyExists(err) {
			return trace.Wrap(err)
		}

		planetPackage, err := s.app.Manifest.RuntimePackage(node.Profile)
		if err != nil {
			return trace.Wrap(err)
		}

		secretsPackage := s.planetSecretsPackage(node, planetPackage.Version)

		err = s.configurePlanetNodeSecrets(ctx, node, secretsPackage)
		if err != nil && !trace.IsAlreadyExists(err) {
			return trace.Wrap(err)
		}

		configPackage := s.planetConfigPackage(node, planetPackage.Version)

		nodeEtcdConfig, ok := etcdConfig[node.AdvertiseIP]
		if !ok {
			return trace.NotFound("etcd config not found for %v: %v",
				node.AdvertiseIP, etcdConfig)
		}

		config := planetConfig{
			server:        *node,
			installExpand: ctx.operation,
			etcd:          nodeEtcdConfig,
			master:        masterConfig{addr: p.FirstMaster().AdvertiseIP},
			docker:        s.dockerConfig(),
			planetPackage: *planetPackage,
			configPackage: configPackage,
			manifest:      s.app.Manifest,
			env:           req.Env,
			config:        clusterConfig,
		}

		err = s.configurePlanetNode(config, configPackage)
		if err != nil && !trace.IsAlreadyExists(err) {
			return trace.Wrap(err)
		}
	}

	return nil
}

// configureRemoteCluster creates a RemoteCluster resource that represents
// the cluster that is being installed on the installer side
func (s *site) configureRemoteCluster() error {
	remoteCluster, err := teleservices.NewRemoteCluster(s.domainName)
	if err != nil {
		return trace.Wrap(err)
	}
	err = s.users().CreateRemoteCluster(remoteCluster)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(err)
}

// prepareEtcdConfig assigns each of the provisioned servers a etcd config that indicates
// whether the server is a full etcd member or a proxy
func (s *site) prepareEtcdConfig(ctx *operationContext) clusterEtcdConfig {
	// choose up to max allowed number of servers that will be running full etcd members
	var fullMembers provisionedServers
	for i := 0; i < len(ctx.provisionedServers); i++ {
		if ctx.provisionedServers[i].IsMaster() {
			fullMembers = append(fullMembers, ctx.provisionedServers[i])
		}
	}

	// the chosen servers will form the initial cluster
	initialCluster := fullMembers.InitialCluster(s.domainName)

	// assign each server etcd config according to its role as a full member or a proxy
	config := make(clusterEtcdConfig)
	for _, server := range ctx.provisionedServers {
		if fullMembers.Contains(server.AdvertiseIP) {
			config[server.AdvertiseIP] = etcdConfig{
				initialCluster:      initialCluster,
				initialClusterState: etcdNewCluster,
				proxyMode:           etcdProxyOff,
			}
		} else {
			config[server.AdvertiseIP] = etcdConfig{
				initialCluster:      initialCluster,
				initialClusterState: etcdExistingCluster,
				proxyMode:           etcdProxyOn,
			}
		}
		log.Debugf("server %v (%v) etcd config: %v",
			server.Hostname, server.AdvertiseIP, config[server.AdvertiseIP])
	}

	return config
}

func (s *site) configurePlanetCertAuthority(ctx *operationContext) error {
	caPackage := s.planetCertAuthorityPackage()
	if _, err := s.packages().ReadPackageEnvelope(caPackage); err == nil {
		s.Debugf("%v already created", caPackage)
		return nil
	}

	s.Debugf("generating certificate authority package")
	planetCertAuthority, err := authority.GenerateSelfSignedCA(csr.CertificateRequest{
		CN: s.siteRepoName(),
		CA: &csr.CAConfig{
			Expiry: defaults.CACertificateExpiry.String(),
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}

	s.Debugf("generating apiserver keypair key")
	// we have to share the same private key for various apiservers
	// due to this issue:
	// https://github.com/kubernetes/kubernetes/issues/11000#issuecomment-232469678
	// TODO(security) separate this out to a separate secret for serviceaccounttokens
	// For reference: https://github.com/kelseyhightower/kubernetes-the-hard-way/issues/248
	apiServer, err := authority.GenerateCertificate(csr.CertificateRequest{
		CN:    constants.APIServerKeyPair,
		Hosts: []string{"127.0.0.1"},
		Names: []csr.Name{
			{
				O: defaults.SystemAccountOrg,
			},
		},
	}, planetCertAuthority, nil, defaults.CertificateExpiry)
	if err != nil {
		return trace.Wrap(err)
	}

	// also add OpsCenter's cert authority (without private key)
	opsCertAuthority, err := pack.ReadCertificateAuthority(s.packages())
	if err != nil {
		return trace.Wrap(err)
	}
	opsCertAuthority.KeyPEM = nil

	reader, err := utils.CreateTLSArchive(utils.TLSArchive{
		constants.APIServerKeyPair: apiServer,
		constants.RootKeyPair:      planetCertAuthority,
		constants.OpsCenterKeyPair: opsCertAuthority,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	defer reader.Close()

	_, err = s.packages().CreatePackage(caPackage, reader, pack.WithLabels(
		map[string]string{
			pack.PurposeLabel:     pack.PurposeCA,
			pack.OperationIDLabel: ctx.operation.ID,
		}))
	return trace.Wrap(err)
}

// ReadCertAuthorityPackage returns the certificate authority package for
// the specified cluster
func ReadCertAuthorityPackage(packages pack.PackageService, clusterName string) (utils.TLSArchive, error) {
	caPackage := PlanetCertAuthorityPackage(clusterName)
	_, reader, err := packages.ReadPackage(caPackage)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer reader.Close()
	return utils.ReadTLSArchive(reader)
}

func (s *site) readCertAuthorityPackage() (utils.TLSArchive, error) {
	return ReadCertAuthorityPackage(s.packages(), s.domainName)
}

type planetMasterParams struct {
	master         *ProvisionedServer
	secretsPackage *loc.Locator
	serviceSubnet  string
	sniHost        string
}

func (s *site) getPlanetMasterSecretsPackage(ctx *operationContext, p planetMasterParams) (*ops.RotatePackageResponse, error) {
	archive, err := s.readCertAuthorityPackage()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	caKeyPair, err := archive.GetKeyPair(constants.RootKeyPair)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Don't rotate apiserver secrets, as this secret is currently used to authenticate service account tokens
	// TODO(security) support rotation of apiserver / serviceaccount secrets
	apiserverKeyPair, err := archive.GetKeyPair(constants.APIServerKeyPair)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	serviceSubnet, err := configure.ParseCIDR(p.serviceSubnet)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	apiServerIP := serviceSubnet.FirstIP().String()

	newArchive := make(utils.TLSArchive)

	if err := newArchive.AddKeyPair(constants.RootKeyPair, *caKeyPair); err != nil {
		return nil, trace.Wrap(err)
	}

	keyPairTypes := map[string]rbacConfig{
		constants.APIServerKeyPair: {},
		constants.ETCDKeyPair:      {},
		constants.SchedulerKeyPair: {group: constants.ClusterAdminGroup},
		constants.KubectlKeyPair:   {group: constants.ClusterAdminGroup},
		constants.ProxyKeyPair:     {userName: constants.ClusterKubeProxyUser, group: constants.ClusterNodeGroup},
		constants.KubeletKeyPair: {
			userName: constants.ClusterNodeNamePrefix + ":" + p.master.KubeNodeID(),
			group:    constants.ClusterNodeGroup,
		},
		constants.APIServerKubeletClientKeyPair: {group: constants.ClusterAdminGroup},
		constants.PlanetRpcKeyPair:              {},
		constants.CoreDNSKeyPair:                {},
		constants.FrontProxyClientKeyPair:       {},
		constants.LograngeAdaptorKeyPair:        {},
		constants.LograngeAggregatorKeyPair:     {},
		constants.LograngeCollectorKeyPair:      {},
		constants.LograngeForwarderKeyPair:      {},
	}

	for name, config := range keyPairTypes {
		req := csr.CertificateRequest{
			Hosts: []string{constants.LoopbackIP, constants.AlternativeLoopbackIP, p.master.AdvertiseIP, p.master.Hostname},
		}
		commonName := config.userName
		if commonName == "" {
			commonName = name
		}
		req.CN = commonName
		if config.group != "" {
			req.Names = []csr.Name{{O: config.group}}
		}

		var privateKeyPEM []byte
		switch name {
		case constants.APIServerKeyPair:
			req.Hosts = append(req.Hosts,
				constants.APIServerDomainNameGravity,
				constants.APIServerDomainName,
				constants.LegacyAPIServerDomainName,
				constants.RegistryDomainName,
				apiServerIP)
			req.Hosts = append(req.Hosts, constants.KubernetesServiceDomainNames...)
			if p.master.Nodename != "" {
				req.Hosts = append(req.Hosts, p.master.Nodename)
			}
			// this will make APIServer's certificate valid for requests
			// via OpsCenter, e.g. siteDomain.opscenter.example.com
			if p.sniHost != "" {
				req.Hosts = append(req.Hosts, strings.Join([]string{
					s.domainName, p.sniHost}, "."))
			}
			// in wizard mode, we should add the original OpsCenter hostname too
			if s.service.cfg.Wizard {
				for _, host := range s.service.cfg.SeedConfig.SNIHosts() {
					req.Hosts = append(req.Hosts, strings.Join([]string{
						s.domainName, host}, "."))
				}
			}

			// Don't rotate the APIServer key, the secret is currently used for validating serviceaccounttokens
			// TODO(security) enable rotation of secret for apiserver/serviceaccounttokens
			privateKeyPEM = apiserverKeyPair.KeyPEM
		case constants.ProxyKeyPair:
			req.Hosts = append(req.Hosts,
				constants.APIServerDomainNameGravity,
				constants.APIServerDomainName)
		case constants.LograngeAggregatorKeyPair:
			req.Hosts = append(req.Hosts, utils.KubeServiceNames(
				defaults.LograngeAggregatorServiceName,
				defaults.KubeSystemNamespace)...)
		}

		keyPair, err := authority.GenerateCertificate(req, caKeyPair, privateKeyPEM, defaults.CertificateExpiry)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if err := newArchive.AddKeyPair(name, *keyPair); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	reader, err := utils.CreateTLSArchive(newArchive)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	labels := map[string]string{
		pack.PurposeLabel:     pack.PurposePlanetSecrets,
		pack.AdvertiseIPLabel: p.master.AdvertiseIP,
		pack.OperationIDLabel: ctx.operation.ID,
	}

	return &ops.RotatePackageResponse{
		Locator: *p.secretsPackage,
		Reader:  reader,
		Labels:  labels,
	}, nil
}

func (s *site) configurePlanetMasterSecrets(ctx *operationContext, p planetMasterParams) error {
	resp, err := s.getPlanetMasterSecretsPackage(ctx, p)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = s.packages().CreatePackage(resp.Locator, resp.Reader, pack.WithLabels(resp.Labels))
	return trace.Wrap(err)
}

func (s *site) getPlanetNodeSecretsPackage(ctx *operationContext, node *ProvisionedServer, secretsPackage loc.Locator) (*ops.RotatePackageResponse, error) {
	archive, err := s.readCertAuthorityPackage()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	caKeyPair, err := archive.GetKeyPair(constants.RootKeyPair)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	newArchive := make(utils.TLSArchive)

	caCertKeyPair := *caKeyPair
	caCertKeyPair.KeyPEM = nil

	if err := newArchive.AddKeyPair(constants.RootKeyPair, caCertKeyPair); err != nil {
		return nil, trace.Wrap(err)
	}

	keyPairTypes := map[string]rbacConfig{
		constants.APIServerKeyPair:         {},
		constants.ETCDKeyPair:              {},
		constants.KubectlKeyPair:           {group: constants.ClusterNodeGroup},
		constants.ProxyKeyPair:             {userName: constants.ClusterKubeProxyUser, group: constants.ClusterNodeGroup},
		constants.KubeletKeyPair:           {userName: constants.ClusterNodeNamePrefix + ":" + node.KubeNodeID(), group: constants.ClusterNodeGroup},
		constants.PlanetRpcKeyPair:         {},
		constants.CoreDNSKeyPair:           {},
		constants.LograngeCollectorKeyPair: {},
	}

	for keyName, config := range keyPairTypes {
		req := csr.CertificateRequest{
			Hosts: []string{constants.LoopbackIP, node.AdvertiseIP, node.Hostname},
		}
		if keyName == constants.ProxyKeyPair {
			req.Hosts = append(req.Hosts,
				constants.APIServerDomainNameGravity,
				constants.APIServerDomainName)
		}
		if node.Nodename != "" {
			req.Hosts = append(req.Hosts, node.Nodename)
		}
		commonName := config.userName
		if commonName == "" {
			commonName = keyName
		}
		req.CN = commonName
		if config.group != "" {
			req.Names = []csr.Name{{O: config.group}}
		}
		keyPair, err := authority.GenerateCertificate(req, caKeyPair, nil, defaults.CertificateExpiry)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if err := newArchive.AddKeyPair(keyName, *keyPair); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	reader, err := utils.CreateTLSArchive(newArchive)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	labels := map[string]string{
		pack.PurposeLabel:     pack.PurposePlanetSecrets,
		pack.AdvertiseIPLabel: node.AdvertiseIP,
		pack.OperationIDLabel: ctx.operation.ID,
	}

	return &ops.RotatePackageResponse{
		Locator: secretsPackage,
		Reader:  reader,
		Labels:  labels,
	}, nil
}

func (s *site) configurePlanetNodeSecrets(ctx *operationContext, node *ProvisionedServer, secretsPackage loc.Locator) error {
	resp, err := s.getPlanetNodeSecretsPackage(ctx, node, secretsPackage)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = s.packages().CreatePackage(resp.Locator, resp.Reader, pack.WithLabels(resp.Labels))
	return trace.Wrap(err)
}

// rbacConfig groups attributes to generate TLS configuration for RBAC
type rbacConfig struct {
	userName string
	group    string
}

// etcdConfig represents a single server etcd configuration, e.g. whether
// it's running in a proxy mode or not
type etcdConfig struct {
	// initialCluster is etcd initial cluster configuration for bootstrapping
	initialCluster string
	// initialClusterState determines whether the member is a part of initial
	// bootstrapping cluster or joining an existing one
	initialClusterState string
	// proxyMode is whether the member is running in a proxy mode or not
	proxyMode string
}

// clusterEtcdConfig maps server to its etcd configuration
type clusterEtcdConfig map[string]etcdConfig

// masterConfig defines initial master configuration
type masterConfig struct {
	// electionEnabled controls if the new master node
	// starts with election participation enabled
	electionEnabled bool
	// addr is the IP address of the currently active master node
	addr string
}

func (s *site) configurePlanetMaster(config planetConfig, configPackage loc.Locator) error {
	if server := config.installExpand.InstallExpand.Servers.FindByIP(config.server.AdvertiseIP); server != nil {
		config.dockerRuntime = server.Docker
	}
	err := s.configurePlanetServer(config)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (s *site) configurePlanetNode(config planetConfig, configPackage loc.Locator) error {
	if server := config.installExpand.InstallExpand.Servers.FindByIP(config.server.AdvertiseIP); server != nil {
		config.dockerRuntime = server.Docker
	}
	err := s.configurePlanetServer(config)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (s *site) getPlanetConfigPackage(config planetConfig) (*ops.RotatePackageResponse, error) {
	args, err := s.getPlanetConfig(config)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	reader, err := pack.GetConfigPackage(s.packages(), config.planetPackage, config.configPackage, args)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	labels := map[string]string{
		pack.PurposeLabel:     pack.PurposePlanetConfig,
		pack.ConfigLabel:      config.planetPackage.ZeroVersion().String(),
		pack.AdvertiseIPLabel: config.server.AdvertiseIP,
		pack.OperationIDLabel: config.installExpand.ID,
	}
	return &ops.RotatePackageResponse{
		Locator: config.configPackage,
		Reader:  reader,
		Labels:  labels,
	}, nil
}

func (s *site) getPlanetConfig(config planetConfig) (args []string, err error) {
	node := config.server
	manifest := config.manifest
	profile, err := manifest.NodeProfiles.ByName(node.Role)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	args = []string{
		fmt.Sprintf("--node-name=%v", node.KubeNodeID()),
		fmt.Sprintf("--hostname=%v", node.Hostname),
		fmt.Sprintf("--master-ip=%v", config.master.addr),
		fmt.Sprintf("--public-ip=%v", node.AdvertiseIP),
		fmt.Sprintf("--cluster-id=%v", s.domainName),
		fmt.Sprintf("--etcd-proxy=%v", config.etcd.proxyMode),
		fmt.Sprintf("--etcd-member-name=%v", node.EtcdMemberName(s.domainName)),
		fmt.Sprintf("--initial-cluster=%v", config.etcd.initialCluster),
		fmt.Sprintf("--secrets-dir=%v", node.InGravity(defaults.SecretsDir)),
		fmt.Sprintf("--etcd-initial-cluster-state=%v", config.etcd.initialClusterState),
		fmt.Sprintf("--volume=%v:/ext/etcd", node.InGravity("planet", "etcd")),
		fmt.Sprintf("--volume=%v:/ext/registry", node.InGravity("planet", "registry")),
		fmt.Sprintf("--volume=%v:/ext/docker", node.InGravity("planet", "docker")),
		fmt.Sprintf("--volume=%v:/ext/share", node.InGravity("planet", "share")),
		fmt.Sprintf("--volume=%v:/ext/state", node.InGravity("planet", "state")),
		fmt.Sprintf("--volume=%v:/var/log", node.InGravity("planet", "log")),
		fmt.Sprintf("--volume=%v:%v", node.StateDir(), defaults.GravityDir),
		fmt.Sprintf("--service-uid=%v", s.uid()),
		fmt.Sprintf("--service-gid=%v", s.gid()),
	}
	overrideArgs := map[string]string{
		"service-subnet": config.serviceSubnet(),
		"pod-subnet":     config.podSubnet(),
	}

	if config.master.electionEnabled {
		args = append(args, "--election-enabled")
	} else {
		args = append(args, "--no-election-enabled")
	}

	for k, v := range config.env {
		args = append(args, fmt.Sprintf("--env=%v=%v", k, strconv.Quote(v)))
	}

	args = append(args, s.addCloudConfig(config.config)...)
	args = append(args, s.addClusterConfig(config.config, overrideArgs)...)

	if node.IsMaster() {
		args = append(args, "--role=master")
	} else {
		args = append(args, "--role=node")
	}
	args = append(args, manifest.RuntimeArgs(*profile)...)

	if len(s.backendSite.DNSOverrides.Hosts) > 0 {
		args = append(args, fmt.Sprintf("--dns-hosts=%v",
			s.backendSite.DNSOverrides.FormatHosts()))
	}

	if len(s.backendSite.DNSOverrides.Zones) > 0 {
		args = append(args, fmt.Sprintf("--dns-zones=%v",
			s.backendSite.DNSOverrides.FormatZones()))
	}

	vxlanPort := config.installExpand.InstallExpand.Vars.OnPrem.VxlanPort
	if vxlanPort != 0 {
		args = append(args, fmt.Sprintf("--vxlan-port=%v", vxlanPort))
	}

	dnsConfig := s.dnsConfig()
	for _, addr := range dnsConfig.Addrs {
		args = append(args, fmt.Sprintf("--dns-listen-addr=%v", addr))
	}
	args = append(args, fmt.Sprintf("--dns-port=%v", dnsConfig.Port))

	dockerArgs, err := configureDockerOptions(config.installExpand, node,
		config.docker, config.dockerRuntime)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	args = append(args, dockerArgs...)

	etcdArgs := manifest.EtcdArgs(*profile)
	if len(etcdArgs) != 0 {
		args = append(args, fmt.Sprintf("--etcd-options=%v", strings.Join(etcdArgs, " ")))
	}

	var kubeletArgs []string
	if len(manifest.KubeletArgs(*profile)) != 0 {
		kubeletArgs = append(kubeletArgs, manifest.KubeletArgs(*profile)...)
	}

	if len(kubeletArgs) > 0 {
		args = append(args, fmt.Sprintf("--kubelet-options=%v", strings.Join(kubeletArgs, " ")))
	}

	mounts, err := GetMounts(manifest, node.Server)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	for _, mount := range mounts {
		spec := fmt.Sprintf("--volume=%v:%v", mount.Source, mount.Destination)
		if mount.SkipIfMissing {
			spec = fmt.Sprintf("--volume=%v:%v:skip", mount.Source, mount.Destination)
		}
		if mount.Recursive {
			spec = fmt.Sprintf("%v:rec", spec)
		}
		args = append(args, spec)
	}

	devices, err := manifest.DevicesForProfile(node.Role)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, device := range devices {
		args = append(args, fmt.Sprintf("--device=%v", device.Format()))
	}

	for _, taint := range profile.Taints {
		args = append(args, fmt.Sprintf("--taint=%v=%v:%v", taint.Key, taint.Value, taint.Effect))
	}

	for k, v := range node.GetKubeletLabels(profile.Labels) {
		args = append(args, fmt.Sprintf("--node-label=%v=%v", k, v))
	}

	// If the manifest contains an install hook to install a separate overlay network, disable flannel inside planet
	if manifest.Hooks != nil && manifest.Hooks.NetworkInstall != nil {
		args = append(args, "--disable-flannel")
	}

	if manifest.SystemOptions != nil && manifest.SystemOptions.AllowPrivileged {
		args = append(args, "--allow-privileged")
	}

	for k, v := range overrideArgs {
		args = append(args, fmt.Sprintf("--%v=%v", k, v))
	}

	log.WithField("args", args).Info("Runtime configuration.")
	return args, nil
}

func (s *site) configurePlanetServer(config planetConfig) error {
	resp, err := s.getPlanetConfigPackage(config)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = s.packages().CreatePackage(resp.Locator, resp.Reader, pack.WithLabels(resp.Labels))
	if err != nil {
		if trace.IsAlreadyExists(err) {
			s.WithField("package", config.configPackage).Debug("Planet configuration package already exists.")
		}
		return trace.Wrap(err)
	}
	return nil
}

func (r planetConfig) podSubnet() string {
	var override string
	if r.config != nil {
		override = r.config.GetGlobalConfig().PodCIDR
	}
	return podSubnet(r.installExpand.InstallExpand, override)
}

func (r planetConfig) serviceSubnet() string {
	var override string
	if r.config != nil {
		override = r.config.GetGlobalConfig().ServiceCIDR
	}
	return serviceSubnet(r.installExpand.InstallExpand, override)
}

type planetConfig struct {
	manifest      schema.Manifest
	installExpand ops.SiteOperation
	server        ProvisionedServer
	etcd          etcdConfig
	master        masterConfig
	docker        storage.DockerConfig
	dockerRuntime storage.Docker
	planetPackage loc.Locator
	configPackage loc.Locator
	// env specifies additional environment variables to pass to runtime container
	env map[string]string
	// config specifies optional cluster configuration
	config clusterconfig.Interface
}

// getPrincipals returns a list of SANs (x509's Subject Alternative Names)
// for the provided server.
func (s *site) getPrincipals(node *ProvisionedServer) []string {
	principals := []string{
		node.AdvertiseIP,
		node.Hostname,
	}
	if node.Nodename != "" {
		principals = append(principals, node.Nodename)
	}
	return principals
}

func (s *site) getTeleportMasterConfig(ctx *operationContext, configPackage loc.Locator, master *ProvisionedServer) (*ops.RotatePackageResponse, error) {
	fileConf := &telecfg.FileConfig{
		Global: telecfg.Global{
			Ciphers:       defaults.TeleportCiphers,
			KEXAlgorithms: defaults.TeleportKEXAlgorithms,
			MACAlgorithms: defaults.TeleportMACAlgorithms,
		},
	}

	fileConf.DataDir = defaults.InGravity("site/teleport")
	fileConf.Storage.Type = teleetcd.GetName()
	secretsDir := defaults.InGravity(defaults.SecretsDir)
	// Config represents JSON config for etcd backend
	params, err := toObject(teleetcd.Config{
		Nodes:       []string{fmt.Sprintf("https://%v:%v", master.AdvertiseIP, etcdEndpointPort)},
		Key:         "/teleport",
		TLSKeyFile:  filepath.Join(secretsDir, "etcd.key"),
		TLSCertFile: filepath.Join(secretsDir, "etcd.cert"),
		TLSCAFile:   filepath.Join(secretsDir, "root.cert"),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	fileConf.Storage.Params = params

	advertiseIP := net.ParseIP(master.AdvertiseIP)
	if advertiseIP == nil {
		return nil, trace.BadParameter("failed to parse master advertise IP: %v",
			master.AdvertiseIP)
	}

	fileConf.AdvertiseIP = advertiseIP.String()
	fileConf.Global.NodeName = master.FQDN(s.domainName)

	// turn on auth service
	fileConf.Auth.EnabledFlag = "yes"
	fileConf.Auth.ClusterName = telecfg.ClusterName(s.domainName)

	// turn on proxy and Kubernetes integration
	fileConf.Proxy.EnabledFlag = "yes"
	fileConf.Proxy.Kube.EnabledFlag = "yes"
	fileConf.Proxy.Kube.PublicAddr = teleutils.Strings(s.getPrincipals(master))

	// turn off SSH - we won't SSH into container with Gravity running
	fileConf.SSH.EnabledFlag = "no"

	bytes, err := yaml.Marshal(fileConf)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	args := []string{
		fmt.Sprintf("--config-string=%v", base64.StdEncoding.EncodeToString(bytes)),
	}

	reader, err := pack.GetConfigPackage(s.packages(), s.teleportPackage, configPackage, args)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &ops.RotatePackageResponse{
		Locator: configPackage,
		Reader:  reader,
		Labels: map[string]string{
			pack.PurposeLabel:     pack.PurposeTeleportMasterConfig,
			pack.AdvertiseIPLabel: master.AdvertiseIP,
			pack.OperationIDLabel: ctx.operation.ID,
		},
	}, nil
}

func toObject(in interface{}) (map[string]interface{}, error) {
	data, err := json.Marshal(in)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var out map[string]interface{}
	err = json.Unmarshal(data, &out)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return out, nil
}

func (s *site) getTeleportNodeConfig(ctx *operationContext, masterIPs []string, configPackage loc.Locator, node *ProvisionedServer) (*ops.RotatePackageResponse, error) {
	joinToken, err := s.service.GetExpandToken(s.key)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	fileConf := &telecfg.FileConfig{
		Global: telecfg.Global{
			Ciphers:       defaults.TeleportCiphers,
			KEXAlgorithms: defaults.TeleportKEXAlgorithms,
			MACAlgorithms: defaults.TeleportMACAlgorithms,
		},
	}

	fileConf.DataDir = node.InGravity("teleport")

	if s.service.cfg.Devmode {
		fileConf.Logger.Severity = "debug"
	} else {
		fileConf.Logger.Severity = "info"
	}

	for _, masterIP := range masterIPs {
		fileConf.AuthServers = append(fileConf.AuthServers, fmt.Sprintf("%v:3025", masterIP))
	}
	fileConf.AuthToken = joinToken.Token

	fileConf.SSH.Labels = map[string]string{}

	configureTeleportLabels(node, fileConf.SSH.Labels, s.domainName)

	// for AWS sites, use dynamic teleport labels to periodically query AWS metadata
	// for servers' public IPs
	if s.cloudProviderName() == schema.ProviderAWS {
		fileConf.SSH.Commands = append(fileConf.SSH.Commands, telecfg.CommandLabel{
			Name:    defaults.TeleportPublicIPv4Label,
			Command: defaults.AWSPublicIPv4Command,
			Period:  defaults.TeleportCommandLabelInterval,
		})
	}

	// never expire cache
	fileConf.CachePolicy.EnabledFlag = "yes"
	// FIXME(klizhentas) "never" is not working in current
	// version of teleport, switch to "never" once fixed.
	// for now set to 365 days
	fileConf.CachePolicy.TTL = fmt.Sprintf("%v", 365*24*time.Hour)

	advertiseIP := net.ParseIP(node.AdvertiseIP)
	if advertiseIP == nil {
		return nil, trace.BadParameter("failed to parse advertise IP: %v",
			node.AdvertiseIP)
	}

	fileConf.AdvertiseIP = advertiseIP.String()
	fileConf.Global.NodeName = node.FQDN(s.domainName)

	// turn off auth service and proxy, turn on SSH
	fileConf.Auth.EnabledFlag = "no"
	fileConf.Proxy.EnabledFlag = "no"
	fileConf.SSH.EnabledFlag = "yes"

	bytes, err := yaml.Marshal(fileConf)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	args := []string{
		fmt.Sprintf("--config-string=%v", base64.StdEncoding.EncodeToString(bytes)),
	}

	reader, err := pack.GetConfigPackage(s.packages(), s.teleportPackage, configPackage, args)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &ops.RotatePackageResponse{
		Locator: configPackage,
		Reader:  reader,
		Labels: map[string]string{
			pack.PurposeLabel:     pack.PurposeTeleportNodeConfig,
			pack.AdvertiseIPLabel: node.AdvertiseIP,
			pack.OperationIDLabel: ctx.operation.ID,
			pack.ConfigLabel:      s.teleportPackage.ZeroVersion().String(),
		},
	}, nil
}

func (s *site) configureTeleportMaster(ctx *operationContext, master *ProvisionedServer) error {
	configPackage := s.teleportMasterConfigPackage(master)
	resp, err := s.getTeleportMasterConfig(ctx, configPackage, master)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = s.packages().CreatePackage(resp.Locator, resp.Reader, pack.WithLabels(resp.Labels))
	if err != nil {
		if trace.IsAlreadyExists(err) {
			s.WithField("package", configPackage.String()).Debug("Teleport master configuration package already exists.")
		}
		return trace.Wrap(err)
	}
	return nil
}

func (s *site) configureTeleportNode(ctx *operationContext, masterIPs []string, node *ProvisionedServer) error {
	configPackage := s.teleportNodeConfigPackage(node)
	resp, err := s.getTeleportNodeConfig(ctx, masterIPs, configPackage, node)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = s.packages().CreatePackage(resp.Locator, resp.Reader, pack.WithLabels(resp.Labels))
	if err != nil {
		if trace.IsAlreadyExists(err) {
			s.WithField("package", configPackage.String()).Debug("Teleport node configuration package already exists.")
		}
		return trace.Wrap(err)
	}
	return nil
}

func (s *site) configureSiteExportPackage(ctx *operationContext) error {
	exportPackage := s.siteExportPackage()
	exportDir := s.siteDir("export")
	if err := os.MkdirAll(exportDir, defaults.PrivateDirMask); err != nil {
		return trace.Wrap(err)
	}

	cluster, err := s.backend().GetSite(s.key.SiteDomain)
	if err != nil {
		return trace.Wrap(err)
	}
	reader, err := transfer.ExportSite(cluster, &exportBackend{s}, exportDir,
		s.seedConfig.TrustedClusters)
	if err != nil {
		return trace.Wrap(err)
	}
	defer func() {
		reader.Close()
		if err := os.RemoveAll(exportDir); err != nil {
			s.Warnf("Failed to delete temporary export directory %v: %v.", exportDir, err)
		}
	}()

	_, err = s.packages().CreatePackage(exportPackage, reader, pack.WithLabels(
		map[string]string{
			pack.PurposeLabel:     pack.PurposeExport,
			pack.OperationIDLabel: ctx.operation.ID,
		},
	))
	if err != nil {
		if trace.IsAlreadyExists(err) {
			s.WithField("package", exportPackage.String()).Debug("Cluster export package already exists.")
		}
		return trace.Wrap(err)
	}

	return nil
}

func (s *site) configureLicensePackage(ctx *operationContext) error {
	if s.license == "" {
		return nil // nothing to do
	}

	licensePackage := s.licensePackage()
	reader := strings.NewReader(s.license)
	_, err := s.packages().CreatePackage(licensePackage, reader, pack.WithLabels(
		map[string]string{
			pack.PurposeLabel:     pack.PurposeLicense,
			pack.OperationIDLabel: ctx.operation.ID,
		},
	))
	if err != nil {
		if trace.IsAlreadyExists(err) {
			s.WithField("package", licensePackage.String()).Debug("License package already exists.")
		}
		return trace.Wrap(err)
	}
	return nil
}

func configureTeleportLabels(node *ProvisionedServer, labels map[string]string, domainName string) {
	labels[ops.AdvertiseIP] = node.AdvertiseIP
	labels[ops.ServerFQDN] = node.FQDN(domainName)
	labels[ops.AppRole] = node.Role
	labels[ops.Hostname] = node.Hostname
	labels[ops.InstanceType] = node.InstanceType
	for k, v := range node.Profile.Labels {
		labels[k] = v
	}
	if labels[schema.DisplayRole] == "" {
		labels[schema.DisplayRole] = node.Profile.Description
	}
	labels[schema.ServiceLabelRole] = node.ClusterRole
}

func (s *site) addCloudConfig(config clusterconfig.Interface) (args []string) {
	if s.cloudProviderName() == "" {
		return nil
	}
	args = append(args, fmt.Sprintf("--cloud-provider=%v", s.cloudProviderName()))
	var cloudConfig string
	if config != nil {
		cloudConfig = config.GetGlobalConfig().CloudConfig
	}
	if cloudConfig != "" {
		args = append(args, fmt.Sprintf("--cloud-config=%v",
			base64.StdEncoding.EncodeToString([]byte(cloudConfig))))
	} else if s.cloudProviderName() == schema.ProviderGCE {
		args = append(args, fmt.Sprintf("--gce-node-tags=%v", s.gceNodeTags()))
	}
	return args
}

func (s *site) addClusterConfig(config clusterconfig.Interface, overrideArgs map[string]string) (args []string) {
	if config == nil {
		return nil
	}

	if config := config.GetKubeletConfig(); config != nil && len(config.Config) != 0 {
		args = append(args, fmt.Sprintf("--kubelet-config=%v",
			base64.StdEncoding.EncodeToString(config.Config)))
	}

	globalConfig := config.GetGlobalConfig()
	if globalConfig.ServiceCIDR != "" {
		overrideArgs["service-subnet"] = globalConfig.ServiceCIDR
	}
	if globalConfig.PodCIDR != "" {
		overrideArgs["pod-subnet"] = globalConfig.PodCIDR
	}
	if globalConfig.ServiceNodePortRange != "" {
		args = append(args,
			fmt.Sprintf("--service-node-portrange=%v", globalConfig.ServiceNodePortRange),
		)
	}
	if globalConfig.ProxyPortRange != "" {
		args = append(args,
			fmt.Sprintf("--proxy-portrange=%v", globalConfig.ProxyPortRange),
		)
	}
	if len(globalConfig.FeatureGates) != 0 {
		features := make([]string, 0, len(globalConfig.FeatureGates))
		for k, v := range globalConfig.FeatureGates {
			features = append(features, fmt.Sprintf("%v=%v", k, v))
		}
		args = append(args,
			fmt.Sprintf("--feature-gates=%v", strings.Join(features, ",")))
	}
	return args
}

// configureDockerOptions creates a set of Docker-specific command line arguments to Planet on the specified node
// based on the operation op and docker manifest configuration block.
func configureDockerOptions(
	op ops.SiteOperation,
	node ProvisionedServer,
	docker storage.DockerConfig,
	dockerRuntime storage.Docker,
) (args []string, err error) {
	formatOptions := func(args []string) string {
		return fmt.Sprintf(`--docker-options=%v`, strings.Join(args, " "))
	}

	args = []string{fmt.Sprintf("--docker-backend=%v", docker.StorageDriver)}

	switch docker.StorageDriver {
	case constants.DockerStorageDriverDevicemapper:
		// Override udev sync check to support devicemapper on a system with a kernel that does not support it
		// See: https://github.com/docker/docker/pull/11412
		const dmUdevSyncOverride = "--storage-opt=dm.override_udev_sync_check=1"
		const dmFilesystem = "--storage-opt=dm.fs=xfs"
		dmThinpool := fmt.Sprintf("--storage-opt=dm.thinpooldev=/dev/mapper/%v", devicemapper.PoolName)

		options := append(docker.Args, dmUdevSyncOverride)
		if dockerRuntime.Device.Path() == "" {
			// if no device has been been configured for devicemapper direct-lvm
			// use the loop-lvm mode by configuring just the storage driver
			// with no other configuration
			return append(args, formatOptions(options)), nil
		}
		systemDir := dockerRuntime.LVMSystemDirectory

		options = append(options, []string{dmFilesystem, dmThinpool}...)
		args = append(args, formatOptions(options))
		// expose directories used by LVM
		args = append(args, "--volume=/dev/mapper:/dev/mapper")
		args = append(args, "--volume=/dev/docker:/dev/docker")
		if systemDir != "" {
			args = append(args, fmt.Sprintf("--volume=%v:%v", systemDir, constants.LVMSystemDir))
		}
	case constants.DockerStorageDriverOverlay2:
		// Override kernel check to support overlay2
		// See: https://github.com/docker/docker/issues/26559
		const overlayKernelOverride = "--storage-opt=overlay2.override_kernel_check=1"

		options := append(docker.Args, []string{overlayKernelOverride}...)
		args = append(args, formatOptions(options))
	default:
		args = append(args, formatOptions(docker.Args))
	}

	return args, nil
}

func podSubnet(installExpand *storage.InstallExpandOperationState, override string) string {
	if override != "" {
		return override
	}
	if installExpand == nil || installExpand.Subnets.Overlay == "" {
		return storage.DefaultSubnets.Overlay
	}
	return installExpand.Subnets.Overlay
}

func serviceSubnet(installExpand *storage.InstallExpandOperationState, override string) string {
	if override != "" {
		return override
	}
	if installExpand == nil || installExpand.Subnets.Service == "" {
		return storage.DefaultSubnets.Service
	}
	return installExpand.Subnets.Service
}

// exportBackend defines a shim to export site information
// using various attributes of the specified site
// It implements transfer.ExportBackend
type exportBackend struct {
	*site
}

func (r *exportBackend) GetAccount(accountID string) (*storage.Account, error) {
	return r.site.backend().GetAccount(accountID)
}
func (r *exportBackend) GetSiteUsers(domain string) ([]storage.User, error) {
	return r.site.backend().GetSiteUsers(domain)
}

// GetPackage returns a package specified with (repository, packageName, packageVersion) tuple
// using a pack.PackageService to enable package discovery with layered package implementation.
func (r *exportBackend) GetPackage(repository, packageName, packageVersion string) (*storage.Package, error) {
	locator, err := loc.NewLocator(repository, packageName, packageVersion)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	envelope, err := r.site.packages().ReadPackageEnvelope(*locator)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &storage.Package{
		Repository:    repository,
		Name:          packageName,
		Version:       packageVersion,
		SHA512:        envelope.SHA512,
		SizeBytes:     int(envelope.SizeBytes),
		RuntimeLabels: envelope.RuntimeLabels,
		Type:          envelope.Type,
		Hidden:        envelope.Hidden,
		Manifest:      envelope.Manifest,
	}, nil
}
func (r *exportBackend) GetAPIKeys(email string) ([]storage.APIKey, error) {
	return r.site.backend().GetAPIKeys(email)
}
func (r *exportBackend) GetUserRoles(email string) ([]teleservices.Role, error) {
	return r.site.backend().GetUserRoles(email)
}
func (r *exportBackend) GetSiteOperations(domain string) ([]storage.SiteOperation, error) {
	return r.site.backend().GetSiteOperations(domain)
}
func (r *exportBackend) GetTrustedClusters() ([]teleservices.TrustedCluster, error) {
	return r.site.backend().GetTrustedClusters()
}
func (r *exportBackend) GetSiteProvisioningTokens(domain string) ([]storage.ProvisioningToken, error) {
	return r.site.backend().GetSiteProvisioningTokens(domain)
}
func (r *exportBackend) GetLastProgressEntry(domain, operationID string) (*storage.ProgressEntry, error) {
	return r.site.backend().GetLastProgressEntry(domain, operationID)
}
func (r *exportBackend) GetOperationPlan(domain, operationID string) (*storage.OperationPlan, error) {
	return r.site.backend().GetOperationPlan(domain, operationID)
}
