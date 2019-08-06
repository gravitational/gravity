/*
Copyright 2019 Gravitational, Inc.

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

package disks

// FioResult represents a subset of fio test output in json format.
type FioResult struct {
	// Jobs is a list of executed jobs.
	Jobs []FioJobResult `json:"jobs"`
}

// FioJobResult represents a result of a single fio job.
type FioJobResult struct {
	// JobName is the name of the job.
	JobName string `json:"jobname"`
	// Read contains metrics related to performed reads.
	Read FioReadResult `json:"read"`
	// Write contains metrics related to performed writes.
	Write FioWriteResult `json:"write"`
	// Sync contains metrics related to performed fsync calls.
	Sync FioSyncResult `json:"sync"`
}

// GetWriteIOPS returns number of write iops.
func (j FioJobResult) GetWriteIOPS() float64 {
	return j.Write.IOPS
}

// GetFsyncLatency returns 99th percentile of fsync latency in milliseconds.
func (j FioJobResult) GetFsyncLatency() int64 {
	return j.Sync.Latency.Percentile[bucket99] / 1000000
}

// FioReadResult contains reads-related metrics.
type FioReadResult struct {
	// IOPS is the number of read iops.
	IOPS float64 `json:"iops"`
}

// FioWriteResult contains writes-related metrics.
type FioWriteResult struct {
	// IOPS is the number of write iops.
	IOPS float64 `json:"iops"`
}

// FioSyncResult contains fsync-related metrics.
type FioSyncResult struct {
	// Latency contains fsync latencies distribution.
	Latency FioSyncLatency `json:"lat_ns"`
}

// FioSyncLatency contains fsync latencies distribution.
type FioSyncLatency struct {
	// Percentile is the fsync percentile buckets.
	Percentile map[string]int64 `json:"percentile"`
}

// bucket99 is the name of the fio's 99th percentile bucket.
const bucket99 = "99.000000"
