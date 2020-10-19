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

package loc

import (
	"sort"

	"github.com/gravitational/teleport/lib/utils"
)

// RepositoryDiff represents the difference in Docker images for a particular repository.
type RepositoryDiff struct {
	// Repository is the Docker image repository.
	Repository string
	// Tags are the tags for this repository.
	Tags []TagDiff
}

// TagDiff represents the particular tag presence in the compared lists.
type TagDiff struct {
	// Tag is the Docker image tag.
	Tag string
	// Left is true if the tag is present in the "left" images list.
	Left bool
	// Right is true if the tag is present in the "right" images list.
	Right bool
}

// DiffDockerImages returns the difference between the provided Docker images.
func DiffDockerImages(left, right DockerImages) (results []RepositoryDiff) {
	allRepos := append(left.Repositories(), right.Repositories()...)
	allRepos = utils.Deduplicate(allRepos)
	sort.Strings(allRepos)
	for _, repo := range allRepos {
		repoTags := append(left.Tags(repo), right.Tags(repo)...)
		repoTags = utils.Deduplicate(repoTags)
		sort.Strings(repoTags)
		result := RepositoryDiff{Repository: repo}
		for _, tag := range repoTags {
			result.Tags = append(result.Tags, TagDiff{
				Tag:   tag,
				Left:  left.Has(repo, tag),
				Right: right.Has(repo, tag),
			})
		}
		results = append(results, result)
	}
	return results
}
