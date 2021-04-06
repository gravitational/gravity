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

package hooks

import (
	"context"
	"io"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/ghodss/yaml"
	"github.com/gravitational/rigging"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"gopkg.in/check.v1"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func writer(in io.Writer) io.Writer {
	if testing.Verbose() {
		return io.MultiWriter(in, os.Stdout)
	}
	return in
}

func TestService(t *testing.T) { check.TestingT(t) }

type HooksSuite struct{}

var _ = check.Suite(&HooksSuite{})

func (s *HooksSuite) SetUpSuite(c *check.C) {
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

func (s *HooksSuite) SetUpTest(c *check.C) {

	testEnabled := os.Getenv(defaults.TestK8s)
	if ok, _ := strconv.ParseBool(testEnabled); !ok {
		c.Skip("skipping Kubernetes test")
	}
}

func (s *HooksSuite) TestHookSuccess(c *check.C) {
	client, _, err := utils.GetLocalKubeClient()
	c.Assert(err, check.IsNil)
	c.Assert(client, check.NotNil)

	out := utils.NewSyncBuffer()
	defer out.Close()

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "kube-system",
		},
		Spec: batchv1.JobSpec{
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					RestartPolicy: v1.RestartPolicyOnFailure,
					Containers: []v1.Container{
						{
							Name:            "hello-1",
							Image:           "quay.io/gravitational/debian-grande:buster",
							Command:         []string{"/bin/bash", "-c", "echo 'hello, world 1'; sleep 1;"},
							ImagePullPolicy: v1.PullIfNotPresent,
						},
						{
							Name:            "hello-2",
							Image:           "quay.io/gravitational/debian-grande:buster",
							Command:         []string{"/bin/bash", "-c", "echo 'hello, world 2'; sleep 1;"},
							ImagePullPolicy: v1.PullIfNotPresent,
						},
					},
				},
			},
		},
	}
	job.APIVersion = batchv1.SchemeGroupVersion.String()
	job.Kind = rigging.KindJob

	jobBytes, err := yaml.Marshal(job)
	c.Assert(err, check.IsNil)

	hook := &schema.Hook{
		Type: schema.HookStatus,
		Job:  string(jobBytes),
	}
	runner, err := NewRunner(client)
	c.Assert(err, check.IsNil)
	c.Assert(runner, check.NotNil)

	ctx, cancel := context.WithTimeout(context.TODO(), 60*time.Second)
	defer cancel()
	params := Params{
		NodeSelector: map[string]string{},
		Locator: loc.Locator{
			Repository: "gravitational.io",
			Name:       "telekube",
			Version:    "0.0.0+latest",
		},
		Hook: hook,
	}
	ref, err := runner.Start(ctx, params)
	c.Assert(ctx.Err(), check.IsNil)
	c.Assert(err, check.IsNil)

	err = runner.StreamLogs(ctx, *ref, writer(out))
	c.Assert(ctx.Err(), check.IsNil)
	c.Assert(err, check.IsNil)
	c.Assert(utils.RemoveNewlines(out.String()), check.Matches, ".*hello, world 1.*")
	c.Assert(utils.RemoveNewlines(out.String()), check.Matches, ".*hello, world 2.*")

	err = runner.DeleteJob(context.TODO(), DeleteJobRequest{JobRef: *ref})
	c.Assert(err, check.IsNil)
}

