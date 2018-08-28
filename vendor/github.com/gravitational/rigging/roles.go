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

	log "github.com/sirupsen/logrus"
	"github.com/gravitational/trace"
	"k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// NewRoleControl returns a new instance of the Role controller
func NewRoleControl(config RoleConfig) (*RoleControl, error) {
	err := config.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &RoleControl{
		RoleConfig: config,
		Role:       config.Role,
		Entry: log.WithFields(log.Fields{
			"role": formatMeta(config.Role.ObjectMeta),
		}),
	}, nil
}

// RoleConfig defines controller configuration
type RoleConfig struct {
	// Role is the existing role
	Role v1.Role
	// Client is k8s client
	Client *kubernetes.Clientset
}

func (c *RoleConfig) CheckAndSetDefaults() error {
	if c.Client == nil {
		return trace.BadParameter("missing parameter Client")
	}
	c.Role.Kind = KindRole
	c.Role.APIVersion = RBACAPIVersion
	return nil
}

// RoleControl is a roles controller,
// adds various operations, like delete, status check and update
type RoleControl struct {
	RoleConfig
	v1.Role
	*log.Entry
}

func (c *RoleControl) Delete(ctx context.Context, cascade bool) error {
	c.Infof("delete %v", formatMeta(c.ObjectMeta))

	err := c.Client.RbacV1().Roles(c.Namespace).Delete(c.Name, nil)
	return ConvertError(err)
}

func (c *RoleControl) Upsert(ctx context.Context) error {
	c.Infof("upsert %v", formatMeta(c.ObjectMeta))

	roles := c.Client.RbacV1().Roles(c.Namespace)
	c.UID = ""
	c.SelfLink = ""
	c.ResourceVersion = ""
	_, err := roles.Get(c.Name, metav1.GetOptions{})
	err = ConvertError(err)
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		_, err = roles.Create(&c.Role)
		return ConvertErrorWithContext(err, "cannot create role %q", formatMeta(c.ObjectMeta))
	}
	_, err = roles.Update(&c.Role)
	return ConvertError(err)
}

func (c *RoleControl) Status() error {
	roles := c.Client.RbacV1().Roles(c.Namespace)
	_, err := roles.Get(c.Name, metav1.GetOptions{})
	return ConvertError(err)
}

// NewClusterRoleControl returns a new instance of the ClusterRole controller
func NewClusterRoleControl(config ClusterRoleConfig) (*ClusterRoleControl, error) {
	err := config.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &ClusterRoleControl{
		ClusterRoleConfig: config,
		ClusterRole:       config.Role,
		Entry: log.WithFields(log.Fields{
			"cluster_role": formatMeta(config.Role.ObjectMeta),
		}),
	}, nil
}

// ClusterRoleConfig defines controller configuration
type ClusterRoleConfig struct {
	// Role is the existing cluster role
	Role v1.ClusterRole
	// Client is k8s client
	Client *kubernetes.Clientset
}

func (c *ClusterRoleConfig) CheckAndSetDefaults() error {
	if c.Client == nil {
		return trace.BadParameter("missing parameter Client")
	}
	c.Role.Kind = KindClusterRole
	c.Role.APIVersion = RBACAPIVersion
	return nil
}

// ClusterRoleControl is a cluster roles controller,
// adds various operations, like delete, status check and update
type ClusterRoleControl struct {
	ClusterRoleConfig
	v1.ClusterRole
	*log.Entry
}

func (c *ClusterRoleControl) Delete(ctx context.Context, cascade bool) error {
	c.Infof("delete %v", formatMeta(c.ObjectMeta))

	err := c.Client.RbacV1().ClusterRoles().Delete(c.Name, nil)
	return ConvertError(err)
}

func (c *ClusterRoleControl) Upsert(ctx context.Context) error {
	c.Infof("upsert %v", formatMeta(c.ObjectMeta))

	roles := c.Client.RbacV1().ClusterRoles()
	c.UID = ""
	c.SelfLink = ""
	c.ResourceVersion = ""
	_, err := roles.Get(c.Name, metav1.GetOptions{})
	err = ConvertError(err)
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		_, err = roles.Create(&c.Role)
		return ConvertErrorWithContext(err, "cannot create cluster role %q", formatMeta(c.ObjectMeta))
	}
	_, err = roles.Update(&c.Role)
	return ConvertError(err)
}

func (c *ClusterRoleControl) Status() error {
	roles := c.Client.RbacV1().ClusterRoles()
	_, err := roles.Get(c.Name, metav1.GetOptions{})
	return ConvertError(err)
}

