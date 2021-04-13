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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"
	"time"

	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	monitoring "github.com/coreos/prometheus-operator/pkg/client/versioned"
	monitoring_scheme "github.com/coreos/prometheus-operator/pkg/client/versioned/scheme"
	goyaml "github.com/ghodss/yaml"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	runtimeutil "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	apiregistration "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset"
)

func init() {
	runtimeutil.Must(monitoringv1.AddToScheme(scheme.Scheme))
	runtimeutil.Must(apiregistrationv1.AddToScheme(scheme.Scheme))
}

type ChangesetConfig struct {
	// Client is k8s client
	Client *kubernetes.Clientset
	// Config is rest client config
	Config *rest.Config
}

func (c *ChangesetConfig) CheckAndSetDefaults() error {
	if c.Client == nil {
		return trace.BadParameter("missing parameter Client")
	}
	if c.Config == nil {
		return trace.BadParameter("missing parameter Config")
	}
	return nil
}

func NewChangeset(ctx context.Context, config ChangesetConfig) (*Changeset, error) {
	if err := config.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	cfg := *config.Config
	cfg.APIPath = "/apis"
	if cfg.UserAgent == "" {
		cfg.UserAgent = rest.DefaultKubernetesUserAgent()
	}

	cfg.NegotiatedSerializer = monitoring_scheme.Codecs.WithoutConversion()
	cfg.GroupVersion = &schema.GroupVersion{Group: ChangesetGroup, Version: ChangesetVersion}

	clt, err := rest.RESTClientFor(&cfg)
	if err != nil {
		return nil, ConvertError(err)
	}

	apiExtensionsClient, err := apiextensionsclientset.NewForConfig(&cfg)
	if err != nil {
		return nil, ConvertError(err)
	}

	monitoringClient, err := monitoring.NewForConfig(&cfg)
	if err != nil {
		return nil, ConvertError(err)
	}

	apiRegistrationClient, err := apiregistration.NewForConfig(&cfg)
	if err != nil {
		return nil, ConvertError(err)
	}

	cs := &Changeset{
		ChangesetConfig:       config,
		client:                clt,
		APIExtensionsClient:   apiExtensionsClient,
		APIRegistrationClient: apiRegistrationClient,
		MonitoringClient:      monitoringClient,
	}
	if err := cs.Init(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	return cs, nil
}

// Changeset is a is a collection changeset log that can revert a series of
// changes to the system
type Changeset struct {
	ChangesetConfig
	client *rest.RESTClient
	// APIExtensionsClient is a client for the extensions server
	APIExtensionsClient *apiextensionsclientset.Clientset
	// APIRegistrationClient is Kube aggregator clientset
	APIRegistrationClient *apiregistration.Clientset
	// MonitoringClient is Prometheus operator client
	MonitoringClient *monitoring.Clientset
}

// Upsert upserts resource in a context of a changeset
func (cs *Changeset) Upsert(ctx context.Context, changesetNamespace, changesetName string, data []byte) error {
	// To support re-entrant calls to Upsert, we need to check to see if the last operation in the changeset is
	// incomplete. If it is incomplete, we roll it back before continuing.
	err := cs.revertIncompleteOperation(ctx, changesetNamespace, changesetName)
	if err != nil {
		return trace.Wrap(err)
	}

	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(data), DefaultBufferSize)

	for {
		var raw runtime.Unknown
		err := decoder.Decode(&raw)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return trace.Wrap(err)
		}
		err = cs.upsertResource(ctx, changesetNamespace, changesetName, raw.Raw)
		if err != nil {
			return trace.Wrap(err)
		}
	}
}

func (cs *Changeset) upsertResource(ctx context.Context, changesetNamespace, changesetName string, data []byte) error {
	tr, err := cs.createOrRead(ctx, changesetNamespace, changesetName, ChangesetSpec{Status: ChangesetStatusInProgress})
	if err != nil {
		return trace.Wrap(err)
	}
	if tr.Spec.Status != ChangesetStatusInProgress {
		return trace.CompareFailed("cannot update changeset - expected status %q, got %q", ChangesetStatusInProgress, tr.Spec.Status)
	}
	var kind metav1.TypeMeta
	err = yaml.NewYAMLOrJSONDecoder(bytes.NewReader(data), DefaultBufferSize).Decode(&kind)
	if err != nil {
		return trace.Wrap(err)
	}
	switch kind.Kind {
	case KindJob:
		_, err = cs.upsertJob(ctx, tr, data)
	case KindDaemonSet:
		_, err = cs.upsertDaemonSet(ctx, tr, data)
	case KindStatefulSet:
		_, err = cs.upsertStatefulSet(ctx, tr, data)
	case KindReplicationController:
		_, err = cs.upsertRC(ctx, tr, data)
	case KindDeployment:
		_, err = cs.upsertDeployment(ctx, tr, data)
	case KindService:
		_, err = cs.upsertService(ctx, tr, data)
	case KindServiceAccount:
		_, err = cs.upsertServiceAccount(ctx, tr, data)
	case KindConfigMap:
		_, err = cs.upsertConfigMap(ctx, tr, data)
	case KindSecret:
		_, err = cs.upsertSecret(ctx, tr, data)
	case KindRole:
		_, err = cs.upsertRole(ctx, tr, data)
	case KindClusterRole:
		_, err = cs.upsertClusterRole(ctx, tr, data)
	case KindRoleBinding:
		_, err = cs.upsertRoleBinding(ctx, tr, data)
	case KindClusterRoleBinding:
		_, err = cs.upsertClusterRoleBinding(ctx, tr, data)
	case KindPodSecurityPolicy:
		_, err = cs.upsertPodSecurityPolicy(ctx, tr, data)
	case KindCustomResourceDefinition:
		_, err = cs.upsertCustomResourceDefinition(ctx, tr, data)
	case KindNamespace:
		_, err = cs.upsertNamespace(ctx, tr, data)
	case KindPriorityClass:
		_, err = cs.upsertPriorityClass(ctx, tr, data)
	case KindValidatingWebhookConfiguration:
		_, err = cs.upsertValidatingWebhookConfiguration(ctx, tr, data)
	case KindMutatingWebhookConfiguration:
		_, err = cs.upsertMutatingWebhookConfiguration(ctx, tr, data)
	case KindAPIService:
		_, err = cs.upsertAPIService(ctx, tr, data)
	case KindServiceMonitor:
		_, err = cs.upsertServiceMonitor(ctx, tr, data)
	case KindAlertmanager:
		_, err = cs.upsertAlertmanager(ctx, tr, data)
	case KindPrometheus:
		_, err = cs.upsertPrometheus(ctx, tr, data)
	case KindPrometheusRule:
		_, err = cs.upsertPrometheusRule(ctx, tr, data)
	default:
		return trace.BadParameter("unsupported resource type %v", kind.Kind)
	}
	return err
}

// Status checks all statuses for all resources updated or added in the context of a given changeset
func (cs *Changeset) Status(ctx context.Context, changesetNamespace, changesetName string, retryAttempts int, retryPeriod time.Duration) error {
	tr, err := cs.get(ctx, changesetNamespace, changesetName)
	if err != nil {
		return trace.Wrap(err)
	}

	// Fast path for certain states
	switch tr.Spec.Status {
	case ChangesetStatusCommitted:
		// Nothing to do
		return nil
	}

	if retryAttempts == 0 {
		retryAttempts = DefaultRetryAttempts
	}

	if retryPeriod == 0 {
		retryPeriod = DefaultRetryPeriod
	}

	// If any operation in the changeset is incomplete, the status won't update in the retry loop to be complete.
	// So early exit if the changeset will not pass the status check in its current state.
	for _, op := range tr.Spec.Items {
		if op.Status == OpStatusCreated {
			return trace.BadParameter("%v is not completed yet. Changelog needs to be rolled back.", tr)
		}
	}

	return retry(ctx, retryAttempts, retryPeriod, func(context.Context) error {
		for _, op := range tr.Spec.Items {
			switch op.Status {
			case OpStatusCreated:
				return trace.BadParameter("%v is not completed yet", tr)
			case OpStatusCompleted, OpStatusReverted:
				if op.To != "" {
					err := cs.status(ctx, []byte(op.To), "")
					if err != nil {
						if op.Status != OpStatusReverted || !trace.IsNotFound(err) {
							return trace.Wrap(err)
						}
					}
				} else {
					info, err := GetOperationInfo(op)
					if err != nil {
						return trace.Wrap(err)
					}
					err = cs.status(ctx, []byte(op.From), op.UID)
					if err == nil || !trace.IsNotFound(err) {
						return trace.CompareFailed("%v with UID %q still active: %v",
							formatMeta(info.From.ObjectMeta), op.UID, err)
					}
				}
			default:
				return trace.BadParameter("unsupported operation status: %v", op.Status)
			}
		}
		return nil
	})
}

// DeleteResource deletes a resources in the context of a given changeset
func (cs *Changeset) DeleteResource(ctx context.Context, changesetNamespace, changesetName string, resourceNamespace string, resource Ref, cascade bool) error {
	tr, err := cs.createOrRead(ctx, changesetNamespace, changesetName, ChangesetSpec{Status: ChangesetStatusInProgress})
	if err != nil {
		return trace.Wrap(err)
	}
	if tr.Spec.Status != ChangesetStatusInProgress {
		return trace.CompareFailed("cannot update changeset - expected status %q, got %q", ChangesetStatusInProgress, tr.Spec.Status)
	}
	log := log.WithFields(log.Fields{
		"cs": tr.String(),
	})
	log.Infof("Deleting %v/%s", resourceNamespace, resource)
	switch resource.Kind {
	case KindDaemonSet:
		return cs.deleteDaemonSet(ctx, tr, resourceNamespace, resource.Name, cascade)
	case KindStatefulSet:
		return cs.deleteStatefulSet(ctx, tr, resourceNamespace, resource.Name, cascade)
	case KindJob:
		return cs.deleteJob(ctx, tr, resourceNamespace, resource.Name, cascade)
	case KindReplicationController:
		return cs.deleteRC(ctx, tr, resourceNamespace, resource.Name, cascade)
	case KindDeployment:
		return cs.deleteDeployment(ctx, tr, resourceNamespace, resource.Name, cascade)
	case KindSecret:
		return cs.deleteSecret(ctx, tr, resourceNamespace, resource.Name, cascade)
	case KindConfigMap:
		return cs.deleteConfigMap(ctx, tr, resourceNamespace, resource.Name, cascade)
	case KindService:
		return cs.deleteService(ctx, tr, resourceNamespace, resource.Name, cascade)
	case KindServiceAccount:
		return cs.deleteServiceAccount(ctx, tr, resourceNamespace, resource.Name, cascade)
	case KindRole:
		return cs.deleteRole(ctx, tr, resourceNamespace, resource.Name, cascade)
	case KindClusterRole:
		return cs.deleteClusterRole(ctx, tr, resource.Name, cascade)
	case KindRoleBinding:
		return cs.deleteRoleBinding(ctx, tr, resourceNamespace, resource.Name, cascade)
	case KindClusterRoleBinding:
		return cs.deleteClusterRoleBinding(ctx, tr, resource.Name, cascade)
	case KindPodSecurityPolicy:
		return cs.deletePodSecurityPolicy(ctx, tr, resource.Name, cascade)
	case KindCustomResourceDefinition:
		return cs.deleteCustomResourceDefinition(ctx, tr, resource.Name, cascade)
	case KindNamespace:
		return cs.deleteNamespace(ctx, tr, resource.Name, cascade)
	case KindPriorityClass:
		return cs.deletePriorityClass(ctx, tr, resource.Name, cascade)
	case KindValidatingWebhookConfiguration:
		return cs.deleteValidatingWebhookConfiguration(ctx, tr, resource.Name, cascade)
	case KindMutatingWebhookConfiguration:
		return cs.deleteMutatingWebhookConfiguration(ctx, tr, resource.Name, cascade)
	case KindAPIService:
		return cs.deleteAPIService(ctx, tr, resource.Name, cascade)
	case KindServiceMonitor:
		return cs.deleteServiceMonitor(ctx, tr, resourceNamespace, resource.Name, cascade)
	case KindAlertmanager:
		return cs.deleteAlertmanager(ctx, tr, resourceNamespace, resource.Name, cascade)
	case KindPrometheus:
		return cs.deletePrometheus(ctx, tr, resourceNamespace, resource.Name, cascade)
	case KindPrometheusRule:
		return cs.deletePrometheusRule(ctx, tr, resourceNamespace, resource.Name, cascade)
	}
	return trace.BadParameter("delete: unimplemented resource %v", resource.Kind)
}

// Freeze "freezes" changeset, prohibits adding or removing any changes to it
func (cs *Changeset) Freeze(ctx context.Context, changesetNamespace, changesetName string) error {
	tr, err := cs.get(ctx, changesetNamespace, changesetName)
	if err != nil {
		return trace.Wrap(err)
	}
	if tr.Spec.Status != ChangesetStatusInProgress {
		return trace.CompareFailed("changeset is not in progress")
	}
	for i := len(tr.Spec.Items) - 1; i >= 0; i-- {
		item := &tr.Spec.Items[i]
		if item.Status != OpStatusCompleted {
			return trace.CompareFailed("operation %v is not completed", i)
		}
	}
	tr.Spec.Status = ChangesetStatusCommitted
	_, err = cs.update(ctx, tr)
	return trace.Wrap(err)
}

