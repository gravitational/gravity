package install

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	ephases "github.com/gravitational/gravity/e/lib/install/phases"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/install"
	"github.com/gravitational/gravity/lib/install/phases"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/ops/opsservice"
	"github.com/gravitational/gravity/lib/ops/suite"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/pack/encryptedpack"
	"github.com/gravitational/gravity/lib/process"
	"github.com/gravitational/gravity/lib/processconfig"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/systeminfo"

	"github.com/cloudflare/cfssl/csr"
	"github.com/gravitational/license"
	"github.com/gravitational/license/authority"
	"github.com/gravitational/trace"
	"github.com/pborman/uuid"
	"github.com/sirupsen/logrus"
	"gopkg.in/check.v1"
)

func TestInstaller(t *testing.T) { check.TestingT(t) }

type PlanSuite struct {
	services           opsservice.TestServices
	config             Config
	masterNode         storage.Server
	regularNode        storage.Server
	adminAgent         *storage.LoginEntry
	regularAgent       *storage.LoginEntry
	teleportPackage    *loc.Locator
	gravityPackage     *loc.Locator
	runtimePackage     loc.Locator
	rbacPackage        *loc.Locator
	runtimeApplication *loc.Locator
	dnsPackage         *loc.Locator
	loggingPackage     *loc.Locator
	monitoringPackage  *loc.Locator
	sitePackage        *loc.Locator
	serviceUser        systeminfo.User
	dnsConfig          storage.DNSConfig
	cluster            ops.Site
	operationKey       ops.SiteOperationKey
	planner            *Planner
}

var _ = check.Suite(&PlanSuite{})

