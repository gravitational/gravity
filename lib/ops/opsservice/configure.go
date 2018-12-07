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
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/user"
	"path/filepath"
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
	"github.com/gravitational/gravity/lib/transfer"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/cloudflare/cfssl/csr"
	"github.com/gravitational/configure"
	"github.com/gravitational/license/authority"
	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth/native"
	teleetcd "github.com/gravitational/teleport/lib/backend/etcdbk"
	telecfg "github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/lib/services"
	teleservices "github.com/gravitational/teleport/lib/services"
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
func (o *Operator) ConfigurePackages(key ops.SiteOperationKey) error {
	log.Infof("Configuring packages: %#v.", key)
	operation, err := o.GetSiteOperation(key)
	if err != nil {
		return trace.Wrap(err)
	}
	switch operation.Type {
	case ops.OperationInstall, ops.OperationExpand:
	default:
		return trace.BadParameter("expected install or expand operation, got: %v",
			operation)
	}
	site, err := o.openSite(key.SiteKey())
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
		err = site.configurePackages(ctx)
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

func (s *site) getTeleportMaster(ctx context.Context) (*teleportServer, error) {
	masters, err := s.teleport().GetServers(ctx, s.domainName, map[string]string{
		schema.ServiceLabelRole: string(schema.ServiceRoleMaster),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(masters) == 0 {
		return nil, trace.NotFound("no master server found")
	}
	return newTeleportServer(masters[0])
}

func (s *site) configureExpandPackages(ctx context.Context, opCtx *operationContext) error {
	teleportCA, err := s.getTeleportSecrets()
	if err != nil {
		return trace.Wrap(err)
	}
	teleportMaster, err := s.getTeleportMaster(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	provisionedServer := opCtx.provisionedServers[0]
	etcdConfig, err := s.getEtcdConfig(ctx, opCtx, provisionedServer)
	if err != nil {
		return trace.Wrap(err)
	}
	secretsPackage, err := s.planetSecretsPackage(provisionedServer)
	if err != nil {
		return trace.Wrap(err)
	}
	planetPackage, err := s.app.Manifest.RuntimePackage(provisionedServer.Profile)
	if err != nil {
		return trace.Wrap(err)
	}
	configPackage, err := s.planetConfigPackage(provisionedServer, planetPackage.Version)
	if err != nil {
		return trace.Wrap(err)
	}
	planetConfig := planetConfig{
		etcd:          *etcdConfig,
		docker:        s.dockerConfig(),
		planetPackage: *planetPackage,
		configPackage: *configPackage,
	}
	if provisionedServer.IsMaster() {
		err := s.configureTeleportMaster(opCtx, teleportCA, provisionedServer)
		if err != nil {
			return trace.Wrap(err)
		}
		masterParams := planetMasterParams{
			master:            provisionedServer,
			secretsPackage:    secretsPackage,
			serviceSubnetCIDR: opCtx.operation.InstallExpand.Subnets.Service,
		}
		// if we have connection to an Ops Center set up, configure
		// SNI host so it can dial in
		trustedCluster, err := storage.GetTrustedCluster(s.backend())
		if err == nil {
			masterParams.sniHost = trustedCluster.GetSNIHost()
		}
		err = s.configurePlanetMasterSecrets(opCtx, masterParams)
		if err != nil {
			return trace.Wrap(err)
		}
		planetConfig.master = masterConfig{
			electionEnabled: false,
			addr:            s.teleport().GetPlanetLeaderIP(),
		}
		err = s.configurePlanetMaster(provisionedServer, opCtx.operation,
			planetConfig, *secretsPackage, *configPackage)
		if err != nil {
			return trace.Wrap(err)
		}
	} else {
		err := s.configurePlanetNodeSecrets(opCtx, provisionedServer, secretsPackage)
		if err != nil {
			return trace.Wrap(err)
		}
		err = s.configurePlanetNode(provisionedServer, opCtx.operation,
			planetConfig, *secretsPackage, *configPackage)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	err = s.configureTeleportKeyPair(teleportCA, provisionedServer, teleport.RoleNode)
	if err != nil {
		return trace.Wrap(err)
	}
	err = s.configureTeleportNode(opCtx, teleportMaster.IP, provisionedServer)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (s *site) configurePackages(ctx *operationContext) error {
	err := s.packages().UpsertRepository(s.siteRepoName(), time.Time{})
	if err != nil {
		return trace.Wrap(err)
	}

	p := ctx.provisionedServers

	if err := s.configurePlanetCertAuthority(ctx); err != nil {
		return trace.Wrap(err)
	}

	// configure teleport master keys and secrets
	teleportCA, err := s.initTeleportCertAuthority()
	if err != nil {
		return trace.Wrap(err)
	}

	for _, node := range ctx.provisionedServers {
		node.PackageSet.AddPackage(
			s.gravityPackage, map[string]string{pack.InstalledLabel: pack.InstalledLabel})

		err := s.configureTeleportKeyPair(teleportCA, node, teleport.RoleNode)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	siteExportPackage, err := s.configureSiteExportPackage(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	licensePackage, err := s.configureLicensePackage(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	var resourcesPackage *loc.Locator
	if s.hasResources() {
		resourcesPackage, err = s.configureResourcesPackage(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
		s.Debugf("configured resources package %v", resourcesPackage.String())
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

	for i, master := range masters {
		secretsPackage, err := s.planetSecretsPackage(master)
		if err != nil {
			return trace.Wrap(err)
		}

		planetPackage, err := s.app.Manifest.RuntimePackage(master.Profile)
		if err != nil {
			return trace.Wrap(err)
		}

		configPackage, err := s.planetConfigPackage(master, planetPackage.Version)
		if err != nil {
			return trace.Wrap(err)
		}

		err = s.configurePlanetMasterSecrets(ctx, planetMasterParams{
			master:            master,
			secretsPackage:    secretsPackage,
			serviceSubnetCIDR: ctx.operation.InstallExpand.Subnets.Service,
			sniHost:           s.service.cfg.SNIHost,
		})
		if err != nil {
			return trace.Wrap(err)
		}

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
			etcd:          masterEtcdConfig,
			master:        masterConfig,
			docker:        s.dockerConfig(),
			planetPackage: *planetPackage,
			configPackage: *configPackage,
		}
		err = s.configurePlanetMaster(master, ctx.operation, config, *secretsPackage, *configPackage)
		if err != nil {
			return trace.Wrap(err)
		}

		if err := s.configureTeleportMaster(ctx, teleportCA, master); err != nil {
			return trace.Wrap(err)
		}

		if err := s.configureTeleportNode(ctx, activeMaster.AdvertiseIP, master); err != nil {
			return trace.Wrap(err)
		}

		master.PackageSet.AddArchivePackages(s.webAssetsPackage)
		master.PackageSet.AddPackages(*planetPackage)
		master.PackageSet.AddPackages(*siteExportPackage)
		if licensePackage != nil {
			master.PackageSet.AddPackages(*licensePackage)
		}
		if s.hasResources() {
			master.PackageSet.AddPackages(*resourcesPackage)
		}
		if err := s.configureUserApp(master); err != nil {
			return trace.Wrap(err)
		}
	}

	for _, node := range p.Nodes() {
		if err := s.configureTeleportNode(ctx, p.FirstMaster().AdvertiseIP, node); err != nil {
			return trace.Wrap(err)
		}

		secretsPackage, err := s.planetSecretsPackage(node)
		if err != nil {
			return trace.Wrap(err)
		}

		if err := s.configurePlanetNodeSecrets(ctx, node, secretsPackage); err != nil {
			return trace.Wrap(err)
		}

		planetPackage, err := s.app.Manifest.RuntimePackage(node.Profile)
		if err != nil {
			return trace.Wrap(err)
		}

		configPackage, err := s.planetConfigPackage(node, planetPackage.Version)
		if err != nil {
			return trace.Wrap(err)
		}

		nodeEtcdConfig, ok := etcdConfig[node.AdvertiseIP]
		if !ok {
			return trace.NotFound("etcd config not found for %v: %v",
				node.AdvertiseIP, etcdConfig)
		}

		config := planetConfig{
			etcd:          nodeEtcdConfig,
			master:        masterConfig{addr: p.FirstMaster().AdvertiseIP},
			docker:        s.dockerConfig(),
			planetPackage: *planetPackage,
			configPackage: *configPackage,
		}

		err = s.configurePlanetNode(node, ctx.operation, config, *secretsPackage, *configPackage)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
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

// configureUserApp adds the application being installed as well as all its dependencies to
// the package set of master servers
func (s *site) configureUserApp(server *ProvisionedServer) error {
	var apps []loc.Locator

	for _, dep := range s.app.Manifest.Dependencies.Apps {
		apps = append(apps, dep.Locator)
	}

	appPackage, err := s.appPackage()
	if err != nil {
		return trace.Wrap(err)
	}

	server.PackageSet.AddApps(append(apps, *appPackage)...)
	return nil
}

func (s *site) configurePlanetCertAuthority(ctx *operationContext) error {
	caPackage, err := s.planetCertAuthorityPackage()
	if err != nil {
		return trace.Wrap(err)
	}

	if _, err := s.packages().ReadPackageEnvelope(*caPackage); err == nil {
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

	_, err = s.packages().CreatePackage(*caPackage, reader, pack.WithLabels(
		map[string]string{
			pack.PurposeLabel:     pack.PurposeCA,
			pack.OperationIDLabel: ctx.operation.ID,
		}))
	return trace.Wrap(err)
}

// ReadCertAuthorityPackage returns the certificate authority package for
// the specified cluster
func ReadCertAuthorityPackage(packages pack.PackageService, clusterName string) (utils.TLSArchive, error) {
	caPackage, err := PlanetCertAuthorityPackage(clusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	_, reader, err := packages.ReadPackage(*caPackage)
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
	master            *ProvisionedServer
	secretsPackage    *loc.Locator
	serviceSubnetCIDR string
	sniHost           string
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

	baseKeyPair, err := archive.GetKeyPair(constants.APIServerKeyPair)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	serviceSubnet, err := configure.ParseCIDR(p.serviceSubnetCIDR)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	apiServerIP := serviceSubnet.FirstIP().String()

	newArchive := make(utils.TLSArchive)

	caCertKeyPair := *caKeyPair
	caCertKeyPair.KeyPEM = nil

	if err := newArchive.AddKeyPair(constants.RootKeyPair, caCertKeyPair); err != nil {
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
		constants.RuntimeAgentKeyPair:           {},
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
		switch name {
		case constants.APIServerKeyPair:
			req.Hosts = append(req.Hosts,
				constants.APIServerDomainName,
				constants.LegacyAPIServerDomainName,
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
		case constants.ProxyKeyPair:
			req.Hosts = append(req.Hosts, constants.APIServerDomainName)
		}
		keyPair, err := authority.GenerateCertificate(req, caKeyPair, baseKeyPair.KeyPEM, defaults.CertificateExpiry)
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

func (s *site) getPlanetNodeSecretsPackage(ctx *operationContext, node *ProvisionedServer, secretsPackage *loc.Locator) (*ops.RotatePackageResponse, error) {
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
		constants.APIServerKeyPair:    {},
		constants.ETCDKeyPair:         {},
		constants.KubectlKeyPair:      {group: constants.ClusterNodeGroup},
		constants.ProxyKeyPair:        {userName: constants.ClusterKubeProxyUser, group: constants.ClusterNodeGroup},
		constants.KubeletKeyPair:      {userName: constants.ClusterNodeNamePrefix + ":" + node.KubeNodeID(), group: constants.ClusterNodeGroup},
		constants.PlanetRpcKeyPair:    {},
		constants.CoreDNSKeyPair:      {},
		constants.RuntimeAgentKeyPair: {},
	}

	var privateKeyPEM []byte
	for keyName, config := range keyPairTypes {
		req := csr.CertificateRequest{
			Hosts: []string{constants.LoopbackIP, node.AdvertiseIP, node.Hostname},
		}
		if keyName == constants.ProxyKeyPair {
			req.Hosts = append(req.Hosts, constants.APIServerDomainName)
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
		keyPair, err := authority.GenerateCertificate(req, caKeyPair, privateKeyPEM, defaults.CertificateExpiry)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		// Store the private key from the first generated key pair and re-use
		// it on subsequent requests to speed up certificate generation
		if len(privateKeyPEM) == 0 {
			privateKeyPEM = keyPair.KeyPEM
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
		Locator: *secretsPackage,
		Reader:  reader,
		Labels:  labels,
	}, nil
}

func (s *site) configurePlanetNodeSecrets(ctx *operationContext, node *ProvisionedServer, secretsPackage *loc.Locator) error {
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

func (s *site) configurePlanetMaster(
	master *ProvisionedServer,
	installOrExpand ops.SiteOperation,
	config planetConfig,
	secretsPackage, configPackage loc.Locator,
) error {
	if server := installOrExpand.InstallExpand.Servers.FindByIP(master.AdvertiseIP); server != nil {
		config.dockerRuntime = server.Docker
	}

	err := s.configurePlanetServer(master, installOrExpand, config)
	if err != nil {
		return trace.Wrap(err)
	}

	certAuthorityPackage, err := s.planetCertAuthorityPackage()
	if err != nil {
		return trace.Wrap(err)
	}

	master.PackageSet.AddArchivePackages(*certAuthorityPackage)
	master.PackageSet.AddArchivePackage(secretsPackage,
		map[string]string{pack.InstalledLabel: pack.InstalledLabel})
	master.PackageSet.AddArchivePackage(
		config.planetPackage, map[string]string{
			pack.InstalledLabel: pack.InstalledLabel,
			pack.PurposeLabel:   pack.PurposeRuntime,
		})
	master.PackageSet.AddArchivePackage(configPackage,
		pack.ConfigLabels(config.planetPackage, pack.PurposePlanetConfig))
	return nil
}

func (s *site) configurePlanetNode(
	node *ProvisionedServer,
	installOrExpand ops.SiteOperation,
	config planetConfig,
	secretsPackage, configPackage loc.Locator,
) error {
	if server := installOrExpand.InstallExpand.Servers.FindByIP(node.AdvertiseIP); server != nil {
		config.dockerRuntime = server.Docker
	}

	err := s.configurePlanetServer(node, installOrExpand, config)
	if err != nil {
		return trace.Wrap(err)
	}

	node.PackageSet.AddArchivePackage(secretsPackage, map[string]string{
		pack.InstalledLabel: pack.InstalledLabel,
	})
	node.PackageSet.AddArchivePackage(config.planetPackage, map[string]string{
		pack.InstalledLabel: pack.InstalledLabel,
		pack.PurposeLabel:   pack.PurposeRuntime,
	})
	node.PackageSet.AddArchivePackage(configPackage,
		pack.ConfigLabels(config.planetPackage, pack.PurposePlanetConfig))
	return nil
}

func (s *site) getPlanetConfigPackage(
	node *ProvisionedServer,
	installOrExpand ops.SiteOperation,
	config planetConfig,
	manifest schema.Manifest) (*ops.RotatePackageResponse, error) {
	profile, err := manifest.NodeProfiles.ByName(node.Role)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	args := []string{
		fmt.Sprintf("--node-name=%v", node.KubeNodeID()),
		fmt.Sprintf("--hostname=%v", node.Hostname),
		fmt.Sprintf("--master-ip=%v", config.master.addr),
		fmt.Sprintf("--public-ip=%v", node.AdvertiseIP),
		fmt.Sprintf("--service-subnet=%v", installOrExpand.InstallExpand.Subnets.Service),
		fmt.Sprintf("--pod-subnet=%v", installOrExpand.InstallExpand.Subnets.Overlay),
		fmt.Sprintf("--cluster-id=%v", s.domainName),
		fmt.Sprintf("--etcd-proxy=%v", config.etcd.proxyMode),
		fmt.Sprintf("--etcd-member-name=%v", node.EtcdMemberName(s.domainName)),
		fmt.Sprintf("--initial-cluster=%v", config.etcd.initialCluster),
		fmt.Sprintf("--secrets-dir=%v", node.InGravity(defaults.SecretsDir)),
		fmt.Sprintf("--etcd-initial-cluster-state=%v", config.etcd.initialClusterState),
		fmt.Sprintf("--election-enabled=%v", config.master.electionEnabled),
		fmt.Sprintf("--volume=%v:/ext/etcd", node.InGravity("planet", "etcd")),
		fmt.Sprintf("--volume=%v:/ext/registry", node.InGravity("planet", "registry")),
		fmt.Sprintf("--volume=%v:/ext/docker", node.InGravity("planet", "docker")),
		fmt.Sprintf("--volume=%v:/ext/share", node.InGravity("planet", "share")),
		fmt.Sprintf("--volume=%v:/ext/state", node.InGravity("planet", "state")),
		fmt.Sprintf("--volume=%v:/var/log", node.InGravity("planet", "log")),
		fmt.Sprintf("--volume=%v:%v", node.StateDir(), defaults.GravityDir),
		fmt.Sprintf("--service-uid=%v", s.uid()),
	}

	if node.IsMaster() {
		args = append(args, "--role=master")
	} else {
		args = append(args, "--role=node")
	}
	args = append(args, manifest.RuntimeArgs(*profile)...)

	if manifest.HairpinMode(*profile) == constants.HairpinModePromiscuousBridge {
		args = append(args, "--docker-promiscuous-mode=true")
	}

	if s.cloudProviderName() != "" {
		args = append(args, fmt.Sprintf("--cloud-provider=%v", s.cloudProviderName()))
		if s.cloudProviderName() == schema.ProviderGCE {
			args = append(args, fmt.Sprintf("--gce-node-tags=%v", s.gceNodeTags()))
		}
	}

	if len(s.backendSite.DNSOverrides.Hosts) > 0 {
		args = append(args, fmt.Sprintf("--dns-hosts=%v",
			s.backendSite.DNSOverrides.FormatHosts()))
	}

	if len(s.backendSite.DNSOverrides.Zones) > 0 {
		args = append(args, fmt.Sprintf("--dns-zones=%v",
			s.backendSite.DNSOverrides.FormatZones()))
	}

	vxlanPort := installOrExpand.InstallExpand.Vars.OnPrem.VxlanPort
	if vxlanPort != 0 {
		args = append(args, fmt.Sprintf("--vxlan-port=%v", vxlanPort))
	}

	dnsConfig := s.dnsConfig()
	for _, addr := range dnsConfig.Addrs {
		args = append(args, fmt.Sprintf("--dns-listen-addr=%v", addr))
	}
	args = append(args, fmt.Sprintf("--dns-port=%v", dnsConfig.Port))

	dockerArgs, err := configureDockerOptions(&installOrExpand, node,
		config.docker, config.dockerRuntime)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	args = append(args, dockerArgs...)

	etcdArgs := manifest.EtcdArgs(*profile)
	if len(etcdArgs) != 0 {
		args = append(args, fmt.Sprintf("--etcd-options=%v", strings.Join(etcdArgs, " ")))
	}

	kubeletArgs := defaults.KubeletArgs
	if len(manifest.KubeletArgs(*profile)) != 0 {
		kubeletArgs = append(kubeletArgs, manifest.KubeletArgs(*profile)...)
	}

	switch manifest.HairpinMode(*profile) {
	case constants.HairpinModeVeth:
		kubeletArgs = append(kubeletArgs, "--hairpin-mode=hairpin-veth")
	case constants.HairpinModePromiscuousBridge:
		// Turn off kubelet's hairpin mode if promiscuous-bridge is requested.
		// This mode manually configures promiscuous mode on the docker bridge
		// and creates ebtables rules to de-duplicate packets
		kubeletArgs = append(kubeletArgs, "--hairpin-mode=none")
	}

	args = append(args, fmt.Sprintf("--kubelet-options=%v", strings.Join(kubeletArgs, " ")))

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

	reader, err := pack.GetConfigPackage(s.packages(), config.planetPackage, config.configPackage, args)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	labels := map[string]string{
		pack.PurposeLabel:     pack.PurposePlanetConfig,
		pack.ConfigLabel:      config.planetPackage.ZeroVersion().String(),
		pack.AdvertiseIPLabel: node.AdvertiseIP,
		pack.OperationIDLabel: installOrExpand.ID,
	}

	return &ops.RotatePackageResponse{
		Locator: config.configPackage,
		Reader:  reader,
		Labels:  labels,
	}, nil
}

func (s *site) configurePlanetServer(node *ProvisionedServer, installOrExpand ops.SiteOperation, config planetConfig) error {
	resp, err := s.getPlanetConfigPackage(node, installOrExpand, config, s.app.Manifest)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = s.packages().CreatePackage(resp.Locator, resp.Reader, pack.WithLabels(resp.Labels))
	return trace.Wrap(err)
}

type planetConfig struct {
	etcd          etcdConfig
	master        masterConfig
	docker        storage.DockerConfig
	dockerRuntime storage.Docker
	planetPackage loc.Locator
	configPackage loc.Locator
}

func (s *site) configureTeleportMaster(ctx *operationContext, secrets *teleportSecrets, master *ProvisionedServer) error {
	configPackage, err := s.teleportMasterConfigPackage(master)
	if err != nil {
		return trace.Wrap(err)
	}

	fileConf := &telecfg.FileConfig{}

	// trust host and user keys of our portal
	proxyAuthorities, err := s.teleport().CertAuthorities(false)
	if err != nil {
		return trace.Wrap(err)
	}
	fileConf.Auth.Authorities = make([]telecfg.Authority, 0, len(proxyAuthorities)+2)

	currentUser, err := user.Current()
	if err != nil {
		log.Warningf("failed to query current user: %v", trace.ConvertSystemError(err))
	}

	for _, a := range proxyAuthorities {
		authority := telecfg.Authority{
			Type:          a.GetType(),
			DomainName:    a.GetClusterName(),
			CheckingKeys:  make([]string, 0, len(a.GetCheckingKeys())),
			AllowedLogins: storage.GetAllowedLogins(currentUser),
		}
		for _, key := range a.GetCheckingKeys() {
			authority.CheckingKeys = append(authority.CheckingKeys, string(key))
		}
		fileConf.Auth.Authorities = append(fileConf.Auth.Authorities, authority)
	}

	if secrets != nil {
		// teleport master has pre-configured keypair that we've used
		// to sign certs for other nodes
		fileConf.Auth.Authorities = append(
			fileConf.Auth.Authorities,
			telecfg.Authority{
				Type:         services.HostCA,
				DomainName:   s.domainName,
				SigningKeys:  []string{string(secrets.HostCAPrivateKey)},
				CheckingKeys: []string{string(secrets.HostCAPublicKey)},
			})

		fileConf.Auth.Authorities = append(
			fileConf.Auth.Authorities,
			telecfg.Authority{
				Type:         services.UserCA,
				DomainName:   s.domainName,
				SigningKeys:  []string{string(secrets.UserCAPrivateKey)},
				CheckingKeys: []string{string(secrets.UserCAPublicKey)},
			})
	}

	dynamicConfig := true
	fileConf.Auth.DynamicConfig = &dynamicConfig
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
		return trace.Wrap(err)
	}
	fileConf.Storage.Params = params

	fileConf.SSH.Labels = map[string]string{}

	configureTeleportLabels(master, &ctx.operation, fileConf.SSH.Labels, s.domainName)

	for key, val := range master.Profile.Labels {
		fileConf.SSH.Labels[key] = val
	}

	fileConf.AdvertiseIP = net.ParseIP(master.AdvertiseIP)
	fileConf.Global.NodeName = master.FQDN(s.domainName)

	// turn on auth service
	fileConf.Auth.EnabledFlag = "yes"
	fileConf.Auth.ClusterName = telecfg.ClusterName(s.domainName)

	// turn on proxy
	fileConf.Proxy.EnabledFlag = "yes"

	// turn off SSH - we won't SSH into container with Gravity running
	fileConf.SSH.EnabledFlag = "no"

	bytes, err := yaml.Marshal(fileConf)
	if err != nil {
		return trace.Wrap(err)
	}

	args := []string{
		fmt.Sprintf("--config-string=%v", base64.StdEncoding.EncodeToString(bytes)),
	}

	err = pack.ConfigurePackage(
		s.packages(), s.teleportPackage, *configPackage, args, map[string]string{
			pack.PurposeLabel:     pack.PurposeTeleportConfig,
			pack.AdvertiseIPLabel: master.AdvertiseIP,
			pack.OperationIDLabel: ctx.operation.ID,
		})
	if err != nil {
		return trace.Wrap(err)
	}

	master.PackageSet.AddArchivePackage(*configPackage, nil)

	return nil
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

func (s *site) teleportMasterConfigPackage(master remoteServer) (*loc.Locator, error) {
	configPackage, err := loc.ParseLocator(
		fmt.Sprintf("%v/%v:0.0.1-%v", s.siteRepoName(), constants.TeleportMasterConfigPackage,
			PackageSuffix(master, s.domainName)))
	return configPackage, trace.Wrap(err)
}

func (s *site) configureTeleportNode(ctx *operationContext, masterIP string, node *ProvisionedServer) error {
	configPackage, err := s.teleportNodeConfigPackage(node)
	if err != nil {
		return trace.Wrap(err)
	}

	fileConf := &telecfg.FileConfig{}

	fileConf.DataDir = node.InGravity("teleport")

	fileConf.AuthServers = []string{fmt.Sprintf("%v:3025", masterIP)}

	fileConf.SSH.Labels = map[string]string{}

	configureTeleportLabels(node, &ctx.operation, fileConf.SSH.Labels, s.domainName)

	// for AWS sites, use dynamic teleport labels to periodically query AWS metadata
	// for servers' public IPs
	if s.cloudProviderName() == schema.ProviderAWS {
		fileConf.SSH.Commands = append(fileConf.SSH.Commands, telecfg.CommandLabel{
			Name:    defaults.TeleportPublicIPv4Label,
			Command: defaults.AWSPublicIPv4Command,
			Period:  defaults.TeleportCommandLabelInterval,
		})
	}

	for key, val := range node.Profile.Labels {
		fileConf.SSH.Labels[key] = val
	}

	// never expire cache
	fileConf.CachePolicy.EnabledFlag = "yes"
	// FIXME(klizhentas) "never" is not working in current
	// version of teleport, switch to "never" once fixed.
	// for now set to 365 days
	fileConf.CachePolicy.TTL = fmt.Sprintf("%v", 365*24*time.Hour)

	fileConf.AdvertiseIP = net.ParseIP(node.AdvertiseIP)
	fileConf.Global.NodeName = node.FQDN(s.domainName)

	// turn off auth service and proxy, turn on SSH
	fileConf.Auth.EnabledFlag = "no"
	fileConf.Proxy.EnabledFlag = "no"
	fileConf.SSH.EnabledFlag = "yes"

	fileConf.Keys = []telecfg.KeyPair{
		{
			PrivateKey: string(node.keyPair.PrivateKey),
			Cert:       string(node.keyPair.Cert),
		},
	}

	bytes, err := yaml.Marshal(fileConf)
	if err != nil {
		return trace.Wrap(err)
	}

	args := []string{
		fmt.Sprintf("--config-string=%v", base64.StdEncoding.EncodeToString(bytes)),
	}

	err = pack.ConfigurePackage(
		s.packages(), s.teleportPackage, *configPackage, args, map[string]string{
			pack.PurposeLabel:     pack.PurposeTeleportConfig,
			pack.AdvertiseIPLabel: node.AdvertiseIP,
			pack.OperationIDLabel: ctx.operation.ID,
		})
	if err != nil {
		return trace.Wrap(err)
	}

	node.PackageSet.AddArchivePackage(s.teleportPackage, map[string]string{pack.InstalledLabel: pack.InstalledLabel})
	node.PackageSet.AddArchivePackage(*configPackage, map[string]string{pack.ConfigLabel: s.teleportPackage.ZeroVersion().String()})

	return nil
}

func (s *site) teleportNodeConfigPackage(node remoteServer) (*loc.Locator, error) {
	configPackage, err := loc.ParseLocator(
		fmt.Sprintf("%v/%v:0.0.1-%v", s.siteRepoName(), constants.TeleportNodeConfigPackage,
			PackageSuffix(node, s.domainName)))
	return configPackage, trace.Wrap(err)
}

func (s *site) configureResourcesPackage(ctx *operationContext) (*loc.Locator, error) {
	resourcesPackage, err := s.resourcesPackage()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	_, err = s.packages().CreatePackage(
		*resourcesPackage, bytes.NewBuffer(s.resources), pack.WithLabels(
			map[string]string{
				pack.PurposeLabel:     pack.PurposeResources,
				pack.OperationIDLabel: ctx.operation.ID,
			},
		))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resourcesPackage, nil
}

func (s *site) configureSiteExportPackage(ctx *operationContext) (*loc.Locator, error) {
	exportPackage, err := s.siteExportPackage()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	exportDir := s.siteDir("export")
	if err := os.MkdirAll(exportDir, defaults.PrivateDirMask); err != nil {
		return nil, trace.Wrap(err)
	}

	site, err := s.backend().GetSite(s.key.SiteDomain)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	reader, err := transfer.ExportSite(site, &exportBackend{s}, exportDir,
		s.seedConfig.TrustedClusters)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer func() {
		reader.Close()
		if err := os.RemoveAll(exportDir); err != nil {
			log.Warningf("failed to delete temporary export directory %v: %v", exportDir, err)
		}
	}()

	_, err = s.packages().CreatePackage(*exportPackage, reader, pack.WithLabels(
		map[string]string{
			pack.PurposeLabel:     pack.PurposeExport,
			pack.OperationIDLabel: ctx.operation.ID,
		},
	))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return exportPackage, nil
}

func (s *site) configureLicensePackage(ctx *operationContext) (*loc.Locator, error) {
	if s.license == "" {
		return nil, nil // nothing to do
	}

	licensePackage, err := s.licensePackage()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	reader := strings.NewReader(s.license)
	_, err = s.packages().CreatePackage(*licensePackage, reader, pack.WithLabels(
		map[string]string{
			pack.PurposeLabel:     pack.PurposeLicense,
			pack.OperationIDLabel: ctx.operation.ID,
		},
	))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return licensePackage, nil
}

func configureTeleportLabels(node *ProvisionedServer, operation *ops.SiteOperation, labels map[string]string, domainName string) {
	labels[ops.AdvertiseIP] = node.AdvertiseIP
	labels[ops.ServerFQDN] = node.FQDN(domainName)
	labels[ops.AppRole] = node.Role
	labels[ops.Hostname] = node.Hostname

	state := operation.InstallExpand
	if state == nil {
		return
	}

	profile := state.Profiles[node.Role]
	displayRole := profile.Labels[schema.DisplayRole]
	if displayRole == "" {
		displayRole = profile.Description
	}

	labels[schema.DisplayRole] = displayRole
	labels[ops.InstanceType] = node.InstanceType
}

func (s *site) initTeleportCertAuthority() (*teleportSecrets, error) {
	hostPriv, hostPub, err := native.New().GenerateKeyPair("")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	userPriv, userPub, err := native.New().GenerateKeyPair("")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	secrets := &teleportSecrets{
		HostCAPublicKey:  hostPub,
		HostCAPrivateKey: hostPriv,
		UserCAPublicKey:  userPub,
		UserCAPrivateKey: userPriv,
	}

	// make sure teleport proxy trusts the keys we've just generated
	err = s.teleport().TrustCertAuthority(
		services.NewCertAuthority(
			services.UserCA,
			s.domainName,
			nil,
			[][]byte{userPub},
			nil,
		))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = s.teleport().TrustCertAuthority(services.NewCertAuthority(
		services.HostCA,
		s.domainName,
		nil,
		[][]byte{hostPub},
		nil))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return secrets, nil
}

// configureTeleportKeyPair configures private key and a cert signed by the correct authority
func (s *site) configureTeleportKeyPair(secrets *teleportSecrets, server *ProvisionedServer, role teleport.Role) error {
	unqualifiedName := server.UnqualifiedName(s.domainName)

	log.Infof("%v going to generate token for %v", s, unqualifiedName)

	auth := native.New()

	hostPriv, hostPub, err := auth.GenerateKeyPair("")
	if err != nil {
		return trace.Wrap(err)
	}
	cert, err := auth.GenerateHostCert(teleservices.HostCertParams{
		PrivateCASigningKey: secrets.HostCAPrivateKey,
		PublicHostKey:       hostPub,
		HostID:              unqualifiedName,
		ClusterName:         s.domainName,
		Roles:               teleport.Roles{role},
	})
	if err != nil {
		return trace.Wrap(err)
	}

	server.keyPair = &teleportKeyPair{Cert: cert, PrivateKey: hostPriv}
	return nil
}

// PlanetCertAuthorityPackage returns the name of the planet CA package
func PlanetCertAuthorityPackage(clusterName string) (*loc.Locator, error) {
	return loc.ParseLocator(
		fmt.Sprintf("%v/%v:0.0.1", clusterName, constants.CertAuthorityPackage))
}

func (s *site) planetCertAuthorityPackage() (*loc.Locator, error) {
	return PlanetCertAuthorityPackage(s.siteRepoName())
}

// opsCertAuthorityPackage is a shorthand to return locator for OpsCenter's certificate
// authority package
func (s *site) opsCertAuthorityPackage() (*loc.Locator, error) {
	return loc.ParseLocator(
		fmt.Sprintf("%v/%v", defaults.SystemAccountOrg, constants.OpsCenterCAPackage))
}

// siteExport package exports site state as BoltDB database dump
func (s *site) siteExportPackage() (*loc.Locator, error) {
	return loc.ParseLocator(
		fmt.Sprintf("%v/%v:0.0.1", s.siteRepoName(), constants.SiteExportPackage))
}

func (s *site) licensePackage() (*loc.Locator, error) {
	return loc.ParseLocator(
		fmt.Sprintf("%v/%v:0.0.1", s.siteRepoName(), constants.LicensePackage))
}

func (s *site) planetSecretsPackage(node *ProvisionedServer) (*loc.Locator, error) {
	return loc.ParseLocator(
		fmt.Sprintf("%v/planet-%v-secrets:0.0.1", s.siteRepoName(), node.AdvertiseIP))
}

func (s *site) planetSecretsNextPackage(node *ProvisionedServer) (*loc.Locator, error) {
	return loc.ParseLocator(
		fmt.Sprintf("%v/planet-%v-secrets:0.0.%v", s.siteRepoName(), node.AdvertiseIP, time.Now().UTC().Unix()))
}

func (s *site) resourcesPackage() (*loc.Locator, error) {
	return loc.ParseLocator(
		fmt.Sprintf("%v/resources:0.0.1", s.siteRepoName()))
}

// planetConfigPackage creates a planet configuration package reference
// using the specified version as a package version and the given node to add unique
// suffix to the name.
// This is in contrast to the old naming with PackageSuffix used as a prerelease part
// of the version which made them hard to match when looking for an update.
func (s *site) planetConfigPackage(node remoteServer, version string) (*loc.Locator, error) {
	return loc.ParseLocator(
		fmt.Sprintf("%v/%v-%v:%v", s.siteRepoName(), constants.PlanetConfigPackage,
			PackageSuffix(node, s.domainName), version))
}

// serverPackages returns a list of package locators specific to the provided server
func (s *site) serverPackages(server *ProvisionedServer) ([]loc.Locator, error) {
	masterConfigPackage, err := s.teleportMasterConfigPackage(server)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	nodeConfigPackage, err := s.teleportNodeConfigPackage(server)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	planetSecretsPackage, err := s.planetSecretsPackage(server)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	planetPackage, err := s.app.Manifest.RuntimePackageForProfile(server.Role)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	planetConfigPackage, err := s.planetConfigPackage(server, planetPackage.Version)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return []loc.Locator{
		*masterConfigPackage,
		*nodeConfigPackage,
		*planetSecretsPackage,
		*planetConfigPackage,
	}, nil
}

func NewPackageSet() *PackageSet {
	return &PackageSet{
		packages: make([]packageEnvelope, 0),
	}
}

type packageEnvelope struct {
	loc    loc.Locator
	labels map[string]string
	// archive specifies if this package is archive
	// that should be unpacked during install process
	archive bool
}

func (p *packageEnvelope) labelFlag() string {
	kv := configure.KeyVal(p.labels)
	return kv.String()
}

// PackageSet defines a collection of packages and applications
type PackageSet struct {
	packages []packageEnvelope
	apps     []packageEnvelope
}

// AddArchivePackages adds packages specified with ps to this set as archive packages
func (s *PackageSet) AddArchivePackages(ps ...loc.Locator) {
	for _, p := range ps {
		s.AddArchivePackage(p, nil)
	}
}

// AddApps adds appications specified with apps to this set
func (s *PackageSet) AddApps(apps ...loc.Locator) {
	for _, app := range apps {
		s.AddApp(app, nil)
	}
}

// AddPackages adds packages specified with packages to this set
// AddArchivePackage adds a package specified with p as an archive package
func (s *PackageSet) AddArchivePackage(p loc.Locator, labels map[string]string) {
	addPackage(p, labels, &s.packages, true)
}

// AddPackages adds packages specified with packages to this set
func (s *PackageSet) AddPackages(packages ...loc.Locator) {
	for _, p := range packages {
		s.AddPackage(p, nil)
	}
}

// AddPackage adds a package specified with p with given labels to this set
func (s *PackageSet) AddPackage(p loc.Locator, labels map[string]string) {
	addPackage(p, labels, &s.packages, false)
}

// GetPackage returns the package envelope for the package specified with packageName.
// Returns trace.NotFound if no package with the specified name exists
func (s *PackageSet) GetPackage(packageName string) (*packageEnvelope, error) {
	for _, p := range s.packages {
		if p.loc.Name == packageName {
			return &p, nil
		}
	}
	return nil, trace.NotFound("package with name %v not found", packageName)
}

// GetPackageByLabels returns the package envelope for the package specified with labels.
// Returns trace.NotFound if no package with the specified set of labels exists
func (s *PackageSet) GetPackageByLabels(labels map[string]string) (*packageEnvelope, error) {
L:
	for _, p := range s.packages {
		for name, value := range labels {
			if existing, ok := p.labels[name]; !ok || existing != value {
				continue L
			}
		}
		return &p, nil
	}
	return nil, trace.NotFound("package matching labels %v not found", labels)
}

// Packages returns all packages in the set
func (s *PackageSet) Packages() []packageEnvelope {
	return s.packages
}

// Apps returns all application packages in the set
func (s *PackageSet) Apps() []packageEnvelope {
	return s.apps
}

// AddApp adds an application specified with p with given labels to this set
func (s *PackageSet) AddApp(p loc.Locator, labels map[string]string) {
	addPackage(p, labels, &s.apps, true)
}

func addPackage(newPackage loc.Locator, labels map[string]string, source *[]packageEnvelope, archive bool) {
	(*source) = append((*source), packageEnvelope{
		loc:     newPackage,
		labels:  labels,
		archive: archive,
	})
}

// configureDockerOptions creates a set of Docker-specific command line arguments to Planet on the specified node
// based on the operation op and docker manifest configuration block.
func configureDockerOptions(
	op *ops.SiteOperation,
	node *ProvisionedServer,
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

type teleportSecrets struct {
	HostCAPublicKey  []byte `json:"host_ca_public_key"`
	HostCAPrivateKey []byte `json:"host_ca_private_key"`
	UserCAPublicKey  []byte `json:"user_ca_public_key"`
	UserCAPrivateKey []byte `json:"user_ca_private_key"`
}

type teleportKeyPair struct {
	Cert       []byte `json:"cert"`
	PrivateKey []byte `json:"private_key"`
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
