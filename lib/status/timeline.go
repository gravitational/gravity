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

package status

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/gravitational/gravity/lib/httplib"

	"github.com/fatih/color"
	pb "github.com/gravitational/satellite/agent/proto/agentpb"
	"github.com/gravitational/trace"
)

// Timeline queries the currently stored cluster timeline.
func Timeline(ctx context.Context) (*pb.TimelineResponse, error) {
	client, err := httplib.GetGRPCPlanetClient(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return client.Timeline(ctx, &pb.TimelineRequest{})
}

// PrintEvent prints the event to the provided writer.
func PrintEvent(w io.Writer, event *pb.TimelineEvent) {
	timestamp := event.GetTimestamp().ToTime().Format(time.RFC3339)
	switch event.GetData().(type) {
	case *pb.TimelineEvent_ClusterDegraded:
		fmt.Fprintln(w, color.RedString("%s [Cluster Degraded]",
			timestamp))
	case *pb.TimelineEvent_ClusterHealthy:
		fmt.Fprintln(w, color.GreenString("%s [Cluster Healthy]",
			timestamp))
	case *pb.TimelineEvent_NodeAdded:
		fmt.Fprintln(w, color.YellowString("%s [Node Added]\tnode=%s",
			timestamp, event.GetNodeAdded().GetNode()))
	case *pb.TimelineEvent_NodeRemoved:
		fmt.Fprintln(w, color.YellowString("%s [Node Removed]\tnode=%s",
			timestamp, event.GetNodeRemoved().GetNode()))
	case *pb.TimelineEvent_NodeDegraded:
		fmt.Fprintln(w, color.RedString("%s [Node Degraded]\tnode=%s",
			timestamp, event.GetNodeDegraded().GetNode()))
	case *pb.TimelineEvent_NodeHealthy:
		fmt.Fprintln(w, color.GreenString("%s [Node Healthy]\tnode=%s",
			timestamp, event.GetNodeHealthy().GetNode()))
	case *pb.TimelineEvent_ProbeFailed:
		e := event.GetProbeFailed()
		fmt.Fprintln(w, color.RedString("%s [Probe Failed]\tnode=%s\tchecker=%s",
			timestamp, e.GetNode(), e.GetProbe()))
	case *pb.TimelineEvent_ProbeSucceeded:
		e := event.GetProbeSucceeded()
		fmt.Fprintln(w, color.GreenString("%s [Probe Succeeded]\tnode=%s\tchecker=%s",
			timestamp, e.GetNode(), e.GetProbe()))
	case *pb.TimelineEvent_LeaderElected:
		e := event.GetLeaderElected()
		if e.GetPrev() == "" {
			fmt.Fprintln(w, color.YellowString("%s [Leader Elected]\tnew leader %s",
				timestamp, e.GetNew()))
		} else {
			fmt.Fprintln(w, color.YellowString("%s [Leader Elected]\tleader changed %s -> %s",
				timestamp, e.GetPrev(), e.GetNew()))
		}
	default:
		fmt.Fprintln(w, color.YellowString("%s [Unknown Event]\t%s", timestamp, event))
		log.WithField("event", event).Warn("Received unknown event type.")
	}
}
