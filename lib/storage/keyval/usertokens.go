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
	"encoding/json"

	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	teledefaults "github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/trace"

	"github.com/tstranex/u2f"
)

// CreateUserToken creates a token that is used for signups and resets
func (b *backend) CreateUserToken(t storage.UserToken) (*storage.UserToken, error) {
	if err := storage.CheckUserToken(t.Type); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := b.createVal(b.key(userTokensP, t.Token, valP), t, b.ttl(t.Expires)); err != nil {
		return nil, trace.Wrap(err)
	}
	return &t, nil
}

// DeleteUserToken deletes a token by its ID
func (b *backend) DeleteUserToken(tokenID string) error {
	if err := b.deleteDir(b.key(userTokensP, tokenID)); err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("user token(%v) not found", tokenID)
		}
		return trace.Wrap(err)
	}
	return nil
}

// GetUserToken returns a token by its ID
func (b *backend) GetUserToken(tokenID string) (*storage.UserToken, error) {
	var t storage.UserToken
	err := b.getVal(b.key(userTokensP, tokenID, valP), &t)
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("user token(%v) not found", tokenID)
		}
		return nil, trace.Wrap(err)
	}
	utils.UTC(&t.Created)
	return &t, nil
}

// DeleteUserTokens deletes all tokens with given type and user
func (b *backend) DeleteUserTokens(tokenType string, user string) error {
	tokens, err := b.GetUserTokens(user)
	if err != nil {
		return trace.Wrap(err)
	}

	for _, token := range tokens {
		if token.Type != tokenType {
			continue
		}

		err = b.DeleteUserToken(token.Token)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// GetUserTokens returns all tokens for a given user
func (b *backend) GetUserTokens(user string) ([]storage.UserToken, error) {
	tokenIDs, err := b.getKeys(b.key(userTokensP))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var out []storage.UserToken
	for _, tokeID := range tokenIDs {
		i, err := b.GetUserToken(tokeID)
		if err != nil {
			if trace.IsNotFound(err) {
				continue
			}
			return nil, trace.Wrap(err)
		}

		if i.User == user {
			out = append(out, *i)
		}
	}

	return out, nil
}

// UpsertU2FRegisterChallenge upserts U2F challenge to given token
func (b *backend) UpsertU2FRegisterChallenge(tokenID string, u2fChallenge *u2f.Challenge) error {
	data, err := json.Marshal(u2fChallenge)
	if err != nil {
		return trace.Wrap(err)
	}
	err = b.upsertValBytes(b.key(userTokensP, tokenID, userTokensU2fChallengesP), data, teledefaults.U2FChallengeTimeout)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetU2FRegisterChallenge returns U2F challenge by token ID
func (b *backend) GetU2FRegisterChallenge(tokenID string) (*u2f.Challenge, error) {
	data, err := b.getValBytes(b.key(userTokensP, tokenID, userTokensU2fChallengesP))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	u2fChal := u2f.Challenge{}
	err = json.Unmarshal(data, &u2fChal)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &u2fChal, nil
}
