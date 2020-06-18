/*
Copyright 2019 Gravitational, Inc.

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

// Package intermediate implements support for intermediate upgrade steps.
package intermediate

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/state"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// NewPackageRotatorForPath returns a new instance of the configuration package
// rotator that uses a gravity binary of the specified version for operation
func NewPackageRotatorForPath(packages pack.PackageService, path, operationID string) *gravityPackageRotator {
	return &gravityPackageRotator{
		packages:    packages,
		path:        path,
		operationID: operationID,
	}
}

// GravityPathForVersion returns the path to the gravity binary
// for a specific version of the runtime
func GravityPathForVersion(version string) (path string, err error) {
	stateDir, err := state.GetStateDir()
	if err != nil {
		return "", trace.Wrap(err)
	}
	return filepath.Join(state.GravityUpdateDir(stateDir), version, constants.GravityBin), nil
}

// ExportGravityBinary exports the gravity binary given with loc under the specified path
func ExportGravityBinary(ctx context.Context, loc loc.Locator, uid, gid int, path string, packages pack.PackageService) error {
	if err := os.MkdirAll(filepath.Dir(path), defaults.SharedDirMask); err != nil {
		return trace.Wrap(trace.ConvertSystemError(err),
			"failed to create directory for export at %v", filepath.Dir(path))
	}
	ctx, cancel := context.WithTimeout(ctx, defaults.TransientErrorTimeout)
	defer cancel()
	return utils.CopyWithRetries(ctx, path,
		func() (io.ReadCloser, error) {
			_, rc, err := packages.ReadPackage(loc)
			return rc, trace.Wrap(err)
		},
		utils.PermOption(defaults.SharedExecutableMask),
		utils.OwnerOption(uid, gid),
	)
}

// PackageRotator defines the subset of the operator to generate
// new configuration packages
type PackageRotator interface {
	// RotateSecrets generates a new secrets package for the specified request
	RotateSecrets(ops.RotateSecretsRequest) (*ops.RotatePackageResponse, error)
	// RotatePlanetConfig generates a new planet configuration package for the specified request
	RotatePlanetConfig(ops.RotatePlanetConfigRequest) (*ops.RotatePackageResponse, error)
	// RotateTeleportConfig generates new teleport configuration packages for the specified request
	RotateTeleportConfig(ops.RotateTeleportConfigRequest) (
		masterConfig, nodeConfig *ops.RotatePackageResponse,
		err error,
	)
}

// RotateSecrets generates a new secrets package for the specified request
func (r gravityPackageRotator) RotateSecrets(req ops.RotateSecretsRequest) (*ops.RotatePackageResponse, error) {
	args := []string{
		"update", "rotate-secrets",
		"--addr", req.Server.AdvertiseIP,
		"--id", r.operationID,
	}
	if req.Package != nil {
		args = append(args, "--package", req.Package.String())
	}
	return r.exec(req.DryRun, req.Package, args...)
}

// RotatePlanetConfig generates a new planet configuration package for the specified request
func (r gravityPackageRotator) RotatePlanetConfig(req ops.RotatePlanetConfigRequest) (*ops.RotatePackageResponse, error) {
	args := []string{
		"update", "rotate-planet-config",
		"--runtime-package", req.RuntimePackage.String(),
		"--addr", req.Server.AdvertiseIP,
		"--id", r.operationID,
	}
	if req.Package != nil {
		args = append(args, "--package", req.Package.String())
	}
	return r.exec(req.DryRun, req.Package, args...)
}

// RotateTeleportConfig generates new teleport configuration packages for the specified request.
// This method is a no-op in this version
func (r gravityPackageRotator) RotateTeleportConfig(req ops.RotateTeleportConfigRequest) (masterConfig, nodeConfig *ops.RotatePackageResponse, err error) {
	return nil, nil, nil
}

func (r gravityPackageRotator) exec(onlyPackageName bool, loc *loc.Locator, args ...string) (resp *ops.RotatePackageResponse, err error) {
	cmd := exec.Command(r.path, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.WithFields(log.Fields{
			log.ErrorKey: err,
			"path":       r.path,
			"args":       args,
			"output":     string(out),
		}).Warn("Failed to exec.")
		return nil, trace.Wrap(err)
	}
	out = bytes.TrimSpace(out)
	if loc == nil {
		if len(out) == 0 {
			// No package name generated - active package will be used
			return nil, nil
		}
		loc, err = parseLocatorFromOutput(out)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	resp = &ops.RotatePackageResponse{Locator: *loc}
	if onlyPackageName {
		return resp, nil
	}
	var env *pack.PackageEnvelope
	env, resp.Reader, err = r.packages.ReadPackage(*loc)
	if err != nil {
		return nil, trace.Wrap(err, "failed to read package %v", loc)
	}
	resp.Labels = env.RuntimeLabels
	return resp, nil
}

func parseLocatorFromOutput(output []byte) (*loc.Locator, error) {
	if len(output) == 0 {
		return nil, trace.BadParameter("package locator is empty")
	}
	loc, err := loc.ParseLocator(string(output))
	if err != nil {
		return nil, trace.Wrap(err, "failed to interpret %q as package locator", string(output))
	}
	return loc, nil
}

// gravityPackageRotator configures packages using a gravity binary
type gravityPackageRotator struct {
	// packages specifies the package service
	packages pack.PackageService
	// path specifies the path to the gravity binary
	path string
	// operationID specifies the ID of the active update operation
	operationID string
}

var _ PackageRotator = (*gravityPackageRotator)(nil)
