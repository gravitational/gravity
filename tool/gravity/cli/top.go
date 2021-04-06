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

package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/ops/monitoring"
	"github.com/gravitational/gravity/lib/ops/opsservice"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/buger/goterm"
	"github.com/dustin/go-humanize"
	"github.com/gizak/termui"
	"github.com/gravitational/trace"
)

func top(env *localenv.LocalEnvironment, interval, step time.Duration) error {
	prometheusAddr, err := utils.ResolveAddr(env.DNS.Addr(), defaults.PrometheusServiceAddr)
	if err != nil {
		return trace.Wrap(err)
	}

	prometheusClient, err := monitoring.NewPrometheus(prometheusAddr)
	if err != nil {
		return trace.Wrap(err)
	}

	err = termui.Init()
	if err != nil {
		return trace.Wrap(err)
	}
	defer termui.Close()

	go render(env, context.TODO(), prometheusClient, interval, step)

	termui.Handle("/sys/kbd/q", func(termui.Event) {
		termui.StopLoop()
	})
	termui.Loop()

	return nil
}

// render continuously spins in a loop retrieving cluster metrics and rendering
// terminal widgets at a certain interval.
func render(env *localenv.LocalEnvironment, ctx context.Context, client monitoring.Metrics, interval, step time.Duration) {
	ticker := time.NewTicker(refreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			cluster, err := env.LocalCluster()
			if err != nil {
				log.Errorf(trace.DebugReport(err))
				continue
			}
			metrics, err := opsservice.GetClusterMetrics(ctx, client, ops.ClusterMetricsRequest{
				SiteKey:  cluster.Key(),
				Interval: interval,
				Step:     step,
			})
			if err != nil {
				log.Errorf(trace.DebugReport(err))
				continue
			}
			reRender(*cluster, *metrics)
		case <-ctx.Done():
			return
		}
	}
}

// reRender renders terminal widgets for the provided metrics data.
func reRender(cluster ops.Site, metrics ops.ClusterMetricsResponse) {
	var cpuData []float64
	var cpuLabels []string
	for _, point := range metrics.CPURates.Historic {
		cpuData = append(cpuData, float64(point.Value))
		cpuLabels = append(cpuLabels, point.Time.Format(constants.TimeFormat))
	}

	var ramData []float64
	var ramLabels []string
	for _, point := range metrics.MemoryRates.Historic {
		ramData = append(ramData, float64(point.Value))
		ramLabels = append(ramLabels, point.Time.Format(constants.TimeFormat))
	}

	dim := getDimensions()

	widgets := []termui.Bufferer{
		getTitle(titleParams{
			Title: fmt.Sprintf("Totals / Last Updated: %v",
				time.Now().Format(constants.HumanDateFormatSeconds)),
			Text: fmt.Sprintf("Nodes: %v\tCPU Cores: %v\tMemory: %v",
				len(cluster.ClusterState.Servers),
				metrics.TotalCPUCores,
				humanize.Bytes(uint64(metrics.TotalMemoryBytes))),
			H: dim.TitleH,
			W: dim.TitleW,
			X: 0,
			Y: 0,
		}),
		getGauge(gaugeParams{
			Title:   "Current CPU",
			Percent: metrics.CPURates.Current,
			H:       dim.GaugeH,
			W:       dim.GaugeW,
			X:       0,
			Y:       dim.TitleH,
		}),
		getGauge(gaugeParams{
			Title:   "Peak CPU",
			Percent: metrics.CPURates.Max,
			H:       dim.GaugeH,
			W:       dim.GaugeW,
			X:       0,
			Y:       dim.TitleH + dim.GaugeH,
		}),
		getChart(chartParams{
			Title:  "CPU",
			Data:   cpuData,
			Labels: cpuLabels,
			H:      dim.ChartH,
			W:      dim.ChartW,
			X:      dim.GaugeW,
			Y:      dim.TitleH,
		}),
		getGauge(gaugeParams{
			Title:   "Current RAM",
			Percent: metrics.MemoryRates.Current,
			H:       dim.GaugeH,
			W:       dim.GaugeW,
			X:       0,
			Y:       dim.TitleH + 2*dim.GaugeH,
		}),
		getGauge(gaugeParams{
			Title:   "Peak RAM",
			Percent: metrics.MemoryRates.Max,
			H:       dim.GaugeH,
			W:       dim.GaugeW,
			X:       0,
			Y:       dim.TitleH + 3*dim.GaugeH,
		}),
		getChart(chartParams{
			Title:  "RAM",
			Data:   ramData,
			Labels: ramLabels,
			H:      dim.ChartH,
			W:      dim.ChartW,
			X:      dim.GaugeW,
			Y:      dim.TitleH + 2*dim.GaugeH,
		}),
	}

	termui.Clear()
	termui.Render(widgets...)
}

type titleParams struct {
	Title string
	Text  string
	H, W  int
	X, Y  int
}

// getTitle returns title widget with specified parameters.
func getTitle(p titleParams) *termui.Par {
	title := termui.NewPar(p.Text)
	title.BorderLabel = p.Title
	title.Height = p.H
	title.Width = p.W
	title.X = p.X
	title.Y = p.Y
	return title
}

type gaugeParams struct {
	Title   string
	Percent int
	H, W    int
	X, Y    int
}

// getGauge returns gauge widget with specified parameters.
func getGauge(p gaugeParams) *termui.Gauge {
	gauge := termui.NewGauge()
	gauge.BorderLabel = p.Title
	gauge.Percent = p.Percent
	gauge.BarColor = getColor(p.Percent)
	gauge.Height = p.H
	gauge.Width = p.W
	gauge.X = p.X
	gauge.Y = p.Y
	return gauge
}

type chartParams struct {
	Title  string
	Data   []float64
	Labels []string
	H, W   int
	X, Y   int
}

// getChart returns line chart widget with specified parameters.
func getChart(p chartParams) *termui.LineChart {
	chart := termui.NewLineChart()
	chart.BorderLabel = p.Title
	chart.Data = p.Data
	chart.DataLabels = p.Labels
	chart.Mode = "dot"
	chart.Height = p.H
	chart.Width = p.W
	chart.X = p.X
	chart.Y = p.Y
	return chart
}

// Dimensions contains dimentions (height/width) for terminal widgets.
type Dimensions struct {
	TitleH, TitleW int
	GaugeH, GaugeW int
	ChartH, ChartW int
}

// getDimensions returns terminal widget dimensions based on the terminal size.
func getDimensions() Dimensions {
	termH := goterm.Height()
	termW := goterm.Width()

	// Let the title widget occupy 10% of terminal height.
	titleH := termH / 10
	titleW := termW

	// We currently have 4 gauges one under another.
	gaugeH := (termH - titleH) / 4
	gaugeW := termW / 5

	// We currently have 2 charts one under another next to the gauges.
	chartH := 2 * gaugeH
	chartW := termW - gaugeW

	return Dimensions{
		TitleH: titleH,
		TitleW: titleW,
		GaugeH: gaugeH,
		GaugeW: gaugeW,
		ChartH: chartH,
		ChartW: chartW,
	}
}

// getColor returns appropriate color based on percent value.
func getColor(percent int) termui.Attribute {
	if percent <= 25 {
		return termui.ColorGreen
	} else if percent > 75 {
		return termui.ColorRed
	}
	return termui.ColorYellow
}

// refreshInterval is how often terminal widges are refreshed.
const refreshInterval = 2 * time.Second
