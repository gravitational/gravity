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

	"github.com/gravitational/trace"
)

func (b *backend) CreatePermission(p storage.Permission) (*storage.Permission, error) {
	if err := p.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	if p.CollectionID == "" {
		p.CollectionID = AllCollectionIDs
	}
	if err := b.createVal(b.key(usersP, p.UserEmail, permissionsP, p.Action, p.Collection, p.CollectionID), p, forever); err != nil {
		if trace.IsAlreadyExists(err) {
			return nil, trace.AlreadyExists("%v already exists", &p)
		}
		return nil, trace.Wrap(err)
	}
	return &p, nil
}

func (b *backend) DeletePermissionsForUser(email string) error {
	if err := b.deleteDir(b.key(usersP, email, permissionsP)); err != nil {
		if trace.IsNotFound(err) {
			return nil
		}
		return trace.Wrap(err)
	}
	return nil
}

func (b *backend) GetPermission(s storage.Permission) (*storage.Permission, error) {
	if s.CollectionID == "" {
		s.CollectionID = AllCollectionIDs
	}
	var p storage.Permission
	err := b.getVal(b.key(usersP, s.UserEmail, permissionsP, s.Action, s.Collection, s.CollectionID), &p)
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("user(%v) has no permission to %v on %v",
				s.UserEmail, s.Action, s.Collection, s.CollectionID)
		}
	}
	return &p, nil
}

func (b *backend) GetUserPermissions(email string) ([]storage.Permission, error) {
	actionKeys, err := b.getKeys(b.key(usersP, email, permissionsP))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var out []storage.Permission
	for _, action := range actionKeys {
		collectionKeys, err := b.getKeys(b.key(usersP, email, permissionsP, action))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		for _, collection := range collectionKeys {
			collectionIDKeys, err := b.getKeys(b.key(usersP, email, permissionsP, action, collection))
			if err != nil {
				return nil, trace.Wrap(err)
			}
			if len(collectionIDKeys) == 0 {
				collectionIDKeys = []string{AllCollectionIDs}
			}
			for _, collectionID := range collectionIDKeys {
				var p storage.Permission
				if err := b.getVal(b.key(usersP, email, permissionsP, action, collection, collectionID), &p); err != nil {
					return nil, trace.Wrap(err)
				}
				out = append(out, p)
			}
		}
	}
	return out, nil
}
