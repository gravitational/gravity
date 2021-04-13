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

// NewDeploymentControl returns new instance of Deployment updater
func NewDeploymentControl(config DeploymentConfig) (*DeploymentControl, error) {
	err := config.checkAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &DeploymentControl{
		DeploymentConfig: config,
		FieldLogger: log.WithFields(log.Fields{
			"deployment": formatMeta(config.ObjectMeta),
		}),
	}, nil
}

// DeploymentConfig  is a Deployment control configuration
type DeploymentConfig struct {
	// Deployment specifies the existing deployment
	*appsv1.Deployment
	// Client is k8s client
	Client *kubernetes.Clientset
}

func (c *DeploymentConfig) checkAndSetDefaults() error {
	if c.Deployment == nil {
		return trace.BadParameter("missing parameter Deployment")
	}
	if c.Client == nil {
		return trace.BadParameter("missing parameter Client")
	}
	updateTypeMetaDeployment(c.Deployment)
	return nil
}

// DeploymentControl is a deployment controller,
// adds various operations, like delete, status check and update
type DeploymentControl struct {
	DeploymentConfig
	log.FieldLogger
}

func (c *DeploymentControl) Delete(ctx context.Context, cascade bool) error {
	c.Infof("delete %v", formatMeta(c.Deployment.ObjectMeta))

	deployments := c.Client.AppsV1().Deployments(c.Deployment.Namespace)
	currentDeployment, err := deployments.Get(ctx, c.Deployment.Name, metav1.GetOptions{})
	if err != nil {
		return ConvertError(err)
	}

	pods := c.Client.CoreV1().Pods(c.Deployment.Namespace)
	currentPods, err := c.collectPods(ctx, currentDeployment)
	if err != nil {
		return trace.Wrap(err)
	}

	if cascade {
		// scale deployment down to delete the pods
		var replicas int32
		currentDeployment.Spec.Replicas = &replicas
		currentDeployment, err = deployments.Update(ctx, currentDeployment, metav1.UpdateOptions{})
		if err != nil {
			return ConvertError(err)
		}
	}
	deletePolicy := metav1.DeletePropagationForeground
	err = deployments.Delete(ctx, c.Deployment.Name, metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	})
	if err != nil {
		return ConvertError(err)
	}

	err = waitForObjectDeletion(func() error {
		_, err := deployments.Get(ctx, c.Deployment.Name, metav1.GetOptions{})
		return ConvertError(err)
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// wait until all Pods have been cleaned up
	err = waitForPods(ctx, pods, currentPods)
	if err != nil {
		c.Warningf("failed to wait for Pods to clean up: %v", trace.DebugReport(err))
	}
	return nil
}

func (c *DeploymentControl) Upsert(ctx context.Context) error {
	c.Infof("upsert %v", formatMeta(c.Deployment.ObjectMeta))

	deployments := c.Client.AppsV1().Deployments(c.Deployment.Namespace)
	c.Deployment.UID = ""
	c.Deployment.SelfLink = ""
	c.Deployment.ResourceVersion = ""
	existing, err := deployments.Get(ctx, c.Deployment.Name, metav1.GetOptions{})
	err = ConvertError(err)
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		_, err = deployments.Create(ctx, c.Deployment, metav1.CreateOptions{})
		return ConvertError(err)
	}

	if checkCustomerManagedResource(existing.Annotations) {
		c.WithField("deploy", formatMeta(c.Deployment.ObjectMeta)).Info("Skipping update since object is customer managed.")
		return nil
	}

	_, err = deployments.Update(ctx, c.Deployment, metav1.UpdateOptions{})
	return ConvertError(err)
}

func (c *DeploymentControl) nodeSelector() labels.Selector {
	set := make(labels.Set)
	for key, val := range c.Deployment.Spec.Template.Spec.NodeSelector {
		set[key] = val
	}
	return set.AsSelector()
}

func (c *DeploymentControl) Status(ctx context.Context) error {
	deployments := c.Client.AppsV1().Deployments(c.Deployment.Namespace)
	currentDeployment, err := deployments.Get(ctx, c.Deployment.Name, metav1.GetOptions{})
	if err != nil {
		return ConvertError(err)
	}
	var replicas int32 = 1
	if currentDeployment.Spec.Replicas != nil {
		replicas = *(currentDeployment.Spec.Replicas)
	}
	deployment := formatMeta(c.Deployment.ObjectMeta)
	if currentDeployment.Status.UpdatedReplicas != replicas {
		return trace.CompareFailed("deployment %v not successful: expected replicas: %v, updated: %v",
			deployment, replicas, currentDeployment.Status.UpdatedReplicas)
	}
	if currentDeployment.Status.AvailableReplicas != replicas {
		return trace.CompareFailed("deployment %v not successful: expected replicas: %v, available: %v",
			deployment, replicas, currentDeployment.Status.AvailableReplicas)
	}
	return nil
}

func (c *DeploymentControl) collectPods(ctx context.Context, deployment *appsv1.Deployment) (map[string]v1.Pod, error) {
	var labels map[string]string
	if deployment.Spec.Selector != nil {
		labels = deployment.Spec.Selector.MatchLabels
	}
	pods, err := CollectPods(ctx, deployment.Namespace, labels, c.FieldLogger, c.Client, func(ref metav1.OwnerReference) bool {
		return ref.Kind == KindDeployment && ref.UID == deployment.UID
	})
	return pods, ConvertError(err)
}

func updateTypeMetaDeployment(r *appsv1.Deployment) {
	r.Kind = KindDeployment
	if r.APIVersion == "" {
		r.APIVersion = appsv1.SchemeGroupVersion.String()
	}
}
