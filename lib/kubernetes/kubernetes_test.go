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
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/cenkalti/backoff"
	"github.com/gravitational/rigging"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	extensionsv1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"

	. "gopkg.in/check.v1"
)

func TestOperations(t *testing.T) { TestingT(t) }

type S struct {
	*kubernetes.Clientset
	v1.Node
}

var _ = Suite(&S{})

func (s *S) SetUpSuite(c *C) {
	log.StandardLogger().Hooks = make(log.LevelHooks)
	formatter := &trace.TextFormatter{}
	formatter.DisableTimestamp = true
	log.SetFormatter(formatter)
	if testing.Verbose() {
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.ErrorLevel)
	}
	log.SetOutput(os.Stdout)
}

func (s *S) SetUpTest(c *C) {
	testEnabled := os.Getenv(defaults.TestK8s)
	if ok, _ := strconv.ParseBool(testEnabled); !ok {
		c.Skip("skipping Kubernetes test")
	}
	var err error
	s.Clientset, _, err = utils.GetKubeClient("")
	c.Assert(err, IsNil)

	ns := newNamespace(testNamespace)
	b := backoff.NewExponentialBackOff()
	b.MaxElapsedTime = 5 * time.Minute
	err = utils.RetryWithInterval(context.TODO(), b, func() error {
		_, err = s.CoreV1().Namespaces().
			Create(context.TODO(), ns, metav1.CreateOptions{})
		err = retryOnAlreadyExists(err)
		return err
	})
	c.Assert(err, IsNil)

	client := s.CoreV1().Nodes()
	s.Node = getNode(c, client)

	if s.Labels == nil {
		s.Labels = make(map[string]string)
	}
	s.Labels["test"] = "yes"
	_, err = client.Update(context.TODO(), &s.Node, metav1.UpdateOptions{})
	c.Assert(err, IsNil)
}

func (s *S) TearDownTest(c *C) {
	err := s.CoreV1().Namespaces().
		Delete(context.TODO(), testNamespace, metav1.DeleteOptions{})
	c.Assert(err, IsNil)

	client := s.CoreV1().Nodes()
	node, err := client.Get(context.TODO(), s.Node.Name, metav1.GetOptions{})
	c.Assert(err, IsNil)

	delete(node.Labels, "test")
	_, err = client.Update(context.TODO(), node, metav1.UpdateOptions{})
	c.Assert(err, IsNil)
}

func (s *S) TestDrainsNode(c *C) {
	client := s.CoreV1().Nodes()
	ctx, cancel := context.WithTimeout(context.TODO(), testTimeout)
	defer cancel()

	// setup
	pod := newPod("foo")
	_, err := s.CoreV1().Pods(testNamespace).
		Create(ctx, pod, metav1.CreateOptions{})
	c.Assert(err, IsNil)

	d := newDeployment("bar", pod.Spec)
	_, err = s.ExtensionsV1beta1().Deployments(testNamespace).
		Create(ctx, d, metav1.CreateOptions{})
	c.Assert(err, IsNil)

	ds := newDaemonSet("qux", pod.Spec)
	_, err = s.ExtensionsV1beta1().DaemonSets(testNamespace).
		Create(ctx, ds, metav1.CreateOptions{})
	c.Assert(err, IsNil)

	podList, err := s.CoreV1().Pods(testNamespace).List(
		ctx,
		metav1.ListOptions{
			FieldSelector: fields.SelectorFromSet(fields.Set{"spec.nodeName": s.Name}).String(),
			LabelSelector: labels.SelectorFromSet(labels.Set{"test-app": "foo"}).String(),
		})
	c.Assert(err, IsNil)
	err = waitForPods(ctx, s.CoreV1(), podList.Items, v1.PodRunning)
	c.Assert(err, IsNil)

	// exercise
	err = Drain(ctx, s.Clientset, s.Name)
	c.Assert(err, IsNil)

	podList, err = s.CoreV1().Pods(testNamespace).List(
		ctx,
		metav1.ListOptions{
			FieldSelector: fields.SelectorFromSet(fields.Set{"spec.nodeName": s.Name}).String(),
			LabelSelector: labels.SelectorFromSet(labels.Set{"test-app": "foo"}).String(),
		})
	c.Assert(err, IsNil)
	c.Assert(podList.Items, HasLen, 0)

	// Clean up
	err = SetUnschedulable(ctx, client, s.Name, false)
	c.Assert(err, IsNil)
}

