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

package monitoring

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"sync"
	"time"

	"github.com/gravitational/satellite/agent"
	"github.com/gravitational/satellite/agent/health"
	"github.com/gravitational/satellite/utils"

	"github.com/gravitational/trace"
	"github.com/mailgun/holster"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
)

// NethealthConfig specifies configuration for a nethealth checker.
type NethealthConfig struct {
	// AdvertiseIP specifies the advertised ip address of the host running this checker.
	AdvertiseIP string
	// NethealthPort specifies the port that nethealth is listening on.
	NethealthPort int
	// NetStatsInterval specifies the duration to store net stats.
	NetStatsInterval time.Duration
	// KubeConfig specifies kubernetes access information.
	*KubeConfig
}

// CheckAndSetDefaults validates that this configuration is correct and sets
// value defaults where necessary.
func (c *NethealthConfig) CheckAndSetDefaults() error {
	var errors []error
	if c.AdvertiseIP == "" {
		errors = append(errors, trace.BadParameter("host advertise ip must be provided"))
	}
	if c.KubeConfig == nil {
		errors = append(errors, trace.BadParameter("kubernetes access config must be provided"))
	}
	if c.NethealthPort == 0 {
		c.NethealthPort = defaultNethealthPort
	}
	if c.NetStatsInterval == time.Duration(0) {
		c.NetStatsInterval = defaultNetStatsInterval
	}
	return trace.NewAggregate(errors...)
}

// nethealthChecker checks network communication between peers.
type nethealthChecker struct {
	// NethealthConfig contains caller specified nethealth checker configuration
	// values.
	NethealthConfig
	// Mutex locks access to peerStats
	sync.Mutex
	// peerStats maps a peer to its recorded nethealth stats.
	peerStats *netStats
}

// NewNethealthChecker returns a new nethealth checker.
func NewNethealthChecker(config NethealthConfig) (*nethealthChecker, error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	// seriesCapacity is determined by the NetStatsInterval / StatusUpdateTimeout
	// rounded up to the nearest integer
	seriesCapacity := math.Ceil(float64(config.NetStatsInterval) / float64(agent.StatusUpdateTimeout))

	return &nethealthChecker{
		peerStats:       newNetStats(netStatsCapacity, int(seriesCapacity)),
		NethealthConfig: config,
	}, nil
}

// Name returns this checker name
// Implements health.Checker
func (c *nethealthChecker) Name() string {
	return nethealthCheckerID
}

// Check verifies the network is healthy.
// Implements health.Checker
func (c *nethealthChecker) Check(ctx context.Context, reporter health.Reporter) {
	err := c.check(ctx, reporter)
	if err != nil {
		reporter.Add(NewProbeFromErr(c.Name(), "failed to verify nethealth", err))
		return
	}
	reporter.Add(NewSuccessProbe(c.Name()))
}

func (c *nethealthChecker) check(ctx context.Context, reporter health.Reporter) error {
	addr, err := c.getNethealthAddr()
	if trace.IsNotFound(err) {
		log.Debug("Nethealth pod was not found.")
		return nil // pod was not found, log and treat gracefully
	}
	if err != nil {
		return trace.Wrap(err) // received unexpected error, maybe network-related, will add error probe above
	}

	netData, err := fetchNethealthMetrics(ctx, addr)
	if err != nil {
		return trace.Wrap(err, "failed to fetch nethealth metrics")
	}

	updated, err := c.updateStats(netData)
	if err != nil {
		return trace.Wrap(err, "failed to update nethealth stats")
	}

	return c.verifyNethealth(updated, reporter)
}

// getNethealthAddr returns the address of the local nethealth pod.
func (c *nethealthChecker) getNethealthAddr() (addr string, err error) {
	opts := metav1.ListOptions{
		LabelSelector: nethealthLabelSelector.String(),
		FieldSelector: fields.OneTermEqualSelector("spec.nodeName", c.AdvertiseIP).String(),
		Limit:         1,
	}
	pods, err := c.Client.CoreV1().Pods(nethealthNamespace).List(opts)
	if err != nil {
		return addr, utils.ConvertError(err) // this will convert error to a proper trace error, e.g. trace.NotFound
	}

	if len(pods.Items) == 0 {
		return addr, trace.NotFound("unable to find local nethealth pod")
	}
	return fmt.Sprintf("http://%s:%d", pods.Items[0].Status.PodIP, c.NethealthPort), nil
}

