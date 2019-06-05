package dispatcher

import (
	"bytes"
	"fmt"
	"unicode"

	installpb "github.com/gravitational/gravity/lib/install/proto"
	"github.com/gravitational/gravity/lib/ops"
)

// EventDispatcher dispatches progress events to clients
type EventDispatcher interface {
	Send(Event)
	// Close stops the dispatcher and release its resources.
	// It is an error to invoke Send after Close
	Close()
	// Chan returns the channel that receives events
	Chan() <-chan *installpb.ProgressResponse
}

// IsCompleted determines if this event indicates a completed operation event
func (r Event) IsCompleted() bool {
	return r.Status == StatusCompleted ||
		r.Status == StatusCompletedPending
}

// String formats this event as text
func (r Event) String() string {
	var buf bytes.Buffer
	fmt.Print(&buf, "event(")
	if r.Progress != nil {
		fmt.Fprintf(&buf, "progress(completed=%v, message=%v),",
			r.Progress.Completion, r.Progress.Message)
	}
	if r.Error != nil {
		fmt.Fprintf(&buf, "error(%v),", r.Error.Error())
	}
	fmt.Fprintf(&buf, "status(%v)", r.Status)
	fmt.Print(&buf, ")")
	return buf.String()
}

// AsProgressResponse translates this event to proto format
func (r *Event) AsProgressResponse() *installpb.ProgressResponse {
	resp := &installpb.ProgressResponse{}
	if r.Error != nil {
		resp.Error = &installpb.Error{Message: r.Error.Error()}
	}
	if r.Progress == nil {
		return resp
	}
	resp.Message = r.Progress.Message
	switch r.Status {
	case StatusCompleted:
		resp.Status = installpb.StatusCompleted
	case StatusCompletedPending:
		resp.Status = installpb.StatusCompletedPending
	}
	return resp
}

// Event describes the installer progress step
type Event struct {
	// Progress describes the operation progress
	Progress *ops.ProgressEntry
	// Error specifies the error if any
	Error error
	// Completed indicates whether this event is terminal
	Status Status
}

// Status defines the progress status
type Status byte

const (
	// StatusUnknown indicates an unknown progress status
	StatusUnknown Status = 0
	// StatusCompleted indicates a completed operation
	StatusCompleted Status = iota
	// StatusCompletedPending indicates a completed operation
	// but with installer still active
	StatusCompletedPending Status = iota
)

// NewWriter creates a new event writer that can be used a progress output sink
func NewWriter(dispatcher EventDispatcher) *EventWriter {
	return &EventWriter{EventDispatcher: dispatcher}
}

// Write sends p as progress event to the server.
// Implements io.Writer
func (r *EventWriter) Write(p []byte) (n int, err error) {
	// TODO(dmitri): truncate explicit newlines to avoid having
	// empty lines in output. This needs a more consistent way to
	// format progress messages
	p = bytes.TrimRightFunc(p, unicode.IsSpace)
	r.EventDispatcher.Send(Event{
		Progress: &ops.ProgressEntry{Message: string(p)},
	})
	return len(p), nil
}

// EventWriter implements io.Writer.
// It can be used as a progress output sink for relaying events
type EventWriter struct {
	EventDispatcher
}
