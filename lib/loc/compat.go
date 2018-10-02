package loc

// IsLegacyRuntimePackage returns true if the specified package envelope describes
// a legacy runtime package
func IsLegacyRuntimePackage(loc Locator) bool {
	if loc.Repository != LegacyPlanetMaster.Repository {
		// Skip runtime package with a non-standard repository
		return false
	}
	switch loc.Name {
	case LegacyPlanetMaster.Name, LegacyPlanetNode.Name:
		return true
	default:
		return false
	}
}
