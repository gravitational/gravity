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

package monitoring

import (
	"fmt"

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
	// HighWatermark is the disk occupancy percentage that is considered degrading
	HighWatermark uint
}

// HighWatermarkCheckerData is attached to high watermark check results
type HighWatermarkCheckerData struct {
	// HighWatermark is the watermark percentage value
	HighWatermark uint `json:"high_watermark"`
	// Path is the absolute path to check
	Path string `json:"path"`
	// TotalBytes is the total disk capacity
	TotalBytes uint64 `json:"total_bytes"`
	// AvailableBytes is the available disk capacity
	AvailableBytes uint64 `json:"available_bytes"`
}

// FailureMessage returns failure watermark check message
func (d HighWatermarkCheckerData) FailureMessage() string {
	return fmt.Sprintf("disk utilization on %s exceeds %v percent (%s is available out of %s), see https://gravitational.com/telekube/docs/cluster/#garbage-collection",
		d.Path, d.HighWatermark, humanize.Bytes(d.AvailableBytes), humanize.Bytes(d.TotalBytes))
}

// SuccessMessage returns success watermark check message
func (d HighWatermarkCheckerData) SuccessMessage() string {
	return fmt.Sprintf("disk utilization on %s is below %v percent (%s is available out of %s)",
		d.Path, d.HighWatermark, humanize.Bytes(d.AvailableBytes), humanize.Bytes(d.TotalBytes))
}

// DiskSpaceCheckerID is the checker that checks disk space utilization
const DiskSpaceCheckerID = "disk-space"
