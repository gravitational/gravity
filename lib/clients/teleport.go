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

package clients

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"strconv"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/client"
	teledefaults "github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/sshutils"

	"github.com/cloudflare/cfssl/csr"
	"github.com/gravitational/license/authority"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
)

// Teleport returns a new teleport client
func Teleport(operator ops.Operator, proxyHost, clusterName string) (*client.TeleportClient, error) {
	auth, tlsConfig, err := authenticateWithTeleport(operator)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	host, webPort, sshPort, err := utils.ParseProxyAddr(
		proxyHost,
		strconv.Itoa(defaults.GravityServicePort),
		strconv.Itoa(teledefaults.SSHProxyListenPort))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return client.NewClient(&client.Config{
		Username:        constants.OpsCenterUser,
		AuthMethods:     auth,
		SkipLocalAuth:   true,
		HostLogin:       defaults.SSHUser,
		WebProxyAddr:    fmt.Sprintf("%v:%v", host, webPort),
		SSHProxyAddr:    fmt.Sprintf("%v:%v", host, sshPort),
		SiteName:        clusterName,
		HostKeyCallback: sshHostCheckerAcceptAny,
		TLS:             tlsConfig,
		Env: map[string]string{
			defaults.PathEnv: defaults.PathEnvVal,
		},
	})
}

// TeleportProxy returns a new teleport proxy client
func TeleportProxy(ctx context.Context, operator ops.Operator, proxyHost, clusterName string) (*client.ProxyClient, error) {
	teleport, err := Teleport(operator, proxyHost, clusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return teleport.ConnectToProxy(ctx)
}

// TeleportAuth returns a new teleport auth server client
func TeleportAuth(ctx context.Context, operator ops.Operator, proxyHost, clusterName string) (*AuthClient, error) {
	teleport, err := Teleport(operator, proxyHost, clusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Teleport auth server prior to version 3.0 didn't support TLS so
	// do not attempt to connect if there's no TLS information, otherwise
	// we'll get panic.
	if teleport.TLS == nil {
		return nil, trace.BadParameter("auth server %v does not support TLS", proxyHost)
	}
	proxyClient, err := teleport.ConnectToProxy(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	authClient, err := proxyClient.ConnectToCurrentCluster(ctx, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &AuthClient{
		ClientI:     authClient,
		proxyClient: proxyClient,
	}, nil
}

// Close closes this client
func (r *AuthClient) Close() error {
	return r.proxyClient.Close()
}

// AuthClient represents the client to the auth server
type AuthClient struct {
	io.Closer
	auth.ClientI
	proxyClient *client.ProxyClient
}

func authenticateWithTeleport(operator ops.Operator) ([]ssh.AuthMethod, *tls.Config, error) {
	keygen, err := native.New()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	private, public, err := keygen.GenerateKeyPair("")
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	csr, key, err := authority.GenerateCSR(csr.CertificateRequest{
		CN:    constants.OpsCenterUser,
		Names: []csr.Name{{O: defaults.SystemAccountOrg}},
	}, private)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	response, err := operator.SignSSHKey(ops.SSHSignRequest{
		User:          constants.OpsCenterUser,
		AccountID:     defaults.SystemAccountID,
		PublicKey:     public,
		TTL:           defaults.CertTTL,
		AllowedLogins: []string{defaults.SSHUser},
		CSR:           csr,
	})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	signer, err := sshutils.NewSigner(private, response.Cert)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	var tlsConfig *tls.Config
	// older clusters do not return TLS certificate
	if response.TLSCert != nil {
		tlsConfig, err = (&client.Key{
			TLSCert: response.TLSCert,
			Priv:    key,
			TrustedCA: []auth.TrustedCerts{{
				TLSCertificates: [][]byte{response.CACert},
			}},
		}).ClientTLSConfig()
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
	}
	return []ssh.AuthMethod{ssh.PublicKeys(signer)}, tlsConfig, nil
}

func sshHostCheckerAcceptAny(hostId string, remote net.Addr, key ssh.PublicKey) error {
	return nil
}
