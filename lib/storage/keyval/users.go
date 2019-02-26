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
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/json"
	"sort"
	"time"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/storage"

	teledefaults "github.com/gravitational/teleport/lib/defaults"
	teleservices "github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"

	"github.com/pborman/uuid"
	"github.com/tstranex/u2f"
)

// GetAllUsers returns all users
func (b *backend) GetAllUsers() ([]storage.User, error) {
	keys, err := b.getKeys(b.key(usersP))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var out []storage.User
	for _, name := range keys {
		u, err := b.GetUser(name)
		if err != nil {
			if trace.IsNotFound(err) {
				continue
			}
			return nil, trace.Wrap(err)
		}
		out = append(out, u)
	}
	return out, nil
}

// GetUsers returns interactive users registered in account
func (b *backend) GetUsers(accountID string) ([]storage.User, error) {
	keys, err := b.getKeys(b.key(usersP))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var out []storage.User
	for _, name := range keys {
		u, err := b.GetUser(name)
		if err != nil {
			if trace.IsNotFound(err) {
				continue
			}
			return nil, trace.Wrap(err)
		}
		if u.GetAccountID() == accountID || accountID == "" {
			out = append(out, u)
		}
	}
	return out, nil
}

func (b *backend) GetSiteUsers(clusterName string) ([]storage.User, error) {
	keys, err := b.getKeys(b.key(usersP))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var out []storage.User
	for _, name := range keys {
		u, err := b.GetUser(name)
		if err != nil {
			if trace.IsNotFound(err) {
				continue
			}
			return nil, trace.Wrap(err)
		}
		if u.GetClusterName() == clusterName {
			out = append(out, u)
		}
	}
	return out, nil
}

func (b *backend) CreateUser(u storage.User) (storage.User, error) {
	createdBy := u.GetCreatedBy()
	if createdBy.Time.IsZero() {
		createdBy.Time = b.Now().UTC()
		u.SetCreatedBy(createdBy)
	}
	data, err := teleservices.GetUserMarshaler().MarshalUser(u)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = b.createValBytes(b.key(usersP, u.GetName(), valP), data, b.ttl(u.Expiry()))
	if err != nil {
		if trace.IsAlreadyExists(err) {
			return nil, trace.AlreadyExists("user %q already exists", u)
		}
		return nil, trace.Wrap(err)
	}
	return u, nil
}

func (b *backend) UpsertUser(u storage.User) (storage.User, error) {
	data, err := teleservices.GetUserMarshaler().MarshalUser(u)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = b.upsertValBytes(b.key(usersP, u.GetName(), valP), data, b.ttl(u.Expiry()))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return u, nil
}

func (b *backend) UpdateUser(name string, req storage.UpdateUserReq) error {
	u, err := b.GetUser(name)
	if err != nil {
		return trace.Wrap(err)
	}
	if req.HOTP != nil {
		u.SetHOTP(*req.HOTP)
	}
	if req.Password != nil {
		u.SetPassword(*req.Password)
	}
	if req.Roles != nil {
		u.SetRoles(*req.Roles)
	}
	if req.FullName != nil {
		u.SetFullName(*req.FullName)
	}

	data, err := teleservices.GetUserMarshaler().MarshalUser(u)
	if err != nil {
		return trace.Wrap(err)
	}
	err = b.updateValBytes(b.key(usersP, name, valP), data, forever)
	return trace.Wrap(err)
}

func (b *backend) DeleteUser(email string) error {
	err := b.deleteDir(b.key(usersP, email))
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.Wrap(err, "user(email=%v) not found", email)
		}
		return trace.Wrap(err)
	}
	return nil
}

