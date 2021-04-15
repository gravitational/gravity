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

package opsservice

import (
	"context"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/ops/events"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/rigging"
	"github.com/gravitational/trace"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// GetLogForwarders returns a list of configured log forwarders
func (o *Operator) GetLogForwarders(key ops.SiteKey) ([]storage.LogForwarder, error) {
	if o.cfg.LogForwarders == nil {
		return nil, trace.BadParameter(
			"this operator does not support log forwarders management")
	}

	forwarders, err := o.cfg.LogForwarders.Get()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return forwarders, nil
}

// UpdateForwarders replaces the list of active log forwarders
// TODO(r0mant,alexeyk) this is a legacy method used only by UI, alexeyk to remove it when
// refactoring resources and use upsert/delete instead
func (o *Operator) UpdateLogForwarders(key ops.SiteKey, forwarders []storage.LogForwarderV1) error {
	if o.cfg.LogForwarders == nil {
		return trace.BadParameter(
			"this operator does not support log forwarders management")
	}

	forwardersV2 := make([]storage.LogForwarder, len(forwarders))
	for i := range forwarders {
		forwardersV2[i] = storage.NewLogForwarderFromV1(forwarders[i])
	}

	err := o.cfg.LogForwarders.Replace(forwardersV2)
	if err != nil {
		return trace.Wrap(err)
	}

	err = o.cfg.LogForwarders.Reload()
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// CreateLogForwarder creates a new log forwarder
func (o *Operator) CreateLogForwarder(ctx context.Context, key ops.SiteKey, forwarder storage.LogForwarder) error {
	if o.cfg.LogForwarders == nil {
		return trace.BadParameter(
			"this operator does not support log forwarders management")
	}

	err := o.cfg.LogForwarders.Create(forwarder)
	if err != nil {
		return trace.Wrap(err)
	}

	events.Emit(ctx, o, events.LogForwarderCreated, events.Fields{
		events.FieldName: forwarder.GetName(),
	})

	err = o.cfg.LogForwarders.Reload()
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// UpdateLogForwarder updates an existing log forwarder
func (o *Operator) UpdateLogForwarder(ctx context.Context, key ops.SiteKey, forwarder storage.LogForwarder) error {
	if o.cfg.LogForwarders == nil {
		return trace.BadParameter(
			"this operator does not support log forwarders management")
	}

	err := o.cfg.LogForwarders.Update(forwarder)
	if err != nil {
		return trace.Wrap(err)
	}

	events.Emit(ctx, o, events.LogForwarderCreated, events.Fields{
		events.FieldName: forwarder.GetName(),
	})

	err = o.cfg.LogForwarders.Reload()
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// DeleteLogForwarder deletes a log forwarder
func (o *Operator) DeleteLogForwarder(ctx context.Context, key ops.SiteKey, name string) error {
	if o.cfg.LogForwarders == nil {
		return trace.BadParameter(
			"this operator does not support log forwarders management")
	}

	err := o.cfg.LogForwarders.Delete(name)
	if err != nil {
		return trace.Wrap(err)
	}

	events.Emit(ctx, o, events.LogForwarderDeleted, events.Fields{
		events.FieldName: name,
	})

	err = o.cfg.LogForwarders.Reload()
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// LogForwarderControl defines methods for managing log forwarders in Kubernetes,
// mostly so implementation can be substituted in tests w/o Kubernetes
type LogForwardersControl interface {
	// Get returns a list of log forwarders from config map
	Get() ([]storage.LogForwarder, error)
	// Replace replaces all log forwarders in config map with the provided ones
	Replace([]storage.LogForwarder) error
	// Create adds a new log forwarder in config map
	Create(storage.LogForwarder) error
	// Update updates an existing log forwarder in config map
	Update(storage.LogForwarder) error
	// Delete deletes log forwarder from config map
	Delete(string) error
	// Reload forces log collector to reload forwarder configuration
	Reload() error
}

type logForwardersControl struct {
	client *kubernetes.Clientset
}

// NewLogForwardersControl returns a default Kubernetes-managed log forwarder controller
func NewLogForwardersControl(client *kubernetes.Clientset) LogForwardersControl {
	return &logForwardersControl{client}
}

// Get returns a list of log forwarders from config map
func (c *logForwardersControl) Get() ([]storage.LogForwarder, error) {
	configMap, err := c.client.CoreV1().ConfigMaps(defaults.KubeSystemNamespace).
		Get(context.TODO(), defaults.LogForwardersConfigMap, metav1.GetOptions{})
	if err != nil {
		return nil, rigging.ConvertError(err)
	}

	var forwarders []storage.LogForwarder
	for _, data := range configMap.Data {
		forwarder, err := storage.GetLogForwarderMarshaler().Unmarshal([]byte(data))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		forwarders = append(forwarders, forwarder)
	}

	return forwarders, nil
}

// Replace replaces the contents of the forwarder config map with the provided list of forwarders
func (c *logForwardersControl) Replace(forwarders []storage.LogForwarder) error {
	configMap, err := c.client.CoreV1().ConfigMaps(defaults.KubeSystemNamespace).
		Get(context.TODO(), defaults.LogForwardersConfigMap, metav1.GetOptions{})
	if err != nil {
		return rigging.ConvertError(err)
	}

	configMap.Data = make(map[string]string)
	for _, forwarder := range forwarders {
		bytes, err := storage.GetLogForwarderMarshaler().Marshal(forwarder)
		if err != nil {
			return trace.Wrap(err)
		}
		configMap.Data[forwarder.GetName()] = string(bytes)
	}

	_, err = c.client.CoreV1().ConfigMaps(defaults.KubeSystemNamespace).
		Update(context.TODO(), configMap, metav1.UpdateOptions{})
	if err != nil {
		return rigging.ConvertError(err)
	}

	return nil
}

// Create adds a new log forwarder to the forwarders config map
func (c *logForwardersControl) Create(forwarder storage.LogForwarder) error {
	configMap, err := c.client.CoreV1().ConfigMaps(defaults.KubeSystemNamespace).
		Get(context.TODO(), defaults.LogForwardersConfigMap, metav1.GetOptions{})
	if err != nil {
		return rigging.ConvertError(err)
	}

	_, ok := configMap.Data[forwarder.GetName()]
	if ok {
		return trace.AlreadyExists("log forwarder %q already exists", forwarder.GetName())
	}

	bytes, err := storage.GetLogForwarderMarshaler().Marshal(forwarder)
	if err != nil {
		return trace.Wrap(err)
	}

	if configMap.Data == nil {
		configMap.Data = make(map[string]string)
	}

	configMap.Data[forwarder.GetName()] = string(bytes)

	_, err = c.client.CoreV1().ConfigMaps(defaults.KubeSystemNamespace).
		Update(context.TODO(), configMap, metav1.UpdateOptions{})
	if err != nil {
		return rigging.ConvertError(err)
	}

	return nil
}

// Update updates an existing log forwarder in the k8s config map
func (c *logForwardersControl) Update(forwarder storage.LogForwarder) error {
	configMap, err := c.client.CoreV1().ConfigMaps(defaults.KubeSystemNamespace).
		Get(context.TODO(), defaults.LogForwardersConfigMap, metav1.GetOptions{})
	if err != nil {
		return rigging.ConvertError(err)
	}

	_, ok := configMap.Data[forwarder.GetName()]
	if !ok {
		return trace.NotFound("log forwarder %q not found", forwarder.GetName())
	}

	bytes, err := storage.GetLogForwarderMarshaler().Marshal(forwarder)
	if err != nil {
		return trace.Wrap(err)
	}

	if configMap.Data == nil {
		configMap.Data = make(map[string]string)
	}

	configMap.Data[forwarder.GetName()] = string(bytes)

	_, err = c.client.CoreV1().ConfigMaps(defaults.KubeSystemNamespace).
		Update(context.TODO(), configMap, metav1.UpdateOptions{})
	if err != nil {
		return rigging.ConvertError(err)
	}

	return nil
}

// Delete deletes the specified log forwarder from the k8s config map
func (c *logForwardersControl) Delete(name string) error {
	configMap, err := c.client.CoreV1().ConfigMaps(defaults.KubeSystemNamespace).
		Get(context.TODO(), defaults.LogForwardersConfigMap, metav1.GetOptions{})
	if err != nil {
		return rigging.ConvertError(err)
	}

	_, ok := configMap.Data[name]
	if !ok {
		return trace.NotFound("log forwarder %q not found", name)
	}

	delete(configMap.Data, name)

	_, err = c.client.CoreV1().ConfigMaps(defaults.KubeSystemNamespace).
		Update(context.TODO(), configMap, metav1.UpdateOptions{})
	if err != nil {
		return rigging.ConvertError(err)
	}

	return nil
}

// Reload forces log collector to reload forwarder configuration
func (c *logForwardersControl) Reload() error {
	err := c.client.CoreV1().Pods(defaults.KubeSystemNamespace).
		DeleteCollection(context.TODO(),
			metav1.DeleteOptions{},
			metav1.ListOptions{
				LabelSelector: utils.MakeSelector(map[string]string{
					"role": "log-collector",
				}).String(),
			})
	if err != nil {
		return rigging.ConvertError(err)
	}

	return nil
}
