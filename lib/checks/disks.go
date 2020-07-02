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
	"os"
	"path/filepath"
	"strconv"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/network/validation/proto"
	"github.com/gravitational/gravity/lib/state"

	"github.com/gravitational/satellite/agent/proto/agentpb"
	"github.com/gravitational/trace"
)

// checkEtcdDisk makes sure that the disk used for etcd wal satisfies
// performance requirements.
func (r *checker) checkEtcdDisk(ctx context.Context, server Server) ([]*agentpb.Probe, error) {
	// Test file should reside where etcd data will be.
	testPath := state.InEtcdDir(server.ServerInfo.StateDir, testFile)
	res, err := r.Remote.CheckDisks(ctx, server.AdvertiseIP, fioEtcdJob(testPath))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	log.Debugf("Server %v disk test results: %s.", server.Hostname, res.String())
	if len(res.Jobs) != 1 {
		return nil, trace.BadParameter("expected 1 job result: %v", res)
	}
	iops := res.Jobs[0].GetWriteIOPS()
	latency := res.Jobs[0].GetFsyncLatency()
	probes := formatEtcdProbes(server, testPath, iops, latency)
	if len(probes) > 0 {
		return probes, nil
	}
	log.Infof("Server %v passed etcd disk check, has %v sequential write iops and %vms fsync latency.",
		server.Hostname, iops, latency)
	return nil, nil
}

// fioEtcdJob constructs a request to check etcd disk performance.
func fioEtcdJob(filename string) *proto.CheckDisksRequest {
	// The recommendations for the fio configuration for etcd disk test
	// were adopted from the following blog post:
	//
	// https://www.ibm.com/cloud/blog/using-fio-to-tell-whether-your-storage-is-fast-enough-for-etcd
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

// formatEtcdProbes returns appropritate probes based on the etcd disk
// performance test results.
func formatEtcdProbes(server Server, testPath string, iops float64, latency int64) (probes []*agentpb.Probe) {
	if iops < getEtcdIOPSHard() {
		probes = append(probes, &agentpb.Probe{
			Detail: fmt.Sprintf("node %v sequential write IOPS on %v is lower than %v (%v)",
				server.Hostname, filepath.Dir(testPath), EtcdMinWriteIOPSHard, iops),
		})
	} else if iops < getEtcdIOPSSoft() {
		probes = append(probes, &agentpb.Probe{
			Detail: fmt.Sprintf("node %v sequential write IOPS on %v is lower than %v (%v) which may result in poor etcd performance",
				server.Hostname, filepath.Dir(testPath), EtcdMinWriteIOPSSoft, iops),
			Severity: agentpb.Probe_Warning,
		})
	}
	if latency > getEtcdLatencyHard() {
		probes = append(probes, &agentpb.Probe{
			Detail: fmt.Sprintf("node %v fsync latency on %v is higher than %vms (%vms)",
				server.Hostname, filepath.Dir(testPath), EtcdMaxFsyncLatencyMsHard, latency),
		})
	} else if latency > getEtcdLatencySoft() {
		probes = append(probes, &agentpb.Probe{
			Detail: fmt.Sprintf("node %v fsync latency on %v is higher than %vms (%vms) which may result in poor etcd performance",
				server.Hostname, filepath.Dir(testPath), EtcdMaxFsyncLatencyMsHard, latency),
			Severity: agentpb.Probe_Warning,
		})
	}
	return probes
}

const (
	// EtcdMinWriteIOPSSoft defines the soft threshold for a minimum number of
	// sequential write iops required for etcd to perform effectively.
	//
	// The number is recommended by etcd documentation:
	// https://github.com/etcd-io/etcd/blob/master/Documentation/op-guide/hardware.md#disks
	//
	// The soft threshold will generate a warning.
	EtcdMinWriteIOPSSoft = 50
	// EtcdMinWriteIOPSHard is the lowest number of IOPS Gravity will tolerate
	// before generating a critical probe failure.
	EtcdMinWriteIOPSHard = 500 //10

	// EtcdMaxFsyncLatencyMsSoft defines the soft threshold for a maximum fsync
	// latency required for etcd to perform effectively, in milliseconds.
	//
	// Etcd documentation recommends 10ms for optimal performance but we're
	// being conservative here to ensure better dev/test experience:
	// https://github.com/etcd-io/etcd/blob/master/Documentation/faq.md#what-does-the-etcd-warning-failed-to-send-out-heartbeat-on-time-mean
	//
	// The soft threshold will generate a warning.
	EtcdMaxFsyncLatencyMsSoft = 50
	// EtcdMaxFsyncLatencyMsHard is the highest fsync latency Gravity prechecks
	// will tolerate before generating a critical probe failure.
	EtcdMaxFsyncLatencyMsHard = 1 // 150

	// testFile is the name of the disk performance test file.
	testFile = "fio.test"
)

func getEtcdIOPSSoft() float64 {
	value, err := getEnvInt(EtcdIOPSSoftEnvVar)
	if err == nil {
		return float64(value)
	}
	return EtcdMinWriteIOPSSoft
}

func getEtcdIOPSHard() float64 {
	value, err := getEnvInt(EtcdIOPSHardEnvVar)
	if err == nil {
		return float64(value)
	}
	return EtcdMinWriteIOPSHard
}

func getEtcdLatencySoft() int64 {
	value, err := getEnvInt(EtcdLatencySoftEnvVar)
	if err == nil {
		return int64(value)
	}
	return EtcdMaxFsyncLatencyMsSoft
}

func getEtcdLatencyHard() int64 {
	value, err := getEnvInt(EtcdLatencyHardEnvVar)
	if err == nil {
		return int64(value)
	}
	return EtcdMaxFsyncLatencyMsHard
}

func getEnvInt(name string) (int, error) {
	value := os.Getenv(name)
	if value == "" {
		return 0, trace.NotFound("environment variable %v not set", name)
	}
	valueS, err := strconv.Atoi(name)
	if err != nil {
		return 0, trace.Wrap(err)
	}
	return valueS, nil
}

const (
	EtcdIOPSSoftEnvVar    = "GRAVITY_ETCD_IOPS_SOFT"
	EtcdIOPSHardEnvVar    = "GRAVITY_ETCD_IOPS_HARD"
	EtcdLatencySoftEnvVar = "GRAVITY_ETCD_LATENCY_SOFT"
	EtcdLatencyHardEnvVar = "GRAVITY_ETCD_LATENCY_HARD"
)
