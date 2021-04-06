/*
Copyright 2018-2020 Gravitational, Inc.

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
	"fmt"

	"github.com/gravitational/trace"

	humanize "github.com/dustin/go-humanize"
)

// StorageConfig describes checker configuration
type StorageConfig struct {
	// Path represents volume to be checked
	Path string
	// WillBeCreated when true, then all checks will be applied to first existing dir, or fail otherwise
	WillBeCreated bool
	// MinBytesPerSecond is minimum write speed for probe to succeed
	MinBytesPerSecond uint64
	// Filesystems define list of supported filesystems, or any if empty
	Filesystems []string
	// MinFreeBytes define minimum free volume capacity
	MinFreeBytes uint64
	// LowWatermark is the disk occupancy percentage that will trigger a warning probe
	LowWatermark uint
	// HighWatermark is the disk occupancy percentage that will trigger a critical probe.
	// Disk usage check will be skipped if HighWatermark is unspecified or 0.
	HighWatermark uint
	// UID is the expected user owner of the path.
	UID *uint32
	// GID is the expected group owner of the path.
	GID *uint32
}

// CheckAndSetDefaults validates that this configuration is correct and sets
// value defaults where necessary.
func (c *StorageConfig) CheckAndSetDefaults() error {
	var errors []error
	if c.Path == "" {
		errors = append(errors, trace.BadParameter("volume path must be provided"))
	}

	if c.LowWatermark > 100 {
		errors = append(errors, trace.BadParameter("low watermark must be 0-100"))
	}

	if c.HighWatermark > 100 {
		errors = append(errors, trace.BadParameter("high watermark must be 0-100"))
	}

	if c.LowWatermark > c.HighWatermark {
		c.LowWatermark = c.HighWatermark
	}

	return trace.NewAggregate(errors...)
}

// HighWatermarkCheckerData is attached to high watermark check results
type HighWatermarkCheckerData struct {
	// LowWatermark is the low watermark percentage value
	LowWatermark uint `json:"low_watermark"`
	// HighWatermark is the high watermark percentage value
	HighWatermark uint `json:"high_watermark"`
	// Path is the absolute path to check
	Path string `json:"path"`
	// TotalBytes is the total disk capacity
	TotalBytes uint64 `json:"total_bytes"`
	// AvailableBytes is the available disk capacity
	AvailableBytes uint64 `json:"available_bytes"`
}

// WarningMessage returns warning watermark check message
func (d HighWatermarkCheckerData) WarningMessage() string {
	diskUsage := float64(d.TotalBytes-d.AvailableBytes) / float64(d.TotalBytes) * 100
	return fmt.Sprintf("disk utilization on %s exceeds %v%%, currently at %v%% (%s is available out of %s), cluster will degrade if usage exceeds %v%%, see https://gravitational.com/gravity/docs/cluster/#garbage-collection",
		d.Path, d.LowWatermark, diskUsage, humanize.Bytes(d.AvailableBytes), humanize.Bytes(d.TotalBytes), d.HighWatermark)
}

// CriticalMessage returns critical watermark check message
func (d HighWatermarkCheckerData) CriticalMessage() string {
	diskUsage := float64(d.TotalBytes-d.AvailableBytes) / float64(d.TotalBytes) * 100
	return fmt.Sprintf("disk utilization on %s exceeds %v%%, currently at %v%% (%s is available out of %s), see https://gravitational.com/gravity/docs/cluster/#garbage-collection",
		d.Path, d.HighWatermark, diskUsage, humanize.Bytes(d.AvailableBytes), humanize.Bytes(d.TotalBytes))
}

// SuccessMessage returns success watermark check message
func (d HighWatermarkCheckerData) SuccessMessage() string {
	return fmt.Sprintf("disk utilization on %s is below %v%% (%s is available out of %s)",
		d.Path, d.HighWatermark, humanize.Bytes(d.AvailableBytes), humanize.Bytes(d.TotalBytes))
}

// DiskSpaceCheckerID is the checker that checks disk space utilization
const DiskSpaceCheckerID = "disk-space"

// PathUIDCheckerData is attached to path UID check results.
type PathUIDCheckerData struct {
	// ExpectedUID is the expected path UID.
	ExpectedUID uint32 `json:"expected_uid"`
	// ActualUID is the actual path UID.
	ActualUID uint32 `json:"actual_uid"`
	// Path is the path being checked.
	Path string
}

// SuccessMessage returns success UID check message.
func (d PathUIDCheckerData) SuccessMessage() string {
	return fmt.Sprintf("path %s owner UID is %v", d.Path, d.ActualUID)
}

// FailureMessage return failure UID check message.
func (d PathUIDCheckerData) FailureMessage() string {
	return fmt.Sprintf("path %s owner UID is %v but is expected to be %v", d.Path, d.ActualUID, d.ExpectedUID)
}

// PathGIDCheckerData is attached to path GID check results.
type PathGIDCheckerData struct {
	// ExpectedGID is the expected path GID.
	ExpectedGID uint32 `json:"expected_gid"`
	// ActualGID is the actual path GID.
	ActualGID uint32 `json:"actual_gid"`
	// Path is the path being checked.
	Path string
}

// SuccessMessage returns success GID check message.
func (d PathGIDCheckerData) SuccessMessage() string {
	return fmt.Sprintf("path %s owner GID is %v", d.Path, d.ActualGID)
}

// FailureMessage return failure GID check message.
func (d PathGIDCheckerData) FailureMessage() string {
	return fmt.Sprintf("path %s owner GID is %v but is expected to be %v", d.Path, d.ActualGID, d.ExpectedGID)
}

// PathUIDCheckerID is the checker that verifies path owner UID.
const PathUIDCheckerID = "path-uid"

// PathGIDCheckerID is the checker that verifies path owner GID.
const PathGIDCheckerID = "path-gid"
