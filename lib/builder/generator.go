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

package builder

import (
	"io"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/schema"
)

// Generator defines a method for generating standalone installers
type Generator interface {
	// Generate generates an installer tarball for the specified application
	// using the provided builder and returns its data as a stream
	Generate(*Engine, *schema.Manifest, app.InstallerRequest) (io.ReadCloser, error)
}

type generator struct{}

// Generate generates an installer tarball for the specified application
// using the provided builder and returns its data as a stream
func (g *generator) Generate(engine *Engine, manifest *schema.Manifest, req app.InstallerRequest) (io.ReadCloser, error) {
	return engine.Apps.GetAppInstaller(req)
}
