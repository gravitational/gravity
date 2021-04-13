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
	"io"

	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/gravitational/trace"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/api/policy/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	schedulingv1beta1 "k8s.io/api/scheduling/v1beta1"
	extensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
)

type ResourceHeader struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
}

// ParseResourceHeader parses resource header information
func ParseResourceHeader(reader io.Reader) (*ResourceHeader, error) {
	var out ResourceHeader
	err := yaml.NewYAMLOrJSONDecoder(reader, DefaultBufferSize).Decode(&out)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &out, nil
}

// ParseDaemonSet parses daemon set from reader
func ParseDaemonSet(r io.Reader) (*appsv1.DaemonSet, error) {
	if r == nil {
		return nil, trace.BadParameter("missing reader")
	}
	ds := appsv1.DaemonSet{}
	err := yaml.NewYAMLOrJSONDecoder(r, DefaultBufferSize).Decode(&ds)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &ds, nil
}

// ParseStatefulSet parses statefulset resource from reader
func ParseStatefulSet(r io.Reader) (*appsv1.StatefulSet, error) {
	if r == nil {
		return nil, trace.BadParameter("missing reader")
	}
	ss := appsv1.StatefulSet{}
	err := yaml.NewYAMLOrJSONDecoder(r, DefaultBufferSize).Decode(&ss)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &ss, nil
}

// ParseJob parses the specified reader as a Job resource
func ParseJob(r io.Reader) (*batchv1.Job, error) {
	if r == nil {
		return nil, trace.BadParameter("missing reader")
	}

	var job batchv1.Job
	err := yaml.NewYAMLOrJSONDecoder(r, DefaultBufferSize).Decode(&job)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &job, nil
}

// ParseReplicationController parses replication controller
func ParseReplicationController(r io.Reader) (*v1.ReplicationController, error) {
	if r == nil {
		return nil, trace.BadParameter("missing reader")
	}
	rc := v1.ReplicationController{}
	err := yaml.NewYAMLOrJSONDecoder(r, DefaultBufferSize).Decode(&rc)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &rc, nil
}

// ParseDeployment parses deployment
func ParseDeployment(r io.Reader) (*appsv1.Deployment, error) {
	if r == nil {
		return nil, trace.BadParameter("missing reader")
	}
	rc := appsv1.Deployment{}
	err := yaml.NewYAMLOrJSONDecoder(r, DefaultBufferSize).Decode(&rc)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &rc, nil
}

// ParseService parses service
func ParseService(r io.Reader) (*v1.Service, error) {
	if r == nil {
		return nil, trace.BadParameter("missing reader")
	}
	svc := v1.Service{}
	err := yaml.NewYAMLOrJSONDecoder(r, DefaultBufferSize).Decode(&svc)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &svc, nil
}

// ParseConfigMap parses Config Map
func ParseConfigMap(r io.Reader) (*v1.ConfigMap, error) {
	if r == nil {
		return nil, trace.BadParameter("missing reader")
	}
	cm := v1.ConfigMap{}
	err := yaml.NewYAMLOrJSONDecoder(r, DefaultBufferSize).Decode(&cm)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &cm, nil
}

// ParseCustomResourceDefinition parses a Custom Resource Definition
func ParseCustomResourceDefinition(r io.Reader) (*extensionsv1beta1.CustomResourceDefinition, error) {
	if r == nil {
		return nil, trace.BadParameter("missing reader")
	}
	cm := extensionsv1beta1.CustomResourceDefinition{}
	err := yaml.NewYAMLOrJSONDecoder(r, DefaultBufferSize).Decode(&cm)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &cm, nil
}

// ParseSecret parses a secret from the specified stream
func ParseSecret(r io.Reader) (*v1.Secret, error) {
	if r == nil {
		return nil, trace.BadParameter("missing reader")
	}
	secret := v1.Secret{}
	err := yaml.NewYAMLOrJSONDecoder(r, DefaultBufferSize).Decode(&secret)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &secret, nil
}

// ParseServiceAccount parses a service account from the specified stream
func ParseServiceAccount(r io.Reader) (*v1.ServiceAccount, error) {
	var account v1.ServiceAccount
	err := yaml.NewYAMLOrJSONDecoder(r, DefaultBufferSize).Decode(&account)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &account, nil
}

// ParseRole parses an rbac role from the specified stream
func ParseRole(r io.Reader) (*rbacv1.Role, error) {
	var role rbacv1.Role
	err := yaml.NewYAMLOrJSONDecoder(r, DefaultBufferSize).Decode(&role)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &role, nil
}

