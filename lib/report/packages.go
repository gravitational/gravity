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
package report

import (
	"context"
	"io"
	"time"

	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/dustin/go-humanize"
	"github.com/gravitational/trace"
	"gopkg.in/yaml.v2"
	serializer "k8s.io/apimachinery/pkg/runtime/serializer/json"
)

// NewPackageCollector returns a new package collector for the specified package service
func NewPackageCollector(packages pack.PackageService) *PackageCollector {
	return &PackageCollector{
		packages: packages,
	}
}

// Collect iterates through the system packages and outputs them
// using the specified reportWriter.
func (r PackageCollector) Collect(ctx context.Context, reportWriter FileWriter, runner utils.CommandRunner) error {
	w, err := reportWriter.NewWriter("gravity-packages.yaml")
	if err != nil {
		return trace.Wrap(err)
	}
	framer := serializer.YAMLFramer.NewFrameWriter(w)
	defer w.Close()

	return pack.ForeachPackage(r.packages, formatPackage(framer))
}

func formatPackage(w io.Writer) func(pack.PackageEnvelope) error {
	return func(env pack.PackageEnvelope) error {
		pkg := &envelope{
			Name:      locator(env.Locator),
			Size:      size(env.SizeBytes),
			Digest:    env.SHA512,
			Labels:    env.RuntimeLabels,
			Hidden:    env.Hidden,
			Encrypted: env.Encrypted,
			Type:      env.Type,
			Created:   env.Created,
		}
		if !env.Encrypted {
			pkg.Manifest = manifest(env.Manifest)
		}

		out, err := yaml.Marshal(pkg)
		if err != nil {
			return trace.Wrap(err)
		}

		_, err = w.Write(out)
		return trace.ConvertSystemError(err)
	}
}

// PackageCollector collects package-specific diagnostic information
type PackageCollector struct {
	packages pack.PackageService
}

type envelope struct {
	Name      locator           `yaml:"name"`
	Size      size              `yaml:"size"`
	Digest    string            `yaml:"digest"`
	Labels    map[string]string `yaml:"labels,omitempty"`
	Hidden    bool              `yaml:"hidden"`
	Encrypted bool              `yaml:"encrypted"`
	Type      string            `yaml:"type,omitempty"`
	Created   time.Time         `yaml:"created"`
	Manifest  manifest          `yaml:"manifest,omitempty"`
}

// MarshalYAML formats this locator value as a package name in YAML
func (r locator) MarshalYAML() (interface{}, error) {
	return (loc.Locator)(r).String(), nil
}

type locator loc.Locator

// MarshalYAML formats this size as human-readable value in YAML
func (r size) MarshalYAML() (interface{}, error) {
	return humanize.Bytes(uint64(r)), nil
}

type size int64

// MarshalYAML formats this manifest in YAML as text
func (r manifest) MarshalYAML() (interface{}, error) {
	return string(r), nil
}

type manifest []byte
