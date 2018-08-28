package process

import (
	"github.com/gravitational/gravity/lib/constants"

	teleservice "github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/trace"
)

// WaitForServiceStarted blocks until an event notifying that the service had started
// is received
func WaitForServiceStarted(service GravityProcess) error {
	eventC := make(chan teleservice.Event)
	service.WaitForEvent(constants.ServiceStartedEvent, eventC, nil)
	event := <-eventC
	serviceStartedEvent, ok := event.Payload.(*ServiceStartedEvent)
	if !ok {
		return trace.BadParameter("expected ServiceStartedEvent but got %T", serviceStartedEvent)
	}

	return trace.Wrap(serviceStartedEvent.Error)
}

// WaitForServiceLeader blocks until it receives the leader notification from the service.
// Only the notification of this service itself becoming the leader are dispatched.
func WaitForServiceSelfLeader(service *Process) {
	eventC := make(chan teleservice.Event)
	service.WaitForEvent(constants.ServiceSelfLeaderEvent, eventC, nil)
	<-eventC
}