// Revert rolls back all the operations in reverse order they were applied
func (cs *Changeset) Revert(ctx context.Context, changesetNamespace, changesetName string) error {
	tr, err := cs.get(ctx, changesetNamespace, changesetName)
	if err != nil {
		return trace.Wrap(err)
	}
	if tr.Spec.Status == ChangesetStatusReverted {
		return trace.CompareFailed("changeset is already reverted")
	}
	log := log.WithFields(log.Fields{
		"cs": tr.String(),
	})
	for i := len(tr.Spec.Items) - 1; i >= 0; i-- {
		op := &tr.Spec.Items[i]

		// Reentrancy: skip any phases that may already be reverted by a previous rollback
		if op.Status == OpStatusReverted {
			continue
		}

		info, err := GetOperationInfo(*op)
		if err != nil {
			return trace.Wrap(err)
		}

		log.Infof("Reverting changeset item %v, status: %v ", info, op.Status)
		if err := cs.revert(ctx, op, info); err != nil {
			return trace.Wrap(err)
		}

		op.Status = OpStatusReverted

		tr, err = cs.update(ctx, tr)
		if err != nil {
			return trace.Wrap(err)
		}

	}

	tr.Spec.Status = ChangesetStatusReverted
	_, err = cs.update(ctx, tr)
	return trace.Wrap(err)
}

