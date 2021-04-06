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

package process

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"

	"github.com/gravitational/teleport/lib/auth"
	teleauth "github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/native"
	teleclient "github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/tlsca"
	teleutils "github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/license/authority"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

func newTeleportProxyService(cfg teleportProxyConfig) (*teleportProxyService, error) {
	proxy := &teleportProxyService{
		cfg:         cfg,
		authClient:  cfg.AuthClient,
		FieldLogger: logrus.WithField(trace.Component, "teleproxy"),
	}
	proxy.Debugf("Creating teleportProxyService with %#v.", cfg)
	if err := proxy.initAuthMethods(); err != nil {
		return nil, trace.Wrap(err)
	}
	proxy.ctx, proxy.cancel = context.WithCancel(context.TODO())
	return proxy, nil
}

type teleportProxyConfig struct {
	// AuthClient is the teleport auth server client
	AuthClient *auth.Client
	// ReverseTunnelAddr is the address of the reverse tunnel server
	ReverseTunnelAddr teleutils.NetAddr
	// WebProxyAddr is the address of the proxy web server
	WebProxyAddr string
	// SSHProxyAddr is the address of the proxy SSH server
	SSHProxyAddr string
	// AuthorityDomain is the teleport's authority domain (gravity cluster name)
	AuthorityDomain string
}

type teleportProxyService struct {
	sync.Mutex
	authClient  *auth.Client
	cfg         teleportProxyConfig
	authMethods []ssh.AuthMethod
	ctx         context.Context
	cancel      context.CancelFunc
	// leaderIP is the IP address of the active planet leader
	leaderIP string
	logrus.FieldLogger
}

func (t *teleportProxyService) setAuthMethods(methods []ssh.AuthMethod) {
	t.Lock()
	defer t.Unlock()
	t.authMethods = methods
}

func (t *teleportProxyService) getAuthMethods() []ssh.AuthMethod {
	t.Lock()
	defer t.Unlock()
	out := make([]ssh.AuthMethod, len(t.authMethods))
	copy(out, t.authMethods)
	return out
}

func (t *teleportProxyService) initAuthMethods() error {
	priv, pub, err := t.generateKeyPair()
	if err != nil {
		return trace.Wrap(err)
	}

	// try to renew cert right away
	if err := t.renewCert(priv, pub); err != nil {
		t.WithError(err).Warnf("Failed to generate certificate for %v.",
			constants.OpsCenterUser)
	}

	go func() {
		ticker := time.NewTicker(defaults.CertRenewPeriod)
		defer ticker.Stop()
		for {
			select {
			case <-t.ctx.Done():
				return
			case <-ticker.C:
				if err := t.renewCert(priv, pub); err != nil {
					t.WithError(err).Warnf("Failed to renew certificate for %v.",
						constants.OpsCenterUser)
				}
			}
		}
	}()
	return nil
}

func (t *teleportProxyService) Close() error {
	t.cancel()
	return nil
}

func (t *teleportProxyService) GenerateUserCert(pub []byte, user string, ttl time.Duration) ([]byte, error) {
	return t.authClient.GenerateUserCert(pub, user, ttl, "")
}

// GetClient returns admin client to local proxy
func (t *teleportProxyService) GetClient() teleauth.ClientI {
	return t.authClient
}

// GetPlanetLeaderIP returns the IP address of the active planet leader node
func (t *teleportProxyService) GetPlanetLeaderIP() (ip string) {
	t.Lock()
	defer t.Unlock()
	return t.leaderIP
}

// ReverseTunnelAddress is the address for
// remote teleport cluster nodes to dial back
func (t *teleportProxyService) ReverseTunnelAddr() string {
	return t.cfg.ReverseTunnelAddr.Addr
}

func (t *teleportProxyService) GetLocalAuthorityDomain() string {
	return t.cfg.AuthorityDomain
}

