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
	"fmt"
	"strings"

	"github.com/gravitational/gravity/lib/storage"

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

// ErrorToMessage returns a new message using the specified error
func ErrorToMessage(err error) *Message {
	return &Message{Element: &Message_Error{EncodeError(err)}}
}

// String describes this request as text
func (r *PeerJoinRequest) String() string {
	return fmt.Sprintf("PeerJoinRequest(addr=%v, config=%v)", r.Addr, r.Config)
}

// String describes this request as text
func (r *PeerLeaveRequest) String() string {
	return fmt.Sprintf("PeerLeaveRequest(addr=%v, config=%v)", r.Addr, r.Config)
}

// String describes this configuration as text
func (r *RuntimeConfig) String() string {
	var mounts []string
	for _, m := range r.Mounts {
		mounts = append(mounts, m.String())
	}
	return fmt.Sprintf("RuntimeConfig(role=%v, addr=%v, system-dev=%q, "+
		"state-dir=%v, temp-dir=%v, token=%v, key-values=%v, mounts=%v, cloud=%v)",
		r.Role,
		r.AdvertiseAddr,
		r.SystemDevice,
		r.StateDir,
		r.TempDir,
		r.Token,
		r.KeyValues,
		strings.Join(mounts, ","),
		r.CloudMetadata,
	)
}

// String describes this mount point as text
func (r Mount) String() string {
	return fmt.Sprintf("Mount(name=%v, source=%v)", r.Name, r.Source)
}

// MountsToProto converts list of mounts to protobuf format
func MountsToProto(mounts []storage.Mount) []*Mount {
	result := make([]*Mount, 0, len(mounts))
	for _, mount := range mounts {
		result = append(result, MountToProto(mount))
	}
	return result
}

// MountsFromProto converts list of mounts from protobuf format
func MountsFromProto(mounts []*Mount) []storage.Mount {
	result := make([]storage.Mount, 0, len(mounts))
	for _, mount := range mounts {
		result = append(result, MountFromProto(mount))
	}
	return result
}

// MountToProto converts mount to protobuf format
func MountToProto(mount storage.Mount) *Mount {
	return &Mount{
		Name:   mount.Name,
		Source: mount.Source,
	}
}

// MountFromProto converts mount from protobuf format
func MountFromProto(mount *Mount) storage.Mount {
	return storage.Mount{
		Name:   mount.Name,
		Source: mount.Source,
	}
}

// String describes this metadata object as text
func (r *CloudMetadata) String() string {
	if r == nil {
		return "CloudMetadata(<empty>)"
	}
	return fmt.Sprintf("CloudMetadata(node=%v, type=%v, id=%v)",
		r.NodeName, r.InstanceType, r.InstanceId)
}
