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
	"time"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/ops/events"
	"github.com/gravitational/gravity/lib/ops/monitoring"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/rigging"
	"github.com/gravitational/trace"
	monitoringv1 "github.com/prometheus/client_golang/api/prometheus/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubelabels "k8s.io/apimachinery/pkg/labels"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

// GetClusterMetrics returns basic CPU/RAM metrics for the specified cluster.
func (o *Operator) GetClusterMetrics(ctx context.Context, req ops.ClusterMetricsRequest) (*ops.ClusterMetricsResponse, error) {
	return GetClusterMetrics(ctx, o.cfg.Metrics, req)
}

// GetClusterMetrics retrieves all cluster metrics from the provided client.
func GetClusterMetrics(ctx context.Context, metrics monitoring.Metrics, req ops.ClusterMetricsRequest) (*ops.ClusterMetricsResponse, error) {
	err := req.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	totalCPUCores, err := metrics.GetTotalCPU(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	currentCPURate, err := metrics.GetCurrentCPURate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	maxCPURate, err := metrics.GetMaxCPURate(ctx, req.Interval)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	historicCPURate, err := metrics.GetCPURate(ctx, monitoringv1.Range{
		Start: time.Now().Add(-req.Interval),
		End:   time.Now(),
		Step:  req.Step,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	totalRAMBytes, err := metrics.GetTotalMemory(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	currentRAMRate, err := metrics.GetCurrentMemoryRate(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	maxRAMRate, err := metrics.GetMaxMemoryRate(ctx, req.Interval)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	historicRAMRate, err := metrics.GetMemoryRate(ctx, monitoringv1.Range{
		Start: time.Now().Add(-req.Interval),
		End:   time.Now(),
		Step:  req.Step,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &ops.ClusterMetricsResponse{
		TotalCPUCores:    totalCPUCores,
		TotalMemoryBytes: totalRAMBytes,
		CPURates: ops.ClusterMetricsRates{
			Current:  currentCPURate,
			Max:      maxCPURate,
			Historic: historicCPURate,
		},
		MemoryRates: ops.ClusterMetricsRates{
			Current:  currentRAMRate,
			Max:      maxRAMRate,
			Historic: historicRAMRate,
		},
	}, nil
}

// GetAlerts returns a list of configured monitoring alerts
func (o *Operator) GetAlerts(key ops.SiteKey) (alerts []storage.Alert, err error) {
	client, err := o.GetKubeClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	labels := kubelabels.Set{
		constants.MonitoringType: constants.MonitoringTypeAlert,
	}
	options := metav1.ListOptions{
		LabelSelector: labels.String(),
	}
	configmaps, err := client.CoreV1().ConfigMaps(defaults.MonitoringNamespace).
		List(context.TODO(), options)
	if err != nil {
		return nil, trace.Wrap(rigging.ConvertError(err))
	}

	var errors []error
	alerts = make([]storage.Alert, 0, len(configmaps.Items))
	for _, config := range configmaps.Items {
		data, ok := config.Data[constants.ResourceSpecKey]
		if !ok {
			continue
		}
		alert, err := storage.UnmarshalAlert([]byte(data))
		if err != nil {
			errors = append(errors, err)
			continue
		}
		alerts = append(alerts, alert)
	}

	if len(errors) != 0 {
		return nil, trace.NewAggregate(errors...)
	}

	return alerts, nil
}

// UpdateAlert updates the specified monitoring alert
func (o *Operator) UpdateAlert(ctx context.Context, key ops.SiteKey, alert storage.Alert) error {
	client, err := o.GetKubeClient()
	if err != nil {
		return trace.Wrap(err)
	}

	data, err := storage.MarshalAlert(alert)
	if err != nil {
		return trace.Wrap(err)
	}

	labels := map[string]string{
		constants.MonitoringType: constants.MonitoringTypeAlert,
	}
	err = updateConfigMap(client.CoreV1().ConfigMaps(defaults.MonitoringNamespace),
		alert.GetName(), defaults.MonitoringNamespace, string(data), labels)
	if err != nil {
		return trace.Wrap(err)
	}

	events.Emit(ctx, o, events.AlertCreated, events.Fields{
		events.FieldName: alert.GetName(),
	})
	return nil
}

// DeleteAlert deletes the specified monitoring alert
func (o *Operator) DeleteAlert(ctx context.Context, key ops.SiteKey, name string) error {
	client, err := o.GetKubeClient()
	if err != nil {
		return trace.Wrap(err)
	}

	labels := kubelabels.Set{
		constants.MonitoringType: constants.MonitoringTypeAlert,
	}
	options := metav1.ListOptions{
		LabelSelector: labels.String(),
	}
	configmaps, err := client.CoreV1().ConfigMaps(defaults.MonitoringNamespace).
		List(ctx, options)
	if err != nil {
		return trace.Wrap(rigging.ConvertError(err))
	}

	var alert *v1.ConfigMap
	for _, config := range configmaps.Items {
		if config.Name == name {
			alert = &config
			break
		}
	}
	if alert == nil {
		return trace.NotFound("alert %q not found", name)
	}

	err = client.CoreV1().ConfigMaps(defaults.MonitoringNamespace).
		Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		return trace.Wrap(rigging.ConvertError(err))
	}

	events.Emit(ctx, o, events.AlertDeleted, events.Fields{
		events.FieldName: name,
	})
	return nil

}

// GetAlertTargets returns a list of configured monitoring alert targets
func (o *Operator) GetAlertTargets(key ops.SiteKey) (targets []storage.AlertTarget, err error) {
	client, err := o.GetKubeClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	data, err := getConfigMap(client.CoreV1().ConfigMaps(defaults.MonitoringNamespace),
		constants.AlertTargetConfigMap)
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("alert target not found")
		}
		return nil, trace.Wrap(err)
	}

	target, err := storage.UnmarshalAlertTarget([]byte(data))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return []storage.AlertTarget{target}, nil
}

// UpdateAlertTarget updates the cluster monitoring alert target
func (o *Operator) UpdateAlertTarget(ctx context.Context, key ops.SiteKey, target storage.AlertTarget) error {
	client, err := o.GetKubeClient()
	if err != nil {
		return trace.Wrap(err)
	}

	data, err := storage.MarshalAlertTarget(target)
	if err != nil {
		return trace.Wrap(err)
	}

	labels := map[string]string{
		constants.MonitoringType: constants.MonitoringTypeAlertTarget,
	}
	err = updateConfigMap(client.CoreV1().ConfigMaps(defaults.MonitoringNamespace),
		constants.AlertTargetConfigMap, defaults.MonitoringNamespace, string(data), labels)
	if err != nil {
		return trace.Wrap(err)
	}

	events.Emit(ctx, o, events.AlertTargetCreated)
	return nil

}

// DeleteAlertTarget deletes the cluster monitoring alert target
func (o *Operator) DeleteAlertTarget(ctx context.Context, key ops.SiteKey) error {
	client, err := o.GetKubeClient()
	if err != nil {
		return trace.Wrap(err)
	}

	err = rigging.ConvertError(client.CoreV1().ConfigMaps(defaults.MonitoringNamespace).
		Delete(ctx, constants.AlertTargetConfigMap, metav1.DeleteOptions{}))
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("no alert targets found")
		}
		return trace.Wrap(err)
	}

	events.Emit(ctx, o, events.AlertTargetDeleted)
	return nil
}

func getConfigMap(client corev1.ConfigMapInterface, name string) (string, error) {
	config, err := client.Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return "", trace.Wrap(rigging.ConvertError(err))
	}

	data, ok := config.Data[constants.ResourceSpecKey]
	if !ok {
		return "", trace.NotFound("no resource found")
	}

	return data, nil
}

func updateConfigMap(client corev1.ConfigMapInterface, name, namespace, data string, labels map[string]string) error {
	config := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Data: map[string]string{
			constants.ResourceSpecKey: data,
		},
	}

	_, err := client.Create(context.TODO(), config, metav1.CreateOptions{})
	err = rigging.ConvertError(err)
	if err == nil {
		return nil
	}

	if !trace.IsAlreadyExists(err) {
		return trace.Wrap(err)
	}

	_, err = client.Update(context.TODO(), config, metav1.UpdateOptions{})
	return trace.Wrap(rigging.ConvertError(err))
}
