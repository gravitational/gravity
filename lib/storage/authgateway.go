/*
Copyright 2019 Gravitational, Inc.

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

	"github.com/gravitational/gravity/lib/utils"

	teleconfig "github.com/gravitational/teleport/lib/config"
	teledefaults "github.com/gravitational/teleport/lib/defaults"
	teleservices "github.com/gravitational/teleport/lib/services"
	teleutils "github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

// AuthGateway defines a resource that controls embedded Teleport configuration.
type AuthGateway interface {
	// Resource provides common resource methods.
	teleservices.Resource
	// CheckAndSetDefaults validates the resource and fills in some defaults.
	CheckAndSetDefaults() error
	// GetMaxConnections returns maximum allowed connections number.
	GetMaxConnections() int64
	// GetMaxUsers returns maximum allowed users number.
	GetMaxUsers() int
	// GetConnectionLimits returns all configured connection limits.
	GetConnectionLimits() *ConnectionLimits
	// SetConnectionLimits sets connection limits on the resource.
	SetConnectionLimits(ConnectionLimits)
	// GetClientIdleTimeout returns idle timeout for SSH sessions.
	GetClientIdleTimeout() *teleservices.Duration
	// SetClientIdleTimeout sets idle timeout setting on the resource.
	SetClientIdleTimeout(teleservices.Duration)
	// GetDisconnectExpiredCert returns whether ongoing SSH session will be
	// disconnected immediately upon certificate expiration.
	GetDisconnectExpiredCert() *teleservices.Bool
	// SetDisconnectExpiredCert sets expired cert policy setting on the resource.
	SetDisconnectExpiredCert(teleservices.Bool)
	// GetAuthentication returns authentication preference setting.
	GetAuthentication() *teleservices.AuthPreferenceSpecV2
	// SetAuthentication sets authentication preference setting on the resource.
	SetAuthentication(teleservices.AuthPreferenceSpecV2)
	// GetAuthPreference returns authentication preference resource.
	GetAuthPreference() (teleservices.AuthPreference, error)
	// SetAuthPreference sets authentication settings from the provided auth preference resource.
	SetAuthPreference(teleservices.AuthPreference) error
	// GetSSHPublicAddrs returns SSH public addresses.
	GetSSHPublicAddrs() []string
	// SetSSHPublicAddrs sets SSH public addresses on the resource.
	SetSSHPublicAddrs([]string)
	// GetKubernetesPublicAddrs returns Kubernetes public addresses.
	GetKubernetesPublicAddrs() []string
	// SetKubernetesPublicAddrs sets Kubernetes public addresses on the resource.
	SetKubernetesPublicAddrs([]string)
	// GetWebPublicAddrs returns web service public addresses.
	GetWebPublicAddrs() []string
	// SetWebPublicAddrs sets web service public addresses on the resource.
	SetWebPublicAddrs([]string)
	// GetPublicAddrs returns public addresses set for all services.
	GetPublicAddrs() []string
	// SetPublicAddrs sets public addresses that apply to all services.
	SetPublicAddrs([]string)
	// ApplyTo applies auth gateway settings to the provided auth gateway resource.
	ApplyTo(AuthGateway)
	// ApplyToTeleportConfig applies auth gateway settings to the provided Teleport config.
	ApplyToTeleportConfig(*teleconfig.FileConfig)
	// PrincipalsChanged returns true if list of principals is different b/w two auth gateway configs.
	PrincipalsChanged(AuthGateway) bool
	// SettingsChanged returns true is connection settings changed b/w two auth gateway configs.
	SettingsChanged(AuthGateway) bool
}

// NewAuthGateway creates a new auth gateway resource for the provided spec.
func NewAuthGateway(spec AuthGatewaySpecV1) AuthGateway {
	return &AuthGatewayV1{
		Kind:    KindAuthGateway,
		Version: teleservices.V1,
		Metadata: teleservices.Metadata{
			Name:      KindAuthGateway,
			Namespace: teledefaults.Namespace,
		},
		Spec: spec,
	}
}

// DefaultAuthGateway returns auth gateway resource with default parameters.
func DefaultAuthGateway() AuthGateway {
	var maxConnections int64 = 10000
	maxUsers := teledefaults.LimiterMaxConcurrentUsers
	clientIdleTimeout := teleservices.NewDuration(0)
	disconnectExpiredCert := teleservices.NewBool(false)
	return NewAuthGateway(AuthGatewaySpecV1{
		ConnectionLimits: &ConnectionLimits{
			MaxConnections: &maxConnections,
			MaxUsers:       &maxUsers,
		},
		ClientIdleTimeout:     &clientIdleTimeout,
		DisconnectExpiredCert: &disconnectExpiredCert,
	})
}

// AuthGatewayV1 defines the auth gateway resource.
type AuthGatewayV1 struct {
	// Kind is the resource kind.
	Kind string `json:"kind"`
	// Version is the resource version.
	Version string `json:"version"`
	// Metadata is the resource metadata.
	Metadata teleservices.Metadata `json:"metadata"`
	// Spec is the resource specification.
	Spec AuthGatewaySpecV1 `json:"spec"`
}

// AuthGatewaySpecV1 defines the auth gateway resource specification.
type AuthGatewaySpecV1 struct {
	// ConnectionLimits describes configured connection limits.
	ConnectionLimits *ConnectionLimits `json:"connection_limits,omitempty"`
	// ClientIdleTimeout is the idle session timeout.
	ClientIdleTimeout *teleservices.Duration `json:"client_idle_timeout,omitempty"`
	// DisconnectExpiredCert is whether expired certificate interrupts session.
	DisconnectExpiredCert *teleservices.Bool `json:"disconnect_expired_cert,omitempty"`
	// Authentication is authentication preferences.
	Authentication *teleservices.AuthPreferenceSpecV2 `json:"authentication,omitempty"`
	// PublicAddr sets public addresses for all Teleport services.
	PublicAddr *[]string `json:"public_addr,omitempty"`
	// SSHPublicAddr sets public addresses for proxy SSH service.
	SSHPublicAddr *[]string `json:"ssh_public_addr,omitempty"`
	// KubernetesPublicAddr sets public addresses for Kubernetes proxy service.
	KubernetesPublicAddr *[]string `json:"kubernetes_public_addr,omitempty"`
	// WebPublicAddr sets public addresses for web service.
	WebPublicAddr *[]string `json:"web_public_addr,omitempty"`
}

// ConnectionLimits defines connection limits setting on auth gateway resource.
type ConnectionLimits struct {
	// MaxConnections is the maximum number of connections to auth/proxy services.
	MaxConnections *int64 `json:"max_connections,omitempty"`
	// MaxUsers is the maximum number of simultaneously connected users.
	MaxUsers *int `json:"max_users,omitempty"`
}

// Check validates the limits settings.
func (l *ConnectionLimits) Check() error {
	if l == nil {
		return nil
	}
	if l.MaxConnections != nil && *l.MaxConnections < 0 {
		return trace.BadParameter("max connections can't be negative")
	}
	if l.MaxUsers != nil && *l.MaxUsers < 0 {
		return trace.BadParameter("max users can't be negative")
	}
	return nil
}

// String returns the object's string representation.
func (l ConnectionLimits) String() string {
	var parts []string
	if l.MaxConnections != nil {
		parts = append(parts, fmt.Sprintf("MaxConnections=%v", *l.MaxConnections))
	}
	if l.MaxUsers != nil {
		parts = append(parts, fmt.Sprintf("MaxUsers=%v", *l.MaxUsers))
	}
	return fmt.Sprintf("ConnectionLimits(%v)", strings.Join(parts, ","))
}

// PrincipalsChanged returns true if a list of principals is different between
// this and provided auth gateway configurations.
//
// "Principals" are hostname parts of public addresses of different services
// that get encoded as SAN extensions (Subject Alternative Names) into their
// respective certificates.
func (gw *AuthGatewayV1) PrincipalsChanged(other AuthGateway) bool {
	if principalsChanged(gw.GetSSHPublicAddrs(), other.GetSSHPublicAddrs()) {
		return true
	}
	if principalsChanged(gw.GetKubernetesPublicAddrs(), other.GetKubernetesPublicAddrs()) {
		return true
	}
	if principalsChanged(gw.GetWebPublicAddrs(), other.GetWebPublicAddrs()) {
		return true
	}
	return false
}

func principalsChanged(old, new []string) bool {
	oldSet := utils.NewStringSetFromSlice(utils.Hosts(old))
	newSet := utils.NewStringSetFromSlice(utils.Hosts(new))
	return len(oldSet.Diff(newSet)) > 0
}

// SettingsChanged returns true if connection settings are different between
// this and provided auth gateway configuration.
func (gw *AuthGatewayV1) SettingsChanged(other AuthGateway) bool {
	if gw.GetMaxConnections() != other.GetMaxConnections() {
		return true
	}
	if gw.GetMaxUsers() != other.GetMaxUsers() {
		return true
	}
	if gw.GetClientIdleTimeout() != other.GetClientIdleTimeout() &&
		(gw.GetClientIdleTimeout() == nil || other.GetClientIdleTimeout() == nil ||
			gw.GetClientIdleTimeout().Value() != other.GetClientIdleTimeout().Value()) {
		return true
	}
	if gw.GetDisconnectExpiredCert() != other.GetDisconnectExpiredCert() &&
		(gw.GetDisconnectExpiredCert() == nil || other.GetDisconnectExpiredCert() == nil ||
			gw.GetDisconnectExpiredCert().Value() != other.GetDisconnectExpiredCert().Value()) {
		return true
	}
	return false
}

// ApplyTo applies auth gateway settings to the provided other auth gateway.
//
// Only non-nil settings are applied.
func (gw *AuthGatewayV1) ApplyTo(other AuthGateway) {
	if v := gw.GetConnectionLimits(); v != nil {
		other.SetConnectionLimits(*v)
	}
	if v := gw.GetClientIdleTimeout(); v != nil {
		other.SetClientIdleTimeout(*v)
	}
	if v := gw.GetDisconnectExpiredCert(); v != nil {
		other.SetDisconnectExpiredCert(*v)
	}
	if v := gw.GetAuthentication(); v != nil {
		other.SetAuthentication(*v)
	}
	if gw.Spec.PublicAddr != nil {
		other.SetSSHPublicAddrs(*gw.Spec.PublicAddr)
		other.SetKubernetesPublicAddrs(*gw.Spec.PublicAddr)
		other.SetWebPublicAddrs(*gw.Spec.PublicAddr)
	}
	if gw.Spec.SSHPublicAddr != nil {
		other.SetSSHPublicAddrs(*gw.Spec.SSHPublicAddr)
	}
	if gw.Spec.KubernetesPublicAddr != nil {
		other.SetKubernetesPublicAddrs(*gw.Spec.KubernetesPublicAddr)
	}
	if gw.Spec.WebPublicAddr != nil {
		other.SetWebPublicAddrs(*gw.Spec.WebPublicAddr)
	}
}

// ApplyToTeleportConfig applies auth gateway settings to the provided config.
func (gw *AuthGatewayV1) ApplyToTeleportConfig(config *teleconfig.FileConfig) {
	if gw.Spec.ConnectionLimits != nil {
		if gw.Spec.ConnectionLimits.MaxConnections != nil {
			config.Global.Limits.MaxConnections = *gw.Spec.ConnectionLimits.MaxConnections
		}
		if gw.Spec.ConnectionLimits.MaxUsers != nil {
			config.Global.Limits.MaxUsers = *gw.Spec.ConnectionLimits.MaxUsers
		}
	}
	if gw.Spec.ClientIdleTimeout != nil {
		config.Auth.ClientIdleTimeout = *gw.Spec.ClientIdleTimeout
	}
	if gw.Spec.DisconnectExpiredCert != nil {
		config.Auth.DisconnectExpiredCert = *gw.Spec.DisconnectExpiredCert
	}
	if gw.Spec.Authentication != nil {
		var u2f *teleconfig.UniversalSecondFactor
		if gw.Spec.Authentication.U2F != nil {
			u2f = &teleconfig.UniversalSecondFactor{
				AppID:  gw.Spec.Authentication.U2F.AppID,
				Facets: gw.Spec.Authentication.U2F.Facets,
			}
		}
		config.Auth.Authentication = &teleconfig.AuthenticationConfig{
			Type:          gw.Spec.Authentication.Type,
			SecondFactor:  gw.Spec.Authentication.SecondFactor,
			ConnectorName: gw.Spec.Authentication.ConnectorName,
			U2F:           u2f,
		}
	}
	// Make sure user-set values take precedence as Teleport may just
	// grab first value from the list, for example when advertising
	// Kubernetes proxy public address.
	config.Auth.PublicAddr = append(gw.GetSSHPublicAddrs(),
		config.Auth.PublicAddr...)
	config.Proxy.SSHPublicAddr = append(gw.GetSSHPublicAddrs(),
		config.Proxy.SSHPublicAddr...)
	config.Proxy.PublicAddr = append(gw.GetWebPublicAddrs(),
		config.Proxy.PublicAddr...)
	config.Proxy.Kube.PublicAddr = append(gw.GetKubernetesPublicAddrs(),
		config.Proxy.Kube.PublicAddr...)
}

// GetMaxConnections returns max connections setting.
func (gw *AuthGatewayV1) GetMaxConnections() int64 {
	if gw.Spec.ConnectionLimits != nil {
		if gw.Spec.ConnectionLimits.MaxConnections != nil {
			return *gw.Spec.ConnectionLimits.MaxConnections
		}
	}
	return 0
}

// GetMaxUsers returns max users setting.
func (gw *AuthGatewayV1) GetMaxUsers() int {
	if gw.Spec.ConnectionLimits != nil {
		if gw.Spec.ConnectionLimits.MaxUsers != nil {
			return *gw.Spec.ConnectionLimits.MaxUsers
		}
	}
	return 0
}

// GetConnectionLimits returns connection limit settings.
func (gw *AuthGatewayV1) GetConnectionLimits() *ConnectionLimits {
	return gw.Spec.ConnectionLimits
}

// SetConnectionLimits sets connection limits settings on the resource.
func (gw *AuthGatewayV1) SetConnectionLimits(value ConnectionLimits) {
	if gw.Spec.ConnectionLimits == nil {
		gw.Spec.ConnectionLimits = &ConnectionLimits{}
	}
	if value.MaxConnections != nil {
		gw.Spec.ConnectionLimits.MaxConnections = value.MaxConnections
	}
	if value.MaxUsers != nil {
		gw.Spec.ConnectionLimits.MaxUsers = value.MaxUsers
	}
}

// GetClientIdleTimeout returns the client idle timeout setting.
func (gw *AuthGatewayV1) GetClientIdleTimeout() *teleservices.Duration {
	return gw.Spec.ClientIdleTimeout
}

// SetClientIdleTimeout sets the client idle timeout setting on the resource.
func (gw *AuthGatewayV1) SetClientIdleTimeout(value teleservices.Duration) {
	gw.Spec.ClientIdleTimeout = &value
}

// GetDisconnectExpiredCert returns the expired certificate policy setting.
func (gw *AuthGatewayV1) GetDisconnectExpiredCert() *teleservices.Bool {
	return gw.Spec.DisconnectExpiredCert
}

// SetDisconnectExpiredCert sets the expired certificate policy setting on the resource.
func (gw *AuthGatewayV1) SetDisconnectExpiredCert(value teleservices.Bool) {
	gw.Spec.DisconnectExpiredCert = &value
}

// GetAuthentication returns authentication preference setting.
func (gw *AuthGatewayV1) GetAuthentication() *teleservices.AuthPreferenceSpecV2 {
	return gw.Spec.Authentication
}

// SetAuthentication sets authentication preference setting on the resource.
func (gw *AuthGatewayV1) SetAuthentication(value teleservices.AuthPreferenceSpecV2) {
	gw.Spec.Authentication = &value
}

// GetAuthPreference returns authentication preference resource.
func (gw *AuthGatewayV1) GetAuthPreference() (teleservices.AuthPreference, error) {
	if gw.Spec.Authentication != nil {
		return teleservices.NewAuthPreference(*gw.Spec.Authentication)
	}
	return nil, trace.NotFound("no authentication preferences set")
}

// SetAuthPreference sets the authentication settings from the provided auth
// preference resource.
func (gw *AuthGatewayV1) SetAuthPreference(authPreference teleservices.AuthPreference) error {
	u2f, err := authPreference.GetU2F()
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	gw.Spec.Authentication = &teleservices.AuthPreferenceSpecV2{
		Type:          authPreference.GetType(),
		SecondFactor:  authPreference.GetSecondFactor(),
		ConnectorName: authPreference.GetConnectorName(),
		U2F:           u2f,
	}
	return nil
}

// GetPublicAddrs returns public addresses for all services.
func (gw *AuthGatewayV1) GetPublicAddrs() []string {
	if gw.Spec.SSHPublicAddr != nil {
		return *gw.Spec.SSHPublicAddr
	}
	return nil
}

// SetPublicAddrs sets public addresses for all services.
func (gw *AuthGatewayV1) SetPublicAddrs(value []string) {
	gw.Spec.PublicAddr = &value
}

// GetSSHPublicAddrs returns public addresses for proxy SSH service.
func (gw *AuthGatewayV1) GetSSHPublicAddrs() []string {
	if gw.Spec.SSHPublicAddr != nil {
		return *gw.Spec.SSHPublicAddr
	}
	if gw.Spec.PublicAddr != nil {
		return *gw.Spec.PublicAddr
	}
	return nil
}

// SetSSHPublicAddrs sets proxy SSH service public addresses.
func (gw *AuthGatewayV1) SetSSHPublicAddrs(value []string) {
	gw.Spec.SSHPublicAddr = &value
}

// GetKubernetesPublicAddrs returns public addresses for Kubernetes proxy service.
func (gw *AuthGatewayV1) GetKubernetesPublicAddrs() []string {
	if gw.Spec.KubernetesPublicAddr != nil {
		return *gw.Spec.KubernetesPublicAddr
	}
	if gw.Spec.PublicAddr != nil {
		return *gw.Spec.PublicAddr
	}
	return nil
}

// SetKubernetesPublicAddrs sets Kubernetes proxy service public addresses.
func (gw *AuthGatewayV1) SetKubernetesPublicAddrs(value []string) {
	gw.Spec.KubernetesPublicAddr = &value
}

// GetWebPublicAddrs returns proxy web service public addresses.
func (gw *AuthGatewayV1) GetWebPublicAddrs() (addrs []string) {
	if gw.Spec.WebPublicAddr != nil {
		return *gw.Spec.WebPublicAddr
	}
	if gw.Spec.PublicAddr != nil {
		return *gw.Spec.PublicAddr
	}
	return nil
}

// SetWebPublicAddrs sets proxy web service public addresses.
func (gw *AuthGatewayV1) SetWebPublicAddrs(value []string) {
	gw.Spec.WebPublicAddr = &value
}

// CheckAndSetDefaults validates the resource and fills in some defaults.
func (gw *AuthGatewayV1) CheckAndSetDefaults() error {
	if gw.Metadata.Name == "" {
		gw.Metadata.Name = KindAuthGateway
	}
	err := gw.Metadata.CheckAndSetDefaults()
	if err != nil {
		return trace.Wrap(err)
	}
	err = gw.Spec.ConnectionLimits.Check()
	if err != nil {
		return trace.Wrap(err)
	}
	if gw.Spec.Authentication != nil {
		auth, err := gw.GetAuthPreference()
		if err != nil && !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		if auth != nil {
			err := auth.CheckAndSetDefaults()
			if err != nil {
				return trace.Wrap(err)
			}
		}
	}
	var addresses []string
	if gw.Spec.PublicAddr != nil {
		addresses = append(addresses, *gw.Spec.PublicAddr...)
	}
	if gw.Spec.SSHPublicAddr != nil {
		addresses = append(addresses, *gw.Spec.SSHPublicAddr...)
	}
	if gw.Spec.KubernetesPublicAddr != nil {
		addresses = append(addresses, *gw.Spec.KubernetesPublicAddr...)
	}
	if gw.Spec.WebPublicAddr != nil {
		addresses = append(addresses, *gw.Spec.WebPublicAddr...)
	}
	err = checkAddrs(addresses)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// checkAddrs validates provided network addresses slice.
func checkAddrs(addrs []string) error {
	_, err := teleutils.Strings(addrs).Addrs(0)
	return trace.Wrap(err)
}

// GetName returns the resource name.
func (gw *AuthGatewayV1) GetName() string {
	return gw.Metadata.Name
}

// SetName sets the resource name.
func (gw *AuthGatewayV1) SetName(name string) {
	gw.Metadata.Name = name
}

// GetMetadata returns the resource metadata.
func (gw *AuthGatewayV1) GetMetadata() teleservices.Metadata {
	return gw.Metadata
}

// SetExpiry sets the resource expiration time.
func (gw *AuthGatewayV1) SetExpiry(expires time.Time) {
	gw.Metadata.SetExpiry(expires)
}

// Expiry returns the resource expiration time.
func (gw *AuthGatewayV1) Expiry() time.Time {
	return gw.Metadata.Expiry()
}

// SetTTL sets the resource TTL.
func (gw *AuthGatewayV1) SetTTL(clock clockwork.Clock, ttl time.Duration) {
	gw.Metadata.SetTTL(clock, ttl)
}

// String returns the object's string representation.
func (gw AuthGatewayV1) String() string {
	var parts []string
	if gw.Spec.ConnectionLimits != nil {
		parts = append(parts, gw.Spec.ConnectionLimits.String())
	}
	if gw.Spec.ClientIdleTimeout != nil {
		parts = append(parts, fmt.Sprintf("ClientIdleTimeout=%s",
			gw.Spec.ClientIdleTimeout.Value()))
	}
	if gw.Spec.DisconnectExpiredCert != nil {
		parts = append(parts, fmt.Sprintf("DisconnectExpiredCert=%v",
			gw.Spec.DisconnectExpiredCert.Value()))
	}
	if gw.Spec.Authentication != nil {
		parts = append(parts, fmt.Sprintf("Authentication(Type=%v, SecondFactor=%v)",
			gw.Spec.Authentication.Type, gw.Spec.Authentication.SecondFactor))
	}
	if gw.Spec.PublicAddr != nil {
		parts = append(parts, fmt.Sprintf("PublicAddr=%v",
			strings.Join(*gw.Spec.PublicAddr, ",")))
	}
	if gw.Spec.SSHPublicAddr != nil {
		parts = append(parts, fmt.Sprintf("SSHPublicAddr=%v",
			strings.Join(*gw.Spec.SSHPublicAddr, ",")))
	}
	if gw.Spec.KubernetesPublicAddr != nil {
		parts = append(parts, fmt.Sprintf("KubernetesPublicAddr=%v",
			strings.Join(*gw.Spec.KubernetesPublicAddr, ",")))
	}
	if gw.Spec.WebPublicAddr != nil {
		parts = append(parts, fmt.Sprintf("WebPublicAddr=%v",
			strings.Join(*gw.Spec.WebPublicAddr, ",")))
	}
	return fmt.Sprintf("AuthGatewayV1(%s)", strings.Join(parts, ","))
}

// UnmarshalAuthGateway unmarshals auth gateway resource from the provided JSON data.
func UnmarshalAuthGateway(data []byte) (AuthGateway, error) {
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
	case teleservices.V1:
		var gw AuthGatewayV1
		err := teleutils.UnmarshalWithSchema(GetAuthGatewaySchema(), &gw, jsonData)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		err = gw.CheckAndSetDefaults()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &gw, nil
	}
	return nil, trace.BadParameter("%v resource version %q is not supported",
		KindAuthGateway, header.Version)
}

// MarshalAuthGateway marshals provided auth gateway resource to JSON.
func MarshalAuthGateway(gw AuthGateway, opts ...teleservices.MarshalOption) ([]byte, error) {
	return json.Marshal(gw)
}

// GetAuthGatewaySchema returns the full auth gateway resource schema.
func GetAuthGatewaySchema() string {
	return fmt.Sprintf(teleservices.V2SchemaTemplate, MetadataSchema,
		AuthGatewaySpecV1Schema, "")
}

// AuthGatewaySpecV1Schema defines the auth gateway spec schema.
var AuthGatewaySpecV1Schema = fmt.Sprintf(`{
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "connection_limits": {
      "type": "object",
      "additionalProperties": false,
      "properties": {
        "max_connections": {"type": "number"},
        "max_users": {"type": "number"}
      }
    },
    "authentication": %v,
    "client_idle_timeout": {"type": "string"},
    "disconnect_expired_cert": {"type": "boolean"},
    "public_addr": {"type": "array", "items": {"type": "string"}},
    "ssh_public_addr": {"type": "array", "items": {"type": "string"}},
    "kubernetes_public_addr": {"type": "array", "items": {"type": "string"}},
    "web_public_addr": {"type": "array", "items": {"type": "string"}}
  }
}`, fmt.Sprintf(teleservices.AuthPreferenceSpecSchemaTemplate, ""))