// updateStats updates netStats with new incoming data.
// Returns the list of updated peers.
func (c *nethealthChecker) updateStats(incoming map[string]networkData) (updated []string, err error) {
	for peer, incomingData := range incoming {
		if err := c.updatePeer(peer, incomingData); err != nil {
			return updated, trace.Wrap(err)
		}

		// Record updated peers to be returned for later use in network verification step.
		updated = append(updated, peer)
	}
	return updated, nil
}

// updatePeer updates the peer's stats with the incoming data.
func (c *nethealthChecker) updatePeer(peer string, incomingData networkData) error {
	storedData, err := c.peerStats.Get(peer)
	if err != nil {
		return trace.Wrap(err)
	}

	// Calculate counter delta since last check and replace total.
	requestInc := incomingData.requestTotal - storedData.prevRequestTotal
	timeoutInc := incomingData.timeoutTotal - storedData.prevTimeoutTotal
	storedData.prevRequestTotal = incomingData.requestTotal
	storedData.prevTimeoutTotal = incomingData.timeoutTotal

	// Request counter should be strictly increasing and timeout counter should
	// be monotonically increasing. If this is not the case, nethealth pod most
	// likely restarted and reset the counters.
	if requestInc <= 0 || timeoutInc < 0 {
		log.WithField("request-inc", requestInc).
			WithField("timeout-inc", timeoutInc).
			Warn("Received network data may indicate that the nethealth pod was restarted.")
		if err := c.peerStats.Set(peer, storedData); err != nil {
			return trace.Wrap(err)
		}
		return nil
	}

	// It should not be possible for the timeout counter to have increased
	// more than the request counter. Log and ignore this situation.
	if timeoutInc > requestInc {
		log.WithField("request-inc", requestInc).
			WithField("timeout-inc", timeoutInc).
			Warn("Timeout counter increased more than request counter.")
		return nil
	}

	// Record new packet loss percentage, remove first data point if slice has reached capacity.
	if len(storedData.packetLoss) == cap(storedData.packetLoss) {
		copy(storedData.packetLoss, storedData.packetLoss[1:])
		storedData.packetLoss = storedData.packetLoss[:len(storedData.packetLoss)-1]
	}
	storedData.packetLoss = append(storedData.packetLoss, timeoutInc/requestInc)

	if err := c.peerStats.Set(peer, storedData); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// verifyNethealth verifies that the overlay network communication is healthy
// for the nodes specified by the provided list of peers. Failed probes will be
// reported for unhealthy peers.
func (c *nethealthChecker) verifyNethealth(peers []string, reporter health.Reporter) error {
	for _, peer := range peers {
		healthy, err := c.isHealthy(peer)
		if err != nil {
			return trace.Wrap(err)
		}
		if !healthy {
			reporter.Add(NewProbeFromErr(c.Name(), nethealthDetail(peer), nil))
		}
	}
	return nil
}

// isHealthy returns true if the overlay network is healthy for the specified
// peer.
func (c *nethealthChecker) isHealthy(peer string) (healthy bool, err error) {
	storedData, err := c.peerStats.Get(peer)
	if err != nil {
		return false, trace.Wrap(err)
	}

	// Checker has not collected enough data yet to check network health.
	if len(storedData.packetLoss) < cap(storedData.packetLoss) {
		return true, nil
	}

	// If the packet loss percentage is above the packet loss threshold throughout
	// the entire interval, overlay network communication to that peer will be
	// considered unhealthy.
	for _, packetLoss := range storedData.packetLoss {
		if packetLoss <= packetLossThreshold {
			return true, nil
		}
	}

	return false, nil
}

// fetchNethealthMetrics collects the network metrics from the nethealth pod
// specified by addr. Returns mapping of peer to networkData.
func fetchNethealthMetrics(ctx context.Context, addr string) (map[string]networkData, error) {
	// The two relevant metrics exposed by nethealth are 'nethealth_echo_request_total' and
	// 'nethealth_echo_timeout_total'. We expect a pair of request/timeout metrics per peer.
	// Example metrics received from nethealth may look something like the output below:
	//
	//      # HELP nethealth_echo_request_total The number of echo requests that have been sent
	//      # TYPE nethealth_echo_request_total counter
	//      nethealth_echo_request_total{node_name="10.128.0.96",peer_name="10.128.0.70"} 236
	//      nethealth_echo_request_total{node_name="10.128.0.96",peer_name="10.128.0.97"} 273
	//      # HELP nethealth_echo_timeout_total The number of echo requests that have timed out
	//      # TYPE nethealth_echo_timeout_total counter
	//      nethealth_echo_timeout_total{node_name="10.128.0.96",peer_name="10.128.0.70"} 37
	//      nethealth_echo_timeout_total{node_name="10.128.0.96",peer_name="10.128.0.97"} 0
	req, err := http.NewRequestWithContext(ctx, "GET", addr+"/metrics", nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, trace.Wrap(err, "failed to fetch metrics")
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.WithError(err).Error("Failed to close response body.")
		}
	}()

	byteData, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var parser expfmt.TextParser
	metricFamilies, err := parser.TextToMetricFamilies(bytes.NewReader(byteData))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	netData, err := parseMetrics(metricFamilies)
	if err != nil {
		log.WithError(err).
			WithField("nethealth-metrics", string(byteData)).
			Error("Received incomplete set of metrics. Could be due to a bug in nethealth or a change in labels.")
		return nil, trace.Wrap(err)
	}

	return netData, nil
}

