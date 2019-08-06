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
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"

	"github.com/gravitational/gravity/lib/checks/disks"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/state"

	"github.com/gravitational/trace"
)

// checkEtcdDisk makes sure that the disk used for etcd wal satisfies
// performance requirements.
func (r *checker) checkEtcdDisk(ctx context.Context, server Server) error {
	// Test file should reside where etcd data will be.
	testPath := state.InEtcdDir(server.ServerInfo.StateDir, testFile)
	err := r.ensureDir(ctx, server.AdvertiseIP, filepath.Dir(testPath))
	if err != nil {
		return trace.Wrap(err)
	}
	spec, err := disks.EtcdSpec(testPath, defaults.DiskTestDuration)
	if err != nil {
		return trace.Wrap(err)
	}
	resp, err := r.Remote.CheckDisks(ctx, server.AdvertiseIP, spec)
	if err != nil {
		return trace.Wrap(err)
	}
	err = r.rmFile(ctx, server.AdvertiseIP, testPath)
	if err != nil {
		log.WithError(err).Warnf("Failed to remove %v on %v.", testPath, server.Hostname)
	}
	var res disks.FioResult
	if err := json.Unmarshal(resp, &res); err != nil {
		return trace.Wrap(err)
	}
	log.Debugf("Server %v disk test results: %#v.", server.Hostname, res)
	if len(res.Jobs) != 1 {
		return trace.BadParameter("expected 1 job result: %v", res)
	}
	iops := res.Jobs[0].GetWriteIOPS()
	latency := res.Jobs[0].GetFsyncLatency()
	if iops < EtcdMinWriteIOPS || latency > EtcdMaxFsyncLatencyMs {
		return trace.BadParameter(
			`It looks like on node %v etcd data resides on a disk (%v) that has low sequential write IOPS (%v) or high fsync latency (%vms).

For optimal performance, etcd requires its WAL placed on a disk with a minimum of 50 sequential write IOPS and fsync
latency not greater than 10ms, otherwise the cluster will experience stability issues.

Please make sure that the directory with etcd data specified above resides on the device that meets the aforementioned
hardware requirements before proceeding.`, server.Hostname, filepath.Dir(testPath), iops, latency)
	}
	log.Infof("Server %v passed etcd disk check, has %v sequential write iops and %vms fsync latency.",
		server.Hostname, iops, latency)
	return nil
}

func (r *checker) ensureDir(ctx context.Context, addr, dir string) error {
	var out bytes.Buffer
	if err := r.Remote.Exec(ctx, addr, []string{"mkdir", "-p", dir}, &out); err != nil {
		return trace.Wrap(err, "failed to create directory %v on %v: %v",
			dir, addr, out.String())
	}
	return nil
}

func (r *checker) rmFile(ctx context.Context, addr, path string) error {
	var out bytes.Buffer
	if err := r.Remote.Exec(ctx, addr, []string{"rm", "-f", path}, &out); err != nil {
		return trace.Wrap(err, "failed to remove file %v on %v: %v",
			path, addr, out.String())
	}
	return nil
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
