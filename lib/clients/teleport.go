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
	"net"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/ops"

	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/sshutils"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
)

// Teleport returns a new teleport client
func Teleport(operator ops.Operator, proxyHost string) (*client.TeleportClient, error) {
	cluster, err := operator.GetLocalSite()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	auth, err := authenticateWithTeleport(operator, cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return client.NewClient(&client.Config{
		Username:        constants.OpsCenterUser,
		AuthMethods:     auth,
		SkipLocalAuth:   true,
		HostLogin:       defaults.SSHUser,
		ProxyHostPort:   proxyHost,
		SiteName:        cluster.Domain,
		HostKeyCallback: sshHostCheckerAcceptAny,
		Env: map[string]string{
			defaults.PathEnv: defaults.PathEnvVal,
		},
	})
}

// TeleportProxy returns a new teleport proxy client
func TeleportProxy(operator ops.Operator, proxyHost string) (*client.ProxyClient, error) {
	teleport, err := Teleport(operator, proxyHost)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return teleport.ConnectToProxy()
}

func authenticateWithTeleport(operator ops.Operator, cluster *ops.Site) ([]ssh.AuthMethod, error) {
	private, public, err := native.New().GenerateKeyPair("")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	response, err := operator.SignSSHKey(ops.SSHSignRequest{
		User:          constants.OpsCenterUser,
		AccountID:     cluster.AccountID,
		PublicKey:     public,
		TTL:           defaults.CertTTL,
		AllowedLogins: []string{defaults.SSHUser},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	signer, err := sshutils.NewSigner(private, response.Cert)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return []ssh.AuthMethod{ssh.PublicKeys(signer)}, nil
}

func sshHostCheckerAcceptAny(hostId string, remote net.Addr, key ssh.PublicKey) error {
	return nil
}
