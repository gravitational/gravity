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
	"encoding/json"
	"fmt"
	"sort"
	"strconv"

	"github.com/gravitational/gravity/lib/compare"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/install/phases"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/ops/opsservice"
	"github.com/gravitational/gravity/lib/ops/resources"
	"github.com/gravitational/gravity/lib/ops/suite"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/pack/encryptedpack"
	"github.com/gravitational/gravity/lib/process"
	"github.com/gravitational/gravity/lib/processconfig"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/systeminfo"

	"github.com/cloudflare/cfssl/csr"
	"github.com/gravitational/license/authority"
	teleservices "github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
	"github.com/pborman/uuid"
	"github.com/sirupsen/logrus"
	"gopkg.in/check.v1"
)

type PlanSuite struct {
	services           opsservice.TestServices
	installer          *Installer
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
	operationKey       *ops.SiteOperationKey
	dnsConfig          storage.DNSConfig
	cluster            *ops.Site
}

func (s *PlanSuite) Services() opsservice.TestServices {
	return s.services
}

func (s *PlanSuite) Master() storage.Server {
	return s.masterNode
}

func (s *PlanSuite) Cluster() *ops.Site {
	return s.cluster
}

func (s *PlanSuite) Operation(c *check.C) ops.SiteOperation {
	operation, err := s.services.Operator.GetSiteOperation(*s.operationKey)
	c.Assert(err, check.IsNil)
	return *operation
}

func (s *PlanSuite) PlanBuilderGetter() PlanBuilderGetter {
	return &s.installer.config.Config
}

func (s *PlanSuite) Package() *loc.Locator {
	return &s.installer.config.App.Package
}

func (s *PlanSuite) TeleportPackage() *loc.Locator {
	return s.teleportPackage
}

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
	s.dnsConfig = storage.DNSConfig{
		Addrs: []string{"127.0.0.3"},
		Port:  10053,
	}
	s.serviceUser = systeminfo.User{
		Name: "user",
		UID:  999,
		GID:  999,
	}
	serviceUser := s.user()
	s.cluster, err = s.services.Operator.CreateSite(
		ops.NewSiteRequest{
			AccountID:   account.ID,
			DomainName:  "example.com",
			AppPackage:  appPackage.String(),
			Provider:    schema.ProviderAWS,
			DNSConfig:   s.dnsConfig,
			ServiceUser: *serviceUser,
		})
	c.Assert(err, check.IsNil)
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
	s.operationKey, err = s.services.Operator.CreateSiteInstallOperation(context.TODO(),
		ops.CreateSiteInstallOperationRequest{
			AccountID:   account.ID,
			SiteDomain:  s.cluster.Domain,
			Provisioner: schema.ProvisionerAWSTerraform,
		})
	c.Assert(err, check.IsNil)
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
		*s.operationKey, ops.OperationUpdateRequest{
			Profiles: map[string]storage.ServerProfileRequest{
				"node": {Count: 2},
			},
			Servers: []storage.Server{
				s.masterNode,
				s.regularNode,
			},
		})
	c.Assert(err, check.IsNil)
	runtimeResources, clusterResources, err := resources.Split(bytes.NewReader(resourceBytes))
	c.Assert(err, check.IsNil)
	s.installer = &Installer{
		config: RuntimeConfig{
			Config: Config{
				FieldLogger:      logrus.WithField(trace.Component, "plan-suite"),
				RuntimeResources: runtimeResources,
				ClusterResources: clusterResources,
				ServiceUser:      s.serviceUser,
				DNSConfig:        s.dnsConfig,
				Process:          &mockProcess{},
				App:              *app,
				Apps:             s.services.Apps,
				Operator:         s.services.Operator,
				Packages:         s.services.Packages,
			},
		},
	}
}

