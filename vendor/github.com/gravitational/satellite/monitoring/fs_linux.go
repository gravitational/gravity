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
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"syscall"
	"unsafe"

	"github.com/gravitational/satellite/agent/health"
	pb "github.com/gravitational/satellite/agent/proto/agentpb"

	"github.com/gravitational/trace"
)

// This file implements a checker to determine if a particular filesystem has d_type attribute support.
// It is based on https://github.com/moby/moby/pull/27433

// NewDTypeChecker returns a checker that verifies that path
// is mounted on a filesystem with d_type support.
// The checker is not meant to be run periodically but once before the Docker has been set up.
//
// See https://github.com/moby/moby/blob/v1.13.0-rc4/docs/deprecated.md#backing-filesystem-without-d_type-support-for-overlayoverlay2
func NewDTypeChecker(path string) health.Checker {
	return dtypeChecker(path)
}

// Name returns name of the checker
func (r dtypeChecker) Name() string {
	return fmt.Sprintf("%v(%v)", dtypeCheckerID, r)
}

// Check determines if the filesystem mounted on r supports d_type
func (r dtypeChecker) Check(ctx context.Context, reporter health.Reporter) {
	var probes health.Probes
	if err := r.check(ctx, &probes); err != nil {
		reporter.Add(NewProbeFromErr(r.Name(),
			"failed to determine d_type support in filesystem", err))
		return
	}

	health.AddFrom(reporter, &probes)
	if probes.NumProbes() != 0 {
		return
	}

	reporter.Add(NewSuccessProbe(r.Name()))
}

func (r dtypeChecker) check(ctx context.Context, reporter health.Reporter) error {
	supports, err := supportsDType(string(r))
	if err != nil {
		return trace.Wrap(err)
	}

	if supports {
		return nil
	}
	reporter.Add(&pb.Probe{
		Checker: r.Name(),
		Detail: fmt.Sprintf("filesystem on %v does not support d_type, "+
			"see https://www.gravitational.com/gravity/docs/faq/#d_type-support-in-filesystem", string(r)),
		Status: pb.Probe_Failed,
	})
	return nil
}

type dtypeChecker string

// supportsDType returns whether the filesystem mounted on path supports d_type.
//
// The overlay and overlay2 storage drivers do not work as expected if the backing
// filesystem does not support d_type.
// For example, XFS does not support d_type if it is formatted with the ftype=0 option.
func supportsDType(path string) (bool, error) {
	// locate dummy so that we have at least one dirent
	dummy, err := locateDummyIfEmpty(path)
	if err != nil {
		return false, trace.Wrap(err)
	}
	if dummy != "" {
		defer os.Remove(dummy)
	}

	visited := 0
	supportsDType := true
	fn := func(ent *syscall.Dirent) bool {
		visited++
		if ent.Type == syscall.DT_UNKNOWN {
			supportsDType = false
			// stop iteration
			return true
		}
		// continue iteration
		return false
	}
	if err = iterateReadDir(path, fn); err != nil {
		return false, trace.Wrap(err)
	}
	if visited == 0 {
		return false, trace.NotFound("no directory entries found in %v", path)
	}
	return supportsDType, nil
}

func iterateReadDir(path string, fn func(*syscall.Dirent) bool) error {
	d, err := os.Open(path)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	defer d.Close()
	fd := int(d.Fd())
	buf := make([]byte, 4096)
	for {
		nbytes, err := syscall.ReadDirent(fd, buf)
		if err != nil {
			return trace.ConvertSystemError(err)
		}
		if nbytes == 0 {
			break
		}
		for off := 0; off < nbytes; {
			ent := (*syscall.Dirent)(unsafe.Pointer(&buf[off]))
			if stop := fn(ent); stop {
				return nil
			}
			off += int(ent.Reclen)
		}
	}
	return nil
}

// localDummyIfEmpty creates a dummy file in the directory
// specified with path if the directory is empty and
// returns the path to it.
//
// A file is required to test for d_type attribute support
func locateDummyIfEmpty(path string) (string, error) {
	children, err := ioutil.ReadDir(path)
	if err != nil {
		return "", trace.ConvertSystemError(err)
	}
	if len(children) != 0 {
		return "", nil
	}
	dummyFile, err := ioutil.TempFile(path, "dtype-dummy")
	if err != nil {
		return "", trace.ConvertSystemError(err)
	}
	name := dummyFile.Name()
	if err = dummyFile.Close(); err != nil {
		return name, trace.ConvertSystemError(err)
	}
	return name, nil
}

const dtypeCheckerID = "dtype-check"
