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
	"k8s.io/api/policy/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// NewPodSecurityPolicyControl returns a new instance of the PodSecurityPolicy controller
func NewPodSecurityPolicyControl(config PodSecurityPolicyConfig) (*PodSecurityPolicyControl, error) {
	err := config.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &PodSecurityPolicyControl{
		PodSecurityPolicyConfig: config,
		FieldLogger: log.WithFields(log.Fields{
			"podsecuritypolicy": formatMeta(config.ObjectMeta),
		}),
	}, nil
}

// PodSecurityPolicyConfig defines controller configuration
type PodSecurityPolicyConfig struct {
	// PodSecurityPolicy is the existing pod security policy
	*v1beta1.PodSecurityPolicy
	// Client is k8s client
	Client *kubernetes.Clientset
}

func (c *PodSecurityPolicyConfig) CheckAndSetDefaults() error {
	if c.PodSecurityPolicy == nil {
		return trace.BadParameter("missing parameter PodSecurityPolicy")
	}
	if c.Client == nil {
		return trace.BadParameter("missing parameter Client")
	}
	updateTypeMetaPodSecurityPolicy(c.PodSecurityPolicy)
	return nil
}

// PodSecurityPolicyControl is a pod security policy controller,
// adds various operations, like delete, status check and update
type PodSecurityPolicyControl struct {
	PodSecurityPolicyConfig
	log.FieldLogger
}

func (c *PodSecurityPolicyControl) Delete(ctx context.Context, cascade bool) error {
	c.Infof("delete %v", formatMeta(c.ObjectMeta))

	err := c.Client.PolicyV1beta1().PodSecurityPolicies().Delete(ctx, c.Name, metav1.DeleteOptions{})
	return ConvertError(err)
}

func (c *PodSecurityPolicyControl) Upsert(ctx context.Context) error {
	c.Infof("upsert %v", formatMeta(c.ObjectMeta))

	policies := c.Client.PolicyV1beta1().PodSecurityPolicies()
	c.UID = ""
	c.SelfLink = ""
	c.ResourceVersion = ""
	existing, err := policies.Get(ctx, c.Name, metav1.GetOptions{})
	err = ConvertError(err)
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		_, err = policies.Create(ctx, c.PodSecurityPolicy, metav1.CreateOptions{})
		return ConvertErrorWithContext(err, "cannot create pod security policy %q", formatMeta(c.ObjectMeta))
	}

	if checkCustomerManagedResource(existing.Annotations) {
		c.WithField("psp", formatMeta(c.ObjectMeta)).Info("Skipping update since object is customer managed.")
		return nil
	}

	_, err = policies.Update(ctx, c.PodSecurityPolicy, metav1.UpdateOptions{})
	return ConvertError(err)
}

func (c *PodSecurityPolicyControl) Status(ctx context.Context) error {
	policies := c.Client.PolicyV1beta1().PodSecurityPolicies()
	_, err := policies.Get(ctx, c.Name, metav1.GetOptions{})
	return ConvertError(err)
}

func updateTypeMetaPodSecurityPolicy(r *v1beta1.PodSecurityPolicy) {
	r.Kind = KindPodSecurityPolicy
	if r.APIVersion == "" {
		r.APIVersion = v1beta1.SchemeGroupVersion.String()
	}
}
