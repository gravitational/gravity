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
	"context"
	"fmt"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// GetMounts returns a list of server mounts
func GetMounts(m schema.Manifest, server storage.Server) ([]storage.Mount, error) {
	profile, err := m.NodeProfiles.ByName(server.Role)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// get mounts specified in the manifest for this server profile
	var mounts []storage.Mount
	for _, m := range profile.Mounts() {
		mounts = append(mounts, newMount(m))
	}

	// server may override a mount source directory
	// as a result of the user changing the source via web UI
	for i := range mounts {
		for _, m := range server.Mounts {
			if mounts[i].Name == m.Name {
				mounts[i].Source = m.Source
			}
		}
	}

	return mounts, nil
}

// runPackageHook invokes the specified hook for the application identified by the provided locator.
func (s *site) runPackageHook(ctx *operationContext, locator loc.Locator, hook schema.HookType) error {
	var out []byte
	var err error
	if s.service.cfg.Local {
		_, out, err = app.RunAppHook(context.TODO(), s.appService, app.HookRunRequest{
			Application: locator,
			Hook:        hook,
			ServiceUser: s.serviceUser(),
		})
	} else {
		command := s.planetGravityCommand("app", "hook", locator.String(), hook.String())
		out, err = s.runOnMaster(ctx, command)
	}
	if err != nil {
		return trace.Wrap(err, "failed to run %v hook: %s", hook, out)
	}
	log.Infof("hook %v output: %s", hook, out)
	return nil
}

// runHook invokes the specified hook for the application currently installed on the site.
func (s *site) runHook(ctx *operationContext, hook schema.HookType) error {
	return s.runPackageHook(ctx, s.app.Package, hook)
}

// runOnMaster executes the provided command on the master server.
func (s *site) runOnMaster(ctx *operationContext, command []string) ([]byte, error) {
	runner, err := s.getMasterRunner(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return runner.Run(command...)
}

// getMasterRunner returns the runner that allows to run commands remotely on master server.
func (s *site) getMasterRunner(ctx *operationContext) (*serverRunner, error) {
	if ctx.operation.Type == ops.OperationInstall {
		// during install operation it is more reliable to use the agent runner b/c
		// teleport may not be fully initialized yet
		master := ctx.provisionedServers.FirstMaster()
		return &serverRunner{
			master, &agentRunner{ctx, s.agentService()},
		}, nil
	}
	// for all other operations use teleport to run remote commands
	master, err := s.getTeleportServer(schema.ServiceLabelRole, string(schema.ServiceRoleMaster))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &serverRunner{
		master, &teleportRunner{ctx, s.domainName, s.teleport()},
	}, nil
}

// numExistingServers returns a number of servers in this installation.
//
// For the install operation this number is, of course, 0.
func (s *site) numExistingServers(op *ops.SiteOperation) (int, error) {
	if op.Type == ops.OperationInstall {
		return 0, nil
	}
	return s.teleport().GetServerCount(context.TODO(), s.domainName)
}

// newMount constructs storage mount from schema Volume
func newMount(m schema.Volume) storage.Mount {
	mount := storage.Mount{
		Name:            m.Name,
		Source:          m.Path,
		Destination:     m.TargetPath,
		CreateIfMissing: utils.BoolValue(m.CreateIfMissing),
		SkipIfMissing:   utils.BoolValue(m.SkipIfMissing),
		Mode:            m.Mode,
		Recursive:       m.Recursive,
	}
	if m.UID != nil {
		mount.UID = utils.IntPtr(*m.UID)
	}
	if m.GID != nil {
		mount.GID = utils.IntPtr(*m.GID)
	}
	return mount
}

// chownExpr generates chown expression in the form used by chown command
// based on uid and gid parameters
func chownExpr(uid, gid *int) string {
	// When both uid and gid are specified, the syntax is "chown <uid>:<gid> <dir>"
	if uid != nil && gid != nil {
		return fmt.Sprintf("%v:%v", *uid, *gid)
	}
	// When only uid is specified, the syntax is "chown <uid> <dir>"
	if uid != nil {
		return fmt.Sprintf("%v", *uid)
	}
	// When only gid is specified, the syntax is "chown :<gid> <dir>"
	return fmt.Sprintf(":%v", *gid)
}
