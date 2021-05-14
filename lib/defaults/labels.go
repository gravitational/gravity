/*
Copyright 2021 Gravitational, Inc.

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

package defaults

import (
	"strings"

	"github.com/gravitational/trace"
)

const (
	// KubernetesRoleLabel is the Kubernetes node label with system role
	KubernetesRoleLabel = "gravitational.io/k8s-role"

	// KubernetesAdvertiseIPLabel is the kubernetes node label of the advertise IP address
	KubernetesAdvertiseIPLabel = "gravitational.io/advertise-ip"

	// RunLevelLabel is the Kubernetes node taint label representing a run-level
	RunLevelLabel = "gravitational.io/runlevel"

	// KubernetesReconcileLabel is the kubernetes node label to control the reconcile process for the node
	KubernetesReconcileLabel = "gravitational.io/reconcile"
)

// ReconcileMode is the type for reconcile mode values
type ReconcileMode string

const (
	// ReconcileModeEnsureExists describes a label reconciliation mode when the labels will only be checked for existence. Users can edit the labels as they want.
	// This is default value for ReconcileMode.
	// Valid values: "EnsureExists"
	ReconcileModeEnsureExists = "EnsureExists"

	// ReconcileModeEnabled enables full reconciliation.
	// If the value of the label on the node is not equal to the value from the NodeProfile, the value will be restored from the profile.
	// Valid values: "Enabled", "enabled", "true", "True"
	ReconcileModeEnabled = "Enabled"

	// ReconcileModeDisabled disables reconciliation.
	// Valid values: "Disabled", "disabled", "false", "False"
	ReconcileModeDisabled = "Disabled"
)

// ParseReconcileMode parses the value to determine the reconciliation mode
func ParseReconcileMode(v string) (ReconcileMode, error) {
	if len(strings.TrimSpace(v)) == 0 {
		return "", trace.BadParameter("empty ReconcileMode value")
	}
	switch strings.ToLower(v) {
	case strings.ToLower(ReconcileModeEnsureExists):
		return ReconcileModeEnsureExists, nil
	case strings.ToLower(ReconcileModeEnabled), "true":
		return ReconcileModeEnabled, nil
	case strings.ToLower(ReconcileModeDisabled), "false":
		return ReconcileModeDisabled, nil
	}
	return "", trace.BadParameter("unable to parse ReconcileMode value: %q", v)
}
