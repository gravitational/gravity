package rigging

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"time"

	"github.com/gravitational/trace"

	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

type action string

const (
	ActionCreate  action = "create"
	ActionDelete  action = "delete"
	ActionReplace action = "replace"
	ActionApply   action = "apply"
)

// StatusReporter reports the status of the resource.
type StatusReporter interface {
	// Status returns the state of the resource.
	// Returns nil if successful (created/deleted/updated), otherwise an error
	Status(ctx context.Context) error
	// Infof logs the specified message and arguments in context of this resource
	Infof(message string, args ...interface{})
}

// KubeCommand returns an exec.Command for kubectl with the supplied arguments.
func KubeCommand(args ...string) *exec.Cmd {
	return exec.Command("/usr/local/bin/kubectl", args...)
}

// FromFile performs action on the Kubernetes resources specified in the path supplied as an argument.
func FromFile(act action, path string) ([]byte, error) {
	cmd := KubeCommand(string(act), "-f", path)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, trace.Wrap(err)
	}
	return out, nil
}

// FromStdin performs action on the Kubernetes resources specified in the string supplied as an argument.
func FromStdIn(act action, data string) ([]byte, error) {
	cmd := KubeCommand(string(act), "-f", "-")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var b bytes.Buffer
	cmd.Stdout = &b
	cmd.Stderr = &b

	if err := cmd.Start(); err != nil {
		return b.Bytes(), trace.Wrap(err)
	}

	io.WriteString(stdin, data)
	stdin.Close()

	if err := cmd.Wait(); err != nil {
		log.Errorf("%v", err)
		return b.Bytes(), trace.Wrap(err)
	}

	return b.Bytes(), nil
}

// PollStatus polls status periodically
func PollStatus(ctx context.Context, retryAttempts int, retryPeriod time.Duration, reporter StatusReporter) error {
	if retryAttempts == 0 {
		retryAttempts = DefaultRetryAttempts
	}
	if retryPeriod == 0 {
		retryPeriod = DefaultRetryPeriod
	}
	reporter.Infof("Checking status retryAttempts=%v, retryPeriod=%v", retryAttempts, retryPeriod)

	return retry(ctx, retryAttempts, retryPeriod, reporter.Status)
}

// CollectPods collects pods matched by fn
func CollectPods(ctx context.Context, namespace string, matchLabels map[string]string, logger log.FieldLogger, client *kubernetes.Clientset,
	fn func(metav1.OwnerReference) bool) (map[string]v1.Pod, error) {
	set := make(labels.Set)
	for key, val := range matchLabels {
		set[key] = val
	}

	podList, err := client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: set.AsSelector().String(),
	})
	if err != nil {
		return nil, ConvertError(err)
	}

	pods := make(map[string]v1.Pod, 0)
	for _, pod := range podList.Items {
		for _, ref := range pod.OwnerReferences {
			if fn(ref) {
				pods[pod.Spec.NodeName] = pod
				logger.Infof("found pod %v on node %v", formatMeta(pod.ObjectMeta), pod.Spec.NodeName)
			}
		}
	}
	return pods, nil
}

func retry(ctx context.Context, times int, period time.Duration, fn func(ctx context.Context) error) error {
	if times < 1 {
		return nil
	}
	err := fn(ctx)
	for i := 1; i < times && err != nil; i += 1 {
		log.Infof("attempt %v, result: %v, retry in %v", i+1, trace.DebugReport(err), period)
		select {
		case <-ctx.Done():
			log.Infof("context is closing, return")
			return err
		case <-time.After(period):
		}
		err = fn(ctx)
	}
	return err
}

func withRecover(fn func() error, recoverFn func() error) error {
	shouldRecover := true
	defer func() {
		if !shouldRecover {
			log.Infof("no recovery needed, returning")
			return
		}
		log.Infof("need to recover")
		err := recoverFn()
		if err != nil {
			log.Error(trace.DebugReport(err))
			return
		}
		log.Infof("recovered successfully")
		return
	}()

	if err := fn(); err != nil {
		return err
	}
	shouldRecover = false
	return nil
}

func nodeSelector(spec *v1.PodSpec) labels.Selector {
	set := make(labels.Set)
	for key, val := range spec.NodeSelector {
		set[key] = val
	}
	return set.AsSelector()
}

func checkRunning(pods map[string]v1.Pod, nodes []v1.Node, logger log.FieldLogger) error {
	ready, err := checkRunningAndReady(pods, nodes, logger)
	if ready || err == errPodCompleted {
		return nil
	}
	return trace.Wrap(err)
}

func checkRunningAndReady(pods map[string]v1.Pod, nodes []v1.Node, logger log.FieldLogger) (bool, error) {
	for _, node := range nodes {
		pod, ok := pods[node.Name]
		if !ok {
			logger.Infof("no pod found on node %v", node.Name)
			return false, trace.NotFound("no pod found on node %v", node.Name)
		}
		meta := formatMeta(pod.ObjectMeta)
		switch pod.Status.Phase {
		case v1.PodFailed, v1.PodSucceeded:
			logger.Infof("node %v: pod %v is %q", node.Name, meta, pod.Status.Phase)
			return false, errPodCompleted
		case v1.PodRunning:
			ready := isPodReadyConditionTrue(pod.Status)
			if ready {
				logger.Infof("node %v: pod %v is up and running", node.Name, meta)
			}
			return ready, nil
		default:
			return false, trace.CompareFailed("pod %v is not running yet, status: %q, ready: false",
				meta, pod.Status.Phase)
		}
	}
	return false, trace.NotFound("no pods %v found on any nodes %v", pods, nodes)
}

