// Copyright 2016 Gravitational Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package rigging

import (
	"context"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
)

// NewDaemonSetControl returns new instance of DaemonSet controller
func NewDaemonSetControl(config DSConfig) (*DSControl, error) {
	err := config.checkAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &DSControl{
		DSConfig: config,
		FieldLogger: log.WithFields(log.Fields{
			"daemonset": formatMeta(config.ObjectMeta),
		}),
	}, nil
}

// DSConfig is a DaemonSet control configuration
type DSConfig struct {
	// DaemonSet specifies the existing resource
	*appsv1.DaemonSet
	// Client is k8s client
	Client *kubernetes.Clientset
}

func (c *DSConfig) checkAndSetDefaults() error {
	if c.DaemonSet == nil {
		return trace.BadParameter("missing parameter DaemonSet")
	}
	if c.Client == nil {
		return trace.BadParameter("missing parameter Client")
	}
	updateTypeMetaDaemonset(c.DaemonSet)
	return nil
}

// DSControl is a daemon set controller,
// adds various operations, like delete, status check and update
type DSControl struct {
	DSConfig
	log.FieldLogger
}

// collectPods returns pods created by this daemon set
func (c *DSControl) collectPods(ctx context.Context, daemonSet *appsv1.DaemonSet) (map[string]v1.Pod, error) {
	var labels map[string]string
	if daemonSet.Spec.Selector != nil {
		labels = daemonSet.Spec.Selector.MatchLabels
	}
	pods, err := CollectPods(ctx, daemonSet.Namespace, labels, c.FieldLogger, c.Client, func(ref metav1.OwnerReference) bool {
		return ref.Kind == KindDaemonSet && ref.UID == daemonSet.UID
	})
	return pods, trace.Wrap(err)
}

func (c *DSControl) Delete(ctx context.Context, cascade bool) error {
	c.Infof("delete %v", formatMeta(c.DaemonSet.ObjectMeta))

	daemons := c.Client.AppsV1().DaemonSets(c.DaemonSet.Namespace)
	currentDS, err := daemons.Get(ctx, c.DaemonSet.Name, metav1.GetOptions{})
	if err != nil {
		return ConvertError(err)
	}
	pods := c.Client.CoreV1().Pods(c.DaemonSet.Namespace)
	currentPods, err := c.collectPods(ctx, currentDS)
	if err != nil {
		return trace.Wrap(err)
	}
	c.Debug("deleting current daemon set")
	deletePolicy := metav1.DeletePropagationForeground
	err = daemons.Delete(ctx, c.DaemonSet.Name, metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	})
	if err != nil {
		return ConvertError(err)
	}

	err = waitForObjectDeletion(func() error {
		_, err := daemons.Get(ctx, c.DaemonSet.Name, metav1.GetOptions{})
		return ConvertError(err)
	})
	if err != nil {
		return trace.Wrap(err)
	}

	if !cascade {
		c.Info("cascade not set, returning")
	}
	err = deletePods(ctx, pods, currentPods, c.FieldLogger)
	return trace.Wrap(err)
}

func (c *DSControl) Upsert(ctx context.Context) error {
	c.Infof("upsert %v", formatMeta(c.DaemonSet.ObjectMeta))

	daemons := c.Client.AppsV1().DaemonSets(c.DaemonSet.Namespace)
	currentDS, err := daemons.Get(ctx, c.DaemonSet.Name, metav1.GetOptions{})
	err = ConvertError(err)
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		// api always returns object, this is inconvenient
		currentDS = nil
	}

	if currentDS != nil {
		if checkCustomerManagedResource(currentDS.Annotations) {
			c.WithField("daemonset", formatMeta(c.DaemonSet.ObjectMeta)).Info("Skipping update since object is customer managed.")
			return nil
		}

		control, err := NewDaemonSetControl(DSConfig{DaemonSet: currentDS, Client: c.Client})
		if err != nil {
			return trace.Wrap(err)
		}
		err = control.Delete(ctx, true)
		if err != nil {
			return ConvertError(err)
		}
	}

	c.Debug("creating new daemon set")
	c.DaemonSet.UID = ""
	c.DaemonSet.SelfLink = ""
	c.DaemonSet.ResourceVersion = ""

	err = withExponentialBackoff(func() error {
		_, err = daemons.Create(ctx, c.DaemonSet, metav1.CreateOptions{})
		return ConvertError(err)
	})
	return trace.Wrap(err)
}

func (c *DSControl) nodeSelector() labels.Selector {
	set := make(labels.Set)
	for key, val := range c.DaemonSet.Spec.Template.Spec.NodeSelector {
		set[key] = val
	}
	return set.AsSelector()
}

func (c *DSControl) Status(ctx context.Context) error {
	daemons := c.Client.AppsV1().DaemonSets(c.DaemonSet.Namespace)
	currentDS, err := daemons.Get(ctx, c.DaemonSet.Name, metav1.GetOptions{})
	if err != nil {
		return ConvertError(err)
	}

	currentPods, err := c.collectPods(ctx, currentDS)
	if err != nil {
		return trace.Wrap(err)
	}

	nodes, err := c.Client.CoreV1().Nodes().List(ctx, metav1.ListOptions{
		LabelSelector: c.nodeSelector().String(),
	})
	if err != nil {
		return ConvertError(err)
	}

	// If the node selector doesn't match any nodes, just return nil
	// This is for use cases, where the selector is set to say run on worker nodes, but the cluster only has masters
	if len(nodes.Items) == 0 {
		return nil
	}

	return checkRunning(currentPods, nodes.Items, c.FieldLogger)
}

func updateTypeMetaDaemonset(r *appsv1.DaemonSet) {
	r.Kind = KindDaemonSet
	if r.APIVersion == "" {
		r.APIVersion = appsv1.SchemeGroupVersion.String()
	}
}
