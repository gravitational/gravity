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

package install

import (
	"context"
	"os"
	"time"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/state"
	systemstate "github.com/gravitational/gravity/lib/system/state"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
)

// bootstrap prepares the local installer state for the operation based
// on the installation mode
func (i *Installer) bootstrap(ctx context.Context) error {
	if i.Mode == constants.InstallModeInteractive {
		return nil
	}
	err := installBinary(i.ServiceUser.UID, i.ServiceUser.GID)
	if err != nil {
		return trace.Wrap(err, "failed to install binary")
	}
	err = i.configureStateDirectory()
	if err != nil {
		return trace.Wrap(err, "failed to configure state directory")
	}
	return nil
}

// Cleanup performs post-installation cleanups, e.g. tears down reverse tunnels
// that were required during installation
func (i *Installer) Cleanup(progress ops.ProgressEntry) error {
	var errors []error
	// do not take the actions below on failure
	if progress.State != ops.ProgressStateCompleted {
		return trace.NewAggregate(errors...)
	}
	if err := i.uploadInstallLog(); err != nil {
		errors = append(errors, err)
	}
	if err := i.completeFinalInstallStep(); err != nil {
		errors = append(errors, err)
	}
	return trace.NewAggregate(errors...)
}

// completeFinalInstallStep marks the final install step as completed unless
// the application has a custom install step - in which case it does nothing
// because it will be completed by user later
func (i *Installer) completeFinalInstallStep() error {
	// see if the app defines custom install step
	application, err := i.Apps.GetApp(i.AppPackage)
	if err != nil {
		return trace.Wrap(err)
	}
	// if app has a custom setup endpoint, user will complete it
	if application.Manifest.SetupEndpoint() != nil {
		return nil
	}
	// determine delay for removing connection from installed cluster to this
	// installer process - in case of interactive installs, we can not remove
	// the link right away because it is used to tunnel final install step
	var delay time.Duration
	if i.Mode == constants.InstallModeInteractive {
		delay = defaults.WizardLinkTTL
	}
	req := ops.CompleteFinalInstallStepRequest{
		AccountID:           defaults.SystemAccountID,
		SiteDomain:          i.SiteDomain,
		WizardConnectionTTL: delay,
	}
	i.Debugf("Completing final install step: %s.", req)
	if err := i.Operator.CompleteFinalInstallStep(req); err != nil {
		return trace.Wrap(err, "failed to complete final install step")
	}
	return nil
}

// OnPlanComplete is called when the operation plan finishes execution
func (i *Installer) OnPlanComplete(fsm *fsm.FSM, fsmErr error) {
	if err := fsm.Complete(fsmErr); err != nil {
		i.WithError(err).Error("Failed to complete operation.")
	}
}

// uploadInstallLog uploads user-facing operation log to the installed cluster
func (i *Installer) uploadInstallLog() error {
	file, err := os.Open(i.UserLogFile)
	if err != nil {
		return trace.Wrap(err)
	}
	defer file.Close()
	err = i.Operator.StreamOperationLogs(i.OperationKey, file)
	if err != nil {
		return trace.Wrap(err, "failed to upload install log")
	}
	i.Debug("Uploaded install log to the cluster.")
	return nil
}

// configureStateDirectory configures local gravity state directory
func (i *Installer) configureStateDirectory() error {
	stateDir, err := state.GetStateDir()
	if err != nil {
		return trace.Wrap(err)
	}
	err = systemstate.ConfigureStateDirectory(stateDir, i.SystemDevice)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// watchSignals installs a signal handler to be able trap a terminating signal
// and properly cleanup before exiting
func (i *Installer) watchSignals() {
	utils.WatchTerminationSignals(i.Context, i.Cancel, i, i.FieldLogger)
}