// revertIncompleteOperation checks and rolls back the last operation in the changeset if it's incomplete.
func (cs *Changeset) revertIncompleteOperation(ctx context.Context, changesetNamespace, changesetName string) error {
	tr, err := cs.get(ctx, changesetNamespace, changesetName)
	if err != nil {
		if trace.IsNotFound(err) {
			return nil
		}
		return trace.Wrap(err)
	}

	if tr.Spec.Status == ChangesetStatusReverted {
		return nil
	}

	if len(tr.Spec.Items) == 0 {
		return nil
	}

	log := log.WithFields(log.Fields{
		"cs": tr.String(),
	})

	op := &tr.Spec.Items[len(tr.Spec.Items)-1]
	if op.Status != OpStatusCreated {
		return nil
	}

	info, err := GetOperationInfo(*op)
	if err != nil {
		return trace.Wrap(err)
	}

	log.Infof("Reverting incomplete changeset item %v, status: %v ", info, op.Status)
	if err := cs.revert(ctx, op, info); err != nil {
		return trace.Wrap(err)
	}

	// Remove the item from the changelog that we rolled back so `rig freeze` doesn't see the changelog as
	// incomplete.
	tr.Spec.Items = tr.Spec.Items[:len(tr.Spec.Items)-1]

	tr, err = cs.update(ctx, tr)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (cs *Changeset) status(ctx context.Context, data []byte, uid string) error {
	header, err := ParseResourceHeader(bytes.NewReader(data))
	if err != nil {
		return trace.Wrap(err)
	}
	switch header.Kind {
	case KindDaemonSet:
		return cs.statusDaemonSet(ctx, data, uid)
	case KindStatefulSet:
		return cs.statusStatefulSet(ctx, data, uid)
	case KindJob:
		return cs.statusJob(ctx, data, uid)
	case KindReplicationController:
		return cs.statusRC(ctx, data, uid)
	case KindDeployment:
		return cs.statusDeployment(ctx, data, uid)
	case KindService:
		return cs.statusService(ctx, data, uid)
	case KindServiceAccount:
		return cs.statusServiceAccount(ctx, data, uid)
	case KindSecret:
		return cs.statusSecret(ctx, data, uid)
	case KindConfigMap:
		return cs.statusConfigMap(ctx, data, uid)
	case KindRole:
		return cs.statusRole(ctx, data, uid)
	case KindClusterRole:
		return cs.statusClusterRole(ctx, data, uid)
	case KindRoleBinding:
		return cs.statusRoleBinding(ctx, data, uid)
	case KindClusterRoleBinding:
		return cs.statusClusterRoleBinding(ctx, data, uid)
	case KindPodSecurityPolicy:
		return cs.statusPodSecurityPolicy(ctx, data, uid)
	case KindCustomResourceDefinition:
		return cs.statusCustomResourceDefinition(ctx, data, uid)
	case KindNamespace:
		return cs.statusNamespace(ctx, data, uid)
	case KindPriorityClass:
		return cs.statusPriorityClass(ctx, data, uid)
	case KindValidatingWebhookConfiguration:
		return cs.statusValidatingWebhookConfiguration(ctx, data, uid)
	case KindMutatingWebhookConfiguration:
		return cs.statusMutatingWebhookConfiguration(ctx, data, uid)
	case KindAPIService:
		return cs.statusAPIService(ctx, data, uid)
	case KindServiceMonitor:
		return cs.statusServiceMonitor(ctx, data, uid)
	case KindAlertmanager:
		return cs.statusAlertmanager(ctx, data, uid)
	case KindPrometheus:
		return cs.statusPrometheus(ctx, data, uid)
	case KindPrometheusRule:
		return cs.statusPrometheusRule(ctx, data, uid)
	}
	return trace.BadParameter("unsupported resource type %v for resource %v", header.Kind, header.Name)
}

func (cs *Changeset) statusDaemonSet(ctx context.Context, data []byte, uid string) error {
	daemonset, err := ParseDaemonSet(bytes.NewReader(data))
	if err != nil {
		return trace.Wrap(err)
	}
	if uid != "" {
		existing, err := cs.Client.AppsV1().DaemonSets(daemonset.Namespace).Get(ctx, daemonset.Name, metav1.GetOptions{})
		if err != nil {
			return ConvertError(err)
		}
		if string(existing.GetUID()) != uid {
			return trace.NotFound("daemonset with UID %v not found", uid)
		}
	}
	control, err := NewDaemonSetControl(DSConfig{DaemonSet: daemonset, Client: cs.Client})
	if err != nil {
		return trace.Wrap(err)
	}
	return control.Status(ctx)
}

func (cs *Changeset) statusStatefulSet(ctx context.Context, data []byte, uid string) error {
	statefulSet, err := ParseStatefulSet(bytes.NewReader(data))
	if err != nil {
		return trace.Wrap(err)
	}
	if uid != "" {
		existing, err := cs.Client.AppsV1().StatefulSets(statefulSet.Namespace).Get(ctx, statefulSet.Name, metav1.GetOptions{})
		if err != nil {
			return ConvertError(err)
		}
		if string(existing.GetUID()) != uid {
			return trace.NotFound("statefulset with UID %v not found", uid)
		}
	}
	control, err := NewStatefulSetControl(StatefulSetConfig{StatefulSet: statefulSet, Client: cs.Client})
	if err != nil {
		return trace.Wrap(err)
	}
	return control.Status(ctx)
}

func (cs *Changeset) statusJob(ctx context.Context, data []byte, uid string) error {
	job, err := ParseJob(bytes.NewReader(data))
	if err != nil {
		return trace.Wrap(err)
	}
	if uid != "" {
		existing, err := cs.Client.BatchV1().Jobs(job.Namespace).Get(ctx, job.Name, metav1.GetOptions{})
		if err != nil {
			return ConvertError(err)
		}
		if string(existing.GetUID()) != uid {
			return trace.NotFound("job with UID %v not found", uid)
		}
	}
	control, err := NewJobControl(JobConfig{Job: job, Clientset: cs.Client})
	if err != nil {
		return trace.Wrap(err)
	}
	return control.Status(ctx)
}

func (cs *Changeset) statusRC(ctx context.Context, data []byte, uid string) error {
	rc, err := ParseReplicationController(bytes.NewReader(data))
	if err != nil {
		return trace.Wrap(err)
	}
	if uid != "" {
		existing, err := cs.Client.CoreV1().ReplicationControllers(rc.Namespace).Get(ctx, rc.Name, metav1.GetOptions{})

		if err != nil {
			return ConvertError(err)
		}
		if string(existing.GetUID()) != uid {
			return trace.NotFound("replication controller with UID %v not found", uid)
		}
	}
	control, err := NewReplicationControllerControl(RCConfig{ReplicationController: rc, Client: cs.Client})
	if err != nil {
		return trace.Wrap(err)
	}
	return control.Status(ctx)
}

func (cs *Changeset) statusDeployment(ctx context.Context, data []byte, uid string) error {
	deployment, err := ParseDeployment(bytes.NewReader(data))
	if err != nil {
		return trace.Wrap(err)
	}
	if uid != "" {
		existing, err := cs.Client.AppsV1().Deployments(deployment.Namespace).Get(ctx, deployment.Name, metav1.GetOptions{})
		if err != nil {
			return ConvertError(err)
		}
		if string(existing.GetUID()) != uid {
			return trace.NotFound("deployment with UID %v not found", uid)
		}
	}
	control, err := NewDeploymentControl(DeploymentConfig{Deployment: deployment, Client: cs.Client})
	if err != nil {
		return trace.Wrap(err)
	}
	return control.Status(ctx)
}

func (cs *Changeset) statusService(ctx context.Context, data []byte, uid string) error {
	service, err := ParseService(bytes.NewReader(data))
	if err != nil {
		return trace.Wrap(err)
	}
	if uid != "" {
		existing, err := cs.Client.CoreV1().Services(service.Namespace).Get(ctx, service.Name, metav1.GetOptions{})
		if err != nil {
			return ConvertError(err)
		}
		if string(existing.GetUID()) != uid {
			return trace.NotFound("service with UID %v not found", uid)
		}
	}
	control, err := NewServiceControl(ServiceConfig{Service: service, Client: cs.Client})
	if err != nil {
		return trace.Wrap(err)
	}
	return control.Status(ctx)
}

func (cs *Changeset) statusSecret(ctx context.Context, data []byte, uid string) error {
	secret, err := ParseSecret(bytes.NewReader(data))
	if err != nil {
		return trace.Wrap(err)
	}
	if uid != "" {
		existing, err := cs.Client.CoreV1().Secrets(secret.Namespace).Get(ctx, secret.Name, metav1.GetOptions{})
		if err != nil {
			return ConvertError(err)
		}
		if string(existing.GetUID()) != uid {
			return trace.NotFound("secret with UID %v not found", uid)
		}
	}
	control, err := NewSecretControl(SecretConfig{Secret: secret, Client: cs.Client})
	if err != nil {
		return trace.Wrap(err)
	}
	return control.Status(ctx)
}

func (cs *Changeset) statusConfigMap(ctx context.Context, data []byte, uid string) error {
	configMap, err := ParseConfigMap(bytes.NewReader(data))
	if err != nil {
		return trace.Wrap(err)
	}
	if uid != "" {
		existing, err := cs.Client.CoreV1().ConfigMaps(configMap.Namespace).Get(ctx, configMap.Name, metav1.GetOptions{})
		if err != nil {
			return ConvertError(err)
		}
		if string(existing.GetUID()) != uid {
			return trace.NotFound("configmap with UID %v not found", uid)
		}
	}
	control, err := NewConfigMapControl(ConfigMapConfig{ConfigMap: configMap, Client: cs.Client})
	if err != nil {
		return trace.Wrap(err)
	}
	return control.Status(ctx)
}

func (cs *Changeset) statusServiceAccount(ctx context.Context, data []byte, uid string) error {
	account, err := ParseServiceAccount(bytes.NewReader(data))
	if err != nil {
		return trace.Wrap(err)
	}
	if uid != "" {
		existing, err := cs.Client.CoreV1().ServiceAccounts(account.Namespace).Get(ctx, account.Name, metav1.GetOptions{})
		if err != nil {
			return ConvertError(err)
		}
		if string(existing.GetUID()) != uid {
			return trace.NotFound("service account with UID %v not found", uid)
		}
	}
	control, err := NewServiceAccountControl(ServiceAccountConfig{ServiceAccount: account, Client: cs.Client})
	if err != nil {
		return trace.Wrap(err)
	}
	return control.Status(ctx)
}

func (cs *Changeset) statusRole(ctx context.Context, data []byte, uid string) error {
	role, err := ParseRole(bytes.NewReader(data))
	if err != nil {
		return trace.Wrap(err)
	}
	if uid != "" {
		existing, err := cs.Client.RbacV1().Roles(role.Namespace).Get(ctx, role.Name, metav1.GetOptions{})
		if err != nil {
			return ConvertError(err)
		}
		if string(existing.GetUID()) != uid {
			return trace.NotFound("role with UID %v not found", uid)
		}
	}
	control, err := NewRoleControl(RoleConfig{Role: role, Client: cs.Client})
	if err != nil {
		return trace.Wrap(err)
	}
	return control.Status(ctx)
}

func (cs *Changeset) statusClusterRole(ctx context.Context, data []byte, uid string) error {
	role, err := ParseClusterRole(bytes.NewReader(data))
	if err != nil {
		return trace.Wrap(err)
	}
	if uid != "" {
		existing, err := cs.Client.RbacV1().ClusterRoles().Get(ctx, role.Name, metav1.GetOptions{})
		if err != nil {
			return ConvertError(err)
		}
		if string(existing.GetUID()) != uid {
			return trace.NotFound("cluster role with UID %v not found", uid)
		}
	}
	control, err := NewClusterRoleControl(ClusterRoleConfig{ClusterRole: role, Client: cs.Client})
	if err != nil {
		return trace.Wrap(err)
	}
	return control.Status(ctx)
}

func (cs *Changeset) statusNamespace(ctx context.Context, data []byte, uid string) error {
	namespace, err := ParseNamespace(bytes.NewReader(data))
	if err != nil {
		return trace.Wrap(err)
	}
	if uid != "" {
		existing, err := cs.Client.CoreV1().Namespaces().Get(ctx, namespace.Name, metav1.GetOptions{})
		if err != nil {
			return ConvertError(err)
		}
		if string(existing.GetUID()) != uid {
			return trace.NotFound("cluster role with UID %v not found", uid)
		}
	}
	control, err := NewNamespaceControl(NamespaceConfig{Namespace: namespace, Client: cs.Client})
	if err != nil {
		return trace.Wrap(err)
	}
	return control.Status(ctx)
}

func (cs *Changeset) statusPriorityClass(ctx context.Context, data []byte, uid string) error {
	pc, err := ParsePriorityClass(bytes.NewReader(data))
	if err != nil {
		return trace.Wrap(err)
	}
	if uid != "" {
		existing, err := cs.Client.SchedulingV1beta1().PriorityClasses().Get(ctx, pc.Name, metav1.GetOptions{})
		if err != nil {
			return ConvertError(err)
		}
		if string(existing.GetUID()) != uid {
			return trace.NotFound("cluster role with UID %v not found", uid)
		}
	}
	control, err := NewPriorityClassControl(PriorityClassConfig{PriorityClass: pc, Client: cs.Client})
	if err != nil {
		return trace.Wrap(err)
	}
	return control.Status(ctx)

}

func (cs *Changeset) statusRoleBinding(ctx context.Context, data []byte, uid string) error {
	binding, err := ParseRoleBinding(bytes.NewReader(data))
	if err != nil {
		return trace.Wrap(err)
	}
	if uid != "" {
		existing, err := cs.Client.RbacV1().RoleBindings(binding.Namespace).Get(ctx, binding.Name, metav1.GetOptions{})
		if err != nil {
			return ConvertError(err)
		}
		if string(existing.GetUID()) != uid {
			return trace.NotFound("role binding with UID %v not found", uid)
		}
	}
	control, err := NewRoleBindingControl(RoleBindingConfig{RoleBinding: binding, Client: cs.Client})
	if err != nil {
		return trace.Wrap(err)
	}
	return control.Status(ctx)
}

func (cs *Changeset) statusClusterRoleBinding(ctx context.Context, data []byte, uid string) error {
	binding, err := ParseClusterRoleBinding(bytes.NewReader(data))
	if err != nil {
		return trace.Wrap(err)
	}
	if uid != "" {
		existing, err := cs.Client.RbacV1().ClusterRoleBindings().Get(ctx, binding.Name, metav1.GetOptions{})
		if err != nil {
			return ConvertError(err)
		}
		if string(existing.GetUID()) != uid {
			return trace.NotFound("cluster role binding with UID %v not found", uid)
		}
	}
	control, err := NewClusterRoleBindingControl(ClusterRoleBindingConfig{ClusterRoleBinding: binding, Client: cs.Client})
	if err != nil {
		return trace.Wrap(err)
	}
	return control.Status(ctx)
}

func (cs *Changeset) statusPodSecurityPolicy(ctx context.Context, data []byte, uid string) error {
	policy, err := ParsePodSecurityPolicy(bytes.NewReader(data))
	if err != nil {
		return trace.Wrap(err)
	}
	if uid != "" {
		existing, err := cs.Client.PolicyV1beta1().PodSecurityPolicies().Get(ctx, policy.Name, metav1.GetOptions{})
		if err != nil {
			return ConvertError(err)
		}
		if string(existing.GetUID()) != uid {
			return trace.NotFound("pod security policy with UID %v not found", uid)
		}
	}
	control, err := NewPodSecurityPolicyControl(PodSecurityPolicyConfig{PodSecurityPolicy: policy, Client: cs.Client})
	if err != nil {
		return trace.Wrap(err)
	}
	return control.Status(ctx)
}

func (cs *Changeset) statusCustomResourceDefinition(ctx context.Context, data []byte, uid string) error {
	crd, err := ParseCustomResourceDefinition(bytes.NewReader(data))
	if err != nil {
		return trace.Wrap(err)
	}
	if uid != "" {
		existing, err := cs.APIExtensionsClient.ApiextensionsV1beta1().CustomResourceDefinitions().Get(ctx, crd.Name,
			metav1.GetOptions{})
		if err != nil {
			return ConvertError(err)
		}
		if string(existing.GetUID()) != uid {
			return trace.NotFound("custom resource definition with UID %v not found", uid)
		}
	}
	control, err := NewCustomResourceDefinitionControl(CustomResourceDefinitionConfig{
		CustomResourceDefinition: crd,
		Client:                   cs.APIExtensionsClient,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return control.Status(ctx)
}

func (cs *Changeset) statusValidatingWebhookConfiguration(ctx context.Context, data []byte, uid string) error {
	webhook, err := ParseValidatingWebhookConfiguration(bytes.NewReader(data))
	if err != nil {
		return trace.Wrap(err)
	}
	if uid != "" {
		existing, err := cs.Client.AdmissionregistrationV1().ValidatingWebhookConfigurations().Get(ctx, webhook.Name, metav1.GetOptions{})
		if err != nil {
			return ConvertError(err)
		}
		if string(existing.GetUID()) != uid {
			return trace.NotFound("validating webhook configuration with UID %v not found", uid)
		}
	}
	control, err := NewValidatingWebhookConfigurationControl(ValidatingWebhookConfigurationConfig{
		ValidatingWebhookConfiguration: webhook,
		Client:                         cs.Client,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return control.Status(ctx)
}

func (cs *Changeset) statusMutatingWebhookConfiguration(ctx context.Context, data []byte, uid string) error {
	webhook, err := ParseMutatingWebhookConfiguration(bytes.NewReader(data))
	if err != nil {
		return trace.Wrap(err)
	}
	if uid != "" {
		existing, err := cs.Client.AdmissionregistrationV1().MutatingWebhookConfigurations().Get(ctx, webhook.Name, metav1.GetOptions{})
		if err != nil {
			return ConvertError(err)
		}
		if string(existing.GetUID()) != uid {
			return trace.NotFound("mutating webhook configuration with UID %v not found", uid)
		}
	}
	control, err := NewMutatingWebhookConfigurationControl(MutatingWebhookConfigurationConfig{
		MutatingWebhookConfiguration: webhook,
		Client:                       cs.Client,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return control.Status(ctx)
}

func (cs *Changeset) statusAPIService(ctx context.Context, data []byte, uid string) error {
	apiService, err := ParseAPIService(bytes.NewReader(data))
	if err != nil {
		return trace.Wrap(err)
	}
	if uid != "" {
		existing, err := cs.APIRegistrationClient.ApiregistrationV1().APIServices().Get(ctx, apiService.Name, metav1.GetOptions{})
		if err != nil {
			return ConvertError(err)
		}
		if string(existing.GetUID()) != uid {
			return trace.NotFound("api service with UID %v not found", uid)
		}
	}
	control, err := NewAPIServiceControl(APIServiceConfig{
		APIService: apiService,
		Client:     cs.APIRegistrationClient,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return control.Status(ctx)
}

func (cs *Changeset) statusServiceMonitor(ctx context.Context, data []byte, uid string) error {
	serviceMonitor, err := ParseServiceMonitor(bytes.NewReader(data))
	if err != nil {
		return trace.Wrap(err)
	}
	if uid != "" {
		existing, err := cs.MonitoringClient.MonitoringV1().ServiceMonitors(
			serviceMonitor.Namespace).Get(ctx, serviceMonitor.Name, metav1.GetOptions{})
		if err != nil {
			return ConvertError(err)
		}
		if string(existing.GetUID()) != uid {
			return trace.NotFound("service monitor with UID %v not found", uid)
		}
	}
	control, err := NewServiceMonitorControl(ServiceMonitorConfig{
		ServiceMonitor: serviceMonitor,
		Client:         cs.MonitoringClient,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return control.Status(ctx)
}

func (cs *Changeset) statusAlertmanager(ctx context.Context, data []byte, uid string) error {
	alertmanager, err := ParseAlertmanager(bytes.NewReader(data))
	if err != nil {
		return trace.Wrap(err)
	}
	if uid != "" {
		existing, err := cs.MonitoringClient.MonitoringV1().Alertmanagers(
			alertmanager.Namespace).Get(ctx, alertmanager.Name, metav1.GetOptions{})
		if err != nil {
			return ConvertError(err)
		}
		if string(existing.GetUID()) != uid {
			return trace.NotFound("alert manager with UID %v not found", uid)
		}
	}
	control, err := NewAlertmanagerControl(AlertmanagerConfig{
		Alertmanager: alertmanager,
		Client:       cs.MonitoringClient,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return control.Status(ctx)
}

func (cs *Changeset) statusPrometheus(ctx context.Context, data []byte, uid string) error {
	prometheus, err := ParsePrometheus(bytes.NewReader(data))
	if err != nil {
		return trace.Wrap(err)
	}
	if uid != "" {
		existing, err := cs.MonitoringClient.MonitoringV1().Prometheuses(
			prometheus.Namespace).Get(ctx, prometheus.Name, metav1.GetOptions{})
		if err != nil {
			return ConvertError(err)
		}
		if string(existing.GetUID()) != uid {
			return trace.NotFound("prometheus with UID %v not found", uid)
		}
	}
	control, err := NewPrometheusControl(PrometheusConfig{
		Prometheus: prometheus,
		Client:     cs.MonitoringClient,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return control.Status(ctx)
}

func (cs *Changeset) statusPrometheusRule(ctx context.Context, data []byte, uid string) error {
	prometheusRule, err := ParsePrometheusRule(bytes.NewReader(data))
	if err != nil {
		return trace.Wrap(err)
	}
	if uid != "" {
		existing, err := cs.MonitoringClient.MonitoringV1().PrometheusRules(
			prometheusRule.Namespace).Get(ctx, prometheusRule.Name, metav1.GetOptions{})
		if err != nil {
			return ConvertError(err)
		}
		if string(existing.GetUID()) != uid {
			return trace.NotFound("prometheus rule with UID %v not found", uid)
		}
	}
	control, err := NewPrometheusRuleControl(PrometheusRuleConfig{
		PrometheusRule: prometheusRule,
		Client:         cs.MonitoringClient,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return control.Status(ctx)
}

func (cs *Changeset) withDeleteOp(ctx context.Context, tr *ChangesetResource, obj metav1.Object, fn func() error) error {
	data, err := goyaml.Marshal(obj)
	if err != nil {
		return trace.Wrap(err)
	}
	tr.Spec.Items = append(tr.Spec.Items, ChangesetItem{
		From:              string(data),
		UID:               string(obj.GetUID()),
		Status:            OpStatusCreated,
		CreationTimestamp: time.Now().UTC(),
	})
	tr, err = cs.update(ctx, tr)
	if err != nil {
		return trace.Wrap(err)
	}
	err = fn()
	if err != nil {
		return trace.Wrap(err)
	}
	tr.Spec.Items[len(tr.Spec.Items)-1].Status = OpStatusCompleted
	_, err = cs.update(ctx, tr)
	return err
}

func (cs *Changeset) deleteDaemonSet(ctx context.Context, tr *ChangesetResource, namespace, name string, cascade bool) error {
	daemonSet, err := cs.Client.AppsV1().DaemonSets(Namespace(namespace)).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return ConvertError(err)
	}
	control, err := NewDaemonSetControl(DSConfig{DaemonSet: daemonSet, Client: cs.Client})
	if err != nil {
		return trace.Wrap(err)
	}
	return cs.withDeleteOp(ctx, tr, control.DaemonSet, func() error {
		return control.Delete(ctx, cascade)
	})
}

func (cs *Changeset) deleteStatefulSet(ctx context.Context, tr *ChangesetResource, namespace, name string, cascade bool) error {
	statefulSet, err := cs.Client.AppsV1().StatefulSets(Namespace(namespace)).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return ConvertError(err)
	}
	control, err := NewStatefulSetControl(StatefulSetConfig{StatefulSet: statefulSet, Client: cs.Client})
	if err != nil {
		return trace.Wrap(err)
	}
	return cs.withDeleteOp(ctx, tr, control.StatefulSet, func() error {
		return control.Delete(ctx, cascade)
	})
}

func (cs *Changeset) deleteJob(ctx context.Context, tr *ChangesetResource, namespace, name string, cascade bool) error {
	job, err := cs.Client.BatchV1().Jobs(Namespace(namespace)).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return ConvertError(err)
	}
	control, err := NewJobControl(JobConfig{Job: job, Clientset: cs.Client})
	if err != nil {
		return trace.Wrap(err)
	}
	return cs.withDeleteOp(ctx, tr, control.Job, func() error {
		return control.Delete(ctx, cascade)
	})
}

func (cs *Changeset) deleteRC(ctx context.Context, tr *ChangesetResource, namespace, name string, cascade bool) error {
	rc, err := cs.Client.CoreV1().ReplicationControllers(Namespace(namespace)).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return ConvertError(err)
	}
	control, err := NewReplicationControllerControl(RCConfig{ReplicationController: rc, Client: cs.Client})
	if err != nil {
		return trace.Wrap(err)
	}
	return cs.withDeleteOp(ctx, tr, control.ReplicationController, func() error {
		return control.Delete(ctx, cascade)
	})
}

func (cs *Changeset) deleteDeployment(ctx context.Context, tr *ChangesetResource, namespace, name string, cascade bool) error {
	deployment, err := cs.Client.AppsV1().Deployments(Namespace(namespace)).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return ConvertError(err)
	}
	control, err := NewDeploymentControl(DeploymentConfig{Deployment: deployment, Client: cs.Client})
	if err != nil {
		return trace.Wrap(err)
	}
	return cs.withDeleteOp(ctx, tr, control.Deployment, func() error {
		return control.Delete(ctx, cascade)
	})
}

func (cs *Changeset) deleteService(ctx context.Context, tr *ChangesetResource, namespace, name string, cascade bool) error {
	service, err := cs.Client.CoreV1().Services(Namespace(namespace)).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return ConvertError(err)
	}
	control, err := NewServiceControl(ServiceConfig{Service: service, Client: cs.Client})
	if err != nil {
		return trace.Wrap(err)
	}
	return cs.withDeleteOp(ctx, tr, control.Service, func() error {
		return control.Delete(ctx, cascade)
	})
}

func (cs *Changeset) deleteConfigMap(ctx context.Context, tr *ChangesetResource, namespace, name string, cascade bool) error {
	configMap, err := cs.Client.CoreV1().ConfigMaps(Namespace(namespace)).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return ConvertError(err)
	}
	control, err := NewConfigMapControl(ConfigMapConfig{ConfigMap: configMap, Client: cs.Client})
	if err != nil {
		return trace.Wrap(err)
	}
	return cs.withDeleteOp(ctx, tr, control.ConfigMap, func() error {
		return control.Delete(ctx, cascade)
	})
}

func (cs *Changeset) deleteSecret(ctx context.Context, tr *ChangesetResource, namespace, name string, cascade bool) error {
	secret, err := cs.Client.CoreV1().Secrets(Namespace(namespace)).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return ConvertError(err)
	}
	control, err := NewSecretControl(SecretConfig{Secret: secret, Client: cs.Client})
	if err != nil {
		return trace.Wrap(err)
	}
	return cs.withDeleteOp(ctx, tr, control.Secret, func() error {
		return control.Delete(ctx, cascade)
	})
}

func (cs *Changeset) deleteServiceAccount(ctx context.Context, tr *ChangesetResource, namespace, name string, cascade bool) error {
	account, err := cs.Client.CoreV1().ServiceAccounts(Namespace(namespace)).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return ConvertError(err)
	}
	control, err := NewServiceAccountControl(ServiceAccountConfig{ServiceAccount: account, Client: cs.Client})
	if err != nil {
		return trace.Wrap(err)
	}
	return cs.withDeleteOp(ctx, tr, control.ServiceAccount, func() error {
		return control.Delete(ctx, cascade)
	})
}

func (cs *Changeset) deleteRole(ctx context.Context, tr *ChangesetResource, namespace, name string, cascade bool) error {
	role, err := cs.Client.RbacV1().Roles(Namespace(namespace)).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return ConvertError(err)
	}
	control, err := NewRoleControl(RoleConfig{Role: role, Client: cs.Client})
	if err != nil {
		return trace.Wrap(err)
	}
	return cs.withDeleteOp(ctx, tr, control.Role, func() error {
		return control.Delete(ctx, cascade)
	})
}

func (cs *Changeset) deleteClusterRole(ctx context.Context, tr *ChangesetResource, name string, cascade bool) error {
	role, err := cs.Client.RbacV1().ClusterRoles().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return ConvertError(err)
	}
	control, err := NewClusterRoleControl(ClusterRoleConfig{ClusterRole: role, Client: cs.Client})
	if err != nil {
		return trace.Wrap(err)
	}
	return cs.withDeleteOp(ctx, tr, control.ClusterRole, func() error {
		return control.Delete(ctx, cascade)
	})
}

func (cs *Changeset) deleteNamespace(ctx context.Context, tr *ChangesetResource, name string, cascade bool) error {
	namespace, err := cs.Client.CoreV1().Namespaces().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return ConvertError(err)
	}
	control, err := NewNamespaceControl(NamespaceConfig{Namespace: namespace, Client: cs.Client})
	if err != nil {
		return trace.Wrap(err)
	}
	return cs.withDeleteOp(ctx, tr, control.Namespace, func() error {
		return control.Delete(ctx, cascade)
	})
}

func (cs *Changeset) deletePriorityClass(ctx context.Context, tr *ChangesetResource, name string, cascade bool) error {
	pc, err := cs.Client.SchedulingV1beta1().PriorityClasses().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return ConvertError(err)
	}
	control, err := NewPriorityClassControl(PriorityClassConfig{PriorityClass: pc, Client: cs.Client})
	if err != nil {
		return trace.Wrap(err)
	}
	return cs.withDeleteOp(ctx, tr, control.PriorityClass, func() error {
		return control.Delete(ctx, cascade)
	})
}

func (cs *Changeset) deleteValidatingWebhookConfiguration(ctx context.Context, tr *ChangesetResource, name string, cascade bool) error {
	webhook, err := cs.Client.AdmissionregistrationV1().ValidatingWebhookConfigurations().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return ConvertError(err)
	}
	control, err := NewValidatingWebhookConfigurationControl(ValidatingWebhookConfigurationConfig{
		ValidatingWebhookConfiguration: webhook,
		Client:                         cs.Client,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return cs.withDeleteOp(ctx, tr, control.ValidatingWebhookConfiguration, func() error {
		return control.Delete(ctx, cascade)
	})
}

func (cs *Changeset) deleteMutatingWebhookConfiguration(ctx context.Context, tr *ChangesetResource, name string, cascade bool) error {
	webhook, err := cs.Client.AdmissionregistrationV1().MutatingWebhookConfigurations().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return ConvertError(err)
	}
	control, err := NewMutatingWebhookConfigurationControl(MutatingWebhookConfigurationConfig{
		MutatingWebhookConfiguration: webhook,
		Client:                       cs.Client,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return cs.withDeleteOp(ctx, tr, control.MutatingWebhookConfiguration, func() error {
		return control.Delete(ctx, cascade)
	})
}

func (cs *Changeset) deleteAPIService(ctx context.Context, tr *ChangesetResource, name string, cascade bool) error {
	apiService, err := cs.APIRegistrationClient.ApiregistrationV1().APIServices().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return ConvertError(err)
	}
	control, err := NewAPIServiceControl(APIServiceConfig{APIService: apiService, Client: cs.APIRegistrationClient})
	if err != nil {
		return trace.Wrap(err)
	}
	return cs.withDeleteOp(ctx, tr, control.APIService, func() error {
		return control.Delete(ctx, cascade)
	})
}

func (cs *Changeset) deleteServiceMonitor(ctx context.Context, tr *ChangesetResource, namespace, name string, cascade bool) error {
	monitor, err := cs.MonitoringClient.MonitoringV1().ServiceMonitors(Namespace(namespace)).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return ConvertError(err)
	}
	control, err := NewServiceMonitorControl(ServiceMonitorConfig{ServiceMonitor: monitor, Client: cs.MonitoringClient})
	if err != nil {
		return trace.Wrap(err)
	}
	return cs.withDeleteOp(ctx, tr, control.ServiceMonitor, func() error {
		return control.Delete(ctx, cascade)
	})
}

func (cs *Changeset) deleteAlertmanager(ctx context.Context, tr *ChangesetResource, namespace, name string, cascade bool) error {
	alertmanager, err := cs.MonitoringClient.MonitoringV1().Alertmanagers(Namespace(namespace)).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return ConvertError(err)
	}
	control, err := NewAlertmanagerControl(AlertmanagerConfig{Alertmanager: alertmanager, Client: cs.MonitoringClient})
	if err != nil {
		return trace.Wrap(err)
	}
	return cs.withDeleteOp(ctx, tr, control.Alertmanager, func() error {
		return control.Delete(ctx, cascade)
	})
}

func (cs *Changeset) deletePrometheus(ctx context.Context, tr *ChangesetResource, namespace, name string, cascade bool) error {
	prometheus, err := cs.MonitoringClient.MonitoringV1().Prometheuses(Namespace(namespace)).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return ConvertError(err)
	}
	control, err := NewPrometheusControl(PrometheusConfig{Prometheus: prometheus, Client: cs.MonitoringClient})
	if err != nil {
		return trace.Wrap(err)
	}
	return cs.withDeleteOp(ctx, tr, control.Prometheus, func() error {
		return control.Delete(ctx, cascade)
	})
}

func (cs *Changeset) deletePrometheusRule(ctx context.Context, tr *ChangesetResource, namespace, name string, cascade bool) error {
	prometheusRule, err := cs.MonitoringClient.MonitoringV1().PrometheusRules(Namespace(namespace)).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return ConvertError(err)
	}
	control, err := NewPrometheusRuleControl(PrometheusRuleConfig{PrometheusRule: prometheusRule, Client: cs.MonitoringClient})
	if err != nil {
		return trace.Wrap(err)
	}
	return cs.withDeleteOp(ctx, tr, control.PrometheusRule, func() error {
		return control.Delete(ctx, cascade)
	})
}

func (cs *Changeset) deleteRoleBinding(ctx context.Context, tr *ChangesetResource, namespace, name string, cascade bool) error {
	binding, err := cs.Client.RbacV1().RoleBindings(Namespace(namespace)).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return ConvertError(err)
	}
	control, err := NewRoleBindingControl(RoleBindingConfig{RoleBinding: binding, Client: cs.Client})
	if err != nil {
		return trace.Wrap(err)
	}
	return cs.withDeleteOp(ctx, tr, control.RoleBinding, func() error {
		return control.Delete(ctx, cascade)
	})
}