func (s *PlanSuite) SetUpSuite(c *check.C) {
	s.services = opsservice.SetupTestServices(c)
	// use encrypted package service to test decryption phase as well
	s.services.Packages = encryptedpack.New(s.services.Packages, encryptionKey)
	account, err := s.services.Operator.CreateAccount(
		ops.NewAccountRequest{
			ID:  defaults.SystemAccountID,
			Org: defaults.SystemAccountOrg,
		})
	c.Assert(err, check.IsNil)
	appPackage := suite.SetUpTestPackage(c, s.services.Apps,
		s.services.Packages)
	app, err := s.services.Apps.GetApp(appPackage)
	c.Assert(err, check.IsNil)
	s.teleportPackage, err = app.Manifest.Dependencies.ByName(constants.TeleportPackage)
	c.Assert(err, check.IsNil)
	s.gravityPackage, err = app.Manifest.Dependencies.ByName(constants.GravityPackage)
	c.Assert(err, check.IsNil)
	runtimePackage, err := app.Manifest.DefaultRuntimePackage()
	c.Assert(err, check.IsNil)
	s.runtimePackage = *runtimePackage
	s.rbacPackage, err = app.Manifest.Dependencies.ByName(constants.BootstrapConfigPackage)
	c.Assert(err, check.IsNil)
	s.runtimeApplication = app.Manifest.Base()
	c.Assert(s.runtimeApplication, check.NotNil)
	s.dnsPackage, err = app.Manifest.Dependencies.ByName("dns-app")
	c.Assert(err, check.IsNil)
	s.loggingPackage, err = app.Manifest.Dependencies.ByName("logging-app")
	c.Assert(err, check.IsNil)
	s.monitoringPackage, err = app.Manifest.Dependencies.ByName("monitoring-app")
	c.Assert(err, check.IsNil)
	s.sitePackage, err = app.Manifest.Dependencies.ByName("site")
	c.Assert(err, check.IsNil)
	ca, err := authority.GenerateSelfSignedCA(csr.CertificateRequest{
		CN: "localhost",
	})
	c.Assert(err, check.IsNil)
	err = pack.CreateCertificateAuthority(pack.CreateCAParams{
		Packages: s.services.Packages,
		KeyPair:  *ca,
	})
	c.Assert(err, check.IsNil)
	license, err := license.NewLicense(license.NewLicenseInfo{
		MaxNodes:      2,
		ValidFor:      time.Hour,
		EncryptionKey: []byte(encryptionKey),
		TLSKeyPair:    *ca,
	})
	c.Assert(err, check.IsNil)
	s.dnsConfig = storage.DNSConfig{
		Addrs: []string{"127.0.0.3"},
		Port:  10053,
	}
	cluster, err := s.services.Operator.CreateSite(
		ops.NewSiteRequest{
			AccountID:  account.ID,
			DomainName: "example.com",
			AppPackage: appPackage.String(),
			Provider:   schema.ProviderAWS,
			License:    license,
			DNSConfig:  s.dnsConfig,
		})
	c.Assert(err, check.IsNil)
	s.cluster = *cluster
	_, err = s.services.Users.CreateClusterAdminAgent(s.cluster.Domain,
		storage.NewUser(storage.ClusterAdminAgent(s.cluster.Domain), storage.UserSpecV2{
			AccountID: defaults.SystemAccountID,
		}))
	c.Assert(err, check.IsNil)
	s.adminAgent, err = s.services.Operator.GetClusterAgent(ops.ClusterAgentRequest{
		AccountID:   account.ID,
		ClusterName: s.cluster.Domain,
		Admin:       true,
	})
	c.Assert(err, check.IsNil)
	s.regularAgent, err = s.services.Operator.GetClusterAgent(ops.ClusterAgentRequest{
		AccountID:   account.ID,
		ClusterName: s.cluster.Domain,
	})
	c.Assert(err, check.IsNil)
	operationKey, err := s.services.Operator.CreateSiteInstallOperation(context.TODO(),
		ops.CreateSiteInstallOperationRequest{
			AccountID:   account.ID,
			SiteDomain:  s.cluster.Domain,
			Provisioner: schema.ProvisionerAWSTerraform,
		})
	c.Assert(err, check.IsNil)
	s.operationKey = *operationKey
	s.masterNode = storage.Server{
		AdvertiseIP: "10.10.0.1",
		Hostname:    "node-1",
		Role:        "node",
		ClusterRole: string(schema.ServiceRoleMaster),
	}
	s.regularNode = storage.Server{
		AdvertiseIP: "10.10.0.2",
		Hostname:    "node-2",
		Role:        "knode",
		ClusterRole: string(schema.ServiceRoleNode),
	}
	err = s.services.Operator.UpdateInstallOperationState(
		*operationKey, ops.OperationUpdateRequest{
			Profiles: map[string]storage.ServerProfileRequest{
				"node": {Count: 2},
			},
			Servers: []storage.Server{
				s.masterNode,
				s.regularNode,
			},
		})
	c.Assert(err, check.IsNil)
	s.serviceUser = systeminfo.User{
		Name: "user",
		UID:  999,
		GID:  999,
	}
	baseConfig := install.Config{
		FieldLogger: logrus.WithField(trace.Component, "plan-suite"),
		ServiceUser: s.serviceUser,
		DNSConfig:   s.dnsConfig,
		Process:     &mockProcess{},
		App:         *app,
		Packages:    s.services.Packages,
		Apps:        s.services.Apps,
		Operator:    s.services.Operator,
	}
	s.config = Config{
		RemoteOpsURL:   "https://ops.example.com",
		RemoteOpsToken: "ops-token",
		OpsTunnelToken: "tun-token",
		Config:         baseConfig,
	}
	s.planner = &Planner{
		PlanBuilderGetter: &s.config.Config,
		PreflightChecks:   true,
		OpsCenterCluster:  "opscenter.example.com",
		Packages:          s.services.Packages,
		OpsTunnelToken:    s.config.OpsTunnelToken,
		OpsSNIHost:        s.config.OpsSNIHost,
		RemoteOpsURL:      s.config.RemoteOpsURL,
		RemoteOpsToken:    s.config.RemoteOpsToken,
	}
}

