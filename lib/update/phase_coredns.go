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

package update

import (
	"context"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/rigging"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	corednsResourceName = "gravity:coredns"
)

func (r phaseBuilder) corednsPhase(leadMaster storage.Server) *phase {
	phase := root(phase{
		ID:          "coredns",
		Description: "Provision coredns resources",
		Executor:    coredns,
		Data: &storage.OperationPhaseData{
			Server: &leadMaster,
		},
	})
	return &phase
}

type updatePhaseCoreDNS struct {
	log.FieldLogger
	kubernetesOperation
}

// NewPhaseCoreDNS does provisioning for coredns during the upgrade
func NewPhaseCoreDNS(c FSMConfig, plan storage.OperationPlan, phase storage.OperationPhase) (*updatePhaseCoreDNS, error) {
	op, err := newKubernetesOperation(c, plan, phase)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &updatePhaseCoreDNS{
		kubernetesOperation: *op,
		FieldLogger:         log.NewEntry(log.New()),
	}, nil
}

// Execute
func (p *updatePhaseCoreDNS) Execute(ctx context.Context) error {
	_, err := p.kubernetesOperation.Client.RbacV1().ClusterRoles().Create(&rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: corednsResourceName},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Verbs:     []string{"list", "watch"},
				Resources: []string{"endpoints", "services", "namespaces"},
			},
		},
	})
	err = rigging.ConvertError(err)
	if err != nil && !trace.IsAlreadyExists(err) {
		return trace.Wrap(err)
	}

	_, err = p.kubernetesOperation.Client.RbacV1().ClusterRoleBindings().Create(&rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: corednsResourceName},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     corednsResourceName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:     "User",
				Name:     "coredns",
				APIGroup: "rbac.authorization.k8s.io",
			},
		},
	})
	err = rigging.ConvertError(err)
	if err != nil && !trace.IsAlreadyExists(err) {
		return trace.Wrap(err)
	}

	_, err = p.kubernetesOperation.Client.RbacV1().Roles(constants.KubeSystemNamespace).Create(&rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      corednsResourceName,
			Namespace: constants.KubeSystemNamespace,
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups:     []string{""},
				Verbs:         []string{"get", "watch", "list"},
				Resources:     []string{"configmaps"},
				ResourceNames: []string{"coredns"},
			},
		},
	})
	err = rigging.ConvertError(err)
	if err != nil && !trace.IsAlreadyExists(err) {
		return trace.Wrap(err)
	}

	_, err = p.kubernetesOperation.Client.RbacV1().RoleBindings(constants.KubeSystemNamespace).Create(&rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: corednsResourceName},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     corednsResourceName,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:     "User",
				Name:     "coredns",
				APIGroup: "rbac.authorization.k8s.io",
			},
		},
	})
	err = rigging.ConvertError(err)
	if err != nil && !trace.IsAlreadyExists(err) {
		return trace.Wrap(err)
	}

	return nil
}

// Rollback
func (p *updatePhaseCoreDNS) Rollback(context.Context) error {
	err := p.kubernetesOperation.Client.RbacV1().ClusterRoles().Delete(corednsResourceName, &metav1.DeleteOptions{})
	err = rigging.ConvertError(err)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}

	err = p.kubernetesOperation.Client.RbacV1().ClusterRoleBindings().Delete(corednsResourceName, &metav1.DeleteOptions{})
	err = rigging.ConvertError(err)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}

	err = p.kubernetesOperation.Client.RbacV1().Roles(constants.KubeSystemNamespace).Delete(corednsResourceName, &metav1.DeleteOptions{})
	err = rigging.ConvertError(err)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}

	err = p.kubernetesOperation.Client.RbacV1().RoleBindings(constants.KubeSystemNamespace).Delete(corednsResourceName, &metav1.DeleteOptions{})
	err = rigging.ConvertError(err)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}

	return nil
}

func shouldUpdateCoreDNS(client *kubernetes.Clientset) (bool, error) {
	_, err := client.RbacV1().ClusterRoles().Get(corednsResourceName, metav1.GetOptions{})
	err = rigging.ConvertError(err)
	if err != nil {
		if trace.IsNotFound(err) {
			return true, nil
		}
		return false, trace.Wrap(err)
	}

	_, err = client.RbacV1().ClusterRoleBindings().Get(corednsResourceName, metav1.GetOptions{})
	err = rigging.ConvertError(err)
	if err != nil {
		if trace.IsNotFound(err) {
			return true, nil
		}
		return false, trace.Wrap(err)
	}

	_, err = client.RbacV1().Roles(constants.KubeSystemNamespace).Get(corednsResourceName, metav1.GetOptions{})
	err = rigging.ConvertError(err)
	if err != nil {
		if trace.IsNotFound(err) {
			return true, nil
		}
		return false, trace.Wrap(err)
	}

	_, err = client.RbacV1().RoleBindings(constants.KubeSystemNamespace).Get(corednsResourceName, metav1.GetOptions{})
	err = rigging.ConvertError(err)
	if err != nil {
		if trace.IsNotFound(err) {
			return true, nil
		}
		return false, trace.Wrap(err)
	}

	return false, nil
}
