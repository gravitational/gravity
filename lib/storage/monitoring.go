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

	teleservices "github.com/gravitational/teleport/lib/services"
	teleutils "github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

// Alert describes a monitoring alert
type Alert interface {
	// Resource provides common resource methods
	teleservices.Resource
	// CheckAndSetDefaults that the object is valid
	CheckAndSetDefaults() error
	// GetFormula returns the kapacitor formula
	GetFormula() string
}

// AlertV2 defines a monitoring alert
type AlertV2 struct {
	// Metadata is resource metadata
	teleservices.Metadata `json:"metadata"`
	// Kind is a resource kind
	Kind string `json:"kind"`
	// Version is a resource version
	Version string `json:"version"`
	// Spec defines the monitoring alert
	Spec AlertSpecV2 `json:"spec"`
}

// GetFormula returns alert's kapacitor formula
func (r *AlertV2) GetFormula() string {
	return r.Spec.Formula
}

// CheckAndSetDefaults checks validity of all parameters and sets defaults
func (r *AlertV2) CheckAndSetDefaults() error {
	if r.Spec.Formula == "" {
		return trace.BadParameter("missing parameter Formula")
	}

	if r.Metadata.Name == "" {
		return trace.BadParameter("missing parameter Name")
	}

	return nil
}

// UnmarshalAlert unmarshals an alert from JSON
func UnmarshalAlert(data []byte) (*AlertV2, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("empty alert")
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
		var alert AlertV2
		err := teleutils.UnmarshalWithSchema(GetAlertSchema(), &alert, jsonData)
		if err != nil {
			return nil, trace.BadParameter(err.Error())
		}
		alert.Metadata.CheckAndSetDefaults()
		return &alert, nil
	}
	return nil, trace.BadParameter(
		"%v resource version %q is not supported", KindAlert, hdr.Version)
}

// MarshalAlert marshals an alert into JSON
func MarshalAlert(alert Alert, opts ...teleservices.MarshalOption) ([]byte, error) {
	return json.Marshal(alert)
}

// AlertSpecV2 defines a monitoring alert
type AlertSpecV2 struct {
	// Formula defines a formula for kapacitor
	Formula string `json:"formula"`
}

// AlertSpecV2Schema is JSON schema for a monitoring alert
const AlertSpecV2Schema = `{
  "type": "object",
  "additionalProperties": false,
  "required": ["formula"],
  "properties": {
    "formula": {"type": "string"}
  }
}`

// GetAlertSchema returns alert schema for version V2
func GetAlertSchema() string {
	return fmt.Sprintf(teleservices.V2SchemaTemplate, teleservices.MetadataSchema,
		AlertSpecV2Schema, "")
}

// AlertTarget describes a monitoring alert target
type AlertTarget interface {
	// Resource provides common resource methods
	teleservices.Resource
	// CheckAndSetDefaults that the object is valid
	CheckAndSetDefaults() error
	// GetEmail returns the recipient's email
	GetEmail() string
}

// AlertTargetV2 defines a monitoring alert target
type AlertTargetV2 struct {
	// Metadata is resource metadata
	teleservices.Metadata `json:"metadata"`
	// Kind is a resource kind
	Kind string `json:"kind"`
	// Version is a resource version
	Version string `json:"version"`
	// Spec defines the alert target
	Spec AlertTargetSpecV2 `json:"spec"`
}

// GetEmail returns recipient's email
func (r *AlertTargetV2) GetEmail() string {
	return r.Spec.Email
}

// CheckAndSetDefaults checks validity of all parameters and sets defaults
func (r *AlertTargetV2) CheckAndSetDefaults() error {
	if r.Spec.Email == "" {
		return trace.BadParameter("missing parameter Email")
	}

	return nil
}

// UnmarshalAlertTarget unmarshals an alert target from JSON
func UnmarshalAlertTarget(data []byte) (*AlertTargetV2, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("empty alert target")
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
		var target AlertTargetV2
		err := teleutils.UnmarshalWithSchema(GetAlertTargetSchema(), &target, jsonData)
		if err != nil {
			return nil, trace.BadParameter(err.Error())
		}
		target.Metadata.CheckAndSetDefaults()
		return &target, nil
	}
	return nil, trace.BadParameter(
		"%v resource version %q is not supported", KindAlertTarget, hdr.Version)
}

// MarshalAlertTarget marshals an alert target into JSON
func MarshalAlertTarget(target AlertTarget, opts ...teleservices.MarshalOption) ([]byte, error) {
	return json.Marshal(target)
}

// AlertTargetSpecV2 defines a monitoring alert target
type AlertTargetSpecV2 struct {
	// Email specifies recipient's email
	Email string `json:"email"`
}

// AlertTargetSpecV2Schema is JSON schema for a monitoring alert target
const AlertTargetSpecV2Schema = `{
  "type": "object",
  "additionalProperties": false,
  "required": ["email"],
  "properties": {
    "email": {"type": "string"}
  }
}`

// GetAlertTargetSchema returns alert target schema for version V2
func GetAlertTargetSchema() string {
	return fmt.Sprintf(teleservices.V2SchemaTemplate, teleservices.MetadataSchema,
		AlertTargetSpecV2Schema, "")
}
