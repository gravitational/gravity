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

func (b *backend) CreateAPIKey(k storage.APIKey) (*storage.APIKey, error) {
	if err := k.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	err := b.createVal(b.key(usersP, k.UserEmail, apikeysP, k.Token), k, b.ttl(k.Expires))
	if trace.IsNotFound(err) {
		return nil, trace.Wrap(err, "user(email=%v) not found", k.UserEmail)
	}
	return &k, trace.Wrap(err)
}

func (b *backend) UpsertAPIKey(k storage.APIKey) (*storage.APIKey, error) {
	if err := k.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	err := b.upsertVal(b.key(usersP, k.UserEmail, apikeysP, k.Token), k, b.ttl(k.Expires))
	if trace.IsNotFound(err) {
		return nil, trace.Wrap(err, "user(email=%v) not found", k.UserEmail)
	}
	return &k, trace.Wrap(err)
}

func (b *backend) GetAPIKeys(email string) ([]storage.APIKey, error) {
	keys, err := b.getKeys(b.key(usersP, email, apikeysP))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var out []storage.APIKey
	for _, key := range keys {
		var apikey storage.APIKey
		err = b.getVal(b.key(usersP, email, apikeysP, key), &apikey)
		if err != nil {
			if trace.IsNotFound(err) {
				continue
			}
			return nil, trace.Wrap(err)
		}
		utils.UTC(&apikey.Expires)
		out = append(out, apikey)
	}
	return out, nil
}

func (b *backend) GetAPIKey(token string) (*storage.APIKey, error) {
	users, err := b.GetUsers("")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, user := range users {
		keys, err := b.GetAPIKeys(user.GetName())
		if err != nil {
			return nil, trace.Wrap(err)
		}
		for _, key := range keys {
			if key.Token == token {
				return &key, nil
			}
		}
	}
	return nil, trace.NotFound("api key %v not found", token)
}

func (b *backend) DeleteAPIKey(email, token string) error {
	err := b.deleteKey(b.key(usersP, email, apikeysP, token))
	if trace.IsNotFound(err) {
		return trace.NotFound("key(email=%v, token=%v) not found", email, token)
	}
	return trace.Wrap(err)
}
