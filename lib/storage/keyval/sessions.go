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