func (s *PlanSuite) TestPlan(c *check.C) {
	op, err := s.services.Operator.GetSiteOperation(s.operationKey)
	c.Assert(err, check.IsNil)

	plan, err := s.planner.GetOperationPlan(s.services.Operator, s.cluster, *op)
	c.Assert(err, check.IsNil)

	expected := []struct {
		// phaseID is the ID of the phase to verify
		phaseID string
		// phaseVerifier is the function used to verify the phase
		phaseVerifier func(*check.C, storage.OperationPhase)
	}{
		{phases.InitPhase, s.verifyInitPhase},
		{phases.ChecksPhase, s.verifyChecksPhase},
		{ephases.DecryptPhase, s.verifyDecryptPhase},
		{phases.ConfigurePhase, s.verifyConfigurePhase},
		{phases.BootstrapPhase, s.verifyBootstrapPhase},
		{phases.PullPhase, s.verifyPullPhase},
		{phases.MastersPhase, s.verifyMastersPhase},
		{phases.NodesPhase, s.verifyNodesPhase},
		{phases.WaitPhase, s.verifyWaitPhase},
		{phases.RBACPhase, s.verifyRBACPhase},
		{phases.CorednsPhase, s.verifyCorednsPhase},
		{phases.SystemResourcesPhase, s.verifySystemResourcesPhase},
		{ephases.LicensePhase, s.verifyLicensePhase},
		{phases.ExportPhase, s.verifyExportPhase},
		{phases.InstallOverlayPhase, s.verifyInstallOverlayPhase},
		{phases.HealthPhase, s.verifyHealthPhase},
		{phases.RuntimePhase, s.verifyRuntimePhase},
		{phases.AppPhase, s.verifyAppPhase},
		{phases.ConnectInstallerPhase, s.verifyConnectInstallerPhase},
		{ephases.ConnectPhase, s.verifyConnectPhase},
		{phases.EnableElectionPhase, s.verifyEnableElectionPhase},
	}

	for i, phase := range plan.Phases {
		c.Assert(phase.ID, check.Equals, expected[i].phaseID, check.Commentf(
			"expected phase number %v to be %v but got %v", i, expected[i].phaseID, phase.ID))
		expected[i].phaseVerifier(c, phase)
	}
}

func (s *PlanSuite) verifyInitPhase(c *check.C, phase storage.OperationPhase) {
	storage.DeepComparePhases(c, storage.OperationPhase{
		ID: phases.InitPhase,
		Phases: []storage.OperationPhase{
			{
				ID: fmt.Sprintf("%v/%v", phases.InitPhase, s.masterNode.Hostname),
				Data: &storage.OperationPhaseData{
					Server:     &s.masterNode,
					ExecServer: &s.masterNode,
					Package:    &s.config.App.Package,
				},
			},
			{
				ID: fmt.Sprintf("%v/%v", phases.InitPhase, s.regularNode.Hostname),
				Data: &storage.OperationPhaseData{
					Server:     &s.regularNode,
					ExecServer: &s.regularNode,
					Package:    &s.config.App.Package,
				},
			},
		},
		Parallel: true,
	}, phase)
}

func (s *PlanSuite) verifyChecksPhase(c *check.C, phase storage.OperationPhase) {
	storage.DeepComparePhases(c, storage.OperationPhase{
		ID: phases.ChecksPhase,
		Data: &storage.OperationPhaseData{
			Package: &s.config.App.Package,
		},
		Requires: []string{phases.InitPhase},
	}, phase)
}

func (s *PlanSuite) verifyDecryptPhase(c *check.C, phase storage.OperationPhase) {
	storage.DeepComparePhases(c, storage.OperationPhase{
		ID: ephases.DecryptPhase,
		Data: &storage.OperationPhaseData{
			Package: &s.config.App.Package,
			Data:    encryptionKey,
		},
		Requires: []string{phases.ChecksPhase},
	}, phase)
}