func (s *PlanSuite) TestPlan(c *check.C) {
	op, err := s.services.Operator.GetSiteOperation(*s.operationKey)
	c.Assert(err, check.IsNil)

	planner := NewPlanner(true, &s.installer.config.Config)
	plan, err := planner.GetOperationPlan(s.services.Operator, *s.cluster, *op)
	c.Assert(err, check.IsNil)

	expected := []struct {
		// phaseID is the ID of the phase to verify
		phaseID string
		// phaseVerifier is the function used to verify the phase
		phaseVerifier func(*check.C, storage.OperationPhase)
	}{
		{phases.InitPhase, s.VerifyInitPhase},
		{phases.ChecksPhase, s.VerifyChecksPhase},
		{phases.ConfigurePhase, s.VerifyConfigurePhase},
		{phases.BootstrapPhase, s.VerifyBootstrapPhase},
		{phases.PullPhase, s.VerifyPullPhase},
		{phases.MastersPhase, s.VerifyMastersPhase},
		{phases.NodesPhase, s.VerifyNodesPhase},
		{phases.WaitPhase, s.VerifyWaitPhase},
		{phases.RBACPhase, s.VerifyRBACPhase},
		{phases.CorednsPhase, s.VerifyCorednsPhase},
		{phases.SystemResourcesPhase, s.VerifySystemResourcesPhase},
		{phases.UserResourcesPhase, s.VerifyUserResourcesPhase},
		{phases.ExportPhase, s.VerifyExportPhase},
		{phases.InstallOverlayPhase, s.VerifyInstallOverlayPhase},
		{phases.HealthPhase, s.VerifyHealthPhase},
		{phases.RuntimePhase, s.VerifyRuntimePhase},
		{phases.AppPhase, s.VerifyAppPhase},
		{phases.ConnectInstallerPhase, s.VerifyConnectInstallerPhase},
		{phases.EnableElectionPhase, s.VerifyEnableElectionPhase},
		{phases.GravityResourcesPhase, s.VerifyGravityResourcesPhase},
	}

	c.Assert(len(expected), check.Equals, len(plan.Phases))

	for i, phase := range plan.Phases {
		c.Assert(phase.ID, check.Equals, expected[i].phaseID, check.Commentf(
			"expected phase number %v to be %v but got %v", i, expected[i].phaseID, phase.ID))
		expected[i].phaseVerifier(c, phase)
	}
}

func (s *PlanSuite) VerifyInitPhase(c *check.C, phase storage.OperationPhase) {
	storage.DeepComparePhases(c, storage.OperationPhase{
		ID: phases.InitPhase,
		Phases: []storage.OperationPhase{
			{
				ID: fmt.Sprintf("%v/%v", phases.InitPhase, s.masterNode.Hostname),
				Data: &storage.OperationPhaseData{
					Server:     &s.masterNode,
					ExecServer: &s.masterNode,
					Package:    &s.installer.config.App.Package,
				},
			},
			{
				ID: fmt.Sprintf("%v/%v", phases.InitPhase, s.regularNode.Hostname),
				Data: &storage.OperationPhaseData{
					Server:     &s.regularNode,
					ExecServer: &s.regularNode,
					Package:    &s.installer.config.App.Package,
				},
			},
		},
		LimitParallel: NumParallel,
	}, phase)
}

func (s *PlanSuite) VerifyChecksPhase(c *check.C, phase storage.OperationPhase) {
	storage.DeepComparePhases(c, storage.OperationPhase{
		ID: phases.ChecksPhase,
		Data: &storage.OperationPhaseData{
			Package: &s.installer.config.App.Package,
		},
		Requires: []string{phases.InitPhase},
	}, phase)
}

func (s *PlanSuite) VerifyConfigurePhase(c *check.C, phase storage.OperationPhase) {
	storage.DeepComparePhases(c, storage.OperationPhase{
		ID: phases.ConfigurePhase,
		Data: &storage.OperationPhaseData{
			Install: &storage.InstallOperationData{
				Env: map[string]string{
					"HTTP_PROXY": "http://example.com:8081",
				},
			},
		},
	}, phase)
}

