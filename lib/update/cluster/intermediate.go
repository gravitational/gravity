package cluster

import (
	"sort"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/pack"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"
	"github.com/pborman/uuid"
)

func collectIntermediateUpgrades(packages pack.PackageService) (upgrades []intermediateUpgrade, err error) {
	result := make(map[string]*intermediateUpgrade)
	err = pack.ForeachPackage(packages, func(env pack.PackageEnvelope) error {
		labels := pack.Labels(env.RuntimeLabels)
		if !labels.HasPurpose(pack.PurposeRuntimeUpgrade) {
			return nil
		}
		version := labels[pack.PurposeRuntimeUpgrade]
		if result[version] == nil {
			v, err := semver.NewVersion(version)
			if err != nil {
				return trace.Wrap(err, "invalid semver: %q", version)
			}
			result[version] = newIntermediateUpgrade(*v)
		}
		result[version].fromPackage(env)
		return nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	upgrades = make([]intermediateUpgrade, 0, len(result))
	for _, upgrade := range result {
		upgrades = append(upgrades, *upgrade)
	}
	sort.Sort(upgradesByVersion(upgrades))
	return upgrades, nil
}

func (r upgradesByVersion) Len() int           { return len(r) }
func (r upgradesByVersion) Swap(i, j int)      { r[i], r[j] = r[j], r[i] }
func (r upgradesByVersion) Less(i, j int) bool { return r[i].version.Compare(r[j].version) < 0 }

type upgradesByVersion []intermediateUpgrade

func newIntermediateUpgrade(version semver.Version) *intermediateUpgrade {
	return &intermediateUpgrade{
		changesetID: uuid.New(),
		version:     version,
	}
}

func (r *intermediateUpgrade) fromPackage(env pack.PackageEnvelope) {
	switch env.Locator.Name {
	case constants.PlanetPackage:
		r.runtime = env.Locator
	case constants.TeleportPackage:
		r.teleport = &env.Locator
	case constants.GravityPackage:
		r.gravity = env.Locator
	default:
		if env.Type != "" {
			r.apps = append(r.apps, env.Locator)
		}
	}
}

// intermediateUpgrade groups package dependencies for a specific
// intermediate version of the runtime
type intermediateUpgrade struct {
	// version defines the runtime application version as semver
	version semver.Version
	// changesetID defines the ID for the system update operation
	changesetID string
	// gravity specifies the package with the gravity binary
	gravity loc.Locator
	// runtime specifies the planet package
	runtime loc.Locator
	// teleport specifies the package with teleport
	teleport *loc.Locator
	// apps lists system application packages
	apps []loc.Locator
}
