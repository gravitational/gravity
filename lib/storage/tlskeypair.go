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
	"time"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"

	cfsslerrors "github.com/cloudflare/cfssl/errors"
	cfsslhelpers "github.com/cloudflare/cfssl/helpers"
	teleservices "github.com/gravitational/teleport/lib/services"
	teleutils "github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

// TLSKeyPair describes a TLS key pair resource that can be checked for validity and queried.
type TLSKeyPair interface {
	// Resource provides common resource methods
	teleservices.Resource
	// CheckAndSetDefaults makes sure the TLS keypair is valid
	CheckAndSetDefaults() error
	// GetCert returns certificate and optional certificate chain
	GetCert() string
	// GetPrivateKey returns private key
	GetPrivateKey() string
}

// NewTLSKeyPair creates new TLS key pair from cert and private key
func NewTLSKeyPair(cert, privateKey []byte) TLSKeyPair {
	return &TLSKeyPairV2{
		Kind:    KindTLSKeyPair,
		Version: teleservices.V2,
		Metadata: teleservices.Metadata{
			Name:      constants.KeyPair,
			Namespace: defaults.Namespace,
		},
		Spec: TLSKeyPairSpecV2{
			Cert:       string(cert),
			PrivateKey: string(privateKey),
		},
	}
}

// TLSKeyPairV2 represents TLS key pair specification
type TLSKeyPairV2 struct {
	// Kind is a resource kind - always tlskeypair
	Kind string `json:"kind"`
	// Version is a resource version
	Version string `json:"version"`
	// Metadata is TLS keypair metadata
	Metadata teleservices.Metadata `json:"metadata"`
	// Spec contains TLS keypair specification
	Spec TLSKeyPairSpecV2 `json:"spec"`
}

// GetName returns TLS keypair name and is a shortcut for GetMetadata().Name
func (t *TLSKeyPairV2) GetName() string {
	return t.Metadata.Name
}

// SetName sets TLS keypair name
func (t *TLSKeyPairV2) SetName(name string) {
	t.Metadata.Name = name
}

// GetMetadata returns TLS keypair metadata
func (t *TLSKeyPairV2) GetMetadata() teleservices.Metadata {
	return t.Metadata
}

// SetExpiry sets TLS keypair expiration time
func (t *TLSKeyPairV2) SetExpiry(expires time.Time) {
	t.Metadata.SetExpiry(expires)
}

// Expiry returns TLS keypair expiration time
func (t *TLSKeyPairV2) Expiry() time.Time {
	return t.Metadata.Expiry()
}

// SetTTL sets Expires header using realtime clock
func (t *TLSKeyPairV2) SetTTL(clock clockwork.Clock, ttl time.Duration) {
	t.Metadata.SetTTL(clock, ttl)
}

// GetPrivateKey returns private key
func (t *TLSKeyPairV2) GetPrivateKey() string {
	return t.Spec.PrivateKey
}

// GetCert returns certificate
func (t *TLSKeyPairV2) GetCert() string {
	return t.Spec.Cert
}

// CheckAndSetDefaults checks validity of all parameters and sets defaults
func (t *TLSKeyPairV2) CheckAndSetDefaults() error {
	// For now we only have one name
	if t.Metadata.Name == "" {
		t.Metadata.Name = constants.KeyPair
	}

	if t.Spec.Cert == "" {
		return trace.BadParameter("missing parameter 'cert'")
	}

	if t.Spec.PrivateKey == "" {
		return trace.BadParameter("missing parameter 'private_key'")
	}

	_, err := cfsslhelpers.ParseCertificatesPEM([]byte(t.Spec.Cert))
	if err != nil {
		if cfsslerr, ok := err.(*cfsslerrors.Error); ok {
			return trace.BadParameter(cfsslerr.Message)
		}
		return trace.Wrap(err, "failed to parse certificate, expected PEM formatted block")
	}
	_, err = cfsslhelpers.ParsePrivateKeyPEM([]byte(t.Spec.PrivateKey))
	if err != nil {
		if cfsslerr, ok := err.(*cfsslerrors.Error); ok {
			return trace.BadParameter(cfsslerr.Message)
		}
		return trace.Wrap(err, "failed to parse private key, expected PEM formatted block")
	}

	return nil
}

// UnmarshalTLSKeyPair unmarshals TLS keypair from JSON
func UnmarshalTLSKeyPair(data []byte) (TLSKeyPair, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing cluster data")
	}
	jsonData, err := teleutils.ToJSON(data)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var h teleservices.ResourceHeader
	err = json.Unmarshal(jsonData, &h)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch h.Version {
	case teleservices.V2:
		var t TLSKeyPairV2
		err := teleutils.UnmarshalWithSchema(GetTLSKeyPairSchema(), &t, jsonData)
		if err != nil {
			return nil, trace.BadParameter(err.Error())
		}
		//nolint:errcheck
		t.Metadata.CheckAndSetDefaults()
		return &t, nil
	}
	return nil, trace.BadParameter(
		"%v resource version %q is not supported", KindTLSKeyPair, h.Version)
}

// MarshalTLSKeyPair marshals TLS keypair into JSON
func MarshalTLSKeyPair(keyPair TLSKeyPair, opts ...teleservices.MarshalOption) ([]byte, error) {
	return json.Marshal(keyPair)
}

// TLSKeyPairSpecV2 is TLS keypair V2 specification
type TLSKeyPairSpecV2 struct {
	// Cert is a PEM encoded certificate chain
	// including intermediaries
	Cert string `json:"cert"`
	// PrivateKey is PEM encoded private key
	PrivateKey string `json:"private_key"`
}

// TLSKeyPairSpecV2Schema is JSON schema for TLS keypair
const TLSKeyPairSpecV2Schema = `{
  "type": "object",
  "additionalProperties": false,
  "required": ["cert", "private_key"],
  "properties": {
    "cert": {"type": "string"},
    "private_key": {"type": "string"}
  }
}`

// GetTLSKeyPairSchema returns TLS keypair schema for V2 resource
func GetTLSKeyPairSchema() string {
	return fmt.Sprintf(teleservices.V2SchemaTemplate, teleservices.MetadataSchema,
		TLSKeyPairSpecV2Schema, "")
}
