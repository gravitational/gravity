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

var root = magnet.Root(magnet.Config{
	Version:     buildVersion,
	PrintConfig: true,
})

var (

	// golangVersion
	golangVersion = magnet.E(magnet.EnvVar{
		Key:   "GOLANG_VER",
		Short: "The golang version from golang container: quay.io/gravitational/debian-venti:go${GOLANG_VER}",
	})

	// golangciVersion is the version of golangci-lint to use for linting
	// https://github.com/golangci/golangci-lint/releases
	golangciVersion = "v1.27.0"

	// FIO vars
	fioVersion = magnet.E(magnet.EnvVar{
		Key:   "FIO_VER",
		Short: "The version of fio to include for volume IO testing",
	})
	fioTag    = fmt.Sprintf("fio-%v", fioVersion)
	fioPkgTag = fmt.Sprintf("%v.0", fioVersion)

	// Teleport
	teleportTag = magnet.E(magnet.EnvVar{
		Key:   "TELEPORT_TAG",
		Short: "The teleport tag to build and include with gravity",
	})
	teleportRepoTag = fmt.Sprintf("v%s", teleportTag) // Adapts teleportTag to the teleport tagging scheme

	// Grpc
	grpcProtocVersion = magnet.E(magnet.EnvVar{
		Key:   "GRPC_PROTOC_VER",
		Short: "The protoc version to use",
	})
	grpcProtocPlatform = "linux-x86_64"
	grpcGoGoTag        = magnet.E(magnet.EnvVar{
		Key:   "GOGO_PROTO_TAG",
		Short: "The grpc gogo version to use",
	})
	grpcGatewayTag = magnet.E(magnet.EnvVar{
		Key:   "GRPC_GATEWAY_TAG",
		Short: "The grpc gateway version to use",
	})

	// internal repos

	// BuildVersion
	buildVersion = magnet.E(magnet.EnvVar{
		Key:     "BUILD_VERSION",
		Default: magnet.DefaultVersion(),
		Short:   "The version to assign when building artifacts",
	})

	// Planet

	// k8sVersion is the version of kubernetes we're shipping
	k8sVersion = magnet.E(magnet.EnvVar{
		Key:   "K8S_VER",
		Short: "The k8s version to use (and locate the planet tag)",
	})

	// planetTag is <planet version>-<encoded kubernetes version> as would be tagged in the planet repo
	// TODO: We should consider a way to import planet directly from a docker image for OSS users customizing planet
	// or add support for building off forks of the repo
	//planetTag = fmt.Sprintf("7.1.4-%v", k8sVersionToPlanetFormat(k8sVersion))
	planetTag = ""

	planetBranch = magnet.E(magnet.EnvVar{
		Key: "PLANET_BRANCH",
		//Default: planetTag,
		Default: planetVersion,
		Short:   "Alternate branch to build planet",
	})
	planetVersion = magnet.E(magnet.EnvVar{
		Key:     "PLANET_TAG",
		Default: planetTag,
		Short:   "Planet application tag/branch to build",
	})

	// Gravity Internal Applications
	appIngressVersion = magnet.E(magnet.EnvVar{
		Key:   "INGRESS_APP_VERSION",
		Short: "Ingress application - version to assign to internal application",
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
		Key:   "STORAGE_APP_VERSION",
		Short: "Storage application - version to assign to internal application",
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
		Key:   "LOGGING_APP_VERSION",
		Short: "Logging application - version to assign to internal application",
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
		Key:   "MONITORING_APP_VERSION",
		Short: "Monitoring application - version to assign to internal application",
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
		Key:   "BANDWAGON_APP_TAG",
		Short: "Bandwagon application - version to assign to internal application",
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
		Key:   "DNS_APP_VERSION",
		Short: "DNS application - version to assign to internal application",
	})
	appRBACVersion = magnet.E(magnet.EnvVar{
		Key:     "RBAC_APP_TAG",
		Default: buildVersion,
		Short:   "Logging application tag/branch to build",
	})
	appTillerVersion = magnet.E(magnet.EnvVar{
		Key:   "TILLER_APP_TAG",
		Short: "Logging application tag/branch to build",
	})

	// Dependency Versions
	tillerVersion = magnet.E(magnet.EnvVar{
		Key:   "TILLER_VERSION",
		Short: "Tiller version to include",
	})
	selinuxVersion = magnet.E(magnet.EnvVar{
		Key:   "SELINUX_VERSION",
		Short: "",
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
		Key:   "WORMHOLE_IMG",
		Short: "ImagePath to wormhole docker container",
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

	// Enterprise builds
	enterprise = magnet.E(magnet.EnvVar{
		Key:   "ENTERPRISE",
		Short: "Set to enable enterprise builds",
	})
)

func k8sVersionToPlanetFormat(s string) string {
	version, err := semver.NewVersion(s)
	if err != nil {
		panic(trace.DebugReport(err))
	}

	return fmt.Sprintf("%d%02d%02d", version.Major, version.Minor, version.Patch)
}

func buildFlags() []string {
	return []string{
		fmt.Sprint(`-X github.com/gravitational/version.gitCommit=`, magnet.DefaultHash()),
		fmt.Sprint(`-X github.com/gravitational/version.version=`, buildVersion),
		fmt.Sprint(`-X github.com/gravitational/gravity/lib/defaults.WormholeImg=`, wormholeImage),
		fmt.Sprint(`-X github.com/gravitational/gravity/lib/defaults.TeleportVersionString=`, teleportTag),
		"-s -w", // shrink the binary
	}
}
