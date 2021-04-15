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

// Package nethealth implements a daemonset that when deployed to a kubernetes cluster, will locate and send ICMP echos
// (pings) to the nethealth pod on every other node in the cluster. This will give an indication into whether the
// overlay network is functional for pod -> pod communications, and also record packet loss on the network.
package nethealth

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"reflect"
	"sort"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	// heartbeatInterval is the duration between sending heartbeats to each peer. Any heartbeat that takes more
	// than one interval to respond will also be considered timed out.
	heartbeatInterval = 1 * time.Second

	// resyncInterval is the duration between full resyncs of local state with kubernetes. If a node is deleted it
	// may not be detected until the full resync completes.
	resyncInterval = 15 * time.Minute

	// dnsDiscoveryInterval is the duration of time for doing DNS based service discovery for pod changes. This is a
	// lightweight test for whether there is a change to the nethealth pods within the cluster.
	dnsDiscoveryInterval = 10 * time.Second

	// Default selector to use for finding nethealth pods
	DefaultSelector = "k8s-app=nethealth"

	// DefaultServiceDiscoveryQuery is the default name to query for service discovery changes
	DefaultServiceDiscoveryQuery = "any.nethealth"

	// RxQueueSize is the size of queued ping responses to process
	// Main processing occurs in a single goroutine, so we need a large enough processing queue to hold onto all ping
	// responses while the routine is working on other operations.
	// 2000 is chosen as double the maximum supported cluster size (1k)
	RxQueueSize = 2000

	// DefaultNethealthSocket is the default location of a unix domain socket that contains the prometheus metrics
	DefaultNethealthSocket = "/run/nethealth/nethealth.sock"

	// LabelNodeName specifies metrics label mapped to node name
	LabelNodeName = "node_name"
	// LabelPeerName specifies metrics label mapped to peer node name
	LabelPeerName = "peer_name"
)

const (
	// Init is peer state that we've found the node but don't know anything about it yet.
	Init = "init"
	// Up is a peer state that the peer is currently reachable
	Up = "up"
	// Timeout is a peer state that the peer is currently timing out to pings
	Timeout = "timeout"
)

type Config struct {
	// PrometheusSocket is the path to a unix socket that can be used to retrieve the prometheus metrics
	PrometheusSocket string

	// PrometheusPort is the port to bind to for serving prometheus metrics
	PrometheusPort uint32

	// Namespace is the kubernetes namespace to monitor for other nethealth instances
	Namespace string
	// NodeName is the node this instance is running on
	NodeName string
	// Selector is a kubernetes selector to find all the nethealth pods in the configured namespace
	Selector string
	// ServiceDiscoveryQuery is a DNS name that will be used for lightweight service discovery checks. A query to
	// any.<service>.default.svc.cluster.local will return a list of pods for the service. If the list of pods
	// changes we know to resync with the kubernetes API. This method uses significantly less resources than running a
	// kubernetes watcher on the API. Defaults to any.nethealth which will utilize the search path from resolv.conf.
	ServiceDiscoveryQuery string
}

