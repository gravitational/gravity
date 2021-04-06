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
	"encoding/json"
	"fmt"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/cenkalti/backoff"
	"github.com/gravitational/rigging"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/kubernetes"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

// Drain safely drains the specified node and uses Eviction API if supported on the api server.
func Drain(ctx context.Context, client *kubernetes.Clientset, nodeName string) error {
	err := SetUnschedulable(ctx, client.CoreV1().Nodes(), nodeName, true)
	if err != nil {
		return trace.Wrap(err)
	}

	d := drainer{
		client:             client,
		nodeName:           nodeName,
		gracePeriodSeconds: defaults.ResourceGracePeriod,
	}
	err = d.drainPods(ctx)
	return trace.Wrap(err)
}

// SetUnschedulable marks the specified node as unschedulable depending on the value of the specified flag.
// Retries the operation internally on update conflicts.
func SetUnschedulable(ctx context.Context, client corev1.NodeInterface, nodeName string, unschedulable bool) error {
	node, err := client.Get(nodeName, metav1.GetOptions{})
	if err != nil {
		return rigging.ConvertError(err)
	}

	if node.Spec.Unschedulable == unschedulable {
		log := log.WithField("node", nodeName)
		if unschedulable {
			log.Debug("already cordoned")
		} else {
			log.Debug("already uncordoned")
		}
		// No update
		return nil
	}

	err = Retry(ctx, func() error {
		return trace.Wrap(setUnschedulable(client, nodeName, unschedulable))
	})

	return rigging.ConvertError(err)
}

// UpdateTaints adds and/or removes taints specified with add/remove correspondingly on the specified node.
func UpdateTaints(ctx context.Context, client corev1.NodeInterface, nodeName string, taintsToAdd []v1.Taint, taintsToRemove []v1.Taint) error {
	node, err := client.Get(nodeName, metav1.GetOptions{})
	if err != nil {
		return rigging.ConvertError(err)
	}
	newTaints := append([]v1.Taint{}, taintsToAdd...)
	oldTaints := node.Spec.Taints
	// add taints that already exist but are not updated to newTaints
	added := addTaints(oldTaints, &newTaints)
	deleted, err := deleteTaints(taintsToRemove, &newTaints)
	if err != nil {
		return trace.Wrap(err)
	}

	if !added && !deleted {
		// No update
		return nil
	}

	err = Retry(ctx, func() error {
		return trace.Wrap(updateTaints(client, nodeName, newTaints))
	})

	return rigging.ConvertError(err)
}

// UpdateLabels adds labels on the node specified with nodeName
func UpdateLabels(ctx context.Context, client corev1.NodeInterface, nodeName string, labels map[string]string) error {
	err := Retry(ctx, func() error {
		return trace.Wrap(updateLabels(client, nodeName, labels))
	})

	return rigging.ConvertError(err)
}

// GetNode returns Kubernetes node corresponding to the provided server
func GetNode(client *kubernetes.Clientset, server storage.Server) (*v1.Node, error) {
	nodes, err := client.CoreV1().Nodes().List(metav1.ListOptions{
		LabelSelector: utils.MakeSelector(map[string]string{
			v1.LabelHostname: server.KubeNodeID(),
		}).String(),
	})
	if err != nil {
		return nil, rigging.ConvertErrorWithContext(err,
			"failed to list Kubernetes nodes")
	}
	if len(nodes.Items) == 0 {
		return nil, trace.NotFound(
			"could not find a Kubernetes node for %v", server).
			AddField("label", fmt.Sprintf("%v=%v", v1.LabelHostname, server.KubeNodeID()))
	}
	if len(nodes.Items) > 1 {
		return nil, trace.BadParameter(
			"found more than 1 Kubernetes node for %v: %v", server, nodes.Items)
	}
	return &nodes.Items[0], nil
}

