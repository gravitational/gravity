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

package service

import (
	"archive/tar"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/archive"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/schema"

	dockerarchive "github.com/docker/docker/pkg/archive"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// manifestFromUnpackedSource reads an application manifest from the specified source tarball
// by unpacking into a temporary directory.
// It returns the path to the temporary directory as a result - it is caller's responsibility
// to remove the directory after it is no longer needed using the returned cleanup handler.
// The resulting cleanup handler is guaranteed to be non-nil so it's always safe to call
func manifestFromUnpackedSource(source io.Reader) (manifest []byte, dir string, cleanup cleanup, err error) {
	dir, cleanup, err = unpackedSource(source, false)
	if err != nil {
		return nil, "", cleanup, trace.Wrap(err)
	}

	manifest, err = manifestFromDir(dir)
	if err != nil {
		return nil, "", cleanup, trace.Wrap(err)
	}

	return manifest, dir, cleanup, nil
}

// manifestFromSource reads an application manifest from the specified tarball
// without unpacking it.
func manifestFromSource(source io.Reader) (manifest []byte, err error) {
	decompressed, err := dockerarchive.DecompressStream(source)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	reader := tar.NewReader(decompressed)
	root := "."
	if err = archive.TarGlob(reader, root, []string{"*.*"}, func(match string, file io.Reader) (err error) {
		if filepath.Base(match) == defaults.ManifestFileName {
			manifest, err = ioutil.ReadAll(file)
			if err != nil {
				return trace.Wrap(err)
			}
		}
		return nil
	}); err != nil {
		return nil, trace.Wrap(err)
	}
	if manifest == nil {
		return nil, trace.NotFound("no application manifest %v found in tarball", defaults.ManifestFileName)
	}
	return manifest, nil
}

func manifestFromDir(dir string) (manifest []byte, err error) {
	path := filepath.Join(dir, defaults.ResourcesDir, defaults.ManifestFileName)
	file, err := os.Open(path)
	if err != nil {
		err = trace.ConvertSystemError(err)
		return nil, trace.Wrap(err, "failed to open application manifest %v", path)
	}
	defer file.Close()
	manifest, err = ioutil.ReadAll(file)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return manifest, nil
}

// unpackedSource decompresses the specified application source into
// a temporary directory and returns the unpacked path.
// It is caller's responsibility to remove the directory after it is no longer needed
// using the returned cleanup handler.
// The resulting cleanup handler is guaranteed to be non-nil so it's always safe to call
func unpackedSource(source io.Reader, excludeRegistry bool) (dir string, cleanup cleanup, err error) {
	dir, err = ioutil.TempDir("", "gravity")
	log.Infof("creating temp directory %q", dir)
	if err != nil {
		return "", emptyCleanup, trace.Wrap(trace.ConvertSystemError(err),
			"failed to create directory %q", dir)
	}
	cleanup = func() {
		err := os.RemoveAll(dir)
		if err != nil {
			log.Warningf("failed to remove directory %q: %v", err)
		}
	}

	tarOptions := archive.DefaultOptions()
	if excludeRegistry {
		tarOptions.ExcludePatterns = []string{"registry"}
	}

	if err = dockerarchive.Untar(source, dir, tarOptions); err != nil {
		return dir, cleanup, trace.Wrap(err)
	}
	return dir, cleanup, nil
}

// toApp translates an application representation from storage format
func toApp(pkg *pack.PackageEnvelope, apps *applications) (*app.Application, error) {
	manifest, err := apps.resolveManifest(pkg.Manifest)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &app.Application{
		Package:         pkg.Locator,
		PackageEnvelope: *pkg,
		Manifest:        *manifest,
	}, nil
}

// newApp returns an instance of Application using the specified manifest and package locator.
// The manifest is resolved (in case of inhertance) and validated.
func newApp(pkg *pack.PackageEnvelope, manifestBytes []byte, locator loc.Locator, apps *applications) (*app.Application, error) {
	manifest, err := apps.resolveManifest(manifestBytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if manifest.Metadata.Namespace == "" {
		manifest.Metadata.Namespace = app.DefaultNamespace
	}
	return &app.Application{
		Package:         locator,
		PackageEnvelope: *pkg,
		Manifest:        *manifest,
	}, nil
}

// PostProcessManifest runs post-processing tasks on a validated manifest
// Note: exported only for the testing code
// TODO: find a way to unexport this
func PostProcessManifest(manifest *schema.Manifest) {
	for i, profile := range manifest.NodeProfiles {
		labels := profile.Labels
		if labels == nil {
			labels = make(map[string]string)
		}
		labels[schema.ServiceLabelRole] = string(profile.ServiceRole)
		if _, ok := labels[schema.LabelRole]; !ok {
			labels[schema.LabelRole] = profile.Name
		}
		manifest.NodeProfiles[i].Labels = labels
	}
}

// cleanup defines a type of handler that runs a cleanup task.
// It can be returned from functions that allocate resources and
// then transfer allocation ownership to client
type cleanup func()

// emptyCleanup defines a cleanup handler that does nothing
func emptyCleanup() {}
