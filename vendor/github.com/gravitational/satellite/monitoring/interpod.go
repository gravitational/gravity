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
	"strings"
	"time"
	"unicode"

	"github.com/gravitational/satellite/agent/health"
	"github.com/gravitational/satellite/lib/kubernetes"

	"github.com/blang/semver"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/discovery"
	kube "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// This file implements a functional pod communication test.
// It has been adopted from https://github.com/kubernetes/kubernetes/blob/master/test/e2e/networking.go

// testNamespace is the name of the namespace used for functional k8s tests.
const testNamespace = "planet-test"

// serviceNamePrefix is the prefix used to name test pods.
const serviceNamePrefix = "nettest-"

// interPodChecker is a Checker that runs a networking test in the cluster
// by scheduling pods and verifying communication.
type interPodChecker struct {
	*KubeChecker
	nettestContainerImage string
}

// NewInterPodChecker returns an instance of interPodChecker.
func NewInterPodChecker(masterURL, nettestContainerImage string) health.Checker {
	checker := &interPodChecker{
		nettestContainerImage: nettestContainerImage,
	}
	kubeChecker := &KubeChecker{
		name:      "networking",
		masterURL: masterURL,
		checker:   checker.testInterPodCommunication,
	}
	checker.KubeChecker = kubeChecker
	return kubeChecker
}

// testInterPodCommunication implements the inter-pod communication test.
func (r *interPodChecker) testInterPodCommunication(ctx context.Context, client *kube.Clientset) error {
	serviceName := generateName(serviceNamePrefix)
	if err := createNamespaceIfNeeded(client, testNamespace); err != nil {
		return trace.Wrap(err, "failed to create namespace `%v`", testNamespace)
	}

	const shouldWait = true
	const userName = "default"

	if _, err := getServiceAccount(client, testNamespace, userName, shouldWait); err != nil {
		return trace.Wrap(err, "service account has not yet been created - test postponed")
	}

	svc, err := client.Core().Services(testNamespace).Create(&v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: serviceName,
			Labels: map[string]string{
				"name": serviceName,
			},
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{{
				Protocol:   "TCP",
				Port:       8080,
				TargetPort: intstr.FromInt(8080),
			}},
			Selector: map[string]string{
				"name": serviceName,
			},
		},
	})
	if err != nil {
		return trace.Wrap(err, "failed to create test service %q", serviceName)
	}

	cleanupService := func() {
		if err = client.Core().Services(testNamespace).Delete(svc.Name, &metav1.DeleteOptions{}); err != nil {
			log.Infof("failed to delete service %q: %v", svc.Name, err)
		}
	}
	defer cleanupService()

	nodes, err := waitForAllNodesSchedulable(ctx, client)
	if err != nil {
		return trace.Wrap(err, "failed to wait for all nodes to become schedulable")
	}

	if len(nodes.Items) < 2 {
		return trace.Errorf("expected at least 2 ready nodes - got %d (%v)", len(nodes.Items), nodes.Items)
	}

	podNames, err := launchNetTestPodPerNode(client, nodes, serviceName, r.nettestContainerImage, testNamespace)
	if err != nil {
		return trace.Wrap(err, "failed to start `nettest` pod")
	}

	cleanupPods := func() {
		for _, podName := range podNames {
			if err = client.Core().Pods(testNamespace).Delete(podName, nil); err != nil {
				log.Infof("failed to delete pod %q: %v", podName, err)
			}
		}
	}
	defer cleanupPods()

	for _, podName := range podNames {
		err = waitTimeoutForPodRunningInNamespace(client, podName, testNamespace, podStartTimeout)
		if err != nil {
			return trace.Wrap(err, "pod %q failed to transition to Running state", podName)
		}
	}

	passed := false

	getDetail := func(detail string) ([]byte, error) {
		proxyRequest, errProxy := getServicesProxyRequest(client, client.DiscoveryClient.RESTClient().Get())
		if errProxy != nil {
			return nil, trace.Wrap(errProxy)
		}
		return proxyRequest.Namespace(testNamespace).
			Name(svc.Name).
			Suffix(detail).
			DoRaw()
	}

	getDetails := func() ([]byte, error) { return getDetail("read") }
	getStatus := func() ([]byte, error) { return getDetail("status") }

	var body []byte
	timeout := time.Now().Add(2 * time.Minute)
	for i := 0; !passed && timeout.After(time.Now()); i++ {
		time.Sleep(pollInterval)
		body, err = getStatus()
		if err != nil {
			log.Infof("attempt %v: service/pod still starting: %v)", i, err)
			continue
		}
		// validate if the container was able to find peers
		switch {
		case string(body) == "pass":
			passed = true
		case string(body) == "running":
			log.Debugf("attempt %v: test still running", i)
		case string(body) == "fail":
			if body, err = getDetails(); err != nil {
				return trace.Wrap(err, "failed to read test details")
			} else {
				return trace.Wrap(err, "containers failed to find peers")
			}
		case strings.Contains(string(body), "no endpoints available"):
			log.Debugf("attempt %v: waiting on service/endpoints", i)
		default:
			return trace.Errorf("unexpected response: [%s]", body)
		}

		select {
		case <-ctx.Done():
			return trace.ConnectionProblem(nil, "test timed out")
		default:
		}
	}

	if !passed {
		if body, err = getDetails(); err != nil {
			return trace.Wrap(err, "test timed out")
		} else {
			return trace.Errorf("test timed out:\n%s", string(body))
		}
	}
	return nil
}

