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

package loc

import (
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/configure/cstrings"
	"github.com/gravitational/trace"
)

const (
	// ZeroVersion is a special "zero" package version that means "no particular version"
	ZeroVersion = "0.0.0"

	// FirstVersion is a special package version that indicates the initial package version
	FirstVersion = "0.0.1"

	// LatestVersion defines a special placeholder for greatest available version
	LatestVersion = "0.0.0+latest"

	// StableVersion defines a special placeholder for the latest stable version
	StableVersion = "0.0.0+stable"
)

// locRe expression specifies the format for package name that
// consists of the repository name and a version separated by the :
var locRe = regexp.MustCompile(`^([a-zA-Z0-9\-_\.]+):(.+)$`)

func NewDigest(t string, val []byte) (*Digest, error) {
	if t != "sha512" {
		return nil, trace.Errorf("unsupported digest type: '%v'", t)
	}
	if len(val) == 0 {
		return nil, trace.Errorf("empty hash")
	}
	return &Digest{Type: t, Hash: val}, nil
}

func NewDigestFromHex(t string, val string) (*Digest, error) {
	if t != "sha512" {
		return nil, trace.Errorf("unsupported digest type: '%v'", t)
	}
	b, err := hex.DecodeString(val)
	if len(val) == 0 {
		return nil, trace.Wrap(err, "failed to decode hash string")
	}
	return &Digest{Type: t, Hash: b}, nil
}

type Digest struct {
	Type string
	Hash []byte
}

func (d Digest) Hex() string {
	return fmt.Sprintf("%x", d.Hash)
}

func (d Digest) String() string {
	return fmt.Sprintf("%x", d.Hash)
}

func NewLocator(repository, name, ver string) (*Locator, error) {
	if !cstrings.IsValidDomainName(repository) {
		return nil, trace.BadParameter(
			"repository %q has invalid format, should be valid domain name, e.g. example.com", repository)
	}
	if name == "" {
		return nil, trace.BadParameter(
			"package name %q has invalid format, should be valid identifier e.g. package-name", name)
	}
	_, err := semver.NewVersion(ver)
	if err != nil {
		return nil, trace.BadParameter(
			"unsupported version format %q, need semver format e.g 1.0.0: %v",
			ver, err)
	}
	return &Locator{Repository: repository, Name: name, Version: ver}, nil
}

// Locator is a unique package locator. It consists of the name
// and version in the form of sem ver
type Locator struct {
	Repository string `json:"repository"` // package software repository
	Name       string `json:"name"`       // example: "planet-dev"
	Version    string `json:"version"`    // example: "0.0.36"
}

// ZeroVersion returns a special 0.0.0 version of the package
func (l *Locator) ZeroVersion() Locator {
	return Locator{Repository: l.Repository, Name: l.Name, Version: ZeroVersion}
}

// SemVer obtains emver from a marshalled version
func (l *Locator) SemVer() (*semver.Version, error) {
	v, err := semver.NewVersion(l.Version)
	if err != nil {
		return nil, trace.Wrap(err,
			"unsupported version format, need semver format: %v, e.g 1.0.0", l.Version)
	}
	return v, nil
}

// EqualTo returns 'true' if this locator is equal to others
func (l Locator) IsEqualTo(other Locator) bool {
	return l.Repository == other.Repository && l.Name == other.Name && l.Version == other.Version
}

// IsEmpty returns true if locator presents empty value
func (l *Locator) IsEmpty() bool {
	return l.Repository == "" && l.Name == "" && l.Version == ""
}

// IsNewerThan returns true if this locator is of greater version than the provided locator
func (l *Locator) IsNewerThan(other Locator) (bool, error) {
	if l.Repository != other.Repository || l.Name != other.Name {
		return false, trace.BadParameter("repository/name mismatch: %v %v", l, other)
	}
	ourVer, err := l.SemVer()
	if err != nil {
		return false, trace.Wrap(err)
	}
	otherVer, err := other.SemVer()
	if err != nil {
		return false, trace.Wrap(err)
	}
	return otherVer.LessThan(*ourVer), nil
}

func (l *Locator) Set(v string) error {
	p, err := ParseLocator(v)
	if err != nil {
		return err
	}
	l.Repository = p.Repository
	l.Name = p.Name
	l.Version = p.Version
	return nil
}

// String returns the locator's string representation.
func (l Locator) String() string {
	str := l.Name
	if l.Repository != "" {
		str = fmt.Sprintf("%v/%v", l.Repository, str)
	}
	if l.Version != "" {
		str = fmt.Sprintf("%v:%v", str, l.Version)
	}
	return str
}

// WithVersion returns a copy of this locator with version set to the specified one
func (l Locator) WithVersion(version semver.Version) Locator {
	return l.WithLiteralVersion(version.String())
}

// WithLiteralVersion returns a copy of this locator with version set to the specified one
func (l Locator) WithLiteralVersion(version string) Locator {
	return Locator{
		Repository: l.Repository,
		Name:       l.Name,
		Version:    version,
	}
}