func (b *backend) DeleteAllUsers() error {
	err := b.deleteDir(b.key(usersP))
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (b *backend) GetUserByOIDCIdentity(id teleservices.ExternalIdentity) (teleservices.User, error) {
	users, err := b.getUsers()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, u := range users {
		for _, oidc := range u.GetOIDCIdentities() {
			if oidc.ConnectorID == id.ConnectorID && oidc.Username == id.Username {
				return u, nil
			}
		}
	}
	return nil, trace.NotFound("user with OIDC identity %v, connector %v not found", id.Username, id.ConnectorID)
}

// GetUserBySAMLIdentity returns a user by its specified SAML Identity, returns first
// user specified with this identity
func (b *backend) GetUserBySAMLIdentity(id teleservices.ExternalIdentity) (teleservices.User, error) {
	users, err := b.getUsers()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, u := range users {
		for _, saml := range u.GetSAMLIdentities() {
			if saml.ConnectorID == id.ConnectorID && saml.Username == id.Username {
				return u, nil
			}
		}
	}
	return nil, trace.NotFound("user with SAML identity %v, connector %v not found", id.Username, id.ConnectorID)
}

// GetUserByGithubIdentity returns a user by its specified Github Identity, returns first
// user specified with this identity
func (b *backend) GetUserByGithubIdentity(id teleservices.ExternalIdentity) (teleservices.User, error) {
	users, err := b.getUsers()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, u := range users {
		for _, gh := range u.GetGithubIdentities() {
			if gh.ConnectorID == id.ConnectorID && gh.Username == id.Username {
				return u, nil
			}
		}
	}
	return nil, trace.NotFound("user with Github identity %v, connector %v not found",
		id.Username, id.ConnectorID)
}

func (b *backend) GetUser(name string) (storage.User, error) {
	data, err := b.getValBytes(b.key(usersP, name, valP))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("user(name=%q) not found", name)
		}
		return nil, trace.Wrap(err)
	}
	userI, err := teleservices.GetUserMarshaler().UnmarshalUser(data)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	user, ok := userI.(storage.User)
	if !ok {
		return nil, trace.BadParameter("internal marshal error")
	}
	return user, nil
}

func (b *backend) GetUserRoles(name string) ([]teleservices.Role, error) {
	user, err := b.GetUser(name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	roles := make([]teleservices.Role, 0, len(user.GetRoles()))
	for _, r := range user.GetRoles() {
		role, err := b.GetRole(r)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		roles = append(roles, role)
	}
	return roles, nil
}

func (b *backend) getUsers() ([]storage.User, error) {
	keys, err := b.getKeys(b.key(usersP))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var out []storage.User
	for _, key := range keys {
		u, err := b.GetUser(key)
		if err != nil {
			if trace.IsNotFound(err) {
				continue
			}
			return nil, trace.Wrap(err)
		}
		out = append(out, u)
	}
	return out, nil
}

// AddUserLoginAttempt logs user login attempt
func (b *backend) AddUserLoginAttempt(user string, attempt teleservices.LoginAttempt, ttl time.Duration) error {
	if err := attempt.Check(); err != nil {
		return trace.Wrap(err)
	}
	err := b.upsertVal(b.key(usersP, user, "attempts", uuid.New()), attempt, ttl)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(err)
}

// GetUserLoginAttempts returns user login attempts
func (b *backend) GetUserLoginAttempts(user string) ([]teleservices.LoginAttempt, error) {
	keys, err := b.getKeys(b.key(usersP, user, "attempts"))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out := make([]teleservices.LoginAttempt, 0, len(keys))
	for _, id := range keys {
		var attempt teleservices.LoginAttempt
		err := b.getVal(b.key(usersP, user, "attempts", id), &attempt)
		if err != nil {
			if !trace.IsNotFound(err) {
				return nil, trace.Wrap(err)
			}
			continue
		}
		out = append(out, attempt)
	}
	sort.Sort(teleservices.SortedLoginAttempts(out))
	return out, nil
}

// DeleteUserLoginAttempts removes all login attempts of a user. Should be called after successful login.
func (b *backend) DeleteUserLoginAttempts(user string) error {
	err := b.deleteDir(b.key(usersP, user, "attempts"))
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("login attempts are not found for user %q", user)
		}
	}
	return trace.Wrap(err)
}