// New creates a new server to ping each peer.
func (c Config) New() (*Server, error) {
	promPeerRTT := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "nethealth",
		Subsystem: "echo",
		Name:      "duration_seconds",
		Help:      "The round trip time to reach the peer",
		Buckets: []float64{
			0.0001, // 0.1 ms
			0.0002, // 0.2 ms
			0.0003, // 0.3 ms
			0.0004, // 0.4 ms
			0.0005, // 0.5 ms
			0.0006, // 0.6 ms
			0.0007, // 0.7 ms
			0.0008, // 0.8 ms
			0.0009, // 0.9 ms
			0.001,  // 1ms
			0.0015, // 1.5ms
			0.002,  // 2ms
			0.003,  // 3ms
			0.004,  // 4ms
			0.005,  // 5ms
			0.01,   // 10ms
			0.02,   // 20ms
			0.04,   // 40ms
			0.08,   // 80ms
		},
	}, []string{LabelNodeName, LabelPeerName})
	promPeerRTTSummary := prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Namespace:  "nethealth",
		Subsystem:  "echo",
		Name:       "latency_summary_milli",
		Help:       "The round trip time between peers in milliseconds",
		MaxAge:     30 * time.Second,
		AgeBuckets: 5,
		Objectives: map[float64]float64{
			0.1:  0.09, // 10th percentile
			0.2:  0.08, // ...
			0.3:  0.07,
			0.4:  0.06,
			0.5:  0.05,
			0.6:  0.04,
			0.7:  0.03,
			0.8:  0.02,
			0.9:  0.01,
			0.95: 0.005,
			0.99: 0.001, // 99th percentile
		},
	}, []string{LabelNodeName, LabelPeerName})
	promPeerTimeout := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "nethealth",
		Subsystem: "echo",
		Name:      "timeout_total",
		Help:      "The number of echo requests that have timed out",
	}, []string{LabelNodeName, LabelPeerName})
	promPeerRequest := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "nethealth",
		Subsystem: "echo",
		Name:      "request_total",
		Help:      "The number of echo requests that have been sent",
	}, []string{LabelNodeName, LabelPeerName})

	prometheus.MustRegister(
		promPeerRTT,
		promPeerRTTSummary,
		promPeerTimeout,
		promPeerRequest,
	)

	selector := DefaultSelector
	if c.Selector != "" {
		selector = c.Selector
	}

	labelSelector, err := labels.Parse(selector)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if c.ServiceDiscoveryQuery == "" {
		c.ServiceDiscoveryQuery = DefaultServiceDiscoveryQuery
	}

	return &Server{
		config:             c,
		FieldLogger:        logrus.WithField(trace.Component, "nethealth"),
		promPeerRTT:        promPeerRTT,
		promPeerRTTSummary: promPeerRTTSummary,
		promPeerTimeout:    promPeerTimeout,
		promPeerRequest:    promPeerRequest,
		selector:           labelSelector,
		triggerResync:      make(chan bool, 1),
		rxMessage:          make(chan messageWrapper, RxQueueSize),
		peers:              make(map[string]*peer),
		addrToPeer:         make(map[string]string),
	}, nil
}

// Server is an instance of nethealth that is running on each node responsible for sending and responding to heartbeats.
type Server struct {
	logrus.FieldLogger

	config     Config
	clock      clockwork.Clock
	conn       *icmp.PacketConn
	httpServer *http.Server
	selector   labels.Selector

	// rxMessage is a processing queue of received echo responses
	rxMessage     chan messageWrapper
	triggerResync chan bool

	peers      map[string]*peer
	addrToPeer map[string]string

	client kubernetes.Interface

	promPeerRTT        *prometheus.HistogramVec
	promPeerRTTSummary *prometheus.SummaryVec
	promPeerTimeout    *prometheus.CounterVec
	promPeerRequest    *prometheus.CounterVec
}

type peer struct {
	name        string
	addr        net.Addr
	echoCounter int
	echoTime    time.Time
	echoTimeout bool

	status           string
	lastStatusChange time.Time
}

type messageWrapper struct {
	message  *icmp.Message
	rxTime   time.Time
	peerAddr net.Addr
}

