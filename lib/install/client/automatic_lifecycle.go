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
	"github.com/gravitational/gravity/lib/system/signals"

	"github.com/gravitational/trace"
)

// HandleStatus handles the results of a completed operation.
// It executes the handler corresponding to the outcome
func (r *AutomaticLifecycle) HandleStatus(ctx context.Context, c *Client, status installpb.ProgressResponse_Status, err error) error {
	switch {
	case err == nil:
		switch status {
		case installpb.StatusUnknown:
			if err1 := c.shutdown(ctx); err1 != nil {
				c.WithError(err1).Warn("Failed to shut down.")
			}
			return nil
		case installpb.StatusAborted:
			return r.Abort(ctx, c)
		}
		// We received completion status
		err = r.Complete(ctx, c, status)
		return trace.Wrap(err)
	case trace.IsEOF(err):
		// Stream done but no completion event
		if err1 := c.shutdown(ctx); err1 != nil {
			c.WithError(err1).Warn("Failed to shut down.")
		}
		return nil
	default:
		if err1 := r.generateDebugReport(ctx, c); err1 != nil {
			c.WithError(err1).Warn("Failed to generate debug report.")
		}
		if err1 := c.shutdown(ctx); err1 != nil {
			c.WithError(err1).Warn("Failed to shut down.")
		}
		return trace.Wrap(err)
	}
}

func (r *AutomaticLifecycle) Complete(ctx context.Context, c *Client, status installpb.ProgressResponse_Status) error {
	err := c.shutdown(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(r.Completer(ctx, c.InterruptHandler, status))
}

func (r *AutomaticLifecycle) Abort(ctx context.Context, c *Client) error {
	if r.Aborter == nil {
		return nil
	}
	if err := r.Aborter(ctx); err != nil {
		c.WithError(err).Warn("Failed to abort.")
	}
	return installpb.ErrAborted
}

// AutomaticLifecycle handles the completion of an operation.
// If the operation is interrupted, it runs the abort handler.
// If the operation completes successfully, it runs the completion handler.
// If the operation fails, it generates a debug report
type AutomaticLifecycle struct {
	// Aborter specifies the completion handler for when the operation is aborted
	Aborter func(context.Context) error
	// Completer specifies the optional completion handler for when the operation
	// is completed successfully
	Completer CompletionHandler
	// DebugReportPath specifies the path to the debug report file
	DebugReportPath string
	// LocalDebugReporter specifies the handler for generating host-local debug
	// report
	LocalDebugReporter func(ctx context.Context, path string) error
}

func (r *AutomaticLifecycle) checkAndSetDefaults() error {
	if r.Completer == nil {
		return trace.BadParameter("Completer is required")
	}
	return nil
}

func (r *AutomaticLifecycle) generateDebugReport(ctx context.Context, c *Client) error {
	if r.DebugReportPath == "" {
		return nil
	}
	c.PrintStep("Saving debug report to %v", r.DebugReportPath)
	err := c.generateDebugReport(ctx, r.DebugReportPath)
	if err != nil {
		if r.LocalDebugReporter != nil {
			r.LocalDebugReporter(ctx, r.DebugReportPath)
		}
	}
	return trace.Wrap(err)
}

// CompletionHandler describes a functional handler for tasks to run after
// operation is complete
type CompletionHandler func(context.Context, *signals.InterruptHandler, installpb.ProgressResponse_Status) error
