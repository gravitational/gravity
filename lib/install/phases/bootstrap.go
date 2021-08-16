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

package phases

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/devicemapper"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/ops/opsservice"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"
	libselinux "github.com/gravitational/gravity/lib/system/selinux"
	"github.com/gravitational/gravity/lib/systeminfo"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	"github.com/opencontainers/selinux/go-selinux"
	"github.com/sirupsen/logrus"
)

// NewBootstrap returns a new "bootstrap" phase executor
func NewBootstrap(p fsm.ExecutorParams, operator ops.Operator, apps app.Applications, backend storage.LocalBackend,
	remote fsm.Remote) (fsm.PhaseExecutor, error) {
	if p.Phase.Data == nil || p.Phase.Data.ServiceUser == nil {
		return nil, trace.BadParameter("service user is required: %#v", p.Phase.Data)
	}
	if p.Phase.Data.Package == nil {
		return nil, trace.BadParameter("application package is required: %#v", p.Phase.Data)
	}

	serviceUser, err := systeminfo.UserFromOSUser(*p.Phase.Data.ServiceUser)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	operation, err := operator.GetSiteOperation(opKey(p.Plan))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if operation.Type != ops.OperationInstall {
		operation, err = ops.GetCompletedInstallOperation(operation.ClusterKey(), operator)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	application, err := apps.GetApp(*p.Phase.Data.Package)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	mounts, err := opsservice.GetMounts(application.Manifest, *p.Phase.Data.Server)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	logger := &fsm.Logger{
		FieldLogger: logrus.WithFields(logrus.Fields{
			constants.FieldPhase:       p.Phase.ID,
			constants.FieldAdvertiseIP: p.Phase.Data.Server.AdvertiseIP,
			constants.FieldHostname:    p.Phase.Data.Server.Hostname,
		}),
		Key:      opKey(p.Plan),
		Operator: operator,
		Server:   p.Phase.Data.Server,
	}
	return &bootstrapExecutor{
		FieldLogger:      logger,
		InstallOperation: *operation,
		Application:      *application,
		LocalBackend:     backend,
		ExecutorParams:   p,
		ServiceUser:      *serviceUser,
		remote:           remote,
		mounts:           mounts,
		stateDir:         p.Phase.Data.Server.StateDir(),
		seLinux:          p.Phase.Data.Server.SELinux,
	}, nil
}

type bootstrapExecutor struct {
	// FieldLogger is used for logging
	logrus.FieldLogger
	// InstallOperation is the cluster install operation
	InstallOperation ops.SiteOperation
	// Application is the application being installed
	Application app.Application
	// LocalBackend is the machine-local backend
	LocalBackend storage.LocalBackend
	// ServiceUser is the user used for services and system storage
	ServiceUser systeminfo.User
	// ExecutorParams is common executor params
	fsm.ExecutorParams
	// remote specifies the server remote control interface
	remote fsm.Remote
	// mounts lists additional application-specific volume mounts
	mounts []storage.Mount
	// stateDir specifies the local state directory
	stateDir string
	// seLinux indicates whether the node has SELinux support on
	seLinux bool
}

// Execute executes the bootstrap phase
func (p *bootstrapExecutor) Execute(ctx context.Context) error {
	dockerConfig, err := p.getDockerConfig()
	if err != nil {
		return trace.Wrap(err)
	}
	if dockerConfig.StorageDriver == constants.DockerStorageDriverDevicemapper {
		err := p.configureDeviceMapper()
		if err != nil {
			return trace.Wrap(err)
		}
	}
	err = p.configureSystemDirectories(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	err = p.configureApplicationVolumes()
	if err != nil {
		return trace.Wrap(err)
	}
	err = p.applySELinuxFileContexts(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	err = p.logIntoCluster()
	if err != nil {
		return trace.Wrap(err)
	}
	err = p.configureSystemMetadata()
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// getDockerConfig returns Docker configuration merged from the application
// manifest and operation variables
func (p *bootstrapExecutor) getDockerConfig() (*schema.Docker, error) {
	profile, err := p.Application.Manifest.NodeProfiles.ByName(p.Phase.Data.Server.Role)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	config := p.Application.Manifest.Docker(*profile)
	vars := p.InstallOperation.InstallExpand.Vars.System
	if vars.Docker.StorageDriver != "" {
		config.StorageDriver = vars.Docker.StorageDriver
	}
	config.Args = append(config.Args, vars.Docker.Args...)
	return &config, nil
}

// configureDeviceMapper configures Docker devicemapper storage
func (p *bootstrapExecutor) configureDeviceMapper() error {
	p.Progress.NextStep("Configuring device for Docker devicemapper storage driver")
	node := p.Phase.Data.Server
	if node.Docker.Device.Path() == "" {
		p.Warnf("No device has been specified for Docker: %#v.", node.Docker)
		return nil
	}
	p.Info("Configuring device for Docker devicemapper storage driver.")
	err := devicemapper.Mount(node.Docker.Device.Path(), os.Stderr, p.FieldLogger)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// configureSystemDirectories creates the necessary directories under the
// configured system directory with proper permissions
func (p *bootstrapExecutor) configureSystemDirectories(ctx context.Context) error {
	p.Progress.NextStep("Configuring system directories")
	p.Info("Configuring system directories.")
	stateDir := p.stateDir
	mkdirList := []string{
		filepath.Join(stateDir, "local", "packages", "blobs"),
		filepath.Join(stateDir, "local", "packages", "unpacked"),
		filepath.Join(stateDir, "local", "packages", "tmp"),
		filepath.Join(stateDir, "teleport", "auth"),
		filepath.Join(stateDir, "teleport", "node"),
		filepath.Join(stateDir, "planet", "state"),
		filepath.Join(stateDir, "planet", "etcd"),
		filepath.Join(stateDir, "planet", "registry"),
		filepath.Join(stateDir, "planet", "docker"),
		filepath.Join(stateDir, "planet", "kubelet"),
		filepath.Join(stateDir, "planet", "share", "hooks"),
		filepath.Join(stateDir, "planet", "log", "journal"),
		filepath.Join(stateDir, "site", "teleport", "log"),
		filepath.Join(stateDir, "site", "packages", "unpacked"),
		filepath.Join(stateDir, "site", "packages", "blobs"),
		filepath.Join(stateDir, "site", "packages", "tmp"),
		filepath.Join(stateDir, "secrets"),
		filepath.Join(stateDir, "backup"),
		filepath.Join(stateDir, "logrange"),
		// names prometheus-db/alertmanager-db are hardcoded subPath values
		// in prometheus-operator
		filepath.Join(stateDir, "monitoring", "prometheus-db"),
		filepath.Join(stateDir, "monitoring", "alertmanager-db"),
	}
	for _, dir := range mkdirList {
		p.Infof("Creating system directory %v.", dir)
		err := os.MkdirAll(dir, defaults.SharedDirMask)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	// adjust ownership of the state directory non-recursively
	p.Infof("Setting ownership on system directory %v to %v:%v.",
		stateDir, p.ServiceUser.UID, p.ServiceUser.GID)
	err := os.Chown(stateDir, p.ServiceUser.UID, p.ServiceUser.GID)
	if err != nil {
		return trace.Wrap(err)
	}
	// adjust ownerships of the internal directories, Go does not
	// have a method for recursive chown so use CLI here
	chownList := []string{
		filepath.Join(stateDir, "local"),
		filepath.Join(stateDir, "teleport"),
		filepath.Join(stateDir, "planet"),
		filepath.Join(stateDir, "site"),
		filepath.Join(stateDir, "secrets"),
		filepath.Join(stateDir, "backup"),
		filepath.Join(stateDir, "monitoring"),
		filepath.Join(stateDir, "logrange"),
	}
	for _, dir := range chownList {
		p.Infof("Setting ownership on system directory %v to %v:%v.",
			dir, p.ServiceUser.UID, p.ServiceUser.GID)
		out, err := exec.CommandContext(ctx, "chown", "-R", fmt.Sprintf("%v:%v",
			p.ServiceUser.UID, p.ServiceUser.GID), dir).CombinedOutput()
		if err != nil {
			return trace.Wrap(err, "failed to chown %v: %s", dir, out)
		}
	}
	chmodList := []string{
		stateDir,
		filepath.Join(stateDir, "local"),
	}
	for _, dir := range chmodList {
		p.Infof("Setting mode on system directory %v to %v.",
			dir, os.FileMode(defaults.SharedDirMask))
		err := os.Chmod(dir, defaults.SharedDirMask)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// configureApplicationVolumes creates necessary directories for
// application mounts
func (p *bootstrapExecutor) configureApplicationVolumes() error {
	p.Progress.NextStep("Configuring application-specific volumes")
	p.Info("Configuring application-specific volumes.")
	for _, mount := range p.mounts {
		isDir, err := utils.IsDirectory(mount.Source)
		if mount.SkipIfMissing && trace.IsNotFound(err) {
			p.Debugf("Skipping non-existent volume %v.", mount.Source)
			continue
		}
		existingDir := err == nil && isDir
		if mount.CreateIfMissing && trace.IsNotFound(err) {
			p.Infof("Creating application volume %v.", mount.Source)
			err := os.MkdirAll(mount.Source, defaults.SharedDirMask)
			if err != nil {
				return trace.Wrap(err)
			}
		}
		uid, gid := mount.UID, mount.GID
		if !existingDir {
			if uid == nil {
				uid = utils.IntPtr(p.ServiceUser.UID)
			}
			if gid == nil {
				gid = utils.IntPtr(p.ServiceUser.GID)
			}
		}
		// Only chown directories/files if necessary
		if uid != nil && gid != nil {
			p.Infof("Setting ownership on application volume %v to %v:%v.",
				mount.Source, *uid, *gid)
			err = os.Chown(mount.Source, *uid, *gid)
			if err != nil {
				return trace.Wrap(err)
			}
		}
		if mount.Mode != "" {
			mode, err := strconv.ParseUint(mount.Mode, 8, 32)
			if err != nil {
				return trace.Wrap(err)
			}
			p.Infof("Setting mode on application volume %v to %v.",
				mount.Source, mode)
			err = os.Chmod(mount.Source, os.FileMode(mode))
			if err != nil {
				return trace.Wrap(err)
			}
		}
	}
	return nil
}

// logIntoCluster creates a login entry for the local gravity site
func (p *bootstrapExecutor) logIntoCluster() error {
	_, err := p.LocalBackend.UpsertLoginEntry(*p.Phase.Data.Agent)
	if err != nil {
		return trace.Wrap(err)
	}
	p.Infof("Created agent user %s.", p.Phase.Data.Agent.Email)
	return nil
}

func (p *bootstrapExecutor) configureSystemMetadata() error {
	if err := p.LocalBackend.SetSELinux(p.seLinux); err != nil {
		return trace.Wrap(err)
	}
	if err := p.configureDNS(); err != nil {
		return trace.Wrap(err)
	}
	if err := p.configureNodeAddr(); err != nil {
		return trace.Wrap(err)
	}
	return p.configureServiceUser()
}

// configureDNS creates local cluster DNS configuration in local state database
func (p *bootstrapExecutor) configureDNS() error {
	err := p.LocalBackend.SetDNSConfig(p.Plan.DNSConfig)
	if err != nil {
		return trace.Wrap(err)
	}
	p.Infof("Created DNS configuration: %v.", p.Plan.DNSConfig)
	return nil
}

// configureNodeAddr persists the node advertise IP in local state database
func (p *bootstrapExecutor) configureNodeAddr() error {
	err := p.LocalBackend.SetNodeAddr(p.Phase.Data.Server.AdvertiseIP)
	if err != nil {
		return trace.Wrap(err)
	}
	p.Infof("Set node address: %v.", p.Phase.Data.Server.AdvertiseIP)
	return nil
}

// configureServiceUser persists the service user in local state database
func (p *bootstrapExecutor) configureServiceUser() error {
	err := p.LocalBackend.SetServiceUser(p.ServiceUser.OSUser())
	if err != nil {
		return trace.Wrap(err)
	}
	p.Infof("Set service user: %v.", p.ServiceUser)
	return nil
}

// Rollback is no-op for this phase
func (*bootstrapExecutor) Rollback(ctx context.Context) error {
	return nil
}

// PreCheck makes sure this phase is executed on a proper server
func (p *bootstrapExecutor) PreCheck(ctx context.Context) error {
	err := p.remote.CheckServer(ctx, *p.Phase.Data.Server)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// PostCheck is no-op for this phase
func (*bootstrapExecutor) PostCheck(ctx context.Context) error {
	return nil
}

func (p *bootstrapExecutor) applySELinuxFileContexts(ctx context.Context) error {
	if !(selinux.GetEnabled() && p.seLinux) {
		p.Info("SELinux is disabled.")
		return nil
	}
	paths := []string{p.stateDir}
	for _, volume := range p.mounts {
		paths = append(paths, volume.Source)
	}
	var out bytes.Buffer
	// Set file/directory labels as defined by the policy on the state directory
	if err := libselinux.ApplyFileContexts(ctx, &out, paths...); err != nil {
		return trace.Wrap(err, "failed to restore file contexts: %s", out.String())
	}
	p.WithField("output", out.String()).Info("Restore file contexts.")
	return nil
}

func opKey(plan storage.OperationPlan) ops.SiteOperationKey {
	return ops.SiteOperationKey{
		AccountID:   plan.AccountID,
		SiteDomain:  plan.ClusterName,
		OperationID: plan.OperationID,
	}
}
