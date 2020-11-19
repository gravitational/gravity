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
	"github.com/gravitational/satellite/lib/rpc/client"

	"github.com/gravitational/trace"
	serf "github.com/hashicorp/serf/client"
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
	// clients contains RPC clients for other cluster nodes.
	clients map[string]client.Client
}

// TimeDriftCheckerConfig stores configuration for the time drift check.
type TimeDriftCheckerConfig struct {
	// CAFile is the path to the certificate authority file for Satellite agent.
	CAFile string
	// CertFile is the path to the Satellite agent client certificate file.
	CertFile string
	// KeyFile is the path to the Satellite agent private key file.
	KeyFile string
	// SerfClient is the client to the local serf agent.
	SerfClient agent.SerfClient
	// SerfMember is the local serf member.
	SerfMember *serf.Member
	// DialRPC is used to create Satellite RPC client.
	DialRPC client.DialRPC
	// Clock is used in tests to mock time.
	Clock clockwork.Clock
}

// CheckAndSetDefaults validates the config and sets default values.
func (c *TimeDriftCheckerConfig) CheckAndSetDefaults() error {
	if c.CAFile == "" {
		return trace.BadParameter("agent CA certificate file can't be empty")
	}
	if c.CertFile == "" {
		return trace.BadParameter("agent certificate file can't be empty")
	}
	if c.KeyFile == "" {
		return trace.BadParameter("agent certificate key file can't be empty")
	}
	if c.SerfClient == nil {
		return trace.BadParameter("local serf client can't be empty")
	}
	if c.SerfMember == nil {
		return trace.BadParameter("local serf member can't be empty")
	}
	if c.DialRPC == nil {
		c.DialRPC = client.DefaultDialRPC(c.CAFile, c.CertFile, c.KeyFile)
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
		clients:                make(map[string]client.Client),
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
		r.Add(c.successProbe())
	}
}

// check does a time drift check between this and other cluster nodes.
func (c *timeDriftChecker) check(ctx context.Context, r health.Reporter) (err error) {
	nodes, err := c.nodesToCheck()
	if err != nil {
		return trace.Wrap(err)
	}

	nodesC := make(chan serf.Member, len(nodes))
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
					r.Add(c.failureProbe(node, drift))
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
func (c *timeDriftChecker) getTimeDrift(ctx context.Context, node serf.Member) (time.Duration, error) {
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

// successProbe constructs a probe that represents successful time drift check.
func (c *timeDriftChecker) successProbe() *pb.Probe {
	return &pb.Probe{
		Checker: c.Name(),
		Detail: fmt.Sprintf("time drift between %s and other nodes is within the allowed threshold of %s",
			c.SerfMember.Addr, timeDriftThreshold),
		Status: pb.Probe_Running,
	}
}

// failureProbe constructs a probe that represents failed time drift check
// against the specified node.
func (c *timeDriftChecker) failureProbe(node serf.Member, drift time.Duration) *pb.Probe {
	return &pb.Probe{
		Checker: c.Name(),
		Detail: fmt.Sprintf("time drift between %s and %s is higher than the allowed threshold of %s: %s",
			c.SerfMember.Addr, node.Addr, timeDriftThreshold, drift),
		Status: pb.Probe_Failed,
	}
}

// nodesToCheck returns nodes to check time drift against.
func (c *timeDriftChecker) nodesToCheck() (result []serf.Member, err error) {
	nodes, err := c.SerfClient.Members()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	c.removeExpiredClients(nodes)

	for _, node := range nodes {
		if c.shouldCheckNode(node) {
			result = append(result, node)
		}
	}
	return result, nil
}

// removeExpiredClients closes client connections to nodes that have left the
// cluster and deletes the entry from the cache.
func (c *timeDriftChecker) removeExpiredClients(members []serf.Member) {
	currentMembers := make(map[string]struct{})
	for _, member := range members {
		currentMembers[member.Addr.String()] = struct{}{}
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	for addr, conn := range c.clients {
		if _, ok := currentMembers[addr]; ok {
			continue
		}
		if err := conn.Close(); err != nil {
			log.WithError(err).WithField("address", addr).Error("Failed to close client connection.")
			continue
		}
		log.WithField("address", addr).Info("Closed client connection.")
		delete(c.clients, addr)
	}
}

// shouldCheckNode returns true if the check should be run against specified
// serf member.
func (c *timeDriftChecker) shouldCheckNode(node serf.Member) bool {
	return strings.ToLower(node.Status) == strings.ToLower(pb.MemberStatus_Alive.String()) &&
		c.SerfMember.Addr.String() != node.Addr.String()
}

// getAgentClient returns Satellite agent client for the provided node.
func (c *timeDriftChecker) getAgentClient(ctx context.Context, node serf.Member) (client.Client, error) {
	c.mu.Lock()
	if conn, exists := c.clients[node.Addr.String()]; exists {
		c.mu.Unlock()
		return conn, nil
	}
	c.mu.Unlock()

	newConn, err := c.DialRPC(ctx, &node)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Close newly created client connection if a new client was already cached while dialing.
	if conn, exists := c.clients[node.Addr.String()]; exists {
		if err := newConn.Close(); err != nil {
			log.WithError(err).WithField("address", node.Addr.String()).Error("Failed to close client connection.")
		}
		return conn, nil
	}

	// Cache and return new client connection.
	c.clients[node.Addr.String()] = newConn
	return newConn, nil
}

// isDriftHigh returns true if the provided drift value is over the threshold.
func isDriftHigh(drift time.Duration) bool {
	return drift < 0 && -drift > timeDriftThreshold || drift > timeDriftThreshold
}
