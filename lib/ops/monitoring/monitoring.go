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

package monitoring

import (
	"time"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/rigging"
	"github.com/gravitational/trace"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

// Monitoring defines the interface for monitoring provider
type Monitoring interface {
	// GetRetentionPolicies returns a list of retention policies
	GetRetentionPolicies() ([]RetentionPolicy, error)
	// UpdateRetentionPolicy updates a retention policy
	UpdateRetentionPolicy(RetentionPolicy) error
}

// RetentionPolicy represents a single retention policy
type RetentionPolicy struct {
	// Name is the policy name
	Name string `json:"name"`
	// Duration is the policy duration
	Duration time.Duration `json:"duration"`
}

// GetNamespace uses the provided Kubernetes client to determine namespace
// where monitoring resources reside
func GetNamespace(client corev1.CoreV1Interface) (string, error) {
	// try "monitoring" namespace first, then "kube-system"
	for _, ns := range []string{defaults.MonitoringNamespace, defaults.KubeSystemNamespace} {
		_, err := client.Services(ns).Get(defaults.GrafanaServiceName, metav1.GetOptions{})
		if err != nil && !trace.IsNotFound(rigging.ConvertError(err)) {
			return "", trace.Wrap(err)
		}
		if err == nil {
			return ns, nil
		}
	}
	return "", trace.NotFound("service %q was not found", defaults.GrafanaServiceName)
}

// GetInfluxDBCredentials uses to get username and password for
// InfluxDB administrator user
func GetInfluxDBCredentials(client corev1.CoreV1Interface) (username, password []byte, err error) {
	secret, err := client.Secrets(defaults.MonitoringNamespace).Get(defaults.InfluxDBSecretName, metav1.GetOptions{})
	if err != nil {
		return nil, nil, trace.Wrap(rigging.ConvertError(err))
	}

	username, ok := secret.Data["username"]
	if !ok {
		return nil, nil, trace.NotFound("username for InfuxDB administrator not found")
	}
	password, ok = secret.Data["password"]
	if !ok {
		return nil, nil, trace.NotFound("password for InfuxDB administrator not found")
	}

	return username, password, nil
}
