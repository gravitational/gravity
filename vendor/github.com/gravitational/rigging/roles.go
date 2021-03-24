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
	"context"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// NewRoleControl returns a new instance of the Role controller
func NewRoleControl(config RoleConfig) (*RoleControl, error) {
	err := config.checkAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &RoleControl{
		RoleConfig: config,
		FieldLogger: log.WithFields(log.Fields{
			"role": formatMeta(config.Role.ObjectMeta),
		}),
	}, nil
}

// RoleConfig defines controller configuration
type RoleConfig struct {
	// Role is the existing role
	*v1.Role
	// Client is k8s client
	Client *kubernetes.Clientset
}

func (c *RoleConfig) checkAndSetDefaults() error {
	if c.Role == nil {
		return trace.BadParameter("missing parameter Role")
	}
	if c.Client == nil {
		return trace.BadParameter("missing parameter Client")
	}
	updateTypeMetaRole(c.Role)
	return nil
}

// RoleControl is a roles controller,
// adds various operations, like delete, status check and update
type RoleControl struct {
	RoleConfig
	log.FieldLogger
}

func (c *RoleControl) Delete(ctx context.Context, cascade bool) error {
	c.Infof("delete %v", formatMeta(c.ObjectMeta))

	err := c.Client.RbacV1().Roles(c.Namespace).Delete(ctx, c.Name, metav1.DeleteOptions{})
	return ConvertError(err)
}

func (c *RoleControl) Upsert(ctx context.Context) error {
	c.Infof("upsert %v", formatMeta(c.ObjectMeta))

	roles := c.Client.RbacV1().Roles(c.Namespace)
	c.UID = ""
	c.SelfLink = ""
	c.ResourceVersion = ""
	existing, err := roles.Get(ctx, c.Name, metav1.GetOptions{})
	err = ConvertError(err)
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		_, err = roles.Create(ctx, c.Role, metav1.CreateOptions{})
		return ConvertErrorWithContext(err, "cannot create role %q", formatMeta(c.ObjectMeta))
	}

	if checkCustomerManagedResource(existing.Annotations) {
		c.WithField("role", formatMeta(c.ObjectMeta)).Info("Skipping update since object is customer managed.")
		return nil
	}

	_, err = roles.Update(ctx, c.Role, metav1.UpdateOptions{})
	return ConvertError(err)
}

func (c *RoleControl) Status(ctx context.Context) error {
	roles := c.Client.RbacV1().Roles(c.Namespace)
	_, err := roles.Get(ctx, c.Name, metav1.GetOptions{})
	return ConvertError(err)
}

// NewClusterRoleControl returns a new instance of the ClusterRole controller
func NewClusterRoleControl(config ClusterRoleConfig) (*ClusterRoleControl, error) {
	err := config.checkAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &ClusterRoleControl{
		ClusterRoleConfig: config,
		FieldLogger: log.WithFields(log.Fields{
			"cluster_role": formatMeta(config.ObjectMeta),
		}),
	}, nil
}

// ClusterRoleConfig defines controller configuration
type ClusterRoleConfig struct {
	// Role is the existing cluster role
	*v1.ClusterRole
	// Client is k8s client
	Client *kubernetes.Clientset
}

func (c *ClusterRoleConfig) checkAndSetDefaults() error {
	if c.ClusterRole == nil {
		return trace.BadParameter("missing parameter ClusterRole")
	}
	if c.Client == nil {
		return trace.BadParameter("missing parameter Client")
	}
	updateTypeMetaClusterRole(c.ClusterRole)
	return nil
}

// ClusterRoleControl is a cluster roles controller,
// adds various operations, like delete, status check and update
type ClusterRoleControl struct {
	ClusterRoleConfig
	log.FieldLogger
}

func (c *ClusterRoleControl) Delete(ctx context.Context, cascade bool) error {
	c.Infof("delete %v", formatMeta(c.ObjectMeta))

	err := c.Client.RbacV1().ClusterRoles().Delete(ctx, c.Name, metav1.DeleteOptions{})
	return ConvertError(err)
}

