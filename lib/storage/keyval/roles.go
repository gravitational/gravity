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
	"sort"
	"time"

	"github.com/gravitational/gravity/lib/storage"

	teleservices "github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
)

// GetRoles returns a list of roles registered with the local auth server
func (b *backend) GetRoles() ([]teleservices.Role, error) {
	keys, err := b.getKeys(b.key(rolesP))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out := make([]teleservices.Role, len(keys))
	for i, name := range keys {
		u, err := b.GetRole(name)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out[i] = u
	}
	sort.Sort(teleservices.SortedRoles(out))
	return out, nil
}

// UpsertV2Role upserts V2 version of role object, used in tests only
func (b *backend) UpsertV2Role(r storage.RoleV2) error {
	data, err := json.Marshal(r)
	if err != nil {
		return trace.Wrap(err)
	}
	err = b.upsertValBytes(b.key(rolesP, r.Metadata.Name), data, 0)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// CreateRole creates role with predefined TTL, returns trace.AlreadyExists
// in case if role with the same name already exists
func (b *backend) CreateRole(role teleservices.Role, ttl time.Duration) error {
	data, err := teleservices.GetRoleMarshaler().MarshalRole(role)
	if err != nil {
		return trace.Wrap(err)
	}
	err = b.createValBytes(b.key(rolesP, role.GetName()), data, ttl)
	if err != nil {
		if trace.IsAlreadyExists(err) {
			return trace.AlreadyExists("role %v already exists", role.GetName())
		}
		return trace.Wrap(err)
	}
	return nil
}

// UpsertRole updates parameters about role
func (b *backend) UpsertRole(role teleservices.Role, ttl time.Duration) error {
	data, err := teleservices.GetRoleMarshaler().MarshalRole(role)
	if err != nil {
		return trace.Wrap(err)
	}
	err = b.upsertValBytes(b.key(rolesP, role.GetName()), data, ttl)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetRole returns a role by name
func (b *backend) GetRole(name string) (teleservices.Role, error) {
	if name == "" {
		return nil, trace.BadParameter("missing role name")
	}
	data, err := b.getValBytes(b.key(rolesP, name))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("role %q is not found", name)
		}
		return nil, trace.Wrap(err)
	}
	return teleservices.GetRoleMarshaler().UnmarshalRole(data)
}

// DeleteRole deletes a role with all the keys from the backend
func (b *backend) DeleteRole(role string) error {
	err := b.deleteKey(b.key(rolesP, role))
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("role %q is not found", role)
		}
	}
	return trace.Wrap(err)
}

// DeleteAllRoles deletes all roles
func (b *backend) DeleteAllRoles() error {
	err := b.deleteDir(b.key(rolesP))
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}
	return nil
}