// podStartTimeout defines the amount of time to wait for a pod to start.
const podStartTimeout = 15 * time.Second

// pollInterval defines the amount of time to wait between attempts to poll pods/nodes.
const pollInterval = 2 * time.Second

// podCondition is an interface to verify the specific pod condition.
type podCondition func(pod *v1.Pod) (bool, error)

// waitTimeoutForPodRunningInNamespace waits for a pod in the specified namespace
// to transition to 'Running' state within the specified amount of time.
func waitTimeoutForPodRunningInNamespace(client *kube.Clientset, podName string, namespace string, timeout time.Duration) error {
	return waitForPodCondition(client, namespace, podName, "running", timeout, func(pod *v1.Pod) (bool, error) {
		if pod.Status.Phase == v1.PodRunning {
			log.Infof("found pod '%s' on node '%s'", podName, pod.Spec.NodeName)
			return true, nil
		}
		if pod.Status.Phase == v1.PodFailed {
			return true, trace.Errorf("pod in failed status: %s", fmt.Sprintf("%#v", pod))
		}
		return false, nil
	})
}

// waitForPodCondition waits until a pod is in the given condition within the specified amount of time.
func waitForPodCondition(client *kube.Clientset, ns, podName, desc string, timeout time.Duration, condition podCondition) error {
	log.Infof("waiting up to %v for pod %s status to be %s", timeout, podName, desc)
	for start := time.Now(); time.Since(start) < timeout; time.Sleep(pollInterval) {
		pod, err := client.Core().Pods(ns).Get(podName, metav1.GetOptions{})
		if err != nil {
			log.Infof("get pod %s in namespace '%s' failed, ignoring for %v: %v",
				podName, ns, pollInterval, err)
			continue
		}
		done, err := condition(pod)
		if done {
			// TODO: update to latest trace to wrap nil
			if err != nil {
				return trace.Wrap(err)
			}
			log.Infof("waiting for pod succeeded")
			return nil
		}
		log.Infof("waiting for pod %s in namespace '%s' status to be '%s'"+
			"(found phase: %q, readiness: %t) (%v elapsed)",
			podName, ns, desc, pod.Status.Phase, podReady(pod), time.Since(start))
	}
	return trace.Errorf("gave up waiting for pod '%s' to be '%s' after %v", podName, desc, timeout)
}

// launchNetTestPodPerNode schedules a new test pod on each of specified nodes
// using the specified containerImage.
func launchNetTestPodPerNode(client *kube.Clientset, nodes *v1.NodeList, name, containerImage, namespace string) ([]string, error) {
	podNames := []string{}
	totalPods := len(nodes.Items)

	for _, node := range nodes.Items {
		pod, err := client.Core().Pods(namespace).Create(&v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: name + "-",
				Labels: map[string]string{
					"name": name,
				},
			},
			Spec: v1.PodSpec{
				Containers: []v1.Container{
					{
						Name:  "webserver",
						Image: containerImage,
						Args: []string{
							"-service=" + name,
							// `nettest` container finds peers by looking up list of service endpoints
							fmt.Sprintf("-peers=%d", totalPods),
							"-namespace=" + namespace},
						Ports: []v1.ContainerPort{{ContainerPort: 8080}},
					},
				},
				NodeName:      node.Name,
				RestartPolicy: v1.RestartPolicyNever,
			},
		})
		if err != nil {
			return nil, trace.Wrap(err, "failed to create pod")
		}
		log.Infof("created pod %s on node %s", pod.ObjectMeta.Name, node.Name)
		podNames = append(podNames, pod.ObjectMeta.Name)
	}
	return podNames, nil
}

// podReady returns whether pod has a condition of `Ready` with a status of true.
func podReady(pod *v1.Pod) bool {
	for _, cond := range pod.Status.Conditions {
		if cond.Type == v1.PodReady && cond.Status == v1.ConditionTrue {
			return true
		}
	}
	return false
}

// createNamespaceIfNeeded creates a namespace if not already created.
func createNamespaceIfNeeded(client *kube.Clientset, namespace string) error {
	log.Infof("creating %s namespace", namespace)
	if _, err := client.Core().Namespaces().Get(namespace, metav1.GetOptions{}); err != nil {
		log.Infof("%s namespace not found: %v", namespace, err)
		_, err = client.Core().Namespaces().Create(&v1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}})
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// generateName generates a name for a kubernetes object.
// The name generated is guaranteed to satisfy kubernetes requirements.
func generateName(prefix string) string {
	return kubernetes.SimpleNameGenerator.GenerateName(prefix)
}

