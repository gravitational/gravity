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
	"io"
	"net"
	"net/http"
	"strconv"

	"github.com/gravitational/satellite/agent/health"
	pb "github.com/gravitational/satellite/agent/proto/agentpb"
	"github.com/gravitational/trace"
)

// HTTPResponseChecker is a function that can validate service health
// from the provided response
type HTTPResponseChecker func(response io.Reader) error

// HTTPHealthzChecker is a health.Checker that can validate service health over HTTP
type HTTPHealthzChecker struct {
	name    string
	URL     string
	client  *http.Client
	checker HTTPResponseChecker
}

// Name returns the name of this checker
func (r *HTTPHealthzChecker) Name() string { return r.name }

// Check runs an HTTP check and reports errors to the specified Reporter
func (r *HTTPHealthzChecker) Check(ctx context.Context, reporter health.Reporter) {
	req, err := http.NewRequest("GET", r.URL, nil)
	if err != nil {
		reporter.Add(NewProbeFromErr(r.name, noErrorDetail, trace.Errorf("failed to create request: %v", err)))
		return
	}
	req = req.WithContext(ctx)
	resp, err := r.client.Do(req)
	if err != nil {
		reporter.Add(NewProbeFromErr(r.name, noErrorDetail, trace.Errorf("healthz check failed: %v", err)))
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		reporter.Add(&pb.Probe{
			Checker: r.name,
			Status:  pb.Probe_Failed,
			Error: trace.Errorf("unexpected HTTP status: %s",
				http.StatusText(resp.StatusCode)).Error(),
			Code: strconv.Itoa(resp.StatusCode),
		})
		return
	}
	if err = r.checker(resp.Body); err != nil {
		reporter.Add(NewProbeFromErr(r.name, noErrorDetail, err))
		return
	}
	reporter.Add(&pb.Probe{
		Checker: r.name,
		Status:  pb.Probe_Running,
	})
}

// NewHTTPHealthzChecker creates a health.Checker for an HTTP health endpoint
// using the specified URL and a custom response checker
func NewHTTPHealthzChecker(name, URL string, checker HTTPResponseChecker) health.Checker {
	defaultTransport := http.RoundTripper(nil)
	return NewHTTPHealthzCheckerWithTransport(name, URL, defaultTransport, checker)
}

// NewUnixSocketHealthzChecker returns a new Checker that tests
// the specified unix domain socket path and URL
func NewUnixSocketHealthzChecker(name, URL, socketPath string, checker HTTPResponseChecker) health.Checker {
	transport := &http.Transport{
		Dial: func(network, addr string) (net.Conn, error) {
			return net.Dial("unix", socketPath)
		},
	}
	return NewHTTPHealthzCheckerWithTransport(name, URL, transport, checker)
}

// NewHTTPHealthzCheckerWithTransport creates a health.Checker for an HTTP health endpoint
// using the specified transport, URL and a custom response checker
func NewHTTPHealthzCheckerWithTransport(name, URL string, transport http.RoundTripper, checker HTTPResponseChecker) health.Checker {
	client := &http.Client{
		Transport: transport,
		Timeout:   defaultHTTPTimeout,
	}
	return NewHTTPHealthzCheckerWithClient(name, URL, client, checker)
}

// NewHTTPHealthzCheckerWithClient creates a health.Checker for an HTTP health endpoint
// using the specified HTTP client, URL and a custom response checker
func NewHTTPHealthzCheckerWithClient(name, URL string, client *http.Client, checker HTTPResponseChecker) health.Checker {
	return &HTTPHealthzChecker{
		name:    name,
		URL:     URL,
		client:  client,
		checker: checker,
	}
}