func (cs *Changeset) deleteClusterRoleBinding(ctx context.Context, tr *ChangesetResource, name string, cascade bool) error {
	binding, err := cs.Client.RbacV1().ClusterRoleBindings().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return ConvertError(err)
	}
	control, err := NewClusterRoleBindingControl(ClusterRoleBindingConfig{ClusterRoleBinding: binding, Client: cs.Client})
	if err != nil {
		return trace.Wrap(err)
	}
	return cs.withDeleteOp(ctx, tr, control.ClusterRoleBinding, func() error {
		return control.Delete(ctx, cascade)
	})
}

func (cs *Changeset) deletePodSecurityPolicy(ctx context.Context, tr *ChangesetResource, name string, cascade bool) error {
	policy, err := cs.Client.PolicyV1beta1().PodSecurityPolicies().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return ConvertError(err)
	}
	control, err := NewPodSecurityPolicyControl(PodSecurityPolicyConfig{PodSecurityPolicy: policy, Client: cs.Client})
	if err != nil {
		return trace.Wrap(err)
	}
	return cs.withDeleteOp(ctx, tr, control.PodSecurityPolicy, func() error {
		return control.Delete(ctx, cascade)
	})
}

func (cs *Changeset) deleteCustomResourceDefinition(
	ctx context.Context, tr *ChangesetResource, name string, cascade bool) error {
	crd, err := cs.APIExtensionsClient.ApiextensionsV1beta1().CustomResourceDefinitions().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return ConvertError(err)
	}
	control, err := NewCustomResourceDefinitionControl(CustomResourceDefinitionConfig{
		CustomResourceDefinition: crd,
		Client:                   cs.APIExtensionsClient,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return cs.withDeleteOp(ctx, tr, control.CustomResourceDefinition, func() error {
		return control.Delete(ctx, cascade)
	})
}

func (cs *Changeset) revert(ctx context.Context, item *ChangesetItem, info *OperationInfo) error {
	kind := info.Kind()
	switch info.Kind() {
	case KindDaemonSet:
		return cs.revertDaemonSet(ctx, item)
	case KindStatefulSet:
		return cs.revertStatefulSet(ctx, item)
	case KindJob:
		return cs.revertJob(ctx, item)
	case KindReplicationController:
		return cs.revertReplicationController(ctx, item)
	case KindDeployment:
		return cs.revertDeployment(ctx, item)
	case KindService:
		return cs.revertService(ctx, item)
	case KindServiceAccount:
		return cs.revertServiceAccount(ctx, item)
	case KindSecret:
		return cs.revertSecret(ctx, item)
	case KindConfigMap:
		return cs.revertConfigMap(ctx, item)
	case KindRole:
		return cs.revertRole(ctx, item)
	case KindClusterRole:
		return cs.revertClusterRole(ctx, item)
	case KindRoleBinding:
		return cs.revertRoleBinding(ctx, item)
	case KindClusterRoleBinding:
		return cs.revertClusterRoleBinding(ctx, item)
	case KindPodSecurityPolicy:
		return cs.revertPodSecurityPolicy(ctx, item)
	case KindCustomResourceDefinition:
		return cs.revertCustomResourceDefinition(ctx, item)
	case KindNamespace:
		return cs.revertNamespace(ctx, item)
	case KindPriorityClass:
		return cs.revertPriorityClass(ctx, item)
	case KindValidatingWebhookConfiguration:
		return cs.revertValidatingWebhookConfiguration(ctx, item)
	case KindMutatingWebhookConfiguration:
		return cs.revertMutatingWebhookConfiguration(ctx, item)
	case KindAPIService:
		return cs.revertAPIService(ctx, item)
	case KindServiceMonitor:
		return cs.revertServiceMonitor(ctx, item)
	case KindAlertmanager:
		return cs.revertAlertmanager(ctx, item)
	case KindPrometheus:
		return cs.revertPrometheus(ctx, item)
	case KindPrometheusRule:
		return cs.revertPrometheusRule(ctx, item)
	}
	return trace.BadParameter("unsupported resource type %v", kind)
}

func (cs *Changeset) revertDaemonSet(ctx context.Context, item *ChangesetItem) error {
	resource := item.From
	if len(resource) == 0 {
		resource = item.To
	}
	daemonSet, err := ParseDaemonSet(strings.NewReader(resource))
	if err != nil {
		return trace.Wrap(err)
	}
	control, err := NewDaemonSetControl(DSConfig{DaemonSet: daemonSet, Client: cs.Client})
	if err != nil {
		return trace.Wrap(err)
	}
	// this operation created daemon set, so we will delete it
	if len(item.From) == 0 {
		err = control.Delete(ctx, true)
		// If the resource has already been deleted, suppress the error
		if trace.IsNotFound(err) {
			return nil
		}
		return trace.Wrap(err)
	}
	// this operation either created or updated daemon set, so we create a new version
	return control.Upsert(ctx)
}

func (cs *Changeset) revertStatefulSet(ctx context.Context, item *ChangesetItem) error {
	resource := item.From
	if len(resource) == 0 {
		resource = item.To
	}
	statefulSet, err := ParseStatefulSet(strings.NewReader(resource))
	if err != nil {
		return trace.Wrap(err)
	}
	control, err := NewStatefulSetControl(StatefulSetConfig{StatefulSet: statefulSet, Client: cs.Client})
	if err != nil {
		return trace.Wrap(err)
	}
	// this operation created statefulset, so we will delete it
	if len(item.From) == 0 {
		err = control.Delete(ctx, true)
		// If the resource has already been deleted, suppress the error
		if trace.IsNotFound(err) {
			return nil
		}
		return trace.Wrap(err)
	}
	// this operation either created or updated statefulset, so we create a new version
	return control.Upsert(ctx)
}

func (cs *Changeset) revertJob(ctx context.Context, item *ChangesetItem) error {
	resource := item.From
	if len(resource) == 0 {
		resource = item.To
	}
	job, err := ParseJob(strings.NewReader(resource))
	if err != nil {
		return trace.Wrap(err)
	}
	control, err := NewJobControl(JobConfig{Job: job, Clientset: cs.Client})
	if err != nil {
		return trace.Wrap(err)
	}
	if len(item.From) == 0 {
		// this operation created the job, so we will delete it
		err = control.Delete(ctx, true)
		// If the resource has already been deleted, suppress the error
		if trace.IsNotFound(err) {
			return nil
		}
		return trace.Wrap(err)
	}
	// this operation either created or updated the job, so we create a new version
	return control.Upsert(ctx)
}

func (cs *Changeset) revertReplicationController(ctx context.Context, item *ChangesetItem) error {
	resource := item.From
	if len(resource) == 0 {
		resource = item.To
	}
	rc, err := ParseReplicationController(strings.NewReader(resource))
	if err != nil {
		return trace.Wrap(err)
	}
	control, err := NewReplicationControllerControl(RCConfig{ReplicationController: rc, Client: cs.Client})
	if err != nil {
		return trace.Wrap(err)
	}
	// this operation created RC, so we will delete it
	if len(item.From) == 0 {
		err = control.Delete(ctx, true)
		// If the resource has already been deleted, suppress the error
		if trace.IsNotFound(err) {
			return nil
		}
		return trace.Wrap(err)
	}
	// this operation either created or updated RC, so we create a new version
	return control.Upsert(ctx)
}

func (cs *Changeset) revertDeployment(ctx context.Context, item *ChangesetItem) error {
	resource := item.From
	if len(resource) == 0 {
		resource = item.To
	}
	deployment, err := ParseDeployment(strings.NewReader(resource))
	if err != nil {
		return trace.Wrap(err)
	}
	control, err := NewDeploymentControl(DeploymentConfig{Deployment: deployment, Client: cs.Client})
	if err != nil {
		return trace.Wrap(err)
	}
	// this operation created Deployment, so we will delete it
	if len(item.From) == 0 {
		err = control.Delete(ctx, true)
		// If the resource has already been deleted, suppress the error
		if trace.IsNotFound(err) {
			return nil
		}
		return trace.Wrap(err)
	}
	// this operation either created or updated Deployment, so we create a new version
	return control.Upsert(ctx)
}

func (cs *Changeset) revertService(ctx context.Context, item *ChangesetItem) error {
	resource := item.From
	if len(resource) == 0 {
		resource = item.To
	}
	service, err := ParseService(strings.NewReader(resource))
	if err != nil {
		return trace.Wrap(err)
	}
	control, err := NewServiceControl(ServiceConfig{Service: service, Client: cs.Client})
	if err != nil {
		return trace.Wrap(err)
	}
	// this operation created Service, so we will delete it
	if len(item.From) == 0 {
		err = control.Delete(ctx, true)
		// If the resource has already been deleted, suppress the error
		if trace.IsNotFound(err) {
			return nil
		}
		return trace.Wrap(err)
	}
	// this operation either created or updated Service, so we create a new version
	return control.Upsert(ctx)
}

func (cs *Changeset) revertConfigMap(ctx context.Context, item *ChangesetItem) error {
	resource := item.From
	if len(resource) == 0 {
		resource = item.To
	}
	configMap, err := ParseConfigMap(strings.NewReader(resource))
	if err != nil {
		return trace.Wrap(err)
	}
	control, err := NewConfigMapControl(ConfigMapConfig{ConfigMap: configMap, Client: cs.Client})
	if err != nil {
		return trace.Wrap(err)
	}
	// this operation created ConfigMap, so we will delete it
	if len(item.From) == 0 {
		err = control.Delete(ctx, true)
		// If the resource has already been deleted, suppress the error
		if trace.IsNotFound(err) {
			return nil
		}
		return trace.Wrap(err)
	}
	// this operation either created or updated ConfigMap, so we create a new version
	return control.Upsert(ctx)
}

func (cs *Changeset) revertSecret(ctx context.Context, item *ChangesetItem) error {
	resource := item.From
	if len(resource) == 0 {
		resource = item.To
	}
	secret, err := ParseSecret(strings.NewReader(resource))
	if err != nil {
		return trace.Wrap(err)
	}
	control, err := NewSecretControl(SecretConfig{Secret: secret, Client: cs.Client})
	if err != nil {
		return trace.Wrap(err)
	}
	// this operation created Secret, so we will delete it
	if len(item.From) == 0 {
		err = control.Delete(ctx, true)
		// If the resource has already been deleted, suppress the error
		if trace.IsNotFound(err) {
			return nil
		}
		return trace.Wrap(err)
	}
	// this operation either created or updated Secret, so we create a new version
	return control.Upsert(ctx)
}

func (cs *Changeset) revertServiceAccount(ctx context.Context, item *ChangesetItem) error {
	resource := item.From
	if len(resource) == 0 {
		resource = item.To
	}
	account, err := ParseServiceAccount(strings.NewReader(resource))
	if err != nil {
		return trace.Wrap(err)
	}
	control, err := NewServiceAccountControl(ServiceAccountConfig{ServiceAccount: account, Client: cs.Client})
	if err != nil {
		return trace.Wrap(err)
	}
	// this operation created the resource, so we will delete it
	if len(item.From) == 0 {
		err = control.Delete(ctx, true)
		// If the resource has already been deleted, suppress the error
		if trace.IsNotFound(err) {
			return nil
		}
		return trace.Wrap(err)
	}
	// this operation either created or updated the resource, so we create a new version
	return control.Upsert(ctx)
}

func (cs *Changeset) revertRole(ctx context.Context, item *ChangesetItem) error {
	resource := item.From
	if len(resource) == 0 {
		resource = item.To
	}
	role, err := ParseRole(strings.NewReader(resource))
	if err != nil {
		return trace.Wrap(err)
	}
	control, err := NewRoleControl(RoleConfig{Role: role, Client: cs.Client})
	if err != nil {
		return trace.Wrap(err)
	}
	// this operation created the resource, so we will delete it
	if len(item.From) == 0 {
		err = control.Delete(ctx, true)
		// If the resource has already been deleted, suppress the error
		if trace.IsNotFound(err) {
			return nil
		}
		return trace.Wrap(err)
	}
	// this operation either created or updated the resource, so we create a new version
	return control.Upsert(ctx)
}

func (cs *Changeset) revertClusterRole(ctx context.Context, item *ChangesetItem) error {
	resource := item.From
	if len(resource) == 0 {
		resource = item.To
	}
	role, err := ParseClusterRole(strings.NewReader(resource))
	if err != nil {
		return trace.Wrap(err)
	}
	control, err := NewClusterRoleControl(ClusterRoleConfig{ClusterRole: role, Client: cs.Client})
	if err != nil {
		return trace.Wrap(err)
	}
	// this operation created the resource, so we will delete it
	if len(item.From) == 0 {
		err = control.Delete(ctx, true)
		// If the resource has already been deleted, suppress the error
		if trace.IsNotFound(err) {
			return nil
		}
		return trace.Wrap(err)
	}
	// this operation either created or updated the resource, so we create a new version
	return control.Upsert(ctx)
}

func (cs *Changeset) revertNamespace(ctx context.Context, item *ChangesetItem) error {
	resource := item.From
	if len(resource) == 0 {
		resource = item.To
	}
	namespace, err := ParseNamespace(strings.NewReader(resource))
	if err != nil {
		return trace.Wrap(err)
	}
	control, err := NewNamespaceControl(NamespaceConfig{Namespace: namespace, Client: cs.Client})
	if err != nil {
		return trace.Wrap(err)
	}
	// this operation created the resource, so we will delete it
	if len(item.From) == 0 {
		err = control.Delete(ctx, true)
		// If the resource has already been deleted, suppress the error
		if trace.IsNotFound(err) {
			return nil
		}
		return trace.Wrap(err)
	}
	// this operation either created or updated the resource, so we create a new version
	return control.Upsert(ctx)
}

func (cs *Changeset) revertPriorityClass(ctx context.Context, item *ChangesetItem) error {
	resource := item.From
	if len(resource) == 0 {
		resource = item.To
	}
	pc, err := ParsePriorityClass(strings.NewReader(resource))
	if err != nil {
		return trace.Wrap(err)
	}
	control, err := NewPriorityClassControl(PriorityClassConfig{PriorityClass: pc, Client: cs.Client})
	if err != nil {
		return trace.Wrap(err)
	}
	// this operation created the resource, so we will delete it
	if len(item.From) == 0 {
		err = control.Delete(ctx, true)
		// If the resource has already been deleted, suppress the error
		if trace.IsNotFound(err) {
			return nil
		}
		return trace.Wrap(err)
	}
	// this operation either created or updated the resource, so we create a new version
	return control.Upsert(ctx)
}

func (cs *Changeset) revertValidatingWebhookConfiguration(ctx context.Context, item *ChangesetItem) error {
	resource := item.From
	if len(resource) == 0 {
		resource = item.To
	}
	webhook, err := ParseValidatingWebhookConfiguration(strings.NewReader(resource))
	if err != nil {
		return trace.Wrap(err)
	}
	control, err := NewValidatingWebhookConfigurationControl(ValidatingWebhookConfigurationConfig{
		ValidatingWebhookConfiguration: webhook,
		Client:                         cs.Client,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	if len(item.From) == 0 {
		err = control.Delete(ctx, true)
		if trace.IsNotFound(err) {
			return nil
		}
		return trace.Wrap(err)
	}
	return control.Upsert(ctx)
}

func (cs *Changeset) revertMutatingWebhookConfiguration(ctx context.Context, item *ChangesetItem) error {
	resource := item.From
	if len(resource) == 0 {
		resource = item.To
	}
	webhook, err := ParseMutatingWebhookConfiguration(strings.NewReader(resource))
	if err != nil {
		return trace.Wrap(err)
	}
	control, err := NewMutatingWebhookConfigurationControl(MutatingWebhookConfigurationConfig{
		MutatingWebhookConfiguration: webhook,
		Client:                       cs.Client,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	if len(item.From) == 0 {
		err = control.Delete(ctx, true)
		if trace.IsNotFound(err) {
			return nil
		}
		return trace.Wrap(err)
	}
	return control.Upsert(ctx)
}

func (cs *Changeset) revertAPIService(ctx context.Context, item *ChangesetItem) error {
	resource := item.From
	if len(resource) == 0 {
		resource = item.To
	}
	apiService, err := ParseAPIService(strings.NewReader(resource))
	if err != nil {
		return trace.Wrap(err)
	}
	control, err := NewAPIServiceControl(APIServiceConfig{APIService: apiService, Client: cs.APIRegistrationClient})
	if err != nil {
		return trace.Wrap(err)
	}
	if len(item.From) == 0 {
		err = control.Delete(ctx, true)
		if trace.IsNotFound(err) {
			return nil
		}
		return trace.Wrap(err)
	}
	return control.Upsert(ctx)
}

func (cs *Changeset) revertServiceMonitor(ctx context.Context, item *ChangesetItem) error {
	resource := item.From
	if len(resource) == 0 {
		resource = item.To
	}
	monitor, err := ParseServiceMonitor(strings.NewReader(resource))
	if err != nil {
		return trace.Wrap(err)
	}
	control, err := NewServiceMonitorControl(ServiceMonitorConfig{ServiceMonitor: monitor, Client: cs.MonitoringClient})
	if err != nil {
		return trace.Wrap(err)
	}
	if len(item.From) == 0 {
		err = control.Delete(ctx, true)
		if trace.IsNotFound(err) {
			return nil
		}
		return trace.Wrap(err)
	}
	return control.Upsert(ctx)
}

func (cs *Changeset) revertAlertmanager(ctx context.Context, item *ChangesetItem) error {
	resource := item.From
	if len(resource) == 0 {
		resource = item.To
	}
	alertmanager, err := ParseAlertmanager(strings.NewReader(resource))
	if err != nil {
		return trace.Wrap(err)
	}
	control, err := NewAlertmanagerControl(AlertmanagerConfig{Alertmanager: alertmanager, Client: cs.MonitoringClient})
	if err != nil {
		return trace.Wrap(err)
	}
	if len(item.From) == 0 {
		err = control.Delete(ctx, true)
		if trace.IsNotFound(err) {
			return nil
		}
		return trace.Wrap(err)
	}
	return control.Upsert(ctx)
}

func (cs *Changeset) revertPrometheus(ctx context.Context, item *ChangesetItem) error {
	resource := item.From
	if len(resource) == 0 {
		resource = item.To
	}
	prometheus, err := ParsePrometheus(strings.NewReader(resource))
	if err != nil {
		return trace.Wrap(err)
	}
	control, err := NewPrometheusControl(PrometheusConfig{Prometheus: prometheus, Client: cs.MonitoringClient})
	if err != nil {
		return trace.Wrap(err)
	}
	if len(item.From) == 0 {
		err = control.Delete(ctx, true)
		if trace.IsNotFound(err) {
			return nil
		}
		return trace.Wrap(err)
	}
	return control.Upsert(ctx)
}

func (cs *Changeset) revertPrometheusRule(ctx context.Context, item *ChangesetItem) error {
	resource := item.From
	if len(resource) == 0 {
		resource = item.To
	}
	prometheusRule, err := ParsePrometheusRule(strings.NewReader(resource))
	if err != nil {
		return trace.Wrap(err)
	}
	control, err := NewPrometheusRuleControl(PrometheusRuleConfig{PrometheusRule: prometheusRule, Client: cs.MonitoringClient})
	if err != nil {
		return trace.Wrap(err)
	}
	if len(item.From) == 0 {
		err = control.Delete(ctx, true)
		if trace.IsNotFound(err) {
			return nil
		}
		return trace.Wrap(err)
	}
	return control.Upsert(ctx)
}

func (cs *Changeset) revertRoleBinding(ctx context.Context, item *ChangesetItem) error {
	resource := item.From
	if len(resource) == 0 {
		resource = item.To
	}
	binding, err := ParseRoleBinding(strings.NewReader(resource))
	if err != nil {
		return trace.Wrap(err)
	}
	control, err := NewRoleBindingControl(RoleBindingConfig{RoleBinding: binding, Client: cs.Client})
	if err != nil {
		return trace.Wrap(err)
	}
	// this operation created the resource, so we will delete it
	if len(item.From) == 0 {
		err = control.Delete(ctx, true)
		// If the resource has already been deleted, suppress the error
		if trace.IsNotFound(err) {
			return nil
		}
		return trace.Wrap(err)
	}
	// this operation either created or updated the resource, so we create a new version
	return control.Upsert(ctx)
}

func (cs *Changeset) revertClusterRoleBinding(ctx context.Context, item *ChangesetItem) error {
	resource := item.From
	if len(resource) == 0 {
		resource = item.To
	}
	binding, err := ParseClusterRoleBinding(strings.NewReader(resource))
	if err != nil {
		return trace.Wrap(err)
	}
	control, err := NewClusterRoleBindingControl(ClusterRoleBindingConfig{ClusterRoleBinding: binding, Client: cs.Client})
	if err != nil {
		return trace.Wrap(err)
	}
	// this operation created the resource, so we will delete it
	if len(item.From) == 0 {
		err = control.Delete(ctx, true)
		// If the resource has already been deleted, suppress the error
		if trace.IsNotFound(err) {
			return nil
		}
		return trace.Wrap(err)
	}
	// this operation either created or updated the resource, so we create a new version
	return control.Upsert(ctx)
}

func (cs *Changeset) revertPodSecurityPolicy(ctx context.Context, item *ChangesetItem) error {
	resource := item.From
	if len(resource) == 0 {
		resource = item.To
	}
	policy, err := ParsePodSecurityPolicy(strings.NewReader(resource))
	if err != nil {
		return trace.Wrap(err)
	}
	control, err := NewPodSecurityPolicyControl(PodSecurityPolicyConfig{PodSecurityPolicy: policy, Client: cs.Client})
	if err != nil {
		return trace.Wrap(err)
	}
	// this operation created the resource, so we will delete it
	if len(item.From) == 0 {
		err = control.Delete(ctx, true)
		// If the resource has already been deleted, suppress the error
		if trace.IsNotFound(err) {
			return nil
		}
		return trace.Wrap(err)
	}
	// this operation either created or updated the resource, so we create a new version
	return control.Upsert(ctx)
}

func (cs *Changeset) revertCustomResourceDefinition(ctx context.Context, item *ChangesetItem) error {
	resource := item.From
	if len(resource) == 0 {
		resource = item.To
	}
	crd, err := ParseCustomResourceDefinition(strings.NewReader(resource))
	if err != nil {
		return trace.Wrap(err)
	}
	control, err := NewCustomResourceDefinitionControl(CustomResourceDefinitionConfig{
		CustomResourceDefinition: crd,
		Client:                   cs.APIExtensionsClient,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	// this operation created the resource, so we will delete it
	if len(item.From) == 0 {
		err = control.Delete(ctx, true)
		// If the resource has already been deleted, suppress the error
		if trace.IsNotFound(err) {
			return nil
		}
		return trace.Wrap(err)
	}
	// this operation either created or updated the resource, so we create a new version
	return control.Upsert(ctx)
}

func (cs *Changeset) withUpsertOp(ctx context.Context, tr *ChangesetResource, old, new metav1.Object, fn func() error) (*ChangesetResource, error) {
	to, err := goyaml.Marshal(new)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	item := ChangesetItem{
		CreationTimestamp: time.Now().UTC(),
		To:                string(to),
		Status:            OpStatusCreated,
	}
	if !reflect.ValueOf(old).IsNil() {
		from, err := goyaml.Marshal(old)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		item.From = string(from)
		item.UID = string(old.GetUID())
	}
	tr.Spec.Items = append(tr.Spec.Items, item)
	tr, err = cs.update(ctx, tr)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := fn(); err != nil {
		return nil, trace.Wrap(err)
	}
	tr.Spec.Items[len(tr.Spec.Items)-1].Status = OpStatusCompleted
	return cs.update(ctx, tr)
}

func (cs *Changeset) upsertJob(ctx context.Context, tr *ChangesetResource, data []byte) (*ChangesetResource, error) {
	job, err := ParseJob(bytes.NewReader(data))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	log := log.WithFields(log.Fields{
		"cs":  tr.String(),
		"job": fmt.Sprintf("%v/%v", job.Namespace, job.Name),
	})
	log.Infof("upsert job %v", formatMeta(job.ObjectMeta))

	jobs := cs.Client.BatchV1().Jobs(job.Namespace)
	current, err := jobs.Get(ctx, job.Name, metav1.GetOptions{})
	err = ConvertError(err)
	if err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		log.Debug("existing job not found")
		current = nil
	}
	control, err := NewJobControl(JobConfig{Job: job, Clientset: cs.Client})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if current != nil {
		updateTypeMetaJob(current)
	}
	return cs.withUpsertOp(ctx, tr, current, control.Job, func() error {
		return control.Upsert(ctx)
	})
}

func (cs *Changeset) upsertDaemonSet(ctx context.Context, tr *ChangesetResource, data []byte) (*ChangesetResource, error) {
	daemonSet, err := ParseDaemonSet(bytes.NewReader(data))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	log := log.WithFields(log.Fields{
		"cs": tr.String(),
		"ds": fmt.Sprintf("%v/%v", daemonSet.Namespace, daemonSet.Name),
	})
	log.Infof("upsert daemon set %v", formatMeta(daemonSet.ObjectMeta))
	daemons := cs.Client.AppsV1().DaemonSets(daemonSet.Namespace)
	current, err := daemons.Get(ctx, daemonSet.Name, metav1.GetOptions{})
	err = ConvertError(err)
	if err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		log.Debug("existing daemonset not found")
		current = nil
	}
	control, err := NewDaemonSetControl(DSConfig{DaemonSet: daemonSet, Client: cs.Client})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if current != nil {
		updateTypeMetaDaemonset(current)
	}
	return cs.withUpsertOp(ctx, tr, current, control.DaemonSet, func() error {
		return control.Upsert(ctx)
	})
}

func (cs *Changeset) upsertStatefulSet(ctx context.Context, tr *ChangesetResource, data []byte) (*ChangesetResource, error) {
	statefulSet, err := ParseStatefulSet(bytes.NewReader(data))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	log := log.WithFields(log.Fields{
		"cs":          tr.String(),
		"statefulset": fmt.Sprintf("%v/%v", statefulSet.Namespace, statefulSet.Name),
	})
	log.Infof("upsert statefulset %v", formatMeta(statefulSet.ObjectMeta))
	statefulsets := cs.Client.AppsV1().StatefulSets(statefulSet.Namespace)
	current, err := statefulsets.Get(ctx, statefulSet.Name, metav1.GetOptions{})
	err = ConvertError(err)
	if err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		log.Debug("existing statefulset not found")
		current = nil
	}
	control, err := NewStatefulSetControl(StatefulSetConfig{StatefulSet: statefulSet, Client: cs.Client})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if current != nil {
		updateTypeMetaStatefulSet(current)
	}
	return cs.withUpsertOp(ctx, tr, current, control.StatefulSet, func() error {
		return control.Upsert(ctx)
	})
}

func (cs *Changeset) upsertRC(ctx context.Context, tr *ChangesetResource, data []byte) (*ChangesetResource, error) {
	rc, err := ParseReplicationController(bytes.NewReader(data))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	log := log.WithFields(log.Fields{
		"cs": tr.String(),
		"rc": fmt.Sprintf("%v/%v", rc.Namespace, rc.Name),
	})
	log.Infof("upsert replication controller %v", formatMeta(rc.ObjectMeta))
	rcs := cs.Client.CoreV1().ReplicationControllers(rc.Namespace)
	current, err := rcs.Get(ctx, rc.Name, metav1.GetOptions{})
	err = ConvertError(err)
	if err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		log.Debug("existing replication controller not found")
		current = nil
	}
	control, err := NewReplicationControllerControl(RCConfig{ReplicationController: rc, Client: cs.Client})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if current != nil {
		updateTypeMetaReplicationController(current)
	}
	return cs.withUpsertOp(ctx, tr, current, control.ReplicationController, func() error {
		return control.Upsert(ctx)
	})
}

func (cs *Changeset) upsertDeployment(ctx context.Context, tr *ChangesetResource, data []byte) (*ChangesetResource, error) {
	deployment, err := ParseDeployment(bytes.NewReader(data))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	log := log.WithFields(log.Fields{
		"cs":         tr.String(),
		"deployment": fmt.Sprintf("%v/%v", deployment.Namespace, deployment.Name),
	})
	log.Infof("upsert deployment %v", formatMeta(deployment.ObjectMeta))
	deployments := cs.Client.AppsV1().Deployments(deployment.Namespace)
	current, err := deployments.Get(ctx, deployment.Name, metav1.GetOptions{})
	err = ConvertError(err)
	if err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		log.Debug("existing deployment not found")
		current = nil
	}
	control, err := NewDeploymentControl(DeploymentConfig{Deployment: deployment, Client: cs.Client})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if current != nil {
		updateTypeMetaDeployment(current)
	}
	return cs.withUpsertOp(ctx, tr, current, control.Deployment, func() error {
		return control.Upsert(ctx)
	})
}

func (cs *Changeset) upsertService(ctx context.Context, tr *ChangesetResource, data []byte) (*ChangesetResource, error) {
	service, err := ParseService(bytes.NewReader(data))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	log := log.WithFields(log.Fields{
		"cs":      tr.String(),
		"service": fmt.Sprintf("%v/%v", service.Namespace, service.Name),
	})
	log.Infof("upsert service %v", formatMeta(service.ObjectMeta))
	services := cs.Client.CoreV1().Services(service.Namespace)
	current, err := services.Get(ctx, service.Name, metav1.GetOptions{})
	err = ConvertError(err)
	if err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		log.Debug("existing service not found")
		current = nil
	}
	control, err := NewServiceControl(ServiceConfig{Service: service, Client: cs.Client})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if current != nil {
		updateTypeMetaService(current)
	}
	return cs.withUpsertOp(ctx, tr, current, control.Service, func() error {
		return control.Upsert(ctx)
	})
}

func (cs *Changeset) upsertServiceAccount(ctx context.Context, tr *ChangesetResource, data []byte) (*ChangesetResource, error) {
	account, err := ParseServiceAccount(bytes.NewReader(data))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	log := log.WithFields(log.Fields{
		"cs":              tr.String(),
		"service_account": formatMeta(account.ObjectMeta),
	})
	accounts := cs.Client.CoreV1().ServiceAccounts(account.Namespace)
	current, err := accounts.Get(ctx, account.Name, metav1.GetOptions{})
	err = ConvertError(err)
	if err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		log.Debug("existing service account not found")
		current = nil
	}
	control, err := NewServiceAccountControl(ServiceAccountConfig{ServiceAccount: account, Client: cs.Client})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if current != nil {
		updateTypeMetaServiceAccount(current)
	}
	return cs.withUpsertOp(ctx, tr, current, control.ServiceAccount, func() error {
		return control.Upsert(ctx)
	})
}

