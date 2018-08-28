/*
Copyright 2017 Gravitational, Inc.

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
	"bytes"
	"context"
	"io/ioutil"
	"strings"

	"github.com/gravitational/satellite/agent/health"
	"github.com/gravitational/satellite/utils"

	"github.com/gravitational/trace"
)

// NewCGroupChecker creates a new checker to verify existence
// of cgroup mounts given with cgroups.
// The checker should be executed in the host mount namespace.
func NewCGroupChecker(cgroups ...string) health.Checker {
	return cgroupChecker{
		cgroups:   cgroups,
		getMounts: listProcMounts,
	}
}

// cgroupChecker is a checker that verifies existence
// of a set of cgroup mounts
type cgroupChecker struct {
	cgroups   []string
	getMounts mountGetterFunc
}

// Name returns name of the checker
// Implements health.Checker
func (r cgroupChecker) Name() string {
	return cgroupCheckerID
}

// Check verifies existence of cgroup mounts given in r.cgroups.
// Implements health.Checker
func (r cgroupChecker) Check(ctx context.Context, reporter health.Reporter) {
	var probes health.Probes
	err := r.check(ctx, &probes)
	if err != nil && !trace.IsNotFound(err) {
		reporter.Add(NewProbeFromErr(r.Name(), "failed to validate cgroup mounts", err))
		return
	}

	health.AddFrom(reporter, &probes)
	if probes.NumProbes() != 0 {
		return
	}

	reporter.Add(NewSuccessProbe(r.Name()))
}

func (r cgroupChecker) check(ctx context.Context, reporter health.Reporter) error {
	mounts, err := r.getMounts()
	if err != nil {
		return trace.Wrap(err, "failed to read mounts file")
	}

	expectedCgroups := utils.NewStringSetFromSlice(r.cgroups)
	for _, mount := range mounts {
		if mount.FsType == cgroupMountType {
			for _, opt := range mount.Options {
				if expectedCgroups.Has(opt) {
					expectedCgroups.Remove(opt)
				}
			}
		}
	}

	unmountedCgroups := expectedCgroups.Slice()
	if len(unmountedCgroups) > 0 {
		reporter.Add(NewProbeFromErr(r.Name(), "",
			trace.NotFound("Following CGroups have not been mounted: %q", unmountedCgroups)))
	}
	return nil
}

// listProcMounts returns the set of active mounts by interpreting
// the /proc/mounts file.
// The code is adopted from the kubernetes project.
func listProcMounts() ([]mountPoint, error) {
	content, err := consistentRead(mountFilePath, maxListTries)
	if err != nil {
		return nil, err
	}
	return parseProcMounts(content)
}

// consistentRead repeatedly reads a file until it gets the same content twice.
// This is useful when reading files in /proc that are larger than page size
// and kernel may modify them between individual read() syscalls.
func consistentRead(filename string, attempts int) ([]byte, error) {
	oldContent, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	for i := 0; i < attempts; i++ {
		newContent, err := ioutil.ReadFile(filename)
		if err != nil {
			return nil, trace.ConvertSystemError(err)
		}
		if bytes.Compare(oldContent, newContent) == 0 {
			return newContent, nil
		}
		// Files are different, continue reading
		oldContent = newContent
	}
	return nil, trace.LimitExceeded("failed to get consistent content of %v after %v attempts",
		filename, attempts)
}

func parseProcMounts(content []byte) (mounts []mountPoint, err error) {
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		if line == "" {
			// the last split() item is empty string following the last \n
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != expectedNumFieldsPerLine {
			return nil, trace.BadParameter("wrong number of fields (expected %v, got %v): %s",
				expectedNumFieldsPerLine, len(fields), line)
		}

		mount := mountPoint{
			Device:  fields[0],
			Path:    fields[1],
			FsType:  fields[2],
			Options: strings.Split(fields[3], ","),
		}

		mounts = append(mounts, mount)
	}
	return mounts, nil
}

// mountPoint desribes a mounting point as defined in /etc/mtab or /proc/mounts
// See https://linux.die.net/man/5/fstab
type mountPoint struct {
	// Device specifies the mounted device
	Device string
	// Path is the mounting path
	Path string
	// FsType defines the type of the file system
	FsType string
	// Options for the mount (read-only or read-write, etc.)
	Options []string
}

type mountGetterFunc func() ([]mountPoint, error)

// mountFilePath specifies the location of the mount information file
const mountFilePath = "/proc/mounts"

// maxListTries defines the maximum number of attempts to read file contents
const maxListTries = 3

// expectedNumFieldsPerLine specifies the number of fields per line in
// /proc/mounts as per the fstab man page.
const expectedNumFieldsPerLine = 6

// cgroupMountType specifies the filesystem type for CGroup mounts
const cgroupMountType = "cgroup"

const cgroupCheckerID = "cgroup-mounts"
