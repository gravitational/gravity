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

package blob

import (
	"io"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/users"

	teleservices "github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
)

// WithPermissions returns new ACL checking service
func WithPermissions(objects Objects, users users.Users, username string, checker teleservices.AccessChecker) Objects {
	return &ObjectsACL{
		objects:  objects,
		users:    users,
		username: username,
		checker:  checker,
	}
}

// ObjectsACL is permission aware service that wraps
// regular service and applies checks before every operation
type ObjectsACL struct {
	objects  Objects
	users    users.Users
	username string
	checker  teleservices.AccessChecker
}

func (a *ObjectsACL) Close() error {
	return a.objects.Close()
}

// WriteBLOB writes BLOB to storage, on success
// returns the envelope with hash of the blob
func (a *ObjectsACL) WriteBLOB(data io.Reader) (*Envelope, error) {
	if err := a.check(teleservices.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.objects.WriteBLOB(data)
}

// OpenBLOB opens the BLOB by hash and returns reader object
func (a *ObjectsACL) OpenBLOB(hash string) (ReadSeekCloser, error) {
	if err := a.check(teleservices.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.objects.OpenBLOB(hash)
}

// DeleteBLOB deletes the blob by hash
func (a *ObjectsACL) DeleteBLOB(hash string) error {
	if err := a.check(teleservices.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return a.objects.DeleteBLOB(hash)
}

// GetBLOBs returns blobs list present in the store
func (a *ObjectsACL) GetBLOBs() ([]string, error) {
	if err := a.check(teleservices.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.objects.GetBLOBs()
}

// GetBLOBEnvelope returns blob envelope
func (a *ObjectsACL) GetBLOBEnvelope(hash string) (*Envelope, error) {
	if err := a.check(teleservices.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return a.objects.GetBLOBEnvelope(hash)
}

// check checks whether the user has the requested permissions
func (a *ObjectsACL) check(action string) error {
	// first check the access to all repositories
	return a.checker.CheckAccessToRule(&teleservices.Context{}, defaults.Namespace, storage.KindObject, action)
}

const (
	// CollectionBLOBs represents BLOBs collection
	CollectionBLOBs = "blobs"
)

// UpsertUser upserts user that is allowed to read/write on blobs
func UpsertUser(identity users.Identity, email string) (*storage.APIKey, error) {
	role, err := users.NewObjectStorageRole(email)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = identity.UpsertRole(role, storage.Forever)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = identity.CreateUser(storage.NewUser(email, storage.UserSpecV2{
		Type:  storage.AgentUser,
		Roles: []string{role.GetName()},
	}))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	keys, err := identity.GetAPIKeys(email)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(keys) != 0 {
		return &keys[0], nil
	}
	key, err := identity.CreateAPIKey(storage.APIKey{UserEmail: email}, false)
	return key, trace.Wrap(err)
}