func (cs *Changeset) upsertRole(ctx context.Context, tr *ChangesetResource, data []byte) (*ChangesetResource, error) {
	role, err := ParseRole(bytes.NewReader(data))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	log := log.WithFields(log.Fields{
		"cs":   tr.String(),
		"role": formatMeta(role.ObjectMeta),
	})
	roles := cs.Client.RbacV1().Roles(role.Namespace)
	current, err := roles.Get(ctx, role.Name, metav1.GetOptions{})
	err = ConvertError(err)
	if err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		log.Debug("existing role not found")
		current = nil
	}
	control, err := NewRoleControl(RoleConfig{Role: role, Client: cs.Client})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if current != nil {
		updateTypeMetaRole(current)
	}
	return cs.withUpsertOp(ctx, tr, current, control.Role, func() error {
		return control.Upsert(ctx)
	})
}

func (cs *Changeset) upsertClusterRole(ctx context.Context, tr *ChangesetResource, data []byte) (*ChangesetResource, error) {
	role, err := ParseClusterRole(bytes.NewReader(data))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	log := log.WithFields(log.Fields{
		"cs":           tr.String(),
		"cluster_role": formatMeta(role.ObjectMeta),
	})
	roles := cs.Client.RbacV1().ClusterRoles()
	current, err := roles.Get(ctx, role.Name, metav1.GetOptions{})
	err = ConvertError(err)
	if err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		log.Debug("existing cluster role not found")
		current = nil
	}
	control, err := NewClusterRoleControl(ClusterRoleConfig{ClusterRole: role, Client: cs.Client})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if current != nil {
		updateTypeMetaClusterRole(current)
	}
	return cs.withUpsertOp(ctx, tr, current, control.ClusterRole, func() error {
		return control.Upsert(ctx)
	})
}

