/*
Copyright 2017 Gravitational, Inc.

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

	"github.com/gravitational/satellite/agent/health"
	pb "github.com/gravitational/satellite/agent/proto/agentpb"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// NewPortChecker returns a new port range checker
func NewPortChecker(ranges ...PortRange) health.Checker {
	return &portChecker{
		ranges:   ranges,
		getPorts: realGetPorts,
	}
}

// PortRange defines ports and protocol family to check
type PortRange struct {
	// Protocol of the port
	Protocol string
	// Port range.
	// A single port is defined as From == To
	From, To uint64
	// Description specifies the user-friendly range description
	Description string
	// ListenAddr optionally restricts the listener address.
	// Any address will match if unspecified
	ListenAddr string
}

// portChecker will validate that all required ports are in fact unoccupied
type portChecker struct {
	ranges   []PortRange
	getPorts portCollectorFunc
}

// Name returns this checker name
// Implements health.Checker
func (c *portChecker) Name() string {
	return portCheckerID
}

// Check will scan current open ports and report every conflict detected
// Implements health.Checker
func (c *portChecker) Check(ctx context.Context, reporter health.Reporter) {
	processes, err := c.getPorts()
	if err != nil {
		reporter.Add(NewProbeFromErr(portCheckerID, "failed to query socket connections", trace.Wrap(err)))
		return
	}

	type conn struct {
		pid      pid
		port     int
		protocol string
	}
	// Group processes on the pid/port/protocol to avoid duplicates
	unique := make(map[conn]process)
	for _, process := range processes {
		conn := conn{
			pid:      process.pid,
			port:     process.localAddr().port,
			protocol: process.proto(),
		}
		if _, exists := unique[conn]; !exists {
			unique[conn] = process
		}
	}

	conflicts := false
	for _, proc := range unique {
		if c.checkProcess(proc, reporter) {
			conflicts = true
		}
	}

	if conflicts {
		return
	}
	reporter.Add(NewSuccessProbe(c.Name()))
}

func (c *portChecker) checkProcess(proc process, reporter health.Reporter) bool {
	conflicts := false
	for _, r := range c.ranges {
		if r.Protocol != proc.socket.proto() {
			continue
		}
		switch proc.socket.state() {
		case TimeWait, Close, CloseWait:
			// ignore sockets in certain terminal states
			log.WithFields(log.Fields{
				"socket":  formatSocket(proc.socket),
				"process": proc.name,
				"pid":     proc.pid,
			}).Debug("Ignore lingering socket.")
			continue
		}

		if isPortConflict(proc, r) && isIPConflict(proc, r) {
			conflicts = true
			reporter.Add(&pb.Probe{
				Checker: portCheckerID,
				Detail: fmt.Sprintf("conflicting program %q(pid=%v) is occupying port %v/%d(%v)",
					proc.name, proc.pid, proc.socket.proto(), proc.localAddr().port, proc.state()),
				Status: pb.Probe_Failed})
		}
	}
	return conflicts
}

func isPortConflict(proc process, portRange PortRange) bool {
	return uint64(proc.localAddr().port) >= portRange.From &&
		uint64(proc.localAddr().port) <= portRange.To
}

func isIPConflict(proc process, portRange PortRange) bool {
	localAddr := proc.localAddr().ip.String()
	addr := portRange.ListenAddr
	if addr == "" {
		addr = localAddr
	}

	return proc.localAddr().ip.IsUnspecified() || localAddr == addr
}

const (
	protoTCP      = "tcp"
	protoUDP      = "udp"
	portCheckerID = "port-checker"
)