// UpsertTOTP upserts TOTP secret key for a user that can be used to generate and validate tokens.
func (b *backend) UpsertTOTP(user string, secretKey string) error {
	err := b.upsertValBytes(b.key(usersP, user, "totp"), []byte(secretKey), 0)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetTOTP returns the secret key used by the TOTP algorithm to validate tokens
func (b *backend) GetTOTP(user string) (string, error) {
	bytes, err := b.getValBytes(b.key(usersP, user, "totp"))
	if err != nil {
		if trace.IsNotFound(err) {
			return "", trace.NotFound("totp is not found for user %q", user)
		}
		return "", trace.Wrap(err)
	}
	return string(bytes), nil
}

// marshallableU2FRegistration defines marshallable version of u2f.Registration
// that cannot be json marshalled due to the pointer in the public key
type marshallableU2FRegistration struct {
	// Raw is the raw registration message
	Raw []byte `json:"raw"`
	// KeyHandle is the key handle
	KeyHandle []byte `json:"keyhandle"`
	// MarshalledPubKey is marshalled public key
	MarshalledPubKey []byte `json:"marshalled_pubkey"`
}

// UpsertU2FRegistration upserts U2F registration
func (b *backend) UpsertU2FRegistration(user string, u2fReg *u2f.Registration) error {
	marshalledPubkey, err := x509.MarshalPKIXPublicKey(&u2fReg.PubKey)
	if err != nil {
		return trace.Wrap(err)
	}

	marshallableReg := marshallableU2FRegistration{
		Raw:              u2fReg.Raw,
		KeyHandle:        u2fReg.KeyHandle,
		MarshalledPubKey: marshalledPubkey,
	}

	data, err := json.Marshal(marshallableReg)
	if err != nil {
		return trace.Wrap(err)
	}

	err = b.upsertValBytes(b.key(usersP, user, userU2fRegistrationP), data, forever)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetU2FRegistration returns U2F registration by username
func (b *backend) GetU2FRegistration(user string) (*u2f.Registration, error) {
	data, err := b.getValBytes(b.key(usersP, user, userU2fRegistrationP))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	marshallableReg := marshallableU2FRegistration{}
	err = json.Unmarshal(data, &marshallableReg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pubkeyInterface, err := x509.ParsePKIXPublicKey(marshallableReg.MarshalledPubKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pubkey, ok := pubkeyInterface.(*ecdsa.PublicKey)
	if !ok {
		return nil, trace.BadParameter("failed to convert crypto.PublicKey back to ecdsa.PublicKey")
	}

	return &u2f.Registration{
		Raw:             marshallableReg.Raw,
		KeyHandle:       marshallableReg.KeyHandle,
		PubKey:          *pubkey,
		AttestationCert: nil,
	}, nil
}

// U2FRegistrationCounter defines U2F registration counter
type U2FRegistrationCounter struct {
	// Counter is U2F registration counter
	Counter uint32 `json:"counter"`
}

// UpsertU2FRegistrationCounter upserts U2F registration counter
func (b *backend) UpsertU2FRegistrationCounter(user string, counter uint32) error {
	data, err := json.Marshal(U2FRegistrationCounter{
		Counter: counter,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	err = b.upsertValBytes(b.key(usersP, user, userU2fRegistrationCounterP), data, forever)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetU2FRegistrationCounter returns U2F registration counter
func (b *backend) GetU2FRegistrationCounter(user string) (counter uint32, e error) {
	data, err := b.getValBytes(b.key(usersP, user, userU2fRegistrationCounterP))
	if err != nil {
		return 0, trace.Wrap(err)
	}

	u2fRegCounter := U2FRegistrationCounter{}
	err = json.Unmarshal(data, &u2fRegCounter)
	if err != nil {
		return 0, trace.Wrap(err)
	}

	return u2fRegCounter.Counter, nil
}

// UpsertU2FSignChallenge upserts U2F sign challenge
func (b *backend) UpsertU2FSignChallenge(user string, u2fChallenge *u2f.Challenge) error {
	data, err := json.Marshal(u2fChallenge)
	if err != nil {
		return trace.Wrap(err)
	}
	err = b.upsertValBytes(b.key(usersP, user, userU2fSignChallengeP), data, teledefaults.U2FChallengeTimeout)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetU2FSignChallenge returns sign challenge
func (b *backend) GetU2FSignChallenge(user string) (*u2f.Challenge, error) {
	data, err := b.getValBytes(b.key(usersP, user, userU2fSignChallengeP))
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

// UpsertUsedTOTPToken upserts a TOTP token to the backend so it can't be used again
// during the 30 second window it's valid.
func (b *backend) UpsertUsedTOTPToken(user string, otpToken string) error {
	err := b.upsertValBytes(b.key(usersP, user, "used_totp"), []byte(otpToken), defaults.UsedSecondFactorTokenTTL)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetUsedTOTPToken returns the last successfully used TOTP token. If no token is found "0" is returned.
func (b *backend) GetUsedTOTPToken(user string) (string, error) {
	bytes, err := b.getValBytes(b.key(usersP, user, "used_totp"))
	if err != nil {
		if trace.IsNotFound(err) {
			return "0", nil
		}
		return "", trace.Wrap(err)
	}
	return string(bytes), nil
}

// DeleteUsedTOTPToken removes the used token from the backend. This should only
// be used during tests.
func (b *backend) DeleteUsedTOTPToken(user string) error {
	return b.deleteKey(b.key(usersP, user, "used_totp"))
}