// setUnschedulable sets unschedulable status on the node given with nodeName
func setUnschedulable(client corev1.NodeInterface, nodeName string, unschedulable bool) error {
	node, err := client.Get(nodeName, metav1.GetOptions{})
	if err != nil {
		return rigging.ConvertError(err)
	}

	oldData, err := json.Marshal(node)
	if err != nil {
		return rigging.ConvertError(err)
	}

	node.Spec.Unschedulable = unschedulable

	newData, err := json.Marshal(node)
	if err != nil {
		return rigging.ConvertError(err)
	}

	patchBytes, patchErr := strategicpatch.CreateTwoWayMergePatch(oldData, newData, node)
	if patchErr == nil {
		_, err = client.Patch(node.Name, types.StrategicMergePatchType, patchBytes)
	} else {
		log.WithError(err).Warn("Failed to patch node object.")
		_, err = client.Update(node)
	}
	return rigging.ConvertError(err)
}

// updateTaints updates taints on the node given with nodeName from newTaints
func updateTaints(client corev1.NodeInterface, nodeName string, newTaints []v1.Taint) error {
	node, err := client.Get(nodeName, metav1.GetOptions{})
	if err != nil {
		return trace.Wrap(err)
	}

	node.Spec.Taints = newTaints

	_, err = client.Update(node)
	return rigging.ConvertError(err)
}

// updateLabels updates labels on the node specified with nodeName
func updateLabels(client corev1.NodeInterface, nodeName string, labels map[string]string) error {
	node, err := client.Get(nodeName, metav1.GetOptions{})
	if err != nil {
		return trace.Wrap(err)
	}

	for name, value := range labels {
		node.Labels[name] = value
	}

	_, err = client.Update(node)
	return rigging.ConvertError(err)
}

// deleteTaints deletes the given taints from the node's list of taints
func deleteTaints(taintsToDelete []v1.Taint, newTaints *[]v1.Taint) (deleted bool, err error) {
	var errors []error
	for _, taintToDelete := range taintsToDelete {
		deleted = false
		if len(taintToDelete.Effect) > 0 {
			*newTaints, deleted = deleteTaint(*newTaints, &taintToDelete)
		} else {
			*newTaints, deleted = deleteTaintsByKey(*newTaints, taintToDelete.Key)
		}
		if !deleted {
			errors = append(errors, trace.NotFound("taint %q not found", taintToDelete.ToString()))
		}
	}
	if len(errors) != 0 {
		if len(errors) == 1 {
			return false, trace.Wrap(errors[0])
		}
		return false, trace.NewAggregate(errors...)
	}
	return true, nil
}

// deleteTaintsByKey removes all the taints that have the same key to given taintKey
func deleteTaintsByKey(taints []v1.Taint, taintKey string) (result []v1.Taint, deleted bool) {
	for _, taint := range taints {
		if taintKey == taint.Key {
			deleted = true
			continue
		}
		result = append(result, taint)
	}
	return result, deleted
}

// deleteTaint removes all the taints that have the same key and effect to given taintToDelete.
func deleteTaint(taints []v1.Taint, taintToDelete *v1.Taint) (result []v1.Taint, deleted bool) {
	for i := range taints {
		if taintToDelete.MatchTaint(&taints[i]) {
			deleted = true
			continue
		}
		result = append(result, taints[i])
	}
	return result, deleted
}

// addTaints adds the newTaints list to existing ones and updates the newTaints list.
// TODO: This needs a rewrite to take only the new values instead of appended newTaints list to be consistent.
func addTaints(oldTaints []v1.Taint, newTaints *[]v1.Taint) bool {
	for _, oldTaint := range oldTaints {
		existsInNew := false
		for _, taint := range *newTaints {
			if taint.MatchTaint(&oldTaint) {
				existsInNew = true
				break
			}
		}
		if !existsInNew {
			*newTaints = append(*newTaints, oldTaint)
		}
	}
	return len(oldTaints) != len(*newTaints)
}

// Retry retries the specified function fn using classify to determine
// whether to Retry a particular error.
// Returns the first permanent error
func Retry(ctx context.Context, fn func() error) error {
	interval := backoff.NewExponentialBackOff()
	err := utils.RetryWithInterval(ctx, interval, func() error {
		return RetryOnUpdateConflict(fn())
	})
	return trace.Wrap(err)
}
