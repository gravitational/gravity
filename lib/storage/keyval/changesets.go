package keyval

import (
	"sort"

	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/trace"
	"github.com/pborman/uuid"
)

func (b *backend) CreatePackageChangeset(p storage.PackageChangeset) (*storage.PackageChangeset, error) {
	if err := p.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	if p.ID == "" {
		p.ID = uuid.New()
	}
	p.Created = b.Now().UTC()
	err := b.createVal(b.key(changesetsP, p.ID), p, forever)
	if err != nil {
		if trace.IsAlreadyExists(err) {
			return nil, trace.AlreadyExists("changeset(%v) already exists", p.ID)
		}
		return nil, trace.Wrap(err)
	}
	return &p, nil
}

func (b *backend) GetPackageChangesets() ([]storage.PackageChangeset, error) {
	keys, err := b.getKeys(b.key(changesetsP))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var out []storage.PackageChangeset
	for _, id := range keys {
		changeset, err := b.GetPackageChangeset(id)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		out = append(out, *changeset)
	}
	sort.Sort(changesetsSorter(out))
	return out, nil
}

func (b *backend) GetPackageChangeset(id string) (*storage.PackageChangeset, error) {
	var p storage.PackageChangeset
	err := b.getVal(b.key(changesetsP, id), &p)
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.NotFound("changeset(%v) not found", id)
		}
		return nil, trace.Wrap(err)
	}
	return &p, nil
}

type changesetsSorter []storage.PackageChangeset

func (s changesetsSorter) Len() int {
	return len(s)
}

func (s changesetsSorter) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s changesetsSorter) Less(i, j int) bool {
	return s[i].Created.After(s[j].Created)
}
