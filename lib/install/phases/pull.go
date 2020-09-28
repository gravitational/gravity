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

package phases

import (
	"context"
	"path/filepath"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/state"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/systeminfo"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/idtools"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// NewPull returns a new "pull" phase executor
func NewPull(p fsm.ExecutorParams, operator ops.Operator, wizardPack, localPack pack.PackageService,
	wizardApps, localApps app.Applications, remote fsm.Remote) (*pullExecutor, error) {
	if p.Phase.Data == nil || p.Phase.Data.ServiceUser == nil {
		return nil, trace.BadParameter("service user is required")
	}
	if p.Phase.Data.Pull == nil {
		return nil, trace.BadParameter("phase does not contain pull data")
	}

	serviceUser, err := systeminfo.UserFromOSUser(*p.Phase.Data.ServiceUser)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	app, err := wizardApps.GetApp(*p.Phase.Data.Package)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	runtimePackage, err := app.Manifest.RuntimePackageForProfileName(p.Phase.Data.Server.Role)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	logger := &fsm.Logger{
		FieldLogger: logrus.WithFields(logrus.Fields{
			constants.FieldPhase:       p.Phase.ID,
			constants.FieldAdvertiseIP: p.Phase.Data.Server.AdvertiseIP,
			constants.FieldHostname:    p.Phase.Data.Server.Hostname,
		}),
		Key:      opKey(p.Plan),
		Operator: operator,
		Server:   p.Phase.Data.Server,
	}
	return &pullExecutor{
		FieldLogger:    logger,
		WizardPackages: wizardPack,
		WizardApps:     wizardApps,
		LocalPackages:  localPack,
		LocalApps:      localApps,
		ExecutorParams: p,
		ServiceUser:    *serviceUser,
		Pull:           *p.Phase.Data.Pull,
		runtimePackage: *runtimePackage,
		remote:         remote,
	}, nil
}

type pullExecutor struct {
	// FieldLogger is used for logging
	logrus.FieldLogger
	// WizardPackages is the installer process pack service
	WizardPackages pack.PackageService
	// WizardApps is the installer process app service
	WizardApps app.Applications
	// LocalPackages is the machine-local pack service
	LocalPackages pack.PackageService
	// LocalApps is the machine-local app service
	LocalApps app.Applications
	// ServiceUser is the user used for services and system storage
	ServiceUser systeminfo.User
	// Pull contains applications and packages to pull
	Pull storage.PullData
	// ExecutorParams is common executor params
	fsm.ExecutorParams
	// remote specifies the server remote control interface
	remote fsm.Remote
	// runtimePackage specifies the runtime container package to pull
	runtimePackage loc.Locator
}

// Execute executes the pull phase
func (p *pullExecutor) Execute(ctx context.Context) error {
	if len(p.Pull.Packages) != 0 {
		err := p.pullPackages(ctx, p.Pull.Packages)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	if len(p.Pull.Apps) != 0 {
		err := p.pullApps(ctx, p.Pull.Apps)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	err := p.pullConfiguredPackages(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	err = p.applyPackageLabels()
	if err != nil {
		return trace.Wrap(err)
	}
	err = p.unpackPackages()
	if err != nil {
		return trace.Wrap(err)
	}
	// after all pulling and unpacking has been done, set proper ownership
	// on the data dir
	stateDir, err := state.GetStateDir()
	if err != nil {
		return trace.Wrap(err)
	}
	err = utils.Chown(filepath.Join(stateDir, defaults.LocalDir),
		p.ServiceUser.UID, p.ServiceUser.GID)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (p *pullExecutor) pullPackages(ctx context.Context, locators []loc.Locator) error {
	p.Progress.NextStep("Pulling packages")
	p.Infof("Pulling packages: %v.", locators)
	for _, locator := range locators {
		p.Progress.NextStep("Pulling package %v:%v", locator.Name, locator.Version)
		puller := app.Puller{
			FieldLogger: p.FieldLogger,
			SrcPack:     p.WizardPackages,
			DstPack:     p.LocalPackages,
		}
		err := puller.PullPackage(ctx, locator)
		if err != nil && !trace.IsAlreadyExists(err) { // Must be re-entrant.
			return trace.Wrap(err)
		}
	}
	return nil
}

func (p *pullExecutor) pullApps(ctx context.Context, locators []loc.Locator) error {
	p.Progress.NextStep("Pulling applications")
	p.Infof("Pulling applications: %v.", locators)
	for _, locator := range locators {
		p.Progress.NextStep("Pulling application %v:%v", locator.Name, locator.Version)
		puller := app.Puller{
			FieldLogger: p.FieldLogger,
			SrcPack:     p.WizardPackages,
			DstPack:     p.LocalPackages,
			SrcApp:      p.WizardApps,
			DstApp:      p.LocalApps,
		}
		err := puller.PullApp(ctx, locator)
		if err != nil && !trace.IsAlreadyExists(err) { // Must be re-entrant.
			return trace.Wrap(err)
		}
	}
	return nil
}

// applyPackageLabels adds labels to system packages in order for update
// to properly detect an installed version
func (p *pullExecutor) applyPackageLabels() error {
	packages := []string{
		constants.TeleportPackage,
		constants.TeleportNodeConfigPackage,
		constants.GravityPackage,
	}
	purposeLabels := []string{
		pack.PurposePlanetConfig,
		pack.PurposePlanetSecrets,
		pack.PurposeTeleportNodeConfig,
	}
	var locators []loc.Locator
	err := pack.ForeachPackage(p.LocalPackages,
		func(e pack.PackageEnvelope) error {
			if utils.StringInSlice(packages, e.Locator.Name) ||
				pack.Labels(e.RuntimeLabels).HasPurpose(purposeLabels...) {
				locators = append(locators, e.Locator)
			}
			return nil
		})
	if err != nil {
		return trace.Wrap(err)
	}
	for _, locator := range locators {
		p.Infof("Marking installed package: %v.", locator)
		err := p.LocalPackages.UpdatePackageLabels(locator, map[string]string{
			pack.InstalledLabel: pack.InstalledLabel,
		}, nil)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	p.Infof("Marking runtime package: %v.", p.runtimePackage)
	err = p.LocalPackages.UpdatePackageLabels(p.runtimePackage, map[string]string{
		pack.InstalledLabel: pack.InstalledLabel,
		pack.PurposeLabel:   pack.PurposeRuntime,
	}, nil)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (p *pullExecutor) pullConfiguredPackages(ctx context.Context) (err error) {
	p.Progress.NextStep("Pulling configured packages")
	p.Info("Pulling configured packages.")
	var envelopes []pack.PackageEnvelope
	if p.Phase.Data.Server.ClusterRole == string(schema.ServiceRoleMaster) {
		envelopes, err = p.collectMasterPackages()
	} else {
		envelopes, err = p.collectNodePackages()
	}
	if err != nil {
		return trace.Wrap(err)
	}
	for _, e := range envelopes {
		puller := app.Puller{
			SrcPack: p.WizardPackages,
			DstPack: p.LocalPackages,
			Labels:  e.RuntimeLabels,
		}
		err := puller.PullPackage(ctx, e.Locator)
		// Ignore already exists as the steps need to be re-entrant
		if err != nil && !trace.IsAlreadyExists(err) {
			return trace.Wrap(err)
		}
		if isSecret(e) {
			err := p.unpackSecrets(e)
			if err != nil {
				return trace.Wrap(err)
			}
		}
	}
	return nil
}

func (p *pullExecutor) unpackSecrets(e pack.PackageEnvelope) error {
	stateDir, err := state.GetStateDir()
	if err != nil {
		return trace.Wrap(err)
	}
	dir := filepath.Join(stateDir, defaults.SecretsDir)
	p.Infof("Unpacking secrets into %v.", dir)
	return pack.Unpack(p.LocalPackages, e.Locator, dir, &archive.TarOptions{
		ChownOpts: &idtools.Identity{
			UID: p.ServiceUser.UID,
			GID: p.ServiceUser.GID,
		},
	})
}

func (p *pullExecutor) collectMasterPackages() ([]pack.PackageEnvelope, error) {
	var envelopes []pack.PackageEnvelope
	// iterate over all packages in this cluster's repository
	// and build a list of packages to pull
	err := pack.ForeachPackageInRepo(p.WizardPackages, p.Plan.ClusterName,
		func(e pack.PackageEnvelope) error {
			pull := e.HasAnyLabel(map[string][]string{
				pack.PurposeLabel: {
					pack.PurposeCA,
					pack.PurposeExport,
					pack.PurposeLicense,
					pack.PurposeResources,
				},
				pack.AdvertiseIPLabel: {
					p.Phase.Data.Server.AdvertiseIP,
				},
			})
			if pull && e.RuntimeLabels[pack.OperationIDLabel] == p.Plan.OperationID {
				envelopes = append(envelopes, e)
			}
			return nil
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return envelopes, nil
}

func (p *pullExecutor) collectNodePackages() ([]pack.PackageEnvelope, error) {
	var envelopes []pack.PackageEnvelope
	// iterate over all packages in this cluster's repository
	// and build a list of packages to pull
	err := pack.ForeachPackageInRepo(p.WizardPackages, p.Plan.ClusterName,
		func(e pack.PackageEnvelope) error {
			pull := e.HasAnyLabel(map[string][]string{
				pack.AdvertiseIPLabel: {
					p.Phase.Data.Server.AdvertiseIP,
				},
			})
			if pull && e.RuntimeLabels[pack.OperationIDLabel] == p.Plan.OperationID {
				envelopes = append(envelopes, e)
			}
			return nil
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return envelopes, nil
}

// unpackPackages unpacks packages setting proper ownership
func (p *pullExecutor) unpackPackages() error {
	p.Progress.NextStep("Unpacking pulled packages")
	p.Info("Unpacking pulled packages.")
	// collect packages that need to be unpacked
	packages := []string{
		constants.TeleportPackage,
		constants.WebAssetsPackage,
	}
	var locators []loc.Locator
	err := pack.ForeachPackage(p.LocalPackages, func(e pack.PackageEnvelope) error {
		unpack := e.HasAnyLabel(map[string][]string{
			pack.PurposeLabel: {
				pack.PurposeCA,
				pack.PurposePlanetSecrets,
				pack.PurposePlanetConfig,
				pack.PurposeTeleportMasterConfig,
				pack.PurposeTeleportNodeConfig,
			},
		})
		if unpack || utils.StringInSlice(packages, e.Locator.Name) {
			locators = append(locators, e.Locator)
		}
		return nil
	})
	if err != nil {
		return trace.Wrap(err)
	}
	for _, locator := range locators {
		p.Infof("Unpacking package %v.", locator)
		err := pack.Unpack(p.LocalPackages, locator, "", nil)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// Rollback is no-op for this phase
func (*pullExecutor) Rollback(ctx context.Context) error {
	return nil
}

// PreCheck makes sure this phase is executed on a proper node
func (p *pullExecutor) PreCheck(ctx context.Context) error {
	err := p.remote.CheckServer(ctx, *p.Phase.Data.Server)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// PostCheck is no-op for this phase
func (*pullExecutor) PostCheck(ctx context.Context) error {
	return nil
}

// isSecret returns true if the provided envelope is for a secrets package
func isSecret(e pack.PackageEnvelope) bool {
	return e.HasLabel(pack.PurposeLabel, pack.PurposePlanetSecrets)
}
