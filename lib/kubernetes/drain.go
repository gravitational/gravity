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

package kubernetes

import (
	"context"
	"time"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/cenkalti/backoff"
	"github.com/gravitational/rigging"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"k8s.io/api/core/v1"
	policy "k8s.io/api/policy/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

// drainPods removes pods according to the specified configuration
func (d *drain) drainPods(ctx context.Context) error {
	pods, err := d.getPodsForDeletion()
	if err != nil {
		return trace.Wrap(err)
	}

	err = d.deleteOrEvictPods(ctx, pods)
	if err != nil {
		pendingPods, err := d.getPodsForDeletion()
		if err != nil {
			return trace.Wrap(err)
		}
		log.Warningf("error deleting pods: %v\npending pods: %v",
			trace.DebugReport(err), formatPodList(pendingPods))
	}
	return trace.Wrap(err)
}

// deleteOrEvictPods evicts the pods depending if the api server supports Eviction API
// and deletes them otherwise
func (d *drain) deleteOrEvictPods(ctx context.Context, pods []v1.Pod) error {
	if len(pods) == 0 {
		return nil
	}

	policyGroupVersion, err := queryEvictionPolicyGroupVersion(d.client.Discovery())
	if err != nil {
		return trace.Wrap(err)
	}

	if len(policyGroupVersion) > 0 {
		return trace.Wrap(d.evictPods(ctx, pods, policyGroupVersion))
	}
	return trace.Wrap(d.deletePods(ctx, pods))
}

func (d *drain) evictPods(ctx context.Context, pods []v1.Pod, policyGroupVersion string) error {
	errCh := make(chan error, len(pods))

	for _, pod := range pods {
		go func(pod v1.Pod, errCh chan error) {
			err := d.evictPodAndWait(ctx, pod, policyGroupVersion)
			if err != nil {
				errCh <- trace.Wrap(err)
				return
			}
			errCh <- nil
		}(pod, errCh)
	}

	return trace.Wrap(utils.CollectErrors(ctx, errCh))
}

func (d *drain) evictPodAndWait(ctx context.Context, pod v1.Pod, policyGroupVersion string) error {
	b := backoff.NewExponentialBackOff()
	err := utils.RetryWithInterval(ctx, b,
		func() error {
			err := d.evictPod(pod, policyGroupVersion)
			if errors.IsNotFound(trace.Unwrap(err)) {
				return nil
			} else if errors.IsTooManyRequests(trace.Unwrap(err)) {
				return trace.Retry(err, "too many requests")
			}
			return &backoff.PermanentError{Err: rigging.ConvertError(err)}
		})
	if err != nil {
		return trace.Wrap(err, "error evicting pod %v", formatPod(pod))
	}

	// Set the timeout before pod termination based on how long we expect kubernetes to take
	// with a safe margin of error
	waitDuration := terminationWaitPeriod(pod)
	ctx, cancel := context.WithTimeout(ctx, waitDuration)
	defer cancel()

	podArray := []v1.Pod{pod}
	_, err = waitForDelete(ctx, d.client.CoreV1(), podArray, usingEviction(true))
	if err != nil {
		return trace.Wrap(err, "error waiting for pod %v to terminate", formatPod(pod))
	}
	return nil
}

func (d *drain) deletePods(ctx context.Context, pods []v1.Pod) error {
	for _, pod := range pods {
		err := d.deletePod(pod)
		if err != nil && !errors.IsNotFound(trace.Unwrap(err)) {
			return rigging.ConvertError(err)
		}
	}
	_, err := waitForDelete(ctx, d.client.CoreV1(), pods, usingEviction(false))
	return trace.Wrap(err)
}

func (d *drain) deletePod(pod v1.Pod) error {
	options := &metav1.DeleteOptions{}
	if d.gracePeriodSeconds >= 0 {
		options.GracePeriodSeconds = utils.Int64Ptr(d.gracePeriodSeconds)
	}
	// not using rigging.ConvertError on purpose to keep the original error
	return trace.Wrap(d.client.Core().Pods(pod.Namespace).Delete(pod.Name, options))
}

func (d *drain) evictPod(pod v1.Pod, policyGroupVersion string) error {
	options := &metav1.DeleteOptions{}
	if d.gracePeriodSeconds >= 0 {
		options.GracePeriodSeconds = utils.Int64Ptr(d.gracePeriodSeconds)
	}
	eviction := &policy.Eviction{
		TypeMeta: metav1.TypeMeta{
			APIVersion: policyGroupVersion,
			Kind:       EvictionKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      pod.Name,
			Namespace: pod.Namespace,
		},
		DeleteOptions: options,
	}
	// not using rigging.ConvertError on purpose to keep the original error
	return trace.Wrap(d.client.Policy().Evictions(eviction.Namespace).Evict(eviction))
}

// getPodsForDeletion returns all the pods to delete.
// DaemonSet pods are always filtered out
func (d *drain) getPodsForDeletion() (pods []v1.Pod, err error) {
	podList, err := d.client.Core().Pods(metav1.NamespaceAll).List(metav1.ListOptions{
		FieldSelector: fields.SelectorFromSet(fields.Set{"spec.nodeName": d.nodeName}).String()})
	if err != nil {
		return nil, rigging.ConvertError(err)
	}

	for _, pod := range podList.Items {
		okToDelete, err := d.canDeletePod(pod)
		if err != nil {
			log.WithFields(podFields(pod)).Warnf("failed to filter out: %v", trace.DebugReport(err))
			continue
		}
		if okToDelete {
			pods = append(pods, pod)
		}
	}

	// Foregoing checks for mirror pods, unreplicated (not managed by a controller) pods,
	// pods with local storage: we assume the configuration to ignore/delete such pods
	return pods, nil
}

func (d *drain) canDeletePod(pod v1.Pod) (remove bool, err error) {
	// Note that we return false in cases where the pod is DaemonSet managed,
	// regardless of flags.  We never delete them, the only question is whether
	// their presence constitutes an error.
	//
	// The exception is for pods that are orphaned (the referencing
	// controller resource - including DaemonSet - is not found).
	// Such pods will be deleted
	if len(pod.OwnerReferences) == 0 {
		log.WithFields(podFields(pod)).Warnf("Pod does not have controller.")
		return true, nil
	}
	// if the controller is DaemonSet, do not allow to delete the pod
	for _, ref := range pod.OwnerReferences {
		if ref.Kind == rigging.KindDaemonSet {
			return false, nil
		}
	}
	return true, nil
}

type drain struct {
	client   *kubernetes.Clientset
	nodeName string
	// gracePeriodSeconds defines the grace period for eviction.
	// -1 means default grace period defined for a pod is used
	gracePeriodSeconds int64
	// timeout sets the timeout for the operation.
	// zero value means no timeout
	timeout time.Duration
}

// queryEvictionPolicyGroupVersion uses Discovery API to find out if the server supports eviction subresource.
// Returns the version of the eviction policy group if successful.
func queryEvictionPolicyGroupVersion(client discovery.DiscoveryInterface) (policyGroupVersion string, err error) {
	groupList, err := client.ServerGroups()
	if err != nil {
		return "", rigging.ConvertError(err)
	}
	foundPolicyGroup := false
	for _, group := range groupList.Groups {
		if group.Name == "policy" {
			foundPolicyGroup = true
			policyGroupVersion = group.PreferredVersion.GroupVersion
			break
		}
	}
	if !foundPolicyGroup {
		return "", nil
	}
	resourceList, err := client.ServerResourcesForGroupVersion("v1")
	if err != nil {
		return "", rigging.ConvertError(err)
	}
	for _, resource := range resourceList.APIResources {
		if resource.Name == EvictionSubresource && resource.Kind == EvictionKind {
			return policyGroupVersion, nil
		}
	}
	return "", nil
}

func waitForDelete(ctx context.Context, client corev1.CoreV1Interface, pods []v1.Pod, usingEviction usingEviction) (pendingPods []v1.Pod, err error) {
	b := backoff.NewConstantBackOff(defaults.WaitStatusInterval)
	err = utils.RetryWithInterval(ctx, b, func() error {
		pendingPods = []v1.Pod{}
		for i, pod := range pods {
			p, err := client.Pods(pod.Namespace).Get(pod.Name, metav1.GetOptions{})
			if errors.IsNotFound(err) || (p != nil && p.ObjectMeta.UID != pod.ObjectMeta.UID) {
				out := log.WithFields(podFields(pod))
				if usingEviction {
					out.Debug("evicted")
				} else {
					out.Debug("deleted")
				}
				continue
			} else if err != nil {
				return &backoff.PermanentError{rigging.ConvertError(err)}
			}
			pendingPods = append(pendingPods, pods[i])
		}
		if len(pendingPods) > 0 {
			return trace.Retry(nil, "pending pods: %v", formatPodList(pendingPods))
		}
		return nil
	})
	return pendingPods, trace.Wrap(err)
}

// terminationWaitPeriod calculates the amount of time we should wait for a pod to terminate,
// Based on the TerminationGracePeriod plus some amount of time for Kubernetes to force terminate
// the pod.
func terminationWaitPeriod(pod v1.Pod) time.Duration {
	if pod.Spec.TerminationGracePeriodSeconds == nil {
		return v1.DefaultTerminationGracePeriodSeconds*time.Second + defaults.TerminationWaitTimeout
	}
	return time.Duration(*pod.Spec.TerminationGracePeriodSeconds)*time.Second + defaults.TerminationWaitTimeout
}

const (
	// EvictionKind defines the resource kind for Eviction resources
	EvictionKind = "Eviction"
	// EvictionSubresource defines the sub-resource for pod eviction
	EvictionSubresource = "pods/eviction"
)

type usingEviction bool

type podGetter func(namespace, name string) (*v1.Pod, error)
