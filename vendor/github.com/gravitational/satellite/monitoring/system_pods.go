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
	"context"
	"fmt"

	"github.com/gravitational/satellite/agent/health"
	pb "github.com/gravitational/satellite/agent/proto/agentpb"
	"github.com/gravitational/satellite/utils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
)

// SystemPodsConfig specifies configuration for a system pods checker.
type SystemPodsConfig struct {
	// NodeName specifies the kubernetes name of this node.
	NodeName string
	// KubeConfig specifies kubernetes access configuration.
	*KubeConfig
	// Namespaces specifies the list of namespaces to query for critical pods.
	Namespaces []string
}

// checkAndSetDefaults validates that this configuration is correct and sets
// value defaults where necessary.
func (r *SystemPodsConfig) checkAndSetDefaults() error {
	var errors []error
	if r.NodeName == "" {
		errors = append(errors, trace.BadParameter("node name must be provided"))
	}
	if r.KubeConfig == nil {
		errors = append(errors, trace.BadParameter("kubernetes access config must be provided"))
	}
	return trace.NewAggregate(errors...)
}

// systemPodsChecker verifies system pods are operational.
type systemPodsChecker struct {
	// SystemPodsConfig specifies checker configuration values.
	SystemPodsConfig
}

// NewSystemPodsChecker returns a new system pods checker.
func NewSystemPodsChecker(config SystemPodsConfig) (*systemPodsChecker, error) {
	if err := config.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &systemPodsChecker{
		SystemPodsConfig: config,
	}, nil
}

// Name returns this checker name
// Implements health.Checker
func (r *systemPodsChecker) Name() string {
	return systemPodsCheckerID
}

// Check verifies that all system pods are operational.
// Implements health.Checker
func (r *systemPodsChecker) Check(ctx context.Context, reporter health.Reporter) {
	if err := r.check(ctx, reporter); err != nil {
		log.WithError(err).Debug("Failed to verify critical system pods.")
		return
	}
	if reporter.NumProbes() == 0 {
		reporter.Add(NewSuccessProbe(r.Name()))
	}
}

func (r *systemPodsChecker) check(ctx context.Context, reporter health.Reporter) error {
	pods, err := r.getPods()
	if trace.IsNotFound(err) {
		log.Debug("No critical system pods found.")
		return nil // system pods were not found, log and treat gracefully
	}
	if err != nil {
		return trace.Wrap(err)
	}

	r.verifyPods(pods, reporter)
	return nil
}

// getPods returns a list of the local pods that have the
// `gravitational.io/critical-pod` label.
func (r *systemPodsChecker) getPods() (pods []corev1.Pod, err error) {
	opts := metav1.ListOptions{
		LabelSelector: systemPodsSelector.String(),
		FieldSelector: fields.OneTermEqualSelector("spec.nodeName", r.NodeName).String(),
	}

	for _, namespace := range r.Namespaces {
		podList, err := r.Client.CoreV1().Pods(namespace).List(opts)
		if err != nil {
			return pods, utils.ConvertError(err)
		}
		pods = append(pods, podList.Items...)
	}

	return pods, nil
}