func (s *PlanSuite) verifyConfigurePhase(c *check.C, phase storage.OperationPhase) {
	storage.DeepComparePhases(c, storage.OperationPhase{
		ID:       phases.ConfigurePhase,
		Requires: []string{ephases.DecryptPhase},
		Data: &storage.OperationPhaseData{
			Install: &storage.InstallOperationData{},
		},
	}, phase)
}

func (s *PlanSuite) verifyBootstrapPhase(c *check.C, phase storage.OperationPhase) {
	serviceUser := s.user()
	storage.DeepComparePhases(c, storage.OperationPhase{
		ID: phases.BootstrapPhase,
		Phases: []storage.OperationPhase{
			{
				ID: fmt.Sprintf("%v/%v", phases.BootstrapPhase, s.masterNode.Hostname),
				Data: &storage.OperationPhaseData{
					Server:      &s.masterNode,
					ExecServer:  &s.masterNode,
					Package:     &s.config.App.Package,
					Agent:       s.adminAgent,
					ServiceUser: serviceUser,
				},
			},
			{
				ID: fmt.Sprintf("%v/%v", phases.BootstrapPhase, s.regularNode.Hostname),
				Data: &storage.OperationPhaseData{
					Server:      &s.regularNode,
					ExecServer:  &s.regularNode,
					Package:     &s.config.App.Package,
					Agent:       s.regularAgent,
					ServiceUser: serviceUser,
				},
			},
		},
		Parallel: true,
	}, phase)
}

func (s *PlanSuite) verifyPullPhase(c *check.C, phase storage.OperationPhase) {
	serviceUser := s.user()
	storage.DeepComparePhases(c, storage.OperationPhase{
		ID: phases.PullPhase,
		Phases: []storage.OperationPhase{
			{
				ID: fmt.Sprintf("%v/%v", phases.PullPhase, s.masterNode.Hostname),
				Data: &storage.OperationPhaseData{
					Server:      &s.masterNode,
					ExecServer:  &s.masterNode,
					Package:     &s.config.App.Package,
					ServiceUser: serviceUser,
					Pull: &storage.PullData{
						Apps: []loc.Locator{
							s.config.App.Package,
						},
					},
				},
				Requires: []string{phases.ConfigurePhase, phases.BootstrapPhase},
			},
			{
				ID: fmt.Sprintf("%v/%v", phases.PullPhase, s.regularNode.Hostname),
				Data: &storage.OperationPhaseData{
					Server:      &s.regularNode,
					ExecServer:  &s.regularNode,
					Package:     &s.config.App.Package,
					ServiceUser: serviceUser,
					Pull: &storage.PullData{
						Packages: []loc.Locator{
							*s.gravityPackage,
							*s.teleportPackage,
							s.runtimePackage,
						},
					},
				},
				Requires: []string{phases.ConfigurePhase, phases.BootstrapPhase},
			},
		},
		Requires: []string{phases.ConfigurePhase, phases.BootstrapPhase},
		Parallel: true,
	}, phase)
}

