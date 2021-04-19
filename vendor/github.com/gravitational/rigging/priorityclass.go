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
	"k8s.io/api/scheduling/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewPriorityClassControl returns new instance of PriorityClass updater
func NewPriorityClassControl(
	config PriorityClassConfig) (*PriorityClassControl, error) {
	err := config.checkAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &PriorityClassControl{
		PriorityClassConfig: config,
		Entry: log.WithFields(log.Fields{
			"PriorityClass": formatMeta(config.ObjectMeta),
		}),
	}, nil
}

// PriorityClassConfig  is a PriorityClass control configuration
type PriorityClassConfig struct {
	// PriorityClass is already parsed daemon set, will be used if present
	*v1beta1.PriorityClass
	// Client is k8s client
	Client *kubernetes.Clientset
}

func (c *PriorityClassConfig) checkAndSetDefaults() error {
	if c.PriorityClass == nil {
		return trace.BadParameter("missing parameter PriorityClass")
	}
	if c.Client == nil {
		return trace.BadParameter("missing parameter Client")
	}
	updateTypeMetaPriorityClass(c.PriorityClass)
	return nil
}

// PriorityClassControl is a daemon set controller,
// adds various operations, like delete, status check and update
type PriorityClassControl struct {
	PriorityClassConfig
	*log.Entry
}

func (c *PriorityClassControl) Delete(ctx context.Context, cascade bool) error {
	c.Infof("delete %v", formatMeta(c.PriorityClass.ObjectMeta))

	err := c.Client.SchedulingV1().PriorityClasses().Delete(ctx, c.PriorityClass.Name, metav1.DeleteOptions{})
	return ConvertError(err)
}

func (c *PriorityClassControl) Upsert(ctx context.Context) error {
	c.Infof("upsert %v", formatMeta(c.PriorityClass.ObjectMeta))

	PriorityClasss := c.Client.SchedulingV1beta1().PriorityClasses()
	c.PriorityClass.UID = ""
	c.PriorityClass.SelfLink = ""
	c.PriorityClass.ResourceVersion = ""
	existing, err := PriorityClasss.Get(ctx, c.PriorityClass.Name, metav1.GetOptions{})
	err = ConvertError(err)
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		_, err = PriorityClasss.Create(ctx, c.PriorityClass, metav1.CreateOptions{})
		return ConvertError(err)
	}

	if checkCustomerManagedResource(existing.Annotations) {
		c.WithField("priorityclass", formatMeta(c.ObjectMeta)).Info("Skipping update since object is customer managed.")
		return nil
	}

	_, err = PriorityClasss.Update(ctx, c.PriorityClass, metav1.UpdateOptions{})
	return ConvertError(err)
}

func (c *PriorityClassControl) Status(ctx context.Context) error {
	PriorityClasss := c.Client.SchedulingV1beta1().PriorityClasses()
	_, err := PriorityClasss.Get(ctx, c.PriorityClass.Name, metav1.GetOptions{})
	return ConvertError(err)
}

func updateTypeMetaPriorityClass(r *v1beta1.PriorityClass) {
	r.Kind = KindPriorityClass
	if r.APIVersion == "" {
		r.APIVersion = v1.SchemeGroupVersion.String()
	}
}
