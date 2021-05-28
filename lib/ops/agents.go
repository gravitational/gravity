/*
Copyright 2018 Gravitational, Inc.

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

package ops

import (
	"fmt"

	"github.com/gravitational/gravity/lib/checks"
	"github.com/gravitational/gravity/lib/schema"

	"github.com/gravitational/trace"
)

// AgentReport provides information about servers as
// collected by remote install agents run on site during
// install and upgrade procedures
type AgentReport struct {
	// Message is a human readable message presented to the user
	Message string `json:"message"`
	// Servers returns a list of servers that have agents
	// installed on them
	Servers []checks.ServerInfo `json:"servers"`
}

// RawAgentReport is a transport-friendly agent report representation
type RawAgentReport struct {
	// Message is a human readable message presented to the user
	Message string `json:"message"`
	// Servers returns a list of servers that have agents
	// installed on them
	Servers []checks.RawServerInfo `json:"servers"`
}

// Transport returns transport-friendly representation
// of agent report
func (s *AgentReport) Transport() (*RawAgentReport, error) {
	resp := RawAgentReport{Message: s.Message}
	for _, server := range s.Servers {
		info, err := server.Transport()
		if err != nil {
			return nil, trace.Wrap(err)
		}

		resp.Servers = append(resp.Servers, *info)
	}
	return &resp, nil
}

// FromTransport converts from transport-friendly representation
// of agent report
func (r *RawAgentReport) FromTransport() (*AgentReport, error) {
	resp := AgentReport{Message: r.Message}
	for _, server := range r.Servers {
		info, err := server.FromTransport()
		if err != nil {
			return nil, trace.Wrap(err)
		}

		resp.Servers = append(resp.Servers, *info)
	}
	return &resp, nil
}

// String returns textual representation of the report
func (s *AgentReport) String() string {
	return fmt.Sprintf(
		"AgentReport(Message=%v, Servers=%v)", s.Message, len(s.Servers))
}

// Has returns true if this agent report contains server with the provided IP.
func (s *AgentReport) Has(advertiseAddr string) bool {
	for _, server := range s.Servers {
		if server.AdvertiseAddr == advertiseAddr {
			return true
		}
	}
	return false
}

// Diff returns added/removed servers this agent report has compared to
// the provided previous report.
func (s *AgentReport) Diff(previous *AgentReport) (added, removed []checks.ServerInfo) {
	if previous == nil {
		return s.Servers, nil
	}
	for _, server := range s.Servers {
		if !previous.Has(server.AdvertiseAddr) {
			added = append(added, server)
		}
	}
	for _, server := range previous.Servers {
		if !s.Has(server.AdvertiseAddr) {
			removed = append(removed, server)
		}
	}
	return
}

// MatchFlavor verifies if agents from this report satisfy the provided flavor.
//
// Returns number/roles of agents that still need to join as well as any
// extra servers that are not a part of the flavor.
func (s *AgentReport) MatchFlavor(flavor schema.Flavor) (needed map[string]int, extra []checks.ServerInfo) {
	needed = make(map[string]int)
	for _, node := range flavor.Nodes {
		needed[node.Profile] = node.Count
	}
	for _, server := range s.Servers {
		if _, ok := needed[server.Role]; ok {
			needed[server.Role] -= 1
			if needed[server.Role] == 0 {
				delete(needed, server.Role)
			}
		} else {
			extra = append(extra, server)
		}
	}
	return
}
