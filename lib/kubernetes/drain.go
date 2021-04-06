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
	"fmt"

	"github.com/gravitational/gravity/lib/log"

	"github.com/gravitational/rigging"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	kubedrain "k8s.io/kubectl/pkg/drain"
)

// drainPods removes pods according to the specified configuration
func (d *drainer) drainPods(ctx context.Context) error {
	logger := log.New(logrus.WithField(trace.Component, "k8s"))
	w := logger.Writer()
	defer w.Close()
	drainer := &kubedrain.Helper{
		Client:                d.client,
		GracePeriodSeconds:    d.gracePeriodSeconds,
		IgnoreAllDaemonSets:   true,
		DeleteLocalData:       true,
		Out:                   w,
		ErrOut:                w,
		OnPodDeletedOrEvicted: onPodDeletedOrEvicted(logger),
	}

	list, errs := drainer.GetPodsForDeletion(d.nodeName)
	if errs != nil {
		return trace.NewAggregate(errs...)
	}
	if warnings := list.Warnings(); warnings != "" {
		logger.Warnf("WARNING: %s", warnings)
	}

	if err := drainer.DeleteOrEvictPods(list.Pods()); err != nil {
		pendingList, newErrs := drainer.GetPodsForDeletion(d.nodeName)
		if pendingList != nil {
			pods := pendingList.Pods()
			if len(pods) != 0 {
				logger.WithError(err).Warnf("There are pending pods on node %q when an error occurred:\n%s.",
					d.nodeName, formatPodList(pods))
			}
		}
		if newErrs != nil {
			logger.WithError(trace.NewAggregate(newErrs...)).Warn("Failed to get the list of pods to delete.")
		}
		return rigging.ConvertError(err)
	}
	return nil
}

// onPodDeletedOrEvicted is called by drain.Helper, when the pod has been deleted or evicted
func onPodDeletedOrEvicted(logger log.Logger) func(*corev1.Pod, bool) {
	return func(pod *corev1.Pod, usingEviction bool) {
		var verbStr string
		if usingEviction {
			verbStr = "evicted"
		} else {
			verbStr = "deleted"
		}
		logrus.WithField("pod", fmt.Sprintf("%v/%v", pod.Namespace, pod.Name)).Infof("Pod %v.", verbStr)
	}
}

type drainer struct {
	client   *kubernetes.Clientset
	nodeName string
	// gracePeriodSeconds defines the grace period for eviction.
	// -1 means default grace period defined for a pod is used
	gracePeriodSeconds int
}
