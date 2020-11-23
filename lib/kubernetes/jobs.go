/*
Copyright 2020 Gravitational, Inc.

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
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"

	"github.com/gravitational/gravity/lib/app/hooks"
	"github.com/gravitational/gravity/lib/utils"
	"github.com/gravitational/gravity/lib/utils/kubectl"

	"github.com/gravitational/trace"
	"github.com/pborman/uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"text/template"
)

// MakeJobName generates a unique job name.
// k8s job names must be no more than 63 characters long.
// Expects that the prefix param will not be longer that 5 characters.
// The name param will be truncated if longer than 40 characters.
// Appends a short random string taken from UUID.
func MakeJobName(prefix string, name string) string {
	maxNameLen := 40
	if len(name) > maxNameLen {
		name = name[:maxNameLen]
	}

	return fmt.Sprintf("%v-%v-%v", prefix, name, uuid.New()[:13])
}

// ExecJob launches a Kubernetes job specified by a template.
// Uses kubectl to load the job spec yaml file.
// Waits for the job to complete and returns the output of the job.
func ExecJob(ctx context.Context, jobName string, namespace string, template *template.Template,
	templateData interface{}, client *kubernetes.Clientset) (string, error) {
	var buf bytes.Buffer
	err := template.Execute(&buf, templateData)
	if err != nil {
		return "", trace.Wrap(err)
	}

	jobFile := "job.yaml"
	err = ioutil.WriteFile(jobFile, buf.Bytes(), 0644)
	if err != nil {
		return "", trace.Wrap(err)
	}

	out, err := kubectl.Apply(jobFile)
	if err != nil {
		return fmt.Sprintf("Failed to exec kubectl: %v", string(out)), trace.Wrap(err)
	}

	runner, err := hooks.NewRunner(client)
	if err != nil {
		return "", trace.Wrap(err)
	}

	jobRef := hooks.JobRef{Name: jobName, Namespace: namespace}
	logs := utils.NewSyncBuffer()
	err = runner.StreamLogs(ctx, jobRef, logs)
	if err != nil {
		return logs.String(), trace.Wrap(err)
	}

	job, err := client.BatchV1().Jobs(jobRef.Namespace).Get(jobRef.Name, metav1.GetOptions{})
	if err != nil {
		return logs.String(), trace.Wrap(err)
	}

	if job.Status.Failed != 0 {
		return logs.String(), trace.Wrap(errors.New("k8s job has failed pods"))
	}

	return logs.String(), nil
}
