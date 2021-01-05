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
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	teledefaults "github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/trace"
)

// GetOperationPlan builds a plan for the provided operation
func (i *Installer) GetOperationPlan(cluster ops.Site, op ops.SiteOperation) (*storage.OperationPlan, error) {
	ossBuilder, err := i.GetPlanBuilder(cluster, op)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	builder := &PlanBuilder{ossBuilder}

	plan := &storage.OperationPlan{
		OperationID:   op.ID,
		OperationType: op.Type,
		AccountID:     op.AccountID,
		ClusterName:   op.SiteDomain,
		Servers:       append(builder.Masters, builder.Nodes...),
		DNSConfig:     cluster.DNSConfig,
	}

	switch i.Mode {
	case constants.InstallModeCLI:
		builder.AddChecksPhase(plan)
	}

	// (optional) download the installer when installing via Ops Center
	if i.downloadInstaller {
		builder.AddInstallerPhase(plan, i.opsCenterCluster,
			i.Config.RemoteOpsURL, i.Config.RemoteOpsToken)
	}

	// (optional) decrypt packages
	encrypted, key, err := i.checkEncryptedPackages()
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
	builder.AddPullPhase(plan)

	// install system software on master nodes
	builder.AddMastersPhase(plan)

	// (optional) install system software on regular nodes
	if len(builder.Nodes) > 0 {
		builder.AddNodesPhase(plan)
	}

	// perform post system install tasks such as waiting for planet
	// to start up, creating RBAC resources, etc.
	builder.AddWaitPhase(plan)
	builder.AddRBACPhase(plan)
	builder.AddCorednsPhase(plan)

	// if installing an Ops Center, the resources contain a config map
	// with its additional configuration (such as advertise addresses)
	// and service
	builder.AddResourcesPhase(plan)

	// (optional) install cluster license
	if i.Cluster.License != nil {
		builder.AddLicensePhase(plan, i.Cluster.License.Raw)
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
	trustedCluster, err := i.getTrustedCluster()
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
func (i *Installer) getTrustedCluster() (storage.TrustedCluster, error) {
	// see if trusted cluster information was provided on the command line
	if i.Config.RemoteOpsURL != "" && i.Config.OpsTunnelToken != "" {
		return i.makeTrustedCluster()
	}
	// see if there's trusted cluster among the installer tarball packages
	trustedCluster, err := i.findTrustedCluster()
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	if trace.IsNotFound(err) {
		return nil, trace.NotFound("no remote Ops Center information found " +
			"in the installer tarball and none was set via --ops-url and " +
			"--ops-tunnel-token flags")
	}
	return trustedCluster, nil
}

// findTrustedCluster returns the trusted cluster from the installer's tarball
//
// Trusted cluster is present in the tarball for installers that are downloaded
// from Ops Centers.
func (i *Installer) findTrustedCluster() (storage.TrustedCluster, error) {
	_, reader, err := i.Packages.ReadPackage(loc.TrustedCluster)
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
	i.Debugf("Found %s in installer tarball.", trustedCluster)
	return trustedCluster, nil
}

// makeTrustedCluster creates a trusted cluster resource from information
// about remote Ops Center of this installer
func (i *Installer) makeTrustedCluster() (storage.TrustedCluster, error) {
	opsHost, opsPort, err := utils.URLSplitHostPort(i.Config.RemoteOpsURL, defaults.HTTPSPort)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return storage.NewTrustedCluster(i.opsCenterCluster,
		storage.TrustedClusterSpecV2{
			Enabled:      true,
			Token:        i.Config.OpsTunnelToken,
			ProxyAddress: fmt.Sprintf("%v:%v", opsHost, opsPort),
			ReverseTunnelAddress: fmt.Sprintf("%v:%v", opsHost,
				teledefaults.SSHProxyTunnelListenPort),
			SNIHost: i.Config.OpsSNIHost,
			Roles:   []string{constants.RoleAdmin},
		}), nil
}

// checkEncryptedPackages returns true if the installer has encrypted packages
func (i *Installer) checkEncryptedPackages() (encrypted bool, key string, err error) {
	envelopes, err := i.Packages.GetPackages(defaults.SystemAccountOrg)
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
	if i.Cluster.License != nil {
		license = i.Cluster.License.Raw
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
