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

	"github.com/gravitational/gravity/lib/defaults"

	teleservices "github.com/gravitational/teleport/lib/services"
	teleutils "github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

// Token contains a set of permissions or settings
type Token interface {
	// Resource provides common resource methods
	teleservices.Resource
	// GetUser returns username the token belongs to
	GetUser() string
	// SetUser sets the token owner
	SetUser(name string)
	// CheckAndSetDefaults makes sure the token is valid
	CheckAndSetDefaults() error
}

// NewToken returns instance of the new token
func NewToken(name string, user string) Token {
	return &TokenV2{
		Kind:    KindToken,
		Version: teleservices.V2,
		Metadata: teleservices.Metadata{
			Name:      name,
			Namespace: defaults.Namespace,
		},
		Spec: TokenSpecV2{
			User: user,
		},
	}
}

// NewTokenFromV1 creates token from API key
func NewTokenFromV1(key APIKey) Token {
	token := NewToken(key.Token, key.UserEmail)
	if !key.Expires.IsZero() {
		token.SetExpiry(key.Expires)
	}
	return token
}

// TokenV2 represents token resource specification
type TokenV2 struct {
	// Kind is a resource kind - always resource
	Kind string `json:"kind"`
	// Version is a resource version
	Version string `json:"version"`
	// Metadata is token metadata
	Metadata teleservices.Metadata `json:"metadata"`
	// Spec contains token specification
	Spec TokenSpecV2 `json:"spec"`
}

// GetName returns token name and is a shortcut for GetMetadata().Name
func (t *TokenV2) GetName() string {
	return t.Metadata.Name
}

// SetName sets token name
func (t *TokenV2) SetName(name string) {
	t.Metadata.Name = name
}

// GetMetadata returns token metadata
func (t *TokenV2) GetMetadata() teleservices.Metadata {
	return t.Metadata
}

// SetExpiry sets token expiration time
func (t *TokenV2) SetExpiry(expires time.Time) {
	t.Metadata.SetExpiry(expires)
}

// Expiry returns token expiration time
func (t *TokenV2) Expiry() time.Time {
	return t.Metadata.Expiry()
}

// SetTTL sets Expires header using realtime clock
func (t *TokenV2) SetTTL(clock clockwork.Clock, ttl time.Duration) {
	t.Metadata.SetTTL(clock, ttl)
}

// SetUser sets token user
func (t *TokenV2) SetUser(username string) {
	t.Spec.User = username
}

// GetUser returns token user
func (t *TokenV2) GetUser() string {
	return t.Spec.User
}

// CheckAndSetDefaults checks validity of all parameters and sets defaults
func (t *TokenV2) CheckAndSetDefaults() error {
	if t.Metadata.Name == "" {
		return trace.BadParameter("missing parameter Name")
	}
	if t.Spec.User == "" {
		return trace.BadParameter("missing parameter User")
	}
	return nil
}

func (t *TokenV2) ToV1() *APIKey {
	return &APIKey{
		Token:     t.Metadata.Name,
		Expires:   t.Metadata.Expiry(),
		UserEmail: t.Spec.User,
	}
}

// GetTokenMarshaler returns token marshaler
func GetTokenMarshaler() TokenMarshaler {
	return &tokenMarshaler{}
}

// TokenMarshaler is interface for marshaling token
type TokenMarshaler interface {
	// UnmarshalToken unmarshals token from JSON
	UnmarshalToken([]byte) (Token, error)
	// MarshalToken marshals token to JSON
	MarshalToken(Token, ...teleservices.MarshalOption) ([]byte, error)
}

type tokenMarshaler struct{}

// UnmarshalToken unmarshals token from JSON
func (*tokenMarshaler) UnmarshalToken(data []byte) (Token, error) {
	if len(data) == 0 {
		return nil, trace.BadParameter("missing token data")
	}
	var h teleservices.ResourceHeader
	err := json.Unmarshal(data, &h)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	switch h.Version {
	case "":
		var apiKey APIKey
		err := json.Unmarshal(data, &apiKey)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		teleutils.UTC(&apiKey.Expires)
		return apiKey.V2(), nil
	case teleservices.V2:
		var t TokenV2
		err := teleutils.UnmarshalWithSchema(GetTokenSchema(), &t, data)
		if err != nil {
			return nil, trace.BadParameter(err.Error())
		}
		//nolint:errcheck
		t.Metadata.CheckAndSetDefaults()
		return &t, nil
	}
	return nil, trace.BadParameter(
		"token resource version %q is not supported", h.Version)
}

// MarshalToken marshals token into JSON
func (*tokenMarshaler) MarshalToken(token Token, opts ...teleservices.MarshalOption) ([]byte, error) {
	return json.Marshal(token)
}

// TokenSpecV2 is token V2 specification
type TokenSpecV2 struct {
	// User is username associated with this token
	User string `json:"user"`
}

// TokenSpecV2Schema is JSON schema for server
//nolint:gosec // not a credential
const TokenSpecV2Schema = `{
  "type": "object",
  "additionalProperties": false,
  "required": ["user"],
  "properties": {
    "user": {"type": "string"}
  }
}`

// GetTokenSchema returns token schema for V2 resource
func GetTokenSchema() string {
	return fmt.Sprintf(teleservices.V2SchemaTemplate, teleservices.MetadataSchema,
		TokenSpecV2Schema, "")
}
