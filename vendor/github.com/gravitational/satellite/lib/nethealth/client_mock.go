/*
Copyright 2020 Gravitational, Inc.

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

package nethealth

import (
	"context"

	"github.com/gravitational/trace"
	dto "github.com/prometheus/client_model/go"
)

// MockClient is a mock implementation of the nethealth Client. Instead of
// reading metrics from a live Prometheus metrics endpoint, metrics are
// pre-loaded and stored in the MockClient.
type MockClient struct {
	// textMetrics stores the current text formatted metrics.
	textMetrics string
}

// NewMockClient constructs a new MockClient with the provided metrics.
func NewMockClient(metrics string) *MockClient {
	return &MockClient{
		textMetrics: metrics,
	}
}

// LatencySummariesMilli returns the latency summary for each peer. The latency
// values represent milliseconds.
func (r *MockClient) LatencySummariesMilli(_ context.Context) (map[string]*dto.Summary, error) {
	const labelLatencySummary = "nethealth_echo_latency_summary_milli"

	metricFamilies, err := parseMetrics([]byte(r.textMetrics))
	if err != nil {
		return nil, trace.Wrap(err, "failed to parse metrics")
	}

	summaries, err := parseSummaries(metricFamilies, labelLatencySummary)
	if err != nil {
		return nil, trace.Wrap(err, "failed to parse prometheus summaries")
	}

	return summaries, nil
}