// parseMetrics parses the MetricsFamilies and returns the structured network
// data. data maps a peer to its total request counter and total timeout counter.
func parseMetrics(metricFamilies map[string]*dto.MetricFamily) (map[string]networkData, error) {
	// echoRequests maps a peer to the current running total number of requests sent to that peer.
	echoRequests, err := parseCounter(metricFamilies, echoRequestLabel)
	if err != nil {
		return nil, trace.Wrap(err, "failed to parse echo requests")
	}

	// echoTimeouts maps a peer to the current running total number of requests that timed out.
	echoTimeouts, err := parseCounter(metricFamilies, echoTimeoutLabel)
	if err != nil {
		return nil, trace.Wrap(err, "failed to parse echo timeouts")
	}

	if len(echoRequests) != len(echoTimeouts) {
		return nil, trace.BadParameter("received incomplete pair(s) of nethealth metrics. " +
			"This may be due to a bug in prometheus or in the way nethealth is exposing its metrics")
	}

	netData := make(map[string]networkData)
	for peer, requestTotal := range echoRequests {
		timeoutTotal, ok := echoTimeouts[peer]
		if !ok {
			return nil, trace.BadParameter("echo timeout data not available for %s", peer)
		}
		netData[peer] = networkData{
			requestTotal: requestTotal,
			timeoutTotal: timeoutTotal,
		}
	}

	return netData, nil
}

// parseCounter parses the provided metricFamilies and returns a map of counters
// for the desired metrics specified by label. The counters map a peer to a
// counter.
func parseCounter(metricFamilies map[string]*dto.MetricFamily, label string) (counters map[string]float64, err error) {
	metricFamily, ok := metricFamilies[label]
	if !ok {
		return nil, trace.NotFound("%s metrics not found", label)
	}

	counters = make(map[string]float64)
	for _, m := range metricFamily.GetMetric() {
		peerName, err := getPeerName(m.GetLabel())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		counters[peerName] = m.GetCounter().GetValue()
	}
	return counters, nil
}

// getPeerName extracts the 'peer_name' value from the provided labels.
func getPeerName(labels []*dto.LabelPair) (peer string, err error) {
	for _, label := range labels {
		if peerLabel == label.GetName() {
			return label.GetValue(), nil
		}
	}
	return "", trace.NotFound("unable to find %s label", peerLabel)
}

