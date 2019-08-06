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

package checks

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/network/validation/proto"
	"github.com/gravitational/gravity/lib/state"

	"github.com/gravitational/trace"
)

// checkEtcdDisk makes sure that the disk used for etcd wal satisfies
// performance requirements.
func (r *checker) checkEtcdDisk(ctx context.Context, server Server) error {
	// Test file should reside where etcd data will be.
	testPath := state.InEtcdDir(server.ServerInfo.StateDir, testFile)
	res, err := r.Remote.CheckDisks(ctx, server.AdvertiseIP, fioEtcdJob(testPath))
	if err != nil {
		return trace.Wrap(err)
	}
	log.Debugf("Server %v disk test results: %s.", server.Hostname, res.String())
	if len(res.Jobs) != 1 {
		return trace.BadParameter("expected 1 job result: %v", res)
	}
	iops := res.Jobs[0].GetWriteIOPS()
	latency := res.Jobs[0].GetFsyncLatency()
	if iops < EtcdMinWriteIOPS || latency > EtcdMaxFsyncLatencyMs {
		return trace.BadParameter(formatEtcdMessage(server, testPath, iops, latency))
	}
	log.Infof("Server %v passed etcd disk check, has %v sequential write iops and %vms fsync latency.",
		server.Hostname, iops, latency)
	return nil
}

// fioEtcdJob constructs a request to check etcd disk performance.
func fioEtcdJob(filename string) *proto.CheckDisksRequest {
	spec := &proto.FioJobSpec{
		Name: "etcd",
		// perform sequential writes
		ReadWrite: "write",
		// use write() syscall for writes
		IoEngine: "sync",
		// sync every data write to disk
		Fdatasync: true,
		// test file, should reside where etcd WAL will be
		Filename: filename,
		// average block size written by etcd
		BlockSize: "2300",
		// total size of the test file
		Size_: "22m",
		// limit total test runtime
		Runtime: proto.DurationProto(defaults.DiskTestDuration),
	}
	return &proto.CheckDisksRequest{
		Jobs: []*proto.FioJobSpec{spec},
	}
}

// formatEtcdMessage returns appropritate formatted error message based
// on the etcd disk performance test results.
func formatEtcdMessage(server Server, testPath string, iops float64, latency int64) string {
	var errors []string
	if iops < EtcdMinWriteIOPS {
		errors = append(errors, fmt.Sprintf("  * Low sequential write IOPS of %v, required minimum is %v.",
			iops, EtcdMinWriteIOPS))
	}
	if latency > EtcdMaxFsyncLatencyMs {
		errors = append(errors, fmt.Sprintf("  * High fsync latency of %vms, required maximum is %vms.",
			latency, EtcdMaxFsyncLatencyMs))
	}
	return fmt.Sprintf(
		`It looks like on node %v etcd data resides on a disk (%v) that does not satisfy recommended performance requirements:
%v
For optimal performance, please make sure that the directory with etcd data resides on the device that meets the aforementioned
hardware requirements before proceeding.`,
		server.Hostname, filepath.Dir(testPath), strings.Join(errors, "\n"))
}

const (
	// EtcdMinWriteIOPS defines the minimum number of sequential write iops
	// required for etcd to perform effectively.
	//
	// The number is recommended by etcd documentation:
	// https://github.com/etcd-io/etcd/blob/master/Documentation/op-guide/hardware.md#disks
	//
	EtcdMinWriteIOPS = 50

	// EtcdMaxFsyncLatencyMs defines the maximum fsync latency required for
	// etcd to perform effectively, in milliseconds.
	//
	// Etcd documentation recommends 10ms for optimal performance but we're
	// being conservative here to ensure better dev/test experience:
	// https://github.com/etcd-io/etcd/blob/master/Documentation/faq.md#what-does-the-etcd-warning-failed-to-send-out-heartbeat-on-time-mean
	//
	EtcdMaxFsyncLatencyMs = 30

	// testFile is the name of the disk performance test file.
	testFile = "fio.test"
)