func ParseLocator(v string) (*Locator, error) {
	parts := strings.Split(v, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil, trace.Errorf(
			"package locator should be repository/name:semver, e.g. example.com/test:1.0.0, got %v", v)
	}
	m := locRe.FindAllStringSubmatch(parts[1], -1)
	if len(m) != 1 || len(m[0]) != 3 {
		return nil, trace.Errorf(
			"invalid package locator, should be repository/name:semver, e.g. example.com/test:1.0.0")
	}
	return NewLocator(parts[0], m[0][1], m[0][2])
}

// MustCreateLocator creates new locator from parts or panics
func MustCreateLocator(repo, name, ver string) Locator {
	loc, err := NewLocator(repo, name, ver)
	if err != nil {
		panic(err)
	}
	return *loc
}

func MustParseLocator(v string) Locator {
	l, err := ParseLocator(v)
	if err != nil {
		panic(err)
	}
	return *l
}

// Deduplicate returns ls with duplicates removed
func Deduplicate(ls []Locator) (result []Locator) {
	if len(ls) == 0 {
		return ls
	}
	result = make([]Locator, 0, len(ls))
	seen := make(map[Locator]struct{}, len(ls))
	for _, loc := range ls {
		if _, exists := seen[loc]; exists {
			continue
		}
		result = append(result, loc)
		seen[loc] = struct{}{}
	}
	return result
}

// GetUpdatedDependencies compares installedDeps against the updateDeps
// and returns only locators from updateDeps that are updates of those given with installedDeps
func GetUpdatedDependencies(installedDeps, updateDeps []Locator) ([]Locator, error) {
	var updates []Locator
	for _, dep := range updateDeps {
		isUpdate, err := IsUpdate(dep, installedDeps)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if !isUpdate {
			continue
		}
		updates = append(updates, dep)
	}

	return updates, nil
}

// IsPlanetPackage returns if the specified package refers to a planet package
func IsPlanetPackage(loc Locator) bool {
	return Planet.IsEqualTo(loc.ZeroVersion())
}

var (
	// OpsCenterCertificateAuthority is locator for the package containing certificate and private
	// key for the OpsCenter
	OpsCenterCertificateAuthority = MustParseLocator(
		fmt.Sprintf("%v/%v:0.0.1", defaults.SystemAccountOrg, constants.OpsCenterCAPackage))

	// Runtime is the default runtime application locator
	Runtime = MustParseLocator(
		fmt.Sprintf("%v/%v:%v", defaults.SystemAccountOrg, defaults.Runtime, LatestVersion))

	// Bandwagon is the bandwagon application locator
	Bandwagon = MustParseLocator(
		fmt.Sprintf("%v/%v:%v", defaults.SystemAccountOrg, defaults.BandwagonPackageName, LatestVersion))

	// Planet is the planet package locator
	Planet = MustParseLocator(
		fmt.Sprintf("%v/%v:%v", defaults.SystemAccountOrg, constants.PlanetPackage, ZeroVersion))

	// Teleport is the teleport package locator
	Teleport = MustParseLocator(
		fmt.Sprintf("%v/%v:%v", defaults.SystemAccountOrg, constants.TeleportPackage, ZeroVersion))

	// Gravity is the gravity binary package locator
	Gravity = MustParseLocator(
		fmt.Sprintf("%v/%v:%v", defaults.SystemAccountOrg, constants.GravityPackage, ZeroVersion))

	// Fio is the fio binary package locator
	Fio = MustParseLocator(
		fmt.Sprintf("%v/%v:%v", defaults.SystemAccountOrg, constants.FioPackage, ZeroVersion))

	// TrustedCluster is the trusted-cluster package locator
	TrustedCluster = MustParseLocator(
		fmt.Sprintf("%v/%v:0.0.1", defaults.SystemAccountOrg, constants.TrustedClusterPackage))

	// RPCSecrets defines a package with RPC agent credentials
	// that the wizard and cluster controller create.
	// The credentials are then pulled by agents that participate in installation
	// and join/leave operations.
	RPCSecrets = MustParseLocator(
		fmt.Sprintf("%v/%v:0.0.1", defaults.SystemAccountOrg, defaults.RPCAgentSecretsPackage))

	// LegacyPlanetMaster is the package locator for legacy planet master packages
	LegacyPlanetMaster = MustParseLocator(
		fmt.Sprintf("%v/%v:%v", defaults.SystemAccountOrg, "planet-master", ZeroVersion))

	// LegacyPlanetNode is the package locator for legacy planet node packages
	LegacyPlanetNode = MustParseLocator(
		fmt.Sprintf("%v/%v:%v", defaults.SystemAccountOrg, "planet-node", ZeroVersion))

	// WebAssetsPackageLocator is the package with web assets
	WebAssetsPackageLocator = MustParseLocator(
		fmt.Sprintf("%v/%v:%v", defaults.SystemAccountOrg, "web-assets", ZeroVersion))
)
