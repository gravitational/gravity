/*
Copyright 2021 Gravitational, Inc.

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
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gravitational/magnet"
	"github.com/gravitational/trace"

	"github.com/coreos/go-semver/semver"
	log "github.com/sirupsen/logrus"
)

var env = importEnvFromMakefile()

var root = mustRoot(magnet.Config{
	Version:       env.getBuildVersion(),
	PrintConfig:   true,
	LogDir:        fmt.Sprintf("%s/logs", env.getBuildDir()),
	CacheDir:      fmt.Sprintf("%s/cache", env.getBuildDir()),
	ImportEnv:     env,
	PlainProgress: env.isPlainProgress(),
}, env.getBuildDir())

var (

	// Go version
	golangVersion = root.E(magnet.EnvVar{
		Key:   "GOLANG_VER",
		Short: "The version of Go from container: quay.io/gravitational/debian-venti:go${GOLANG_VER}",
	})

	// golangciVersion is the version of golangci-lint to use for linting
	// https://github.com/golangci/golangci-lint/releases
	golangciVersion = "v1.39.0"

	// FIO vars
	fioVersion = root.E(magnet.EnvVar{
		Key:   "FIO_VER",
		Short: "The version of fio to include for volume IO testing",
	})
	fioTag    = fmt.Sprintf("fio-%v", fioVersion)
	fioPkgTag = fmt.Sprintf("%v.0", fioVersion)

	// Teleport
	teleportTag = root.E(magnet.EnvVar{
		Key:   "TELEPORT_TAG",
		Short: "The teleport tag to build and include with gravity",
	})
	teleportRepoTag = fmt.Sprintf("v%s", teleportTag) // Adapts teleportTag to the teleport tagging scheme

	// Grpc
	grpcProtocVersion = root.E(magnet.EnvVar{
		Key:   "GRPC_PROTOC_VER",
		Short: "The protoc version to use",
	})
	grpcProtocPlatform = "linux-x86_64"
	grpcGoGoTag        = root.E(magnet.EnvVar{
		Key:   "GOGO_PROTO_TAG",
		Short: "The grpc gogo version to use",
	})
	grpcGatewayTag = root.E(magnet.EnvVar{
		Key:   "GRPC_GATEWAY_TAG",
		Short: "The grpc gateway version to use",
	})

	// internal repos

	// BuildVersion
	buildVersion = root.E(magnet.EnvVar{
		Key:     "BUILD_VERSION",
		Default: magnet.DefaultVersion(),
		Short:   "The version to assign when building artifacts",
	})

	// Planet

	// k8sVersion is the version of kubernetes we're shipping
	k8sVersion = root.E(magnet.EnvVar{
		Key:   "K8S_VER",
		Short: "The k8s version to use (and locate the planet tag)",
	})

	// planetTag is <planet version>-<encoded kubernetes version> as would be tagged in the planet repo
	// TODO: We should consider a way to import planet directly from a docker image for users customizing planet
	// or add support for building off forks of the repo
	//planetTag = fmt.Sprintf("7.1.4-%v", k8sVersionToPlanetFormat(k8sVersion))
	planetTag = ""

	planetBranch = root.E(magnet.EnvVar{
		Key:     "PLANET_BRANCH",
		Default: planetTag,
		Short:   "Alternate branch to build planet",
	})
	planetVersion = root.E(magnet.EnvVar{
		Key:     "PLANET_TAG",
		Default: planetTag,
		Short:   "Planet application tag/branch to build",
	})

	// Gravity Internal Applications
	appIngressVersion = root.E(magnet.EnvVar{
		Key:   "INGRESS_APP_VERSION",
		Short: "Ingress application - version to assign to internal application",
	})
	appIngressBranch = root.E(magnet.EnvVar{
		Key:     "INGRESS_APP_BRANCH",
		Default: appIngressVersion,
		Short:   "Ingress application - tag/branch to build the application from on upstream repo",
	})
	appIngressRepo = root.E(magnet.EnvVar{
		Key:     "INGRESS_APP_REPO",
		Default: "https://github.com/gravitational/ingress-app",
		Short:   "Ingress application - public repository to pull the application sources from for build",
	})

	appStorageVersion = root.E(magnet.EnvVar{
		Key:   "STORAGE_APP_VERSION",
		Short: "Storage application - version to assign to internal application",
	})
	appStorageBranch = root.E(magnet.EnvVar{
		Key:     "STORAGE_APP_BRANCH",
		Default: appStorageVersion,
		Short:   "Storage application - tag/branch to build the application from on upstream repo",
	})
	appStorageRepo = root.E(magnet.EnvVar{
		Key:     "STORAGE_APP_REPO",
		Default: "https://github.com/gravitational/storage-app",
		Short:   "Storage application - public repository to pull the application sources from for build",
	})

	appLoggingVersion = root.E(magnet.EnvVar{
		Key:   "LOGGING_APP_VERSION",
		Short: "Logging application - version to assign to internal application",
	})
	appLoggingBranch = root.E(magnet.EnvVar{
		Key:     "LOGGING_APP_BRANCH",
		Default: appLoggingVersion,
		Short:   "Logging application - tag/branch to build the application from on upstream repo",
	})
	appLoggingRepo = root.E(magnet.EnvVar{
		Key:     "LOGGING_APP_REPO",
		Default: "https://github.com/gravitational/logging-app",
		Short:   "Storage application - public repository to pull the application sources from for build",
	})

	appMonitoringVersion = root.E(magnet.EnvVar{
		Key:   "MONITORING_APP_VERSION",
		Short: "Monitoring application - version to assign to internal application",
	})
	appMonitoringBranch = root.E(magnet.EnvVar{
		Key:     "MONITORING_APP_BRANCH",
		Default: appMonitoringVersion,
		Short:   "Monitoring application - tag/branch to build the application from on upstream repo",
	})
	appMonitoringRepo = root.E(magnet.EnvVar{
		Key:     "MONITORING_APP_REPO",
		Default: "https://github.com/gravitational/monitoring-app",
		Short:   "Monitoring application - public repository to pull the application sources from for build",
	})

	appBandwagonVersion = root.E(magnet.EnvVar{
		Key:   "BANDWAGON_APP_TAG",
		Short: "Bandwagon application - version to assign to internal application",
	})
	appBandwagonBranch = root.E(magnet.EnvVar{
		Key:     "BANDWAGON_APP_BRANCH",
		Default: appBandwagonVersion,
		Short:   "Bandwagon application - tag/branch to build the application from on upstream repo",
	})
	appBandwagonRepo = root.E(magnet.EnvVar{
		Key:     "BANDWAGON_APP_REPO",
		Default: "https://github.com/gravitational/bandwagon",
		Short:   "Bandwagon application - public repository to pull the application sources from for build",
	})

	// applications within the gravity master repository

	appDNSVersion = root.E(magnet.EnvVar{
		Key:   "DNS_APP_VERSION",
		Short: "DNS application - version to assign to internal application",
	})
	appRBACVersion = root.E(magnet.EnvVar{
		Key:     "RBAC_APP_TAG",
		Default: buildVersion,
		Short:   "Logging application tag/branch to build",
	})
	appTillerVersion = root.E(magnet.EnvVar{
		Key:   "TILLER_APP_TAG",
		Short: "Logging application tag/branch to build",
	})

	// Dependency Versions
	tillerVersion = root.E(magnet.EnvVar{
		Key:   "TILLER_VERSION",
		Short: "Tiller version to include",
	})
	selinuxVersion = root.E(magnet.EnvVar{
		Key:   "SELINUX_VERSION",
		Short: "",
	})
	selinuxBranch = root.E(magnet.EnvVar{
		Key:     "SELINUX_BRANCH",
		Default: "distro/centos_rhel/7",
		Short:   "",
	})
	selinuxRepo = root.E(magnet.EnvVar{
		Key:     "SELINUX_REPO",
		Default: "git@github.com:gravitational/selinux.git",
		Short:   "",
	})

	// which container to include for builds using wormhole networking
	wormholeImage = root.E(magnet.EnvVar{
		Key:   "WORMHOLE_IMG",
		Short: "ImagePath to wormhole docker container",
	})

	// Image Vulnerability Scanning on Publishing
	scanCopyToRegistry = root.E(magnet.EnvVar{
		Key:     "TELE_COPY_TO_REGISTRY",
		Default: "quay.io/gravitational",
		Short:   "Registry <host>/<account>to upload container to for scanning",
	})
	scanCopyToRepository = root.E(magnet.EnvVar{
		Key:     "TELE_COPY_TO_REPOSITORY",
		Default: "gravitational/gravity-scan",
		Short:   "The repository on the registry server to use <account>/<subrepo>",
	})
	scanCopyToPrefix = root.E(magnet.EnvVar{
		Key:     "TELE_COPY_TO_PREFIX",
		Default: buildVersion,
		Short:   "The prefix to add to each image name when uploading to the registry",
	})
	scanCopyToUser = root.E(magnet.EnvVar{
		Key:   "TELE_COPY_TO_USER",
		Short: "User to use with the registry",
	})
	scanCopyToPassword = root.E(magnet.EnvVar{
		Key:    "TELE_COPY_TO_PASS",
		Short:  "Password for the registry",
		Secret: true,
	})

	// Publishing
	distributionOpsCenter = root.E(magnet.EnvVar{
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

func buildFlags() []string {
	return []string{
		fmt.Sprint(`-X github.com/gravitational/version.gitCommit=`, magnet.DefaultHash()),
		fmt.Sprint(`-X github.com/gravitational/version.version=`, buildVersion),
		fmt.Sprint(`-X github.com/gravitational/gravity/lib/defaults.WormholeImg=`, wormholeImage),
		fmt.Sprint(`-X github.com/gravitational/gravity/lib/defaults.TeleportVersionString=`, teleportTag),
		"-s -w", // shrink the binary
	}
}

func importEnvFromMakefile() (env environ) {
	env = make(environ)
	cmd := exec.Command("make", "-f", "Makefile.buildx", "magnet-vars")
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.WithError(err).Warn("Failed to import environ from makefile.")
		return nil
	}
	for _, line := range strings.Split(string(out), "\n") {
		cols := strings.SplitN(line, "=", 2)
		if len(cols) != 2 || !strings.HasPrefix(cols[0], "MAGNET_") {
			log.Debug("Skip line that does not look like magnet envar.")
			continue
		}
		key, value := strings.TrimPrefix(cols[0], "MAGNET_"), cols[1]
		env[key] = value
	}
	return env
}

func (r environ) getBuildDir() string {
	return r["BUILDDIR"]
}

func (r environ) getBuildVersion() string {
	return r["BUILD_VERSION"]
}

func (r environ) isPlainProgress() bool {
	return r["CI"] != ""
}

type environ map[string]string

func mustRoot(config magnet.Config, buildDir string) *rootTarget {
	root, err := magnet.Root(config)
	if err != nil {
		panic(err.Error())
	}
	return &rootTarget{
		Magnet:   root,
		buildDir: buildDir,
	}
}

func (r *rootTarget) inVersionedContainerBuildDir(elems ...string) (dir string) {
	return r.inContainerBuildDir(append([]string{r.Magnet.Version}, elems...)...)
}

func (r *rootTarget) inVersionedBuildDir(elems ...string) (dir string) {
	return r.inBuildDir(append([]string{r.Magnet.Version}, elems...)...)
}

func (r *rootTarget) inContainerBuildDir(elems ...string) (dir string) {
	baseDir := filepath.Base(r.buildDir)
	path := append([]string{"/host"}, baseDir)
	return filepath.Join(append(path, elems...)...)
}

func (r *rootTarget) inBuildDir(elems ...string) (dir string) {
	return filepath.Join(append([]string{r.buildDir}, elems...)...)
}

type rootTarget struct {
	*magnet.Magnet
	// buildDir specifies the absolute build directory
	buildDir string
}