func (s *PlanSuite) VerifyBootstrapPhase(c *check.C, phase storage.OperationPhase) {
	serviceUser := s.user()
	storage.DeepComparePhases(c, storage.OperationPhase{
		ID: phases.BootstrapPhase,
		Phases: []storage.OperationPhase{
			{
				ID: fmt.Sprintf("%v/%v", phases.BootstrapPhase, s.masterNode.Hostname),
				Data: &storage.OperationPhaseData{
					Server:      &s.masterNode,
					ExecServer:  &s.masterNode,
					Package:     &s.installer.config.App.Package,
					Agent:       s.adminAgent,
					ServiceUser: serviceUser,
				},
			},
			{
				ID: fmt.Sprintf("%v/%v", phases.BootstrapPhase, s.regularNode.Hostname),
				Data: &storage.OperationPhaseData{
					Server:      &s.regularNode,
					ExecServer:  &s.regularNode,
					Package:     &s.installer.config.App.Package,
					Agent:       s.regularAgent,
					ServiceUser: serviceUser,
				},
			},
		},
		LimitParallel: NumParallel,
	}, phase)
}

func (s *PlanSuite) VerifyPullPhase(c *check.C, phase storage.OperationPhase) {
	serviceUser := s.user()
	storage.DeepComparePhases(c, storage.OperationPhase{
		ID: phases.PullPhase,
		Phases: []storage.OperationPhase{
			{
				ID: fmt.Sprintf("%v/%v", phases.PullPhase, s.masterNode.Hostname),
				Data: &storage.OperationPhaseData{
					Server:      &s.masterNode,
					ExecServer:  &s.masterNode,
					Package:     &s.installer.config.App.Package,
					ServiceUser: serviceUser,
					Pull: &storage.PullData{
						Apps: []loc.Locator{
							s.installer.config.App.Package,
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
					Package:     &s.installer.config.App.Package,
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
		Requires:      []string{phases.ConfigurePhase, phases.BootstrapPhase},
		LimitParallel: NumParallel,
	}, phase)
}

func (s *PlanSuite) VerifyMastersPhase(c *check.C, phase storage.OperationPhase) {
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
		Requires:      []string{phases.PullPhase},
		LimitParallel: NumParallel,
	}, phase)
}

func (s *PlanSuite) VerifyNodesPhase(c *check.C, phase storage.OperationPhase) {
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
							Package:     s.teleportPackage,
							ServiceUser: serviceUser,
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
		Requires:      []string{phases.PullPhase},
		LimitParallel: NumParallel,
	}, phase)
}

func (s *PlanSuite) VerifyWaitPhase(c *check.C, phase storage.OperationPhase) {
	storage.DeepComparePhases(c, storage.OperationPhase{
		ID: phases.WaitPhase,
		Data: &storage.OperationPhaseData{
			Server: &s.masterNode,
		},
		Requires: []string{phases.MastersPhase, phases.NodesPhase},
	}, phase)
}

func (s *PlanSuite) VerifyHealthPhase(c *check.C, phase storage.OperationPhase) {
	storage.DeepComparePhases(c, storage.OperationPhase{
		ID: phases.HealthPhase,
		Data: &storage.OperationPhaseData{
			Server: &s.masterNode,
		},
		Requires: []string{phases.InstallOverlayPhase, phases.ExportPhase},
	}, phase)
}

func (s *PlanSuite) VerifyInstallOverlayPhase(c *check.C, phase storage.OperationPhase) {
	serviceUser := s.user()
	storage.DeepComparePhases(c, storage.OperationPhase{
		ID: phases.InstallOverlayPhase,
		Data: &storage.OperationPhaseData{
			Server:      &s.masterNode,
			ServiceUser: serviceUser,
			Package:     &s.installer.config.App.Package,
		},
		Requires: []string{phases.ExportPhase},
	}, phase)
}

func (s *PlanSuite) VerifyRBACPhase(c *check.C, phase storage.OperationPhase) {
	storage.DeepComparePhases(c, storage.OperationPhase{
		ID: phases.RBACPhase,
		Data: &storage.OperationPhaseData{
			Server:  &s.masterNode,
			Package: s.rbacPackage,
		},
		Requires: []string{phases.WaitPhase},
	}, phase)
}

func (s *PlanSuite) VerifyCorednsPhase(c *check.C, phase storage.OperationPhase) {
	storage.DeepComparePhases(c, storage.OperationPhase{
		ID: phases.CorednsPhase,
		Data: &storage.OperationPhaseData{
			Server: &s.masterNode,
		},
		Requires: []string{phases.WaitPhase},
	}, phase)
}

func (s *PlanSuite) VerifySystemResourcesPhase(c *check.C, phase storage.OperationPhase) {
	storage.DeepComparePhases(c, storage.OperationPhase{
		ID: phases.SystemResourcesPhase,
		Data: &storage.OperationPhaseData{
			Server: &s.masterNode,
		},
		Requires: []string{phases.RBACPhase},
	}, phase)
}

func (s *PlanSuite) VerifyUserResourcesPhase(c *check.C, phase storage.OperationPhase) {
	obtained := phase.Data.Install.Resources
	expected := []byte(`
{
  "apiVersion": "v1",
  "kind": "ConfigMap",
  "metadata": {
    "name": "test-config"
  },
  "data": {
    "test-key": "test-value"
  }
}
{
  "kind":"ConfigMap",
  "apiVersion":"v1",
  "metadata": {
    "name": "runtimeenvironment",
    "namespace": "kube-system",
    "creationTimestamp": null
  },
  "data": {
    "HTTP_PROXY": "http://example.com:8081"
  }
}
	`)
	phase.Data.Install.Resources = nil // Compare resources separately
	storage.DeepComparePhases(c, storage.OperationPhase{
		ID: phases.UserResourcesPhase,
		Data: &storage.OperationPhaseData{
			Server:  &s.masterNode,
			Install: &storage.InstallOperationData{},
		},
		Requires: []string{phases.RBACPhase},
	}, phase)
	validateResources(c, obtained, expected)
}

func (s *PlanSuite) VerifyGravityResourcesPhase(c *check.C, phase storage.OperationPhase) {
	obtained := phase.Data.Install.GravityResources
	expected := []byte(`
{
  "kind": "AlertTarget",
  "metadata": {
    "name": "foo"
  },
  "spec": {
    "email": "info@example.com"
  },
  "version": "v1"
}`)
	phase.Data.Install.GravityResources = nil // Compare resources separately
	storage.DeepComparePhases(c, storage.OperationPhase{
		ID: phases.GravityResourcesPhase,
		Data: &storage.OperationPhaseData{
			Server:  &s.masterNode,
			Install: &storage.InstallOperationData{},
		},
		Requires: []string{phases.EnableElectionPhase},
	}, phase)
	validateGravityResources(c, obtained, expected)
}

func (s *PlanSuite) VerifyExportPhase(c *check.C, phase storage.OperationPhase) {
	storage.DeepComparePhases(c, storage.OperationPhase{
		ID: phases.ExportPhase,
		Phases: []storage.OperationPhase{
			{
				ID: fmt.Sprintf("%v/%v", phases.ExportPhase, s.masterNode.Hostname),
				Data: &storage.OperationPhaseData{
					Server:     &s.masterNode,
					ExecServer: &s.masterNode,
					Package:    &s.installer.config.App.Package,
				},
				Requires: []string{phases.WaitPhase},
			},
		},
		Requires:      []string{phases.WaitPhase},
		LimitParallel: NumParallel,
	}, phase)
}

func (s *PlanSuite) VerifyRuntimePhase(c *check.C, phase storage.OperationPhase) {
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

func (s *PlanSuite) VerifyAppPhase(c *check.C, phase storage.OperationPhase) {
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
			ID: fmt.Sprintf("%v/%v", phases.AppPhase, s.installer.config.App.Package.Name),
			Data: &storage.OperationPhaseData{
				Server:  &s.masterNode,
				Package: &s.installer.config.App.Package,
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

func (s *PlanSuite) VerifyConnectInstallerPhase(c *check.C, phase storage.OperationPhase) {
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

func (s *PlanSuite) VerifyEnableElectionPhase(c *check.C, phase storage.OperationPhase) {
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

func (s *PlanSuite) TestSplitServers(c *check.C) {
	application, err := s.services.Apps.GetApp(s.installer.config.App.Package)
	c.Assert(err, check.IsNil)

	testCases := []struct {
		input   []storage.Server
		masters []storage.Server
		nodes   []storage.Server
	}{
		{
			input: []storage.Server{
				{Hostname: "node-1", Role: "node"},
			},
			masters: []storage.Server{
				{Hostname: "node-1", Role: "node", ClusterRole: "master"},
			},
		},
		{
			input: []storage.Server{
				{Hostname: "node-1", Role: "node"},
				{Hostname: "node-2", Role: "node"},
				{Hostname: "node-3", Role: "node"},
				{Hostname: "node-4", Role: "node"},
			},
			masters: []storage.Server{
				{Hostname: "node-1", Role: "node", ClusterRole: "master"},
				{Hostname: "node-2", Role: "node", ClusterRole: "master"},
				{Hostname: "node-3", Role: "node", ClusterRole: "master"},
			},
			nodes: []storage.Server{
				{Hostname: "node-4", Role: "node", ClusterRole: "node"},
			},
		},
		{
			input: []storage.Server{
				{Hostname: "node-1", Role: "node"},
				{Hostname: "node-2", Role: "knode"},
				{Hostname: "node-3", Role: "node"},
			},
			masters: []storage.Server{
				{Hostname: "node-1", Role: "node", ClusterRole: "master"},
				{Hostname: "node-3", Role: "node", ClusterRole: "master"},
			},
			nodes: []storage.Server{
				{Hostname: "node-2", Role: "knode", ClusterRole: "node"},
			},
		},
	}

	for _, tc := range testCases {
		masters, nodes, err := splitServers(tc.input, *application)
		c.Assert(err, check.IsNil)
		c.Assert(masters, check.DeepEquals, tc.masters)
		c.Assert(nodes, check.DeepEquals, tc.nodes)
	}
}

func validateResources(c *check.C, obtainedBytes, expectedBytes []byte) {
	obtained := decodeBytes(c, obtainedBytes)
	expected := decodeBytes(c, expectedBytes)
	c.Assert(obtained, compare.DeepEquals, expected, check.Commentf("invalid resources"))
}

func validateGravityResources(c *check.C, obtained []storage.UnknownResource, expectedBytes []byte) {
	expected := decodeBytes(c, expectedBytes)
	c.Assert(decode(c, obtained), compare.DeepEquals, expected, check.Commentf("invalid Gravity resources"))
}

func decodeBytes(c *check.C, data []byte) (result []resource) {
	var rs []storage.UnknownResource
	err := resources.ForEach(bytes.NewReader(data), func(r storage.UnknownResource) error {
		rs = append(rs, r)
		return nil
	})
	c.Assert(err, check.IsNil)
	return decode(c, rs)
}

func decode(c *check.C, resources []storage.UnknownResource) (result []resource) {
	for _, res := range resources {
		var resource resource
		err := json.Unmarshal(res.Raw, &resource)
		c.Assert(err, check.IsNil)
		result = append(result, resource)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Metadata.Name < result[j].Metadata.Name
	})
	return result
}

func (r *resource) UnmarshalJSON(data []byte) error {
	if err := json.Unmarshal(data, &r.ResourceHeader); err != nil {
		return nil
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}
	r.Raw = raw
	return nil
}

type resource struct {
	teleservices.ResourceHeader
	Raw map[string]interface{}
}

// resourceBytes is used as a test resource
var resourceBytes = []byte(`apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
data:
  test-key: test-value
---
version: v1
kind: RuntimeEnvironment
spec:
  data:
    HTTP_PROXY: "http://example.com:8081"
---
version: v1
kind: AlertTarget
metadata:
  name: foo
spec:
  email: "info@example.com"
`)

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
