package install

import (
	"context"

	"github.com/gravitational/gravity/e/lib/ops"
	libapp "github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/install"
	ossops "github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/license/authority"
	"github.com/gravitational/trace"
)

// bootstrap prepares the local installer state for the operation based
// on the installation mode
func (i *Installer) bootstrap(ctx context.Context) error {
	// make sure there's an application in the local installer database
	err := i.ensureApp(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	// if connection to remote Ops Center is configured, we'll have to
	// determine its cluster name since it may be different from its
	// advertise hostname
	if i.Remote != nil {
		err := i.setOpsCenterCluster()
		if err != nil {
			return trace.Wrap(err, "failed to determine Ops Center cluster name")
		}
	}
	// if installing via Ops Center, the installer process should use
	// the Ops Center's CA that is used to sign/verify licenses
	switch i.Mode {
	case constants.InstallModeOpsCenter:
		err := i.pullLicenseCA()
		if err != nil {
			return trace.Wrap(err, "failed to pull CA package from Ops Center")
		}
	}
	return nil
}

// ensureApp makes sure that the installer has the application being
// installed available locally
//
// If there's no installer tarball (and hence, no application), then it
// replicates the metadata of the application and its dependencies from
// remote Ops Center (for which remote Ops Center credentials should have
// been provided).
//
// Only package metadata ("envelopes" in our terminology) is downloaded
// at this point, the actual blobs will be downloaded later during first
// install phase.
func (i *Installer) ensureApp(ctx context.Context) error {
	// first see if we have the installer tarball
	tarballApp, err := install.GetApp(i.Apps)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	if tarballApp != nil {
		i.Infof("Found user app in the installer tarball: %s.", tarballApp)
		return nil
	}
	// there's no installer tarball, so we need to fetch the app from the
	// remote Ops Center, for this to work both app name and Ops Center
	// info should have been passed to the install command
	if i.Remote == nil {
		return trace.NotFound(
			"no installer tarball found and Ops Center credentials weren't specified")
	}
	// set the flag to download blobs when installation starts
	i.downloadInstaller = true
	i.Info("No application data is available locally, downloading from Ops Center.")
	puller := libapp.Puller{
		FieldLogger: i.FieldLogger,
		SrcPack:     i.Remote.Packages,
		DstPack:     i.Packages,
		SrcApp:      i.Remote.Apps,
		DstApp:      i.Apps,
		// download only package envelopes
		MetadataOnly: true,
	}
	err = puller.PullApp(ctx, i.AppPackage)
	return trace.Wrap(err)
}

// pullLicenseCA pulls Ops Center CA package and pushes it to the installer db
//
// Usually the CA package is a part of the installer tarball but when installing
// via Ops Center, there is no tarball.
func (i *Installer) pullLicenseCA() error {
	if i.Remote == nil {
		return trace.NotFound("please specify remote Ops Center credentials " +
			"using --ops-url and --ops-token flags")
	}
	certificate, err := i.Remote.Operator.GetLicenseCA()
	if err != nil {
		return trace.Wrap(err)
	}
	err = pack.CreateCertificateAuthority(pack.CreateCAParams{
		Packages: i.Packages,
		KeyPair:  authority.TLSKeyPair{CertPEM: certificate},
		Upsert:   true,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	i.Info("Pulled license CA from remote Ops Center.")
	return nil
}

// setOpsCenterCluster uses the configured remote Ops Center client to
// determine the Ops Center cluster name
func (i *Installer) setOpsCenterCluster() error {
	cluster, err := i.Remote.Operator.GetLocalSite()
	if err != nil {
		return trace.Wrap(err)
	}
	i.opsCenterCluster = cluster.Domain
	i.Infof("Remote Ops Center cluster name: %v.", i.opsCenterCluster)
	return nil
}

// Cleanup performs post-installation cleanups, e.g. tears down reverse tunnels
// that were required during installation
func (i *Installer) Cleanup(progress ossops.ProgressEntry) error {
	var errors []error
	// first let the open-source installer do the cleanup
	if err := i.Installer.Cleanup(progress); err != nil {
		errors = append(errors, err)
	}
	isSuccess := progress.State == ossops.ProgressStateCompleted
	if i.Mode == constants.InstallModeOpsCenter {
		if err := i.completeOpsOperation(isSuccess, progress.Message); err != nil {
			errors = append(errors, err)
		}
	}
	return trace.NewAggregate(errors...)
}

// completeOpsOperation marks the operation as completed or failed in
// the remote Ops Center
func (i *Installer) completeOpsOperation(success bool, message string) (err error) {
	if success {
		err = ossops.CompleteOperation(i.OperationKey, i.Remote.Operator)
	} else {
		err = ossops.FailOperation(i.OperationKey, i.Remote.Operator, message)
	}
	if err != nil {
		return trace.Wrap(err, "failed to mark Ops Center operation complete")
	}
	i.Info("Marked Ops Center operation complete.")
	return nil
}

// OnPlanComplete is called when the operation plan finishes execution
func (i *Installer) OnPlanComplete(fsm *fsm.FSM, fsmErr error) {
	// destroy the tunnel from wizard to Ops Center, if this fails for some
	// reason, it will eventually expire on the Ops Center side anyway after
	// the installer has shut down
	if i.Mode == constants.InstallModeOpsCenter {
		if err := i.removeOpsCenterConnection(); err != nil {
			i.Errorf("Failed to remove Ops Center connection: %v.",
				trace.DebugReport(err))
		}
	}
	// let the base installer perform cleanups as well
	i.Installer.OnPlanComplete(fsm, fsmErr)
}

// removeOpsCenterConnection destroys the connection from this install
// wizard process to the remote Ops Center
func (i *Installer) removeOpsCenterConnection() error {
	opsHost, err := utils.URLHostname(i.Config.RemoteOpsURL)
	if err != nil {
		return trace.Wrap(err)
	}
	req := ops.DeleteTrustedClusterRequest{
		AccountID:          defaults.SystemAccountID,
		ClusterName:        i.SiteDomain,
		TrustedClusterName: opsHost,
	}
	i.Debugf("Removing Ops Center connection: %s.", req)
	err = i.Operator.DeleteTrustedCluster(req)
	if err != nil {
		return trace.Wrap(err, "failed to destroy Ops Center connection")
	}
	return nil
}
