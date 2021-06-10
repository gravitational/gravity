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

	validationpb "github.com/gravitational/gravity/lib/network/validation/proto"
	"github.com/gravitational/trace"
)

// CheckPorts executes a network port test
func (c *Client) CheckPorts(ctx context.Context, req *validationpb.CheckPortsRequest) (*validationpb.CheckPortsResponse, error) {
	resp, err := c.validation.CheckPorts(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp, nil
}

// CheckBandwidth executes a network bandwidth test
func (c *Client) CheckBandwidth(ctx context.Context, req *validationpb.CheckBandwidthRequest) (*validationpb.CheckBandwidthResponse, error) {
	resp, err := c.validation.CheckBandwidth(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp, nil
}

// CheckDisks executes disk performance test.
func (c *Client) CheckDisks(ctx context.Context, req *validationpb.CheckDisksRequest) (*validationpb.CheckDisksResponse, error) {
	resp, err := c.validation.CheckDisks(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return resp, nil
}
