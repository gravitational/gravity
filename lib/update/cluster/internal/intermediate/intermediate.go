package intermediate

import (
	"bytes"
	"os/exec"
	"path/filepath"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/state"

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

// PackageRotator defines the subset of the operator to generate
// new configuration packages
type PackageRotator interface {
	// RotateSecrets generates a new secrets package for the specified request
	RotateSecrets(ops.RotateSecretsRequest) (*ops.RotatePackageResponse, error)
	// RotatePlanetConfig generates a new planet configuration package for the specified request
	RotatePlanetConfig(ops.RotatePlanetConfigRequest) (*ops.RotatePackageResponse, error)
	// RotateTeleportConfig generates new teleport configuration packages for the specified request
	RotateTeleportConfig(ops.RotateTeleportConfigRequest) (*ops.RotatePackageResponse, *ops.RotatePackageResponse, error)
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
	return r.exec(req.DryRun, args...)
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
	return r.exec(req.DryRun, args...)
}

// RotateTeleportConfig generates new teleport configuration packages for the specified request
func (r gravityPackageRotator) RotateTeleportConfig(req ops.RotateTeleportConfigRequest) (masterConfig *ops.RotatePackageResponse, nodeConfig *ops.RotatePackageResponse, err error) {
	args := []string{
		"update", "rotate-teleport-config",
		"--addr", req.Server.AdvertiseIP,
		"--id", r.operationID,
	}
	if req.NodePackage != nil {
		args = append(args, "--package", req.NodePackage.String())
	}
	resp, err := r.exec(req.DryRun, args...)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return nil, resp, nil
}

func (r gravityPackageRotator) exec(dryRun bool, args ...string) (resp *ops.RotatePackageResponse, err error) {
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
	resp = &ops.RotatePackageResponse{}
	if len(out) == 0 {
		return resp, nil
	}
	loc, err := loc.ParseLocator(string(out))
	if err != nil {
		return nil, trace.Wrap(err, "failed to interpret %q as package locator", out)
	}
	resp.Locator = *loc
	if dryRun {
		return resp, nil
	}
	_, resp.Reader, err = r.packages.ReadPackage(*loc)
	if err != nil {
		return nil, trace.Wrap(err, "failed to read package %v", loc)
	}
	return resp, nil
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
