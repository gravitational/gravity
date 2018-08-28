package client

import (
	"context"

	validationpb "github.com/gravitational/gravity/lib/network/validation/proto"
	"github.com/gravitational/trace"
)

// CheckPorts executes a network port test
func (c *client) CheckPorts(ctx context.Context, req *validationpb.CheckPortsRequest) (*validationpb.CheckPortsResponse, error) {
	resp, err := c.validation.CheckPorts(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp, nil
}

// CheckBandwidth executes a network bandwidth test
func (c *client) CheckBandwidth(ctx context.Context, req *validationpb.CheckBandwidthRequest) (*validationpb.CheckBandwidthResponse, error) {
	resp, err := c.validation.CheckBandwidth(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp, nil
}
