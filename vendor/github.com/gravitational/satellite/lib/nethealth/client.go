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
	"bytes"
	"context"
	"io/ioutil"
	"net"
	"net/http"

	"github.com/gravitational/trace"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/sirupsen/logrus"
)

// Client provides nethealth client interface. Client can be used to query
// exposed nethealth metrics.
type Client struct {
	// socketPath specifies nethealth socket path.
	socketPath string
	// FieldLogger is used for logging.
	logrus.FieldLogger
}

// NewClient constructs a new Client with the provided socket path.
func NewClient(socketPath string) *Client {
	return &Client{
		socketPath:  socketPath,
		FieldLogger: logrus.WithField(trace.Component, "nethealth-client"),
	}
}

// LatencySummariesMilli returns the latency summary for each peer. The latency
// values represent milliseconds.
func (r *Client) LatencySummariesMilli(ctx context.Context) (map[string]*dto.Summary, error) {
	const labelLatencySummary = "nethealth_echo_latency_summary_milli"

	metrics, err := r.metrics(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "failed to retrieve metrics")
	}

	summaries, err := parseSummaries(metrics, labelLatencySummary)
	if err != nil {
		return nil, trace.Wrap(err, "failed to parse latency summaries")
	}

	return summaries, nil
}

// parseSummaries parses the metrics and returns the summaries for the specified
// label. Returns NotFound if the label does not exist.
func parseSummaries(metricFamilies map[string]*dto.MetricFamily, label string) (map[string]*dto.Summary, error) {
	metricFamily, ok := metricFamilies[label]
	if !ok {
		return nil, trace.NotFound("%s metrics not found", label)
	}

	summaries := make(map[string]*dto.Summary)
	for _, m := range metricFamily.GetMetric() {
		peerName, err := getPeerName(m.GetLabel())
		if err != nil {
			logrus.WithError(err).Warn("Failed to get peer name.")
			continue
		}
		summaries[peerName] = m.GetSummary()
	}

	return summaries, nil
}

// metrics fetches the nethealth metrics and returns the metric families.
func (r *Client) metrics(ctx context.Context) (res map[string]*dto.MetricFamily, err error) {
	var dialer net.Dialer
	client := http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return dialer.DialContext(ctx, "unix", r.socketPath)
			},
		},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://unix/metrics", nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, trace.BadParameter("unexpected response from %s: %v", r.socketPath, resp.Status)
	}

	buffer, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}

	metricFamilies, err := parseMetrics(buffer)
	if err != nil {
		r.WithField("nethealth-metrics", string(buffer)).Debug("Failed to parse nethealth metrics.")
		return nil, trace.Wrap(err, "failed to parse nethealth metrics")
	}

	return metricFamilies, nil
}

// parseMetrics parses the metrics and returns the metric families.
func parseMetrics(metrics []byte) (map[string]*dto.MetricFamily, error) {
	var parser expfmt.TextParser
	metricFamilies, err := parser.TextToMetricFamilies(bytes.NewReader(metrics))
	if err != nil {
		return nil, trace.Wrap(err, "failed to parse text to MetricFamilies")
	}
	return metricFamilies, nil
}

// getPeerName extracts the 'peer_name' value from the provided labels.
func getPeerName(labels []*dto.LabelPair) (peer string, err error) {
	for _, label := range labels {
		if LabelPeerName == label.GetName() {
			return label.GetValue(), nil
		}
	}
	return "", trace.NotFound("unable to find %s label", LabelPeerName)
}
