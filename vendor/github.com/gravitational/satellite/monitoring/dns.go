/*
Copyright 2016 Gravitational, Inc.

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
	"net"

	"github.com/gravitational/satellite/agent/health"
	pb "github.com/gravitational/satellite/agent/proto/agentpb"
)

// DNSMonitor will monitor a list of DNS servers for valid responses
type DNSChecker struct {
	// QuestionA is a slice of questions to ask for A (Host) records
	QuestionA []string
}

// Name returns the name of this checker
func (r *DNSChecker) Name() string { return "dns" }

// Check checks if the DNS servers are responding
func (r *DNSChecker) Check(ctx context.Context, reporter health.Reporter) {
	checkFailed := false
	for _, question := range r.QuestionA {
		_, err := net.LookupHost(question)
		if err != nil {
			detail := fmt.Sprintf("failed to resolve %v", question)
			reporter.Add(NewProbeFromErr(r.Name(), detail, err))
			checkFailed = true
		}
	}

	if checkFailed {
		return
	}

	reporter.Add(&pb.Probe{
		Checker: r.Name(),
		Status:  pb.Probe_Running,
	})
}
