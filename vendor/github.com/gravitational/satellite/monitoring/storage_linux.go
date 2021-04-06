/*
Copyright 2017-2020 Gravitational, Inc.

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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/gravitational/satellite/agent/health"
	pb "github.com/gravitational/satellite/agent/proto/agentpb"
	"github.com/gravitational/satellite/utils"

	sigar "github.com/cloudfoundry/gosigar"
	"github.com/dustin/go-humanize"
	"github.com/gravitational/trace"
)

// NewStorageChecker creates a new instance of the volume checker
// using the specified checker as configuration
func NewStorageChecker(config StorageConfig) (health.Checker, error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &storageChecker{
		StorageConfig: config,
		osInterface:   &realOS{},
	}, nil
}

// storageChecker verifies volume requirements
type storageChecker struct {
	// Config describes the checker configuration
	StorageConfig
	// path refers to the parent directory
	// in case Path does not exist yet
	path string
	osInterface
}

const (
	storageWriteCheckerID = "io-check"
	blockSize             = 1e5
	cycles                = 1024
	stRdonly              = int64(1)
)

// Name returns name of the checker
func (c *storageChecker) Name() string {
	return fmt.Sprintf("%s(%s)", storageWriteCheckerID, c.Path)
}

func (c *storageChecker) Check(ctx context.Context, reporter health.Reporter) {
	err := c.check(ctx, reporter)
	if err != nil {
		reporter.Add(NewProbeFromErr(c.Name(),
			"failed to validate storage requirements", trace.Wrap(err)))
	}
}

func (c *storageChecker) check(ctx context.Context, reporter health.Reporter) error {
	err := c.evalPath()
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.NewAggregate(c.checkFsType(ctx, reporter),
		c.checkCapacity(ctx, reporter),
		c.checkOwner(reporter),
		c.checkDiskUsage(ctx, reporter),
		c.checkWriteSpeed(ctx, reporter))
}

// cleanPath returns fully evaluated path with symlinks resolved
// if path doesn't exist but should be created, then it returns
// first available parent directory, and checks will be applied to it
func (c *storageChecker) evalPath() error {
	p := c.Path
	for {
		fi, err := os.Stat(p)

		if err != nil && !os.IsNotExist(err) {
			return trace.ConvertSystemError(err)
		}

		if os.IsNotExist(err) && !c.WillBeCreated {
			return trace.BadParameter("%s does not exist", c.Path)
		}

		if err == nil {
			if fi.IsDir() {
				c.path = p
				return nil
			}
			return trace.BadParameter("%s is not a directory", p)
		}

		parent := filepath.Dir(p)
		if parent == p {
			return trace.BadParameter("%s is root and is not a directory", p)
		}
		p = parent
	}
}

func (c *storageChecker) checkFsType(ctx context.Context, reporter health.Reporter) error {
	if len(c.Filesystems) == 0 {
		return nil
	}

	mnt, err := fsFromPath(c.path, c.osInterface)
	if err != nil {
		return trace.Wrap(err)
	}

	probe := &pb.Probe{Checker: c.Name()}

	if utils.StringInSlice(c.Filesystems, mnt.SysTypeName) {
		probe.Status = pb.Probe_Running
	} else {
		probe.Status = pb.Probe_Failed
		probe.Detail = fmt.Sprintf("path %s requires filesystem %v, belongs to %s mount point of type %s",
			c.Path, c.Filesystems, mnt.DirName, mnt.SysTypeName)
	}
	reporter.Add(probe)
	return nil
}

// checkOwner verifies that the configured path is owned by the correct user/group.
func (c *storageChecker) checkOwner(reporter health.Reporter) error {
	if c.UID == nil && c.GID == nil {
		return nil
	}
	uid, gid, err := c.osInterface.getUIDGID(c.Path)
	if err != nil {
		return trace.Wrap(err)
	}
	if c.UID != nil {
		data := PathUIDCheckerData{
			Path:        c.Path,
			ExpectedUID: *c.UID,
			ActualUID:   uid,
		}
		bytes, err := json.Marshal(data)
		if err != nil {
			return trace.Wrap(err)
		}
		if *c.UID != uid {
			reporter.Add(&pb.Probe{
				Checker:     PathUIDCheckerID,
				CheckerData: bytes,
				Status:      pb.Probe_Failed,
				Severity:    pb.Probe_Warning,
				Detail:      data.FailureMessage(),
			})
		} else {
			reporter.Add(&pb.Probe{
				Checker:     PathUIDCheckerID,
				CheckerData: bytes,
				Status:      pb.Probe_Running,
				Detail:      data.SuccessMessage(),
			})
		}
	}
	if c.GID != nil {
		data := PathGIDCheckerData{
			Path:        c.Path,
			ExpectedGID: *c.GID,
			ActualGID:   gid,
		}
		bytes, err := json.Marshal(data)
		if err != nil {
			return trace.Wrap(err)
		}
		if *c.GID != gid {
			reporter.Add(&pb.Probe{
				Checker:     PathGIDCheckerID,
				CheckerData: bytes,
				Status:      pb.Probe_Failed,
				Severity:    pb.Probe_Warning,
				Detail:      data.FailureMessage(),
			})
		} else {
			reporter.Add(&pb.Probe{
				Checker:     PathGIDCheckerID,
				CheckerData: bytes,
				Status:      pb.Probe_Running,
				Detail:      data.SuccessMessage(),
			})
		}
	}
	return nil
}

// checkDiskUsage checks the disk usage. A warning or critical probe will be
// reported if the usage percentage is above the set thresholds.
func (c *storageChecker) checkDiskUsage(ctx context.Context, reporter health.Reporter) error {
	if c.HighWatermark == 0 {
		return nil
	}
	availableBytes, totalBytes, err := c.diskCapacity(c.path)
	if err != nil {
		return trace.Wrap(err)
	}
	if totalBytes == 0 {
		return trace.BadParameter("disk capacity at %v is 0", c.path)
	}
	checkerData := HighWatermarkCheckerData{
		LowWatermark:   c.LowWatermark,
		HighWatermark:  c.HighWatermark,
		Path:           c.Path,
		TotalBytes:     totalBytes,
		AvailableBytes: availableBytes,
	}
	checkerDataBytes, err := json.Marshal(checkerData)
	if err != nil {
		return trace.Wrap(err)
	}

	diskUsagePercent := float64(totalBytes-availableBytes) / float64(totalBytes) * 100

	if diskUsagePercent > float64(checkerData.HighWatermark) {
		reporter.Add(&pb.Probe{
			Checker:     DiskSpaceCheckerID,
			Detail:      checkerData.CriticalMessage(),
			CheckerData: checkerDataBytes,
			Status:      pb.Probe_Failed,
			Severity:    pb.Probe_Critical,
		})
		return nil
	}

	if diskUsagePercent > float64(checkerData.LowWatermark) {
		reporter.Add(&pb.Probe{
			Checker:     DiskSpaceCheckerID,
			Detail:      checkerData.WarningMessage(),
			CheckerData: checkerDataBytes,
			Status:      pb.Probe_Failed,
			Severity:    pb.Probe_Warning,
		})
		return nil
	}

	reporter.Add(&pb.Probe{
		Checker:     DiskSpaceCheckerID,
		Detail:      checkerData.SuccessMessage(),
		CheckerData: checkerDataBytes,
		Status:      pb.Probe_Running,
	})

	return nil
}

func (c *storageChecker) checkCapacity(ctx context.Context, reporter health.Reporter) error {
	if c.MinFreeBytes == 0 {
		return nil
	}

	avail, _, err := c.diskCapacity(c.path)
	if err != nil {
		return trace.Wrap(err)
	}

	if avail < c.MinFreeBytes {
		reporter.Add(&pb.Probe{
			Checker: c.Name(),
			Detail: fmt.Sprintf("%s available space left on %s, minimum of %s is required",
				humanize.Bytes(avail), c.Path, humanize.Bytes(c.MinFreeBytes)),
			Status: pb.Probe_Failed,
		})
	} else {
		reporter.Add(&pb.Probe{
			Checker: c.Name(),
			Detail: fmt.Sprintf("available disk space %s on %s satisfies minimum requirement of %s",
				humanize.Bytes(avail), c.Path, humanize.Bytes(c.MinFreeBytes)),
			Status: pb.Probe_Running,
		})
	}

	return nil
}

func (c *storageChecker) checkWriteSpeed(ctx context.Context, reporter health.Reporter) (err error) {
	if c.MinBytesPerSecond == 0 {
		return
	}

	bps, err := c.diskSpeed(ctx, c.path, "probe")
	if err != nil {
		return trace.Wrap(err)
	}

	if bps >= c.MinBytesPerSecond {
		reporter.Add(&pb.Probe{
			Checker: c.Name(),
			Detail: fmt.Sprintf("disk write speed %s/sec satisfies minumum requirement of %s",
				humanize.Bytes(bps), humanize.Bytes(c.MinBytesPerSecond)),
			Status: pb.Probe_Running,
		})
		return nil
	}

	reporter.Add(&pb.Probe{
		Checker: c.Name(),
		Detail: fmt.Sprintf("min write speed %s/sec required, have %s",
			humanize.Bytes(c.MinBytesPerSecond), humanize.Bytes(bps)),
		Status: pb.Probe_Failed,
	})
	return nil
}

type childPathFirst []sigar.FileSystem

func (a childPathFirst) Len() int           { return len(a) }
func (a childPathFirst) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a childPathFirst) Less(i, j int) bool { return strings.HasPrefix(a[i].DirName, a[j].DirName) }

func fsFromPath(path string, mountInfo mountInfo) (*sigar.FileSystem, error) {
	cleanpath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	mounts, err := mountInfo.mounts()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sort.Sort(childPathFirst(mounts))

	for _, mnt := range mounts {
		// Ignore rootfs mount to find the actual filesystem path is mounted on
		if strings.HasPrefix(cleanpath, mnt.DirName) && mnt.SysTypeName != "rootfs" {
			return &mnt, nil
		}
	}

	return nil, trace.BadParameter("failed to locate filesystem for %s", path)
}

// mounts returns the list of active mounts on the system.
// mounts implements mountInfo
func (r *realMounts) mounts() ([]sigar.FileSystem, error) {
	err := (*sigar.FileSystemList)(r).Get()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return r.List, nil
}

type realOS struct {
	realMounts
}

func (r realOS) diskSpeed(ctx context.Context, path, prefix string) (bps uint64, err error) {
	file, err := ioutil.TempFile(path, prefix)
	if err != nil {
		return 0, trace.ConvertSystemError(err)
	}
	defer file.Close()

	start := time.Now()

	buf := make([]byte, blockSize)
	err = writeN(ctx, file, buf, cycles)
	if err != nil {
		return 0, trace.ConvertSystemError(err)
	}

	if err = file.Sync(); err != nil {
		return 0, trace.ConvertSystemError(err)
	}

	elapsed := time.Since(start).Seconds()
	bps = uint64(blockSize * cycles / elapsed)

	if err = os.Remove(file.Name()); err != nil {
		return 0, trace.ConvertSystemError(err)
	}
	return bps, nil
}

func (r realOS) diskCapacity(path string) (bytesAvail, bytesTotal uint64, err error) {
	var stat syscall.Statfs_t

	err = syscall.Statfs(path, &stat)
	if err != nil {
		return 0, 0, trace.Wrap(err)
	}

	bytesAvail = uint64(stat.Bsize) * stat.Bavail
	bytesTotal = uint64(stat.Bsize) * stat.Blocks
	return bytesAvail, bytesTotal, nil
}

// getUIDGID returns the specified path's owner uid and gid.
func (r realOS) getUIDGID(path string) (uid uint32, gid uint32, err error) {
	fi, err := os.Stat(path)
	if err != nil {
		return 0, 0, trace.Wrap(err)
	}
	if fi.Sys() == nil {
		return 0, 0, trace.BadParameter("fileinfo is missing sysinfo: %v", fi)
	}
	stat, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, 0, trace.BadParameter("fileinfo unexpected sysinfo type: %[1]v %[1]T", fi.Sys())
	}
	return stat.Uid, stat.Gid, nil
}

func writeN(ctx context.Context, file *os.File, buf []byte, n int) error {
	for i := 0; i < n; i++ {
		_, err := file.Write(buf)
		if err != nil {
			return trace.ConvertSystemError(err)
		}
		if ctx.Err() != nil {
			return trace.Wrap(ctx.Err())
		}
	}
	return nil
}

type realMounts sigar.FileSystemList

type mountInfo interface {
	mounts() ([]sigar.FileSystem, error)
}

type osInterface interface {
	mountInfo
	diskSpeed(ctx context.Context, path, name string) (bps uint64, err error)
	diskCapacity(path string) (bytesAvailable, bytesTotal uint64, err error)
	getUIDGID(path string) (uid uint32, gid uint32, err error)
}
