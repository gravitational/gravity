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
	"sort"

	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/trace"
)

func (b *backend) CreateRepository(r storage.Repository) (storage.Repository, error) {
	err := b.createDir(b.key(repositoriesP, r.GetName()), b.ttl(r.Expiry()))
	if err != nil {
		if trace.IsAlreadyExists(err) {
			return nil, trace.Wrap(err, "repository %q already exists", r.GetName())
		}
		return nil, trace.Wrap(err)
	}
	data, err := storage.MarshalRepository(r)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = b.createValBytes(b.key(repositoriesP, r.GetName(), valP), data, b.ttl(r.Expiry()))
	if err != nil {
		if trace.IsAlreadyExists(err) {
			return nil, trace.Wrap(err, "repository %q already exists", r.GetName())
		}
		return nil, trace.Wrap(err)
	}
	return r, nil
}

func (b *backend) DeleteRepository(name string) error {
	err := b.deleteDir(b.key(repositoriesP, name))
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.Wrap(err, "repository %q not found", name)
		}
		return trace.Wrap(err)
	}
	return nil
}

func (b *backend) GetRepository(name string) (storage.Repository, error) {
	data, err := b.getValBytes(b.key(repositoriesP, name, valP))
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("repository %q not found", name)
		}
		return nil, trace.Wrap(err)
	}
	return storage.UnmarshalRepository(data)
}

func (b *backend) GetRepositories() ([]storage.Repository, error) {
	keys, err := b.getKeys(b.key(repositoriesP))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sort.Strings(keys)
	out := make([]storage.Repository, 0)
	for _, name := range keys {
		r, err := b.GetRepository(name)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out = append(out, r)
	}
	return out, nil
}

func (b *backend) CreatePackage(p storage.Package) (*storage.Package, error) {
	if err := p.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := b.createVal(b.key(repositoriesP, p.Repository, packagesP, p.Name, versionsP, p.Version), p, forever); err != nil {
		if trace.IsAlreadyExists(err) {
			return nil, trace.AlreadyExists("%v already exists", &p)
		}
		return nil, trace.Wrap(err)
	}
	return &p, nil
}

func (b *backend) UpsertPackage(p storage.Package) (*storage.Package, error) {
	if err := p.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := b.upsertVal(b.key(repositoriesP, p.Repository, packagesP, p.Name, versionsP, p.Version), p, forever); err != nil {
		if trace.IsAlreadyExists(err) {
			return nil, trace.AlreadyExists("%v already exists", &p)
		}
		return nil, trace.Wrap(err)
	}
	return &p, nil
}

func (b *backend) DeletePackage(repository string, packageName, packageVersion string) error {
	err := b.deleteKey(b.key(repositoriesP, repository, packagesP, packageName, versionsP, packageVersion))
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("package(%v/%v:%v) not found", repository, packageName, packageVersion)
		}
		return trace.Wrap(err)
	}
	return nil
}

func (b *backend) GetPackage(repository string, packageName, packageVersion string) (*storage.Package, error) {
	var p storage.Package
	if err := b.getVal(b.key(repositoriesP, repository, packagesP, packageName, versionsP, packageVersion), &p); err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("package(%v/%v:%v) not found", repository, packageName, packageVersion)
		}
		return nil, trace.Wrap(err)
	}
	return &p, nil
}

func (b *backend) GetPackages(repository string) ([]storage.Package, error) {
	packageNames, err := b.getKeys(b.key(repositoriesP, repository, packagesP))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sort.Strings(packageNames)
	out := make([]storage.Package, 0)
	for _, packageName := range packageNames {
		packageVers, err := b.getKeys(b.key(repositoriesP, repository, packagesP, packageName, versionsP))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		sort.Strings(packageVers)
		for _, packageVer := range packageVers {
			p, err := b.GetPackage(repository, packageName, packageVer)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			out = append(out, *p)
		}
	}
	return out, nil
}

func (b *backend) UpdatePackageRuntimeLabels(repository, packageName, packageVersion string, addLabels map[string]string, removeLabels []string) error {
	var p storage.Package
	err := b.getVal(b.key(repositoriesP, repository, packagesP, packageName, versionsP, packageVersion), &p)
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.Wrap(err, "package(%v/%v:%v) not found", repository, packageName, packageVersion)
		}
		return trace.Wrap(err)
	}
	for _, label := range removeLabels {
		delete(p.RuntimeLabels, label)
	}
	for label, val := range addLabels {
		if p.RuntimeLabels == nil {
			p.RuntimeLabels = make(map[string]string)
		}
		p.RuntimeLabels[label] = val
	}
	_, err = b.UpsertPackage(p)
	return trace.Wrap(err)
}
