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

	"github.com/gravitational/trace"
	"github.com/miekg/dns"
)

// DNSMonitor will monitor a list of DNS servers for valid responses
type DNSChecker struct {
	// QuestionA is a slice of questions to ask for A (Host) records
	QuestionA []string
	// QuestionNS is a slice of questions to ask for NS (Nameserver) records
	QuestionNS []string
	// Nameservers is a slice of nameserver addresses to use
	Nameservers []string
}

// Name returns the name of this checker
func (r *DNSChecker) Name() string { return "dns" }

// Check checks if the DNS servers are responding
func (r *DNSChecker) Check(ctx context.Context, reporter health.Reporter) {
	nameservers := r.Nameservers
	if len(r.Nameservers) == 0 {
		clientconfig, err := dns.ClientConfigFromFile("/etc/resolv.conf")
		if err != nil {
			reporter.Add(NewProbeFromErr(r.Name(), "failed to load /etc/resolv.conf", err))
			return
		}

		nameservers = clientconfig.Servers
	}

	for _, nameserver := range nameservers {
		r.checkWithResolver(ctx, reporter, nameserver)
	}

}

func (r *DNSChecker) checkWithResolver(
	ctx context.Context,
	reporter health.Reporter,
	nameserver string,
) {
	checkFailed := false
	q := new(dns.Msg)
	q.Id = dns.Id()
	q.RecursionDesired = true
	q.Question = make([]dns.Question, 1)

	for questionType, questions := range map[uint16][]string{dns.TypeA: r.QuestionA, dns.TypeNS: r.QuestionNS} {
		for _, question := range questions {
			q.Question[0] = dns.Question{question, questionType, dns.ClassINET}
			in, err := dns.ExchangeContext(ctx, q, ensurePort(nameserver, "53"))
			if err != nil {
				reporter.Add(NewProbeFromErr(r.Name(), errorDetail(question, dns.TypeToString[questionType], nameserver), err))
				checkFailed = true
				continue
			}
			if in.Rcode != dns.RcodeSuccess {
				if rcode, ok := dns.RcodeToString[in.Rcode]; ok {
					reporter.Add(
						NewProbeFromErr(r.Name(), errorDetail(question, dns.TypeToString[questionType], nameserver),
							trace.BadParameter(rcode)))
				} else {
					reporter.Add(
						NewProbeFromErr(r.Name(), errorDetail(question, dns.TypeToString[questionType], nameserver),
							trace.BadParameter(fmt.Sprint(in.Rcode))))
				}

				checkFailed = true
			}
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

func ensurePort(address, defaultPort string) string {
	if _, _, err := net.SplitHostPort(address); err == nil {
		return address
	}
	return net.JoinHostPort(address, defaultPort)
}

func errorDetail(question, recordType, nameserver string) string {
	return fmt.Sprintf("failed to resolve '%v' (%v) nameserver %v", question, recordType, nameserver)
}
