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

package fsm

import (
	"context"

	"github.com/gravitational/gravity/lib/app/resources"

	"github.com/gravitational/rigging"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
)

// GetUpsertBootstrapResourceFunc returns a function that takes a Kubernetes
// object representing a bootstrap resource (ClusterRole, ClusterRoleBinding
// or PodSecurityPolicy) and creates or updates it using the provided client
func GetUpsertBootstrapResourceFunc(client *kubernetes.Clientset) resources.ResourceFunc {
	return func(object runtime.Object) (err error) {
		switch resource := object.(type) {
		case *rbacv1.ClusterRole:
			_, err = client.RbacV1().ClusterRoles().Create(context.TODO(), resource, metav1.CreateOptions{})
			if err == nil {
				log.Debugf("Created ClusterRole %q.", resource.Name)
				return nil
			}
			if !trace.IsAlreadyExists(rigging.ConvertError(err)) {
				return trace.Wrap(rigging.ConvertError(err))
			}
			_, err = client.RbacV1().ClusterRoles().Update(context.TODO(), resource, metav1.UpdateOptions{})
			if err != nil {
				return trace.Wrap(rigging.ConvertError(err))
			}
			log.Debugf("Updated ClusterRole %q.", resource.Name)
		case *rbacv1.ClusterRoleBinding:
			_, err = client.RbacV1().ClusterRoleBindings().Create(context.TODO(), resource, metav1.CreateOptions{})
			if err == nil {
				log.Debugf("Created ClusterRoleBinding %q.", resource.Name)
				return nil
			}
			if !trace.IsAlreadyExists(rigging.ConvertError(err)) {
				return trace.Wrap(rigging.ConvertError(err))
			}
			_, err = client.RbacV1().ClusterRoleBindings().Update(context.TODO(), resource, metav1.UpdateOptions{})
			if err != nil {
				return trace.Wrap(rigging.ConvertError(err))
			}
			log.Debugf("Updated ClusterRoleBinding %q.", resource.Name)
		case *rbacv1.Role:
			_, err = client.RbacV1().Roles(resource.Namespace).Create(context.TODO(), resource, metav1.CreateOptions{})
			if err == nil {
				log.Debugf("Created Role %q.", resource.Name)
				return nil
			}
			if !trace.IsAlreadyExists(rigging.ConvertError(err)) {
				return trace.Wrap(rigging.ConvertError(err))
			}
			_, err = client.RbacV1().Roles(resource.Namespace).Update(context.TODO(), resource, metav1.UpdateOptions{})
			if err != nil {
				return trace.Wrap(rigging.ConvertError(err))
			}
			log.Debugf("Updated Role %q.", resource.Name)
		case *rbacv1.RoleBinding:
			_, err = client.RbacV1().RoleBindings(resource.Namespace).Create(context.TODO(), resource, metav1.CreateOptions{})
			if err == nil {
				log.Debugf("Created RoleBinding %q.", resource.Name)
				return nil
			}
			if !trace.IsAlreadyExists(rigging.ConvertError(err)) {
				return trace.Wrap(rigging.ConvertError(err))
			}
			_, err = client.RbacV1().RoleBindings(resource.Namespace).Update(context.TODO(), resource, metav1.UpdateOptions{})
			if err != nil {
				return trace.Wrap(rigging.ConvertError(err))
			}
			log.Debugf("Updated RoleBinding %q.", resource.Name)
		case *policyv1beta1.PodSecurityPolicy:
			_, err = client.PolicyV1beta1().PodSecurityPolicies().Create(context.TODO(), resource, metav1.CreateOptions{})
			if err == nil {
				log.Debugf("Created PodSecurityPolicy %q.", resource.Name)
				return nil
			}
			if !trace.IsAlreadyExists(rigging.ConvertError(err)) {
				return trace.Wrap(rigging.ConvertError(err))
			}
			_, err = client.PolicyV1beta1().PodSecurityPolicies().Update(context.TODO(), resource, metav1.UpdateOptions{})
			if err != nil {
				return trace.Wrap(rigging.ConvertError(err))
			}
			log.Debugf("Updated PodSecurityPolicy %q.", resource.Name)
		default:
			log.Warnf("Unsupported bootstrap resource: %#v.", resource)
			return trace.BadParameter("unsupported bootstrap resource: %#v.", resource.GetObjectKind().GroupVersionKind())
		}
		return nil
	}
}
