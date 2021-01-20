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

package localenv

import (
	"io"
	"path"
	"path/filepath"

	libapp "github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/pack/encryptedpack"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/license"
	"github.com/gravitational/trace"
)

// NewTarballEnvironment creates new environment for the cluster image
// unpacked at the configured location
func NewTarballEnvironment(config TarballEnvironmentArgs) (*TarballEnvironment, error) {
	if err := config.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	env, err := NewLocalEnvironment(LocalEnvironmentArgs{
		StateDir:        config.StateDir,
		ReadonlyBackend: true,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer func() {
		if err != nil {
			env.Close()
		}
	}()
	var packages pack.PackageService = env.Packages
	if config.License != "" {
		parsed, err := license.ParseLicense(config.License)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		encryptionKey := parsed.GetPayload().EncryptionKey
		if len(encryptionKey) != 0 {
			packages = encryptedpack.New(packages, string(encryptionKey))
		}
	}
	apps, err := env.AppServiceLocal(AppConfig{
		Packages: packages,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	excludeApps, err := libapp.DepsToExclude(path.Join(config.StateDir, defaults.ManifestFileName))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &TarballEnvironment{
		Closer:      env,
		Packages:    packages,
		Apps:        apps,
		ExcludeApps: excludeApps,
	}, nil
}

// TarballEnvironmentArgs defines configuration for the environment
type TarballEnvironmentArgs struct {
	// StateDir specifies optional state directory.
	// If unspecified, current process's working directory is used
	StateDir string
	// License specifies optional license payload to decode packages
	License string
}

func (r *TarballEnvironmentArgs) checkAndSetDefaults() error {
	if r.StateDir == "" {
		r.StateDir = filepath.Dir(utils.Exe.Path)
	}
	return nil
}

// TarballEnvironment describes application environment in the directory
// with unpacked installer
type TarballEnvironment struct {
	io.Closer
	// Packages specifies the local package service
	Packages pack.PackageService
	// Apps specifies the local application service
	Apps libapp.Applications
	// ExcludeApps defines a list of app dependencies that will be excluded
	ExcludeApps []loc.Locator
}