func (cs *Changeset) upsertNamespace(
	ctx context.Context, tr *ChangesetResource, data []byte) (*ChangesetResource, error) {
	namespace, err := ParseNamespace(bytes.NewReader(data))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	log := log.WithFields(log.Fields{
		"cs":        tr.String(),
		"namespace": formatMeta(namespace.ObjectMeta),
	})
	namespaces := cs.Client.CoreV1().Namespaces()
	current, err := namespaces.Get(ctx, namespace.Name, metav1.GetOptions{})
	err = ConvertError(err)
	if err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		log.Debug("existing namespace not found")
		current = nil
	}
	control, err := NewNamespaceControl(NamespaceConfig{Namespace: namespace, Client: cs.Client})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if current != nil {
		updateTypeMetaNamespace(current)
	}
	return cs.withUpsertOp(ctx, tr, current, control.Namespace, func() error {
		return control.Upsert(ctx)
	})
}

func (cs *Changeset) upsertPriorityClass(
	ctx context.Context, tr *ChangesetResource, data []byte) (*ChangesetResource, error) {
	pc, err := ParsePriorityClass(bytes.NewReader(data))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	log := log.WithFields(log.Fields{
		"cs":             tr.String(),
		"priority_class": formatMeta(pc.ObjectMeta),
	})
	priorityClasses := cs.Client.SchedulingV1beta1().PriorityClasses()
	current, err := priorityClasses.Get(ctx, pc.Name, metav1.GetOptions{})
	err = ConvertError(err)
	if err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		log.Debug("existing PriorityClass not found")
		current = nil
	}
	control, err := NewPriorityClassControl(PriorityClassConfig{PriorityClass: pc, Client: cs.Client})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if current != nil {
		updateTypeMetaPriorityClass(current)
	}
	return cs.withUpsertOp(ctx, tr, current, control.PriorityClass, func() error {
		return control.Upsert(ctx)
	})
}

