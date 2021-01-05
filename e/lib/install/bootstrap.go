package install

import (
	"github.com/gravitational/gravity/lib/app"
	appservice "github.com/gravitational/gravity/lib/app/service"
	ossinstall "github.com/gravitational/gravity/lib/install"

	"github.com/gravitational/trace"
)

// EnsureApp makes sure that the installer has the application being
// installed available locally
//
// If there's no installer tarball (and hence, no application), then it
// replicates the metadata of the application and its dependencies from
// remote Ops Center (for which remote Ops Center credentials should have
// been provided).
//
// Only package metadata ("envelopes" in our terminology) is downloaded
// at this point, the actual blobs will be downloaded later during installation
func EnsureApp(req appservice.AppPullRequest) (*app.Application, error) {
	// first see if we have the installer tarball
	tarballApp, err := ossinstall.GetApp(req.DstApp)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	if tarballApp != nil {
		req.WithField("app", tarballApp).Info("Found user app in the installer tarball.")
		return tarballApp, nil
	}
	// download only package envelopes
	req.MetadataOnly = true
	req.Info("No application data is available locally, downloading from Gravity Hub.")
	app, err := appservice.PullApp(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return app, nil
}
