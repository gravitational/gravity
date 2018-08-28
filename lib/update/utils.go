package update

import (
	appservice "github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

func GetOperationPlan(b storage.Backend) (*storage.OperationPlan, error) {
	op, err := storage.GetLastOperation(b)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	plan, err := b.GetOperationPlan(op.SiteDomain, op.ID)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}

	if plan == nil {
		return nil, trace.NotFound(
			"%q does not have a plan, use 'gravity plan --init' to initialize it", op.Type)
	}

	changelog, err := b.GetOperationPlanChangelog(op.SiteDomain, op.ID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	plan = fsm.ResolvePlan(*plan, changelog)
	return plan, nil
}

// planetNeedsUpdate returns true if the planet version in the update application is
// greater than in the installed one for the specified node profile
func planetNeedsUpdate(profile string, installed, update appservice.Application) (needsUpdate bool, err error) {
	installedProfile, err := installed.Manifest.NodeProfiles.ByName(profile)
	if err != nil {
		return false, trace.Wrap(err)
	}

	updateProfile, err := update.Manifest.NodeProfiles.ByName(profile)
	if err != nil {
		return false, trace.Wrap(err)
	}

	updateRuntimePackage, err := update.Manifest.RuntimePackage(*updateProfile)
	if err != nil {
		return false, trace.Wrap(err)
	}

	updateVersion, err := updateRuntimePackage.SemVer()
	if err != nil {
		return false, trace.Wrap(err)
	}

	runtimePackage, err := installed.Manifest.RuntimePackage(*installedProfile)
	if err != nil && !trace.IsNotFound(err) {
		return false, trace.Wrap(err)
	}
	if err != nil {
		runtimePackage, err = installed.Manifest.Dependencies.ByName(loc.LegacyPlanetMaster.Name)
		if err != nil {
			logrus.Warnf("Failed to fetch the runtime package: %v.", err)
			return false, trace.NotFound("runtime package not found")
		}
	}

	version, err := runtimePackage.SemVer()
	if err != nil {
		return false, trace.Wrap(err)
	}

	logrus.Debugf("Runtime installed: %v, runtime to update to: %v.", runtimePackage, updateRuntimePackage)
	updateNewer := updateVersion.Compare(*version) > 0
	return updateNewer, nil
}
