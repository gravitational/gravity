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

package monitoring

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/satellite/agent"
	"github.com/gravitational/satellite/agent/health"
	pb "github.com/gravitational/satellite/agent/proto/agentpb"

	"github.com/codahale/hdrhistogram"
	"github.com/gravitational/trace"
	"github.com/gravitational/ttlmap/v2"
	serf "github.com/hashicorp/serf/client"
	log "github.com/sirupsen/logrus"
)

// TODO: latencyThreshold should be configurable
// TODO: latency stats should be sent to metrics

const (
	// pingCheckerID specifies the check name
	pingCheckerID = "ping-checker"
	// latencyStatsTTLSeconds specifies how long check results will be kept before being dropped
	latencyStatsTTLSeconds = 3600 // 1 hour
	// latencyStatsCapacity sets the number of TTLMaps that can be stored; this will be the size of the cluster -1
	latencyStatsCapacity = 1000
	// latencyStatsSlidingWindowSize specifies the number of retained check results
	latencyStatsSlidingWindowSize = 20
	// pingMinimum sets the minimum value that can be recorded
	pingMinimum = 0 * time.Second
	// pingMaximum sets the maximum value that can be recorded
	pingMaximum = 10 * time.Second
	// pingSignificantFigures specifies how many decimals should be recorded
	pingSignificantFigures = 3
	// latencyThreshold sets the RTT threshold
	latencyThreshold = 15 * time.Millisecond
	// latencyQuantile sets the quantile used while checking Histograms against RTT results
	latencyQuantile = 95.0
)

// pingChecker is a checker that verifies that ping times (RTT) between nodes in
// the cluster are within a predefined threshold
type pingChecker struct {
	self           serf.Member
	serfClient     agent.SerfClient
	serfMemberName string
	latencyStats   ttlmap.TTLMap
	mux            sync.Mutex
	logger         log.FieldLogger
}

// PingCheckerConfig is used to store all the configuration related to the current check
type PingCheckerConfig struct {
	// SerfRPCAddr is the address used by the Serf RPC client to communicate
	// with the Serf cluster
	SerfRPCAddr string
	// SerfMemberName is the name assigned to this node in Serf
	SerfMemberName string
	// NewSerfClient is an optional Serf Client function that can be used instead
	// of the default one. If not specified it will fallback to the default one
	NewSerfClient agent.NewSerfClientFunc
}

// CheckAndSetDefaults validates that this configuration is correct and sets
// value defaults where necessary
func (c *PingCheckerConfig) CheckAndSetDefaults() error {
	if c.SerfMemberName == "" {
		return trace.BadParameter("serf member name can't be empty")
	}
	if c.NewSerfClient == nil {
		c.NewSerfClient = agent.NewSerfClient
	}
	return nil
}

