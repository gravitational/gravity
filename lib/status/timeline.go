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

	"github.com/gravitational/gravity/lib/httplib"

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