// verifyPods verifies the pods are in a valid state. Reports a failed probe for
// each failed pod.
// https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/#pod-phase
func (r *systemPodsChecker) verifyPods(pods []corev1.Pod, reporter health.Reporter) {
	for _, pod := range pods {
		switch pod.Status.Phase {

		// PodSucceeded indicates that all containers terminated without an error.
		case corev1.PodSucceeded:
			continue

		// PodRunning usually indicates the pod is healthy, but the containers
		// need to be further verified to check if any containers are in a
		// CrashLoopBackOff state.
		case corev1.PodRunning:
			if err := verifyContainers(pod.Status.ContainerStatuses); err != nil {
				reporter.Add(systemPodsFailureProbe(r.Name(), pod.Namespace, pod.Name, err))
			}

		// PodPending indicates that some containers are still in a waiting state.
		// All initContainers and containers need to be verified to ensure that
		// a container is not stuck in an unhealthy state.
		case corev1.PodPending:
			var err error

			// If the pod has already been initialized, skip initContainer
			// verification and only verify containers.
			if isInitialized(pod.Status.Conditions) {
				err = verifyContainers(pod.Status.ContainerStatuses)
			} else {
				err = verifyContainers(pod.Status.InitContainerStatuses)
			}
			if err != nil {
				reporter.Add(systemPodsFailureProbe(r.Name(), pod.Namespace, pod.Name, err))
			}

		// PodFailed indicates that a container terminated with an error.
		case corev1.PodFailed:
			err := trace.BadParameter("pod failed: %v", pod.Status.Reason)
			reporter.Add(systemPodsFailureProbe(r.Name(), pod.Namespace, pod.Name, err))

		// Log any unexpected pod phases and contiune.
		default:
			log.WithField("pod", pod.Name).
				WithField("namespace", pod.Namespace).
				WithField("phase", pod.Status.Phase).
				Debug("Pod is in an unknown phase.")
		}
	}
}

// isInitialized returns true if the pod's `Initialized` condition is `True`.
func isInitialized(conditions []corev1.PodCondition) bool {
	for _, condition := range conditions {
		if condition.Type == corev1.PodInitialized && condition.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}

// verifyContainers verifies all containers are in a healthy state.
// Returns an error with state and reason if a container is in an unhealthy state.
func verifyContainers(containerStatuses []corev1.ContainerStatus) error {
	for _, status := range containerStatuses {
		if status.State.Waiting != nil {
			reason := status.State.Waiting.Reason
			if reason == errImagePullBackOff || reason == errCrashLoopBackOff || reason == errImagePull {
				return trace.BadParameter("%v waiting: %v", status.Name, reason)
			}
		}
		if status.State.Terminated != nil {
			if status.State.Terminated.ExitCode != 0 {
				return trace.BadParameter("%v terminated: %v", status.Name, status.State.Terminated.Reason)
			}
		}
	}
	return nil
}

// systemPodsFailureProbe constructs a probe that represents a failed system pods
// check for the pod specified by podName and namespace.
func systemPodsFailureProbe(checkerName, namespace, podName string, err error) *pb.Probe {
	return &pb.Probe{
		Checker:  checkerName,
		Detail:   fmt.Sprintf("pod %v/%v is not running", namespace, podName),
		Error:    trace.UserMessage(err),
		Status:   pb.Probe_Failed,
		Severity: pb.Probe_Warning, // TODO: set probe to critical
	}
}

const systemPodsCheckerID = "system-pods-checker"
const systemPodKey = "gravitational.io/critical-pod"

// NOTE: Currently, there is limited documentation on k8s container state.
// Information on container state can be found in kubelet pkg:
// https://github.com/kubernetes/kubernetes/tree/master/pkg/kubelet

// These states usually do not indicate an unhealthy pod.
const (
	// podInitializing state indicates that the container is waiting on an initContainer to terminate.
	podInitializing = "PodInitializing"
	// containerCreating state indicates that the container is being created.
	containerCreating = "ContainerCreating"
	// containerCompleted state indicates that the container has terminated without error.
	containerCompleted = "Completed"
)

// These states are indicative of a unhealthy pod.
const (
	// errCrashLoopBackOff state indicates that the container is in a crash loop.
	errCrashLoopBackOff = "CrashLoopBackOff"
	// errImagePullBackOff state indicates that the container image pull failed.
	errImagePullBackOff = "ImagePullBackOff"
	// errImagePull state indicates that the container image pull failed. General image pull error.
	errImagePull = "ErrImagePull"
)

// containerError state indicates that the container terminated with an error.
const containerError = "Error"

// systemPodsSelector defines a label selector used to query critical system pods.
var systemPodsSelector = utils.MustLabelSelector(
	metav1.LabelSelectorAsSelector(
		&metav1.LabelSelector{
			MatchExpressions: []metav1.LabelSelectorRequirement{
				{Key: systemPodKey, Operator: metav1.LabelSelectorOpExists},
			}}))