func (cs *Changeset) upsertValidatingWebhookConfiguration(ctx context.Context, tr *ChangesetResource, data []byte) (*ChangesetResource, error) {
	webhook, err := ParseValidatingWebhookConfiguration(bytes.NewReader(data))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	log := log.WithFields(log.Fields{
		"cs":                               tr.String(),
		"validating_webhook_configuration": fmt.Sprintf("%v/%v", webhook.Namespace, webhook.Name),
	})
	log.Infof("upsert validating webhook configuration %v", formatMeta(webhook.ObjectMeta))
	client := cs.Client.AdmissionregistrationV1().ValidatingWebhookConfigurations()
	current, err := client.Get(ctx, webhook.Name, metav1.GetOptions{})
	err = ConvertError(err)
	if err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		log.Debug("existing validating webhook configuration not found")
		current = nil
	}
	control, err := NewValidatingWebhookConfigurationControl(ValidatingWebhookConfigurationConfig{
		ValidatingWebhookConfiguration: webhook,
		Client:                         cs.Client,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if current != nil {
		updateTypeMetaValidatingWebhookConfiguration(current)
	}
	return cs.withUpsertOp(ctx, tr, current, control.ValidatingWebhookConfiguration, func() error {
		return control.Upsert(ctx)
	})
}

func (cs *Changeset) upsertMutatingWebhookConfiguration(ctx context.Context, tr *ChangesetResource, data []byte) (*ChangesetResource, error) {
	webhook, err := ParseMutatingWebhookConfiguration(bytes.NewReader(data))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	log := log.WithFields(log.Fields{
		"cs":                             tr.String(),
		"mutating_webhook_configuration": fmt.Sprintf("%v/%v", webhook.Namespace, webhook.Name),
	})
	log.Infof("upsert mutating webhook configuration %v", formatMeta(webhook.ObjectMeta))
	client := cs.Client.AdmissionregistrationV1().MutatingWebhookConfigurations()
	current, err := client.Get(ctx, webhook.Name, metav1.GetOptions{})
	if err != nil {
		if !trace.IsNotFound(ConvertError(err)) {
			return nil, trace.Wrap(err)
		}
		log.Debug("existing mutating webhook configuration not found")
		current = nil
	}
	control, err := NewMutatingWebhookConfigurationControl(MutatingWebhookConfigurationConfig{
		MutatingWebhookConfiguration: webhook,
		Client:                       cs.Client,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if current != nil {
		updateTypeMetaMutatingWebhookConfiguration(current)
	}
	return cs.withUpsertOp(ctx, tr, current, control.MutatingWebhookConfiguration, func() error {
		return control.Upsert(ctx)
	})
}

func (cs *Changeset) upsertAPIService(ctx context.Context, tr *ChangesetResource, data []byte) (*ChangesetResource, error) {
	apiService, err := ParseAPIService(bytes.NewReader(data))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	log := log.WithFields(log.Fields{
		"cs":          tr.String(),
		"api_service": fmt.Sprintf("%v/%v", apiService.Namespace, apiService.Name),
	})
	log.Infof("upsert api service %v", formatMeta(apiService.ObjectMeta))
	client := cs.APIRegistrationClient.ApiregistrationV1().APIServices()
	current, err := client.Get(ctx, apiService.Name, metav1.GetOptions{})
	err = ConvertError(err)
	if err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		log.Debug("existing api service not found")
		current = nil
	}
	control, err := NewAPIServiceControl(APIServiceConfig{APIService: apiService, Client: cs.APIRegistrationClient})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if current != nil {
		updateTypeMetaAPIService(current)
	}
	return cs.withUpsertOp(ctx, tr, current, control.APIService, func() error {
		return control.Upsert(ctx)
	})
}

func (cs *Changeset) upsertServiceMonitor(ctx context.Context, tr *ChangesetResource, data []byte) (*ChangesetResource, error) {
	monitor, err := ParseServiceMonitor(bytes.NewReader(data))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	log := log.WithFields(log.Fields{
		"cs":              tr.String(),
		"service_monitor": fmt.Sprintf("%v/%v", monitor.Namespace, monitor.Name),
	})
	log.Infof("upsert service monitor %v", formatMeta(monitor.ObjectMeta))
	client := cs.MonitoringClient.MonitoringV1().ServiceMonitors(monitor.Namespace)
	current, err := client.Get(ctx, monitor.Name, metav1.GetOptions{})
	err = ConvertError(err)
	if err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		log.Debug("existing service monitor not found")
		current = nil
	}
	control, err := NewServiceMonitorControl(ServiceMonitorConfig{ServiceMonitor: monitor, Client: cs.MonitoringClient})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if current != nil {
		updateTypeMetaServiceMonitor(current)
	}
	return cs.withUpsertOp(ctx, tr, current, control.ServiceMonitor, func() error {
		return control.Upsert(ctx)
	})
}

func (cs *Changeset) upsertAlertmanager(ctx context.Context, tr *ChangesetResource, data []byte) (*ChangesetResource, error) {
	alertmanager, err := ParseAlertmanager(bytes.NewReader(data))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	log := log.WithFields(log.Fields{
		"cs":            tr.String(),
		"alert_manager": fmt.Sprintf("%v/%v", alertmanager.Namespace, alertmanager.Name),
	})
	log.Infof("upsert alert manager %v", formatMeta(alertmanager.ObjectMeta))
	client := cs.MonitoringClient.MonitoringV1().Alertmanagers(alertmanager.Namespace)
	current, err := client.Get(ctx, alertmanager.Name, metav1.GetOptions{})
	err = ConvertError(err)
	if err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		log.Debug("existing alert manager not found")
		current = nil
	}
	control, err := NewAlertmanagerControl(AlertmanagerConfig{Alertmanager: alertmanager, Client: cs.MonitoringClient})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if current != nil {
		updateTypeMetaAlertmanager(current)
	}
	return cs.withUpsertOp(ctx, tr, current, control.Alertmanager, func() error {
		return control.Upsert(ctx)
	})
}

func (cs *Changeset) upsertPrometheus(ctx context.Context, tr *ChangesetResource, data []byte) (*ChangesetResource, error) {
	prometheus, err := ParsePrometheus(bytes.NewReader(data))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	log := log.WithFields(log.Fields{
		"cs":         tr.String(),
		"prometheus": fmt.Sprintf("%v/%v", prometheus.Namespace, prometheus.Name),
	})
	log.Infof("upsert prometheus %v", formatMeta(prometheus.ObjectMeta))
	client := cs.MonitoringClient.MonitoringV1().Prometheuses(prometheus.Namespace)
	current, err := client.Get(ctx, prometheus.Name, metav1.GetOptions{})
	err = ConvertError(err)
	if err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		log.Debug("existing prometheus not found")
		current = nil
	}
	control, err := NewPrometheusControl(PrometheusConfig{Prometheus: prometheus, Client: cs.MonitoringClient})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if current != nil {
		updateTypeMetaPrometheus(current)
	}
	return cs.withUpsertOp(ctx, tr, current, control.Prometheus, func() error {
		return control.Upsert(ctx)
	})
}

