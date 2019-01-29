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
	// GetAuthPreference returns authentication preference setting.
	GetAuthPreference() teleservices.AuthPreference
	// SetAuthPreference sets authentication preference setting.
	SetAuthPreference(teleservices.AuthPreference)
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
	// PrincipalsChanged returns true if list of principals is difference b/w two auth gateway configs.
	PrincipalsChanged(AuthGateway) bool
	// SettingsChanged returns true is connection settings changed b/w two auth gateway configs.
	SettingsChanged(AuthGateway) bool
}

// NewAuthGateway creates a new auth gateway resource for the provided spec.
func NewAuthGateway(spec AuthGatewaySpecV2) AuthGateway {
	return &AuthGatewayV2{
		Kind:    KindAuthGateway,
		Version: teleservices.V2,
		Metadata: teleservices.Metadata{
			Name:      KindAuthGateway,
			Namespace: teledefaults.Namespace,
		},
		Spec: spec,
	}
}

// DefaultAuthGateway returns auth gateway resource with default parameters.
func DefaultAuthGateway() AuthGateway {
	var maxConnections int64 = teledefaults.LimiterMaxConnections
	maxUsers := teledefaults.LimiterMaxConcurrentUsers
	clientIdleTimeout := teleservices.NewDuration(0)
	disconnectExpiredCert := teleservices.NewBool(false)
	return NewAuthGateway(AuthGatewaySpecV2{
		ConnectionLimits: &ConnectionLimits{
			MaxConnections: &maxConnections,
			MaxUsers:       &maxUsers,
		},
		ClientIdleTimeout:     &clientIdleTimeout,
		DisconnectExpiredCert: &disconnectExpiredCert,
	})
}

// AuthGatewayV2 defines the auth gateway resource.
type AuthGatewayV2 struct {
	// Kind is the resource kind.
	Kind string `json:"kind"`
	// Version is the resource version.
	Version string `json:"version"`
	// Metadata is the resource metadata.
	Metadata teleservices.Metadata `json:"metadata"`
	// Spec is the resource specification.
	Spec AuthGatewaySpecV2 `json:"spec"`
}

