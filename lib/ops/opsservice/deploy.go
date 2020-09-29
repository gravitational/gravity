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

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// planetEnterCommand returns a new command that executes the command specified
// with args inside planet container
func (s *site) planetEnterCommand(args ...string) []string {
	exe := utils.Executable{Path: defaults.GravityBin}
	return exe.PlanetCommandSlice(args, s.contextArgs()...)
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
		server.InGravity("planet", "kubelet"),
		server.InGravity("planet", "share", "hooks"),
		server.InGravity("planet", "log", "journal"),
		server.InGravity("site", "teleport"),
		server.InGravity("site", "packages", "blobs"),
		server.InGravity("site", "packages", "unpacked"),
		server.InGravity("site", "packages", "tmp"),
		server.InGravity("secrets"),
		server.InGravity("backup"),
		server.InGravity("logrange"),
		// names prometheus-db/alertmanager-db are hardcoded subPath values
		// in prometheus-operator
		server.InGravity("monitoring", "prometheus-db"),
		server.InGravity("monitoring", "alertmanager-db"),
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
		server.InGravity("secrets"),
		server.InGravity("backup"),
		server.InGravity("monitoring"),
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
			commands = append(commands, Cmd(
				[]string{"chown", expr, mount.Source},
				"setting ownership of %v to %v", mount.Source, expr))
		} else {
			// set standard ownership
			commands = append(commands, Cmd(
				[]string{"chown", fmt.Sprintf("%v:%v", uid, gid), mount.Source},
				"setting ownership of %v to %v:%v", mount.Source, uid, gid))
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
