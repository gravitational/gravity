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

package mount

import (
	"os"
	"path/filepath"

	"github.com/gravitational/gravity/lib/defaults"

	"github.com/docker/docker/pkg/mount"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// RoBindMount bind-mounts the specified hostDir in
// read-only mode.
// After chroot(r.rootDir), hostDir will be available as localDir
// inside the new environment
func (r *Mounter) RoBindMount(hostDir, localDir string) error {
	dir := r.abs(localDir)
	if err := r.BindMount(hostDir, localDir); err != nil {
		return trace.Wrap(err)
	}
	if err := mount.ForceMount(hostDir, dir, "none", "remount,ro,bind"); err != nil {
		log.WithFields(log.Fields{
			log.ErrorKey: err,
			"src":        hostDir,
			"dst":        dir,
		}).Warn("Failed to remount.")
		return trace.Wrap(err, "failed to remount %v as %v (read-only)", hostDir, dir)
	}
	return nil
}

// BindMount bind-mounts the specified hostDir.
// After chroot(r.rootDir), hostDir will be available as localDir
// inside the new environment
func (r *Mounter) BindMount(hostDir, localDir string) error {
	dir := r.abs(localDir)
	err := os.MkdirAll(dir, defaults.SharedDirMask)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	if mounted, _ := mount.Mounted(dir); mounted {
		return nil
	}
	if err := mount.Mount(hostDir, dir, "none", "bind,rw"); err != nil {
		log.WithFields(log.Fields{
			log.ErrorKey: err,
			"src":        hostDir,
			"dst":        dir,
		}).Warn("Failed to mount.")
		return trace.Wrap(err, "failed to mount %v as %v", hostDir, dir)
	}
	return nil
}

// Unmount unmounts the specified directory localDir.
// localDir is assumed to be relative to r.rootDir
func (r *Mounter) Unmount(localDir string) error {
	dir := r.abs(localDir)

	if mounted, _ := mount.Mounted(dir); !mounted {
		return nil
	}

	if err := mount.Unmount(dir); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// NewMounter creates a new Mounter for the specified root directory
func NewMounter(rootDir string) *Mounter {
	return &Mounter{rootDir: rootDir}
}

// Mounter is a directory mounter
type Mounter struct {
	rootDir string
}

func (r *Mounter) abs(dir string) string {
	return filepath.Join(r.rootDir, dir)
}
