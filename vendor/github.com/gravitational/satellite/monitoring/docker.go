/*
Copyright 2016 Gravitational, Inc.

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
	"io"
	"os/exec"

	humanize "github.com/dustin/go-humanize"
	"github.com/gravitational/satellite/agent/health"
	pb "github.com/gravitational/satellite/agent/proto/agentpb"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

func dockerChecker(response io.Reader) error {
	// no-op
	return nil
}

// NewDockerDevicemapperChecker returns devicemapper storage checker
func NewDockerDevicemapperChecker(config DockerDevicemapperConfig) health.Checker {
	return &devicemapperChecker{config}
}

// DockerDevicemapperConfig is the docker devicemapper checker configuration
type DockerDevicemapperConfig struct {
	// HighWatermark is the devicemapper high watermark usage
	HighWatermark uint
}

type devicemapperChecker struct {
	// DockerDevicemapperConfig is the devicemapper checker configuration
	DockerDevicemapperConfig
}

// Name returns the devicemapper checker name
func (c *devicemapperChecker) Name() string {
	return "devicemapper"
}

// Check checks devicemapper free space
func (c *devicemapperChecker) Check(ctx context.Context, reporter health.Reporter) {
	err := c.check(ctx, reporter)
	if err != nil {
		logrus.Error(trace.DebugReport(err))
		reporter.Add(NewProbeFromErr(c.Name(), "failed to check devicemapper free space",
			trace.Wrap(err)))
	}
}

func (c *devicemapperChecker) check(ctx context.Context, reporter health.Reporter) error {
	out, err := exec.Command("docker", "info", "--format", "{{json .}}").CombinedOutput()
	if err != nil {
		return trace.Wrap(err, "failed to get docker info: %s", out)
	}
	var info dockerInfo
	if err := json.Unmarshal(out, &info); err != nil {
		return trace.Wrap(err, "failed to unmarshal docker info: %s", out)
	}
	if info.Driver != "devicemapper" {
		return nil
	}
	var usedBytes, availableBytes uint64
	for _, status := range info.DriverStatus {
		switch status[0] {
		case "Data Space Used":
			if usedBytes, err = humanize.ParseBytes(status[1]); err != nil {
				return trace.Wrap(err)
			}
		case "Data Space Available":
			if availableBytes, err = humanize.ParseBytes(status[1]); err != nil {
				return trace.Wrap(err)
			}
		}
	}
	if usedBytes == 0 && availableBytes == 0 {
		return trace.BadParameter("failed to determine used docker space: %v", info)
	}
	totalBytes := usedBytes + availableBytes
	if float64(usedBytes)/float64(totalBytes)*100 > float64(c.HighWatermark) {
		reporter.Add(&pb.Probe{
			Checker: c.Name(),
			Detail: fmt.Sprintf("docker devicemapper disk utilization exceeds %v%% (%s is available out of %s), see https://gravitational.com/telekube/docs/cluster/#garbage-collection",
				c.HighWatermark, humanize.Bytes(availableBytes), humanize.Bytes(totalBytes)),
			Status: pb.Probe_Failed,
		})
	} else {
		reporter.Add(&pb.Probe{
			Checker: c.Name(),
			Detail: fmt.Sprintf("docker devicemapper disk utilization is below %v%% (%s is available out of %s)",
				c.HighWatermark, humanize.Bytes(availableBytes), humanize.Bytes(totalBytes)),
			Status: pb.Probe_Running,
		})
	}
	return nil
}

// dockerInfo represents a subset of docker info output
type dockerInfo struct {
	// Driver is the docker storage driver
	Driver string `json:"Driver"`
	// DriverStatus is the docker storage driver information
	DriverStatus [][]string `json:"DriverStatus"`
}
