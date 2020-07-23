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

package monitoring

import (
	"fmt"
	"strconv"
	"strings"
	"syscall"

	"github.com/gravitational/trace"
)

// KernelVersion describes an abbreviated version of a Linux kernel.
// It contains the kernel version (including major/minor components) and
// patch number
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

// KernelConstraintFunc is a function to determine if the kernel version
// satisfies a particular condition.
type KernelConstraintFunc func(KernelVersion) bool

// KernelVersionLessThan is a kernel constraint checker
// that determines if the specified testVersion is less than
// the actual version.
func KernelVersionLessThan(version KernelVersion) KernelConstraintFunc {
	return func(testVersion KernelVersion) bool {
		if testVersion.Release != version.Release {
			return testVersion.Release < version.Release
		}

		if testVersion.Major != version.Major {
			return testVersion.Major < version.Major
		}

		if testVersion.Minor != version.Minor {
			return testVersion.Minor < version.Minor
		}

		return testVersion.Patch < version.Patch
	}
}

// kernelVersionReader returns the textual kernel version.
type kernelVersionReader func() (version string, err error)

// realKernelVersionReader reads and returns the currently installed Linux
// kernel version.
func realKernelVersionReader() (version string, err error) {
	var uname syscall.Utsname
	if err := syscall.Uname(&uname); err != nil {
		return "", trace.Wrap(err)
	}

	return int8string(uname.Release[:]), nil
}

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
	// componentsLength defines the expected number of kernel version components.
	const componentsLength = 4

	parts := strings.FieldsFunc(input, func(r rune) bool {
		return r == '.' || r == '-'
	})
	if len(parts) < componentsLength {
		return nil, trace.BadParameter("invalid kernel version input: %q", input)
	}

	version, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, trace.BadParameter(
			"invalid kernel version: %v, expected a number",
			input)
	}

	major, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, trace.BadParameter(
			"invalid kernel version major: %v, expected a number",
			input)
	}

	minor, err := strconv.Atoi(parts[2])
	if err != nil {
		return nil, trace.BadParameter(
			"invalid kernel version minor: %v, expected a number",
			input)
	}

	patch, err := strconv.Atoi(parts[3])
	if err != nil {
		return nil, trace.BadParameter(
			"invalid kernel version patch: %v, expected a number",
			input)
	}

	return &KernelVersion{
		Release: version,
		Major:   major,
		Minor:   minor,
		Patch:   patch,
	}, nil
}

// int8string converts the bytes into a string.
func int8string(bytes []int8) string {
	result := make([]byte, 0, len(bytes))

	for _, b := range bytes {
		if b == 0 {
			break
		}

		result = append(result, byte(b))
	}

	return string(result)
}
