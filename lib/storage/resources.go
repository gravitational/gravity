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
	"io"
	"strings"

	"github.com/ghodss/yaml"
	teleservices "github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
	serializer "k8s.io/apimachinery/pkg/runtime/serializer/json"
)

const (
	// KindCluster is a resource kind for gravity clusters
	KindCluster = "cluster"
	// KindRepository represents repositories
	KindRepository = "repository"
	// KindApp represents applications and packages
	KindApp = "app"
	// KindObject represents binary object BLOB
	KindObject = "object"
	// KindAccount represents account resource
	KindAccount = "account"
	// KindToken is security token (e.g. API Key)
	KindToken = "token"
	// KindLicense represents Gravity software license
	KindLicense = "license"
	// VerbRegister is used to allow registering new clusters
	// within an Ops Center
	VerbRegister = "register"
	// VerbConnect is used to allow users to connect to clusters
	VerbConnect = "connect"
	// VerbReadSecrets is used to allow reading secrets
	VerbReadSecrets = "readsecrets"
	// KindLogForwarder is log forwarder resource kind
	KindLogForwarder = "logforwarder"
	// KindTLSKeyPair is a TLS key pair
	KindTLSKeyPair = "tlskeypair"
	// KindSMTPConfig defines the monitoring SMTP configuration resource type
	KindSMTPConfig = "smtp"
	// KindAlert defines the monitoring alert resource type
	KindAlert = "alert"
	// KindAlertTarget defines the monitoring alert target resource type
	KindAlertTarget = "alerttarget"
	// KindSystemInfo defines the system information resource
	KindSystemInfo = "systeminfo"
	// KindEndpoints defines the Ops Center endpoints resource type
	KindEndpoints = "endpoints"
	// KindAuthGateway defines the auth gateway resource type
	KindAuthGateway = "authgateway"
	// KindRuntimeEnvironment defines the resource that manages cluster environment variables
	KindRuntimeEnvironment = "runtimeenvironment"
	// KindClusterConfiguration defines the resource that manages cluster configuration
	KindClusterConfiguration = "clusterconfiguration"
	// KindPersistentStorage is the resource for managing persistent storage in the cluster
	KindPersistentStorage = "persistentstorage"
	// KindOperation is the cluster operation resource type.
	KindOperation = "operation"
	// KindRelease defines the application release resource type
	KindRelease = "release"
	// KindInvite defines the user invite token.
	KindInvite = "invite"
)

// CanonicalKind translates the specified kind to canonical form.
// Returns the kind unmodified if it did not match any known resource
func CanonicalKind(kind string) string {
	switch strings.ToLower(kind) {
	case teleservices.KindGithubConnector:
		return teleservices.KindGithubConnector
	case teleservices.KindAuthConnector, "auth":
		return teleservices.KindAuthConnector
	case teleservices.KindUser, "users":
		return teleservices.KindUser
	case KindToken, "tokens":
		return KindToken
	case KindLogForwarder, "logforwarders":
		return KindLogForwarder
	case KindTLSKeyPair, "tlskeypairs", "tls":
		return KindTLSKeyPair
	case teleservices.KindClusterAuthPreference, "authpreference", "cap":
		return teleservices.KindClusterAuthPreference
	case KindSMTPConfig, "smtps":
		return KindSMTPConfig
	case KindAlert, "alerts":
		return KindAlert
	case KindAlertTarget, "alerttargets":
		return KindAlertTarget
	case KindRuntimeEnvironment, "environment", "env":
		return KindRuntimeEnvironment
	case KindClusterConfiguration, "config":
		return KindClusterConfiguration
	case KindPersistentStorage, "storage", "ps":
		return KindPersistentStorage
	case KindAuthGateway, "gw":
		return KindAuthGateway
	case KindOperation, "operations", "op", "ops":
		return KindOperation
	}
	return kind
}

// UnknownResource represents an unparsed resource with an interpreted ResourceHeader.
// The embedded resource can either be a Kubernetes or a Gravity resource.
// The struct implements both json.Marshaler/json.Unmarshaler
type UnknownResource struct {
	// ResourceHeader describes the resource by providing the metadata common to all resources
	teleservices.ResourceHeader
	// Raw is the unparsed resource data.
	Raw json.RawMessage `json:",inline"`
}

// UnmarshalJSON consumes the specified data as a binary blob w/o interpreting it
func (r *UnknownResource) UnmarshalJSON(data []byte) (err error) {
	if err = json.Unmarshal(data, &r.ResourceHeader); err != nil {
		return trace.Wrap(err)
	}
	if err = r.Raw.UnmarshalJSON(data); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// MarshalJSON returns the raw message
func (r UnknownResource) MarshalJSON() ([]byte, error) {
	return r.Raw.MarshalJSON()
}

// Encode YAML-encodes the specified list of resources into w
func Encode(resources []UnknownResource, w io.Writer) error {
	w = serializer.YAMLFramer.NewFrameWriter(w)
	for _, resource := range resources {
		jsonBytes, err := json.Marshal(resource)
		if err != nil {
			return trace.Wrap(err)
		}
		data, err := yaml.JSONToYAML(jsonBytes)
		if err != nil {
			return trace.Wrap(err)
		}
		_, err = w.Write(data)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// SupportedGravityResources is a list of resources supported by
// "gravity resource create/get" subcommands
var SupportedGravityResources = []string{
	teleservices.KindClusterAuthPreference,
	teleservices.KindGithubConnector,
	teleservices.KindAuthConnector,
	teleservices.KindUser,
	KindToken,
	KindLogForwarder,
	KindSMTPConfig,
	KindAlert,
	KindAlertTarget,
	KindTLSKeyPair,
	KindAuthGateway,
	KindRuntimeEnvironment,
	KindClusterConfiguration,
	KindPersistentStorage,
	KindOperation,
}

// SupportedGravityResourcesToRemove is a list of resources supported by
// "gravity resource rm" subcommand
var SupportedGravityResourcesToRemove = []string{
	teleservices.KindGithubConnector,
	teleservices.KindUser,
	KindToken,
	KindLogForwarder,
	KindSMTPConfig,
	KindAlert,
	KindAlertTarget,
	KindTLSKeyPair,
	KindRuntimeEnvironment,
	KindClusterConfiguration,
}

// MetadataSchema is a copy of teleport/lib/services.MetadataSchema but with
// optional 'name' property because some Gravity resources do not require it
const MetadataSchema = `{
  "type": "object",
  "additionalProperties": false,
  "default": {},
  "properties": {
    "name": {"type": "string"},
    "namespace": {"type": "string"},
    "description": {"type": "string"},
    "expires": {"type": "string"},
    "id": {"type": "integer"},
    "labels": {
      "type": "object",
      "additionalProperties": false,
      "patternProperties": {
         "^[a-zA-Z/.0-9_*-]+$":  {"type": "string"}
      }
    }
  }
}`