// TestHookFailNewPods tests scenario when job recreates pods
// in the scenario when pod specifies restart policy never
func (s *HooksSuite) TestHookFailNewPods(c *check.C) {
	client, _, err := utils.GetLocalKubeClient()
	c.Assert(err, check.IsNil)
	c.Assert(client, check.NotNil)

	out := utils.NewSyncBuffer()
	defer out.Close()

	deadlineSeconds := int64((10 * time.Second).Seconds())
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "kube-system",
		},
		Spec: batchv1.JobSpec{
			ActiveDeadlineSeconds: &deadlineSeconds,
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					RestartPolicy: v1.RestartPolicyNever,
					Containers: []v1.Container{
						{
							Name:    "hello",
							Image:   "quay.io/gravitational/debian-grande:buster",
							Command: []string{"/bin/bash", "-c", "echo 'hello, world'; date; sleep 1; exit 255;"},
						},
					},
				},
			},
		},
	}
	job.APIVersion = batchv1.SchemeGroupVersion.String()
	job.Kind = rigging.KindJob

	jobBytes, err := yaml.Marshal(job)
	c.Assert(err, check.IsNil)

	hook := &schema.Hook{
		Type: schema.HookStatus,
		Job:  string(jobBytes),
	}
	runner, err := NewRunner(client)
	c.Assert(err, check.IsNil)
	c.Assert(runner, check.NotNil)

	ctx, cancel := context.WithTimeout(context.TODO(), 60*time.Second)
	defer cancel()

	params := Params{
		NodeSelector: map[string]string{},
		Locator: loc.Locator{
			Repository: "gravitational.io",
			Name:       "telekube",
			Version:    "0.0.0+latest",
		},
		Hook: hook,
	}
	ref, err := runner.Start(ctx, params)
	c.Assert(err, check.NotNil)
	c.Assert(ctx.Err(), check.IsNil, check.Commentf("unexpected timeout"))

	err = runner.StreamLogs(ctx, *ref, writer(out))
	c.Assert(err, check.NotNil)
	c.Assert(ctx.Err(), check.IsNil, check.Commentf("unexpected timeout"))
	output := out.String()
	comment := check.Commentf("expected more matches in %v", output)
	c.Assert(strings.Count(output, "hello, world") >= 2, check.Equals, true, comment)

	err = runner.DeleteJob(context.TODO(), DeleteJobRequest{JobRef: *ref})
	c.Assert(err, check.IsNil)
}

// TestHookFailRestart tests scenario when job recreates pods
// in the scenario when pod specifies restart policy on failure
func (s *HooksSuite) TestHookFailPodRestart(c *check.C) {
	client, _, err := utils.GetLocalKubeClient()
	c.Assert(err, check.IsNil)
	c.Assert(client, check.NotNil)

	out := utils.NewSyncBuffer()
	defer out.Close()

	deadlineSeconds := int64((10 * time.Second).Seconds())
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "kube-system",
		},
		Spec: batchv1.JobSpec{
			ActiveDeadlineSeconds: &deadlineSeconds,
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					RestartPolicy: v1.RestartPolicyOnFailure,
					Containers: []v1.Container{
						{
							Name:    "hello",
							Image:   "quay.io/gravitational/debian-grande:buster",
							Command: []string{"/bin/bash", "-c", "echo 'hello, world'; date; sleep 1; exit 255;"},
						},
					},
				},
			},
		},
	}
	job.APIVersion = batchv1.SchemeGroupVersion.String()
	job.Kind = rigging.KindJob

	jobBytes, err := yaml.Marshal(job)
	c.Assert(err, check.IsNil)

	hook := &schema.Hook{
		Type: schema.HookStatus,
		Job:  string(jobBytes),
	}
	runner, err := NewRunner(client)
	c.Assert(err, check.IsNil)
	c.Assert(runner, check.NotNil)

	ctx, cancel := context.WithTimeout(context.TODO(), 60*time.Second)
	defer cancel()
	params := Params{
		NodeSelector: map[string]string{},
		Locator: loc.Locator{
			Repository: "gravitational.io",
			Name:       "telekube",
			Version:    "0.0.0+latest",
		},
		Hook: hook,
	}
	ref, err := runner.Start(ctx, params)
	c.Assert(err, check.NotNil)
	c.Assert(ctx.Err(), check.IsNil, check.Commentf("unexpected timeout"))

	err = runner.StreamLogs(ctx, *ref, writer(out))
	c.Assert(err, check.NotNil)
	c.Assert(ctx.Err(), check.IsNil, check.Commentf("unexpected timeout"))
	output := out.String()
	comment := check.Commentf("expected more matches in %v", output)
	c.Assert(strings.Count(output, "hello, world") >= 2, check.Equals, true, comment)

	err = runner.DeleteJob(context.TODO(), DeleteJobRequest{JobRef: *ref})
	c.Assert(err, check.IsNil)
}