func (s *PlanSuite) verifyMastersPhase(c *check.C, phase storage.OperationPhase) {
	serviceUser := s.user()
	storage.DeepComparePhases(c, storage.OperationPhase{
		ID: phases.MastersPhase,
		Phases: []storage.OperationPhase{
			{
				ID: fmt.Sprintf("%v/%v", phases.MastersPhase, s.masterNode.Hostname),
				Phases: []storage.OperationPhase{
					{
						ID: fmt.Sprintf("%v/%v/teleport", phases.MastersPhase, s.masterNode.Hostname),
						Data: &storage.OperationPhaseData{
							Server:      &s.masterNode,
							ExecServer:  &s.masterNode,
							Package:     s.teleportPackage,
							ServiceUser: serviceUser,
						},
						Requires: []string{fmt.Sprintf("%v/%v", phases.PullPhase, s.masterNode.Hostname)},
					},
					{
						ID: fmt.Sprintf("%v/%v/planet", phases.MastersPhase, s.masterNode.Hostname),
						Data: &storage.OperationPhaseData{
							Server:      &s.masterNode,
							ExecServer:  &s.masterNode,
							Package:     &s.runtimePackage,
							ServiceUser: serviceUser,
							Labels:      pack.RuntimePackageLabels,
						},
						Requires: []string{fmt.Sprintf("%v/%v", phases.PullPhase, s.masterNode.Hostname)},
						Step:     4,
					},
				},
				Requires: []string{fmt.Sprintf("%v/%v", phases.PullPhase, s.masterNode.Hostname)},
			},
		},
		Requires: []string{phases.PullPhase},
		Parallel: true,
	}, phase)
}

func (s *PlanSuite) verifyNodesPhase(c *check.C, phase storage.OperationPhase) {
	serviceUser := s.user()
	storage.DeepComparePhases(c, storage.OperationPhase{
		ID: phases.NodesPhase,
		Phases: []storage.OperationPhase{
			{
				ID: fmt.Sprintf("%v/%v", phases.NodesPhase, s.regularNode.Hostname),
				Phases: []storage.OperationPhase{
					{
						ID: fmt.Sprintf("%v/%v/teleport", phases.NodesPhase, s.regularNode.Hostname),
						Data: &storage.OperationPhaseData{
							Server:      &s.regularNode,
							ExecServer:  &s.regularNode,
							ServiceUser: serviceUser,
							Package:     s.teleportPackage,
						},
						Requires: []string{fmt.Sprintf("%v/%v", phases.PullPhase, s.regularNode.Hostname)},
					},
					{
						ID: fmt.Sprintf("%v/%v/planet", phases.NodesPhase, s.regularNode.Hostname),
						Data: &storage.OperationPhaseData{
							Server:      &s.regularNode,
							ExecServer:  &s.regularNode,
							Package:     &s.runtimePackage,
							ServiceUser: serviceUser,
							Labels:      pack.RuntimePackageLabels,
						},
						Requires: []string{fmt.Sprintf("%v/%v", phases.PullPhase, s.regularNode.Hostname)},
						Step:     4,
					},
				},
				Requires: []string{fmt.Sprintf("%v/%v", phases.PullPhase, s.regularNode.Hostname)},
			},
		},
		Requires: []string{phases.PullPhase},
		Parallel: true,
	}, phase)
}

func (s *PlanSuite) verifyWaitPhase(c *check.C, phase storage.OperationPhase) {
	storage.DeepComparePhases(c, storage.OperationPhase{
		ID: phases.WaitPhase,
		Data: &storage.OperationPhaseData{
			Server: &s.masterNode,
		},
		Requires: []string{phases.MastersPhase, phases.NodesPhase},
	}, phase)
}

func (s *PlanSuite) verifyHealthPhase(c *check.C, phase storage.OperationPhase) {
	storage.DeepComparePhases(c, storage.OperationPhase{
		ID: phases.HealthPhase,
		Data: &storage.OperationPhaseData{
			Server: &s.masterNode,
		},
		Requires: []string{phases.InstallOverlayPhase, phases.ExportPhase},
	}, phase)
}

func (s *PlanSuite) verifyInstallOverlayPhase(c *check.C, phase storage.OperationPhase) {
	serviceUser := s.user()
	storage.DeepComparePhases(c, storage.OperationPhase{
		ID: phases.InstallOverlayPhase,
		Data: &storage.OperationPhaseData{
			Server:      &s.masterNode,
			ServiceUser: serviceUser,
			Package:     &s.config.App.Package,
		},
		Requires: []string{phases.ExportPhase},
	}, phase)
}

