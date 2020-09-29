/*
Copyright 2020 Gravitational, Inc.

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

package report

import (
	"context"
	"io"

	"github.com/gravitational/gravity/lib/defaults"
	statusapi "github.com/gravitational/gravity/lib/status"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
)

// NewTimelineCollector returns a new timeline collector.
func NewTimelineCollector() *TimelineCollector {
	return &TimelineCollector{}
}

// Collect collects the cluster status timeline.
func (r TimelineCollector) Collect(ctx context.Context, reportWriter FileWriter, runner utils.CommandRunner) error {
	w, err := reportWriter.NewWriter(timelineFilename)
	if err != nil {
		return trace.Wrap(err)
	}
	defer w.Close()
	ctx, cancel := context.WithTimeout(ctx, defaults.StatusCollectionTimeout)
	defer cancel()
	return trace.Wrap(collectTimeline(ctx, w))
}

// collectTimeline collects the cluster status timeline and writes to the
// provided writer.
func collectTimeline(ctx context.Context, w io.Writer) error {
	resp, err := statusapi.Timeline(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, event := range resp.GetEvents() {
		statusapi.PrintEvent(w, event)
	}
	return nil
}

// TimelineCollector collects the cluster status timeline.
type TimelineCollector struct{}

const (
	// timelineFilename is the name of the file that stores the status timeline output
	timelineFilename = "timeline"
)
