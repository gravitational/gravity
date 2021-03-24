// Copyright 2019 Gravitational Inc.
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

	"k8s.io/client-go/kubernetes"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewNamespaceControl returns new instance of Namespace updater
func NewNamespaceControl(
	config NamespaceConfig) (*NamespaceControl, error) {
	err := config.checkAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &NamespaceControl{
		NamespaceConfig: config,
		Entry: log.WithFields(log.Fields{
			"Namespace": formatMeta(config.ObjectMeta),
		}),
	}, nil
}

// NamespaceConfig  is a Namespace control configuration
type NamespaceConfig struct {
	// Namespace is already parsed daemon set, will be used if present
	*v1.Namespace
	// Client is k8s client
	Client *kubernetes.Clientset
}

func (c *NamespaceConfig) checkAndSetDefaults() error {
	if c.Namespace == nil {
		return trace.BadParameter("missing parameter Namespace")
	}
	if c.Client == nil {
		return trace.BadParameter("missing parameter Client")
	}
	updateTypeMetaNamespace(c.Namespace)
	return nil
}

// NamespaceControl is a daemon set controller,
// adds various operations, like delete, status check and update
type NamespaceControl struct {
	NamespaceConfig
	*log.Entry
}

func (c *NamespaceControl) Delete(ctx context.Context, cascade bool) error {
	c.Infof("delete %v", formatMeta(c.Namespace.ObjectMeta))

	err := c.Client.CoreV1().Namespaces().Delete(ctx, c.Namespace.Name, metav1.DeleteOptions{})
	return ConvertError(err)
}

func (c *NamespaceControl) Upsert(ctx context.Context) error {
	c.Infof("upsert %v", formatMeta(c.Namespace.ObjectMeta))

	Namespaces := c.Client.CoreV1().Namespaces()
	c.Namespace.UID = ""
	c.Namespace.SelfLink = ""
	c.Namespace.ResourceVersion = ""
	existing, err := Namespaces.Get(ctx, c.Namespace.Name, metav1.GetOptions{})
	err = ConvertError(err)
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		_, err = Namespaces.Create(ctx, c.Namespace, metav1.CreateOptions{})
		return ConvertError(err)
	}

	if checkCustomerManagedResource(existing.Annotations) {
		c.WithField("namespace", formatMeta(c.Namespace.ObjectMeta)).Info("Skipping update since object is customer managed.")
		return nil
	}

	_, err = Namespaces.Update(ctx, c.Namespace, metav1.UpdateOptions{})
	return ConvertError(err)
}

func (c *NamespaceControl) Status(ctx context.Context) error {
	Namespaces := c.Client.CoreV1().Namespaces()
	_, err := Namespaces.Get(ctx, c.Namespace.Name, metav1.GetOptions{})
	return ConvertError(err)
}

func updateTypeMetaNamespace(r *v1.Namespace) {
	r.Kind = KindNamespace
	if r.APIVersion == "" {
		r.APIVersion = v1.SchemeGroupVersion.String()
	}
}
