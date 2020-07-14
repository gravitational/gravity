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

package client

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gravitational/gravity/lib/modules"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gogo/protobuf/types"
	pb "github.com/gravitational/gravity/lib/rpc/proto"
	"github.com/gravitational/trace"
)

// GetSystemInfo queries remote system information
func (c *client) GetSystemInfo(ctx context.Context) (storage.System, error) {
	resp, err := c.discovery.GetSystemInfo(ctx, &types.Empty{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	system, err := storage.UnmarshalSystemInfo(resp.Payload)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return system, nil
}

// GetRuntimeConfig returns agent's runtime configuration
func (c *client) GetRuntimeConfig(ctx context.Context) (*pb.RuntimeConfig, error) {
	config, err := c.discovery.GetRuntimeConfig(ctx, &types.Empty{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return config, nil
}

// GetCurrentTime returns agent's current time as UTC timestamp
func (c *client) GetCurrentTime(ctx context.Context) (*time.Time, error) {
	proto, err := c.discovery.GetCurrentTime(ctx, &types.Empty{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ts, err := types.TimestampFromProto(proto)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &ts, nil
}

// GetVersion returns agent's version information
func (c *client) GetVersion(ctx context.Context) (version modules.Version, err error) {
	resp, err := c.discovery.GetVersion(ctx, &types.Empty{})
	if err != nil {
		return version, trace.Wrap(err)
	}

	if err := json.Unmarshal(resp.Payload, &version); err != nil {
		return version, trace.Wrap(err)
	}

	return version, nil
}
