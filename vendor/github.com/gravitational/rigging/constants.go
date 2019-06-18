package rigging

import (
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ChangesetResourceName     = "changesets.changeset.gravitational.io"
	ChangesetGroup            = "changeset.gravitational.io"
	ChangesetVersion          = "v1"
	ChangesetCollection       = "changesets"
	ChangesetPlural           = "changesets"
	ChangesetSingular         = "changeset"
	ChangesetScope            = "Namespaced"
	DefaultNamespace          = "default"
	KindDaemonSet             = "DaemonSet"
	KindStatefulSet           = "StatefulSet"
	KindChangeset             = "Changeset"
	KindConfigMap             = "ConfigMap"
	KindDeployment            = "Deployment"
	KindReplicaSet            = "ReplicaSet"
	KindReplicationController = "ReplicationController"
	KindService               = "Service"
	KindServiceAccount        = "ServiceAccount"
	KindSecret                = "Secret"
	KindJob                   = "Job"
	KindRole                  = "Role"
	KindClusterRole           = "ClusterRole"
	KindRoleBinding           = "RoleBinding"
	KindClusterRoleBinding    = "ClusterRoleBinding"
	KindPodSecurityPolicy     = "PodSecurityPolicy"
	ControllerUIDLabel        = "controller-uid"
	OpStatusCreated           = "created"
	OpStatusCompleted         = "completed"
	OpStatusReverted          = "reverted"
	ChangesetStatusReverted   = "reverted"
	ChangesetStatusInProgress = "in-progress"
	ChangesetStatusCommitted  = "committed"
	// DefaultRetryAttempts specifies amount of retry attempts for checks
	DefaultRetryAttempts = 60
	// RetryPeriod is a period between Retries
	DefaultRetryPeriod = time.Second
	DefaultBufferSize  = 1024

	ChangesetAPIVersion = "changeset.gravitational.io/v1"
)

// NamespaceOrDefault returns a default namespace if the specified namespace is empty
func Namespace(namespace string) string {
	if namespace == "" {
		return DefaultNamespace
	}
	return namespace
}

// formatMeta formats this meta as text
func formatMeta(meta metav1.ObjectMeta) string {
	if meta.Namespace == "" {
		return meta.Name
	}
	return fmt.Sprintf("%v/%v", Namespace(meta.Namespace), meta.Name)
}
