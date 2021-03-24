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

package phases

import (
	"context"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/kubernetes"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/update"

	"github.com/gravitational/rigging"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	rbacv1beta1 "k8s.io/api/rbac/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeapi "k8s.io/client-go/kubernetes"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

// phaseTaint defines the operation of adding a taint to the node
type phaseTaint struct {
	kubernetesOperation
}

// NewPhaseTaint returns a new executor for adding a taint to a node
func NewPhaseTaint(p fsm.ExecutorParams, client *kubeapi.Clientset, logger log.FieldLogger) (*phaseTaint, error) {
	op, err := newKubernetesOperation(p, client, logger)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &phaseTaint{
		kubernetesOperation: *op,
	}, nil
}

// Execute adds a taint on the specified node.
func (p *phaseTaint) Execute(ctx context.Context) error {
	p.Infof("Taint %v.", p.Server)
	err := taint(ctx, p.Client.CoreV1().Nodes(), p.Server.KubeNodeID(), addTaint(true))
	return trace.Wrap(err)
}

// Rollback removes the taint from the node
func (p *phaseTaint) Rollback(ctx context.Context) error {
	p.Infof("Remove taint from %v.", p.Server)
	err := taint(ctx, p.Client.CoreV1().Nodes(), p.Server.KubeNodeID(), addTaint(false))
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	return nil
}

// phaseUntaint defines the operation of removing a taint from the node
type phaseUntaint struct {
	kubernetesOperation
}

// NewPhaseUntaint returns a new executor for removing a taint from a node
func NewPhaseUntaint(p fsm.ExecutorParams, client *kubeapi.Clientset, logger log.FieldLogger) (*phaseUntaint, error) {
	op, err := newKubernetesOperation(p, client, logger)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &phaseUntaint{
		kubernetesOperation: *op,
	}, nil
}

// Execute removes a taint from the specified node.
func (p *phaseUntaint) Execute(ctx context.Context) error {
	p.Infof("Remove taint from %v.", p.Server)
	err := taint(ctx, p.Client.CoreV1().Nodes(), p.Server.KubeNodeID(), addTaint(false))
	// If the remove step has partially run, the taint might have also been removed
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	return nil
}

// Rollback is a no-op for this phase
func (p *phaseUntaint) Rollback(context.Context) error {
	return nil
}

// phaseDrain defines the operation of draining a node
type phaseDrain struct {
	kubernetesOperation
}

// NewPhaseDrain returns a new executor for draining a node
func NewPhaseDrain(p fsm.ExecutorParams, client *kubeapi.Clientset, logger log.FieldLogger) (*phaseDrain, error) {
	op, err := newKubernetesOperation(p, client, logger)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &phaseDrain{
		kubernetesOperation: *op,
	}, nil
}

// Execute drains the specified node
func (p *phaseDrain) Execute(ctx context.Context) error {
	p.Infof("Drain %v.", p.Server)
	ctx, cancel := context.WithTimeout(ctx, defaults.DrainTimeout)
	defer cancel()
	err := update.Retry(ctx, func() error {
		return trace.Wrap(drain(ctx, p.Client, p.Server.KubeNodeID()))
	}, defaults.DrainErrorTimeout)
	return trace.Wrap(err)
}

// Rollback reverts the effect of drain by uncordoning the node
func (p *phaseDrain) Rollback(ctx context.Context) error {
	p.Infof("Uncordon %v.", p.Server)
	err := uncordon(ctx, p.Client.CoreV1().Nodes(), p.Server.KubeNodeID())
	return trace.Wrap(err)
}

// phaseKubeletPermissions defines the operation to bootstrap additional permissions for kubelet.
// This is necessary for a master node that is upgraded first and needs to update node status (via patch)
// on an older api server.
type phaseKubeletPermissions struct {
	kubernetesOperation
}

// NewPhaseKubeletPermissions returns a new executor for bootstrapping additional kubelet permissions
func NewPhaseKubeletPermissions(p fsm.ExecutorParams, client *kubeapi.Clientset, logger log.FieldLogger) (*phaseKubeletPermissions, error) {
	op, err := newKubernetesOperation(p, client, logger)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &phaseKubeletPermissions{
		kubernetesOperation: *op,
	}, nil
}

// Execute adds additional permissions for kubelet
func (p *phaseKubeletPermissions) Execute(context.Context) error {
	p.Infof("Update kubelet perrmissiong on %v.", p.Server)
	err := updateKubeletPermissions(p.Client)
	return trace.Wrap(err)
}

// Rollback removes the previously added clusterrole/clusterrolebinding for kubelet
func (p *phaseKubeletPermissions) Rollback(context.Context) error {
	return trace.Wrap(removeKubeletPermissions(p.Client))
}

// phaseUncordon defines the operation of uncordoning a node
type phaseUncordon struct {
	kubernetesOperation
}

// NewPhaseUncordon returns a new executor for uncordoning a node
func NewPhaseUncordon(p fsm.ExecutorParams, client *kubeapi.Clientset, logger log.FieldLogger) (*phaseUncordon, error) {
	op, err := newKubernetesOperation(p, client, logger)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &phaseUncordon{
		kubernetesOperation: *op,
	}, nil
}

// Execute uncordons the specified node.
// This will block until DNS/cluster controller endpoints are populated
func (p *phaseUncordon) Execute(ctx context.Context) error {
	p.Infof("Uncordon %v.", p.Server)
	err := uncordon(ctx, p.Client.CoreV1().Nodes(), p.Server.KubeNodeID())
	return trace.Wrap(err)
}

// Rollback is a no-op for this phase
func (p *phaseUncordon) Rollback(context.Context) error {
	return nil
}

// phaseEndpoints defines the operation waiting for DNS/cluster endpoints after
// a node has been drained
type phaseEndpoints struct {
	kubernetesOperation
}

// NewPhaseEndpoints returns a new executor for waiting for endpoints
func NewPhaseEndpoints(p fsm.ExecutorParams, client *kubeapi.Clientset, logger log.FieldLogger) (*phaseEndpoints, error) {
	op, err := newKubernetesOperation(p, client, logger)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &phaseEndpoints{
		kubernetesOperation: *op,
	}, nil
}

// Execute waits for endpoints
func (p *phaseEndpoints) Execute(ctx context.Context) error {
	p.Infof("Wait for endpoints on %v.", p.Server)
	err := update.WaitForEndpoints(ctx, p.Client.CoreV1(), p.Server)
	return trace.Wrap(err)
}

// Rollback is a no-op for this phase
func (p *phaseEndpoints) Rollback(context.Context) error {
	return nil
}

func newKubernetesOperation(p fsm.ExecutorParams, client *kubeapi.Clientset, logger log.FieldLogger) (*kubernetesOperation, error) {
	if p.Phase.Data == nil || p.Phase.Data.Server == nil {
		return nil, trace.NotFound("no server specified for phase %q", p.Phase.ID)
	}

	if client == nil {
		return nil, trace.BadParameter("phase %q must be run from a master node (requires kubernetes client)", p.Phase.ID)
	}
	return &kubernetesOperation{
		Client:      client,
		OperationID: p.Plan.OperationID,
		Server:      *p.Phase.Data.Server,
		Servers:     p.Plan.Servers,
		FieldLogger: logger,
	}, nil
}

// kubernetesOperation defines a kubernetes operation
type kubernetesOperation struct {
	// Client specifies the kubernetes API client
	Client *kubeapi.Clientset
	// OperationID is the id of the current update operation
	OperationID string
	// Server is the server currently being updated
	Server storage.Server
	// Servers is the list of servers being updated
	Servers []storage.Server
	log.FieldLogger
}

// PreCheck makes sure the phase is being executed on the correct server
func (p *kubernetesOperation) PreCheck(context.Context) error {
	return trace.Wrap(fsm.CheckMasterServer(p.Servers))
}

// PostCheck is no-op for this phase
func (p *kubernetesOperation) PostCheck(context.Context) error {
	return nil
}

func taint(ctx context.Context, client corev1.NodeInterface, node string, add addTaint) error {
	taint := v1.Taint{
		Key:    defaults.RunLevelLabel,
		Value:  defaults.RunLevelSystem,
		Effect: v1.TaintEffectNoExecute,
	}

	var taintsToAdd, taintsToRemove []v1.Taint
	if add {
		taintsToAdd = append(taintsToAdd, taint)
	} else {
		taintsToRemove = append(taintsToRemove, taint)
	}

	err := kubernetes.UpdateTaints(ctx, client, node, taintsToAdd, taintsToRemove)
	if err != nil {
		if add {
			return trace.Wrap(err, "failed to add taint %v to node %q", taint, node)
		}
		return trace.Wrap(err, "failed to remove taint %v from node %q", taint, node)
	}
	return nil
}

func drain(ctx context.Context, client *kubeapi.Clientset, node string) error {
	err := kubernetes.Drain(ctx, client, node)
	return trace.Wrap(err)
}

func uncordon(ctx context.Context, client corev1.NodeInterface, node string) error {
	err := kubernetes.SetUnschedulable(ctx, client, node, false)
	return trace.Wrap(err)
}

func updateKubeletPermissions(client *kubeapi.Clientset) error {
	err := createKubeletRole(client)
	if err != nil && !trace.IsAlreadyExists(err) {
		return trace.Wrap(err)
	}

	err = createKubeletRoleBinding(client)
	if err != nil && !trace.IsAlreadyExists(err) {
		return trace.Wrap(err)
	}
	return nil
}

func createKubeletRole(client *kubeapi.Clientset) error {
	_, err := client.RbacV1().ClusterRoles().Create(context.TODO(), &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: defaults.KubeletUpdatePermissionsRole},
		Rules: []rbacv1.PolicyRule{
			{Verbs: []string{"patch"}, APIGroups: []string{""}, Resources: []string{"nodes/status"}},
		},
	}, metav1.CreateOptions{})

	err = rigging.ConvertError(err)
	if err == nil {
		return nil
	}
	if !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	// If there's no RBAC v1 support, drop down to v1beta1
	_, err = client.RbacV1beta1().ClusterRoles().Create(context.TODO(), &rbacv1beta1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: defaults.KubeletUpdatePermissionsRole},
		Rules: []rbacv1beta1.PolicyRule{
			{Verbs: []string{"patch"}, APIGroups: []string{""}, Resources: []string{"nodes/status"}},
		},
	}, metav1.CreateOptions{})
	return trace.Wrap(rigging.ConvertError(err))
}

