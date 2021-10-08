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

package testutils

import (
	"context"
	"io"
	"net"
	"net/url"
	"time"

	"github.com/gravitational/gravity/lib/docker"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/events"
	teletunnel "github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/trace"

	"github.com/tstranex/u2f"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

type FakeImageService struct{}

func (s *FakeImageService) Sync(dir string) ([]docker.TagSpec, error) {
	return nil, nil
}

func (s *FakeImageService) Wrap(image string) string {
	return ""
}

func (s *FakeImageService) Unwrap(image string) string {
	return ""
}

// FakeReverseTunnel allows to configure the state of teleport's reverse tunnel in tests,
// for example to set up available sites and their states
type FakeReverseTunnel struct {
	Sites []FakeRemoteSite
}

func (s *FakeReverseTunnel) GetSites() []teletunnel.RemoteSite {
	var sites []teletunnel.RemoteSite
	for _, site := range s.Sites {
		sites = append(sites, site)
	}
	return sites
}

func (s *FakeReverseTunnel) GetSite(name string) (teletunnel.RemoteSite, error) {
	for _, site := range s.Sites {
		if site.GetName() == name {
			return site, nil
		}
	}
	return nil, trace.NotFound("site %v not found", name)
}

func (s *FakeReverseTunnel) RemoveSite(name string) error {
	for i, site := range s.Sites {
		if site.GetName() == name {
			s.Sites = append(s.Sites[:i], s.Sites[i+1:]...)
			return nil
		}
	}
	return trace.NotFound("site %v not found", name)
}

func (*FakeReverseTunnel) Start() error {
	return nil
}

func (*FakeReverseTunnel) Close() error {
	return nil
}

func (*FakeReverseTunnel) Wait() {}

func (*FakeReverseTunnel) Shutdown(context.Context) error {
	return nil
}

// FakeRemoteSite represents a teleport's remote site that is used to configure the fake
// reverse tunnel in tests
type FakeRemoteSite struct {
	Name          string
	Status        string
	LastConnected time.Time
}

func (s FakeRemoteSite) ConnectToServer(addr, user string, auth []ssh.AuthMethod) (*ssh.Client, error) {
	return nil, nil
}

func (s FakeRemoteSite) DialServer(addr string) (net.Conn, error) {
	return nil, nil
}

func (s FakeRemoteSite) DialAgentServer() (net.Conn, error) {
	return nil, nil
}

func (s FakeRemoteSite) DialAuthServer() (net.Conn, error) {
	return nil, nil
}

func (s FakeRemoteSite) Dial(net.Addr, net.Addr, agent.Agent) (net.Conn, error) {
	return nil, nil
}

func (s FakeRemoteSite) DialTCP(net.Addr, net.Addr) (net.Conn, error) {
	return nil, nil
}

func (s FakeRemoteSite) GetLastConnected() time.Time {
	return s.LastConnected
}

func (s FakeRemoteSite) GetName() string {
	return s.Name
}

func (s FakeRemoteSite) GetStatus() string {
	return s.Status
}

func (s FakeRemoteSite) GetClient() (auth.ClientI, error) {
	return nil, nil
}

func (s FakeRemoteSite) CachingAccessPoint() (auth.AccessPoint, error) {
	return nil, nil
}

func (s FakeRemoteSite) GetTunnelsCount() int {
	return 0
}

// AuthClient implements Teleport's auth.ClientI interface
type AuthClient struct {
	storage.Backend
}

func (s *AuthClient) SetAuth(auth.ClientI) {
}

func (s *AuthClient) DeleteGithubConnector(id string) error {
	return nil
}

func (s *AuthClient) ValidateGithubAuthCallback(q url.Values) (*auth.GithubAuthResponse, error) {
	return nil, nil
}

func (s *AuthClient) CreateGithubAuthRequest(req services.GithubAuthRequest) (*services.GithubAuthRequest, error) {
	return nil, nil
}

func (s *AuthClient) ChangePassword(req services.ChangePasswordReq) error {
	return nil
}

func (s *AuthClient) UpsertPassword(user string, password []byte) error {
	return nil
}
func (s *AuthClient) UpsertOIDCConnector(connector services.OIDCConnector) error {
	return nil
}
func (s *AuthClient) GetOIDCConnector(id string, withSecrets bool) (services.OIDCConnector, error) {
	return nil, nil
}
func (s *AuthClient) GetOIDCConnectors(withSecrets bool) ([]services.OIDCConnector, error) {
	return nil, nil
}
func (s *AuthClient) DeleteOIDCConnector(connectorID string) error {
	return nil
}
func (s *AuthClient) CreateOIDCAuthRequest(req services.OIDCAuthRequest) (*services.OIDCAuthRequest, error) {
	return nil, nil
}
func (s *AuthClient) ValidateOIDCAuthCallback(q url.Values) (*auth.OIDCAuthResponse, error) {
	return nil, nil
}
func (s *AuthClient) CreateSAMLConnector(connector services.SAMLConnector) error {
	return nil
}
func (s *AuthClient) UpsertSAMLConnector(connector services.SAMLConnector) error {
	return nil
}
func (s *AuthClient) GetSAMLConnector(id string, withSecrets bool) (services.SAMLConnector, error) {
	return nil, nil
}
func (s *AuthClient) GetSAMLConnectors(withSecrets bool) ([]services.SAMLConnector, error) {
	return nil, nil
}
func (s *AuthClient) DeleteSAMLConnector(connectorID string) error {
	return nil
}
func (s *AuthClient) CreateSAMLAuthRequest(req services.SAMLAuthRequest) (*services.SAMLAuthRequest, error) {
	return nil, nil
}
func (s *AuthClient) ValidateSAMLResponse(re string) (*auth.SAMLAuthResponse, error) {
	return nil, nil
}
func (s *AuthClient) GetU2FSignRequest(user string, password []byte) (*u2f.SignRequest, error) {
	return nil, nil
}
func (s *AuthClient) GetSignupU2FRegisterRequest(token string) (*u2f.RegisterRequest, error) {
	return nil, nil
}
func (s *AuthClient) CreateUserWithU2FToken(token string, password string, u2fRegisterResponse u2f.RegisterResponse) (services.WebSession, error) {
	return nil, nil
}
func (s *AuthClient) PreAuthenticatedSignIn(user string) (services.WebSession, error) {
	return nil, nil
}
func (s *AuthClient) GetUser(name string) (services.User, error) {
	return nil, nil
}
func (s *AuthClient) UpsertUser(user services.User) error {
	return nil
}
func (s *AuthClient) DeleteUser(user string) error {
	return nil
}
func (s *AuthClient) GetUsers() ([]services.User, error) {
	return nil, nil
}
func (s *AuthClient) CheckPassword(user string, password []byte, otpToken string) error {
	return nil
}
func (s *AuthClient) SignIn(user string, password []byte) (services.WebSession, error) {
	return nil, nil
}
func (s *AuthClient) CreateUserWithOTP(token, password, otpToken string) (services.WebSession, error) {
	return nil, nil
}
func (s *AuthClient) CreateUserWithoutOTP(token string, password string) (services.WebSession, error) {
	return nil, nil
}
func (s *AuthClient) GenerateToken(req auth.GenerateTokenRequest) (string, error) {
	return "", nil
}
func (s *AuthClient) GenerateKeyPair(pass string) ([]byte, []byte, error) {
	return nil, nil, nil
}
func (s *AuthClient) GenerateHostCert(key []byte, hostID, nodeName string, principals []string, clusterName string, roles teleport.Roles, ttl time.Duration) ([]byte, error) {
	return nil, nil
}
func (s *AuthClient) GenerateUserCert(key []byte, user string, ttl time.Duration, compatibility string) ([]byte, error) {
	return nil, nil
}
func (s *AuthClient) GenerateUserCerts(key []byte, user string, ttl time.Duration, compatibility string) ([]byte, []byte, error) {
	return nil, nil, nil
}
func (s *AuthClient) GetSignupTokenData(token string) (user string, otpQRCode []byte, e error) {
	return "", nil, nil
}
func (s *AuthClient) CreateSignupToken(user services.UserV1, ttl time.Duration) (string, error) {
	return "", nil
}
func (s *AuthClient) GetTokens() (tokens []services.ProvisionToken, err error) {
	return nil, nil
}
func (s *AuthClient) GetToken(token string) (*services.ProvisionToken, error) {
	return nil, nil
}
func (s *AuthClient) DeleteToken(token string) error {
	return nil
}
func (s *AuthClient) RegisterUsingToken(req auth.RegisterUsingTokenRequest) (*auth.PackedKeys, error) {
	return nil, nil
}
func (s *AuthClient) RegisterNewAuthServer(token string) error {
	return nil
}
func (s *AuthClient) EmitAuditEvent(event events.Event, fields events.EventFields) error {
	return nil
}
func (s *AuthClient) PostSessionSlice(events.SessionSlice) error {
	return nil
}
func (s *AuthClient) PostSessionChunk(namespace string, sid session.ID, reader io.Reader) error {
	return nil
}
func (s *AuthClient) GetSessionChunk(namespace string, sid session.ID, offsetBytes, maxBytes int) ([]byte, error) {
	return nil, nil
}
func (s *AuthClient) GetSessionEvents(namespace string, sid session.ID, after int, b bool) ([]events.EventFields, error) {
	return nil, nil
}
func (s *AuthClient) SearchEvents(fromUTC, toUTC time.Time, query string, i int) ([]events.EventFields, error) {
	return nil, nil
}
func (s *AuthClient) SearchSessionEvents(fromUTC time.Time, toUTC time.Time, i int) ([]events.EventFields, error) {
	return nil, nil
}
func (s *AuthClient) WaitForDelivery(context.Context) error {
	return nil
}
func (s *AuthClient) Close() error {
	return nil
}
func (s *AuthClient) GetWebSessionInfo(user, sid string) (services.WebSession, error) {
	return nil, nil
}
func (s *AuthClient) ExtendWebSession(user, prevSID string) (services.WebSession, error) {
	return nil, nil
}
func (s *AuthClient) CreateWebSession(user string) (services.WebSession, error) {
	return nil, nil
}
func (s *AuthClient) DeleteWebSession(user, sid string) error {
	return nil
}
func (s *AuthClient) GetSessions(namespace string) ([]session.Session, error) {
	return nil, nil
}
func (s *AuthClient) GetSession(namespace string, id session.ID) (*session.Session, error) {
	return nil, nil
}
func (s *AuthClient) CreateSession(sess session.Session) error {
	return nil
}
func (s *AuthClient) UpdateSession(req session.UpdateRequest) error {
	return nil
}
func (s *AuthClient) DeleteSession(namespace string, id session.ID) error {
	return nil
}
func (s *AuthClient) GetClusterName() (services.ClusterName, error) {
	return nil, nil
}
func (s *AuthClient) SetClusterName(name services.ClusterName) error {
	return nil
}

func (s *AuthClient) GetClusterConfig() (services.ClusterConfig, error) {
	return nil, nil
}

func (s *AuthClient) SetClusterConfig(clusterConfig services.ClusterConfig) error {
	return nil
}

func (s *AuthClient) GetStaticTokens() (services.StaticTokens, error) {
	return nil, nil
}
func (s *AuthClient) SetStaticTokens(tokens services.StaticTokens) error {
	return nil
}
func (s *AuthClient) GetAuthPreference() (services.AuthPreference, error) {
	return nil, nil
}
func (s *AuthClient) SetAuthPreference(pref services.AuthPreference) error {
	return nil
}
func (s *AuthClient) ValidateTrustedCluster(req *auth.ValidateTrustedClusterRequest) (*auth.ValidateTrustedClusterResponse, error) {
	return nil, nil
}
func (s *AuthClient) GetDomainName() (string, error) {
	return "", nil
}
func (s *AuthClient) GenerateServerKeys(auth.GenerateServerKeysRequest) (*auth.PackedKeys, error) {
	return nil, nil
}
func (s *AuthClient) AuthenticateWebUser(req auth.AuthenticateUserRequest) (services.WebSession, error) {
	return nil, nil
}
func (s *AuthClient) AuthenticateSSHUser(req auth.AuthenticateSSHRequest) (*auth.SSHLoginResponse, error) {
	return nil, nil
}
func (s *AuthClient) ProcessKubeCSR(req auth.KubeCSR) (*auth.KubeCSRResponse, error) {
	return nil, nil
}
func (s *AuthClient) RotateCertAuthority(req auth.RotateRequest) error {
	return nil
}
func (s *AuthClient) RotateExternalCertAuthority(ca services.CertAuthority) error {
	return nil
}
func (s *AuthClient) UploadSessionRecording(r events.SessionRecording) error {
	return nil
}
func (s *AuthClient) GetClusterCACert() (*auth.LocalCAResponse, error) {
	return nil, nil
}
