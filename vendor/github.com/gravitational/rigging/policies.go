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
	"k8s.io/api/extensions/v1beta1"
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
		PodSecurityPolicy:       config.Policy,
		Entry: log.WithFields(log.Fields{
			"pod_security_policy": formatMeta(config.Policy.ObjectMeta),
		}),
	}, nil
}

// PodSecurityPolicyConfig defines controller configuration
type PodSecurityPolicyConfig struct {
	// Policy is the existing pod security policy
	Policy v1beta1.PodSecurityPolicy
	// Client is k8s client
	Client *kubernetes.Clientset
}

func (c *PodSecurityPolicyConfig) CheckAndSetDefaults() error {
	if c.Client == nil {
		return trace.BadParameter("missing parameter Client")
	}
	c.Policy.Kind = KindPodSecurityPolicy
	c.Policy.APIVersion = ExtensionsAPIVersion
	return nil
}

// PodSecurityPolicyControl is a pod security policy controller,
// adds various operations, like delete, status check and update
type PodSecurityPolicyControl struct {
	PodSecurityPolicyConfig
	v1beta1.PodSecurityPolicy
	*log.Entry
}

func (c *PodSecurityPolicyControl) Delete(ctx context.Context, cascade bool) error {
	c.Infof("delete %v", formatMeta(c.ObjectMeta))

	err := c.Client.ExtensionsV1beta1().PodSecurityPolicies().Delete(c.Name, nil)
	return ConvertError(err)
}

func (c *PodSecurityPolicyControl) Upsert(ctx context.Context) error {
	c.Infof("upsert %v", formatMeta(c.ObjectMeta))

	policies := c.Client.ExtensionsV1beta1().PodSecurityPolicies()
	c.UID = ""
	c.SelfLink = ""
	c.ResourceVersion = ""
	_, err := policies.Get(c.Name, metav1.GetOptions{})
	err = ConvertError(err)
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		_, err = policies.Create(&c.Policy)
		return ConvertErrorWithContext(err, "cannot create pod security policy %q", formatMeta(c.ObjectMeta))
	}
	_, err = policies.Update(&c.Policy)
	return ConvertError(err)
}

func (c *PodSecurityPolicyControl) Status() error {
	policies := c.Client.ExtensionsV1beta1().PodSecurityPolicies()
	_, err := policies.Get(c.Name, metav1.GetOptions{})
	return ConvertError(err)
}