func createKubeletRoleBinding(client *kubeapi.Clientset) error {
	_, err := client.RbacV1().ClusterRoleBindings().Create(context.TODO(), &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: defaults.KubeletUpdatePermissionsRole},
		Subjects:   []rbacv1.Subject{{Kind: constants.KubernetesKindUser, Name: constants.KubeletUser}},
		RoleRef: rbacv1.RoleRef{
			APIGroup: constants.RbacAPIGroup,
			Name:     defaults.KubeletUpdatePermissionsRole,
			Kind:     rigging.KindClusterRole,
		},
	}, metav1.CreateOptions{})
	err = rigging.ConvertError(err)
	if err == nil {
		return nil
	}
	if !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	// If there's no RBAC v1 support, drop down to v1beta1
	_, err = client.RbacV1beta1().ClusterRoleBindings().Create(context.TODO(), &rbacv1beta1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: defaults.KubeletUpdatePermissionsRole},
		Subjects:   []rbacv1beta1.Subject{{Kind: constants.KubernetesKindUser, Name: constants.KubeletUser}},
		RoleRef: rbacv1beta1.RoleRef{
			APIGroup: constants.RbacAPIGroup,
			Name:     defaults.KubeletUpdatePermissionsRole,
			Kind:     rigging.KindClusterRole,
		},
	}, metav1.CreateOptions{})
	return trace.Wrap(rigging.ConvertError(err))
}

func removeKubeletPermissions(client *kubeapi.Clientset) error {
	err := rigging.ConvertError(client.RbacV1().ClusterRoles().
		Delete(context.TODO(), defaults.KubeletUpdatePermissionsRole, metav1.DeleteOptions{}))
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	err = rigging.ConvertError(client.RbacV1().ClusterRoleBindings().
		Delete(context.TODO(), defaults.KubeletUpdatePermissionsRole, metav1.DeleteOptions{}))
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	return nil
}

type addTaint bool
