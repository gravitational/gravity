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

// CleanRegistry removes images not present in requiredImages from registry rooted at registryDir
func CleanRegistry(ctx context.Context, registryDir string, requiredImages []string) error {
	c, err := newCleaner(ctx, registryDir)
	if err != nil {
		return trace.Wrap(err)
	}

	unusedIndex, err := c.indexUnused(ctx, requiredImages)
	if err != nil {
		return trace.Wrap(err)
	}

	if err = c.untag(ctx, unusedIndex); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(c.deleteUnusedBlobs(ctx))
}

func newCleaner(ctx context.Context, registryDir string) (*cleaner, error) {
	driver := filesystem.New(filesystem.DriverParameters{
		RootDirectory: registryDir,
		MaxThreads:    defaults.ImageServiceMaxThreads,
	})
	registry, err := storage.NewRegistry(ctx, driver, storage.EnableDelete)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	repositoryEnumerator, ok := registry.(distribution.RepositoryEnumerator)
	if !ok {
		return nil, trace.Errorf("unable to convert Registry to RepositoryEnumerator")
	}
	return &cleaner{
		registry: registry,
		driver:   driver,
		enum:     repositoryEnumerator,
	}, nil
}

func (c *cleaner) indexUnused(ctx context.Context, images []string) (*repoIndex, error) {
	repoIndex, err := imagesToRepoIndex(images)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	deletedIndex := newRepoIndex()
	err = c.enum.Enumerate(ctx, func(repoName string) error {
		requiredRepoTagIndex := repoIndex.getTagIndex(repoName)

		repository, err := c.getRepository(ctx, repoName)
		if err != nil {
			return trace.Wrap(err)
		}
		tags, err := repository.Tags(ctx).All(ctx)
		if err != nil {
			return trace.Wrap(err, "failed to get all tags for repo name %s", repoName)
		}
		for _, tag := range tags {
			// need to delete entire repository
			if requiredRepoTagIndex == nil {
				deletedIndex.ensureTagIndex(repoName).add(tag)
				continue
			}

			if !requiredRepoTagIndex.tags[tag] {
				deletedIndex.ensureTagIndex(repoName).add(tag)
			}
		}
		return nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return deletedIndex, nil
}

func (c *cleaner) getRepository(ctx context.Context, name string) (distribution.Repository, error) {
	named, err := dockerref.WithName(name)
	if err != nil {
		return nil, trace.Wrap(err, "failed to parse repo name %s", name)
	}
	repository, err := c.registry.Repository(ctx, named)
	if err != nil {
		return nil, trace.Wrap(err, "failed to construct repository")
	}
	return repository, nil
}

// untag untags the images given with index from the registry
func (c *cleaner) untag(ctx context.Context, index *repoIndex) error {
	for _, repoTagIndex := range index.repos {
		repository, err := c.getRepository(ctx, repoTagIndex.name)
		if err != nil {
			return trace.Wrap(err)
		}
		tagService := repository.Tags(ctx)
		for tag := range repoTagIndex.tags {
			err := tagService.Untag(ctx, tag)
			if err != nil {
				return trace.Wrap(err, "unable to untag %s:%s", repoTagIndex.name, tag)
			}
		}
	}
	return nil
}

type cleaner struct {
	enum     distribution.RepositoryEnumerator
	registry distribution.Namespace
	driver   *filesystem.Driver
}

func (c *cleaner) deleteUnusedBlobs(ctx context.Context) error {
	opts := storage.GCOpts{
		RemoveUntagged: true,
	}
	return trace.Wrap(storage.MarkAndSweep(ctx, c.driver, c.registry, opts))
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