// AuthGatewaySpecV2 defines the auth gateway resource specification.
type AuthGatewaySpecV2 struct {
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
func (gw *AuthGatewayV2) PrincipalsChanged(other AuthGateway) bool {
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
func (gw *AuthGatewayV2) SettingsChanged(other AuthGateway) bool {
	if gw.GetMaxConnections() != other.GetMaxConnections() {
		return true
	}
	if gw.GetMaxUsers() != other.GetMaxUsers() {
		return true
	}
	if gw.GetClientIdleTimeout() != nil && other.GetClientIdleTimeout() == nil ||
		gw.GetClientIdleTimeout() == nil && other.GetClientIdleTimeout() != nil ||
		gw.GetClientIdleTimeout() != nil && other.GetClientIdleTimeout() != nil &&
			gw.GetClientIdleTimeout().Value() != other.GetClientIdleTimeout().Value() {
		return true
	}
	if gw.GetDisconnectExpiredCert() != nil && other.GetDisconnectExpiredCert() == nil ||
		gw.GetDisconnectExpiredCert() == nil && other.GetDisconnectExpiredCert() != nil ||
		gw.GetDisconnectExpiredCert() != nil && other.GetDisconnectExpiredCert() != nil &&
			gw.GetDisconnectExpiredCert().Value() != other.GetDisconnectExpiredCert().Value() {
		return true
	}
	return false
}

// ApplyTo applies auth gateway settings to the provided other auth gateway.
//
// Only non-nil settings are applied.
func (gw *AuthGatewayV2) ApplyTo(other AuthGateway) {
	if v := gw.GetConnectionLimits(); v != nil {
		other.SetConnectionLimits(*v)
	}
	if v := gw.GetClientIdleTimeout(); v != nil {
		other.SetClientIdleTimeout(*v)
	}
	if v := gw.GetDisconnectExpiredCert(); v != nil {
		other.SetDisconnectExpiredCert(*v)
	}
	if v := gw.GetAuthPreference(); v != nil {
		other.SetAuthPreference(v)
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

// Apply applies auth gateway settings to the provided config.
func (gw *AuthGatewayV2) ApplyToTeleportConfig(config *teleconfig.FileConfig) {
	if gw.Spec.ConnectionLimits != nil {
		config.Global.Limits.MaxConnections = *gw.Spec.ConnectionLimits.MaxConnections
		config.Global.Limits.MaxUsers = *gw.Spec.ConnectionLimits.MaxUsers
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
	config.Auth.PublicAddr = append(config.Auth.PublicAddr,
		gw.GetSSHPublicAddrs()...)
	config.Proxy.SSHPublicAddr = append(config.Proxy.SSHPublicAddr,
		gw.GetSSHPublicAddrs()...)
	config.Proxy.PublicAddr = append(config.Proxy.PublicAddr,
		gw.GetWebPublicAddrs()...)
	config.Proxy.Kube.PublicAddr = append(config.Proxy.Kube.PublicAddr,
		gw.GetKubernetesPublicAddrs()...)
}

// GetMaxConnections returns max connections setting.
func (gw *AuthGatewayV2) GetMaxConnections() int64 {
	if gw.Spec.ConnectionLimits != nil {
		if gw.Spec.ConnectionLimits.MaxConnections != nil {
			return *gw.Spec.ConnectionLimits.MaxConnections
		}
	}
	return 0
}

// GetMaxUsers returns max users setting.
func (gw *AuthGatewayV2) GetMaxUsers() int {
	if gw.Spec.ConnectionLimits != nil {
		if gw.Spec.ConnectionLimits.MaxUsers != nil {
			return *gw.Spec.ConnectionLimits.MaxUsers
		}
	}
	return 0
}

// GetConnectionLimits returns connection limit settings.
func (gw *AuthGatewayV2) GetConnectionLimits() *ConnectionLimits {
	return gw.Spec.ConnectionLimits
}

// SetConnectionLimits sets connection limits settings on the resource.
func (gw *AuthGatewayV2) SetConnectionLimits(value ConnectionLimits) {
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
func (gw *AuthGatewayV2) GetClientIdleTimeout() *teleservices.Duration {
	return gw.Spec.ClientIdleTimeout
}

// SetClientIdleTimeout sets the client idle timeout setting on the resource.
func (gw *AuthGatewayV2) SetClientIdleTimeout(value teleservices.Duration) {
	gw.Spec.ClientIdleTimeout = &value
}

// GetDisconnectExpiredCert returns the expired certificate policy setting.
func (gw *AuthGatewayV2) GetDisconnectExpiredCert() *teleservices.Bool {
	return gw.Spec.DisconnectExpiredCert
}

// SetDisconnectExpiredCert sets the expired certificate policy setting on the resource.
func (gw *AuthGatewayV2) SetDisconnectExpiredCert(value teleservices.Bool) {
	gw.Spec.DisconnectExpiredCert = &value
}

// GetAuthPreference returns the authentication preference settings.
func (gw *AuthGatewayV2) GetAuthPreference() teleservices.AuthPreference {
	if gw.Spec.Authentication != nil {
		// It never returns an error.
		authPreference, _ := teleservices.NewAuthPreference(*gw.Spec.Authentication)
		return authPreference
	}
	return nil
}

// SetAuthPreference sets the authentication preference settings on the resource.
func (gw *AuthGatewayV2) SetAuthPreference(authPreference teleservices.AuthPreference) {
	// It only returns trace.NotFound() error.
	u2f, _ := authPreference.GetU2F()
	gw.Spec.Authentication = &teleservices.AuthPreferenceSpecV2{
		Type:          authPreference.GetType(),
		SecondFactor:  authPreference.GetSecondFactor(),
		ConnectorName: authPreference.GetConnectorName(),
		U2F:           u2f,
	}
}

// GetPublicAddrs returns public addresses for all services.
func (gw *AuthGatewayV2) GetPublicAddrs() []string {
	if gw.Spec.SSHPublicAddr != nil {
		return *gw.Spec.SSHPublicAddr
	}
	return nil
}

// SetPublicAddrs sets public addresses for all services.
func (gw *AuthGatewayV2) SetPublicAddrs(value []string) {
	gw.Spec.PublicAddr = &value
}

// GetSSHPublicAddrs returns public addresses for proxy SSH service.
func (gw *AuthGatewayV2) GetSSHPublicAddrs() []string {
	if gw.Spec.SSHPublicAddr != nil {
		return *gw.Spec.SSHPublicAddr
	}
	if gw.Spec.PublicAddr != nil {
		return *gw.Spec.PublicAddr
	}
	return nil
}

// SetSSHPublicAddrs sets proxy SSH service public addresses.
func (gw *AuthGatewayV2) SetSSHPublicAddrs(value []string) {
	gw.Spec.SSHPublicAddr = &value
}

// GetKubernetesPublicAddrs returns public addresses for Kubernetes proxy service.
func (gw *AuthGatewayV2) GetKubernetesPublicAddrs() []string {
	if gw.Spec.KubernetesPublicAddr != nil {
		return *gw.Spec.KubernetesPublicAddr
	}
	if gw.Spec.PublicAddr != nil {
		return *gw.Spec.PublicAddr
	}
	return nil
}

// SetKubernetesPublicAddrs sets Kubernetes proxy service public addresses.
func (gw *AuthGatewayV2) SetKubernetesPublicAddrs(value []string) {
	gw.Spec.KubernetesPublicAddr = &value
}

// GetWebPublicAddrs returns proxy web service public addresses.
func (gw *AuthGatewayV2) GetWebPublicAddrs() (addrs []string) {
	if gw.Spec.WebPublicAddr != nil {
		return *gw.Spec.WebPublicAddr
	}
	if gw.Spec.PublicAddr != nil {
		return *gw.Spec.PublicAddr
	}
	return nil
}

// SetWebPublicAddrs sets proxy web service public addresses.
func (gw *AuthGatewayV2) SetWebPublicAddrs(value []string) {
	gw.Spec.WebPublicAddr = &value
}

// CheckAndSetDefaults validates the resource and fills in some defaults.
func (gw *AuthGatewayV2) CheckAndSetDefaults() error {
	if gw.Metadata.Name == "" {
		gw.Metadata.Name = KindAuthGateway
	}
	err := gw.Metadata.CheckAndSetDefaults()
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetName returns the resource name.
func (gw *AuthGatewayV2) GetName() string {
	return gw.Metadata.Name
}

// SetName sets the resource name.
func (gw *AuthGatewayV2) SetName(name string) {
	gw.Metadata.Name = name
}

// GetMetadata returns the resource metadata.
func (gw *AuthGatewayV2) GetMetadata() teleservices.Metadata {
	return gw.Metadata
}

// SetExpiry sets the resource expiration time.
func (gw *AuthGatewayV2) SetExpiry(expires time.Time) {
	gw.Metadata.SetExpiry(expires)
}

// Expires returns the resource expiration time.
func (gw *AuthGatewayV2) Expiry() time.Time {
	return gw.Metadata.Expiry()
}

// SetTTL sets the resource TTL.
func (gw *AuthGatewayV2) SetTTL(clock clockwork.Clock, ttl time.Duration) {
	gw.Metadata.SetTTL(clock, ttl)
}

// String returns the object's string representation.
func (gw AuthGatewayV2) String() string {
	var parts []string
	if gw.Spec.ConnectionLimits != nil {
		parts = append(parts, fmt.Sprintf("%s", *gw.Spec.ConnectionLimits))
	}
	if gw.Spec.ClientIdleTimeout != nil {
		parts = append(parts, fmt.Sprintf("ClientIdleTimeout=%s", gw.Spec.ClientIdleTimeout.Value()))
	}
	if gw.Spec.DisconnectExpiredCert != nil {
		parts = append(parts, fmt.Sprintf("DisconnectExpiredCert=%v", gw.Spec.DisconnectExpiredCert.Value()))
	}
	if gw.Spec.Authentication != nil {
		parts = append(parts, fmt.Sprintf("%s", gw.GetAuthPreference()))
	}
	if gw.Spec.PublicAddr != nil {
		parts = append(parts, fmt.Sprintf("PublicAddr=%v", strings.Join(*gw.Spec.PublicAddr, ",")))
	}
	if gw.Spec.SSHPublicAddr != nil {
		parts = append(parts, fmt.Sprintf("SSHPublicAddr=%v", strings.Join(*gw.Spec.SSHPublicAddr, ",")))
	}
	if gw.Spec.KubernetesPublicAddr != nil {
		parts = append(parts, fmt.Sprintf("KubernetesPublicAddr=%v", strings.Join(*gw.Spec.KubernetesPublicAddr, ",")))
	}
	if gw.Spec.WebPublicAddr != nil {
		parts = append(parts, fmt.Sprintf("WebPublicAddr=%v", strings.Join(*gw.Spec.WebPublicAddr, ",")))
	}
	return fmt.Sprintf("AuthGatewayV2(%s)", strings.Join(parts, ","))
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
	case teleservices.V2:
		var gw AuthGatewayV2
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
		AuthGatewaySpecV2Schema, "")
}

// AuthGatewaySpecV2Schema defines the auth gateway spec schema.
var AuthGatewaySpecV2Schema = fmt.Sprintf(`{
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
