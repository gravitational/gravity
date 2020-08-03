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

package systeminfo

import (
	"net"
	"os"
	"path/filepath"
	"strconv"

	"github.com/gravitational/gravity/lib/devicemapper"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	sigar "github.com/cloudfoundry/gosigar"
	"github.com/gravitational/trace"
	"github.com/mitchellh/go-ps"
)

// New returns a new instance of system information
func New() (*storage.SystemV2, error) {
	spec, err := collect()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return storage.NewSystemInfo(*spec), nil
}

// FilesystemForDir returns information about the file system where the directory dirName is mounted.
// If dirName is not explicitly mounted, the function will return filesystem information about one
// of its parent directories.
func FilesystemForDir(system storage.System, dirName string) (*FilesystemInfo, error) {
	var fileSystem *storage.Filesystem
L:
	for {
		for _, fs := range system.GetFilesystems() {
			// do not take rootfs into account as it is considered always mounted on `/`
			if fs.DirName == dirName && fs.Type != "rootfs" {
				fileSystem = &fs
				break L
			}
		}
		if filepath.Dir(dirName) == dirName {
			return nil, trace.NotFound("no matching filesystem found for %v", dirName)
		}
		dirName = filepath.Dir(dirName)
	}
	var usage *storage.FilesystemUsage
	stats := system.GetFilesystemStats()
	for dir := range stats {
		dirStats := stats[dir]
		if dir == fileSystem.DirName {
			usage = &dirStats
			break
		}
	}
	if usage == nil {
		return nil, trace.NotFound("no matching filesystem found for %v", dirName)
	}
	return &FilesystemInfo{
		Filesystem: *fileSystem,
		Usage:      *usage,
	}, nil
}

// FilesystemInfo groups a filesystem and usage statistics
type FilesystemInfo struct {
	// Filesystem describes a filesystem on host
	Filesystem storage.Filesystem
	// Usage is a usage information (total, free bytes)
	Usage storage.FilesystemUsage
}

// TotalBytes reportes usage in KiB (kibibits or 1024 bits)
func (r FilesystemInfo) TotalBytes() uint64 {
	return r.Usage.TotalKB * 1024
}

// FreeBytes reportes free disk space in KiB (kibibits or 1024 bits)
func (r FilesystemInfo) FreeBytes() uint64 {
	return r.Usage.FreeKB * 1024
}

func collect() (*storage.SystemSpecV2, error) {
	var info storage.SystemSpecV2
	hostname, err := os.Hostname()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	info.Hostname = hostname

	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	info.NetworkInterfaces, err = networkInterfaces(ifaces)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	info.Filesystems, err = queryFilesystems()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	memory, err := queryMemory()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	info.Memory = *memory

	swap, err := querySwap()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	info.Swap = *swap

	info.NumCPU, err = queryCPUs()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	info.FilesystemStats, err = collectFilesystemUsage(info.Filesystems)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	osInfo, err := OSInfo()
	if err != nil {
		return nil, trace.Wrap(err, "failed to query operating system details")
	}
	info.OS = storage.OSInfo(*osInfo)

	info.Processes, err = queryProcesses()
	if err != nil {
		return nil, trace.Wrap(err, "failed to obtain running processes info")
	}

	info.Devices, err = devicemapper.GetDevices()
	if err != nil {
		return nil, trace.Wrap(err, "failed to query devices")
	}

	info.SystemPackages = GetSystemPackages(*osInfo)

	userInfo, err := GetRealUser()
	if err != nil {
		return nil, trace.Wrap(err, "failed to obtain current user info")
	}
	info.User = storage.OSUser{
		Name: userInfo.Name,
		UID:  strconv.Itoa(userInfo.UID),
		GID:  strconv.Itoa(userInfo.GID),
	}

	return &info, nil
}

func queryFilesystems() (result []storage.Filesystem, err error) {
	var list sigar.FileSystemList
	if err = list.Get(); err != nil {
		return nil, trace.Wrap(err)
	}

	for _, fs := range list.List {
		result = append(result, storage.Filesystem{
			DirName: fs.DirName,
			Type:    fs.SysTypeName,
		})
	}
	return result, nil
}

func queryMemory() (*storage.Memory, error) {
	var memory sigar.Mem
	if err := memory.Get(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &storage.Memory{Total: memory.Total}, nil
}

func querySwap() (*storage.Swap, error) {
	var swap sigar.Mem
	if err := swap.Get(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &storage.Swap{Total: swap.Total, Free: swap.Free}, nil
}

func queryCPUs() (cpus uint, err error) {
	var cpuList sigar.CpuList
	if err := cpuList.Get(); err != nil {
		return 0, trace.Wrap(err)
	}

	return uint(len(cpuList.List)), nil
}

func queryProcesses() (result []storage.Process, err error) {
	processes, err := ps.Processes()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	for _, process := range processes {
		result = append(result, storage.Process{
			Name: process.Executable(),
			PID:  process.Pid(),
		})
	}
	return result, nil
}

func collectFilesystemUsage(fs []storage.Filesystem) (result storage.FilesystemStats, err error) {
	exceptFstypes := []string{
		"sysfs",
		"devtmpfs",
		"proc",
		"devpts",
		"cgroup",
		"configfs",
		"selinuxfs",
		"debugfs",
	}
	exceptDirs := []string{
		"/sys",
		"/run",
		"/proc",
		"/dev",
	}
	result = make(map[string]storage.FilesystemUsage, len(fs))
	for _, mount := range fs {
		usage := sigar.FileSystemUsage{}
		if utils.HasOneOfPrefixes(mount.DirName, exceptDirs...) {
			continue
		}
		if utils.StringInSlice(exceptFstypes, mount.Type) {
			continue
		}
		err := usage.Get(mount.DirName)
		if err == nil {
			result[mount.DirName] = storage.FilesystemUsage{TotalKB: usage.Total, FreeKB: usage.Free}
			continue
		}
		if !os.IsNotExist(err) {
			return nil, trace.Wrap(err, "failed to get mount info on %v", mount.DirName)
		}
	}
	return result, nil
}
