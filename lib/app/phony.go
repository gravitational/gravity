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

package app

import (
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/schema"
)

// Phony describes a broken application reference.
// It is used to refer to missing applications in sites that have invalid application reference.
// This is a temporary measure in absence of the enforcing FOREIGN KEY constraint to bridge
// the transition to when it is either possible to update an application in-use or application
// packages are never invalidated (and instead new versions are released)
var Phony *Application

func init() {
	Phony = &Application{
		Package: loc.Locator{
			Repository: "phony",
			Name:       "unknown",
			Version:    "0.0.0",
		},
		Manifest: schema.Manifest{
			Dependencies: schema.Dependencies{
				Packages: []schema.Dependency{
					{
						Locator: loc.Locator{
							Repository: "phony",
							Name:       constants.TeleportPackage,
							Version:    "0.0.0",
						},
					},
					{
						Locator: loc.Locator{
							Repository: "phony",
							Name:       constants.WebAssetsPackage,
							Version:    "0.0.0",
						},
					},
					{
						Locator: loc.Locator{
							Repository: "phony",
							Name:       constants.GravityPackage,
							Version:    "0.0.0",
						},
					},
				},
			},
		},
	}
}
