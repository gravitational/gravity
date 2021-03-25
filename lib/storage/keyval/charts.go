/*
Copyright 2019 Gravitational, Inc.

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
	"helm.sh/helm/v3/pkg/repo"

	"github.com/gravitational/trace"
)

// GetIndexFile returns the chart repository index file.
func (b *backend) GetIndexFile() (*repo.IndexFile, error) {
	var indexFile repo.IndexFile
	err := b.getVal(b.key(chartsP, indexP), &indexFile)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &indexFile, nil
}

// CompareAndSwapIndexFile updates the chart repository index file.
func (b *backend) CompareAndSwapIndexFile(new, existing *repo.IndexFile) (err error) {
	var out repo.IndexFile
	if existing == nil {
		err = b.compareAndSwap(b.key(chartsP, indexP), new, nil, &out, 0)
		if err != nil && trace.IsAlreadyExists(err) {
			return trace.CompareFailed("index file is already initialized")
		}
	} else {
		err = b.compareAndSwap(b.key(chartsP, indexP), new, existing, &out, 0)
	}
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// UpsertIndexFile creates or replaces chart repository index file.
func (b *backend) UpsertIndexFile(indexFile repo.IndexFile) error {
	err := b.upsertVal(b.key(chartsP, indexP), indexFile, 0)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}
