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
	"github.com/gravitational/gravity/lib/app/service"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/state"
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

	serviceUser, err := systeminfo.UserFromOSUser(*p.Phase.Data.ServiceUser)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	app, err := wizardApps.GetApp(*p.Phase.Data.Package)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	runtimePackage, err := app.Manifest.RuntimePackageForProfile(p.Phase.Data.Server.Role)
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
	// ExecutorParams is common executor params
	fsm.ExecutorParams
	// remote specifies the server remote control interface
	remote fsm.Remote
	// runtimePackage specifies the runtime container package to pull
	runtimePackage loc.Locator
}

// Execute executes the pull phase
func (p *pullExecutor) Execute(ctx context.Context) error {
	// If the list of packages to pull was explicitly provided, pull only those
	// (e.g. during join), otherwise pull the entire user application (e.g.
	// during initial installation).
	if len(p.Phase.Data.Packages) == 0 {
		err := p.pullUserApplication()
		if err != nil {
			return trace.Wrap(err)
		}
	} else {
		err := p.pullPackages(p.Phase.Data.Packages)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	err := p.pullConfiguredPackages()
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

func (p *pullExecutor) pullPackages(locators []loc.Locator) error {
	p.Progress.NextStep("Pulling packages from cluster")
	p.Info("Pulling packages from cluster.")
	for _, locator := range locators {
		_, err := service.PullPackage(service.PackagePullRequest{
			FieldLogger: p.FieldLogger,
			SrcPack:     p.WizardPackages,
			DstPack:     p.LocalPackages,
			Package:     locator,
		})
		if err != nil && !trace.IsAlreadyExists(err) { // Make sure it's re-entrable.
			return trace.Wrap(err)
		}
	}
	return nil
}

func (p *pullExecutor) pullUserApplication() error {
	p.Progress.NextStep("Pulling user application")
	p.Info("Pulling user application.")
	// TODO do not pull user app on regular nodes
	// FIXME: use context to promptly abort the pull
	_, err := service.PullApp(service.AppPullRequest{
		FieldLogger: p.FieldLogger,
		SrcPack:     p.WizardPackages,
		DstPack:     p.LocalPackages,
		SrcApp:      p.WizardApps,
		DstApp:      p.LocalApps,
		Package:     *p.Phase.Data.Package,
	})
	// Ignore already exists as the steps need to be re-entrant
	if err != nil && !trace.IsAlreadyExists(err) {
		return trace.Wrap(err)
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

func (p *pullExecutor) pullConfiguredPackages() (err error) {
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
		_, err := service.PullPackage(service.PackagePullRequest{
			SrcPack: p.WizardPackages,
			DstPack: p.LocalPackages,
			Package: e.Locator,
			Labels:  e.RuntimeLabels,
			Upsert:  true,
		})
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
