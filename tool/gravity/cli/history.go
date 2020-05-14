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

package cli

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/gravitational/gravity/lib/defaults"
	statusapi "github.com/gravitational/gravity/lib/status"

	pb "github.com/gravitational/satellite/agent/proto/agentpb"
	"github.com/gravitational/trace"
)

// statusHistory collects cluster status history and prints the information
// to stdout.
func statusHistory() error {
	ctx, cancel := context.WithTimeout(context.Background(), defaults.StatusCollectionTimeout)
	defer cancel()
	timeline, err := statusapi.Timeline(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	return printEvents(timeline.GetEvents())
}

func printEvents(events []*pb.TimelineEvent) error {
	if len(events) == 0 {
		fmt.Println("No status history available to display.")
		return nil
	}

	w := new(tabwriter.Writer)
	w.Init(os.Stdout, 0, 8, 1, '\t', 0)
	for _, event := range events {
		statusapi.PrintEvent(w, event)
	}
	return w.Flush()
}