// getServiceAccount retrieves the service account with the specified name
// in the provided namespace.
func getServiceAccount(c *kube.Clientset, ns, name string, shouldWait bool) (*v1.ServiceAccount, error) {
	if !shouldWait {
		return c.Core().ServiceAccounts(ns).Get(name, metav1.GetOptions{})
	}

	const interval = time.Second
	const timeout = 10 * time.Second

	var err error
	var user *v1.ServiceAccount
	if err = wait.Poll(interval, timeout, func() (bool, error) {
		user, err = c.Core().ServiceAccounts(ns).Get(name, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		return true, nil
	}); err != nil {
		return nil, trace.Wrap(err)
	}
	return user, nil
}

func waitForAllNodesSchedulable(ctx context.Context, c *kube.Clientset) (nodes *v1.NodeList, err error) {
	const (
		interval = 30 * time.Second
		timeout  = 4 * time.Minute
	)
	err = wait.PollImmediate(interval, timeout, func() (bool, error) {
		opts := metav1.ListOptions{
			ResourceVersion: "0",
			FieldSelector:   fields.Set{"spec.unschedulable": "false"}.String(),
		}
		nodes, err = c.Core().Nodes().List(opts)
		if err != nil {
			log.Infof("unexpected error listing nodes: %v", err)
			// ignore the error here - it will be retried.
			return false, nil
		}
		schedulable := 0
		for _, node := range nodes.Items {
			if isNodeSchedulable(&node) {
				schedulable++
			}
		}
		if schedulable != len(nodes.Items) {
			log.Infof("%v/%v nodes schedulable (polling after 30s)", schedulable, len(nodes.Items))
			return false, nil
		}

		select {
		case <-ctx.Done():
			return false, trace.ConnectionProblem(nil, "timed out waiting for nodes to become schedulable")
		default:
		}

		return true, nil
	})
	return nodes, trace.Wrap(err)
}

// Node is schedulable if:
// 1) doesn't have "unschedulable" field set
// 2) its Ready condition is set to true
// 3) doesn't have NetworkUnavailable condition set to true
func isNodeSchedulable(node *v1.Node) bool {
	nodeReady := isNodeConditionSetAsExpected(node, v1.NodeReady, true)
	networkReady := isNodeConditionUnset(node, v1.NodeNetworkUnavailable) ||
		isNodeConditionSetAsExpected(node, v1.NodeNetworkUnavailable, false)
	return !node.Spec.Unschedulable && nodeReady && networkReady
}

func isNodeConditionSetAsExpected(node *v1.Node, conditionType v1.NodeConditionType, wantTrue bool) bool {
	// check the node readiness condition
	for _, cond := range node.Status.Conditions {
		// ensure that the condition type and the status matches as desired
		if cond.Type == conditionType {
			if (cond.Status == v1.ConditionTrue) == wantTrue {
				return true
			} else {
				log.Debugf("condition %q of node %q is %v instead of %t. Reason: %v, message: %v",
					conditionType, node.Name, cond.Status == v1.ConditionTrue, wantTrue, cond.Reason, cond.Message)
				return false
			}
		}
	}

	log.Infof("failed to find condition %v on node %v", conditionType, node.Name)
	return false
}

func isNodeConditionUnset(node *v1.Node, conditionType v1.NodeConditionType) bool {
	for _, cond := range node.Status.Conditions {
		if cond.Type == conditionType {
			return false
		}
	}
	return true
}

var subResourceServiceAndNodeProxyVersion = mustParseVersion("v1.2.0")

// getServicesProxyRequest returns the service request based on the server version
func getServicesProxyRequest(c *kube.Clientset, request *rest.Request) (*rest.Request, error) {
	subResourceProxyAvailable, err := serverVersionGTE(subResourceServiceAndNodeProxyVersion, c)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if subResourceProxyAvailable {
		return request.Resource("services").SubResource("proxy"), nil
	}
	return request.Prefix("proxy").Resource("services"), nil
}

// TODO: this functionality eventually becomes part of client.VersionInterface
// serverVersionGTE determines if server version >= v
func serverVersionGTE(v semver.Version, c discovery.ServerVersionInterface) (bool, error) {
	serverVersion, err := c.ServerVersion()
	if err != nil {
		return false, trace.Wrap(err, "unable to get server version")
	}
	parsedVersion, err := parseVersion(serverVersion.GitVersion)
	if err != nil {
		return false, trace.Wrap(err, "unable to parse server version %q", serverVersion.GitVersion)
	}
	return parsedVersion.GTE(v), nil
}

func mustParseVersion(gitversion string) semver.Version {
	v, err := parseVersion(gitversion)
	if err != nil {
		log.Fatalf("failed to parse semver from gitversion %q: %v", gitversion, err)
	}
	return v
}

func parseVersion(gitversion string) (semver.Version, error) {
	// optionally trim leading spaces then one v
	var seen bool
	gitversion = strings.TrimLeftFunc(gitversion, func(ch rune) bool {
		if seen {
			return false
		}
		if ch == 'v' {
			seen = true
			return true
		}
		return unicode.IsSpace(ch)
	})

	return semver.Make(gitversion)
}
