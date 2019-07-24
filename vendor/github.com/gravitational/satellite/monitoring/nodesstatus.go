/*
Copyright 2017-2019 Gravitational, Inc.

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
	"time"

	humanize "github.com/dustin/go-humanize"
	"github.com/gravitational/satellite/agent/health"
	pb "github.com/gravitational/satellite/agent/proto/agentpb"
	"github.com/gravitational/satellite/utils"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/trace"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

// NewNodesStatusChecker returns a Checker that tests kubernetes nodes availability
func NewNodesStatusChecker(config KubeConfig, nodesReadyThreshold int) health.Checker {
	return &nodesStatusChecker{
		client:              config.Client.CoreV1(),
		nodesReadyThreshold: nodesReadyThreshold,
	}
}

// nodesStatusChecker tests and reports health failures in kubernetes
// nodes availability
type nodesStatusChecker struct {
	client              corev1.CoreV1Interface
	nodesReadyThreshold int
}

// Name returns the name of this checker
func (r *nodesStatusChecker) Name() string { return NodesStatusCheckerID }

// Check validates the status of kubernetes components
func (r *nodesStatusChecker) Check(ctx context.Context, reporter health.Reporter) {
	listOptions := metav1.ListOptions{
		LabelSelector: labels.Everything().String(),
		FieldSelector: fields.Everything().String(),
	}
	statuses, err := r.client.Nodes().List(listOptions)
	if err != nil {
		reason := "failed to query nodes"
		reporter.Add(NewProbeFromErr(r.Name(), reason, trace.Wrap(err)))
		return
	}
	var nodesReady int
	for _, item := range statuses.Items {
		for _, condition := range item.Status.Conditions {
			if condition.Type != v1.NodeReady {
				continue
			}
			if condition.Status == v1.ConditionTrue {
				nodesReady++
				continue
			}
		}
	}

	if nodesReady < r.nodesReadyThreshold {
		reporter.Add(&pb.Probe{
			Checker: r.Name(),
			Status:  pb.Probe_Failed,
			Error: fmt.Sprintf("Not enough ready nodes: %v (threshold %v)",
				nodesReady, r.nodesReadyThreshold),
		})
	} else {
		reporter.Add(&pb.Probe{
			Checker: r.Name(),
			Status:  pb.Probe_Running,
		})
	}
}

// NodeStatusCheckerConfig is the Kubernetes node status checker configuration.
type NodeStatusCheckerConfig struct {
	// KubeConfig provides Kubernetes access.
	KubeConfig
	// NodeName is the Kubernetes node name.
	NodeName string
	// Conditions is a list of Kubernetes node conditions to monitor.
	Conditions []v1.NodeConditionType
	// Events is a list of Kubernetes node events to monitor.
	Events []string
	// EventsAge is the maximum age of monitored events to display.
	EventsAge time.Duration
	// Clock is used in tests to mock time.
	Clock clockwork.Clock
}

// SetDefaults sets default values on the config.
func (c *NodeStatusCheckerConfig) SetDefaults() {
	if len(c.Conditions) == 0 {
		c.Conditions = NodeConditions
	}
	if len(c.Events) == 0 {
		c.Events = NodeEvents
	}
	if c.EventsAge == 0 {
		c.EventsAge = MaxEventsAge
	}
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
	}
}

// NewNodeStatusChecker returns a Checker that validates availability
// of a single Kubernetes node.
func NewNodeStatusChecker(config NodeStatusCheckerConfig) health.Checker {
	config.SetDefaults()
	nodeLister := kubeNodeLister{client: config.Client.CoreV1()}
	conditions := make([]string, 0, len(config.Conditions))
	for _, condition := range config.Conditions {
		conditions = append(conditions, string(condition))
	}
	return &nodeStatusChecker{
		nodeLister: nodeLister,
		nodeName:   config.NodeName,
		conditions: conditions,
		events:     config.Events,
		eventsAge:  config.EventsAge,
		clock:      config.Clock,
	}
}

// nodeStatusChecker is a Checker that validates availability
// of a single Kubernetes node.
type nodeStatusChecker struct {
	nodeLister
	nodeName   string
	conditions []string
	events     []string
	eventsAge  time.Duration
	clock      clockwork.Clock
}

// Name returns the name of this checker
func (r *nodeStatusChecker) Name() string { return NodeStatusCheckerID }

// Check validates the status of kubernetes components
func (r *nodeStatusChecker) Check(ctx context.Context, reporter health.Reporter) {
	node, err := r.queryNode()
	if err != nil {
		reporter.Add(NewProbeFromErr(r.Name(), trace.UserMessage(err), err))
		return
	}

	events, err := r.queryNodeEvents()
	if err != nil {
		reporter.Add(NewProbeFromErr(r.Name(), trace.UserMessage(err), err))
		return
	}

	var failureConditions []v1.NodeCondition
	for _, condition := range node.Status.Conditions {
		if r.isNotReadyCondition(condition) || r.isFailureCondition(condition) {
			failureConditions = append(failureConditions, condition)
		}
	}

	var failureEvents []v1.Event
	for _, event := range events {
		if r.isFailureEvent(event) {
			failureEvents = append(failureEvents, event)
		}
	}

	if len(failureConditions)+len(failureEvents) == 0 {
		reporter.Add(&pb.Probe{
			Checker: r.Name(),
			Status:  pb.Probe_Running,
		})
		return
	}

	for _, condition := range failureConditions {
		reporter.Add(r.probeForCondition(condition))
	}

	for _, event := range failureEvents {
		reporter.Add(r.probeForEvent(event))
	}
}

// queryNode returns Kubernetes node for the checker's node.
func (r *nodeStatusChecker) queryNode() (*v1.Node, error) {
	options := metav1.ListOptions{
		LabelSelector: labels.Everything().String(),
		FieldSelector: fields.SelectorFromSet(fields.Set{
			"metadata.name": r.nodeName,
		}).String(),
	}
	nodes, err := r.nodeLister.Nodes(options)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(nodes.Items) != 1 {
		return nil, trace.NotFound("node %q not found", r.nodeName)
	}
	return &nodes.Items[0], nil
}

// queryNodeEvents returns Kubernetes events for the checker's node.
func (r *nodeStatusChecker) queryNodeEvents() ([]v1.Event, error) {
	options := metav1.ListOptions{
		LabelSelector: labels.Everything().String(),
		FieldSelector: fields.SelectorFromSet(fields.Set{
			"involvedObject.kind": "Node",
			"involvedObject.name": r.nodeName,
		}).String(),
	}
	events, err := r.nodeLister.Events(options)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return events.Items, nil
}

// isNotReadyCondition returns true if the provided node condition indicates
// that the node is not ready.
func (r *nodeStatusChecker) isNotReadyCondition(condition v1.NodeCondition) bool {
	return condition.Type == v1.NodeReady &&
		condition.Status != v1.ConditionTrue
}

// isFailureCondition returns true if the provided condition is one of
// conditions monitored by this checker and is triggered for the node.
func (r *nodeStatusChecker) isFailureCondition(condition v1.NodeCondition) bool {
	return utils.StringInSlice(r.conditions, string(condition.Type)) &&
		condition.Status == v1.ConditionTrue
}

// isFailureEvent returns true if the provided event is one of events
// monitored by this checker and is not too old.
func (r *nodeStatusChecker) isFailureEvent(event v1.Event) bool {
	return utils.StringInSlice(r.events, event.Reason) &&
		r.clock.Now().Sub(event.LastTimestamp.Time) < r.eventsAge
}

// probeForCondition returns failure probe for the provided condition.
func (r *nodeStatusChecker) probeForCondition(condition v1.NodeCondition) *pb.Probe {
	return &pb.Probe{
		Checker:  r.Name(),
		Status:   pb.Probe_Failed,
		Severity: pb.Probe_Warning,
		Detail:   fmt.Sprintf("%v/%v", condition.Type, condition.Reason),
		Error:    condition.Message,
	}
}

// probeForEvent returns failure probe for the provided event.
func (r *nodeStatusChecker) probeForEvent(event v1.Event) *pb.Probe {
	return &pb.Probe{
		Checker:  r.Name(),
		Status:   pb.Probe_Failed,
		Severity: pb.Probe_Info, // Events are informational for now.
		Detail: fmt.Sprintf("%v (%v)", event.Reason, humanize.RelTime(
			event.LastTimestamp.Time, r.clock.Now(), "ago", "")),
		Error: event.Message,
	}
}

type nodeLister interface {
	Nodes(metav1.ListOptions) (*v1.NodeList, error)
	Events(metav1.ListOptions) (*v1.EventList, error)
}

func (r kubeNodeLister) Nodes(options metav1.ListOptions) (*v1.NodeList, error) {
	nodes, err := r.client.Nodes().List(options)
	if err != nil {
		return nil, trace.Wrap(err, "failed to query nodes")
	}
	return nodes, nil
}

func (r kubeNodeLister) Events(options metav1.ListOptions) (*v1.EventList, error) {
	events, err := r.client.Events(v1.NamespaceDefault).List(options)
	if err != nil {
		return nil, trace.Wrap(err, "failed to query node events")
	}
	return events, nil
}

type kubeNodeLister struct {
	client corev1.CoreV1Interface
}

const (
	// NodeStatusCheckerID identifies the checker that detects whether a node is not ready
	NodeStatusCheckerID = "node-status"
	// NodesStatusCheckerID identifies the checker that validates node availability in a cluster
	NodesStatusCheckerID = "nodes-status"
)

var (
	// NodeConditions defines default Kubernetes node conditions to monitor.
	//
	// It includes both default Kubernetes conditions, as well as those
	// commonly used by Kubernetes Node Problem Detector.
	NodeConditions = []v1.NodeConditionType{
		v1.NodeOutOfDisk,
		v1.NodeMemoryPressure,
		v1.NodeDiskPressure,
		v1.NodePIDPressure,
		v1.NodeNetworkUnavailable,
		NodeKernelDeadlock,
		NodeReadonlyFilesystem,
		NodeCorruptDockerOverlay2,
		NodeFrequentUnregisterNetDevice,
		NodeFrequentKubeletRestart,
		NodeFrequentDockerRestart,
		NodeFrequentContainerdRestart,
	}
	// NodeEvents defines Kubernetes node events to monitor.
	//
	// It primarily includes events fired by Node Problem Detector.
	NodeEvents = []string{
		EventOOMKilling,
		EventTaskHung,
		EventUnregisterNetDevice,
		EventKernelOops,
		EventCorruptDockerImage,
	}
)

const (
	// NodeKernelDeadlock is set by Node Problem Detector when it detects
	// a deadlock in the kernel.
	NodeKernelDeadlock v1.NodeConditionType = "KernelDeadlock"
	// NodeReadonlyFilesystem is set by Node Problem Detector when it
	// detects a readonly filesystem.
	NodeReadonlyFilesystem v1.NodeConditionType = "ReadonlyFilesystem"
	// NodeCorruptDockerOverlay2 is set by Node Problem Detector when it
	// detects corruption in the Docker overlay2 data directory.
	NodeCorruptDockerOverlay2 v1.NodeConditionType = "CorruptDockerOverlay2"
	// NodeFrequentUnregisterNetDevice is set by Node Problem Detector
	// when it detects a kernel crash that may lead to Docker issues.
	NodeFrequentUnregisterNetDevice v1.NodeConditionType = "FrequentUnregisterNetDevice"
	// NodeFrequentKubeletRestart is set by Node Problem Detector when
	// it detects frequent Kubelet restarts.
	NodeFrequentKubeletRestart v1.NodeConditionType = "FrequentKubeletRestart"
	// NodeFrequentDockerRestart is set by Node Problem Detector when
	// it detects frequent Docker restarts.
	NodeFrequentDockerRestart v1.NodeConditionType = "FrequentDockerRestart"
	// NodeFrequentContainerdRestarts is set by Node Problem Detector
	// when it detects frequent Containerd restarts.
	NodeFrequentContainerdRestart v1.NodeConditionType = "FrequentContainerdRestart"
)

const (
	// MaxEventsAge is the default maximum age of events displayed by
	// Satellite, to prevent displaying old events.
	MaxEventsAge = 5 * time.Minute // 24 * time.Hour
	// EventOOMKilling is fired by Node Problem Detector when it detects
	// that a process was killed by OOM killer.
	EventOOMKilling = "OOMKilling"
	// EventTaskHung is fired by Node Problem Detector when it detects
	// that a certain process has been blocked for a long time.
	EventTaskHung = "TaskHung"
	// EventUnregisterNetDevice is fired by Node Problem Detector when
	// it detects a kernel crash that may lead to Docker failure.
	EventUnregisterNetDevice = "UnregisterNetDevice"
	// EventKernelOops is fired by Node Problem Detector when it detects
	// a kernel crash.
	EventKernelOops = "KernelOops"
	// EventCorruptDockerImage is fired by Node Problem detector when
	// it detects a corrupted Docker image.
	EventCorruptDockerImage = "CorruptDockerImage"
)
