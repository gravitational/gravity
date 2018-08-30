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

package proto

import (
	"errors"

	"github.com/gravitational/trace"
	"golang.org/x/net/context"
)

// defines expected file names in package
const (
	CA         = "ca"
	Client     = "client"
	Server     = "server"
	Key        = "key"
	Cert       = "cert"
	ServerName = "gravity-agent"
)

// IncomingMessageStream defines an incoming message stream
type IncomingMessageStream interface {
	// Recv receives a message from the stream.
	// Blocks until the message is received
	Recv() (*Message, error)
}

// OutgoingMessageStream defines an outgoing message stream.
// The stream is used to dispatch progress events and raw command output
// during command execution
type OutgoingMessageStream interface {
	// Send sends a message to the stream.
	Send(*Message) error
	// Context returns context associated with the stream
	Context() context.Context
}

// EncodeError converts error err to protobuf wire-friendly format
func EncodeError(err error) *Error {
	var errWithTraces *trace.TraceErr
	var ok bool
	if errWithTraces, ok = err.(*trace.TraceErr); ok {
		return &Error{Message: trace.UserMessage(err)}
	}

	traces := make([]string, 0, len(errWithTraces.Traces))
	for _, trace := range errWithTraces.Traces {
		traces = append(traces, trace.String())
	}
	return &Error{
		Message: trace.UserMessage(err),
		Traces:  traces,
	}
}

func ErrorToMessage(err error) *Message {
	return &Message{&Message_Error{EncodeError(err)}}
}

func DecodeError(err *Error) error {
	return errors.New(err.Message)
}
