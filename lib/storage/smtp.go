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

	"github.com/gravitational/gravity/lib/defaults"

	teleservices "github.com/gravitational/teleport/lib/services"
	teleutils "github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

// SMTPConfig describes cluster SMTP configuration
type SMTPConfig interface {
	// Resource provides common resource methods
	teleservices.Resource
	// CheckAndSetDefaults verifies that the object is valid
	CheckAndSetDefaults() error
	// GetHost returns the SMTP host
	GetHost() string
	// GetPort returns the SMTP port
	GetPort() int
	// GetUsername returns SMTP username
	GetUsername() string
	// GetPassword returns SMTP password
	GetPassword() string
}

// SMTPConfigV2 defines SMTP configuration
type SMTPConfigV2 struct {
	// Metadata is resource metadata
	teleservices.Metadata `json:"metadata"`
	// Kind is a resource kind
	Kind string `json:"kind"`
	// Version is a resource version
	Version string `json:"version"`
	// Spec defines the SMTP configuration
	Spec SMTPConfigSpecV2 `json:"spec"`
}

// GetHost returns SMTP host
func (r *SMTPConfigV2) GetHost() string {
	return r.Spec.Host
}

// GetPort returns SMTP port
func (r *SMTPConfigV2) GetPort() int {
	return r.Spec.Port
}

// GetUsername returns SMTP username
func (r *SMTPConfigV2) GetUsername() string {
	return r.Spec.Username
}

// GetPassword returns SMTP password
func (r *SMTPConfigV2) GetPassword() string {
	return r.Spec.Password
}

// CheckAndSetDefaults checks validity of all parameters and sets defaults
func (r *SMTPConfigV2) CheckAndSetDefaults() error {
	if r.Spec.Host == "" {
		return trace.BadParameter("missing parameter Host")
	}

	if r.Spec.Port < 0 || r.Spec.Port > 65535 {
		return trace.BadParameter("Invalid port %v", r.Spec.Port)
	}

	if r.Spec.Port == 0 {
		r.Spec.Port = defaults.SMTPPort
	}

	return nil
}

// UnmarshalSMTPConfig unmarshals SMTP configuration from JSON
func UnmarshalSMTPConfig(data []byte) (SMTPConfig, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("empty configuration")
	}

	jsonData, err := teleutils.ToJSON(data)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var hdr teleservices.ResourceHeader
	err = json.Unmarshal(jsonData, &hdr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	switch hdr.Version {
	case teleservices.V2:
		var config SMTPConfigV2
		err := teleutils.UnmarshalWithSchema(GetSMTPConfigSchema(), &config, jsonData)
		if err != nil {
			return nil, trace.BadParameter(err.Error())
		}
		//nolint:errcheck
		config.Metadata.CheckAndSetDefaults()
		return &config, nil
	}
	return nil, trace.BadParameter(
		"%v resource version %q is not supported", KindSMTPConfig, hdr.Version)
}

// MarshalSMTPConfig marshals SMTP config into JSON
func MarshalSMTPConfig(config SMTPConfig, opts ...teleservices.MarshalOption) ([]byte, error) {
	return json.Marshal(config)
}

// SMTPConfigSpecV2 defines SMTP configuration for the cluster
type SMTPConfigSpecV2 struct {
	// Host specifies the SMTP host
	Host string `json:"host"`
	// Port specifies the SMTP port
	Port int `json:"port"`
	// Username specifies the username
	Username string `json:"username"`
	// Password specifies the password
	Password string `json:"password"`
}

// SMTPConfigSpecV2Schema is JSON schema for SMTP configuration
const SMTPConfigSpecV2Schema = `{
  "type": "object",
  "additionalProperties": false,
  "required": ["host"],
  "properties": {
    "host": {"type": "string"},
    "port": {"type": "integer"},
    "username": {"type": "string"},
    "password": {"type": "string"}
  }
}`

// GetSMTPConfigSchema returns SMTP configuration schema for version V2
func GetSMTPConfigSchema() string {
	return fmt.Sprintf(teleservices.V2SchemaTemplate, teleservices.MetadataSchema,
		SMTPConfigSpecV2Schema, "")
}
