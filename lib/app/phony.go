package app

import (
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/schema"
)

// Phony describes a broken application reference.
// It is used to refer to missing applications in sites that have invalid application reference.
// This is a temporary measure in abscence of the enforcing FOREIGN KEY constraint to bridge
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
