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

package keyval

import (
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
)

func (b *backend) CreateProvisioningToken(t storage.ProvisioningToken) (*storage.ProvisioningToken, error) {
	if err := t.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	err := b.createVal(b.key(provisioningTokensP, t.Token), t, b.ttl(t.Expires))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &t, nil
}

func (b *backend) DeleteProvisioningToken(token string) error {
	if token == "" {
		return trace.BadParameter("missing token")
	}
	err := b.deleteKey(b.key(provisioningTokensP, token))
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("provisioning token(%v) not found", token)
		}
		return trace.Wrap(err)
	}
	return nil
}

func (b *backend) GetProvisioningToken(token string) (*storage.ProvisioningToken, error) {
	if token == "" {
		return nil, trace.BadParameter("missing token")
	}
	var t storage.ProvisioningToken
	err := b.getVal(b.key(provisioningTokensP, token), &t)
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("provisioning token(%v) not found", token)
		}
		return nil, trace.Wrap(err)
	}
	utils.UTC(&t.Expires)
	return &t, nil
}

func (b *backend) GetOperationProvisioningToken(clusterName, operationID string) (*storage.ProvisioningToken, error) {
	tokens, err := b.getKeys(b.key(provisioningTokensP))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, token := range tokens {
		t, err := b.GetProvisioningToken(token)
		if err != nil {
			if trace.IsNotFound(err) {
				continue
			}
			return nil, trace.Wrap(err)
		}
		if t.OperationID == operationID && t.SiteDomain == clusterName {
			return t, nil
		}
	}
	return nil, trace.NotFound("no provisioning token for cluster %v and operation %v", clusterName, operationID)
}

// GetSiteProvisioningTokens returns install token for site
func (b *backend) GetSiteProvisioningTokens(siteDomain string) ([]storage.ProvisioningToken, error) {
	tokens, err := b.getKeys(b.key(provisioningTokensP))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var out []storage.ProvisioningToken
	for _, token := range tokens {
		t, err := b.GetProvisioningToken(token)
		if err != nil {
			if trace.IsNotFound(err) {
				continue
			}
			return nil, trace.Wrap(err)
		}
		if t.SiteDomain == siteDomain {
			out = append(out, *t)
		}
	}
	return out, nil
}

func (b *backend) CreateInstallToken(t storage.InstallToken) (*storage.InstallToken, error) {
	if err := t.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	err := b.createVal(b.key(installTokensP, t.Token), t, b.ttl(t.Expires))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &t, nil
}

func (b *backend) GetInstallToken(token string) (*storage.InstallToken, error) {
	var t storage.InstallToken
	err := b.getVal(b.key(installTokensP, token), &t)
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("install token(%v) not found", token)
		}
		return nil, trace.Wrap(err)
	}
	return &t, nil
}

func (b *backend) GetInstallTokenByUser(email string) (*storage.InstallToken, error) {
	tokens, err := b.getKeys(b.key(installTokensP))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, token := range tokens {
		t, err := b.GetInstallToken(token)
		if err != nil {
			if trace.IsNotFound(err) {
				continue
			}
			return nil, trace.Wrap(err)
		}
		if t.UserEmail == email {
			return t, nil
		}
	}
	return nil, trace.NotFound("install token for user %v not found", email)
}

// GetInstallTokenForCluster searches for install token by the cluster name
func (b *backend) GetInstallTokenForCluster(name string) (*storage.InstallToken, error) {
	tokens, err := b.getKeys(b.key(installTokensP))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, token := range tokens {
		t, err := b.GetInstallToken(token)
		if err != nil {
			if trace.IsNotFound(err) {
				continue
			}
			return nil, trace.Wrap(err)
		}
		if t.SiteDomain == name {
			return t, nil
		}
	}
	return nil, trace.NotFound("install token for cluster %v not found", name)
}

func (b *backend) UpdateInstallToken(t storage.InstallToken) (*storage.InstallToken, error) {
	if err := t.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	err := b.updateVal(b.key(installTokensP, t.Token), t, b.ttl(t.Expires))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &t, nil
}
