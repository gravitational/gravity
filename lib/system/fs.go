/*
Copyright 2018 Gravitational, Inc.

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
	"path/filepath"
	"strings"

	"github.com/gravitational/gravity/lib/utils"

	"github.com/docker/docker/pkg/mount"
	"github.com/gravitational/trace"
)

// GetFilesystem detects the filesystem on device specified with path
func GetFilesystem(ctx context.Context, path string, runner utils.CommandRunner) (filesystem string, err error) {
	var stdout, stderr bytes.Buffer
	err = runner.RunStream(ctx, &stdout, &stderr, "lsblk", "--noheading", "--output", "FSTYPE", path)
	if err != nil {
		return "", trace.Wrap(err, "failed to determine filesystem type on %v", path)
	}

	s := bufio.NewScanner(&stdout)
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

// GetFilesystemForPath returns the filesystem type for given path.
// It does not verify whether the path actually exists
func GetFilesystemForPath(path string) (fstype string, err error) {
	mounts, err := mount.GetMounts(mount.ParentsFilter(path))
	if err != nil {
		return "", trace.Wrap(trace.ConvertSystemError(err))
	}
	mountPoints := make(map[string]string) // map mount point to filesystem
	for _, m := range mounts {
		mountPoints[m.Mountpoint] = m.Fstype
	}
	dir := path
	for dir != "/" {
		if fstype, ok := mountPoints[dir]; ok {
			return fstype, nil
		}
		dir = filepath.Dir(dir)
	}
	if fstype, ok := mountPoints[dir]; ok {
		return fstype, nil
	}
	return "", trace.NotFound("filesystem not found for path %v", path)
}

// FilesystemTemporary defines the tmpfs filesystem
const FilesystemTemporary = "tmpfs"
