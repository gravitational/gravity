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

	teleservices "github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
)

// GetNamespaces returns a list of namespaces
func (b *backend) GetNamespaces() ([]teleservices.Namespace, error) {
	keys, err := b.getKeys(b.key(namespacesP))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out := make([]teleservices.Namespace, len(keys))
	for i, name := range keys {
		u, err := b.GetNamespace(name)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out[i] = *u
	}
	sort.Sort(teleservices.SortedNamespaces(out))
	return out, nil
}

// UpsertNamespace upserts namespace
func (b *backend) UpsertNamespace(n teleservices.Namespace) error {
	data, err := json.Marshal(n)
	if err != nil {
		return trace.Wrap(err)
	}
	err = b.upsertValBytes(b.key(namespacesP, n.Metadata.Name), data, forever)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetNamespace returns a namespace by name
func (b *backend) GetNamespace(name string) (*teleservices.Namespace, error) {
	if name == "" {
		return nil, trace.BadParameter("missing namespace name")
	}
	data, err := b.getValBytes(b.key(namespacesP, name))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("namespace %v is not found", name)
		}
		return nil, trace.Wrap(err)
	}
	return teleservices.UnmarshalNamespace(data)
}

// DeleteNamespace deletes a namespace with all the keys from the backend
func (b *backend) DeleteNamespace(namespace string) error {
	if namespace == "" {
		return trace.BadParameter("missing namespace name")
	}
	err := b.deleteKey(b.key(namespacesP, namespace))
	if trace.IsNotFound(err) {
		return trace.NotFound("namespace %v is not found", namespace)
	}
	return trace.Wrap(err)
}

// DeleteAllNamespaces deletes all namespaces
func (b *backend) DeleteAllNamespaces() error {
	err := b.deleteDir(b.key(namespacesP))
	if err != nil {
		if !trace.IsNotFound(err) {
			return err
		}
	}
	return nil
}
