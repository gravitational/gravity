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

package localenv

import (
	"fmt"
	"net"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/ops/opsclient"

	"github.com/gravitational/teleport/lib/auth/native"
	teleclient "github.com/gravitational/teleport/lib/client"
	teledefaults "github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/trace"

	"golang.org/x/crypto/ssh"
)

func (env *LocalEnvironment) TeleportClient(proxyHost string) (*teleclient.TeleportClient, error) {
	operator, err := env.SiteOperator()
	if err != nil {
		return nil, trace.Wrap(err, "failed to get cluster operator service")
	}

	site, err := operator.GetLocalSite()
	if err != nil {
		return nil, trace.Wrap(err, "failed to get local cluster")
	}

	auth, err := authenticateWithTeleport(operator, site)
	if err != nil {
		return nil, trace.Wrap(err, "failed to authenticate with teleport")
	}

	config := teleclient.Config{
		Username:        constants.OpsCenterUser,
		AuthMethods:     auth,
		SkipLocalAuth:   true,
		HostLogin:       defaults.SSHUser,
		ProxyHostPort:   proxyHost,
		SiteName:        site.Domain,
		HostKeyCallback: sshHostCheckerAcceptAny,
		Env: map[string]string{
			defaults.PathEnv: defaults.PathEnvVal,
		},
	}

	teleportClient, err := teleclient.NewClient(&config)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return teleportClient, nil
}

func (env *LocalEnvironment) proxyHostPort() string {
	// TODO: what is the right way ?
	return fmt.Sprintf("%v:%v,%v",
		constants.Localhost,
		defaults.GravityServicePort,
		teledefaults.AuthListenPort)
}

func sshHostCheckerAcceptAny(hostId string, remote net.Addr, key ssh.PublicKey) error {
	return nil
}

func authenticateWithTeleport(operator *opsclient.Client, site *ops.Site) ([]ssh.AuthMethod, error) {
	certAuthority := native.New()
	priv, pub, err := certAuthority.GenerateKeyPair("")
	if err != nil {
		return nil, trace.Wrap(err, "generate keypair")
	}

	resp, err := operator.SignSSHKey(ops.SSHSignRequest{
		User:          constants.OpsCenterUser,
		AccountID:     site.AccountID,
		PublicKey:     pub,
		TTL:           defaults.CertTTL,
		AllowedLogins: []string{defaults.SSHUser},
	})
	if err != nil {
		return nil, trace.Wrap(err, "sign SSH key")
	}

	signer, err := sshutils.NewSigner(priv, resp.Cert)
	if err != nil {
		return nil, trace.Wrap(err, "signer")
	}

	return []ssh.AuthMethod{ssh.PublicKeys(signer)}, nil
}