// Start sets up the server and begins normal operation
func (s *Server) Start() error {
	config, err := rest.InClusterConfig()
	if err != nil {
		return trace.Wrap(err)
	}
	s.client, err = kubernetes.NewForConfig(config)
	if err != nil {
		return trace.Wrap(err)
	}

	s.conn, err = icmp.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		return trace.Wrap(err)
	}

	s.clock = clockwork.NewRealClock()
	go s.loop()
	go s.loopServiceDiscovery()
	go s.serve()

	mux := http.ServeMux{}
	mux.Handle("/metrics", promhttp.Handler())
	s.httpServer = &http.Server{Addr: fmt.Sprint(":", s.config.PrometheusPort), Handler: &mux}

	// Workaround for https://github.com/gravitational/gravity/issues/2320
	// Disable keep-alives to avoid the client/server hanging unix domain sockets that don't get cleaned up.
	s.httpServer.SetKeepAlivesEnabled(false)

	go func() {
		if err := s.httpServer.ListenAndServe(); err != http.ErrServerClosed {
			s.Fatalf("ListenAndServe(): %s", err)
		}
	}()

	if s.config.PrometheusSocket != "" {
		_ = os.Remove(s.config.PrometheusSocket)

		unixListener, err := net.Listen("unix", s.config.PrometheusSocket)
		if err != nil {
			return trace.Wrap(err)
		}

		go func() {
			if err := s.httpServer.Serve(unixListener); err != http.ErrServerClosed {
				s.Fatalf("Unix Listen(): %s", err)
			}
		}()
	}

	s.Info("Started nethealth with config:")
	s.Info("  PrometheusSocket: ", s.config.PrometheusSocket)
	s.Info("  PrometheusPort: ", s.config.PrometheusPort)
	s.Info("  Namespace: ", s.config.Namespace)
	s.Info("  NodeName: ", s.config.NodeName)
	s.Info("  Selector: ", s.selector)
	s.Info("  ServiceDiscoveryQuery: ", s.config.ServiceDiscoveryQuery)

	return nil
}

// loop is the main processing loop for sending/receiving heartbeats.
func (s *Server) loop() {
	heartbeatTicker := s.clock.NewTicker(heartbeatInterval)
	defer heartbeatTicker.Stop()

	resyncTicker := s.clock.NewTicker(resyncInterval)
	defer resyncTicker.Stop()

	for {
		select {
		//
		// Re-sync cluster peers
		//
		case <-resyncTicker.Chan():
			err := s.resyncPeerList()
			if err != nil {
				s.WithError(err).Error("Unexpected error re-syncing the list of peer nodes.")
			}

			err = s.resyncNethealthPods()
			if err != nil {
				s.WithError(err).Error("Unexpected error re-syncing the list of peer pods.")
			}
		case <-s.triggerResync:
			err := s.resyncPeerList()
			if err != nil {
				s.WithError(err).Error("Unexpected error re-syncing the list of peer nodes.")
			}

			err = s.resyncNethealthPods()
			if err != nil {
				s.WithError(err).Error("Unexpected error re-syncing the list of peer pods.")
			}

		//
		// Send a heartbeat to each peer we know about
		// Check for peers that are timing out / down
		//
		case <-heartbeatTicker.Chan():
			s.checkTimeouts()
			for _, peer := range s.peers {
				s.sendHeartbeat(peer)
			}

		//
		// Rx heartbeats responses from peers
		//
		case rx := <-s.rxMessage:
			err := s.processAck(rx)
			if err != nil {
				s.WithFields(logrus.Fields{
					logrus.ErrorKey: err,
					"peer_addr":     rx.peerAddr,
					"rx_time":       rx.rxTime,
					"message":       rx.message,
				}).Error("Error processing icmp message.")
			}
		}
	}
}

// loopServiceDiscovery uses cluster-dns service discovery as a lightweight check for pod changes
// and will trigger a resync if the cluster DNS service discovery changes
func (s *Server) loopServiceDiscovery() {
	s.Info("Starting DNS service discovery for nethealth pod.")
	ticker := s.clock.NewTicker(dnsDiscoveryInterval)
	defer ticker.Stop()
	query := s.config.ServiceDiscoveryQuery

	previousNames := []string{}

	for {
		<-ticker.Chan()

		s.Debugf("Querying %v for service discovery", query)
		names, err := net.LookupHost(query)
		if err != nil {
			s.WithError(err).WithField("query", query).Error("Error querying service discovery.")
			continue
		}

		sort.Strings(names)
		if reflect.DeepEqual(names, previousNames) {
			continue
		}
		previousNames = names
		s.Info("Triggering peer resync due to service discovery change")

		select {
		case s.triggerResync <- true:
		default:
			// Don't block
		}
	}
}