// errPodCompleted is returned by checkRunningAndReady to indicate that
// the pod has already reached completed state.
var errPodCompleted = fmt.Errorf("pod ran to completion")

// isPodReady retruns true if a pod is ready; false otherwise.
func isPodReadyConditionTrue(status v1.PodStatus) bool {
	_, condition := getPodCondition(&status, v1.PodReady)
	return condition != nil && condition.Status == v1.ConditionTrue
}

// getPodCondition extracts the provided condition from the given status and returns that.
// Returns nil and -1 if the condition is not present, and the index of the located condition.
func getPodCondition(status *v1.PodStatus, conditionType v1.PodConditionType) (int, *v1.PodCondition) {
	if status == nil {
		return -1, nil
	}
	for i := range status.Conditions {
		if status.Conditions[i].Type == conditionType {
			return i, &status.Conditions[i]
		}
	}
	return -1, nil
}

func ConvertError(err error) error {
	return ConvertErrorWithContext(err, "")
}

func ConvertErrorWithContext(err error, format string, args ...interface{}) error {
	if err == nil {
		return nil
	}
	statusErr, ok := err.(*errors.StatusError)
	if !ok {
		return err
	}

	message := fmt.Sprintf("%v", err)
	if !isEmptyDetails(statusErr.ErrStatus.Details) {
		message = fmt.Sprintf("%v, details: %v", message, statusErr.ErrStatus.Details)
	}
	if format != "" {
		message = fmt.Sprintf("%v: %v", fmt.Sprintf(format, args...), message)
	}

	status := statusErr.Status()
	switch {
	case status.Code == http.StatusConflict && status.Reason == metav1.StatusReasonAlreadyExists:
		return trace.AlreadyExists(message)
	case status.Code == http.StatusNotFound:
		return trace.NotFound(message)
	case status.Code == http.StatusForbidden:
		return trace.AccessDenied(message)
	}
	return err
}

func isEmptyDetails(details *metav1.StatusDetails) bool {
	if details == nil {
		return true
	}

	if details.Name == "" && details.Group == "" && details.Kind == "" && len(details.Causes) == 0 {
		return true
	}
	return false
}

// withExponentialBackoff retries the specified function fn exponentially.
// If fn returns an already exists error, the operation is retried - any other error
// aborts the execution.
// It expects fn to return errors converted to trace type hierarchy with ConvertError
func withExponentialBackoff(fn func() error) error {
	const initialDelay = 1 * time.Second
	backoff := wait.Backoff{
		Duration: initialDelay,
		Factor:   2.0,
		Steps:    10,
	}
	err := wait.ExponentialBackoff(backoff, func() (bool, error) {
		err := fn()
		if err != nil && !trace.IsAlreadyExists(err) {
			// abort
			return false, trace.Wrap(err)
		}
		if trace.IsAlreadyExists(err) {
			// retry
			return false, nil
		}
		return true, nil
	})
	return trace.Wrap(err)
}

func deletePodsList(ctx context.Context, podIface corev1.PodInterface, pods []v1.Pod, logger log.FieldLogger) error {
	for _, pod := range pods {
		logger.Debugf("deleting pod %v", pod.Name)
		err := ConvertError(podIface.Delete(ctx, pod.Name, metav1.DeleteOptions{}))
		if err != nil && !trace.IsNotFound(err) {
			return ConvertError(err)
		}
	}

	return trace.Wrap(waitForPodsList(ctx, podIface, pods))
}

func deletePods(ctx context.Context, podIface corev1.PodInterface, pods map[string]v1.Pod, logger log.FieldLogger) error {
	for _, pod := range pods {
		logger.Debugf("deleting pod %v", pod.Name)
		err := ConvertError(podIface.Delete(ctx, pod.Name, metav1.DeleteOptions{}))
		if err != nil && !trace.IsNotFound(err) {
			return ConvertError(err)
		}
	}

	return trace.Wrap(waitForPods(ctx, podIface, pods))
}

func waitForPodsList(ctx context.Context, podIface corev1.PodInterface, pods []v1.Pod) error {
	var errors []error
	for _, pod := range pods {
		err := waitForObjectDeletion(func() error {
			_, err := podIface.Get(ctx, pod.Name, metav1.GetOptions{})
			return ConvertError(err)
		})
		if err != nil {
			errors = append(errors, err)
		}
	}
	return trace.NewAggregate(errors...)
}

func waitForPods(ctx context.Context, podIface corev1.PodInterface, pods map[string]v1.Pod) error {
	var errors []error
	for _, pod := range pods {
		err := waitForObjectDeletion(func() error {
			_, err := podIface.Get(ctx, pod.Name, metav1.GetOptions{})
			return ConvertError(err)
		})
		if err != nil {
			errors = append(errors, err)
		}
	}
	return trace.NewAggregate(errors...)
}

func waitForObjectDeletion(fn func() error) error {
	return wait.PollImmediate(deletePollInterval, deleteTimeout, func() (bool, error) {
		switch err := fn(); {
		case err == nil:
			return false, nil
		case trace.IsNotFound(err):
			return true, nil
		default:
			return false, trace.Wrap(err)
		}
	})
}

const (
	deletePollInterval = 1 * time.Second
	deleteTimeout      = 10 * time.Minute
)

func checkCustomerManagedResource(annotations map[string]string) bool {
	_, ok := annotations[CustomerManagedAnnotation]
	return ok
}
