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

package utils

import (
	"fmt"
	"net"
	"strings"

	"github.com/gravitational/gravity/lib/defaults"

	"github.com/gravitational/trace"
	"github.com/miekg/dns"
	log "github.com/sirupsen/logrus"
)

// ResolveAddr resolves the provided hostname using the local resolver
func ResolveAddr(addr string) (hostPort string, err error) {
	host := addr
	port := ""
	if strings.Contains(addr, ":") {
		host, port, err = net.SplitHostPort(addr)
		if err != nil {
			return "", trace.Wrap(err)
		}
	}
	if ip := net.ParseIP(host); len(ip) == 0 {
		c := dns.Client{}
		m := dns.Msg{}
		m.SetQuestion(host+".", dns.TypeA)
		r, t, err := c.Exchange(&m, defaults.LocalResolverAddr)
		if err != nil {
			return "", trace.Wrap(err)
		}
		log.Debugf("Resolve %v took %v.", host, t)
		if len(r.Answer) == 0 {
			return "", trace.ConnectionProblem(nil, "failed to resolve %v", addr)
		}
		for _, answer := range r.Answer {
			switch record := answer.(type) {
			case *dns.A:
				log.Debugf("Resolved %v to %v.", host, record.A)
				host = record.A.String()
			case *dns.CNAME:
				// DNS server would resolve CNAME RR to a domain name
				// and restart the query with that domain name.
				// As a result, the Answer section would contain both answers
				// since the initial query was for an A RR.
				// See https://tools.ietf.org/html/rfc1034#section-3.6.2 for details.
				//
				// We're skipping this RR to process the next A record
				continue
			default:
				return "", trace.ConnectionProblem(nil,
					"failed to resolve %v: unexpected record type %T", host, record)
			}
			break
		}
	}
	if port != "" {
		host = fmt.Sprintf("%v:%v", host, port)
	}
	return host, nil
}
