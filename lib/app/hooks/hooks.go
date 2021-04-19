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
	"fmt"
	"io"
	"time"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/cenkalti/backoff"
	"github.com/dustin/go-humanize"
	"github.com/ghodss/yaml"
	"github.com/gravitational/rigging"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	watch "k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	batch "k8s.io/client-go/kubernetes/typed/batch/v1"
	core "k8s.io/client-go/kubernetes/typed/core/v1"
)

// Runner encapsulates the process of running an application hook
type Runner struct {
	*log.Entry
	client *kubernetes.Clientset
}

// Params specifies hook parameters
type Params struct {
	// Hook is job hook
	Hook *schema.Hook
	// Locator is a locator for the app this runner is running hooks for.
	Locator loc.Locator
	// Env contains environment variables that will be made available inside
	// hooks containers
	Env map[string]string
	// Volumes lists additional volumes to add inside a hook job
	Volumes []v1.Volume
	// Mounts lists additional mounts to create inside a hook job
	Mounts []v1.VolumeMount
	// NodeSelector defines the optional set of labels to specify as a node selector
	// instead of the one used by default
	NodeSelector map[string]string
	// SkipInitContainers skips injection of init containers
	SkipInitContainers bool
	// HostNetwork tells the job to use the host network
	HostNetwork bool
	// JobDeadline allows to set specific hook job deadline
	JobDeadline time.Duration
	// AgentUser is the agent username for logging into local cluster
	AgentUser string
	// AgentPassword is the agent password for logging into local cluster
	AgentPassword string
	// GravityPackage references the gravity binary package to run hooks with.
	// If empty, the binary mapped from host is used.
	GravityPackage loc.Locator
	// ServiceUser specifies the service user which overrides the default
	// security context for the job's Pod
	ServiceUser storage.OSUser
	// Values are helm values in a marshaled yaml format
	Values []byte
}

// JobRef is a reference to a hook job
type JobRef struct {
	// Name is a job name
	Name string
	// Namespace is a namespace where job was launched
	Namespace string
}

// CheckAndSetDefaults checks and sets defaults for client
func (p *Params) CheckAndSetDefaults() error {
	if p.NodeSelector == nil {
		p.NodeSelector = map[string]string{
			schema.ServiceLabelRole: string(schema.ServiceRoleMaster),
		}
	}
	if p.Env == nil {
		p.Env = make(map[string]string)
	}
	if p.Hook == nil {
		return trace.BadParameter("missing parameter Hook")
	}
	if p.Locator.IsEmpty() {
		return trace.BadParameter("missing parameter Locator")
	}
	return nil
}

// NewRunner creates a new application hook runner instance
func NewRunner(client *kubernetes.Clientset) (*Runner, error) {
	if client == nil {
		return nil, trace.BadParameter("missing parameter client")
	}
	runner := &Runner{
		client: client,
		Entry: log.WithFields(log.Fields{
			trace.Component: constants.ComponentApp,
		}),
	}
	return runner, nil
}

// DeleteJobRequest combines parameters for job deletion.
type DeleteJobRequest struct {
	// JobRef identifies the job to delete.
	JobRef
	// Cascade specifies whether dependent objects should be deleted.
	Cascade bool
}

// DeleteJob deletes job by ref
func (r *Runner) DeleteJob(ctx context.Context, req DeleteJobRequest) error {
	var opts metav1.DeleteOptions
	if req.Cascade {
		propagationPolicy := metav1.DeletePropagationForeground
		opts = metav1.DeleteOptions{
			PropagationPolicy: &propagationPolicy,
		}
	}
	err := r.client.BatchV1().Jobs(req.Namespace).Delete(ctx, req.Name, opts)
	if err = rigging.ConvertError(err); err != nil {
		return err
	} else {
		r.Debugf("Deleted job %q in namespace %q.", req.Name, req.Namespace)
	}
	return nil
}

// Start configures and starts job, does not wait until job is
// complete or fails, the call does not wait for the job to complete
func (r *Runner) Start(ctx context.Context, p Params) (*JobRef, error) {
	if err := p.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	job, err := p.Hook.GetJob()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// configureJob injects telekube specific mounts
	// and init containers
	if err = configureJob(job, p); err != nil {
		return nil, trace.Wrap(err)
	}

	jobNamespace := job.ObjectMeta.Namespace

	// marshal the updated job spec back into yaml
	// and log it
	jobBytes, err := yaml.Marshal(job)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	r.Debug(string(jobBytes), ".")

	// try to create the namespace and ignore "already exists" errors
	_, err = r.client.CoreV1().Namespaces().Create(ctx, &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: jobNamespace,
		},
	}, metav1.CreateOptions{})
	if err = rigging.ConvertError(err); err != nil {
		if !trace.IsAlreadyExists(err) {
			return nil, trace.Wrap(err)
		}
	}

	job, err = r.client.BatchV1().Jobs(jobNamespace).Create(ctx, job, metav1.CreateOptions{})
	if err = rigging.ConvertError(err); err != nil {
		return nil, trace.Wrap(err)
	}

	return &JobRef{Namespace: job.Namespace, Name: job.Name}, nil
}

