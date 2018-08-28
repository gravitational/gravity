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
	"fmt"
	"io"

	log "github.com/sirupsen/logrus"
	"github.com/gravitational/trace"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
)

// NewRCControl returns new instance of ReplicationController updater
func NewRCControl(config RCConfig) (*RCControl, error) {
	err := config.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var rc *v1.ReplicationController
	if config.ReplicationController != nil {
		rc = config.ReplicationController
	} else {
		rc, err = ParseReplicationController(config.Reader)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	rc.Kind = KindReplicationController
	return &RCControl{
		RCConfig:              config,
		replicationController: *rc,
		Entry: log.WithFields(log.Fields{
			"rc": fmt.Sprintf("%v/%v", Namespace(rc.Namespace), rc.Name),
		}),
	}, nil
}

// RCConfig is a ReplicationController control configuration
type RCConfig struct {
	// Reader with daemon set to update, will be used if present
	Reader io.Reader
	// ReplicationController is already parsed daemon set, will be used if present
	ReplicationController *v1.ReplicationController
	// Client is k8s client
	Client *kubernetes.Clientset
}

func (c *RCConfig) CheckAndSetDefaults() error {
	if c.Reader == nil && c.ReplicationController == nil {
		return trace.BadParameter("missing parameter Reader or ReplicationController")
	}
	if c.Client == nil {
		return trace.BadParameter("missing parameter Client")
	}
	return nil
}

// RCControl is a daemon set controller,
// adds various operations, like delete, status check and update
type RCControl struct {
	RCConfig
	replicationController v1.ReplicationController
	*log.Entry
}

// collectPods returns pods created by this RC
func (c *RCControl) collectPods(replicationController *v1.ReplicationController) ([]v1.Pod, error) {
	set := make(labels.Set)
	for key, val := range c.replicationController.Spec.Selector {
		set[key] = val
	}
	pods, err := CollectPods(replicationController.Namespace, set, c.Entry, c.Client, func(ref metav1.OwnerReference) bool {
		return ref.Kind == KindReplicationController && ref.UID == replicationController.UID
	})
	var podList []v1.Pod
	for _, pod := range pods {
		podList = append(podList, pod)
	}
	return podList, trace.Wrap(err)
}

func (c *RCControl) Delete(ctx context.Context, cascade bool) error {
	c.Infof("delete %v", formatMeta(c.replicationController.ObjectMeta))

	rcs := c.Client.Core().ReplicationControllers(c.replicationController.Namespace)
	currentRC, err := rcs.Get(c.replicationController.Name, metav1.GetOptions{})
	if err != nil {
		return ConvertError(err)
	}
	pods := c.Client.Core().Pods(c.replicationController.Namespace)
	currentPods, err := c.collectPods(currentRC)
	if err != nil {
		return trace.Wrap(err)
	}
	c.Info("deleting current replication controller")
	deletePolicy := metav1.DeletePropagationForeground
	err = rcs.Delete(c.replicationController.Name, &metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	})
	if err != nil {
		return ConvertError(err)
	}

	err = waitForObjectDeletion(func() error {
		_, err := rcs.Get(c.replicationController.Name, metav1.GetOptions{})
		return ConvertError(err)
	})
	if err != nil {
		return trace.Wrap(err)
	}

	if !cascade {
		c.Info("cascade not set, returning")
	}
	err = deletePodsList(pods, currentPods, *c.Entry)
	return trace.Wrap(err)
}

func (c *RCControl) Upsert(ctx context.Context) error {
	c.Infof("upsert %v", formatMeta(c.replicationController.ObjectMeta))

	rcs := c.Client.Core().ReplicationControllers(c.replicationController.Namespace)
	currentRC, err := rcs.Get(c.replicationController.Name, metav1.GetOptions{})
	err = ConvertError(err)
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		currentRC = nil
	}

	if currentRC != nil {
		control, err := NewRCControl(RCConfig{ReplicationController: currentRC, Client: c.Client})
		if err != nil {
			return ConvertError(err)
		}
		err = control.Delete(ctx, true)
		if err != nil {
			return ConvertError(err)
		}
	}

	c.replicationController.UID = ""
	c.replicationController.SelfLink = ""
	c.replicationController.ResourceVersion = ""

	err = withExponentialBackoff(func() error {
		_, err = rcs.Create(&c.replicationController)
		return ConvertError(err)
	})
	return trace.Wrap(err)
}

func (c *RCControl) nodeSelector() labels.Selector {
	set := make(labels.Set)
	for key, val := range c.replicationController.Spec.Template.Spec.NodeSelector {
		set[key] = val
	}
	return set.AsSelector()
}

func (c *RCControl) Status() error {
	rcs := c.Client.Core().ReplicationControllers(c.replicationController.Namespace)
	currentRC, err := rcs.Get(c.replicationController.Name, metav1.GetOptions{})
	if err != nil {
		return ConvertError(err)
	}
	var replicas int32 = 1
	if currentRC.Spec.Replicas != nil {
		replicas = *currentRC.Spec.Replicas
	}
	if currentRC.Status.Replicas != replicas {
		return trace.CompareFailed("expected replicas: %v, ready: %#v", replicas, currentRC.Status.Replicas)
	}
	pods, err := c.collectPods(currentRC)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, pod := range pods {
		if pod.Status.Phase != v1.PodRunning {
			return trace.CompareFailed("pod %v is not running yet: %v", pod.Name, pod.Status.Phase)
		}
	}
	return nil
}
