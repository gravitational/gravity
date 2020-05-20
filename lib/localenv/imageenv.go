/*
Copyright 2018-2020 Gravitational, Inc.

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
	"archive/tar"
	"io"
	"io/ioutil"
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
	// Path is the environment source location, directory or tarball path.
	Path string
	// Manifest is an application/cluster manifest.
	Manifest *schema.Manifest
	// cleanup is the environment cleanup function.
	cleanup func()
}

// ImageEnvironmentConfig is the cluster/application image environment configuration.
type ImageEnvironmentConfig struct {
	// Path is the path to the cluster/application image (unpacked or tarball).
	Path string
	// DBOnly creates environment without unpacked packages (only database).
	DBOnly bool
}

// NewImageEnvironment returns a new environment for a specified application
// or cluster image.
//
// The path can be either an image tarball or an unpacked image tarball.
func NewImageEnvironment(config ImageEnvironmentConfig) (*ImageEnvironment, error) {
	fi, err := os.Stat(config.Path)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var imageEnvironment *ImageEnvironment
	if fi.IsDir() {
		imageEnvironment, err = imageEnvFromDir(config.Path)
	} else {
		if config.DBOnly {
			imageEnvironment, err = imageEnvFromTarDBOnly(config.Path)
		} else {
			imageEnvironment, err = imageEnvFromTar(config.Path)
		}
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return imageEnvironment, nil
}

// imageEnvFromDir creates image environment from an unpacked image.
func imageEnvFromDir(path string) (*ImageEnvironment, error) {
	log.WithField("path", path).Debug("Creating image environment from path.")
	manifest, err := schema.ParseManifest(filepath.Join(path, defaults.ManifestFileName))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	env, err := New(path)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &ImageEnvironment{
		LocalEnvironment: env,
		Manifest:         manifest,
		Path:             path,
	}, nil
}

// imageEnvFromTar creates image environment from a tarball.
func imageEnvFromTar(path string) (*ImageEnvironment, error) {
	log.WithField("path", path).Debug("Creating image environment tar.")
	dir, err := archive.Unpack(path)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	manifestBytes, err := ioutil.ReadFile(filepath.Join(dir, defaults.ManifestFileName))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	manifest, err := schema.ParseManifestYAMLNoValidate(manifestBytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	env, err := New(dir)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &ImageEnvironment{
		LocalEnvironment: env,
		Manifest:         manifest,
		Path:             path,
		cleanup: func() {
			os.RemoveAll(dir)
		},
	}, nil
}

// imageEnvFromTarDBOnly create image environment without package blobs from a tarball.
func imageEnvFromTarDBOnly(path string) (*ImageEnvironment, error) {
	log.WithField("path", path).Debug("Creating image environment without packages from tar.")
	tarball, err := os.Open(path)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer tarball.Close()
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = archive.TarGlob(tar.NewReader(tarball), "",
		[]string{defaults.ManifestFileName, defaults.GravityDBFile},
		func(filename string, r io.Reader) error {
			data, err := ioutil.ReadAll(r)
			if err != nil {
				return trace.Wrap(err)
			}
			err = ioutil.WriteFile(filepath.Join(dir, filename), data, defaults.SharedReadMask)
			if err != nil {
				return trace.Wrap(err)
			}
			return nil
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	manifestBytes, err := ioutil.ReadFile(filepath.Join(dir, defaults.ManifestFileName))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	manifest, err := schema.ParseManifestYAMLNoValidate(manifestBytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	env, err := New(dir)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &ImageEnvironment{
		LocalEnvironment: env,
		Manifest:         manifest,
		Path:             path,
		cleanup: func() {
			os.RemoveAll(dir)
		},
	}, nil
}

// Close closes the image environment.
func (e *ImageEnvironment) Close() error {
	if e.cleanup != nil {
		e.cleanup()
	}
	if e.LocalEnvironment != nil {
		return e.LocalEnvironment.Close()
	}
	return nil
}
