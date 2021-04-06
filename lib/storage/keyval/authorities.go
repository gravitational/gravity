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
	teleservices "github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
)

// CreateCertAuthority updates or inserts a new certificate authority
func (b *backend) CreateCertAuthority(ca teleservices.CertAuthority) error {
	data, err := teleservices.GetCertAuthorityMarshaler().MarshalCertAuthority(ca)
	if err != nil {
		return trace.Wrap(err)
	}
	err = b.createValBytes(b.key(authoritiesP, string(ca.GetType()), ca.GetClusterName()), data, b.ttl(ca.Expiry()))
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// UpsertCertAuthority updates or inserts a new certificate authority
func (b *backend) UpsertCertAuthority(ca teleservices.CertAuthority) error {
	data, err := teleservices.GetCertAuthorityMarshaler().MarshalCertAuthority(ca)
	if err != nil {
		return trace.Wrap(err)
	}
	err = b.upsertValBytes(b.key(authoritiesP, string(ca.GetType()), ca.GetClusterName()), data, b.ttl(ca.Expiry()))
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// DeleteCertAuthority deletes particular certificate authority
func (b *backend) DeleteCertAuthority(id teleservices.CertAuthID) error {
	err := b.deleteKey(b.key(authoritiesP, string(id.Type), id.DomainName))
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("authority(%v, %v) not found", id.DomainName, id.Type)
		}
	}
	return nil
}

// DeleteAllCertAuthorities deletes all cert authorities
func (b *backend) DeleteAllCertAuthorities(authType teleservices.CertAuthType) error {
	err := b.deleteDir(b.key(authoritiesP, string(authType)))
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}
	return nil
}

// GetCertAuthority returns certificate authority by given id. Parameter loadSigningKeys
// controls if signing keys are loaded
func (b *backend) GetCertAuthority(id teleservices.CertAuthID, loadSigningKeys bool, opts ...teleservices.MarshalOption) (teleservices.CertAuthority, error) {
	data, err := b.getValBytes(b.key(authoritiesP, string(id.Type), id.DomainName))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("authority(%v, %v) not found", id.DomainName, id.Type)
		}
	}
	ca, err := teleservices.GetCertAuthorityMarshaler().UnmarshalCertAuthority(data)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if !loadSigningKeys {
		if err := ca.SetSigningKeys(nil); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return ca, nil
}

// GetCertAuthorities returns a list of authorities of a given type
// loadSigningKeys controls whether signing keys should be loaded or not
func (b *backend) GetCertAuthorities(caType teleservices.CertAuthType, loadSigningKeys bool, opts ...teleservices.MarshalOption) ([]teleservices.CertAuthority, error) {
	if err := caType.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	var out []teleservices.CertAuthority
	domainNames, err := b.getKeys(b.key(authoritiesP, string(caType)))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, domain := range domainNames {
		auth, err := b.GetCertAuthority(teleservices.CertAuthID{DomainName: domain, Type: caType}, loadSigningKeys)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out = append(out, auth)
	}
	return out, nil
}

// ActivateCertAuthority moves a CertAuthority from the deactivated list to
// the normal list.
func (b *backend) ActivateCertAuthority(id teleservices.CertAuthID) error {
	data, err := b.getValBytes(b.key(authoritiesP, deactivatedP, string(id.Type), id.DomainName))
	if err != nil {
		return trace.BadParameter("can not activate CertAuthority which has not been deactivated: %v: %v", id, err)
	}

	certAuthority, err := teleservices.GetCertAuthorityMarshaler().UnmarshalCertAuthority(data)
	if err != nil {
		return trace.Wrap(err)
	}

	err = b.UpsertCertAuthority(certAuthority)
	if err != nil {
		return trace.Wrap(err)
	}

	err = b.deleteKey(b.key(authoritiesP, deactivatedP, string(id.Type), id.DomainName))
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// DeactivateCertAuthority moves a CertAuthority from the normal list to
// the deactivated list.
func (b *backend) DeactivateCertAuthority(id teleservices.CertAuthID) error {
	certAuthority, err := b.GetCertAuthority(id, true)
	if err != nil {
		return trace.BadParameter("can not deactivate CertAuthority which does not exist: %v: %v", id, err)
	}

	data, err := teleservices.GetCertAuthorityMarshaler().MarshalCertAuthority(certAuthority)
	if err != nil {
		return trace.Wrap(err)
	}
	ttl := b.ttl(certAuthority.Expiry())

	err = b.upsertValBytes(b.key(authoritiesP, deactivatedP, string(id.Type), id.DomainName), data, ttl)
	if err != nil {
		return trace.Wrap(err)
	}

	err = b.DeleteCertAuthority(id)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// CompareAndSwapCertAuthority updates the cert authority value if the existing
// value matches existing parameter
func (b *backend) CompareAndSwapCertAuthority(new, existing teleservices.CertAuthority) error {
	if err := new.Check(); err != nil {
		return trace.Wrap(err)
	}
	newData, err := teleservices.GetCertAuthorityMarshaler().MarshalCertAuthority(new)
	if err != nil {
		return trace.Wrap(err)
	}
	existingData, err := teleservices.GetCertAuthorityMarshaler().MarshalCertAuthority(existing)
	if err != nil {
		return trace.Wrap(err)
	}
	ttl := b.ttl(new.Expiry())
	var outData []byte
	err = b.compareAndSwapBytes(b.key(authoritiesP, string(new.GetType()), new.GetClusterName()), newData, existingData, &outData, ttl)
	if err != nil {
		if trace.IsCompareFailed(err) {
			return trace.CompareFailed("cluster %v settings have been updated, try again", new.GetClusterName())
		}
		return trace.Wrap(err)
	}
	return nil
}