// NewPingChecker returns a checker that verifies accessibility of nodes in the cluster by exchanging ping requests
func NewPingChecker(conf PingCheckerConfig) (c health.Checker, err error) {
	err = conf.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	latencyTTLMap := ttlmap.NewTTLMap(latencyStatsCapacity)

	client, err := conf.NewSerfClient(serf.Config{
		Addr: conf.SerfRPCAddr,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	self, err := client.FindMember(conf.SerfMemberName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &pingChecker{
		self:           *self,
		serfClient:     client,
		serfMemberName: conf.SerfMemberName,
		latencyStats:   *latencyTTLMap,
		logger:         log.WithField(trace.Component, pingCheckerID),
	}, nil
}

// Name returns the checker name
// Implements health.Checker
func (c *pingChecker) Name() string {
	return pingCheckerID
}

// Check verifies that all nodes' ping with Master Nodes is lower than the
// desired threshold
// Implements health.Checker
func (c *pingChecker) Check(ctx context.Context, r health.Reporter) {
	if err := c.check(ctx, r); err != nil {
		c.logger.WithError(err).Debug("Failed to verify ping latency.")
		return
	}
	if r.NumProbes() == 0 {
		r.Add(NewSuccessProbe(c.Name()))
	}
}

// check runs the actual system status verification code and returns an error
// in case issues arise in the process
func (c *pingChecker) check(_ context.Context, r health.Reporter) (err error) {
	client := c.serfClient

	nodes, err := client.Members()
	if err != nil {
		return trace.Wrap(err)
	}

	if err = c.checkNodesRTT(nodes, client, r); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// checkNodesRTT implements the bulk of the logic by checking the ping time
// between this node (self) and the other Serf Cluster member nodes
func (c *pingChecker) checkNodesRTT(nodes []serf.Member, client agent.SerfClient,
	reporter health.Reporter) (err error) {
	// ping each other node and raise a warning in case the results are over
	// a specified threshold
	for _, node := range nodes {
		// skipping nodes that are not alive (failed, removed, etc..)
		if strings.ToLower(node.Status) != strings.ToLower(pb.MemberStatus_Alive.String()) {
			c.logger.Debugf("skipping node %s because status is %q", node.Name, node.Status)
			continue
		}
		// skip pinging self
		if c.self.Addr.String() == node.Addr.String() {
			c.logger.Debugf("skipping analyzing self node (%s)", node.Name)
			continue
		}
		c.logger.Debugf("node %s status %s", node.Name, node.Status)

		rttNanoSec, err := c.calculateRTT(client, c.self, node)
		if err != nil {
			return trace.Wrap(err)
		}

		latencies, err := c.saveLatencyStats(rttNanoSec, node)
		if err != nil {
			return trace.Wrap(err)
		}

		latencyHistogram, err := c.buildLatencyHistogram(node.Name, latencies)
		if err != nil {
			return trace.Wrap(err)
		}

		latency95 := time.Duration(latencyHistogram.ValueAtQuantile(latencyQuantile))

		c.logger.Debugf("%s <-ping-> %s = %s [latest]", c.self.Name, node.Name, time.Duration(rttNanoSec))
		c.logger.Debugf("%s <-ping-> %s = %s [%.2f percentile]", c.self.Name, node.Name, latency95, latencyQuantile)

		if latency95 >= latencyThreshold {
			c.logger.Warningf("%s <-ping-> %s = slow ping detected. Value %s over threshold %s",
				c.self.Name, node.Name, latency95, latencyThreshold)
			reporter.Add(c.failureProbe(node.Name, latency95))
		} else {
			c.logger.Debugf("%s <-ping-> %s = ping okay. Value %s within threshold %s",
				c.self.Name, node.Name, latency95, latencyThreshold)
		}
	}

	return nil
}

// buildLatencyHistogram maps latencies to a HDRHistrogram
func (c *pingChecker) buildLatencyHistogram(nodeName string, latencies []int64) (latencyHDR *hdrhistogram.Histogram, err error) {
	latencyHDR = hdrhistogram.New(pingMinimum.Nanoseconds(),
		pingMaximum.Nanoseconds(), pingSignificantFigures)

	for _, v := range latencies {
		err := latencyHDR.RecordValue(v)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return latencyHDR, nil
}

// saveLatencyStats is used to store ping values in HDR Histograms in memory
func (c *pingChecker) saveLatencyStats(pingLatency int64, node serf.Member) (latencies []int64, err error) {
	c.mux.Lock()
	defer c.mux.Unlock()

	if value, exists := c.latencyStats.Get(node.Name); exists {
		var ok bool
		if latencies, ok = value.([]int64); !ok {
			return nil, trace.BadParameter("couldn't parse node latency as []int64 on %s", c.serfMemberName)
		}
	}

	if len(latencies) >= latencyStatsSlidingWindowSize {
		// keep the slice within the sliding window size
		// slidingWindowSize is -1 because another element will be added a few lines below
		latencies = latencies[1:latencyStatsSlidingWindowSize]
	}

	latencies = append(latencies, pingLatency)
	c.logger.Debugf("%d recorded ping values for node %s => %v", len(latencies), node.Name, latencies)

	err = c.latencyStats.Set(node.Name, latencies, latencyStatsTTLSeconds)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return latencies, nil
}

// calculateRTT calculates and returns the latency time (in nanoseconds) between two Serf Cluster members
func (c *pingChecker) calculateRTT(serfClient agent.SerfClient, self, node serf.Member) (rttNanos int64, err error) {
	selfCoord, err := serfClient.GetCoordinate(self.Name)
	if err != nil {
		return 0, trace.Wrap(err)
	}
	if selfCoord == nil {
		return 0, trace.NotFound("could not find a coordinate for node %s", self.Name)
	}

	otherNodeCoord, err := serfClient.GetCoordinate(node.Name)
	if err != nil {
		return 0, trace.Wrap(err)
	}
	if otherNodeCoord == nil {
		return 0, trace.NotFound("could not find a coordinate for node %s", node.Name)
	}

	latency := selfCoord.DistanceTo(otherNodeCoord).Nanoseconds()
	c.logger.Debugf("self {%v,%v,%v,%v} === %v ===> other {%v,%v,%v,%v}",
		selfCoord.Vec, selfCoord.Error, selfCoord.Height, selfCoord.Adjustment,
		latency,
		otherNodeCoord.Vec, otherNodeCoord.Error, otherNodeCoord.Height, otherNodeCoord.Adjustment)
	return latency, nil
}

// failureProbe constructs a new probe that represents a failed ping check
// against the specified node.
func (c *pingChecker) failureProbe(node string, latency time.Duration) *pb.Probe {
	return &pb.Probe{
		Checker: c.Name(),
		Detail: fmt.Sprintf("ping between %s and %s is higher than the allowed threshold of %s",
			c.self.Name, node, latencyThreshold),
		Error:    fmt.Sprintf("ping latency at %s", latency),
		Status:   pb.Probe_Failed,
		Severity: pb.Probe_Warning,
	}
}