// ParseClusterRole parses an rbac cluster role from the specified stream
func ParseClusterRole(r io.Reader) (*rbacv1.ClusterRole, error) {
	var role rbacv1.ClusterRole
	err := yaml.NewYAMLOrJSONDecoder(r, DefaultBufferSize).Decode(&role)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &role, nil
}

// ParseNamespace parses a namespace object from the specified stream
func ParseNamespace(r io.Reader) (*v1.Namespace, error) {
	var namespace v1.Namespace
	err := yaml.NewYAMLOrJSONDecoder(r, DefaultBufferSize).Decode(&namespace)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &namespace, nil
}

// ParsePriorityClass parses a PriorityClass object from the specified stream
func ParsePriorityClass(r io.Reader) (*schedulingv1beta1.PriorityClass, error) {
	var pc schedulingv1beta1.PriorityClass
	err := yaml.NewYAMLOrJSONDecoder(r, DefaultBufferSize).Decode(&pc)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &pc, nil
}

// ParseRoleBinding parses an rbac role binding from the specified stream
func ParseRoleBinding(r io.Reader) (*rbacv1.RoleBinding, error) {
	var binding rbacv1.RoleBinding
	err := yaml.NewYAMLOrJSONDecoder(r, DefaultBufferSize).Decode(&binding)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &binding, nil
}

// ParseClusterRoleBinding parses an rbac cluster role binding from the specified stream
func ParseClusterRoleBinding(r io.Reader) (*rbacv1.ClusterRoleBinding, error) {
	var binding rbacv1.ClusterRoleBinding
	err := yaml.NewYAMLOrJSONDecoder(r, DefaultBufferSize).Decode(&binding)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &binding, nil
}

// ParsePodSecurityPolicy parses a pod security policy from the specified stream
func ParsePodSecurityPolicy(r io.Reader) (*v1beta1.PodSecurityPolicy, error) {
	var policy v1beta1.PodSecurityPolicy
	err := yaml.NewYAMLOrJSONDecoder(r, DefaultBufferSize).Decode(&policy)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &policy, nil
}

// ParseAPIService parses a APIService resource from the provided data
func ParseAPIService(r io.Reader) (*apiregistrationv1.APIService, error) {
	var apiService apiregistrationv1.APIService
	err := yaml.NewYAMLOrJSONDecoder(r, DefaultBufferSize).Decode(&apiService)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &apiService, nil
}

// ParseValidatingWebhookConfiguration parses a ValidatingWebhookConfiguration
// resources from the provided data stream.
func ParseValidatingWebhookConfiguration(r io.Reader) (*admissionregistrationv1.ValidatingWebhookConfiguration, error) {
	var webhook admissionregistrationv1.ValidatingWebhookConfiguration
	err := yaml.NewYAMLOrJSONDecoder(r, DefaultBufferSize).Decode(&webhook)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &webhook, nil
}

// ParseMutatingWebhookConfiguration parses a MutatingWebhookConfiguration
// resources from the provided data stream.
func ParseMutatingWebhookConfiguration(r io.Reader) (*admissionregistrationv1.MutatingWebhookConfiguration, error) {
	var webhook admissionregistrationv1.MutatingWebhookConfiguration
	err := yaml.NewYAMLOrJSONDecoder(r, DefaultBufferSize).Decode(&webhook)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &webhook, nil
}

// ParseServiceMonitor parses a ServiceMonitor resource from the provided data
func ParseServiceMonitor(r io.Reader) (*monitoringv1.ServiceMonitor, error) {
	var monitor monitoringv1.ServiceMonitor
	err := yaml.NewYAMLOrJSONDecoder(r, DefaultBufferSize).Decode(&monitor)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &monitor, nil
}

// ParseAlertmanager parses a Alertmanager resource from the provided data
func ParseAlertmanager(r io.Reader) (*monitoringv1.Alertmanager, error) {
	var alertManager monitoringv1.Alertmanager
	err := yaml.NewYAMLOrJSONDecoder(r, DefaultBufferSize).Decode(&alertManager)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &alertManager, nil
}

// ParsePrometheus parses a Prometheus resource from the provided data
func ParsePrometheus(r io.Reader) (*monitoringv1.Prometheus, error) {
	var prometheus monitoringv1.Prometheus
	err := yaml.NewYAMLOrJSONDecoder(r, DefaultBufferSize).Decode(&prometheus)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &prometheus, nil
}

// ParsePrometheusRule parses a PrometheusRule resource from the provided data
func ParsePrometheusRule(r io.Reader) (*monitoringv1.PrometheusRule, error) {
	var prometheusRule monitoringv1.PrometheusRule
	err := yaml.NewYAMLOrJSONDecoder(r, DefaultBufferSize).Decode(&prometheusRule)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &prometheusRule, nil
}
