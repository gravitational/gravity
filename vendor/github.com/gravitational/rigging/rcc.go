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

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
)

// NewReplicationControllerControl returns new instance of ReplicationController control
func NewReplicationControllerControl(config RCConfig) (*RCControl, error) {
	err := config.checkAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &RCControl{
		RCConfig: config,
		FieldLogger: log.WithFields(log.Fields{
			"replicationcontroller": fmt.Sprintf("%v/%v",
				Namespace(config.Namespace), config.Name),
		}),
	}, nil
}

// RCConfig is a ReplicationController control configuration
type RCConfig struct {
	// ReplicationController specifies the existing ReplicationController
	*v1.ReplicationController
	// Client is k8s client
	Client *kubernetes.Clientset
}

func (c *RCConfig) checkAndSetDefaults() error {
	if c.Client == nil {
		return trace.BadParameter("missing parameter Client")
	}
	updateTypeMetaReplicationController(c.ReplicationController)
	return nil
}

// RCControl is a daemon set controller,
// adds various operations, like delete, status check and update
type RCControl struct {
	RCConfig
	log.FieldLogger
}

// collectPods returns pods created by this RC
func (c *RCControl) collectPods(ctx context.Context, replicationController *v1.ReplicationController) ([]v1.Pod, error) {
	set := make(labels.Set)
	for key, val := range c.ReplicationController.Spec.Selector {
		set[key] = val
	}
	pods, err := CollectPods(ctx, replicationController.Namespace, set, c.FieldLogger, c.Client, func(ref metav1.OwnerReference) bool {
		return ref.Kind == KindReplicationController && ref.UID == replicationController.UID
	})
	var podList []v1.Pod
	for _, pod := range pods {
		podList = append(podList, pod)
	}
	return podList, trace.Wrap(err)
}

func (c *RCControl) Delete(ctx context.Context, cascade bool) error {
	c.Infof("delete %v", formatMeta(c.ReplicationController.ObjectMeta))

	rcs := c.Client.CoreV1().ReplicationControllers(c.ReplicationController.Namespace)
	currentRC, err := rcs.Get(ctx, c.ReplicationController.Name, metav1.GetOptions{})
	if err != nil {
		return ConvertError(err)
	}
	pods := c.Client.CoreV1().Pods(c.ReplicationController.Namespace)
	currentPods, err := c.collectPods(ctx, currentRC)
	if err != nil {
		return trace.Wrap(err)
	}
	c.Debug("deleting current replication controller")
	deletePolicy := metav1.DeletePropagationForeground
	err = rcs.Delete(ctx, c.ReplicationController.Name, metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	})
	if err != nil {
		return ConvertError(err)
	}

	err = waitForObjectDeletion(func() error {
		_, err := rcs.Get(ctx, c.ReplicationController.Name, metav1.GetOptions{})
		return ConvertError(err)
	})
	if err != nil {
		return trace.Wrap(err)
	}

	if !cascade {
		c.Info("cascade not set, returning")
	}
	err = deletePodsList(ctx, pods, currentPods, c.FieldLogger)
	return trace.Wrap(err)
}

func (c *RCControl) Upsert(ctx context.Context) error {
	c.Infof("upsert %v", formatMeta(c.ReplicationController.ObjectMeta))

	rcs := c.Client.CoreV1().ReplicationControllers(c.ReplicationController.Namespace)
	currentRC, err := rcs.Get(ctx, c.ReplicationController.Name, metav1.GetOptions{})
	err = ConvertError(err)
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		currentRC = nil
	}

	if currentRC != nil {
		if checkCustomerManagedResource(currentRC.Annotations) {
			c.WithField("replicationcontroller", formatMeta(c.ReplicationController.ObjectMeta)).Info("Skipping update since object is customer managed.")
			return nil
		}

		control, err := NewReplicationControllerControl(RCConfig{ReplicationController: currentRC, Client: c.Client})
		if err != nil {
			return ConvertError(err)
		}
		err = control.Delete(ctx, true)
		if err != nil {
			return ConvertError(err)
		}
	}

	c.ReplicationController.UID = ""
	c.ReplicationController.SelfLink = ""
	c.ReplicationController.ResourceVersion = ""

	err = withExponentialBackoff(func() error {
		_, err = rcs.Create(ctx, c.ReplicationController, metav1.CreateOptions{})
		return ConvertError(err)
	})
	return trace.Wrap(err)
}

func (c *RCControl) nodeSelector() labels.Selector {
	set := make(labels.Set)
	for key, val := range c.ReplicationController.Spec.Template.Spec.NodeSelector {
		set[key] = val
	}
	return set.AsSelector()
}

func (c *RCControl) Status(ctx context.Context) error {
	rcs := c.Client.CoreV1().ReplicationControllers(c.ReplicationController.Namespace)
	currentRC, err := rcs.Get(ctx, c.ReplicationController.Name, metav1.GetOptions{})
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
	pods, err := c.collectPods(ctx, currentRC)
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

func updateTypeMetaReplicationController(r *v1.ReplicationController) {
	r.Kind = KindReplicationController
	if r.APIVersion == "" {
		r.APIVersion = v1.SchemeGroupVersion.String()
	}
}