// resyncPeerList contacts the kubernetes API to sync the list of kubernetes nodes
func (s *Server) resyncPeerList() error {
	nodes, err := s.client.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return trace.Wrap(err)
	}

	peerMap := make(map[string]bool)
	for _, node := range nodes.Items {
		// Don't add our own node as a peer
		if node.Name == s.config.NodeName {
			continue
		}

		peerMap[node.Name] = true
		if _, ok := s.peers[node.Name]; !ok {
			s.peers[node.Name] = &peer{
				name:             node.Name,
				lastStatusChange: s.clock.Now(),
				addr:             &net.IPAddr{},
			}
			s.WithField("peer", node.Name).Info("Adding peer.")
			// Initialize the peer so it shows up in prometheus with a 0 count
			s.promPeerTimeout.WithLabelValues(s.config.NodeName, node.Name).Add(0)
			s.promPeerRequest.WithLabelValues(s.config.NodeName, node.Name).Add(0)
		}
	}

	// check for peers that have been deleted
	for key := range s.peers {
		if _, ok := peerMap[key]; !ok {
			s.WithField("peer", key).Info("Deleting peer.")
			delete(s.peers, key)
			s.promPeerRTT.DeleteLabelValues(s.config.NodeName, key)
			s.promPeerRTTSummary.DeleteLabelValues(s.config.NodeName, key)
			s.promPeerRequest.DeleteLabelValues(s.config.NodeName, key)
			s.promPeerTimeout.DeleteLabelValues(s.config.NodeName, key)
		}
	}

	return nil
}

// resyncNethealthPods contacts the kubernetes API to sync the list of pods running the nethealth daemon
func (s *Server) resyncNethealthPods() error {
	list, err := s.client.CoreV1().Pods(s.config.Namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: s.selector.String(),
	})
	if err != nil {
		return trace.Wrap(err)
	}

	for _, pod := range list.Items {
		// skip our own pod
		if pod.Spec.NodeName == s.config.NodeName {
			continue
		}

		// skip if the peer object can't be located
		if peer, ok := s.peers[pod.Spec.NodeName]; !ok {
			continue
		} else {
			newAddr := &net.IPAddr{
				IP: net.ParseIP(pod.Status.PodIP),
			}

			if peer.addr.String() != newAddr.String() {
				s.WithFields(logrus.Fields{
					"peer":          peer.name,
					"new_peer_addr": newAddr,
					"old_peer_addr": peer.addr,
				}).Info("Updating peer pod IP address.")
				peer.addr = newAddr
				s.addrToPeer[peer.addr.String()] = pod.Spec.NodeName
			}
		}
	}

	// Free entries in the lookup table that no longer point to a valid object
	for key, value := range s.addrToPeer {
		if _, ok := s.peers[value]; !ok {
			delete(s.addrToPeer, key)
		}
	}

	return nil
}

// serve monitors for incoming icmp messages
func (s *Server) serve() {
	buf := make([]byte, 256)

	for {
		n, peerAddr, err := s.conn.ReadFrom(buf)
		rxTime := s.clock.Now()
		log := s.WithFields(logrus.Fields{
			"peer_addr": peerAddr,
			"node":      s.config.NodeName,
			"length":    n,
		})
		if err != nil {
			log.WithError(err).Error("Error in udp socket read.")
			continue
		}

		// The ICMP package doesn't export the protocol numbers
		// 1 - ICMP
		// 58 - ICMPv6
		// https://www.iana.org/assignments/protocol-numbers/protocol-numbers.xhtml
		msg, err := icmp.ParseMessage(1, buf[:n])
		if err != nil {
			log.WithError(err).Error("Error parsing icmp message.")
			continue
		}

		select {
		case s.rxMessage <- messageWrapper{
			message:  msg,
			rxTime:   rxTime,
			peerAddr: peerAddr,
		}:
		default:
			// Don't block
			log.Warn("Dropped icmp message due to full rxMessage queue")
		}
	}
}

func (s *Server) lookupPeer(addr string) (*peer, error) {
	peerName, ok := s.addrToPeer[addr]
	if !ok {
		return nil, trace.BadParameter("address not found in address table").AddField("address", addr)
	}

	p, ok := s.peers[peerName]
	if !ok {
		return nil, trace.BadParameter("peer not found in peer table").AddField("peer_name", peerName)
	}
	return p, nil
}

