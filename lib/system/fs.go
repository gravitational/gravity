/*
Copyright 2018-2019 Gravitational, Inc.

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

package system

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// GetFilesystem detects the filesystem on device specified with path
func GetFilesystem(ctx context.Context, path string, runner utils.CommandRunner) (filesystem string, err error) {
	var out bytes.Buffer
	err = runner.RunStream(ctx, &out, "lsblk", "--noheading", "--output", "FSTYPE", path)
	if err != nil {
		return "", trace.Wrap(err, "failed to determine filesystem type on %v", path)
	}

	s := bufio.NewScanner(&out)
	s.Split(bufio.ScanLines)

	for s.Scan() {
		// Return the first line of output
		return strings.TrimSpace(s.Text()), nil
	}
	if s.Err() != nil {
		return "", trace.Wrap(err)
	}

	return "", trace.NotFound("no filesystem found for %v", path)
}

// RemoveFilesystem erases filesystem from the provided device/partition.
func RemoveFilesystem(path string, log logrus.FieldLogger) error {
	var out bytes.Buffer
	// We don't need to fill the entire disk/partition with zeroes,
	// clearing out the first 1MB should be enough to wipe out the
	// filesystem header.
	cmd := exec.Command("dd", "if=/dev/zero", fmt.Sprintf("of=%v", path), "bs=1M", "count=1")
	if err := utils.ExecL(cmd, &out, log); err != nil {
		return trace.Wrap(err, "failed to erase %v: %s", path, out.String())
	}
	return nil
}

// FormatRequest describes a request for format a device/partition.
type FormatRequest struct {
	// Path is the device/partition path.
	Path string
	// Force when true reformats even if there's already filesystem.
	Force bool
	// Log is used for logging.
	Log logrus.FieldLogger
}

// CheckAndSetDefaults validates the request and sets defaults.
func (r *FormatRequest) CheckAndSetDefaults() error {
	if r.Path == "" {
		return trace.BadParameter("device/partition path must be provided")
	}
	if r.Log == nil {
		r.Log = logrus.WithField(trace.Component, "format")
	}
	return nil
}

// String returns the request string representation.
func (r *FormatRequest) String() string {
	return fmt.Sprintf("FormatRequest(Path=%v,Force=%v)", r.Path, r.Force)
}

// FormatDevice formats block device at the specified path to either xfs or,
// if that fails, ext4, and returns the resulting filesystem.
//
// If the specified path already has filesystem, it is returned as-is and no
// formatting is attempted, unless force flag is set, in which case the path
// is reformatted anyway.
func FormatDevice(ctx context.Context, req FormatRequest) (filesystem string, err error) {
	if err := req.CheckAndSetDefaults(); err != nil {
		return "", trace.Wrap(err)
	}
	req.Log.Infof("%s.", req)

	type formatter struct {
		fsType string
		args   []string
	}
	formatters := []formatter{
		{"xfs", []string{"mkfs.xfs", "-f"}},
		{"ext4", []string{"mkfs.ext4", "-F"}},
	}

	filesystem, err = GetFilesystem(ctx, req.Path, utils.Runner)
	if err != nil {
		return "", trace.Wrap(err)
	}

	if filesystem != "" {
		if !req.Force {
			req.Log.Infof("Filesystem on %q is %v.", req.Path, filesystem)
			return filesystem, nil
		}
		req.Log.Infof("Device %q has filesystem %v, will force-reformat due to force flag.",
			req.Path, filesystem)
	}

	// format the device if the specified device does not have a file system yet
	req.Log.Infof("Device %q has no filesystem.", req.Path)

	var fmt formatter
	var out bytes.Buffer
	for _, fmt = range formatters {
		out.Reset()
		args := append(fmt.args, req.Path)
		req.Log.Debugf("Formatting %q as %v.", req.Path, fmt.fsType)
		cmd := exec.Command(args[0], args[1:]...)
		if err = utils.ExecL(cmd, &out, req.Log); err != nil {
			req.Log.Warnf("Failed to format %q as %q: %v (%v).",
				req.Path, fmt.fsType, out.String(), err)
		}
		if err == nil {
			filesystem = fmt.fsType
			break
		}
	}
	if err != nil {
		return "", trace.Wrap(err, "failed to format %q as %q: %v",
			req.Path, fmt.fsType, out.String())
	}
	return filesystem, nil
}
