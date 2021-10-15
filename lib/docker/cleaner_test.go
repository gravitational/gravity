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
	"sort"

	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/utils"

	dockerapi "github.com/fsouza/go-dockerclient"
	"github.com/sirupsen/logrus"
	"gopkg.in/check.v1"
)

var _ = check.Suite(&CleanerSuite{})

type CleanerSuite struct {
	client      *dockerapi.Client
	sync        *Synchronizer
	registry    *TestRegistry
	registryDir string
}

func (s *CleanerSuite) SetUpTest(c *check.C) {
	var err error
	s.client, err = NewClientFromEnv()
	c.Assert(err, check.IsNil)
	s.sync = NewSynchronizer(logrus.New(), s.client, utils.DiscardProgress)
	s.registryDir = c.MkDir()
	s.registry = NewTestRegistry(s.registryDir, s.sync, c)
}

func (s *CleanerSuite) TearDownTest(*check.C) {
	_ = s.registry.r.Close()
}

func (s *CleanerSuite) removeImages(images []loc.DockerImage) {
	for _, image := range images {
		// Error is ignored since this is a best-effort cleanup
		_ = s.client.RemoveImage(image.String())
	}
}

func (s *CleanerSuite) generateImages(c *check.C) ([]loc.DockerImage, []loc.DockerImage, []loc.DockerImage) {
	cleanImages := GenerateTestDockerImages(s.client, "test/clean", 5, c)
	validImages := GenerateTestDockerImages(s.client, "test/valid", 5, c)
	invalidImages := GenerateTestDockerImages(s.client, "test/invalid", 6, c)

	allImages := make([]loc.DockerImage, 0)
	allImages = append(allImages, cleanImages...)
	allImages = append(allImages, validImages...)
	allImages = append(allImages, invalidImages...)

	requiredImages := make([]loc.DockerImage, 0)
	requiredImages = append(requiredImages, validImages...)
	requiredImages = append(requiredImages, invalidImages[3:]...)

	expectedDeletedImages := make([]loc.DockerImage, 0)
	expectedDeletedImages = append(expectedDeletedImages, cleanImages...)
	expectedDeletedImages = append(expectedDeletedImages, invalidImages[:3]...)

	return allImages, requiredImages, expectedDeletedImages
}

func getTags(images []loc.DockerImage, repoName string) []string {
	tags := make([]string, 0)
	for _, image := range images {
		if image.Repository == repoName {
			tags = append(tags, image.Tag)
		}
	}
	if len(tags) == 0 {
		return nil
	}
	sort.Strings(tags)
	return tags
}

func (s *CleanerSuite) TestCleanRegistry(c *check.C) {
	allImages, requiredImages, expectedDeletedImages := s.generateImages(c)

	defer s.removeImages(allImages)

	s.registry.Push(c, allImages...)
	// registry http server must be stopped since CleanRegistry requires direct access to the registry's root directory
	_ = s.registry.r.Close()

	requiredImageReferences := make([]string, 0)
	for _, i := range requiredImages {
		requiredImageReferences = append(requiredImageReferences, i.String())
	}
	ctx := context.Background()

	// delete unnecessary images
	err := CleanRegistry(ctx, s.registryDir, requiredImageReferences)
	c.Assert(err, check.IsNil)

	// restart the registry http server to make sure all the required images are there
	s.registry = NewTestRegistry(s.registryDir, s.sync, c)

	for _, image := range requiredImages {
		exists, err := s.sync.ImageExists(ctx, s.registry.info.GetURL(), image)
		c.Assert(err, check.IsNil)
		c.Assert(exists, check.Equals, true)
	}
	for _, image := range expectedDeletedImages {
		exists, err := s.sync.ImageExists(ctx, s.registry.info.GetURL(), image)
		c.Assert(err, check.IsNil)
		c.Assert(exists, check.Equals, false)
	}
	validTags, err := s.sync.ImageTags(ctx, s.registry.info.GetURL(), "test/valid")
	c.Assert(err, check.IsNil)
	sort.Strings(validTags)
	c.Assert(validTags, check.DeepEquals, getTags(requiredImages, "test/valid"))

	invalidTags, err := s.sync.ImageTags(ctx, s.registry.info.GetURL(), "test/invalid")
	c.Assert(err, check.IsNil)
	sort.Strings(invalidTags)
	c.Assert(invalidTags, check.DeepEquals, getTags(requiredImages, "test/invalid"))

	cleanedTags, err := s.sync.ImageTags(ctx, s.registry.info.GetURL(), "test/clean")
	c.Assert(err, check.IsNil)
	sort.Strings(cleanedTags)
	c.Assert(cleanedTags, check.DeepEquals, getTags(requiredImages, "test/clean"))
}