// processAck processes a received ICMP Ack message
func (s *Server) processAck(e messageWrapper) error {
	switch e.message.Type {
	case ipv4.ICMPTypeEchoReply:
		// ok
	case ipv4.ICMPTypeEcho:
		// nothing to do with echo requests
		return nil
	default:
		//unexpected / unknown
		return trace.BadParameter("received unexpected icmp message type").AddField("type", e.message.Type)
	}

	switch pkt := e.message.Body.(type) {
	case *icmp.Echo:
		peer, err := s.lookupPeer(e.peerAddr.String())
		if err != nil {
			return trace.Wrap(err)
		}
		if uint16(pkt.Seq) != uint16(peer.echoCounter) {
			return trace.BadParameter("response sequence doesn't match latest request.").
				AddField("expected", uint16(peer.echoCounter)).
				AddField("received", uint16(pkt.Seq))
		}

		rtt := e.rxTime.Sub(peer.echoTime)
		s.promPeerRTT.WithLabelValues(s.config.NodeName, peer.name).Observe(rtt.Seconds())
		s.promPeerRTTSummary.WithLabelValues(s.config.NodeName, peer.name).Observe(float64(rtt.Milliseconds()))
		s.updatePeerStatus(peer, Up)
		peer.echoTimeout = false

		s.WithFields(logrus.Fields{
			"peer_name": peer.name,
			"peer_addr": peer.addr,
			"counter":   peer.echoCounter,
			"seq":       uint16(peer.echoCounter),
			"rtt":       rtt,
		}).Debug("Ack.")
	default:
		s.WithFields(logrus.Fields{
			"peer_addr": e.peerAddr.String(),
		}).Warn("Unexpected icmp message")
	}
	return nil
}

func (s *Server) sendHeartbeat(peer *peer) {
	peer.echoCounter++
	log := s.WithFields(logrus.Fields{
		"peer_name": peer.name,
		"peer_addr": peer.addr,
		"id":        peer.echoCounter,
	})

	// If we don't know the pod IP address of the peer, we still want to generate a timeout, but not actually send
	// a heartbeat
	peer.echoTimeout = true
	if peer.addr == nil || peer.addr.String() == "" || peer.addr.String() == "0.0.0.0" {
		return
	}

	msg := icmp.Message{
		Type: ipv4.ICMPTypeEcho,
		Code: 0,
		Body: &icmp.Echo{
			ID:  1,
			Seq: peer.echoCounter,
		},
	}
	buf, err := msg.Marshal(nil)
	if err != nil {
		log.WithError(err).Warn("Failed to marshal ping.")
		return
	}

	peer.echoTime = s.clock.Now()
	_, err = s.conn.WriteTo(buf, peer.addr)
	if err != nil {
		log.WithError(err).Warn("Failed to send ping.")
		return
	}
	s.promPeerRequest.WithLabelValues(s.config.NodeName, peer.name).Inc()

	log.Debug("Sent echo request.")
}

// checkTimeouts iterates over each peer, and checks whether our last heartbeat has timed out
func (s *Server) checkTimeouts() {
	s.Debug("checking for timeouts")
	for _, peer := range s.peers {
		// if the echoTimeout flag is set, it means we didn't receive a response to our last request
		if peer.echoTimeout {
			s.WithFields(logrus.Fields{
				"peer_name": peer.name,
				"peer_addr": peer.addr,
				"id":        peer.echoCounter,
			}).Debug("echo timeout")
			s.promPeerTimeout.WithLabelValues(s.config.NodeName, peer.name).Inc()
			s.updatePeerStatus(peer, Timeout)
		}
	}
}

func (s *Server) updatePeerStatus(peer *peer, status string) {
	if peer.status == status {
		return
	}

	s.WithFields(logrus.Fields{
		"peer_name":  peer.name,
		"peer_addr":  peer.addr,
		"duration":   s.clock.Now().Sub(peer.lastStatusChange),
		"old_status": peer.status,
		"new_status": status,
	}).Info("Peer status changed.")

	peer.status = status
	peer.lastStatusChange = s.clock.Now()

}
