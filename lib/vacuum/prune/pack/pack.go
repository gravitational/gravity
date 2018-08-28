package pack

import (
	"context"
	"strings"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/utils"
	"github.com/gravitational/gravity/lib/vacuum/prune"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// New creates a new package vacuum cleaner
func New(config Config) (*cleanup, error) {
	if err := config.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	base := config.App.Manifest.Base()
	if base == nil {
		return nil, trace.BadParameter("cluster application does not have a runtime")
	}
	baseVersion, err := base.SemVer()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &cleanup{
		Config:         config,
		runtimeVersion: *baseVersion,
	}, nil
}

func (r *Config) checkAndSetDefaults() error {
	if r.App == nil {
		return trace.BadParameter("application package is required")
	}
	if r.Packages == nil {
		return trace.BadParameter("package service is required")
	}
	if r.FieldLogger == nil {
		r.FieldLogger = log.WithField(trace.Component, "gc:package")
	}
	if r.Emitter == nil {
		r.Emitter = utils.NopEmitter()
	}
	return nil
}

// Config describes configuration for the cleaner of unused packages
type Config struct {
	// Config specifies the common pruner configuration
	prune.Config
	// App specifies the cluster application
	App *Application
	// Apps lists other cluster applications.
	// There might be several applications meaningful for the cluster
	// if it's an Ops Center and has been connected with multiple remote
	// clusters.
	Apps []Application
	// Packages specifies the package service to prune
	Packages packageService
}

// Application describes an application for the package cleaner
type Application struct {
	// Locator references the application package
	loc.Locator
	// Manifest is the application's manifest
	schema.Manifest
}

// packageService defines the subset of package APIs as required for pruning
type packageService interface {
	GetRepositories() ([]string, error)
	GetPackages(respository string) ([]pack.PackageEnvelope, error)
	DeletePackage(loc.Locator) error
}

// Prune removes unused packages from the conigured package service.
// It uses the direct application dependencies to determine the set of packages
// that are still required, and sweeps the rest.
// It will not remove packages from repositories other than the defaults.SystemAccountOrg
// unless it can tell if a package is safe to remove.
func (r *cleanup) Prune(context.Context) error {
	repositories, err := r.Packages.GetRepositories()
	if err != nil {
		return trace.Wrap(err)
	}

	required, err := r.mark()
	if err != nil {
		return trace.Wrap(err)
	}

	for _, repository := range repositories {
		envelopes, err := r.Packages.GetPackages(repository)
		if err != nil {
			return trace.Wrap(err)
		}
		for _, envelope := range envelopes {
			shouldDelete, err := r.shouldDeletePackage(envelope, required)
			if err != nil {
				return trace.Wrap(err)
			}
			if !shouldDelete {
				continue
			}
			r.PrintStep("Deleting package %v.", envelope.Locator)
			if r.DryRun {
				continue
			}
			err = r.Packages.DeletePackage(envelope.Locator)
			if err != nil {
				return trace.Wrap(err)
			}
		}
	}

	return nil
}

// mark marks the direct application and package dependencies of the cluster
// application as required.
// Returns the map of package locator -> descriptor for packages that are not
// eligible for removal
func (r *cleanup) mark() (required packageMap, err error) {
	dependencies := append(r.App.Manifest.AllPackageDependencies(),
		r.App.Manifest.Dependencies.GetApps()...)
	dependencies = append(dependencies, r.App.Locator)
	required = make(packageMap)
	for _, app := range r.Apps {
		dependencies = append(dependencies, app.Manifest.AllPackageDependencies()...)
		dependencies = append(dependencies, app.Manifest.Dependencies.GetApps()...)
		dependencies = append(dependencies, app.Locator)
	}
	for _, dependency := range dependencies {
		semver, err := dependency.SemVer()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		r.PrintStep("Mark package %v as required.", dependency)
		required[dependency.ZeroVersion()] = requiredPackage{
			Version: *semver,
			Locator: dependency,
		}
	}
	return required, nil
}

// shouldDeletePackage determines if the specified package pkg is eligible for removal.
// It will match the package against the specified map of required packages.
// It will also apply a couple of additional ad-hoc heuristics to decide if a package
// should be deleted
func (r *cleanup) shouldDeletePackage(envelope pack.PackageEnvelope, required packageMap) (delete bool, err error) {
	log := r.WithField("package", envelope.Locator)

	version, err := envelope.Locator.SemVer()
	if err != nil {
		return false, trace.Wrap(err)
	}

	if isPlanetConfigPackage(envelope) {
		runtimePackage, err := packageForConfig(envelope, required)
		if err != nil {
			return false, trace.Wrap(err)
		}
		runtimeVersion, err := runtimePackage.Locator.SemVer()
		if err != nil {
			return false, trace.Wrap(err)
		}
		if version.Compare(*runtimeVersion) < 0 {
			log.Debug("Will delete an obsolete runtime configuration package.")
			return true, nil
		}
	}

	if isAppResourcesPackage(envelope) {
		appPackage, err := packageForResources(envelope, required)
		if err != nil {
			return false, trace.Wrap(err)
		}
		appVersion, err := appPackage.Locator.SemVer()
		if err != nil {
			return false, trace.Wrap(err)
		}
		if version.Compare(*appVersion) < 0 {
			log.Debug("Will delete an obsolete application resources package.")
			return true, nil
		}
	}

	if isLegacyRuntimePackage(envelope) {
		log.Debug("Will delete a legacy runtime package.")
		return true, nil
	}

	if isRPCUpdateCredentialsPackage(envelope) && version.Compare(r.runtimeVersion) < 0 {
		// Remove obsolete update RPC credentials used in prior update operations.
		// All RPC packages with versions prior or equal to the currently
		// installed runtime version are eligible for removal.
		log.Debug("Will delete an obsolete RPC credentials package.")
		return true, nil
	}

	if existingVersion, exists := required[envelope.Locator.ZeroVersion()]; exists {
		if existingVersion.Compare(*version) > 0 {
			log.Debug("Will delete an obsolete package.")
			return true, nil
		}
		log.Debug("Will not delete a package still in use.")
	}

	if envelope.Locator.Repository != defaults.SystemAccountOrg {
		log.Debug("Will not delete from a custom repository.")
		return false, nil
	}

	return false, nil
}

func packageForConfig(envelope pack.PackageEnvelope, packages packageMap) (*requiredPackage, error) {
	parentPackageName := envelope.RuntimeLabels[pack.ConfigLabel]
	parentPackageFilter, err := loc.ParseLocator(parentPackageName)
	if err != nil {
		return nil, trace.Wrap(err, "invalid package locator")
	}
	for packageFilter, pkg := range packages {
		if parentPackageFilter.IsEqualTo(packageFilter) {
			return &pkg, nil
		}
	}
	return nil, trace.NotFound("no package found for configuration package %v", envelope.Locator)
}

func packageForResources(envelope pack.PackageEnvelope, packages packageMap) (*requiredPackage, error) {
	appPackageName := strings.TrimSuffix(envelope.Locator.Name, "-resources")
	appPackageFilter := loc.Locator{
		Repository: envelope.Locator.Repository,
		Name:       appPackageName,
		Version:    loc.ZeroVersion,
	}
	for packageFilter, pkg := range packages {
		if appPackageFilter.IsEqualTo(packageFilter) {
			return &pkg, nil
		}
	}
	return nil, trace.NotFound("no package found for resources package %v", envelope.Locator)
}

func isAppResourcesPackage(envelope pack.PackageEnvelope) bool {
	return strings.HasSuffix(envelope.Locator.Name, "-resources")
}

func isLegacyRuntimePackage(envelope pack.PackageEnvelope) bool {
	if envelope.Locator.Repository != loc.LegacyPlanetMaster.Repository {
		// Skip runtime package with a non-standard repository
		return false
	}
	switch envelope.Locator.Name {
	case loc.LegacyPlanetMaster.Name, loc.LegacyPlanetNode.Name:
		return true
	default:
		return false
	}
}

func isPlanetConfigPackage(envelope pack.PackageEnvelope) bool {
	label, exists := envelope.RuntimeLabels[pack.PurposeLabel]
	return exists && label == pack.PurposePlanetConfig
}

// isRPCUpdateCredentialsPackage returns true if the specified package is an RPC credentials
// package used in prior update operations
func isRPCUpdateCredentialsPackage(envelope pack.PackageEnvelope) bool {
	if envelope.Locator.Repository == defaults.SystemAccountOrg {
		// Skip the long-term RPC credentials package
		return false
	}
	if label, exists := envelope.RuntimeLabels[pack.PurposeLabel]; exists && label == pack.PurposeRPCCredentials {
		return true
	}
	// Match the RPC package w/o label by name
	pattern := loc.Locator{
		Repository: envelope.Locator.Repository,
		Name:       defaults.RPCAgentSecretsPackage,
		Version:    loc.ZeroVersion,
	}
	return envelope.Locator.ZeroVersion().IsEqualTo(pattern)
}

type packageMap map[loc.Locator]requiredPackage

type requiredPackage struct {
	semver.Version
	loc.Locator
}

type cleanup struct {
	Config
	// runtimeVersion specifies the version of telekube
	runtimeVersion semver.Version
}