// findSuccess finds condition that indicates job completion
func findSuccess(job batchv1.Job) *batchv1.JobCondition {
	for i := range job.Status.Conditions {
		condition := job.Status.Conditions[i]
		if condition.Type == batchv1.JobComplete {
			return &condition
		}
	}
	return nil
}

// findFailure returns failed condition if it's present
func findFailure(job batchv1.Job) *batchv1.JobCondition {
	for i := range job.Status.Conditions {
		condition := job.Status.Conditions[i]
		if condition.Type == batchv1.JobFailed {
			return &condition
		}
	}
	return nil
}

// Wait waits for job to complete or fail, cancel on the context cancels
// the wait call that is otherwise blocking
func (r *Runner) Wait(ctx context.Context, ref JobRef) error {
	interval := utils.NewUnlimitedExponentialBackOff()
	err := utils.RetryWithInterval(ctx, interval, func() error {
		watcher, err := newJobWatch(r.client.BatchV1(), ref)
		if err != nil {
			return &backoff.PermanentError{Err: err}
		}
		err = r.evalJobStatus(ctx, watcher.ResultChan())
		watcher.Stop()
		if err != nil && !trace.IsRetryError(err) {
			return &backoff.PermanentError{Err: err}
		}
		return trace.Wrap(err)
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// StreamLogs streams logs until the job is either failed or done
func (r *Runner) StreamLogs(ctx context.Context, ref JobRef, out io.Writer) error {
	localContext, localCancel := context.WithCancel(ctx)
	defer localCancel()

	job, err := r.client.BatchV1().Jobs(ref.Namespace).Get(ctx, ref.Name, metav1.GetOptions{})
	if err != nil {
		return rigging.ConvertError(err)
	}

	jobControl, err := rigging.NewJobControl(rigging.JobConfig{
		Job:       job,
		Clientset: r.client,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	go func() {
		defer localCancel()
		err := r.Wait(localContext, ref)
		if err != nil {
			log.Warningf("Hook finished with error: %v.", trace.DebugReport(err))
		}
	}()

	interval := utils.NewUnlimitedExponentialBackOff()
	err = utils.RetryWithInterval(ctx, interval, func() error {
		watcher, err := newPodWatch(r.client.CoreV1(), ref)
		if err != nil {
			return &backoff.PermanentError{Err: err}
		}
		err = r.monitorPods(localContext, watcher.ResultChan(), *job, *jobControl, out)
		watcher.Stop()
		if err != nil && !trace.IsRetryError(err) {
			return &backoff.PermanentError{Err: err}
		}
		return trace.Wrap(err)
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (r *Runner) monitorPods(ctx context.Context, eventsC <-chan watch.Event,
	job batchv1.Job, jobControl rigging.JobControl, w io.Writer) error {
	// podSet keeps state of currently monitored pods
	podSet := map[string]v1.Pod{}
	start := time.Now()

	err := r.checkJob(ctx, &job, &jobControl, podSet, w)
	diff := humanize.RelTime(start, time.Now(), "elapsed", "elapsed")
	if err == nil {
		fmt.Fprintf(w, "%v has completed, %v.\n", describe(&job), diff)
		return nil
	}
	log.Debugf("%v: %v", diff, err)

	for {
		select {
		case event, ok := <-eventsC:
			if !ok {
				return trace.Retry(nil, "events channel closed")
			}
			log := r.WithField("event", event.Type)
			log.Debug(describe(event.Object))
			diff = humanize.RelTime(start, time.Now(), "elapsed", "elapsed")
			err = r.checkJob(ctx, &job, &jobControl, podSet, w)
			if err == nil {
				fmt.Fprintf(w, "%v has completed, %v.\n", describe(&job), diff)
				return nil
			}
			log.Debugf("%v: %v", diff, err)
		case <-ctx.Done():
			return nil
		}
	}
}

// checkJob checks job for new pods arrivals and returns job status
func (r *Runner) checkJob(ctx context.Context, job *batchv1.Job, jobControl *rigging.JobControl, podSet map[string]v1.Pod, out io.Writer) error {
	newSet, err := r.collectPods(job)
	if err != nil {
		return trace.Wrap(err)
	}
	diffs := diffPodSets(podSet, newSet)
	for _, diff := range diffs {
		fmt.Fprintln(out, diff.String())
		pod := *diff.new
		// record new version of the pod state
		podSet[pod.Name] = pod
		for _, containerDiff := range diff.containers {
			// stream logs for running containers
			if containerDiff.new.State.Running != nil {
				go func() {
					if err := r.streamPodContainerLogs(ctx, &pod, containerDiff.name, out); err != nil {
						r.WithError(err).Warn("Failed to stream container logs.")
					}
				}()
			}
		}
	}

	return jobControl.Status(ctx)
}

func (r *Runner) evalJobStatus(ctx context.Context, eventsC <-chan watch.Event) error {
	for {
		select {
		case event, ok := <-eventsC:
			if !ok {
				return trace.Retry(nil, "events channel closed")
			}
			log := r.WithField("event", event.Type)
			log.Debug(describe(event.Object))
			job, ok := event.Object.(*batchv1.Job)
			if !ok {
				log.Warnf("Unexpected resource type: %T?, expected %T.", event.Object, job)
				continue
			}
			if success := findSuccess(*job); success != nil {
				log.Debugf("Completed: %v.", success.Message)
				return nil
			}
			if failure := findFailure(*job); failure != nil {
				log.Debugf("Failed: %v.", failure.Message)
				return trace.BadParameter(failure.Message)
			}
		case <-ctx.Done():
			return nil
		}
	}
}

func newJobWatch(client batch.BatchV1Interface, ref JobRef) (watch.Interface, error) {
	watcher, err := client.Jobs(ref.Namespace).Watch(context.TODO(), metav1.ListOptions{
		TypeMeta: metav1.TypeMeta{
			Kind: rigging.KindJob,
		},
		FieldSelector: fields.Set{"metadata.name": ref.Name}.String(),
		Watch:         true,
	})
	if err != nil {
		return nil, rigging.ConvertError(err)
	}
	return watcher, nil
}

func newPodWatch(client core.CoreV1Interface, ref JobRef) (watch.Interface, error) {
	watcher, err := client.Pods(ref.Namespace).Watch(context.TODO(), metav1.ListOptions{
		TypeMeta: metav1.TypeMeta{
			Kind: "Pod",
		},
		LabelSelector: labels.Set{"job-name": ref.Name}.String(),
		Watch:         true,
	})
	if err != nil {
		return nil, rigging.ConvertError(err)
	}
	return watcher, nil
}

func podSelector(job *batchv1.Job) labels.Set {
	var selector map[string]string
	if job.Spec.Selector != nil {
		selector = job.Spec.Selector.MatchLabels
	}
	set := make(labels.Set)
	for key, val := range selector {
		set[key] = val
	}
	return set
}

// collectPods collects pods created by this job and returns map
// with podName: pod pairs
func (r *Runner) collectPods(job *batchv1.Job) (map[string]v1.Pod, error) {
	set := podSelector(job)
	podList, err := r.client.CoreV1().Pods(job.Namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: set.AsSelector().String(),
	})
	if err != nil {
		return nil, rigging.ConvertError(err)
	}
	pods := make(map[string]v1.Pod)
	for _, pod := range podList.Items {
		for _, ref := range pod.OwnerReferences {
			if ref.Kind == rigging.KindJob && ref.UID == job.UID {
				pods[pod.Name] = pod
			}
		}
	}

	return pods, nil
}

// streamPodLogs attempts to stream pod logs to the provided out writer
func (r *Runner) streamPodContainerLogs(ctx context.Context, pod *v1.Pod, containerName string, out io.Writer) error {
	r.Debugf("Start streaming logs for %q, container %q.", describe(pod), containerName)
	defer r.Debugf("Stopped streaming logs for %q, container %q.", describe(pod), containerName)
	req := r.client.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &v1.PodLogOptions{
		Container: containerName,
		Follow:    true,
	})
	readCloser, err := req.Stream(ctx)
	if err != nil {
		r.Warningf("Failed to stream: %v.", err)
		return trace.Wrap(err)
	}
	localContext, localCancel := context.WithCancel(ctx)
	go func() {
		defer localCancel()
		bytes, err := io.Copy(out, readCloser)
		r.Debugf("Copy finished: copied: %v, result: %v.", bytes, err)
		if err != nil && !utils.IsStreamClosedError(err) {
			r.Warningf("Failed to complete copy: %v.", trace.DebugReport(err))
		}
	}()
	<-localContext.Done()
	// we are closing reader on local completion or higher level cancel
	// depending on what arrives first
	readCloser.Close()
	return nil
}
