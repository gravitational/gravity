/*
Copyright 2020 Gravitational, Inc.
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

package mage

import (
	"fmt"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/magnet"
	"github.com/gravitational/trace"
)

var root = magnet.Root()

var (

	// golangVersion
	golangVersion = "1.13.12-stretch"

	// golangciVersion is the version of golangci-lint to use for linting
	// https://github.com/golangci/golangci-lint/releases
	golangciVersion = "v1.27.0"

	// FIO vars
	fioVersion = "3.15"
	fioTag     = fmt.Sprintf("fio-%v", fioVersion)
	fioPkgTag  = fmt.Sprintf("%v.0", fioVersion)

	// Teleport
	teleportTag     = "3.2.14"
	teleportRepoTag = fmt.Sprintf("v%s", teleportTag) // Adapts teleportTag to the teleport tagging scheme

	// Grpc
	grpcProtocVersion  = "3.10.0"
	grpcProtocPlatform = "linux-x86_64"
	grpcGoGoTag        = "v1.3.0"
	grpcGatewayTag     = "v1.11.3"

	// internal repos

	// BuildVersion
	buildVersion = magnet.E(magnet.EnvVar{
		Key:     "BUILD_VERSION",
		Default: magnet.Version(),
		Short:   "The version to assign when building artifacts",
	})

	// Planet

	// k8sVersion is the version of kubernetes we're shipping
	k8sVersion = "1.17.6"

	// planetTag is <planet version>-<encoded kubernetes version> as would be tagged in the planet repo
	// TODO: We should consider a way to import planet directly from a docker image for OSS users customizing planet
	// or add support for building off forks of the repo
	planetTag    = fmt.Sprintf("7.1.4-%v", k8sVersionToPlanetFormat(k8sVersion))
	planetBranch = magnet.E(magnet.EnvVar{
		Key:     "PLANET_BRANCH",
		Default: planetTag,
		Short:   "Alternate branch to build planet",
	})
	planetVersion = magnet.E(magnet.EnvVar{
		Key:     "PLANET_TAG",
		Default: planetTag,
		Short:   "Planet application tag/branch to build",
	})

	// Gravity Internal Applications
	appIngressVersion = magnet.E(magnet.EnvVar{
		Key:     "INGRESS_APP_VERSION",
		Default: "0.0.1",
		Short:   "Ingress application - version to assign to internal application",
	})
	appIngressBranch = magnet.E(magnet.EnvVar{
		Key:     "INGRESS_APP_BRANCH",
		Default: appIngressVersion,
		Short:   "Ingress application - tag/branch to build the application from on upstream repo",
	})
	appIngressRepo = magnet.E(magnet.EnvVar{
		Key:     "INGRESS_APP_REPO",
		Default: "https://github.com/gravitational/ingress-app",
		Short:   "Ingress application - public repository to pull the application sources from for build",
	})

	appStorageVersion = magnet.E(magnet.EnvVar{
		Key:     "STORAGE_APP_VERSION",
		Default: "0.0.1",
		Short:   "Storage application - version to assign to internal application",
	})
	appStorageBranch = magnet.E(magnet.EnvVar{
		Key:     "STORAGE_APP_BRANCH",
		Default: appStorageVersion,
		Short:   "Storage application - tag/branch to build the application from on upstream repo",
	})
	appStorageRepo = magnet.E(magnet.EnvVar{
		Key:     "STORAGE_APP_REPO",
		Default: "https://github.com/gravitational/storage-app",
		Short:   "Storage application - public repository to pull the application sources from for build",
	})

	appLoggingVersion = magnet.E(magnet.EnvVar{
		Key:     "LOGGING_APP_VERSION",
		Default: "6.0.4",
		Short:   "Logging application - version to assign to internal application",
	})
	appLoggingBranch = magnet.E(magnet.EnvVar{
		Key:     "LOGGING_APP_BRANCH",
		Default: appLoggingVersion,
		Short:   "Logging application - tag/branch to build the application from on upstream repo",
	})
	appLoggingRepo = magnet.E(magnet.EnvVar{
		Key:     "LOGGING_APP_REPO",
		Default: "https://github.com/gravitational/logging-app",
		Short:   "Storage application - public repository to pull the application sources from for build",
	})

	appMonitoringVersion = magnet.E(magnet.EnvVar{
		Key:     "MONITORING_APP_VERSION",
		Default: "7.0.1",
		Short:   "Monitoring application - version to assign to internal application",
	})
	appMonitoringBranch = magnet.E(magnet.EnvVar{
		Key:     "MONITORING_APP_BRANCH",
		Default: appMonitoringVersion,
		Short:   "Monitoring application - tag/branch to build the application from on upstream repo",
	})
	appMonitoringRepo = magnet.E(magnet.EnvVar{
		Key:     "MONITORING_APP_REPO",
		Default: "https://github.com/gravitational/monitoring-app",
		Short:   "Monitoring application - public repository to pull the application sources from for build",
	})

	appBandwagonVersion = magnet.E(magnet.EnvVar{
		Key:     "BANDWAGON_APP_TAG",
		Default: "6.0.1",
		Short:   "Bandwagon application - version to assign to internal application",
	})
	appBandwagonBranch = magnet.E(magnet.EnvVar{
		Key:     "BANDWAGON_APP_BRANCH",
		Default: appBandwagonVersion,
		Short:   "Bandwagon application - tag/branch to build the application from on upstream repo",
	})
	appBandwagonRepo = magnet.E(magnet.EnvVar{
		Key:     "BANDWAGON_APP_REPO",
		Default: "https://github.com/gravitational/bandwagon",
		Short:   "Bandwagon application - public repository to pull the application sources from for build",
	})

	// applications within the gravity master repository

	appDNSVersion = magnet.E(magnet.EnvVar{
		Key:     "DNS_APP_VERSION",
		Default: "0.4.2",
		Short:   "DNS application - version to assign to internal application",
	})
	appRBACVersion = magnet.E(magnet.EnvVar{
		Key:     "RBAC_APP_TAG",
		Default: buildVersion,
		Short:   "Logging application tag/branch to build",
	})
	appTillerVersion = magnet.E(magnet.EnvVar{
		Key:     "TILLER_APP_TAG",
		Default: "7.0.0",
		Short:   "Logging application tag/branch to build",
	})

	// Dependency Versions
	tillerVersion = magnet.E(magnet.EnvVar{
		Key:     "TILLER_VERSION",
		Default: "2.15.0",
		Short:   "Tiller version to include",
	})
	selinuxVersion = magnet.E(magnet.EnvVar{
		Key:     "SELINUX_VERSION",
		Default: "6.0.0",
		Short:   "",
	})
	selinuxBranch = magnet.E(magnet.EnvVar{
		Key:     "SELINUX_BRANCH",
		Default: "distro/centos_rhel/7",
		Short:   "",
	})
	selinuxRepo = magnet.E(magnet.EnvVar{
		Key:     "SELINUX_REPO",
		Default: "git@github.com:gravitational/selinux.git",
		Short:   "",
	})

	// which container to include for builds using wormhole networking
	wormholeImage = magnet.E(magnet.EnvVar{
		Key:     "WORMHOLE_IMG",
		Default: "quay.io/gravitational/wormhole:0.3.3",
		Short:   "ImagePath to wormhole docker container",
	})

	// Image Vulnerability Scanning on Publishing
	scanCopyToRegistry = magnet.E(magnet.EnvVar{
		Key:     "TELE_COPY_TO_REGISTRY",
		Default: "quay.io/gravitational",
		Short:   "Registry <host>/<account>to upload container to for scanning",
	})
	scanCopyToRepository = magnet.E(magnet.EnvVar{
		Key:     "TELE_COPY_TO_REPOSITORY",
		Default: "gravitational/gravity-scan",
		Short:   "The repository on the registry server to use <account>/<subrepo>",
	})
	scanCopyToPrefix = magnet.E(magnet.EnvVar{
		Key:     "TELE_COPY_TO_PREFIX",
		Default: buildVersion,
		Short:   "The prefix to add to each image name when uploading to the registry",
	})
	scanCopyToUser = magnet.E(magnet.EnvVar{
		Key:   "TELE_COPY_TO_USER",
		Short: "User to use with the registry",
	})
	scanCopyToPassword = magnet.E(magnet.EnvVar{
		Key:    "TELE_COPY_TO_PASS",
		Short:  "Password for the registry",
		Secret: true,
	})

	// Publishing
	distributionOpsCenter = magnet.E(magnet.EnvVar{
		Key:     "DISTRIBUTION_OPSCENTER",
		Default: "https://get.gravitational.io",
		Short:   "Address of OpsCenter used to publish gravity enterprise artifacts to",
	})
)

func k8sVersionToPlanetFormat(s string) string {
	version, err := semver.NewVersion(s)
	if err != nil {
		panic(trace.DebugReport(err))
	}

	return fmt.Sprintf("%d%02d%02d", version.Major, version.Minor, version.Patch)
}
