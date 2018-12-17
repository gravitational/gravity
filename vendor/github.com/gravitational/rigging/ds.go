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
	"io"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
)

// NewDSControl returns new instance of DaemonSet updater
func NewDSControl(config DSConfig) (*DSControl, error) {
	err := config.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var ds *appsv1.DaemonSet
	if config.DaemonSet != nil {
		ds = config.DaemonSet
	} else {
		ds, err = ParseDaemonSet(config.Reader)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	// sometimes existing objects pulled from the API don't have type set
	ds.Kind = KindDaemonSet
	return &DSControl{
		DSConfig:  config,
		daemonSet: *ds,
		Entry: log.WithFields(log.Fields{
			"ds": formatMeta(ds.ObjectMeta),
		}),
	}, nil
}

// DSConfig is a DaemonSet control configuration
type DSConfig struct {
	// Reader with daemon set to update, will be used if present
	Reader io.Reader
	// DaemonSet is already parsed daemon set, will be used if present
	DaemonSet *appsv1.DaemonSet
	// Client is k8s client
	Client *kubernetes.Clientset
}

func (c *DSConfig) CheckAndSetDefaults() error {
	if c.Reader == nil && c.DaemonSet == nil {
		return trace.BadParameter("missing parameter Reader or DaemonSet")
	}
	if c.Client == nil {
		return trace.BadParameter("missing parameter Client")
	}
	return nil
}

// DSControl is a daemon set controller,
// adds various operations, like delete, status check and update
type DSControl struct {
	DSConfig
	daemonSet appsv1.DaemonSet
	*log.Entry
}

// collectPods returns pods created by this daemon set
func (c *DSControl) collectPods(daemonSet *v1beta1.DaemonSet) (map[string]v1.Pod, error) {
	var labels map[string]string
	if daemonSet.Spec.Selector != nil {
		labels = daemonSet.Spec.Selector.MatchLabels
	}
	pods, err := CollectPods(daemonSet.Namespace, labels, c.Entry, c.Client, func(ref metav1.OwnerReference) bool {
		return ref.Kind == KindDaemonSet && ref.UID == daemonSet.UID
	})
	return pods, trace.Wrap(err)
}

func (c *DSControl) Delete(ctx context.Context, cascade bool) error {
	c.Infof("delete %v", formatMeta(c.daemonSet.ObjectMeta))

	daemons := c.Client.Extensions().DaemonSets(c.daemonSet.Namespace)
	currentDS, err := daemons.Get(c.daemonSet.Name, metav1.GetOptions{})
	if err != nil {
		return ConvertError(err)
	}
	pods := c.Client.Core().Pods(c.daemonSet.Namespace)
	currentPods, err := c.collectPods(currentDS)
	if err != nil {
		return trace.Wrap(err)
	}
	c.Info("deleting current daemon set")
	deletePolicy := metav1.DeletePropagationForeground
	err = daemons.Delete(c.daemonSet.Name, &metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	})
	if err != nil {
		return ConvertError(err)
	}

	err = waitForObjectDeletion(func() error {
		_, err := daemons.Get(c.daemonSet.Name, metav1.GetOptions{})
		return ConvertError(err)
	})
	if err != nil {
		return trace.Wrap(err)
	}

	if !cascade {
		c.Info("cascade not set, returning")
	}
	err = deletePods(pods, currentPods, *c.Entry)
	return trace.Wrap(err)
}

func (c *DSControl) Upsert(ctx context.Context) error {
	c.Infof("upsert %v", formatMeta(c.daemonSet.ObjectMeta))

	daemons := c.Client.Apps().DaemonSets(c.daemonSet.Namespace)
	currentDS, err := daemons.Get(c.daemonSet.Name, metav1.GetOptions{})
	err = ConvertError(err)
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		// api always returns object, this is inconvenent
		currentDS = nil
	}

	if currentDS != nil {
		control, err := NewDSControl(DSConfig{DaemonSet: currentDS, Client: c.Client})
		if err != nil {
			return trace.Wrap(err)
		}
		err = control.Delete(ctx, true)
		if err != nil {
			return ConvertError(err)
		}
	}

	c.Info("creating new daemon set")
	c.daemonSet.UID = ""
	c.daemonSet.SelfLink = ""
	c.daemonSet.ResourceVersion = ""

	err = withExponentialBackoff(func() error {
		_, err = daemons.Create(&c.daemonSet)
		return ConvertError(err)
	})
	return trace.Wrap(err)
}

func (c *DSControl) nodeSelector() labels.Selector {
	set := make(labels.Set)
	for key, val := range c.daemonSet.Spec.Template.Spec.NodeSelector {
		set[key] = val
	}
	return set.AsSelector()
}

func (c *DSControl) Status() error {
	daemons := c.Client.Extensions().DaemonSets(c.daemonSet.Namespace)
	currentDS, err := daemons.Get(c.daemonSet.Name, metav1.GetOptions{})
	if err != nil {
		return ConvertError(err)
	}

	currentPods, err := c.collectPods(currentDS)
	if err != nil {
		return trace.Wrap(err)
	}

	nodes, err := c.Client.Core().Nodes().List(metav1.ListOptions{
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

	return checkRunning(currentPods, nodes.Items, c.Entry)
}