func (c *ClusterRoleControl) Upsert(ctx context.Context) error {
	c.Infof("upsert %v", formatMeta(c.ObjectMeta))

	roles := c.Client.RbacV1().ClusterRoles()
	c.UID = ""
	c.SelfLink = ""
	c.ResourceVersion = ""
	existing, err := roles.Get(ctx, c.Name, metav1.GetOptions{})
	err = ConvertError(err)
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		_, err = roles.Create(ctx, c.ClusterRole, metav1.CreateOptions{})
		return ConvertErrorWithContext(err, "cannot create cluster role %q", formatMeta(c.ObjectMeta))
	}

	if checkCustomerManagedResource(existing.Annotations) {
		c.WithField("clusterrole", formatMeta(c.ObjectMeta)).Info("Skipping update since object is customer managed.")
		return nil
	}

	_, err = roles.Update(ctx, c.ClusterRole, metav1.UpdateOptions{})
	return ConvertError(err)
}

func (c *ClusterRoleControl) Status(ctx context.Context) error {
	roles := c.Client.RbacV1().ClusterRoles()
	_, err := roles.Get(ctx, c.Name, metav1.GetOptions{})
	return ConvertError(err)
}

// NewRoleBindingControl returns a new instance of the RoleBinding controller
func NewRoleBindingControl(config RoleBindingConfig) (*RoleBindingControl, error) {
	err := config.checkAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &RoleBindingControl{
		RoleBindingConfig: config,
		FieldLogger: log.WithFields(log.Fields{
			"role_binding": formatMeta(config.ObjectMeta),
		}),
	}, nil
}

// RoleBindingConfig defines controller configuration
type RoleBindingConfig struct {
	// RoleBinding is the existing role binding
	*v1.RoleBinding
	// Client is k8s client
	Client *kubernetes.Clientset
}

func (c *RoleBindingConfig) checkAndSetDefaults() error {
	if c.RoleBinding == nil {
		return trace.BadParameter("missing parameter RoleBinding")
	}
	if c.Client == nil {
		return trace.BadParameter("missing parameter Client")
	}
	updateTypeMetaRoleBinding(c.RoleBinding)
	return nil
}

// RoleBindingControl is a role bindings controller,
// adds various operations, like delete, status check and update
type RoleBindingControl struct {
	RoleBindingConfig
	log.FieldLogger
}

func (c *RoleBindingControl) Delete(ctx context.Context, cascade bool) error {
	c.Infof("delete %v", formatMeta(c.ObjectMeta))

	err := c.Client.RbacV1().RoleBindings(c.Namespace).Delete(ctx, c.Name, metav1.DeleteOptions{})
	return ConvertError(err)
}

func (c *RoleBindingControl) Upsert(ctx context.Context) error {
	c.Infof("upsert %v", formatMeta(c.ObjectMeta))

	bindings := c.Client.RbacV1().RoleBindings(c.Namespace)
	c.UID = ""
	c.SelfLink = ""
	c.ResourceVersion = ""
	existing, err := bindings.Get(ctx, c.Name, metav1.GetOptions{})
	err = ConvertError(err)
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		_, err = bindings.Create(ctx, c.RoleBinding, metav1.CreateOptions{})
		return ConvertErrorWithContext(err, "cannot create role binding %q", formatMeta(c.ObjectMeta))
	}

	if checkCustomerManagedResource(existing.Annotations) {
		c.WithField("rolebinding", formatMeta(c.ObjectMeta)).Info("Skipping update since object is customer managed.")
		return nil
	}

	_, err = bindings.Update(ctx, c.RoleBinding, metav1.UpdateOptions{})
	return ConvertError(err)
}

func (c *RoleBindingControl) Status(ctx context.Context) error {
	bindings := c.Client.RbacV1().RoleBindings(c.Namespace)
	_, err := bindings.Get(ctx, c.Name, metav1.GetOptions{})
	return ConvertError(err)
}