// nethealthDetail returns a failed probe detail message.
func nethealthDetail(name string) string {
	return fmt.Sprintf("overlay network communication failure with %s", name)
}

// netStats holds nethealth data for a peer.
type netStats struct {
	// packetLossCapacity specifies the max number of packet loss data points to store.
	packetLossCapacity int
	// Mutex locks access to TTLMap.
	sync.Mutex
	// TTLMap maps a peer to its peerData.
	*holster.TTLMap
}

// newNetStats constructs a new netStats.
func newNetStats(mapCapacity int, packetLossCapacity int) *netStats {
	return &netStats{
		TTLMap:             holster.NewTTLMap(mapCapacity),
		packetLossCapacity: packetLossCapacity,
	}
}

// Get returns the peerData for the specified peer.
func (r *netStats) Get(peer string) (data peerData, err error) {
	r.Lock()
	defer r.Unlock()

	value, ok := r.TTLMap.Get(peer)
	if !ok {
		return peerData{packetLoss: make([]float64, 0, r.packetLossCapacity)}, nil
	}

	data, ok = value.(peerData)
	if !ok {
		return data, trace.BadParameter("expected %T, got %T", data, value)
	}

	return data, nil
}

// Set maps the specified peer and data.
func (r *netStats) Set(peer string, data peerData) error {
	r.Lock()
	defer r.Unlock()
	return r.TTLMap.Set(peer, data, netStatsTTLSeconds)
}

// peerData keeps track of relevant nethealth data for a node.
type peerData struct {
	// prevRequestTotal keeps track of the previously recorded total number of
	// requests sent to the peer. This is necessary to calculate the number of
	// new requests since the last check.
	prevRequestTotal float64
	// prevTimeoutTotal keeps track of the previously recorded total number of
	// requests sent to the peer and timed out. This is necessary to calculate
	// the number of new timeouts since the last check.
	prevTimeoutTotal float64
	// packetLoss records the packet loss in percentages between check intervals.
	packetLoss []float64
}

// networkData contains a request and timeout counter.
type networkData struct {
	// requestTotal specifies the total number of requests sent.
	requestTotal float64
	// timeoutTotal specifies the total number of requests that timed out.
	timeoutTotal float64
}

const (
	nethealthCheckerID = "nethealth-checker"
	// peerLabel specifies the label to collect peer name from nethealth metrics.
	peerLabel = "peer_name"
	// nethealthNamespace specifies the k8s namespace that nethealth exists within.
	nethealthNamespace = "monitoring"
	// echoRequestLabel defines the metric family label for the echo request counter.
	echoRequestLabel = "nethealth_echo_request_total"
	// echoTimeoutLabel defines the metric family label for the echo timeout counter.
	echoTimeoutLabel = "nethealth_echo_timeout_total"

	// defaultNethealthPort defines the default nethealth port.
	defaultNethealthPort = 9801

	// defaultNetStatsInterval defines the default interval duration for the netStats.
	defaultNetStatsInterval = 5 * time.Minute

	// netStatsCapacity specifies the maximum number of nethealth samples to store.
	netStatsCapacity = 1000

	// netStatsTTLSeconds defines the time to live in seconds for the stored
	// netStats. This ensures the checker does not hold on to unused data when
	// a member leaves the cluster.
	netStatsTTLSeconds = 60 * 60 * 24 // 1 day

	// packetLossThreshold defines the packet loss percentage used to determine
	// if overlay network communication with a peer is unhealthy. If the packet
	// loss is consistently observed to be above this threshold over the entire
	// interval, network communication will be considered unhealthy.
	packetLossThreshold = 0.20
)

// nethealthLabelSelector defines label selector used when querying for
// nethealth pods.
var nethealthLabelSelector = mustLabelSelector(metav1.LabelSelectorAsSelector(&metav1.LabelSelector{
	MatchLabels: map[string]string{
		"k8s-app": "nethealth",
	},
}))

func mustLabelSelector(selector labels.Selector, err error) labels.Selector {
	if err != nil {
		panic(err)
	}
	return selector
}
