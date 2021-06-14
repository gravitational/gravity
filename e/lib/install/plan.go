// Copyright 2021 Gravitational Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package install

import (
	"fmt"
	"io/ioutil"

	"github.com/gravitational/gravity/e/lib/install/phases"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	ossinstall "github.com/gravitational/gravity/lib/install"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	teledefaults "github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// GetOperationPlan builds a plan for the provided operation
func (r *Planner) GetOperationPlan(operator ops.Operator, cluster ops.Site, operation ops.SiteOperation) (*storage.OperationPlan, error) {
	ossBuilder, err := r.GetPlanBuilder(operator, cluster, operation)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	builder := &PlanBuilder{ossBuilder}

	plan := &storage.OperationPlan{
		OperationID:   operation.ID,
		OperationType: operation.Type,
		AccountID:     operation.AccountID,
		ClusterName:   operation.SiteDomain,
		Servers:       append(builder.Masters, builder.Nodes...),
		DNSConfig:     cluster.DNSConfig,
	}

	builder.AddBootstrapSELinuxPhase(plan)

	builder.AddInitPhase(plan)

	if r.PreflightChecks {
		builder.AddChecksPhase(plan)
	}

	// (optional) decrypt packages
	encrypted, key, err := r.checkEncryptedPackages(cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if encrypted {
		builder.AddDecryptPhase(plan, key)
	}

	// configure packages for all nodes
	builder.AddConfigurePhase(plan)

	// bootstrap each node: setup directories, users, etc.
	builder.AddBootstrapPhase(plan)

	// pull configured packages on each node
	if err := builder.AddPullPhase(plan); err != nil {
		return nil, trace.Wrap(err)
	}

	// install system software on master nodes
	if err := builder.AddMastersPhase(plan); err != nil {
		return nil, trace.Wrap(err)
	}

	// (optional) install system software on regular nodes
	if len(builder.Nodes) > 0 {
		if err := builder.AddNodesPhase(plan); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	// perform post system install tasks such as waiting for planet
	// to start up, creating RBAC resources, etc.
	builder.AddWaitPhase(plan)
	builder.AddRBACPhase(plan)
	builder.AddCorednsPhase(plan)

	// create OpenEBS configuration if it's enabled, it has to be done
	// before OpenEBS is installed during the runtime phase
	if cluster.App.Manifest.OpenEBSEnabled() {
		if err := builder.AddOpenEBSPhase(plan); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	// create system and user-supplied Kubernetes resources
	builder.AddSystemResourcesPhase(plan)
	builder.AddUserResourcesPhase(plan)

	// install cluster license
	if cluster.License != nil {
		builder.AddLicensePhase(plan, cluster.License.Raw)
	}

	// export applications to registries
	builder.AddExportPhase(plan)

	if cluster.App.Manifest.HasHook(schema.HookNetworkInstall) {
		builder.AddInstallOverlayPhase(plan, &cluster.App.Package)
	}
	builder.AddHealthPhase(plan)

	// install runtime application
	err = builder.AddRuntimePhase(plan)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// install user application
	err = builder.AddApplicationPhase(plan)
	if err != nil {
		return nil, trace.Wrap(err)

	}

	// establish trust b/w installed cluster and installer process
	err = builder.AddConnectInstallerPhase(plan)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// (optional) connect to a remote Ops Center
	trustedCluster, err := r.getTrustedCluster()
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	if trustedCluster != nil {
		err := builder.AddConnectPhase(plan, trustedCluster)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	builder.AddEnableElectionPhase(plan)

	// Add a phase to create optional Gravity resources upon successful installation
	builder.AddGravityResourcesPhase(plan)

	return plan, nil
}

// getTrustedCluster returns the trusted cluster spec that the installed
// cluster should be connected to after installation
//
// It may be either already included in the installer tarball if it was
// downloaded from an Ops Center, or the information about it may have
// been provided on the command line.
func (r *Planner) getTrustedCluster() (storage.TrustedCluster, error) {
	// see if trusted cluster information was provided on the command line
	if r.RemoteOpsURL != "" && r.OpsTunnelToken != "" {
		return r.makeTrustedCluster()
	}
	// see if there's trusted cluster among the installer tarball packages
	trustedCluster, err := r.findTrustedCluster()
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	if trace.IsNotFound(err) {
		return nil, trace.NotFound("no remote Gravity Hub information found " +
			"in the installer tarball and none was set via --ops-url and " +
			"--ops-tunnel-token flags")
	}
	return trustedCluster, nil
}

// findTrustedCluster returns the trusted cluster from the installer's tarball
//
// Trusted cluster is present in the tarball for installers that are downloaded
// from Ops Centers.
func (r *Planner) findTrustedCluster() (storage.TrustedCluster, error) {
	_, reader, err := r.Packages.ReadPackage(loc.TrustedCluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer reader.Close()
	data, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	trustedCluster, err := storage.UnmarshalTrustedCluster(data)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	r.WithField("cluster", trustedCluster).Debug("Found trusted cluster in installer tarball.")
	return trustedCluster, nil
}

// makeTrustedCluster creates a trusted cluster resource from information
// about remote Ops Center of this installer
func (r *Planner) makeTrustedCluster() (storage.TrustedCluster, error) {
	opsHost, opsPort, err := utils.URLSplitHostPort(r.RemoteOpsURL, defaults.HTTPSPort)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return storage.NewTrustedCluster(r.OpsCenterCluster,
		storage.TrustedClusterSpecV2{
			Enabled:      true,
			Token:        r.OpsTunnelToken,
			ProxyAddress: fmt.Sprintf("%v:%v", opsHost, opsPort),
			ReverseTunnelAddress: fmt.Sprintf("%v:%v", opsHost,
				teledefaults.SSHProxyTunnelListenPort),
			SNIHost: r.OpsSNIHost,
			Roles:   []string{constants.RoleAdmin},
		},
	), nil
}

// checkEncryptedPackages returns true if the installer has encrypted packages
func (r *Planner) checkEncryptedPackages(cluster ops.Site) (encrypted bool, key string, err error) {
	envelopes, err := r.Packages.GetPackages(defaults.SystemAccountOrg)
	if err != nil {
		return false, "", trace.Wrap(err)
	}
	for _, envelope := range envelopes {
		if envelope.Encrypted {
			encrypted = true
			break
		}
	}
	if !encrypted {
		return false, "", nil
	}
	var license string
	if cluster.License != nil {
		license = cluster.License.Raw
	}
	if license == "" {
		return false, "", trace.BadParameter("installer contains encrypted " +
			"packages but license wasn't provided, supply a valid license " +
			"using --license flag")
	}
	key, err = phases.GetEncryptionKey(license)
	if err != nil && !trace.IsNotFound(err) {
		return false, "", trace.Wrap(err)
	}
	if key == "" {
		return false, "", trace.BadParameter("installer contains encrypted " +
			"packages but provided license does not include encryption key")
	}
	return true, key, nil
}

// Planner builds an install operation plan
type Planner struct {
	// FieldLogger specifies the logger
	log.FieldLogger
	// PlanBuilderGetter creates a plan builder
	ossinstall.PlanBuilderGetter
	// Packages specifies the package service
	Packages pack.PackageService
	// PreflightChecks specifies whether to include preflight checks step
	PreflightChecks bool
	// OpsCenterCluster specifies the name of the remote Ops Center
	OpsCenterCluster string
	// OpsTunnelToken is the token used when creating a trusted cluster
	OpsTunnelToken string
	// OpsSNIHost is the Ops Center SNI host
	OpsSNIHost string
	// RemoteOpsURL is the URL of the remote Ops Center
	RemoteOpsURL string
	// RemoteOpsToken is the remote Ops Center auth token
	RemoteOpsToken string
}
