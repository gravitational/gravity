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
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// updateUsers adds users to the "planet" group
func (s *site) updateUsers(ctx *operationContext, server *ProvisionedServer) error {
	commands := []Command{
		Cmd(
			[]string{"usermod", "-a", "-G", fmt.Sprintf("%v", s.gid()), server.User.Name},
			"adding system user %q to group %v", server.User.Name, s.gid())}

	for _, command := range commands {
		out, err := s.agentRunner(ctx).RunCmd(*ctx, server, command)
		if err != nil {
			return trace.Wrap(err, string(out))
		}
	}

	return nil
}

func (s *site) setupRemoteEnvironment(ctx *operationContext, server *ProvisionedServer, runner remoteRunner) error {
	ctx.Infof("setupRemoteEnvironment for %v", server)

	s.reportProgress(ctx, ops.ProgressEntry{
		State:      ops.ProgressStateInProgress,
		Completion: 50,
		Message:    "Installing software",
	})

	docker, err := s.selectDockerConfig(ctx.operation, server.Role, s.app.Manifest)
	if err != nil {
		return trace.Wrap(err)
	}

	if docker.StorageDriver == constants.DockerStorageDriverDevicemapper {
		if err := s.configureDevicemapper(&ctx.operation, server, runner); err != nil {
			return trace.Wrap(err)
		}
	}

	commands, err := remoteDirectories(ctx.operation, server, s.app.Manifest, s.uid(), s.gid())
	if err != nil {
		return trace.Wrap(err)
	}

	uidFlag := fmt.Sprintf("--uid=%v", s.uid())
	gidFlag := fmt.Sprintf("--gid=%v", s.gid())

	// login into opscenter
	agentUser, key, err := s.agentUserAndKey()
	if err != nil {
		return trace.Wrap(err)
	}

	opsCenterURL := s.packages().PortalURL()
	opsCenterURLFlag := fmt.Sprintf("--ops-url=%v", opsCenterURL)
	commands = append(
		commands,
		Cmd(
			s.gravityCommand(uidFlag, gidFlag,
				"ops", "connect", opsCenterURL,
				agentUser.GetName(), key.Token),
			"connecting to remote Ops Center %v", opsCenterURL,
		))

	// log into locally running gravity site too
	creds, err := storage.GetClusterAgentCreds(s.backend(), s.domainName, server.IsMaster())
	if err != nil {
		return trace.Wrap(err)
	}

	commands = append(
		commands,
		Cmd(
			s.gravityCommand("ops", "connect", creds.OpsCenterURL, creds.Email, creds.Password),
			"connecting to local cluster %v", creds.OpsCenterURL,
		))

	// download and import packages
	for _, app := range server.PackageSet.Apps() {
		commands = append(commands,
			// TODO (klizhentas) remove hardcoded user id
			RetryCmd(
				s.gravityCommand(uidFlag, gidFlag,
					"--quiet", "app", "pull", "--force",
					app.loc.String(), opsCenterURLFlag),
				"pulling application package %v", app.loc,
			),
			// unpack to avoid the unpacked location being owned by root
			Cmd(
				s.gravityCommand(uidFlag, gidFlag,
					"package", "unpack", app.loc.String()),
				"unpacking application package %v", app.loc,
			),
		)
	}

	secretsPackage, err := s.planetSecretsPackage(server)
	if err != nil {
		return trace.Wrap(err)
	}

	for _, p := range server.PackageSet.Packages() {
		commands = append(commands,
			// TODO (klizhentas) remove hardcoded user id
			RetryCmd(
				s.gravityCommand(uidFlag, gidFlag, "--quiet",
					"package", "pull", "--force", p.loc.String(),
					opsCenterURLFlag, "--labels", p.labelFlag()),
				"downloading package %v", p.loc))
		if p.archive {
			commands = append(commands,
				// unpack so that unpacked directory won't be owned by root
				// (this would happen if 'command' caller is root)
				Cmd(
					s.gravityCommand(uidFlag, gidFlag, "package", "unpack", p.loc.String()),
					"unpacking package %v", p.loc),
			)
		}

		// planet secrets should be unpacked to special directory
		if p.loc.String() == secretsPackage.String() {
			commands = append(commands,
				Cmd(
					s.gravityCommand(uidFlag, gidFlag, "package", "unpack", p.loc.String(),
						server.InGravity(defaults.SecretsDir)),
					"unpacking package %v", p.loc.String(),
				),
			)
		}
	}

	planetPackage, err := server.PackageSet.GetPackageByLabels(pack.RuntimePackageLabels)
	if err != nil {
		return trace.Wrap(err)
	}

	// start teleport and planet as system services
	teleportPackage, err := server.PackageSet.GetPackage(constants.TeleportPackage)
	if err != nil {
		return trace.Wrap(err)
	}

	// execute all the commands
	for _, cmd := range commands {
		out, err := runner.RunCmd(*ctx, server, cmd)
		if err != nil {
			return trace.Wrap(err, string(out))
		}
	}

	serverRunner := &serverRunner{runner: runner, server: server}
	err = s.installServices(ctx, serverRunner, *teleportPackage, *planetPackage)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (s *site) installServices(ctx *operationContext, runner *serverRunner, packages ...packageEnvelope) error {
	serviceName := func(pkg loc.Locator) string {
		name := fmt.Sprintf("install-%v-%v-%v", pkg.Name, pkg.Version, ctx.operation.ID)
		return fmt.Sprintf("%v.service", name)
	}

	for _, pkg := range packages {
		name := serviceName(pkg.loc)
		args := []string{"system", "reinstall", pkg.loc.String(), "--service-name", name}
		if len(pkg.labels) != 0 {
			args = append(args, "--labels", pkg.labelFlag())
		}
		cmd := Cmd(s.gravityCommand(args...),
			"installing system service %v", pkg.loc)
		out, err := runner.RunCmd(*ctx, cmd)
		if err != nil {
			return trace.Wrap(err, "failed to install system service %v: %s", pkg.loc, out)
		}

		err = s.checkSystemServiceStatus(ctx, runner, name, defaults.InstallSystemServiceTimeout)
		if err != nil {
			return trace.Wrap(err)
		}

		out, err = runner.Run(s.gravityCommand("system", "service", "uninstall", "--name", name)...)
		if err != nil {
			return trace.Wrap(err, "failed to uninstall system service %v: %s", pkg.loc, out)
		}
	}
	return nil
}

// planetEnterCommand returns a new command that executes the command specified
// with args inside planet container
func (s *site) planetEnterCommand(args ...string) []string {
	exe := utils.Executable{Path: defaults.GravityBin}
	return exe.PlanetCommandSlice(args, s.contextArgs()...)
}

// planetStatusCommand returns a command that outputs planet status
func (s *site) planetStatusCommand() []string {
	return s.gravityCommand("planet", "status")
}

// gravityCommand generates a command line for a gravity sub-command specified with args.
// It adds additional flags depending on the context.
func (s *site) gravityCommand(args ...string) []string {
	command := []string{constants.GravityBin}
	command = append(command, s.contextArgs()...)
	command = append(command, args...)
	return command
}

// planetGravityCommand returns gravity command that is executed inside planet,
// the difference with "gravityCommand" is that it uses gravity binary at the default
// location in /usr/bin
func (s *site) planetGravityCommand(args ...string) []string {
	command := []string{defaults.GravityBin}
	command = append(command, s.contextArgs()...)
	command = append(command, args...)
	return utils.PlanetCommandSlice(command, s.contextArgs()...)
}

// etcdctlCommand returns etcdctl command that will run inside planet
func (s *site) etcdctlCommand(args ...string) []string {
	cmd := s.planetEnterCommand(
		"/usr/bin/etcdctl",
		"--endpoint=https://127.0.0.1:2379",
		"--cert-file=/var/state/etcd.cert",
		"--key-file=/var/state/etcd.key",
		"--ca-file=/var/state/root.cert")
	return append(cmd, args...)
}

// contextArgs returns a list of additional command line arguments for a gravity binary
// depending on the context
func (s *site) contextArgs() (args []string) {
	if s.shouldUseInsecure() {
		args = append(args, "--insecure")
	}
	if s.service.cfg.Devmode {
		args = append(args, "--debug")
	}
	return args
}

func remoteDirectories(operation ops.SiteOperation, server *ProvisionedServer, manifest schema.Manifest, uid, gid string) (commands []Command, err error) {
	// list of directories to create
	directories := []string{
		server.InGravity("local", "packages", "blobs"),
		server.InGravity("local", "packages", "unpacked"),
		server.InGravity("local", "packages", "tmp"),
		server.InGravity("teleport", "auth"),
		server.InGravity("teleport", "node"),
		server.InGravity("planet", "state"),
		server.InGravity("planet", "etcd"),
		server.InGravity("planet", "registry"),
		server.InGravity("planet", "docker"),
		server.InGravity("planet", "share", "hooks"),
		server.InGravity("planet", "log", "journal"),
		server.InGravity("site", "packages", "blobs"),
		server.InGravity("site", "packages", "unpacked"),
		server.InGravity("site", "packages", "tmp"),
		server.InGravity("site", "teleport"),
	}

	chownList := []string{
		server.InGravity("local"),
		server.InGravity("teleport"),
		server.InGravity("planet", "etcd"),
		server.InGravity("planet", "log"),
		server.InGravity("planet", "registry"),
		server.InGravity("planet", "share"),
		server.InGravity("planet", "state"),
		server.InGravity("site"),
	}

	chmodList := []string{
		server.StateDir(),
		server.InGravity("local"),
	}

	mounts, err := GetMounts(manifest, server.Server)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	for _, mount := range mounts {
		// TODO: this code still runs on expand.
		// We will be upgrading this to use the same install fsm
		// so this will be removed.
		// For now, simply skip the volumes marked with SkipIfMissing
		// since these are meant for already existing directories outside
		// of telekube system root.
		if mount.SkipIfMissing {
			log.Debugf("Skipping volume %v.", mount.Source)
			continue
		}
		if mount.CreateIfMissing {
			commands = append(commands,
				Cmd([]string{"mkdir", "-p", mount.Source}, "creating directory %v", mount.Source),
			)
		}
		if mount.UID != nil || mount.GID != nil {
			expr := chownExpr(mount.UID, mount.GID)
			commands = append(commands,
				Cmd([]string{"chown", expr, mount.Source}, "setting ownership of %v to %v", mount.Source, expr),
			)
		} else {
			// set standard ownership
			chownList = append(chownList, mount.Source)
		}
		if mount.Mode != "" {
			commands = append(commands,
				Cmd([]string{"chmod", mount.Mode, mount.Source},
					"setting file mode of %v to %v", mount.Source, formatFileMode(mount.Mode)),
			)
		}
	}

	for _, dir := range directories {
		commands = append(commands,
			Cmd([]string{"mkdir", "-p", dir}, "creating directory %v", dir),
		)
	}

	commands = append(commands,
		Cmd(
			// Change ownership of the top-level gravity directory non-recursively
			[]string{"chown", fmt.Sprintf("%v:%v", uid, gid), server.StateDir()},
			"setting ownership of %v to %v:%v", server.StateDir(), uid, gid,
		),
	)
	for _, dir := range chownList {
		commands = append(commands,
			Cmd(
				[]string{"chown", "-R", fmt.Sprintf("%v:%v", uid, gid), dir},
				"setting ownership of %v to %v:%v", dir, uid, gid,
			),
		)
	}
	for _, dir := range chmodList {
		commands = append(commands,
			Cmd(
				[]string{"chmod", fmt.Sprintf("%o", defaults.SharedDirMask), dir},
				"setting file mode of %v to %v", dir, os.FileMode(defaults.SharedDirMask)),
		)
	}

	return commands, nil
}

func checkRunning(status []byte) error {
	if strings.Contains(string(status), `"status":"degraded"`) {
		return trace.Errorf("app is not running")
	}
	return nil
}

// formatFileMode formats the specified mode as os.FileMode value.
// It does not fail in case it cannot parse the mode as octal numeral
// and returns the specified input mode unaltered.
func formatFileMode(mode string) string {
	fileMode, err := strconv.ParseUint(mode, 8, 32)
	if err == nil {
		mode = os.FileMode(fileMode).String()
	}
	return mode
}
