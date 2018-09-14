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
	"time"

	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/trace"
)

func (b *backend) GetOpsCenterLinks(siteDomain string) ([]storage.OpsCenterLink, error) {
	hostnames, err := b.getKeys(b.key(sitesP, siteDomain, linksP))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var out []storage.OpsCenterLink
	for _, hostname := range hostnames {
		types, err := b.getKeys(b.key(sitesP, siteDomain, linksP, hostname))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		for _, linkType := range types {
			var link storage.OpsCenterLink
			err := b.getVal(b.key(sitesP, siteDomain, linksP, hostname, linkType), &link)
			if err != nil {
				if trace.IsNotFound(err) {
					continue
				}
				return nil, trace.Wrap(err)
			}
			out = append(out, link)
		}
	}
	sort.Sort(linksSorter(out))
	return out, nil
}

func (b *backend) UpsertOpsCenterLink(l storage.OpsCenterLink, ttl time.Duration) (*storage.OpsCenterLink, error) {
	if err := l.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	_, err := b.GetSite(l.SiteDomain)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = b.upsertVal(b.key(sitesP, l.SiteDomain, linksP, l.Hostname, l.Type), l, ttl)
	return &l, trace.Wrap(err)
}

type linksSorter []storage.OpsCenterLink

func (s linksSorter) Len() int {
	return len(s)
}

func (s linksSorter) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s linksSorter) Less(i, j int) bool {
	return s[i].Hostname+s[i].Type < s[j].Hostname+s[j].Type
}
