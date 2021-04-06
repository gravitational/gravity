/*
Copyright 2019 Gravitational, Inc.

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

package opsservice

import (
	"fmt"
	"strings"
	"time"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
)

// PlanetCertAuthorityPackage returns the name of the planet CA package
func PlanetCertAuthorityPackage(repository string) loc.Locator {
	return loc.Locator{
		Repository: repository,
		Name:       constants.CertAuthorityPackage,
		Version:    loc.FirstVersion,
	}
}

// planetSecretsNextPackage generates a new planet secrets package name for the specified
// node and planet package version.
//
// The package is named as '<cluster-name>/planet-<node-addr>-secrets:<planet-version>(+<increment>)?'
// where increment is an ever-increasing counter and node-addr is a combination of the node address
// and cluster name
func (s *site) planetSecretsNextPackage(node *ProvisionedServer, planetVersion string) loc.Locator {
	planetVersion = fmt.Sprintf("%v+%v", planetVersion, time.Now().UTC().Unix())
	return planetSecretsPackage(node, s.domainName, s.domainName, planetVersion)
}

// planetSecretsPackage generates a new planet secrets package name for the specified
// node and planet package version.
//
// The package is named as '<cluster-name>/planet-<node-addr>-secrets:<planet-version>'
// where node-addr is a combination of the node address and cluster name
func (s *site) planetSecretsPackage(node *ProvisionedServer, planetVersion string) loc.Locator {
	return planetSecretsPackage(node, s.domainName, s.domainName, planetVersion)
}

// planetNextConfigPackage generates a new planet configuration package name for the specified
// node and planet package version.
//
// The package is named as '<cluster-name>/planet-config-<node-addr>:<planet-version>(+<increment>)?'
// where increment is an ever-increasing counter and node-addr is a combination of the node address
// and cluster name
func (s *site) planetNextConfigPackage(node remoteServer, planetVersion string) loc.Locator {
	planetVersion = fmt.Sprintf("%v+%v", planetVersion, time.Now().UTC().Unix())
	return planetConfigPackage(node, s.domainName, s.domainName, planetVersion)
}

// planetConfigPackage generates a planet configuration package name for the specified
// node and planet package version.
//
// The package is named as '<cluster-name>/planet-config-<node-addr>:<planet-version>'
// where node-addr is a combination of the node address and cluster name
func (s *site) planetConfigPackage(node remoteServer, planetVersion string) loc.Locator {
	return planetConfigPackage(node, s.domainName, s.domainName, planetVersion)
}

// teleportNextNodeConfigPackage generates a new teleport configuration package name for the specified
// node and planet package version.
//
// The package is named as '<cluster-name>/teleport-node-config-<node-addr>:<teleport-version>(+<increment>)?'
// where increment is an ever-increasing counter and node-addr is a combination of the node address
// and cluster name
func (s *site) teleportNextNodeConfigPackage(node remoteServer, teleportVersion string) loc.Locator {
	teleportVersion = fmt.Sprintf("%v+%v", teleportVersion, time.Now().UTC().Unix())
	return teleportNodeConfigPackage(node, s.domainName, s.domainName, teleportVersion)
}

// teleportNodeConfigPackage generates a new teleport configuration package name for the specified
// node and planet package version.
//
// The package is named as '<cluster-name>/teleport-node-config-<node-addr>:<teleport-version>'
// where node-addr is a combination of the node address and cluster name
func (s *site) teleportNodeConfigPackage(node remoteServer) loc.Locator {
	return teleportNodeConfigPackage(node, s.domainName, s.domainName, s.teleportPackage.Version)
}

// teleportNextMasterConfigPackage generates a new teleport configuration package name for the specified
// node and planet package version.
//
// The package is named as '<cluster-name>/teleport-master-config-<node-addr>:<teleport-version>(+<increment>)?'
// where increment is an ever-increasing counter and node-addr is a combination of the node address
// and cluster name
func (s *site) teleportNextMasterConfigPackage(master remoteServer, teleportVersion string) loc.Locator {
	teleportVersion = fmt.Sprintf("%v+%v", teleportVersion, time.Now().UTC().Unix())
	return teleportMasterConfigPackage(master, s.domainName, s.domainName, teleportVersion)
}

// teleportMasterConfigPackage generates a new teleport configuration package name for the specified
// node and planet package version.
//
// The package is named as '<cluster-name>/teleport-master-config-<node-addr>:<teleport-version>'
// where node-addr is a combination of the node address and cluster name
func (s *site) teleportMasterConfigPackage(master remoteServer) loc.Locator {
	return teleportMasterConfigPackage(master, s.domainName, s.domainName, s.teleportPackage.Version)
}

func teleportMasterConfigPackage(master remoteServer, repository, clusterName, teleportVersion string) loc.Locator {
	return loc.Locator{
		Repository: repository,
		Name: fmt.Sprintf("%v-%v",
			constants.TeleportMasterConfigPackage,
			PackageSuffix(master, clusterName),
		),
		Version: teleportVersion,
	}
}

func planetSecretsPackage(node *ProvisionedServer, repository, clusterName, planetVersion string) loc.Locator {
	return loc.Locator{
		Repository: repository,
		Name:       fmt.Sprintf("planet-%v-secrets", node.AdvertiseIP),
		Version:    planetVersion,
	}
}

func planetConfigPackage(node remoteServer, repository, clusterName, planetVersion string) loc.Locator {
	return loc.Locator{
		Repository: repository,
		Name: fmt.Sprintf("%v-%v",
			constants.PlanetConfigPackage,
			PackageSuffix(node, clusterName)),
		Version: planetVersion,
	}
}

func teleportNodeConfigPackage(node remoteServer, repository, clusterName, teleportVersion string) loc.Locator {
	return loc.Locator{
		Repository: repository,
		Name: fmt.Sprintf("%v-%v",
			constants.TeleportNodeConfigPackage,
			PackageSuffix(node, clusterName),
		),
		Version: teleportVersion,
	}
}

func (s *site) planetCertAuthorityPackage() loc.Locator {
	return PlanetCertAuthorityPackage(s.siteRepoName())
}

// opsCertAuthorityPackage is a shorthand to return locator for OpsCenter's certificate
// authority package
func (s *site) opsCertAuthorityPackage() loc.Locator {
	return loc.Locator{
		Repository: defaults.SystemAccountOrg,
		Name:       constants.OpsCenterCAPackage,
		Version:    loc.FirstVersion,
	}
}

// siteExport package exports site state as BoltDB database dump
func (s *site) siteExportPackage() loc.Locator {
	return loc.Locator{
		Repository: s.siteRepoName(),
		Name:       constants.SiteExportPackage,
		Version:    loc.FirstVersion,
	}
}

func (s *site) licensePackage() loc.Locator {
	return loc.Locator{
		Repository: s.siteRepoName(),
		Name:       constants.LicensePackage,
		Version:    loc.FirstVersion,
	}
}

// suffixer replaces characters unacceptable as a package suffix
var suffixer = strings.NewReplacer(".", "", ":", "")

// PackageSuffix returns a new package suffix used in package names
// from the specified node address and given cluster name
func PackageSuffix(node remoteServer, clusterName string) string {
	data := fmt.Sprintf("%v.%v", node.Address(), clusterName)
	return suffixer.Replace(data)
}