func (s *PlanSuite) verifyRBACPhase(c *check.C, phase storage.OperationPhase) {
	storage.DeepComparePhases(c, storage.OperationPhase{
		ID: phases.RBACPhase,
		Data: &storage.OperationPhaseData{
			Server:  &s.masterNode,
			Package: s.rbacPackage,
		},
		Requires: []string{phases.WaitPhase},
	}, phase)
}

func (s *PlanSuite) verifyCorednsPhase(c *check.C, phase storage.OperationPhase) {
	storage.DeepComparePhases(c, storage.OperationPhase{
		ID: phases.CorednsPhase,
		Data: &storage.OperationPhaseData{
			Server: &s.masterNode,
		},
		Requires: []string{phases.WaitPhase},
	}, phase)
}

func (s *PlanSuite) verifySystemResourcesPhase(c *check.C, phase storage.OperationPhase) {
	storage.DeepComparePhases(c, storage.OperationPhase{
		ID: phases.SystemResourcesPhase,
		Data: &storage.OperationPhaseData{
			Server: &s.masterNode,
		},
		Requires: []string{phases.RBACPhase},
	}, phase)
}

func (s *PlanSuite) verifyLicensePhase(c *check.C, phase storage.OperationPhase) {
	storage.DeepComparePhases(c, storage.OperationPhase{
		ID: ephases.LicensePhase,
		Data: &storage.OperationPhaseData{
			Server:  &s.masterNode,
			License: []byte(s.cluster.License.Raw),
		},
		Requires: []string{phases.RBACPhase},
	}, phase)
}

func (s *PlanSuite) verifyExportPhase(c *check.C, phase storage.OperationPhase) {
	storage.DeepComparePhases(c, storage.OperationPhase{
		ID: phases.ExportPhase,
		Phases: []storage.OperationPhase{
			{
				ID: fmt.Sprintf("%v/%v", phases.ExportPhase, s.masterNode.Hostname),
				Data: &storage.OperationPhaseData{
					Server:     &s.masterNode,
					ExecServer: &s.masterNode,
					Package:    &s.config.App.Package,
				},
				Requires: []string{phases.WaitPhase},
			},
		},
		Requires: []string{phases.WaitPhase},
		Parallel: true,
	}, phase)
}

func (s *PlanSuite) verifyRuntimePhase(c *check.C, phase storage.OperationPhase) {
	storage.DeepComparePhases(c, storage.OperationPhase{
		ID: phases.RuntimePhase,
		Phases: []storage.OperationPhase{
			{
				ID: fmt.Sprintf("%v/%v", phases.RuntimePhase, s.runtimeApplication.Name),
				Data: &storage.OperationPhaseData{
					Server:      &s.masterNode,
					Package:     s.runtimeApplication,
					ServiceUser: s.user(),
				},
				Requires: []string{phases.RBACPhase},
			},
		},
		Requires: []string{phases.RBACPhase},
	}, phase)
}

