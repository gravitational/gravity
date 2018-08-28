package builder

import (
	"io"

	"github.com/gravitational/gravity/lib/app"
)

// Generator defines a method for generating standalone installers
type Generator interface {
	// Generate generates an installer tarball for the specified application
	// using the provided builder and returns its data as a stream
	Generate(*Builder, app.Application) (io.ReadCloser, error)
}

type generator struct{}

// Generate generates an installer tarball for the specified application
// using the provided builder and returns its data as a stream
func (g *generator) Generate(builder *Builder, application app.Application) (io.ReadCloser, error) {
	return builder.Apps.GetAppInstaller(app.InstallerRequest{
		Application: application.Package,
	})
}
