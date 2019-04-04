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
	fmt "fmt"
	"strings"

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
	return &Message{Element: &Message_Error{EncodeError(err)}}
}

func DecodeError(err *Error) error {
	return errors.New(err.Message)
}

func FormatPeerJoinRequest(req *PeerJoinRequest) string {
	return fmt.Sprintf("PeerJoinRequest(addr=%v, config=%v)",
		req.Addr, FormatRuntimeConfig(req.Config))
}

func FormatPeerLeaveRequest(req *PeerLeaveRequest) string {
	return fmt.Sprintf("PeerLeaveRequest(addr=%v, config=%v)",
		req.Addr, FormatRuntimeConfig(req.Config))
}

func FormatRuntimeConfig(config *RuntimeConfig) string {
	var mounts []string
	for _, m := range config.Mounts {
		mounts = append(mounts, FormatMount(*m))
	}
	return fmt.Sprintf("RuntimeConfig(role=%v, addr=%v, docker-dev=%q, system-dev=%q, "+
		"state-dir=%v, temp-dir=%v, token=%v, key-values=%v, mounts=%v, cloud=%v)",
		config.Role,
		config.AdvertiseAddr,
		config.DockerDevice,
		config.SystemDevice,
		config.StateDir,
		config.TempDir,
		config.Token,
		config.KeyValues,
		strings.Join(mounts, ","),
		FormatCloudMetadata(config.CloudMetadata),
	)
}

func FormatMount(m Mount) string {
	return fmt.Sprintf("Mount(name=%v, source=%v)", m.Name, m.Source)
}

func FormatCloudMetadata(m *CloudMetadata) string {
	if m == nil {
		return "CloudMetadata(<empty>)"
	}
	return fmt.Sprintf("CloudMetadata(node=%v, type=%v, id=%v)",
		m.NodeName, m.InstanceType, m.InstanceId)
}
