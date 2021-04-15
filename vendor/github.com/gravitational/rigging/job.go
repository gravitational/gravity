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
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func NewJobControl(config JobConfig) (*JobControl, error) {
	err := config.checkAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &JobControl{
		JobConfig: config,
		FieldLogger: log.WithFields(log.Fields{
			"job": formatMeta(config.Job.ObjectMeta),
		}),
	}, nil
}

func (c *JobControl) Delete(ctx context.Context, cascade bool) error {
	c.Infof("delete %v", formatMeta(c.Job.ObjectMeta))

	jobs := c.BatchV1().Jobs(c.Job.Namespace)
	currentJob, err := jobs.Get(ctx, c.Job.Name, metav1.GetOptions{})
	if err != nil {
		return ConvertError(err)
	}

	pods := c.CoreV1().Pods(c.Job.Namespace)
	currentPods, err := c.collectPods(ctx, currentJob)
	if err != nil {
		return trace.Wrap(err)
	}

	c.Debug("deleting current job")
	deletePolicy := metav1.DeletePropagationForeground
	err = jobs.Delete(ctx, c.Job.Name, metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	})
	if err != nil {
		return ConvertError(err)
	}

	err = waitForObjectDeletion(func() error {
		_, err := jobs.Get(ctx, c.Job.Name, metav1.GetOptions{})
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

func (c *JobControl) Upsert(ctx context.Context) error {
	c.Infof("upsert %v", formatMeta(c.Job.ObjectMeta))

	jobs := c.BatchV1().Jobs(c.Job.Namespace)
	currentJob, err := jobs.Get(ctx, c.Job.Name, metav1.GetOptions{})
	err = ConvertError(err)
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		// Get always returns an object
		currentJob = nil
	}

	if currentJob != nil {
		if checkCustomerManagedResource(currentJob.Annotations) {
			c.WithField("job", formatMeta(c.Job.ObjectMeta)).Info("Skipping update since object is customer managed.")
			return nil
		}

		control, err := NewJobControl(JobConfig{Job: currentJob, Clientset: c.Clientset})
		if err != nil {
			return ConvertError(err)
		}
		err = control.Delete(ctx, true)
		if err != nil {
			return ConvertError(err)
		}
	}

	c.Debug("creating new job")
	c.Job.UID = ""
	c.Job.SelfLink = ""
	c.Job.ResourceVersion = ""
	if c.Job.Spec.Selector != nil {
		// Remove auto-generated labels
		delete(c.Job.Spec.Selector.MatchLabels, ControllerUIDLabel)
		delete(c.Job.Spec.Template.Labels, ControllerUIDLabel)
	}

	err = withExponentialBackoff(func() error {
		_, err := jobs.Create(ctx, c.Job, metav1.CreateOptions{})
		return ConvertError(err)
	})
	return trace.Wrap(err)
}

func (c *JobControl) Status(ctx context.Context) error {
	jobs := c.BatchV1().Jobs(c.Job.Namespace)
	job, err := jobs.Get(ctx, c.Job.Name, metav1.GetOptions{})
	if err != nil {
		return ConvertError(err)
	}

	succeeded := job.Status.Succeeded
	active := job.Status.Active
	var complete bool
	if job.Spec.Completions == nil {
		// This type of job is complete when any pod exits with success
		if succeeded > 0 && active == 0 {
			complete = true
		}
	} else {
		// Job specifies a number of completions
		completions := *job.Spec.Completions
		if succeeded >= completions {
			complete = true
		}
	}

	if !complete {
		return trace.CompareFailed("job %v not yet complete (succeeded: %v, active: %v)",
			formatMeta(job.ObjectMeta), succeeded, active)
	}
	return nil
}

func (c *JobControl) collectPods(ctx context.Context, job *batchv1.Job) (map[string]v1.Pod, error) {
	var labels map[string]string
	if job.Spec.Selector != nil {
		labels = job.Spec.Selector.MatchLabels
	}
	pods, err := CollectPods(ctx, job.Namespace, labels, c.FieldLogger, c.Clientset, func(ref metav1.OwnerReference) bool {
		return ref.Kind == KindJob && ref.UID == job.UID
	})
	return pods, ConvertError(err)
}

type JobControl struct {
	JobConfig
	log.FieldLogger
}

type JobConfig struct {
	*batchv1.Job
	*kubernetes.Clientset
}

func (c *JobConfig) checkAndSetDefaults() error {
	if c.Job == nil {
		return trace.BadParameter("missing parameter Job")
	}
	if c.Clientset == nil {
		return trace.BadParameter("missing parameter Clientset")
	}
	updateTypeMetaJob(c.Job)
	return nil
}

func updateTypeMetaJob(r *batchv1.Job) {
	r.Kind = KindJob
	if r.APIVersion == "" {
		r.APIVersion = batchv1.SchemeGroupVersion.String()
	}
}
