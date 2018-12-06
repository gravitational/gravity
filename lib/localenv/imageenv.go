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

package localenv

import (
	"os"
	"path/filepath"

	"github.com/gravitational/gravity/lib/archive"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/schema"

	"github.com/gravitational/trace"
)

// ImageEnvironment is a special case of a local environment with the state
// directory rooted in the unpacked application/cluster image tarball.
type ImageEnvironment struct {
	// LocalEnvironment is a wrapped local environment.
	*LocalEnvironment
	// Manifest is an application/cluster manifest.
	Manifest *schema.Manifest
	// cleanup is the environment cleanup function.
	cleanup func()
}

// NewImageEnvironment returns a new environment for a specified image.
//
// The path can be either an image tarball or an unpacked image tarball.
func NewImageEnvironment(path string) (*ImageEnvironment, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// see if it's a path to unpacked image tarball or a tarball
	if fi.IsDir() {
		// see if app.yaml is there
		_, err := os.Stat(filepath.Join(path, defaults.ManifestFileName))
		if err != nil {
			return nil, trace.BadParameter("directory %q does not appear "+
				"to contain an application image", path)
		}
		return newImageEnvironment(path)
	}
	// see if tarball has app.yaml
	err = archive.HasFile(path, defaults.ManifestFileName)
	if err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		return nil, trace.BadParameter("file %q does not appear to be "+
			"a valid application image", path)
	}
	// extract tarball to a temporary directory
	unpackedPath, err := archive.Unpack(path)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return newImageEnvironment(unpackedPath, WithCleanup())
}

func newImageEnvironment(unpackedDir string, opts ...ImageEnvironmentOption) (*ImageEnvironment, error) {
	manifest, err := schema.ParseManifest(filepath.Join(unpackedDir,
		defaults.ManifestFileName))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	localEnv, err := New(unpackedDir)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	imageEnv := &ImageEnvironment{
		LocalEnvironment: localEnv,
		Manifest:         manifest,
	}
	for _, opt := range opts {
		opt(imageEnv)
	}
	return imageEnv, nil
}

// Close closes the image environment.
func (e *ImageEnvironment) Close() error {
	if e.cleanup != nil {
		e.cleanup()
	}
	return e.LocalEnvironment.Close()
}

// ImageEnvironmentOption defines an image environment functional argument.
type ImageEnvironmentOption func(*ImageEnvironment)

// WithCleanup cleans up an image environment state directory on close.
func WithCleanup() ImageEnvironmentOption {
	return func(env *ImageEnvironment) {
		env.cleanup = func() {
			log.Debugf("Cleaning up %v.", env.LocalEnvironment.StateDir)
			os.RemoveAll(env.LocalEnvironment.StateDir)
		}
	}
}