// GetCertAuthorities returns a list of cert authorities
func (t *teleportProxyService) GetCertAuthorities(caType services.CertAuthType) ([]services.CertAuthority, error) {
	authorities, err := t.authClient.GetCertAuthorities(caType, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out := make([]services.CertAuthority, 0)
	for i := range authorities {
		if authorities[i].GetType() == caType {
			out = append(out, authorities[i])
		}
	}
	return out, nil
}

// GetCertAuthority returns the requested certificate authority
func (t *teleportProxyService) GetCertAuthority(id services.CertAuthID, loadSigningKeys bool) (*authority.TLSKeyPair, error) {
	ca, err := t.authClient.GetCertAuthority(id, loadSigningKeys)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	keyPairs := ca.GetTLSKeyPairs()
	if len(keyPairs) == 0 {
		return nil, trace.NotFound("certificate authority %v does not "+
			"have TLS key pairs", id)
	}
	return &authority.TLSKeyPair{
		KeyPEM:  keyPairs[0].Key,
		CertPEM: keyPairs[0].Cert,
	}, nil
}

// CertificateAuthorities returns a list of certificate
// authorities proxy wants remote teleport sites to trust
func (t *teleportProxyService) CertAuthorities(withPrivateKey bool) ([]services.CertAuthority, error) {
	hostAuthorities, err := t.authClient.GetCertAuthorities(services.HostCA, withPrivateKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	userAuthorities, err := t.authClient.GetCertAuthorities(services.UserCA, withPrivateKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// only collect authorities for this OpsCenter (e.g. opscenter.gravitational.io)
	var authorities []services.CertAuthority
	for _, a := range append(hostAuthorities, userAuthorities...) {
		if a.GetClusterName() == t.cfg.AuthorityDomain {
			authorities = append(authorities, a)
		}
	}
	return authorities, nil
}

// DeleteAuthority deletes teleport authorities for the provided domain name
func (t *teleportProxyService) DeleteAuthority(domainName string) error {
	hostAuthID := services.CertAuthID{
		Type:       services.HostCA,
		DomainName: domainName,
	}
	if err := t.authClient.DeleteCertAuthority(hostAuthID); err != nil {
		return trace.Wrap(err)
	}
	userAuthID := services.CertAuthID{
		Type:       services.UserCA,
		DomainName: domainName,
	}
	if err := t.authClient.DeleteCertAuthority(userAuthID); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// DeleteRemoteCluster deletes remote cluster resource
func (t *teleportProxyService) DeleteRemoteCluster(clusterName string) error {
	return t.authClient.DeleteRemoteCluster(clusterName)
}

// Start trusting certificate authority
func (t *teleportProxyService) TrustCertAuthority(cert services.CertAuthority) error {
	err := t.authClient.UpsertCertAuthority(cert)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (t *teleportProxyService) hostCertChecker() (ssh.HostKeyCallback, error) {
	authorities, err := t.authClient.GetCertAuthorities(services.HostCA, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return func(hostId string, remote net.Addr, key ssh.PublicKey) error {
		cert, ok := key.(*ssh.Certificate)
		if !ok {
			return trace.Errorf("expected certificate")
		}

		for _, certAuthority := range authorities {
			checkers, err := certAuthority.Checkers()
			if err != nil {
				return trace.Wrap(err)
			}
			for _, checker := range checkers {
				t.Infof("Remote host signing key: %v, trusted key: %v.",
					sshutils.Fingerprint(cert.SignatureKey),
					sshutils.Fingerprint(checker))
				if sshutils.KeysEqual(cert.SignatureKey, checker) {
					return nil
				}
			}
		}

		return trace.Errorf("no matching authority found")
	}, nil
}

func (t *teleportProxyService) GetProxyClient(ctx context.Context, siteName string, labels map[string]string) (*teleclient.ProxyClient, error) {
	hostChecker, err := t.hostCertChecker()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsConfig, err := t.getTLSConfig(siteName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	config := &teleclient.Config{
		Username:        constants.OpsCenterUser,
		AuthMethods:     t.getAuthMethods(),
		SkipLocalAuth:   true,
		HostLogin:       defaults.SSHUser,
		WebProxyAddr:    t.cfg.WebProxyAddr,
		SSHProxyAddr:    t.cfg.SSHProxyAddr,
		SiteName:        siteName,
		HostKeyCallback: hostChecker,
		TLS:             tlsConfig,
		Env: map[string]string{
			defaults.PathEnv: defaults.PathEnvVal,
		},
		Labels: labels,
	}

	teleportClient, err := teleclient.NewClient(config)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// query a proxy for server list
	proxyClient, err := teleportClient.ConnectToProxy(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return proxyClient, nil
}

func (t *teleportProxyService) GetServers(ctx context.Context, siteName string, labels map[string]string) ([]services.Server, error) {
	t.Infof("GetServers(%v, %v)", siteName, labels)
	proxyClient, err := t.GetProxyClient(ctx, siteName, labels)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	servers, err := proxyClient.FindServersByLabels(ctx, defaults.Namespace, labels)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return servers, nil
}

func (t *teleportProxyService) GetServerCount(ctx context.Context, siteName string) (int, error) {
	servers, err := t.GetServers(ctx, siteName, nil)
	if err != nil && !trace.IsNotFound(err) {
		return 0, trace.Wrap(err)
	}
	return len(servers), nil
}

func (t *teleportProxyService) ExecuteCommand(ctx context.Context, siteName, nodeAddr, command string, stdout, stderr io.Writer) error {
	t.Infof("ExecuteCommand(%v, %v, %v)", siteName, nodeAddr, command)
	hostChecker, err := t.hostCertChecker()
	if err != nil {
		return trace.Wrap(err)
	}
	targetHost, targetPortS, err := net.SplitHostPort(nodeAddr)
	if err != nil {
		return trace.Wrap(err, fmt.Sprintf("bad target node address: %v", nodeAddr))
	}
	targetPort, err := strconv.Atoi(targetPortS)
	if err != nil {
		return trace.Wrap(err, fmt.Sprintf("bad target node address: %v", nodeAddr))
	}
	tlsConfig, err := t.getTLSConfig(siteName)
	if err != nil {
		return trace.Wrap(err)
	}
	proxyClient, err := teleclient.NewClient(&teleclient.Config{
		Username:        constants.OpsCenterUser,
		AuthMethods:     t.getAuthMethods(),
		SkipLocalAuth:   true,
		HostLogin:       defaults.SSHUser,
		WebProxyAddr:    t.cfg.WebProxyAddr,
		SSHProxyAddr:    t.cfg.SSHProxyAddr,
		HostPort:        targetPort,
		Host:            targetHost,
		Stdout:          stdout,
		Stderr:          stderr,
		SiteName:        siteName,
		HostKeyCallback: hostChecker,
		TLS:             tlsConfig,
		Env: map[string]string{
			defaults.PathEnv: defaults.PathEnvVal,
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(proxyClient.SSH(ctx, strings.Split(command, " "), false))
}

// getTLSConfig builds a TLS client config using certificate signed by
// the host's certificate authority of the specified domain
func (t *teleportProxyService) getTLSConfig(clusterName string) (*tls.Config, error) {
	ca, err := t.authClient.GetCertAuthority(services.CertAuthID{
		Type:       services.HostCA,
		DomainName: clusterName,
	}, true)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsAuthority, err := ca.TLSCA()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	privateKey, publicKey, err := t.generateKeyPair()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cryptoPublicKey, err := sshutils.CryptoPublicKey(publicKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	identity := &tlsca.Identity{
		Username: constants.OpsCenterUser,
		Groups:   []string{defaults.SystemAccountOrg},
	}
	subject, err := identity.Subject()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cert, err := tlsAuthority.GenerateCertificate(
		tlsca.CertificateRequest{
			Clock:     clockwork.NewRealClock(),
			PublicKey: cryptoPublicKey,
			Subject:   subject,
			NotAfter:  time.Now().UTC().Add(defaults.CertTTL),
		})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clientKey := &teleclient.Key{
		TLSCert: cert,
		Priv:    privateKey,
		TrustedCA: auth.AuthoritiesToTrustedCerts(
			[]services.CertAuthority{ca}),
	}
	return clientKey.ClientTLSConfig()
}

func (t *teleportProxyService) generateKeyPair() (privateKey []byte, publicKey []byte, err error) {
	keygen, err := native.New()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return keygen.GenerateKeyPair("")
}

func (t *teleportProxyService) renewCert(privateKey, publicKey []byte) error {
	cert, err := t.authClient.GenerateUserCert(publicKey, constants.OpsCenterUser, defaults.CertTTL, "")
	if err != nil {
		return trace.Wrap(err)
	}
	signer, err := sshutils.NewSigner(privateKey, cert)
	if err != nil {
		return trace.Wrap(err)
	}
	t.Debugf("Renewed certificate for %v.", constants.OpsCenterUser)
	t.setAuthMethods([]ssh.AuthMethod{ssh.PublicKeys(signer)})
	return nil
}
