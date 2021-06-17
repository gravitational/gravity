/*
Copyright 2021 Gravitational, Inc.

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

package docker

import (
	"context"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"

	"github.com/docker/distribution"
	dockerref "github.com/docker/distribution/reference"
	"github.com/docker/distribution/registry/storage"
	"github.com/docker/distribution/registry/storage/driver/filesystem"
	"github.com/gravitational/trace"
)

// CleanRegistry removes images in registry that are not needed
func CleanRegistry(ctx context.Context, registryDir string, requiredImages []string) error {
	repoIndex, err := imagesToRepoIndex(requiredImages)
	if err != nil {
		return trace.Wrap(err)
	}
	driver := filesystem.New(filesystem.DriverParameters{
		RootDirectory: registryDir,
		MaxThreads:    defaults.ImageServiceMaxThreads,
	})
	registry, err := storage.NewRegistry(ctx, driver, storage.EnableDelete)
	if err != nil {
		return trace.Wrap(err)
	}
	repositoryEnumerator, ok := registry.(distribution.RepositoryEnumerator)
	if !ok {
		return trace.Errorf("unable to convert Registry to RepositoryEnumerator")
	}
	deleteRepoIndex := newRepoIndex()
	err = repositoryEnumerator.Enumerate(ctx, func(repoName string) error {
		requiredRepoTagIndex := repoIndex.getTagIndex(repoName)

		named, err := dockerref.WithName(repoName)
		if err != nil {
			return trace.Wrap(err, "failed to parse repo name %s", repoName)
		}
		repository, err := registry.Repository(ctx, named)
		if err != nil {
			return trace.Wrap(err, "failed to construct repository")
		}
		tags, err := repository.Tags(ctx).All(ctx)
		if err != nil {
			return trace.Wrap(err, "failed to get all tags for repo name %s", repoName)
		}
		for _, tag := range tags {
			// need to delete entire repository
			if requiredRepoTagIndex == nil {
				deleteRepoIndex.ensureTagIndex(repoName).add(tag)
				continue
			}

			if !requiredRepoTagIndex.tags[tag] {
				deleteRepoIndex.ensureTagIndex(repoName).add(tag)
			}
		}
		return nil
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// untag all unnecessary images
	for _, repoTagIndex := range deleteRepoIndex.repos {
		named, err := dockerref.WithName(repoTagIndex.name)
		if err != nil {
			return trace.Wrap(err, "failed to parse repo name %s", repoTagIndex.name)
		}
		repository, err := registry.Repository(ctx, named)
		if err != nil {
			return trace.Wrap(err, "failed to construct repository")
		}
		tagService := repository.Tags(ctx)
		for tag := range repoTagIndex.tags {
			err := tagService.Untag(ctx, tag)
			if err != nil {
				return trace.Wrap(err, "unable to untag %s:%s", named.String(), tag)
			}
		}
	}

	// delete all blobs
	opts := storage.GCOpts{
		RemoveUntagged: true,
	}
	err = storage.MarkAndSweep(ctx, driver, registry, opts)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

type repoIndex struct {
	repos map[string]*repoTagIndex
}

func newRepoIndex() *repoIndex {
	return &repoIndex{
		repos: map[string]*repoTagIndex{},
	}
}

func (r *repoIndex) getTagIndex(repoName string) *repoTagIndex {
	return r.repos[repoName]
}

func (r *repoIndex) ensureTagIndex(repoName string) *repoTagIndex {
	repoTagIndex := r.repos[repoName]
	if repoTagIndex == nil {
		repoTagIndex = newRepoTagIndex(repoName)
		r.repos[repoName] = repoTagIndex
	}
	return repoTagIndex
}

type repoTagIndex struct {
	name string
	tags map[string]bool
}

func (t *repoTagIndex) add(tag string) {
	t.tags[tag] = true
}

func newRepoTagIndex(name string) *repoTagIndex {
	return &repoTagIndex{
		name: name,
		tags: map[string]bool{},
	}
}

func imagesToRepoIndex(images []string) (*repoIndex, error) {
	index := newRepoIndex()
	for _, image := range images {
		parsedImage, err := loc.ParseDockerImage(image)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		index.ensureTagIndex(parsedImage.Repository).add(parsedImage.Tag)
	}
	return index, nil
}
