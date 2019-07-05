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

// FormatDevice formats block device at the specified path to either xfs or,
// if that fails, ext4, and returns the resulting filesystem.
//
// If the specified path already has filesystem, it is returned as-is and no
// formatting is attempted.
func FormatDevice(ctx context.Context, path string, log logrus.FieldLogger) (filesystem string, err error) {
	type formatter struct {
		fsType string
		args   []string
	}
	formatters := []formatter{
		{"xfs", []string{"mkfs.xfs", "-f"}},
		{"ext4", []string{"mkfs.ext4", "-F"}},
	}

	filesystem, err = GetFilesystem(ctx, path, utils.Runner)
	if err != nil {
		return "", trace.Wrap(err)
	}

	if filesystem != "" {
		log.Infof("File system on %q is %v.", path, filesystem)
		return filesystem, nil
	}

	// format the device if the specified device does not have a file system yet
	log.Infof("Device %q has no file system.", path)

	var fmt formatter
	var out bytes.Buffer
	for _, fmt = range formatters {
		out.Reset()
		args := append(fmt.args, path)
		log.Debugf("Formatting %q as %v.", path, fmt.fsType)
		cmd := exec.Command(args[0], args[1:]...)
		if err = utils.ExecL(cmd, &out, log); err != nil {
			log.Warnf("Failed to format %q as %q: %v (%v).",
				path, fmt.fsType, out.String(), err)
		}
		if err == nil {
			filesystem = fmt.fsType
			break
		}
	}
	if err != nil {
		return "", trace.Wrap(err, "failed to format %q as %q: %v",
			path, fmt.fsType, out.String())
	}
	return filesystem, nil
}
