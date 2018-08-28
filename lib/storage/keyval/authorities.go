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
func (b *backend) GetCertAuthority(id teleservices.CertAuthID, loadSigningKeys bool) (teleservices.CertAuthority, error) {
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
		ca.SetSigningKeys(nil)
	}
	return ca, nil
}

// GetCertAuthorities returns a list of authorities of a given type
// loadSigningKeys controls whether signing keys should be loaded or not
func (b *backend) GetCertAuthorities(
	caType teleservices.CertAuthType, loadSigningKeys bool) ([]teleservices.CertAuthority, error) {
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
