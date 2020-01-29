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

package storage

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gravitational/gravity/lib/defaults"

	teleservices "github.com/gravitational/teleport/lib/services"
	teleutils "github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

// LogForwarder describes a log forwarder resource
type LogForwarder interface {
	teleservices.Resource
	// GetAddress returns log forwarder address
	GetAddress() string
	// GetProtocol returns log forwarder protocol
	GetProtocol() string
	// CheckAndSetDefaults validates log forwarder configuration
	CheckAndSetDefaults() error
}

// LogForwarderV2 represents log forwarder resource
type LogForwarderV2 struct {
	// Kind is the resource kind, "logforwarder"
	Kind string `json:"kind"`
	// Version is the resource version, "v2"
	Version string `json:"version"`
	// Metadata contains log forwarder metadata
	Metadata teleservices.Metadata `json:"metadata"`
	// Spec is log forwarder spec
	Spec LogForwarderSpecV2 `json:"spec"`
}

// NewLogForwarder creates a new log forwarder
func NewLogForwarder(name, address, protocol string) LogForwarder {
	return &LogForwarderV2{
		Kind:    KindLogForwarder,
		Version: teleservices.V2,
		Metadata: teleservices.Metadata{
			Name:      name,
			Namespace: defaults.Namespace,
		},
		Spec: LogForwarderSpecV2{
			Address:  address,
			Protocol: protocol,
		},
	}
}

// NewLogForwarderFromV1 creates a new log forwarder from legacy format
func NewLogForwarderFromV1(l LogForwarderV1) LogForwarder {
	return NewLogForwarder(
		strings.Replace(l.Address, ":", "_", -1), l.Address, l.Protocol)
}

// GetName returns log forwarder name
func (l *LogForwarderV2) GetName() string {
	return l.Metadata.Name
}

// SetName sets log forwarder name
func (l *LogForwarderV2) SetName(name string) {
	l.Metadata.Name = name
}

// GetMetadata returns log forwarder metadata
func (l *LogForwarderV2) GetMetadata() teleservices.Metadata {
	return l.Metadata
}

// SetExpiry sets log forwarder expiration time
func (l *LogForwarderV2) SetExpiry(expires time.Time) {
	l.Metadata.SetExpiry(expires)
}

// Expiry returns log forwarder expiration time
func (l *LogForwarderV2) Expiry() time.Time {
	return l.Metadata.Expiry()
}

// SetTTL sets log forwarder TTL
func (l *LogForwarderV2) SetTTL(clock clockwork.Clock, ttl time.Duration) {
	l.Metadata.SetTTL(clock, ttl)
}

// GetAddress returns log forwarder address
func (l *LogForwarderV2) GetAddress() string {
	return l.Spec.Address
}

// GetProtocol returns log forwarder protocol
func (l *LogForwarderV2) GetProtocol() string {
	return l.Spec.Protocol
}

// CheckAndSetDefaults validates log forwarder configuration
func (l *LogForwarderV2) CheckAndSetDefaults() error {
	if l.Metadata.Name == "" {
		return trace.BadParameter("missing parameter Name")
	}
	if l.Spec.Address == "" {
		return trace.BadParameter("missing parameter Address")
	}
	if l.Spec.Protocol != "" {
		if l.Spec.Protocol != "tcp" && l.Spec.Protocol != "udp" {
			return trace.BadParameter(
				"unsupported protocol %q, must be one of: tcp, udp", l.Spec.Protocol)
		}
	} else {
		l.Spec.Protocol = "tcp"
	}
	return nil
}

// LogForwarderSpecV2 is the log forwarder spec
type LogForwarderSpecV2 struct {
	// Address is log forwarder address
	Address string `json:"address"`
	// Protocol is log forwarder protocol
	Protocol string `json:"protocol,omitempty"`
}

// LogForwarderV2Scheme is the log forwarder JSON schema
const LogForwarderV2Schema = `{
  "type": "object",
  "additionalProperties": false,
  "required": ["address"],
  "properties": {
    "address": {"type": "string"},
    "protocol": {"type": "string"}
  }
}`

// GetLogForwarderMarshaler returns log forwarder marshaler
func GetLogForwarderMarshaler() LogForwarderMarshaler {
	return &logForwarderMarshaler{}
}

// LogForwarderMarshaler defines methods to marshal/unmarshal log forwarders
type LogForwarderMarshaler interface {
	// Unmarshal unmarshals log forwarder
	Unmarshal([]byte) (LogForwarder, error)
	// Marshal marshals log forwarder
	Marshal(LogForwarder, ...teleservices.MarshalOption) ([]byte, error)
}

type logForwarderMarshaler struct{}

// Unmarshal unmarshals log forwarder
func (m *logForwarderMarshaler) Unmarshal(data []byte) (LogForwarder, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing log forwarder data")
	}

	jsonData, err := teleutils.ToJSON(data)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var header teleservices.ResourceHeader
	err = json.Unmarshal(jsonData, &header)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch header.Version {
	case "":
		var l LogForwarderV1
		err := json.Unmarshal(jsonData, &l)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return NewLogForwarderFromV1(l), nil
	case teleservices.V2:
		var l LogForwarderV2
		err := teleutils.UnmarshalWithSchema(GetLogForwarderSchema(), &l, jsonData)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		//nolint:errcheck
		l.Metadata.CheckAndSetDefaults()
		return &l, nil
	}

	return nil, trace.BadParameter(
		"log forwarder resource version %q is not supported", header.Version)
}

// Marshal marshals log forwarder
func (m *logForwarderMarshaler) Marshal(l LogForwarder, opts ...teleservices.MarshalOption) ([]byte, error) {
	return json.Marshal(l)
}

// GetLogForwarderSchema returns log forwarder JSON schema
func GetLogForwarderSchema() string {
	return fmt.Sprintf(teleservices.V2SchemaTemplate,
		teleservices.MetadataSchema, LogForwarderV2Schema, "")
}

// LogForwarderV1 is the legacy log forwarder spec
type LogForwarderV1 struct {
	// Address is log forwarder address
	Address string `json:"address"`
	// Protocol is log forwarder protocol
	Protocol string `json:"protocol"`
}
