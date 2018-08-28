package client

import (
	"context"
	"time"

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
