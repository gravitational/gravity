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

package schema

import (
	"context"
	"regexp"
	"strconv"
	"strings"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/satellite/agent/health"
	pb "github.com/gravitational/satellite/agent/proto/agentpb"
	"github.com/gravitational/satellite/monitoring"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

var (
	// DefaultKernelModules is the list of kernel modules needed for gravity to function properly
	DefaultKernelModules = []monitoring.ModuleRequest{
		moduleName("ebtables"),
		moduleName("ebtable_filter"),
		moduleName("ip_tables"),
		moduleName("iptable_filter"),
		moduleName("iptable_nat"),
		moduleName("br_netfilter"),
		moduleName("overlay"),
		// TODO(knisbet) adding new modules to this list will break upgrades, so disable checking for the dummy module
		// until upgrades will update the module list
		//moduleName("dummy"),
	}

	// DefaultKernelModuleChecker is a satellite kernel module checker with required modules to run kubernetes
	DefaultKernelModuleChecker = monitoring.NewKernelModuleChecker(DefaultKernelModules...)
)

// ValidateDocker validates Docker requirements.
// The specified directory is expected to be on the same filesystem
// as the Docker graph directory (which might not exist at this point).
func ValidateDocker(ctx context.Context, d Docker, dir string) (failed []*pb.Probe, err error) {
	var checkers []health.Checker

	checkers = append(checkers,
		monitoring.GetStorageDriverBootConfigParams(d.StorageDriver))

	switch d.StorageDriver {
	case constants.DockerStorageDriverOverlay, constants.DockerStorageDriverOverlay2:
		checkers = append(checkers,
			monitoring.NewKernelModuleChecker(moduleName("overlay")),
			monitoring.NewDTypeChecker(dir),
		)
	}

	all := monitoring.NewCompositeChecker("docker", checkers)
	var probes health.Probes

	all.Check(ctx, &probes)
	return probes.GetFailed(), nil
}

// ValidateKubelet will check kubelet configuration
func ValidateKubelet(ctx context.Context, profile NodeProfile, manifest Manifest) (failed []*pb.Probe) {
	checkers := append([]health.Checker{},
		DefaultKernelModuleChecker,
		monitoring.NewCGroupChecker("cpu", "cpuacct", "cpuset", "memory"),
	)
	checker := monitoring.NewCompositeChecker("kubelet", checkers)

	var probes health.Probes
	checker.Check(ctx, &probes)
	return probes.GetFailed()
}

// ValidateRequirements will assess local node to match requirements
func ValidateRequirements(ctx context.Context, reqs Requirements, stateDir string) (failed []*pb.Probe, err error) {
	var checkers []health.Checker
	checkers = append(checkers, monitoring.NewHostChecker(
		monitoring.HostConfig{
			MinCPU:      reqs.CPU.Min,
			MinRAMBytes: reqs.RAM.Min.Bytes(),
		}),
	)

	var releases []monitoring.OSRelease
	for _, os := range reqs.OS {
		for _, version := range os.Versions {
			releases = append(releases, monitoring.OSRelease{
				ID:        os.Name,
				VersionID: version,
			})
		}
	}
	if len(releases) > 0 {
		checkers = append(checkers, monitoring.NewOSChecker(releases...))
	}

	var portRanges = make([]monitoring.PortRange, 0, len(reqs.Network.Ports))
	for _, port := range reqs.Network.Ports {
		portRange, err := parsePortRanges(port.Protocol, port.Ranges)
		if err != nil {
			return nil, trace.Wrap(err, "invalid port range in %+v", port)
		}
		portRanges = append(portRanges, portRange...)
	}
	checkers = append(checkers, monitoring.NewPortChecker(portRanges...))

	for _, vol := range reqs.Volumes {
		if vol.Path == defaults.GravityDir {
			// Use the correct system directory in the test
			vol.Path = stateDir
		}
		if !shouldCheckVolume(vol) {
			log.Debugf("Skip check for %v -> %v mount.", vol.Path, vol.TargetPath)
			continue
		}
		storageChecker, err := monitoring.NewStorageChecker(
			monitoring.StorageConfig{
				Path:              vol.Path,
				MinBytesPerSecond: vol.MinTransferRate.BytesPerSecond(),
				WillBeCreated:     utils.BoolValue(vol.CreateIfMissing),
				Filesystems:       vol.Filesystems,
				MinFreeBytes:      vol.Capacity.Bytes(),
			},
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		checkers = append(checkers, storageChecker)
	}

	for _, check := range reqs.CustomChecks {
		checkers = append(checkers,
			monitoring.NewScriptChecker(monitoring.Script{
				Reader:      strings.NewReader(check.Script),
				Description: check.Description,
			}))
	}

	all := monitoring.NewCompositeChecker("common requirements", checkers)
	var probes health.Probes

	all.Check(ctx, &probes)
	return probes.GetFailed(), nil
}

// shouldCheckVolume determines if this volume should be checked
func shouldCheckVolume(volume Volume) bool {
	isDir, err := utils.IsDirectory(volume.Path)
	isFileVolume := err == nil && !isDir
	if isFileVolume {
		// Do not check file mounts
		return false
	}
	if !utils.BoolValue(volume.SkipIfMissing) {
		// Check this volume
		return true
	}
	// Only check this volume if refers to an existing path
	return !trace.IsNotFound(err)
}

var reNumRange = regexp.MustCompile(`(?P<from>\d+)\-(?P<to>\d+)`)

func parsePortRanges(proto string, ranges []string) (res []monitoring.PortRange, err error) {
	for _, p := range ranges {
		r, err := parsePortRange(proto, p)
		if err == nil {
			res = append(res, *r)
			continue
		}

		port, err := strconv.ParseUint(p, 10, 64)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		res = append(res, monitoring.PortRange{
			Protocol: proto, From: uint64(port), To: uint64(port)})
	}
	return res, nil
}

func parsePortRange(proto, p string) (*monitoring.PortRange, error) {
	parsed := reNumRange.FindStringSubmatch(p)
	if len(parsed) != 3 {
		return nil, trace.BadParameter(p)
	}

	from, err := strconv.ParseUint(parsed[1], 10, 64)
	if err != nil {
		return nil, trace.Wrap(err, p)
	}

	to, err := strconv.ParseUint(parsed[2], 10, 64)
	if err != nil {
		return nil, trace.Wrap(err, p)
	}

	return &monitoring.PortRange{
		Protocol: proto,
		From:     uint64(from),
		To:       uint64(to),
	}, nil
}

func moduleName(name string, names ...string) monitoring.ModuleRequest {
	return monitoring.ModuleRequest{Name: name, Names: names}
}
