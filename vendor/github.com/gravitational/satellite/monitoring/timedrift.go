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
	"sync"
	"time"

	"github.com/gravitational/satellite/agent/health"
	pb "github.com/gravitational/satellite/agent/proto/agentpb"
	"github.com/gravitational/satellite/lib/membership"
	"github.com/gravitational/satellite/lib/rpc/client"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	log "github.com/sirupsen/logrus"
)

const (
	// timeDriftCheckerID is the time drift check name.
	timeDriftCheckerID = "time-drift"
	// timeDriftThreshold sets the default threshold of the acceptable time
	// difference between nodes.
	timeDriftThreshold = 300 * time.Millisecond

	// timeDriftCheckTimeout drops time checks where the RPC call to the remote server take too long to respond.
	// If the client or server is busy and the request takes too long to be processed, this will cause an inaccurate
	// comparison of the current time.
	timeDriftCheckTimeout = 100 * time.Millisecond

	// parallelRoutines indicates how many parallel queries we should run to peer nodes
	parallelRoutines = 20
)

// timeDriftChecker is a checker that verifies that the time difference between
// cluster nodes remains within the specified threshold.
type timeDriftChecker struct {
	// TimeDriftCheckerConfig contains checker configuration.
	TimeDriftCheckerConfig
	// FieldLogger is used for logging.
	log.FieldLogger
	// mu protects the clients map.
	mu sync.Mutex
}

// TimeDriftCheckerConfig stores configuration for the time drift check.
type TimeDriftCheckerConfig struct {
	// NodeName specifies the name of the node that is running the check.
	NodeName string
	// Cluster specifies the cluster membership interface.
	membership.Cluster
	// DialRPC is used to create Satellite RPC client.
	DialRPC client.DialRPC
	// Clock is used in tests to mock time.
	Clock clockwork.Clock
}

// CheckAndSetDefaults validates the config and sets default values.
func (c *TimeDriftCheckerConfig) CheckAndSetDefaults() error {
	var errs []error
	if c.NodeName == "" {
		errs = append(errs, trace.BadParameter("NodeName must be provided"))
	}
	if c.Cluster == nil {
		errs = append(errs, trace.BadParameter("Cluster must be provided"))
	}
	if c.DialRPC == nil {
		errs = append(errs, trace.BadParameter("DialRPC must be provided"))
	}
	if len(errs) > 0 {
		return trace.NewAggregate(errs...)
	}
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}
	return nil
}

// NewTimeDriftChecker returns a new instance of time drift checker.
func NewTimeDriftChecker(conf TimeDriftCheckerConfig) (c health.Checker, err error) {
	if err := conf.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &timeDriftChecker{
		TimeDriftCheckerConfig: conf,
		FieldLogger:            log.WithField(trace.Component, timeDriftCheckerID),
	}, nil
}

// Name returns the checker name.
func (c *timeDriftChecker) Name() string {
	return timeDriftCheckerID
}

// Check fills in provided reporter with probes according to time drift check results.
func (c *timeDriftChecker) Check(ctx context.Context, r health.Reporter) {
	if err := c.check(ctx, r); err != nil {
		log.WithError(err).Debug("Failed to check time drift.")
		return
	}
	if r.NumProbes() == 0 {
		r.Add(successProbeTimeDrift(c.NodeName))
	}
}

// check does a time drift check between this and other cluster nodes.
func (c *timeDriftChecker) check(ctx context.Context, r health.Reporter) (err error) {
	nodes, err := c.nodesToCheck()
	if err != nil {
		return trace.Wrap(err)
	}

	nodesC := make(chan *pb.MemberStatus, len(nodes))
	for _, node := range nodes {
		nodesC <- node
	}
	close(nodesC)

	var mutex sync.Mutex

	var wg sync.WaitGroup

	wg.Add(parallelRoutines)

	for i := 0; i < parallelRoutines; i++ {
		go func() {
			for node := range nodesC {
				drift, err := c.getTimeDrift(ctx, node)
				if err != nil {
					log.WithError(err).Debug("Failed to get time drift.")
					continue
				}

				if isDriftHigh(drift) {
					mutex.Lock()
					r.Add(failureProbeTimeDrift(c.NodeName, node.Name, drift))
					mutex.Unlock()
				}
			}
			wg.Done()
		}()
	}

	wg.Wait()
	return nil
}

// getTimeDrift calculates the time drift value between this and the specified
// node using the following algorithm.
//
// Every coordinator node (Kubernetes masters) executes an instance
// of this algorithm.

// For each of the remaining cluster nodes (including other coordinator nodes):

