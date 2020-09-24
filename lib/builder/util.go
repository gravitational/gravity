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

package builder

import (
	"bytes"
	"fmt"
	"net/url"
	"os/exec"
	"runtime"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/docker"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"
	"github.com/gravitational/version"
)

// checkOutputPath validates the image output path.
func checkOutputPath(manifest *schema.Manifest, outputPath string, overwrite bool) (string, error) {
	if outputPath == "" {
		outputPath = fmt.Sprintf("%v-%v.tar", manifest.Metadata.Name,
			manifest.Metadata.ResourceVersion)
	}
	_, err := utils.StatFile(outputPath)
	if err != nil && !trace.IsNotFound(err) {
		return "", trace.Wrap(err)
	}
	if err == nil && !overwrite {
		return "", trace.BadParameter(
			`Output file %v already exists.
To resolve the issue you can do one of the following:
  * Remove the existing output file.
  * Use -f/--overwrite flag to overwrite it.
  * Use -o/--output flag to specify different output file.`, outputPath)
	}
	return outputPath, nil
}

// checkBuildEnv validates the tele build environment.
func checkBuildEnv() error {
	if runtime.GOOS != "linux" {
		return trace.BadParameter(
			`Platform %v is unsupported: tele build is currently only supported on Linux.
`, runtime.GOOS)
	}
	client, err := docker.NewDefaultClient()
	if err != nil {
		return trace.Wrap(err)
	}
	dockerErr := trace.BadParameter(
		`Docker does not seem to be available on this machine.
To resolve the issue:
  * Install Docker (https://docs.docker.com/engine/installation/).
  * Make sure it can be used by a non-root user (https://docs.docker.com/install/linux/linux-postinstall/).`)

	_, err = client.Version()
	if err != nil {
		log.WithError(err).Error("failed to validate docker client connectivity")
		return dockerErr
	}

	var out bytes.Buffer
	err = utils.Exec(exec.Command("docker", "version"), &out)
	if err != nil {
		log.WithError(err).Errorf("failed to validate docker binary, output: %q", out.String())
		return dockerErr
	}

	return nil
}

// checkVersion makes sure that the provided runtime version is compatible
// with the version of this tele binary.
func checkVersion(runtimeVersion *semver.Version) error {
	teleVersion, err := semver.NewVersion(version.Get().Version)
	if err != nil {
		return trace.Wrap(err, "failed to determine tele version")
	}
	if !versionsCompatible(*teleVersion, *runtimeVersion) {
		return trace.BadParameter(
			`Version of this tele binary (%[1]v) is not compatible with the base image version specified in the manifest (%[2]v).
To resolve the issue you can do one of the following:
  * Download tele binary of the same version as the specified base image (%[2]v) and use it to build the image.
  * Specify base image version compatible with this tele in the manifest file: "baseImage: gravity:%[1]v".
  * Do not specify "baseImage" in the manifest file, in which case tele will automatically pick compatible version.`,
			teleVersion, runtimeVersion)
	}
	log.WithField("tele", teleVersion).WithField("runtime", runtimeVersion).
		Debug("Version check passed.")
	return nil
}

// versionsCompatible returns true if the provided tele and runtime versions
// are compatible.
//
// Compatibility is defined as follows:
//   1. Major and minor semver components of both versions are equal.
//   2. Runtime version is not greater than tele version.
func versionsCompatible(teleVer, runtimeVer semver.Version) bool {
	return teleVer.Major == runtimeVer.Major &&
		teleVer.Minor == runtimeVer.Minor &&
		!teleVer.LessThan(runtimeVer)
}

// ensureCacheDir makes sure that the default local cache directory for
// the provided Gravity Hub exists.
func ensureCacheDir(hubURL string) (dir string, err error) {
	u, err := url.Parse(hubURL)
	if err != nil {
		return "", trace.Wrap(err)
	}
	// cache directory is ~/.gravity/cache/<hub>/
	dir, err = utils.EnsureLocalPath("", defaults.LocalCacheDir, u.Host)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return dir, nil
}
