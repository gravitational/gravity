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
	"context"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/rigging"
	"github.com/gravitational/trace"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

// GetNamespace uses the provided Kubernetes client to determine namespace
// where monitoring resources reside
func GetNamespace(client corev1.CoreV1Interface) (string, error) {
	// try "monitoring" namespace first, then "kube-system"
	for _, ns := range []string{defaults.MonitoringNamespace, defaults.KubeSystemNamespace} {
		_, err := client.Services(ns).Get(context.TODO(), defaults.GrafanaServiceName, metav1.GetOptions{})
		if err != nil && !trace.IsNotFound(rigging.ConvertError(err)) {
			return "", trace.Wrap(err)
		}
		if err == nil {
			return ns, nil
		}
	}
	return "", trace.NotFound("service %q was not found", defaults.GrafanaServiceName)
}