func (cs *Changeset) upsertPrometheusRule(ctx context.Context, tr *ChangesetResource, data []byte) (*ChangesetResource, error) {
	prometheusRule, err := ParsePrometheusRule(bytes.NewReader(data))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	log := log.WithFields(log.Fields{
		"cs":              tr.String(),
		"prometheus_rule": fmt.Sprintf("%v/%v", prometheusRule.Namespace, prometheusRule.Name),
	})
	log.Infof("upsert prometheus rule %v", formatMeta(prometheusRule.ObjectMeta))
	client := cs.MonitoringClient.MonitoringV1().PrometheusRules(prometheusRule.Namespace)
	current, err := client.Get(ctx, prometheusRule.Name, metav1.GetOptions{})
	err = ConvertError(err)
	if err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		log.Debug("existing prometheus rule not found")
		current = nil
	}
	control, err := NewPrometheusRuleControl(PrometheusRuleConfig{PrometheusRule: prometheusRule, Client: cs.MonitoringClient})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if current != nil {
		updateTypeMetaPrometheusRule(current)
	}
	return cs.withUpsertOp(ctx, tr, current, control.PrometheusRule, func() error {
		return control.Upsert(ctx)
	})
}

func (cs *Changeset) upsertRoleBinding(ctx context.Context, tr *ChangesetResource, data []byte) (*ChangesetResource, error) {
	binding, err := ParseRoleBinding(bytes.NewReader(data))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	log := log.WithFields(log.Fields{
		"cs":           tr.String(),
		"role_binding": formatMeta(binding.ObjectMeta),
	})
	bindings := cs.Client.RbacV1().RoleBindings(binding.Namespace)
	current, err := bindings.Get(ctx, binding.Name, metav1.GetOptions{})
	err = ConvertError(err)
	if err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		log.Debug("existing role binding not found")
		current = nil
	}
	control, err := NewRoleBindingControl(RoleBindingConfig{RoleBinding: binding, Client: cs.Client})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if current != nil {
		updateTypeMetaRoleBinding(current)
	}
	return cs.withUpsertOp(ctx, tr, current, control.RoleBinding, func() error {
		return control.Upsert(ctx)
	})
}

func (cs *Changeset) upsertClusterRoleBinding(ctx context.Context, tr *ChangesetResource, data []byte) (*ChangesetResource, error) {
	binding, err := ParseClusterRoleBinding(bytes.NewReader(data))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	log := log.WithFields(log.Fields{
		"cs":                   tr.String(),
		"cluster_role_binding": formatMeta(binding.ObjectMeta),
	})
	bindings := cs.Client.RbacV1().ClusterRoleBindings()
	current, err := bindings.Get(ctx, binding.Name, metav1.GetOptions{})
	err = ConvertError(err)
	if err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		log.Debug("existing cluster role binding not found")
		current = nil
	}
	control, err := NewClusterRoleBindingControl(ClusterRoleBindingConfig{ClusterRoleBinding: binding, Client: cs.Client})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if current != nil {
		updateTypeMetaClusterRoleBinding(current)
	}
	return cs.withUpsertOp(ctx, tr, current, control.ClusterRoleBinding, func() error {
		return control.Upsert(ctx)
	})
}

func (cs *Changeset) upsertPodSecurityPolicy(ctx context.Context, tr *ChangesetResource, data []byte) (*ChangesetResource, error) {
	policy, err := ParsePodSecurityPolicy(bytes.NewReader(data))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	log := log.WithFields(log.Fields{
		"cs":                  tr.String(),
		"pod_security_policy": formatMeta(policy.ObjectMeta),
	})
	policies := cs.Client.PolicyV1beta1().PodSecurityPolicies()
	current, err := policies.Get(ctx, policy.Name, metav1.GetOptions{})
	err = ConvertError(err)
	if err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		log.Debug("existing pod security policy not found")
		current = nil
	}
	control, err := NewPodSecurityPolicyControl(PodSecurityPolicyConfig{PodSecurityPolicy: policy, Client: cs.Client})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if current != nil {
		updateTypeMetaPodSecurityPolicy(current)
	}
	return cs.withUpsertOp(ctx, tr, current, control.PodSecurityPolicy, func() error {
		return control.Upsert(ctx)
	})
}

func (cs *Changeset) upsertCustomResourceDefinition(
	ctx context.Context, tr *ChangesetResource, data []byte) (*ChangesetResource, error) {
	crd, err := ParseCustomResourceDefinition(bytes.NewReader(data))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	log := log.WithFields(log.Fields{
		"cs":                        tr.String(),
		"custom_resource_defintion": formatMeta(crd.ObjectMeta),
	})
	current, err := cs.APIExtensionsClient.ApiextensionsV1beta1().CustomResourceDefinitions().
		Get(ctx, crd.Name, metav1.GetOptions{})
	err = ConvertError(err)
	if err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		log.Debug("existing custom resource definition not found")
		current = nil
	}
	control, err := NewCustomResourceDefinitionControl(CustomResourceDefinitionConfig{
		CustomResourceDefinition: crd,
		Client:                   cs.APIExtensionsClient,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if current != nil {
		updateTypeMetaCustomResourceDefinition(current)
	}
	return cs.withUpsertOp(ctx, tr, current, control.CustomResourceDefinition, func() error {
		return control.Upsert(ctx)
	})
}

func (cs *Changeset) upsertConfigMap(ctx context.Context, tr *ChangesetResource, data []byte) (*ChangesetResource, error) {
	configMap, err := ParseConfigMap(bytes.NewReader(data))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	log := log.WithFields(log.Fields{
		"cs":        tr.String(),
		"configMap": fmt.Sprintf("%v/%v", configMap.Namespace, configMap.Name),
	})
	log.Infof("upsert configmap %v", formatMeta(configMap.ObjectMeta))
	configMaps := cs.Client.CoreV1().ConfigMaps(configMap.Namespace)
	current, err := configMaps.Get(ctx, configMap.Name, metav1.GetOptions{})
	err = ConvertError(err)
	if err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		log.Debug("existing configmap not found")
		current = nil
	}
	control, err := NewConfigMapControl(ConfigMapConfig{ConfigMap: configMap, Client: cs.Client})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if current != nil {
		updateTypeMetaConfigMap(current)
	}
	return cs.withUpsertOp(ctx, tr, current, control.ConfigMap, func() error {
		return control.Upsert(ctx)
	})
}

func (cs *Changeset) upsertSecret(ctx context.Context, tr *ChangesetResource, data []byte) (*ChangesetResource, error) {
	secret, err := ParseSecret(bytes.NewReader(data))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	log := log.WithFields(log.Fields{
		"cs":     tr.String(),
		"secret": fmt.Sprintf("%v/%v", secret.Namespace, secret.Name),
	})
	log.Infof("upsert secret %v", formatMeta(secret.ObjectMeta))
	secrets := cs.Client.CoreV1().Secrets(secret.Namespace)
	current, err := secrets.Get(ctx, secret.Name, metav1.GetOptions{})
	err = ConvertError(err)
	if err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		log.Debug("existing secret not found")
		current = nil
	}
	control, err := NewSecretControl(SecretConfig{Secret: secret, Client: cs.Client})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if current != nil {
		updateTypeMetaSecret(current)
	}
	return cs.withUpsertOp(ctx, tr, current, control.Secret, func() error {
		return control.Upsert(ctx)
	})
}

func (cs *Changeset) Init(ctx context.Context) error {
	log.Debug("changeset init")

	// kubernetes 1.8 or newer
	crd := &apiextensions.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: ChangesetResourceName,
		},
		Spec: apiextensions.CustomResourceDefinitionSpec{
			Group:   ChangesetGroup,
			Version: ChangesetVersion,
			Scope:   ChangesetScope,
			Names: apiextensions.CustomResourceDefinitionNames{
				Kind:     KindChangeset,
				Plural:   ChangesetPlural,
				Singular: ChangesetSingular,
			},
		},
	}

	_, err := cs.APIExtensionsClient.ApiextensionsV1beta1().CustomResourceDefinitions().Create(ctx, crd, metav1.CreateOptions{})
	err = ConvertError(err)
	if err != nil {
		if !trace.IsAlreadyExists(err) {
			return trace.Wrap(err)
		}
	}
	// wait for the controller to init by trying to list stuff
	return retry(ctx, 30, time.Second, func(context.Context) error {
		_, err := cs.list(ctx, DefaultNamespace)
		return err
	})

}

func (cs *Changeset) Get(ctx context.Context, namespace, name string) (*ChangesetResource, error) {
	return cs.get(ctx, namespace, name)
}

func (cs *Changeset) List(ctx context.Context, namespace string) (*ChangesetList, error) {
	return cs.list(ctx, namespace)
}

// Create creates a new one given the name and namespace.
// The new changeset is created with status in-progress.
// If there's already a changeset with this name in this namespace, AlreadyExists
// error is returned.
func (cs *Changeset) Create(ctx context.Context, namespace, name string) (*ChangesetResource, error) {
	res := &ChangesetResource{
		TypeMeta: metav1.TypeMeta{
			Kind:       KindChangeset,
			APIVersion: ChangesetAPIVersion,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: ChangesetSpec{
			Status: ChangesetStatusInProgress,
		},
	}
	return cs.create(ctx, res)
}

func (cs *Changeset) upsert(ctx context.Context, tr *ChangesetResource) (*ChangesetResource, error) {
	out, err := cs.create(ctx, tr)
	if err == nil {
		return out, nil
	}
	if !trace.IsAlreadyExists(err) {
		return nil, err
	}
	return cs.update(ctx, tr)
}

func (cs *Changeset) create(ctx context.Context, tr *ChangesetResource) (*ChangesetResource, error) {
	tr.Namespace = Namespace(tr.Namespace)
	data, err := json.Marshal(tr)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var raw runtime.Unknown
	err = cs.client.Post().
		SubResource("namespaces", tr.Namespace, ChangesetCollection).
		Body(data).
		Do(ctx).
		Into(&raw)
	if err != nil {
		return nil, ConvertError(err)
	}
	var result ChangesetResource
	if err := json.Unmarshal(raw.Raw, &result); err != nil {
		return nil, trace.Wrap(err)
	}
	return &result, nil
}

func (cs *Changeset) get(ctx context.Context, namespace, name string) (*ChangesetResource, error) {
	var raw runtime.Unknown
	err := cs.client.Get().
		SubResource("namespaces", namespace, ChangesetCollection, name).
		Do(ctx).
		Into(&raw)
	if err != nil {
		return nil, ConvertError(err)
	}
	var result ChangesetResource
	if err := json.Unmarshal(raw.Raw, &result); err != nil {
		return nil, trace.Wrap(err)
	}
	return &result, nil
}

func (cs *Changeset) createOrRead(ctx context.Context, namespace, name string, spec ChangesetSpec) (*ChangesetResource, error) {
	res := &ChangesetResource{
		TypeMeta: metav1.TypeMeta{
			Kind:       KindChangeset,
			APIVersion: ChangesetAPIVersion,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: spec,
	}
	out, err := cs.create(ctx, res)
	if err == nil {
		return out, nil
	}
	if !trace.IsAlreadyExists(err) {
		return nil, trace.Wrap(err)
	}
	return cs.get(ctx, res.Namespace, res.Name)
}

func (cs *Changeset) Delete(ctx context.Context, namespace, name string) error {
	var raw runtime.Unknown
	err := cs.client.Delete().
		SubResource("namespaces", namespace, ChangesetCollection, name).
		Do(ctx).
		Into(&raw)
	if err != nil {
		return ConvertError(err)
	}
	return nil
}

func (cs *Changeset) update(ctx context.Context, tr *ChangesetResource) (*ChangesetResource, error) {
	tr.Namespace = Namespace(tr.Namespace)
	data, err := json.Marshal(tr)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var raw runtime.Unknown
	err = cs.client.Put().
		SubResource("namespaces", tr.Namespace, ChangesetCollection, tr.Name).
		Body(data).
		Do(ctx).
		Into(&raw)
	if err != nil {
		return nil, ConvertError(err)
	}
	var result ChangesetResource
	if err := json.Unmarshal(raw.Raw, &result); err != nil {
		return nil, trace.Wrap(err)
	}
	return &result, nil
}

func (cs *Changeset) list(ctx context.Context, namespace string) (*ChangesetList, error) {
	var raw runtime.Unknown
	err := cs.client.Get().
		SubResource("namespaces", namespace, ChangesetCollection).
		Do(ctx).
		Into(&raw)
	if err != nil {
		return nil, ConvertError(err)
	}
	var result ChangesetList
	if err := json.Unmarshal(raw.Raw, &result); err != nil {
		return nil, trace.Wrap(err)
	}
	return &result, nil
}
