/*
Copyright 2016 Gravitational, Inc.

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

package monitoring

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"syscall"

	"github.com/gravitational/satellite/agent/health"
	pb "github.com/gravitational/satellite/agent/proto/agentpb"

	"github.com/gravitational/trace"
)

// NewBootConfigParamChecker returns a new checker that verifies
// kernel configuration options
func NewBootConfigParamChecker(params ...BootConfigParam) health.Checker {
	return &bootConfigParamChecker{
		Params:              params,
		kernelVersionReader: realKernelVersionReader,
		bootConfigReader:    realBootConfigReader,
	}
}

// bootConfigParamChecker checks whether parameters provided are specified in linux boot configuration file
type bootConfigParamChecker struct {
	// Params is array of parameters to check for
	Params []BootConfigParam
	kernelVersionReader
	bootConfigReader
}

// BootConfigParam defines parameter name (without CONFIG_ prefix) to check
type BootConfigParam struct {
	// Param is boot config parameter to check
	Name string
	// KernelConstraint specifies an optional kernel version constraint
	KernelConstraint KernelConstraintFunc
}

// Name returns name of the checker
func (c *bootConfigParamChecker) Name() string {
	return bootConfigParamID
}

// Check parses boot config files and validates whether parameters provided are set
func (c *bootConfigParamChecker) Check(ctx context.Context, reporter health.Reporter) {
	var probes health.Probes
	if err := c.check(ctx, &probes); err != nil {
		reporter.Add(NewProbeFromErr(c.Name(), "failed to validate boot configuration", err))
		return
	}

	health.AddFrom(reporter, &probes)
	if probes.NumProbes() != 0 {
		return
	}

	reporter.Add(NewSuccessProbe(bootConfigParamID))
}

// GetStorageDriverBootConfigParams returns config params required for a given filesystem
func GetStorageDriverBootConfigParams(drv string) health.Checker {
	var params []BootConfigParam

	switch drv {
	case "devicemapper":
		params = append(params,
			BootConfigParam{Name: "CONFIG_BLK_DEV_DM"},
			BootConfigParam{Name: "CONFIG_DM_THIN_PROVISIONING"},
		)
	case "overlay", "overlay2":
		params = append(params, BootConfigParam{Name: "CONFIG_OVERLAY_FS"})
	}

	return NewBootConfigParamChecker(params...)
}

// KernelConstraintFunc is a function to determine if the kernel version
// satisfies a particular condition
type KernelConstraintFunc func(KernelVersion) bool

// KernelVersionLessThan is a kernel constraint checker
// that determines if the specified testVersion is less than
// the actual version
func KernelVersionLessThan(version KernelVersion) KernelConstraintFunc {
	return func(testVersion KernelVersion) bool {
		release := testVersion.Release <= version.Release
		major := testVersion.Major <= version.Major
		minor := testVersion.Minor <= version.Minor
		patch := testVersion.Patch < version.Patch

		return release && major && minor && patch
	}
}

// KernelVersion describes an abbreviated version of a Linux kernel.
// It contains only the kernel version (including major/minor components) and
// patch number.
//
// Example:
//  $ uname -r
//  $ 4.4.9-112-generic
//
// The result will be:
//  KernelVersion{Release: 4, Major: 4, Minor: 9, Patch: 112}
type KernelVersion struct {
	// Release specifies the release of the kernel
	Release int
	// Major specifies the major version component
	Major int
	// Minor specifies the minor version component
	Minor int
	// Patch specifies the patch or build number
	Patch int
}

// String returns the kernel version formatted as Release.Major.Minor-Patch.
func (r *KernelVersion) String() string {
	return fmt.Sprintf("%d.%d.%d-%d", r.Release, r.Major, r.Minor, r.Patch)
}

// check verifies boot configuration on host.
func (c *bootConfigParamChecker) check(ctx context.Context, reporter health.Reporter) error {
	release, err := c.kernelVersionReader()
	if err != nil {
		return trace.Wrap(err, "failed to read kernel version")
	}

	kernelVersion, err := parseKernelVersion(release)
	if err != nil {
		return trace.Wrap(err, "failed to determine kernel version")
	}

	r, err := c.bootConfigReader(release)
	if trace.IsNotFound(err) {
		// Skip checks if boot configuration is not available
		return nil
	}
	if err != nil {
		return trace.Wrap(err, "failed to read boot configuration")
	}

	cfg, err := parseBootConfig(r)
	if err != nil {
		return trace.Wrap(err, "failed to parse boot configuration")
	}

	for _, param := range c.Params {
		if param.KernelConstraint != nil &&
			!param.KernelConstraint(*kernelVersion) {
			// Skip if the kernel condition is not satisfied
			continue
		}
		if _, ok := cfg[param.Name]; ok {
			continue
		}

		reporter.Add(&pb.Probe{
			Checker: bootConfigParamID,
			Detail: fmt.Sprintf("required kernel boot config parameter %s missing",
				param.Name),
			Status: pb.Probe_Failed,
		})
	}
	return nil
}

func realBootConfigReader(release string) (io.ReadCloser, error) {
	file, err := os.Open(fmt.Sprintf("/boot/config-%s", release))
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}

	return file, nil
}

func parseBootConfig(r io.ReadCloser) (config map[string]string, err error) {
	config = map[string]string{}
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		if reBootConfigComment.Match(scanner.Bytes()) || scanner.Text() == "" {
			continue
		}

		parsed := reBootConfigParam.FindStringSubmatch(scanner.Text())
		if len(parsed) != 3 {
			continue
		}

		config[parsed[1]] = parsed[2]
	}

	err = scanner.Err()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return config, nil
}

func realKernelVersionReader() (version string, err error) {
	var uname syscall.Utsname
	err = syscall.Uname(&uname)
	if err != nil {
		return "", trace.Wrap(err)
	}

	return string(int8string(uname.Release[:])), nil
}

// kernelVersionReader returns the textual kernel version
type kernelVersionReader func() (version string, err error)

// bootConfigReader reads the kernel boot configuration file
// based on the specified kernel release version
type bootConfigReader func(release string) (io.ReadCloser, error)

// parseKernelVersion parses the input string into a KernelVersion struct. The
// input is expected to be a valid Linux kernel version in the format
// Release.Major.Minor-Patch... trailing components will be ignored.
//
// Example:
//  4.4.9-112-generic
//
// The result will be:
//  KernelVersion{Release: 4, Major: 4, Minor: 9, Patch: 112}
func parseKernelVersion(input string) (*KernelVersion, error) {
	parts := strings.FieldsFunc(input, func(r rune) bool {
		return r == '.' || r == '-'
	})
	if len(parts) < 4 {
		return nil, trace.BadParameter("invalid kernel version input: %q", input)
	}
	version, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, trace.BadParameter("invalid kernel version release: %v, expected a number", input)
	}
	major, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, trace.BadParameter("invalid kernel version major: %v, expected a number", input)
	}
	minor, err := strconv.Atoi(parts[2])
	if err != nil {
		return nil, trace.BadParameter("invalid kernel version minor: %v, expected a number", input)
	}
	patch, err := strconv.Atoi(parts[3])
	if err != nil {
		return nil, trace.BadParameter("invalid kernel version patch: %v, expected a number", input)
	}

	return &KernelVersion{
		Release: version,
		Major:   major,
		Minor:   minor,
		Patch:   patch,
	}, nil
}

func int8string(bytes []int8) (result []byte) {
	result = make([]byte, 0, len(bytes))
	for _, b := range bytes {
		if b == 0 {
			break
		}
		result = append(result, byte(b))
	}
	return result
}

const bootConfigParamID = "boot-config"

var reBootConfigParam = regexp.MustCompile(`(\S+)\=([ym])`)
var reBootConfigComment = regexp.MustCompile(`#.*`)