// * Selected coordinator node records its local timestamp (in UTC). Let’s call
//   this timestamp T1Start.

// * Coordinator initiates a "ping" grpc request to the node.

// * The node responds to the ping request replying with node's local timestamp
//   (in UTC) in the payload. Let's call this timestamp T2.

// * After receiving the remote response, coordinator records the second local
//   timestamp. Let's call it T1End.

// * Coordinator calculates the latency between itself and the node:
//   (T1End-T1Start)/2. Let's call this value Latency.

// * Coordinator calculates the time drift between itself and the node:
//   T2-T1Start-Latency. Let's call this value Drift. Can be negative which would
//   mean the node time is falling behind.

// * Compare abs(Drift) with the threshold.
func (c *timeDriftChecker) getTimeDrift(ctx context.Context, node *pb.MemberStatus) (time.Duration, error) {
	agentClient, err := c.getAgentClient(ctx, node)
	if err != nil {
		return 0, trace.Wrap(err)
	}

	queryStart := c.Clock.Now().UTC()

	// if the RPC call takes a long duration it will result in an inaccurate comparison. Timeout the RPC
	// call to reduce false positives on a slow server.
	ctx, cancel := context.WithTimeout(ctx, timeDriftCheckTimeout)
	defer cancel()

	// Send "time" request to the specified node.
	peerResponse, err := agentClient.Time(ctx, &pb.TimeRequest{})
	if err != nil {
		// If the agent we're making request to is of an older version,
		// it may not support Time() method yet. This can happen, e.g.,
		// during a rolling upgrade. In this case fallback to success.
		if trace.IsNotImplemented(err) {
			c.WithField("node", node.Name).Warnf(trace.UserMessage(err))
			return 0, nil
		}
		return 0, trace.Wrap(err)
	}

	queryEnd := c.Clock.Now().UTC()

	// The request / response will take some time to perform over the network
	// Use an adjustment of half the RTT time under the assumption that the request / response consume
	// equal delays.
	latencyAdjustment := queryEnd.Sub(queryStart) / 2

	adjustedPeerTime := peerResponse.GetTimestamp().ToTime().Add(latencyAdjustment)

	// drift is relative to the current nodes time.
	// if peer time > node time, return a positive duration
	// if peer time < node time, return a negative duration
	drift := adjustedPeerTime.Sub(queryEnd)
	c.WithField("node", node.Name).Debugf("queryStart: %v; queryEnd: %v; peerTime: %v; adjustedPeerTime: %v drift: %v.",
		queryStart, queryEnd, peerResponse.GetTimestamp().ToTime(), adjustedPeerTime, drift)
	return drift, nil
}

// nodesToCheck returns nodes to check time drift against.
func (c *timeDriftChecker) nodesToCheck() (result []*pb.MemberStatus, err error) {
	nodes, err := c.Cluster.Members()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	for _, node := range nodes {
		if c.shouldCheckNode(node) {
			result = append(result, node)
		}
	}
	return result, nil
}

// shouldCheckNode returns true if the check should be run against specified
// member.
func (c *timeDriftChecker) shouldCheckNode(node *pb.MemberStatus) bool {
	return node.Status == pb.MemberStatus_Alive && c.NodeName != node.Name
}

// getAgentClient returns Satellite agent client for the provided node.
func (c *timeDriftChecker) getAgentClient(ctx context.Context, node *pb.MemberStatus) (client.Client, error) {
	return c.DialRPC(ctx, node.Addr)
}

// isDriftHigh returns true if the provided drift value is over the threshold.
func isDriftHigh(drift time.Duration) bool {
	return drift < 0 && -drift > timeDriftThreshold || drift > timeDriftThreshold
}

// successProbeTimeDrift constructs a probe that represents successful time drift check.
func successProbeTimeDrift(node string) *pb.Probe {
	return &pb.Probe{
		Checker: timeDriftCheckerID,
		Detail: fmt.Sprintf("time drift between %s and other nodes is within the allowed threshold of %s",
			node, timeDriftThreshold),
		Status: pb.Probe_Running,
	}
}

// failureProbeTimeDrift constructs a probe that represents failed time drift check
// between the specified nodes.
func failureProbeTimeDrift(node1, node2 string, drift time.Duration) *pb.Probe {
	return &pb.Probe{
		Checker: timeDriftCheckerID,
		Detail:  fmt.Sprintf("time drift between %s and %s is %s", node1, node2, drift),
		Error:   fmt.Sprintf("time drift is higher than the allowed threshold of %s", timeDriftThreshold),
		Status:  pb.Probe_Failed,
	}
}
