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

	installpb "github.com/gravitational/gravity/lib/install/proto"

	"github.com/gravitational/trace"
)

// HandleStatus implements status handling by propagating specified error to the caller.
// If the error indicates end-of-stream, it is ignored
func (r *NoopLifecycle) HandleStatus(ctx context.Context, c *Client, status installpb.ProgressResponse_Status, err error) error {
	switch {
	case err == nil:
		return nil
	case trace.IsEOF(err):
		// Stream done but no completion event
		return nil
	default:
		return trace.Wrap(err)
	}
}

// Complete is a no-op
func (r *NoopLifecycle) Complete(context.Context, *Client, installpb.ProgressResponse_Status) error {
	return nil
}

// Abort is a no-op
func (r *NoopLifecycle) Abort(context.Context, *Client) error {
	return nil
}

// NoopLifecycle implements a client lifecycle that does nothing
type NoopLifecycle struct{}

func (r *NoopLifecycle) checkAndSetDefaults() error {
	return nil
}
