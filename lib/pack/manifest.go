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

// package pack defines packaging format used by gravity
package pack

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"

	"github.com/gravitational/gravity/lib/archive"
	"github.com/gravitational/gravity/lib/systemservice"

	dockerarchive "github.com/docker/docker/pkg/archive"
	"github.com/gravitational/configure/schema"
	"github.com/gravitational/trace"
)

type Manifest struct {
	Version  string                                  `json:"version"`
	Config   *schema.Config                          `json:"config,omitempty"`
	Commands []Command                               `json:"commands,omitempty"`
	Labels   []Label                                 `json:"labels,omitempty"`
	Service  *systemservice.NewPackageServiceRequest `json:"service,omitempty"`
}

type Label struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func (m *Manifest) Label(name string) string {
	if len(m.Labels) == 0 {
		return ""
	}
	for _, l := range m.Labels {
		if l.Name == name {
			return l.Value
		}
	}
	return ""
}

func (m *Manifest) NeedsConfig() bool {
	return m.Config != nil
}

func (m *Manifest) EncodeJSON() ([]byte, error) {
	b, err := json.Marshal(m)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return b, nil
}

func (m *Manifest) Command(name string) (*Command, error) {
	if len(m.Commands) == 0 {
		return nil, trace.Errorf("command %v is not found", name)
	}
	for _, c := range m.Commands {
		if c.Name == name {
			return &c, nil
		}
	}
	return nil, trace.Errorf("command %v is not found", name)
}

const Version = "0.0.1"

type manifestJSON struct {
	Version  string                                  `json:"version"`
	Config   json.RawMessage                         `json:"config,omitempty"`
	Commands []Command                               `json:"commands,omitempty"`
	Labels   []Label                                 `json:"labels,omitempty"`
	Service  *systemservice.NewPackageServiceRequest `json:"service,omitempty"`
}

type Command struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Args        []string `json:"args"`
}

// Tar packs the directory into orbit archive. Manifest is always
// checked if present. If it's not present and checkManifest is set to
// false, orbit still packs the archive - manifests are optional
func Tar(path string, checkManifest bool) (io.ReadCloser, error) {
	_, err := os.Stat(filepath.Join(path, ManifestFilename))
	// in case of unknown error, abort
	if err != nil {
		// if it's unknown error or we were asked to check
		// manifest, abort
		if !os.IsNotExist(err) || checkManifest {
			return nil, trace.Wrap(err)
		}
	} else {
		f, err := os.Open(filepath.Join(path, ManifestFilename))
		if err != nil {
			if !os.IsNotExist(err) || checkManifest {
				return nil, trace.Wrap(err, "failed to open manifest")
			}
		}
		defer f.Close()
		if _, err := ParseManifestJSON(f); err != nil {
			return nil, err
		}
	}

	options := archive.DefaultOptions()
	options.Compression = dockerarchive.Gzip
	return dockerarchive.TarWithOptions(path, options)
}

func Untar(r io.Reader, target string) (*Manifest, error) {
	if err := dockerarchive.Untar(
		r, target, archive.DefaultOptions()); err != nil {
		return nil, trace.Wrap(err)
	}
	f, err := os.Open(filepath.Join(target, ManifestFilename))
	if err != nil && !os.IsNotExist(err) {
		return nil, trace.Wrap(err, "failed to open manifest")
	}
	defer f.Close()
	return ParseManifestJSON(f)
}

func OpenManifest(dir string) (*Manifest, error) {
	f, err := os.Open(filepath.Join(dir, ManifestFilename))
	if err != nil {
		return nil, trace.Wrap(err, "failed to open manifest")
	}
	defer f.Close()
	return ParseManifestJSON(f)
}

// ReadManifest returns the contents of the manifest file from the specified tarball
func ReadManifest(tarball *tar.Reader) (*Manifest, error) {
	var manifest []byte
	err := archive.TarGlob(tarball, ".", []string{ManifestFilename},
		func(match string, file io.Reader) (err error) {
			manifest, err = ioutil.ReadAll(file)
			if err != nil {
				return trace.ConvertSystemError(err)
			}
			return archive.ErrAbort
		},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if manifest == nil {
		return nil, trace.NotFound("package manifest %q not found", ManifestFilename)
	}

	return ParseManifestJSON(bytes.NewReader(manifest))
}

func ParseManifestJSON(r io.Reader) (*Manifest, error) {
	var j *manifestJSON
	if err := json.NewDecoder(r).Decode(&j); err != nil {
		return nil, trace.Wrap(err)
	}
	if j.Version != Version {
		return nil, trace.Errorf("unsupported version: %v", j.Version)
	}
	m := &Manifest{
		Version: j.Version,
	}
	if len(j.Config) != 0 {
		c, err := schema.ParseJSON(bytes.NewReader(j.Config))
		if err != nil {
			return nil, err
		}
		m.Config = c
	}

	seen := map[string]bool{}
	if len(j.Commands) != 0 {
		for _, c := range j.Commands {
			if err := checkWord(c.Name); err != nil {
				return nil, err
			}
			if seen[c.Name] {
				return nil, trace.Errorf(
					"command '%v' already defined",
					c.Name)
			}
			seen[c.Name] = true
			if len(c.Args) == 0 {
				return nil, trace.Errorf(
					"please supply at least one argument for command '%v'",
					c.Name)
			}
		}
	}
	m.Commands = j.Commands

	if len(j.Labels) != 0 {
		for _, c := range j.Labels {
			if err := checkWord(c.Name); err != nil {
				return nil, err
			}
		}
	}
	m.Labels = j.Labels
	m.Service = j.Service
	return m, nil
}

func checkWord(val string) error {
	if !regexp.MustCompile("^[a-zA-z][a-zA-Z0-9_-]*$").MatchString(val) {
		return trace.Errorf("unlike '%v', workd can start with letter and contain letters, numbers, underscore and dash", val)
	}
	return nil
}

// ManifestFilename names the manifest file inside a package
const ManifestFilename = "orbit.manifest.json"