func (s *PlanSuite) verifyAppPhase(c *check.C, phase storage.OperationPhase) {
	serviceUser := s.user()
	appPhases := []storage.OperationPhase{
		{
			ID: fmt.Sprintf("%v/%v", phases.AppPhase, s.rbacPackage.Name),
			Data: &storage.OperationPhaseData{
				Server:  &s.masterNode,
				Package: s.rbacPackage,
			},
			Requires: []string{phases.RuntimePhase},
		},
		{
			ID: fmt.Sprintf("%v/%v", phases.AppPhase, s.dnsPackage.Name),
			Data: &storage.OperationPhaseData{
				Server:  &s.masterNode,
				Package: s.dnsPackage,
			},
			Requires: []string{phases.RuntimePhase},
		},
		{
			ID: fmt.Sprintf("%v/%v", phases.AppPhase, s.loggingPackage.Name),
			Data: &storage.OperationPhaseData{
				Server:  &s.masterNode,
				Package: s.loggingPackage,
			},
			Requires: []string{phases.RuntimePhase},
		},
		{
			ID: fmt.Sprintf("%v/%v", phases.AppPhase, s.monitoringPackage.Name),
			Data: &storage.OperationPhaseData{
				Server:  &s.masterNode,
				Package: s.monitoringPackage,
			},
			Requires: []string{phases.RuntimePhase},
		},
		{
			ID: fmt.Sprintf("%v/%v", phases.AppPhase, s.sitePackage.Name),
			Data: &storage.OperationPhaseData{
				Server:  &s.masterNode,
				Package: s.sitePackage,
			},
			Requires: []string{phases.RuntimePhase},
		},
		{
			ID: fmt.Sprintf("%v/%v", phases.AppPhase, s.config.App.Package.Name),
			Data: &storage.OperationPhaseData{
				Server:  &s.masterNode,
				Package: &s.config.App.Package,
			},
			Requires: []string{phases.RuntimePhase},
		},
	}
	for _, phase := range appPhases {
		phase.Data.ServiceUser = serviceUser
	}
	storage.DeepComparePhases(c, storage.OperationPhase{
		ID:       phases.AppPhase,
		Phases:   appPhases,
		Requires: []string{phases.RuntimePhase},
	}, phase)
}

func (s *PlanSuite) verifyConnectPhase(c *check.C, phase storage.OperationPhase) {
	cluster, err := s.planner.makeTrustedCluster()
	c.Assert(err, check.IsNil)
	bytes, err := storage.MarshalTrustedCluster(cluster)
	c.Assert(err, check.IsNil)
	storage.DeepComparePhases(c, storage.OperationPhase{
		ID: ephases.ConnectPhase,
		Data: &storage.OperationPhaseData{
			Server:         &s.masterNode,
			TrustedCluster: bytes,
		},
		Requires: []string{phases.RuntimePhase},
	}, phase)
}

func (s *PlanSuite) verifyConnectInstallerPhase(c *check.C, phase storage.OperationPhase) {
	bytes, err := storage.MarshalTrustedCluster(installerTrustedCluster)
	c.Assert(err, check.IsNil)
	storage.DeepComparePhases(c, storage.OperationPhase{
		ID: phases.ConnectInstallerPhase,
		Data: &storage.OperationPhaseData{
			Server:         &s.masterNode,
			TrustedCluster: bytes,
		},
		Requires: []string{phases.RuntimePhase},
	}, phase)
}

func (s *PlanSuite) verifyEnableElectionPhase(c *check.C, phase storage.OperationPhase) {
	storage.DeepComparePhases(c, storage.OperationPhase{
		ID: phases.EnableElectionPhase,
		Data: &storage.OperationPhaseData{
			Server: &s.masterNode,
		},
		Requires: []string{phases.AppPhase},
	}, phase)
}

func (s *PlanSuite) user() *storage.OSUser {
	return &storage.OSUser{
		Name: s.serviceUser.Name,
		UID:  strconv.Itoa(s.serviceUser.UID),
		GID:  strconv.Itoa(s.serviceUser.GID),
	}
}

// encryptionKey is used to test encrypted installer packages
const encryptionKey = "secret"

type mockProcess struct {
	process.GravityProcess
}

func (p *mockProcess) Config() *processconfig.Config {
	return &processconfig.Config{
		OpsCenter: processconfig.OpsCenterConfig{
			SeedConfig: &ops.SeedConfig{
				TrustedClusters: []storage.TrustedCluster{
					installerTrustedCluster,
				},
			},
		},
	}
}

var installerTrustedCluster = storage.NewTrustedCluster("installer",
	storage.TrustedClusterSpecV2{
		Enabled:              true,
		Token:                uuid.New(),
		ProxyAddress:         "localhost",
		ReverseTunnelAddress: "localhost",
		Wizard:               true,
	})
