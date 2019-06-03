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

package process

import (
	"context"

	"github.com/gravitational/gravity/lib/constants"

	teleservice "github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/trace"
)

// WaitForServiceStarted blocks until an event notifying that the service had started
// is received
func WaitForServiceStarted(ctx context.Context, service GravityProcess) error {
	eventC := make(chan teleservice.Event)
	service.WaitForEvent(ctx, constants.ServiceStartedEvent, eventC)
	select {
	case event := <-eventC:
		serviceStartedEvent, ok := event.Payload.(*ServiceStartedEvent)
		if !ok {
			return trace.BadParameter("expected ServiceStartedEvent but got %T", serviceStartedEvent)
		}
		return trace.Wrap(serviceStartedEvent.Error)
	case <-ctx.Done():
		return trace.Wrap(ctx.Err())
	}
}
