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
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"

	log "github.com/sirupsen/logrus"
	"github.com/gravitational/teleport/lib/auth"
	teleauth "github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/native"
	teleclient "github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
)

func newTeleportProxyService(cfg teleportProxyConfig) (*teleportProxyService, error) {
	proxy := &teleportProxyService{
		cfg:        cfg,
		authClient: cfg.AuthClient,
	}
	proxy.ctx, proxy.cancel = context.WithCancel(context.TODO())
	certGeneratedCh := make(chan struct{})
	go proxy.initAuthMethods(certGeneratedCh)
	<-certGeneratedCh
	return proxy, nil
}

type teleportProxyConfig struct {
	AuthClient        *auth.TunClient
	ReverseTunnelAddr utils.NetAddr
	ProxyHost         string
	AuthorityDomain   string
}

type teleportProxyService struct {
	sync.Mutex
	authClient  *auth.TunClient
	cfg         teleportProxyConfig
	authMethods []ssh.AuthMethod
	ctx         context.Context
	cancel      context.CancelFunc
	// leaderIP is the IP address of the active planet leader
	leaderIP string
}

func (t *teleportProxyService) authServers() ([]utils.NetAddr, error) {
	servers, err := t.authClient.GetAuthServers()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	authServers := make([]utils.NetAddr, 0, len(servers))
	for _, server := range servers {
		serverAddr, err := utils.ParseAddr(server.GetAddr())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		authServers = append(authServers, *serverAddr)
	}
	return authServers, nil
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

func (t *teleportProxyService) initAuthMethods(certGeneratedCh chan<- struct{}) error {
	certAuthority := native.New()
	priv, pub, err := certAuthority.GenerateKeyPair("")
	if err != nil {
		return trace.Wrap(err)
	}

	renewCert := func() error {
		cert, err := t.authClient.GenerateUserCert(pub, constants.OpsCenterUser, defaults.CertTTL, "")
		if err != nil {
			return trace.Wrap(err)
		}
		signer, err := sshutils.NewSigner(priv, cert)
		if err != nil {
			return trace.Wrap(err)
		}
		log.Debugf("[TELEPORT] generated certificate for %v", constants.OpsCenterUser)
		t.setAuthMethods([]ssh.AuthMethod{ssh.PublicKeys(signer)})
		return nil
	}

	// try to renew cert right away
	if err := renewCert(); err != nil {
		log.Warningf("failed to generate cert: %v", trace.DebugReport(err))
	}
	// Notify the listener that the certificate has been renewed
	close(certGeneratedCh)

	ticker := time.NewTicker(defaults.CertRenewPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-t.ctx.Done():
			return nil
		case <-ticker.C:
			if err := renewCert(); err != nil {
				log.Warningf("failed to generate cert: %v", trace.DebugReport(err))
			}
		}
	}
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

// Start trusting certificate authority
func (t *teleportProxyService) TrustCertAuthority(cert services.CertAuthority) error {
	err := t.authClient.UpsertCertAuthority(cert)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (t *teleportProxyService) hostCertChecker() (teleclient.HostKeyCallback, error) {
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
				log.Infof("remote host signing key: %v, trusted key: %v", sshutils.Fingerprint(cert.SignatureKey), sshutils.Fingerprint(checker))
				if sshutils.KeysEqual(cert.SignatureKey, checker) {
					return nil
				}
			}
		}

		return trace.Errorf("no matching authority found")
	}, nil
}

func (t *teleportProxyService) GetProxyClient(ctx context.Context, siteName string, labels map[string]string) (*teleclient.ProxyClient, error) {
	log.Infof("GetServers(%v, %v)", siteName, labels)
	hostChecker, err := t.hostCertChecker()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	config := &teleclient.Config{
		Username:        constants.OpsCenterUser,
		AuthMethods:     t.getAuthMethods(),
		SkipLocalAuth:   true,
		HostLogin:       defaults.SSHUser,
		ProxyHostPort:   t.cfg.ProxyHost,
		SiteName:        siteName,
		HostKeyCallback: hostChecker,
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
	proxyClient, err := teleportClient.ConnectToProxy()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return proxyClient, nil
}

func (t *teleportProxyService) GetServers(ctx context.Context, siteName string, labels map[string]string) ([]services.Server, error) {
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

func (t *teleportProxyService) ExecuteCommand(ctx context.Context, siteName, nodeAddr, command string, out io.Writer) error {
	log.Infof("ExecuteCommand(%v, %v, %v)", siteName, nodeAddr, command)
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
	proxyClient, err := teleclient.NewClient(&teleclient.Config{
		Username:        constants.OpsCenterUser,
		AuthMethods:     t.getAuthMethods(),
		SkipLocalAuth:   true,
		HostLogin:       defaults.SSHUser,
		ProxyHostPort:   t.cfg.ProxyHost,
		HostPort:        targetPort,
		Host:            targetHost,
		Stdout:          out,
		SiteName:        siteName,
		HostKeyCallback: hostChecker,
		Env: map[string]string{
			defaults.PathEnv: defaults.PathEnvVal,
		},
	})
	return trace.Wrap(proxyClient.SSH(ctx, strings.Split(command, " "), false))
}
