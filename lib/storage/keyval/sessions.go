package keyval

import (
	teleservices "github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
)

func (b *backend) UpsertWebSession(email, sid string, session teleservices.WebSession) error {
	_, err := b.GetUser(email)
	if err != nil {
		return trace.Wrap(err)
	}
	data, err := teleservices.GetWebSessionMarshaler().MarshalWebSession(session)
	if err != nil {
		return trace.Wrap(err)
	}
	err = b.upsertValBytes(b.key(usersP, email, webSessionsP, sid), data, b.ttl(session.GetBearerTokenExpiryTime()))
	return trace.Wrap(err)
}

// GetWebSession returns a web session state for a given user and session id
func (b *backend) GetWebSession(email, sid string) (teleservices.WebSession, error) {
	_, err := b.GetUser(email)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	data, err := b.getValBytes(b.key(usersP, email, webSessionsP, sid))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("session(%v, %v) not found", email, sid)
		}
		return nil, trace.Wrap(err)
	}
	return teleservices.GetWebSessionMarshaler().UnmarshalWebSession(data)
}

func (b *backend) DeleteWebSession(email, sid string) error {
	err := b.deleteKey(b.key(usersP, email, webSessionsP, sid))
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("session(%v, %v) not found", email, sid)
		}
		return trace.Wrap(err)
	}
	return nil
}
