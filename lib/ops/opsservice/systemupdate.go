package opsservice

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/systemservice"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

func (s *site) checkSystemServiceStatus(ctx *operationContext, runner *serverRunner, serviceName string, timeout time.Duration) error {
	err := utils.RetryFor(context.Background(), timeout, func() error {
		out, err := runner.Run(s.gravityCommand("system", "service", "status", "--name", serviceName)...)
		serviceStatus := strings.TrimSpace(string(out))
		ctx.Debugf("service %v status: %v", serviceName, serviceStatus)
		switch serviceStatus {
		case systemservice.ServiceStatusFailed:
			return utils.Abort(
				trace.Errorf("%v reported failed status", serviceName))
		case systemservice.ServiceStatusActive:
			return nil
		case systemservice.ServiceStatusActivating:
			return utils.Continue(fmt.Sprintf("%v is still in progress", serviceName))
		case systemservice.ServiceStatusUnknown, systemservice.ServiceStatusInactive:
			return utils.Abort(trace.NotFound("%v is not found or inactive", serviceName))
		default:
			return trace.Wrap(err, "unsupported status or error")
		}
	})
	return trace.Wrap(err)
}

// rotateSecrets generates a new set of TLS keys for the given node
// as a package that will be automatically downloaded during upgrade
func (s *site) rotateSecrets(node *ProvisionedServer, installOp ops.SiteOperation) (*ops.RotatePackageResponse, error) {
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
		resp, err := s.getPlanetNodeSecretsPackage(node, secretsPackage)
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

	resp, err := s.getPlanetMasterSecretsPackage(masterParams)
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

	docker, err := s.selectDockerConfig(ctx.operation, node.Role, manifest)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var dockerRuntime storage.Docker
	if docker.StorageDriver == constants.DockerStorageDriverDevicemapper {
		server := ctx.update.installOp.InstallExpand.Servers.FindByIP(node.AdvertiseIP)
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

	profile, err := ctx.update.app.Manifest.NodeProfiles.ByName(node.Role)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	config := planetConfig{
		master:        masterConfig{addr: ctx.update.masterIP},
		etcd:          etcd,
		docker:        ctx.update.app.Manifest.Docker(*profile),
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