// NewClusterRoleBindingControl returns a new instance of the ClusterRoleBinding controller
func NewClusterRoleBindingControl(config ClusterRoleBindingConfig) (*ClusterRoleBindingControl, error) {
	err := config.checkAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &ClusterRoleBindingControl{
		ClusterRoleBindingConfig: config,
		FieldLogger: log.WithFields(log.Fields{
			"cluster_role_binding": formatMeta(config.ObjectMeta),
		}),
	}, nil
}

// ClusterRoleBindingConfig defines controller configuration
type ClusterRoleBindingConfig struct {
	// Binding is the existing cluster role binding
	*v1.ClusterRoleBinding
	// Client is k8s client
	Client *kubernetes.Clientset
}

func (c *ClusterRoleBindingConfig) checkAndSetDefaults() error {
	if c.ClusterRoleBinding == nil {
		return trace.BadParameter("missing parameter ClusterRoleBinding")
	}
	if c.Client == nil {
		return trace.BadParameter("missing parameter Client")
	}
	updateTypeMetaClusterRoleBinding(c.ClusterRoleBinding)
	return nil
}

// ClusterRoleBindingControl is a cluster role bindings controller,
// adds various operations, like delete, status check and update
type ClusterRoleBindingControl struct {
	ClusterRoleBindingConfig
	log.FieldLogger
}

func (c *ClusterRoleBindingControl) Delete(ctx context.Context, cascade bool) error {
	c.Infof("delete %v", formatMeta(c.ObjectMeta))

	err := c.Client.RbacV1().ClusterRoleBindings().Delete(ctx, c.Name, metav1.DeleteOptions{})
	return ConvertError(err)
}

func (c *ClusterRoleBindingControl) Upsert(ctx context.Context) error {
	c.Infof("upsert %v", formatMeta(c.ObjectMeta))

	bindings := c.Client.RbacV1().ClusterRoleBindings()
	c.UID = ""
	c.SelfLink = ""
	c.ResourceVersion = ""
	existing, err := bindings.Get(ctx, c.Name, metav1.GetOptions{})
	err = ConvertError(err)
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		_, err = bindings.Create(ctx, c.ClusterRoleBinding, metav1.CreateOptions{})
		return ConvertErrorWithContext(err, "cannot create cluster role binding %q", formatMeta(c.ObjectMeta))
	}

	if checkCustomerManagedResource(existing.Annotations) {
		c.WithField("clusterrolebinding", formatMeta(c.ObjectMeta)).Info("Skipping update since object is customer managed.")
		return nil
	}

	_, err = bindings.Update(ctx, c.ClusterRoleBinding, metav1.UpdateOptions{})
	return ConvertError(err)
}

func (c *ClusterRoleBindingControl) Status(ctx context.Context) error {
	bindings := c.Client.RbacV1().ClusterRoleBindings()
	_, err := bindings.Get(ctx, c.Name, metav1.GetOptions{})
	return ConvertError(err)
}

func updateTypeMetaRole(r *v1.Role) {
	r.Kind = KindRole
	if r.APIVersion == "" {
		r.APIVersion = v1.SchemeGroupVersion.String()
	}
}

func updateTypeMetaRoleBinding(r *v1.RoleBinding) {
	r.Kind = KindRoleBinding
	if r.APIVersion == "" {
		r.APIVersion = v1.SchemeGroupVersion.String()
	}
}

func updateTypeMetaClusterRole(r *v1.ClusterRole) {
	r.Kind = KindClusterRole
	if r.APIVersion == "" {
		r.APIVersion = v1.SchemeGroupVersion.String()
	}
}

func updateTypeMetaClusterRoleBinding(r *v1.ClusterRoleBinding) {
	r.Kind = KindClusterRoleBinding
	if r.APIVersion == "" {
		r.APIVersion = v1.SchemeGroupVersion.String()
	}
}
