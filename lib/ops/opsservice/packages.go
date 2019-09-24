package opsservice

import (
	"fmt"
	"strings"
	"time"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/loc"
)

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

// planetSecretsNextPackage generates a new planet secrets package name for the specified
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

// planetConfigPackage creates a planet configuration package reference
// using the specified version as a package version and the given node to add unique
// suffix to the name.
// This is in contrast to the old naming with PackageSuffix used as a prerelease part
// of the version which made them hard to match when looking for an update.
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

// suffixer replaces characters unacceptable as a package suffix
var suffixer = strings.NewReplacer(".", "", ":", "")

func PackageSuffix(node remoteServer, domain string) string {
	data := fmt.Sprintf("%v.%v", node.Address(), domain)
	return suffixer.Replace(data)
}