func (s *S) TestUpdatesNodeTaints(c *C) {
	client := s.CoreV1().Nodes()
	ctx, cancel := context.WithTimeout(context.TODO(), testTimeout)
	defer cancel()
	taintsToAdd := []v1.Taint{
		{Key: "foo", Value: "bar", Effect: v1.TaintEffectNoSchedule},
	}
	err := UpdateTaints(ctx, client, s.Name, taintsToAdd, nil)
	c.Assert(err, IsNil)

	updatedNode, err := client.Get(ctx, s.Name, metav1.GetOptions{})
	c.Assert(err, IsNil)
	c.Assert(hasTaint(updatedNode.Spec.Taints, taintsToAdd), Equals, true)

	// Remove taint
	err = UpdateTaints(ctx, client, s.Name, nil, taintsToAdd)
	c.Assert(err, IsNil)

	updatedNode, err = client.Get(ctx, s.Name, metav1.GetOptions{})
	c.Assert(err, IsNil)
	c.Assert(hasTaint(updatedNode.Spec.Taints, taintsToAdd), Equals, false)
}

func (s *S) TestCordonsUncordonsNode(c *C) {
	client := s.CoreV1().Nodes()
	ctx, cancel := context.WithTimeout(context.TODO(), testTimeout)
	defer cancel()

	// exercise
	err := SetUnschedulable(ctx, client, s.Name, true)
	c.Assert(err, IsNil)

	// verify
	updatedNode, err := client.Get(ctx, s.Name, metav1.GetOptions{})
	c.Assert(err, IsNil)
	c.Assert(updatedNode.Spec.Unschedulable, Equals, true)

	// exercise
	err = SetUnschedulable(ctx, client, s.Name, false)
	c.Assert(err, IsNil)

	// verify
	updatedNode, err = client.Get(ctx, s.Name, metav1.GetOptions{})
	c.Assert(err, IsNil)
	c.Assert(updatedNode.Spec.Unschedulable, Equals, false)
}

func newDeployment(name string, podSpec v1.PodSpec) *extensionsv1.Deployment {
	return &extensionsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNamespace,
			Name:      name,
			Labels:    map[string]string{"test-app": name},
		},
		Spec: extensionsv1.DeploymentSpec{
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"test-app": name},
				},
				Spec: podSpec,
			},
		},
	}
}

func newDaemonSet(name string, podSpec v1.PodSpec) *extensionsv1.DaemonSet {
	return &extensionsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNamespace,
			Name:      name,
			Labels:    map[string]string{"test-app": name},
		},
		Spec: extensionsv1.DaemonSetSpec{
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"test-app": name},
				},
				Spec: podSpec,
			},
		},
	}
}

func newPod(name string) *v1.Pod {
	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNamespace,
			Name:      name,
			Labels:    map[string]string{"test-app": name},
		},
		Spec: v1.PodSpec{
			NodeSelector: map[string]string{
				"test": "yes",
			},
			Containers: []v1.Container{
				{
					Name:    name,
					Image:   "apiserver:5000/gravitational/debian-tall:buster",
					Command: []string{"/bin/sh", "-c", "sleep 3600"},
				},
			},
		},
	}
}

// hasTaint checks if taints has any taints from taintsToCheck
func hasTaint(taints []v1.Taint, taintsToCheck []v1.Taint) bool {
	for _, taint := range taints {
		for _, taintToCheck := range taintsToCheck {
			if taint.MatchTaint(&taintToCheck) {
				return true
			}
		}
	}
	return false
}

// getNode returns the first available node in the cluster
func getNode(c *C, client corev1.NodeInterface) v1.Node {
	nodes, err := client.List(context.TODO(), metav1.ListOptions{})
	c.Assert(err, IsNil)
	c.Assert(nodes.Items, Not(HasLen), 0, Commentf("need at least one node"))
	return nodes.Items[0]
}

func newNamespace(name string) *v1.Namespace {
	return &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

func waitForPods(ctx context.Context, client corev1.CoreV1Interface, pods []v1.Pod, expected v1.PodPhase) error {
	b := backoff.NewConstantBackOff(defaults.WaitStatusInterval)
	err := utils.RetryWithInterval(ctx, b, func() error {
		for _, pod := range pods {
			p, err := client.Pods(pod.Namespace).Get(ctx, pod.Name, metav1.GetOptions{})
			if errors.IsNotFound(err) || (p != nil && p.Status.Phase != expected) {
				log.WithFields(podFields(pod)).Debug("waiting")
				return trace.NotFound("no pod found")
			}
		}
		return nil
	})
	return trace.Wrap(err)
}

func retryOnAlreadyExists(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.IsAlreadyExists(err):
		return rigging.ConvertError(err)
	default:
		return &backoff.PermanentError{Err: err}
	}
}

const (
	testTimeout = 1 * time.Minute

	testNamespace = "test"
)
