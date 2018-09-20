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
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// rotateSecrets generates a new set of TLS keys for the given node
// as a package that will be automatically downloaded during upgrade
func (s *site) rotateSecrets(ctx *operationContext, node *ProvisionedServer, installOp ops.SiteOperation) (*ops.RotatePackageResponse, error) {
	secretsPackage, err := s.planetSecretsNextPackage(node)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	subnets := installOp.InstallExpand.Subnets
	if subnets.IsEmpty() {
		// Subnets are empty when updating an older installation
		subnets = storage.DefaultSubnets
	}

	if !node.IsMaster() {
		resp, err := s.getPlanetNodeSecretsPackage(ctx, node, secretsPackage)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return resp, nil
	}

	masterParams := planetMasterParams{
		master:            node,
		secretsPackage:    secretsPackage,
		serviceSubnetCIDR: subnets.Service,
	}
	// if we have a connection to Ops Center set up, configure
	// SNI host so Ops Center can dial in
	trustedCluster, err := storage.GetTrustedCluster(s.backend())
	if err == nil {
		masterParams.sniHost = trustedCluster.GetSNIHost()
	}

	resp, err := s.getPlanetMasterSecretsPackage(ctx, masterParams)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return resp, nil
}

func (s *site) configurePlanetOnNode(
	ctx *operationContext,
	runner commandRunner,
	node *ProvisionedServer,
	etcd etcdConfig,
	manifest schema.Manifest) (*ops.RotatePackageResponse, error) {
	cmds, err := remoteDirectories(ctx.operation, node, ctx.update.app.Manifest, s.uid(), s.gid())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	for _, cmd := range cmds {
		out, err := runner.RunCmd(*ctx, cmd)
		if err != nil {
			return nil, trace.Wrap(err, "command %v failed: %s", cmd, out)
		}
	}

	docker, err := s.selectDockerConfig(ctx.operation, s.app.Manifest, &manifest)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var dockerRuntime storage.Docker
	if docker.StorageDriver == constants.DockerStorageDriverDevicemapper {
		server := s.backendSite.ClusterState.Servers.FindByIP(node.AdvertiseIP)
		if server != nil {
			dockerRuntime = server.Docker
			if dockerRuntime.LVMSystemDirectory == "" {
				systemDir, err := lvmGetSystemDir(runner)
				if err != nil {
					return nil, trace.Wrap(err)
				}
				dockerRuntime.LVMSystemDirectory = systemDir
			}
		}
	}

	planetPackage, err := ctx.update.app.Manifest.RuntimePackage(node.Profile)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	configPackage, err := s.planetConfigPackage(node, planetPackage.Version)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	log.Debugf("Created new planet configuration package %v for %v.", configPackage, node)

	config := planetConfig{
		master:        masterConfig{addr: ctx.update.masterIP},
		etcd:          etcd,
		docker:        *docker,
		dockerRuntime: dockerRuntime,
		planetPackage: *planetPackage,
		configPackage: *configPackage,
	}
	config.master.electionEnabled = node.IsMaster()

	resp, err := s.getPlanetConfigPackage(node, ctx.update.installOp, config, manifest)
	if err != nil && !trace.IsAlreadyExists(err) {
		return nil, trace.Wrap(err)
	}

	return resp, nil
}

func lvmGetSystemDir(runner commandRunner) (string, error) {
	out, err := runner.Run(constants.GravityBin, "system", "devicemapper", "system-dir")
	if err != nil {
		log.Errorf("failed to determine LVM system directory: %s (%v)", out, trace.DebugReport(err))
		return "", trace.Wrap(err)
	}
	return string(out), nil
}