// NewRoleBindingControl returns a new instance of the RoleBinding controller
func NewRoleBindingControl(config RoleBindingConfig) (*RoleBindingControl, error) {
	err := config.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &RoleBindingControl{
		RoleBindingConfig: config,
		RoleBinding:       config.Binding,
		Entry: log.WithFields(log.Fields{
			"role_binding": formatMeta(config.Binding.ObjectMeta),
		}),
	}, nil
}

// RoleBindingConfig defines controller configuration
type RoleBindingConfig struct {
	// RoleBinding is the existing role binding
	Binding v1.RoleBinding
	// Client is k8s client
	Client *kubernetes.Clientset
}

func (c *RoleBindingConfig) CheckAndSetDefaults() error {
	if c.Client == nil {
		return trace.BadParameter("missing parameter Client")
	}
	c.Binding.Kind = KindRoleBinding
	c.Binding.APIVersion = RBACAPIVersion
	return nil
}

// RoleBindingControl is a role bindings controller,
// adds various operations, like delete, status check and update
type RoleBindingControl struct {
	RoleBindingConfig
	v1.RoleBinding
	*log.Entry
}

func (c *RoleBindingControl) Delete(ctx context.Context, cascade bool) error {
	c.Infof("delete %v", formatMeta(c.ObjectMeta))

	err := c.Client.RbacV1().RoleBindings(c.Namespace).Delete(c.Name, nil)
	return ConvertError(err)
}

func (c *RoleBindingControl) Upsert(ctx context.Context) error {
	c.Infof("upsert %v", formatMeta(c.ObjectMeta))

	bindings := c.Client.RbacV1().RoleBindings(c.Namespace)
	c.UID = ""
	c.SelfLink = ""
	c.ResourceVersion = ""
	_, err := bindings.Get(c.Name, metav1.GetOptions{})
	err = ConvertError(err)
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		_, err = bindings.Create(&c.RoleBinding)
		return ConvertErrorWithContext(err, "cannot create role binding %q", formatMeta(c.ObjectMeta))
	}
	_, err = bindings.Update(&c.RoleBinding)
	return ConvertError(err)
}

func (c *RoleBindingControl) Status() error {
	bindings := c.Client.RbacV1().RoleBindings(c.Namespace)
	_, err := bindings.Get(c.Name, metav1.GetOptions{})
	return ConvertError(err)
}

// NewClusterRoleBindingControl returns a new instance of the ClusterRoleBinding controller
func NewClusterRoleBindingControl(config ClusterRoleBindingConfig) (*ClusterRoleBindingControl, error) {
	err := config.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &ClusterRoleBindingControl{
		ClusterRoleBindingConfig: config,
		ClusterRoleBinding:       config.Binding,
		Entry: log.WithFields(log.Fields{
			"cluster_role_binding": formatMeta(config.Binding.ObjectMeta),
		}),
	}, nil
}

// ClusterRoleBindingConfig defines controller configuration
type ClusterRoleBindingConfig struct {
	// Binding is the existing cluster role binding
	Binding v1.ClusterRoleBinding
	// Client is k8s client
	Client *kubernetes.Clientset
}

func (c *ClusterRoleBindingConfig) CheckAndSetDefaults() error {
	if c.Client == nil {
		return trace.BadParameter("missing parameter Client")
	}
	c.Binding.Kind = KindClusterRoleBinding
	c.Binding.APIVersion = RBACAPIVersion
	return nil
}

// ClusterRoleBindingControl is a cluster role bindings controller,
// adds various operations, like delete, status check and update
type ClusterRoleBindingControl struct {
	ClusterRoleBindingConfig
	v1.ClusterRoleBinding
	*log.Entry
}

func (c *ClusterRoleBindingControl) Delete(ctx context.Context, cascade bool) error {
	c.Infof("delete %v", formatMeta(c.ObjectMeta))

	err := c.Client.RbacV1().ClusterRoleBindings().Delete(c.Name, nil)
	return ConvertError(err)
}

func (c *ClusterRoleBindingControl) Upsert(ctx context.Context) error {
	c.Infof("upsert %v", formatMeta(c.ObjectMeta))

	bindings := c.Client.RbacV1().ClusterRoleBindings()
	c.UID = ""
	c.SelfLink = ""
	c.ResourceVersion = ""
	_, err := bindings.Get(c.Name, metav1.GetOptions{})
	err = ConvertError(err)
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		_, err = bindings.Create(&c.ClusterRoleBinding)
		return ConvertErrorWithContext(err, "cannot create cluster role binding %q", formatMeta(c.ObjectMeta))
	}
	_, err = bindings.Update(&c.ClusterRoleBinding)
	return ConvertError(err)
}

func (c *ClusterRoleBindingControl) Status() error {
	bindings := c.Client.RbacV1().ClusterRoleBindings()
	_, err := bindings.Get(c.Name, metav1.GetOptions{})
	return ConvertError(err)
}
