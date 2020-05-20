/*
Copyright 2020 Gravitational, Inc.

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
	"fmt"
	"strings"

	registryauth "github.com/docker/distribution/registry/client/auth"
	"github.com/gravitational/gravity/lib/utils"
	"github.com/gravitational/trace"
)

// ScanConfig represents configuration used to push images to a docker repository configured for scanning those
// images for vulnerabilities or other problems
type ScanConfig struct {
	// Remote Repository is the docker repository to use to scan the images.
	// Example: gravitational/gravity-scan
	RemoteRepository string
	// TagPrefix is a string to prepend to each images tag, in order to help identify the source of the image.
	// Example: 7.0.0 for a release of gravity 7.0.0
	TagPrefix string
}

// NewScanningImageService creates an image service that rewrites image paths to a
// repository used for scanning those images.
func NewScanningImageService(req RegistryConnectionRequest, conf ScanConfig) (ImageService, error) {
	base, err := NewImageService(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &scanningImageService{
		imageService: base.(*imageService),
		conf:         conf,
	}, nil
}

// scanningImageService wraps a normal ImageService and rewrites image paths
// in a consistent way for pushing all images to a scanning repository
type scanningImageService struct {
	*imageService
	conf ScanConfig
}

// Sync syncs all images in a local directory, with a remote registry that is capable of scanning and
// reporting vulnerabilities in those images. All images and tags are written into a designated repository, with the
// image tag rewritten to help identify the image.
// Example of gravity 7.0.0:
//   quay.io/gravitational/debian-tall:0.0.1 -> quay.io/gravitational/gravity-scan:7.0.0_gravitational_debian-tall_0.0.1
func (r *scanningImageService) Sync(ctx context.Context, dir string, progress utils.Printer) (installedTags []TagSpec, err error) {
	if err = r.connect(ctx); err != nil {
		return nil, trace.Wrap(err)
	}
	r.Debugf("Synchronizing local directory %q.", dir)
	localStore, err := openLocal(dir)
	if err != nil {
		return nil, trace.Wrap(err, "failed to open local directory %q as local registry", dir)
	}

	repos, err := ListRepos(ctx, localStore)
	if err != nil {
		return nil, trace.Wrap(err, "failed to list local repositories in %q", dir)
	}

	for _, localRepoName := range repos {
		localRepo, err := localStore.Repository(ctx, localRepoName)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		remoteRepo, err := r.remoteStore.Repository(ctx, r.conf.RemoteRepository)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		localManifests, err := localRepo.Manifests(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		localTags := localRepo.Tags(ctx)

		tags, err := localTags.All(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		for _, tag := range tags {
			tagSpec := TagSpec{
				Name: r.conf.RemoteRepository,
				Version: fmt.Sprintf("%s_%s_%s",
					r.conf.TagPrefix,
					strings.ReplaceAll(localRepoName, "/", "_"),
					tag,
				),
			}

			desc, err := localTags.Get(ctx, tag)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			localManifest, err := localManifests.Get(ctx, desc.Digest)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			progress.Printf("Pushing image %s:%s -> %s\n", localRepoName, tag, tagSpec)
			if r.remoteStore.tokenHandler != nil {
				r.remoteStore.tokenHandler.AddScope(registryauth.RepositoryScope{
					Repository: tagSpec.Name,
					Class:      "image",
					Actions:    []string{"push"},
				})
			}
			if _, err = r.remoteStore.updateRepo(ctx, remoteRepo, localRepo, localManifest, tagSpec.Version); err != nil {
				return nil, trace.Wrap(err, "failed to update remote for tag %q: %v", tagSpec, err)
			}

			installedTags = append(installedTags, tagSpec)
		}
	}

	return installedTags, nil
}

func (r *scanningImageService) Wrap(image string) string {
	return r.imageService.Wrap(image)
}

func (r *scanningImageService) Unwrap(image string) string {
	return r.imageService.Unwrap(image)
}

func (r *scanningImageService) List(ctx context.Context) ([]Image, error) {
	return r.imageService.List(ctx)
}
