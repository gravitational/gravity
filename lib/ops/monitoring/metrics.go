/*
Copyright 2019 Gravitational, Inc.

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

package monitoring

import (
	"bytes"
	"context"
	"fmt"
	"text/template"
	"time"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/trace"
)

// Metrics defines an interface for cluster metrics.
type Metrics interface {
	// GetTotalCPU returns total number of CPU cores in the cluster.
	GetTotalCPU(context.Context) (int, error)
	// GetTotalMemory returns total amount of RAM in the cluster in bytes.
	GetTotalMemory(context.Context) (int64, error)
	// GetCPURate returns CPU usage rate for the specified time range.
	GetCPURate(ctx context.Context, timeRange v1.Range) (Series, error)
	// GetMemoryRate returns RAM usage rate for the specified time range.
	GetMemoryRate(ctx context.Context, timeRange v1.Range) (Series, error)
	// GetCurrentCPURate returns instantaneous CPU usage rate.
	GetCurrentCPURate(context.Context) (int, error)
	// GetCurrentMemoryRate returns instantaneous RAM usage rate.
	GetCurrentMemoryRate(context.Context) (int, error)
	// GetMaxCPURate returns highest CPU usage rate on the specified interval.
	GetMaxCPURate(ctx context.Context, interval time.Duration) (int, error)
	// GetMaxMemoryRate returns highest RAM usage rate on the specified interval.
	GetMaxMemoryRate(ctx context.Context, interval time.Duration) (int, error)
}

// Series represents a time series, collection of data points.
type Series []Point

// Point represents a single data point in a time series.
type Point struct {
	// Time is the metric timestamp.
	Time time.Time `json:"time"`
	// Value is the metric value.
	Value int `json:"value"`
}

// prometheus retrieves cluster metrics by querying in-cluster Prometheus.
//
// Implements Metrics interface.
type prometheus struct {
	// API is Prometheus API client.
	v1.API
}

// NewInClusterPrometheus returns in-cluster Prometheus client.
func NewInClusterPrometheus() (*prometheus, error) {
	return NewPrometheus(defaults.PrometheusServiceAddr)
}

// NewPrometheus returns a new Prometheus-backed metrics collector.
func NewPrometheus(address string) (*prometheus, error) {
	client, err := api.NewClient(api.Config{
		Address: fmt.Sprintf("http://%v", address),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &prometheus{
		API: v1.NewAPI(client),
	}, nil
}

// GetTotalCPU returns total number of CPU cores in the cluster.
func (p *prometheus) GetTotalCPU(ctx context.Context) (int, error) {
	vector, err := p.getVector(ctx, queryTotalCPU)
	if err != nil {
		return 0, trace.Wrap(err)
	}
	if len(vector) != 1 {
		return 0, trace.BadParameter("expected single element: %v", vector)
	}
	return int(vector[0].Value), nil
}

// GetTotalMemory returns total amount of RAM in the cluster in bytes.
func (p *prometheus) GetTotalMemory(ctx context.Context) (int64, error) {
	vector, err := p.getVector(ctx, queryTotalMemory)
	if err != nil {
		return 0, trace.Wrap(err)
	}
	if len(vector) != 1 {
		return 0, trace.BadParameter("expected single element: %v", vector)
	}
	return int64(vector[0].Value), nil
}

// GetCPURate returns CPU usage rate for the specified time range.
func (p *prometheus) GetCPURate(ctx context.Context, timeRange v1.Range) (Series, error) {
	matrix, err := p.getMatrix(ctx, queryCPURate, timeRange)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(matrix) != 1 {
		return nil, trace.BadParameter("expected single element: %v", matrix)
	}
	var result Series
	for _, v := range matrix[0].Values {
		result = append(result, Point{
			Value: int(v.Value),
			Time:  v.Timestamp.Time(),
		})
	}
	return result, nil
}

// GetMemoryRate returns RAM usage rate for the specified time range.
func (p *prometheus) GetMemoryRate(ctx context.Context, timeRange v1.Range) (Series, error) {
	matrix, err := p.getMatrix(ctx, queryMemoryRate, timeRange)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(matrix) != 1 {
		return nil, trace.BadParameter("expected single element: %v", matrix)
	}
	var result Series
	for _, v := range matrix[0].Values {
		result = append(result, Point{
			Value: int(v.Value),
			Time:  v.Timestamp.Time(),
		})
	}
	return result, nil
}

// GetCurrentCPURate returns instantaneous CPU usage rate.
func (p *prometheus) GetCurrentCPURate(ctx context.Context) (int, error) {
	vector, err := p.getVector(ctx, queryCPURate)
	if err != nil {
		return 0, trace.Wrap(err)
	}
	if len(vector) != 1 {
		return 0, trace.BadParameter("expected single element: %v", vector)
	}
	return int(vector[0].Value), nil
}

// GetCurrentMemoryRate returns instantaneous RAM usage rate.
func (p *prometheus) GetCurrentMemoryRate(ctx context.Context) (int, error) {
	vector, err := p.getVector(ctx, queryMemoryRate)
	if err != nil {
		return 0, trace.Wrap(err)
	}
	if len(vector) != 1 {
		return 0, trace.BadParameter("expected single element: %v", vector)
	}
	return int(vector[0].Value), nil
}

// GetMaxCPURate returns highest CPU usage rate on the specified interval.
func (p *prometheus) GetMaxCPURate(ctx context.Context, interval time.Duration) (int, error) {
	var query bytes.Buffer
	if err := queryMaxCPU.Execute(&query, map[string]string{"interval": fmt.Sprintf("%vm", interval.Minutes())}); err != nil {
		return 0, trace.Wrap(err)
	}
	vector, err := p.getVector(ctx, query.String())
	if err != nil {
		return 0, trace.Wrap(err)
	}
	if len(vector) != 1 {
		return 0, trace.BadParameter("expected single element: %v", vector)
	}
	return int(vector[0].Value), nil
}

// GetMaxMemoryRate returns highest RAM usage rate on the specified interval.
func (p *prometheus) GetMaxMemoryRate(ctx context.Context, interval time.Duration) (int, error) {
	var query bytes.Buffer
	if err := queryMaxMemory.Execute(&query, map[string]string{"interval": fmt.Sprintf("%vm", interval.Minutes())}); err != nil {
		return 0, trace.Wrap(err)
	}
	vector, err := p.getVector(ctx, query.String())
	if err != nil {
		return 0, trace.Wrap(err)
	}
	if len(vector) != 1 {
		return 0, trace.BadParameter("expected single element: %v", vector)
	}
	return int(vector[0].Value), nil
}

// getVector executes the provided Prometheus query and returns the resulting
// "instant vector":
//
// https://prometheus.io/docs/prometheus/latest/querying/basics/#instant-vector-selectors
func (p *prometheus) getVector(ctx context.Context, query string) (model.Vector, error) {
	value, err := p.Query(ctx, query, time.Time{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if value.Type() != model.ValVector {
		return nil, trace.BadParameter("expected vector: %v %v", value.Type(), value.String())
	}
	return value.(model.Vector), nil
}

// getMatrix issues the provided Prometheus ranged query and returns the
// resulting "range vector":
//
// https://prometheus.io/docs/prometheus/latest/querying/basics/#range-vector-selectors
func (p *prometheus) getMatrix(ctx context.Context, query string, timeRange v1.Range) (model.Matrix, error) {
	value, err := p.QueryRange(ctx, query, timeRange)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if value.Type() != model.ValMatrix {
		return nil, trace.BadParameter("expected matrix: %v %v", value.Type(), value.String())
	}
	return value.(model.Matrix), nil
}

var (
	// queryTotalCPU is the Prometheus query that returns total number
	// of CPU cores in the cluster.
	queryTotalCPU = "cluster:cpu_total"
	// queryTotalMemory is the Prometheus query that returns total amount
	// of memory in the cluster in bytes.
	queryTotalMemory = "cluster:memory_total_bytes"
	// queryCPURate is the Prometheus query that returns CPU usage rate
	// in percent values.
	queryCPURate = "cluster:cpu_usage_rate"
	// queryMemoryRate is the Prometheus query that returns memory usage
	// rate in percent values.
	queryMemoryRate = "cluster:memory_usage_rate"
	// queryMaxCPU is the Prometheus query template that returns peak CPU
	// usage rate percent value on a certain interval.
	queryMaxCPU = template.Must(template.New("").Parse(
		"max_over_time(cluster:cpu_usage_rate[{{.interval}}])"))
	// queryMaxMemory is the Prometheus query template that returns peak
	// memory usage rate percent value on a certain interval.
	queryMaxMemory = template.Must(template.New("").Parse(
		"max_over_time(cluster:memory_usage_rate[{{.interval}}])"))
)
