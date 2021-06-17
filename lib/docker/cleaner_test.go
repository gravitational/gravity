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

	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/utils"

	dockerapi "github.com/fsouza/go-dockerclient"
	"github.com/sirupsen/logrus"
	. "gopkg.in/check.v1"
)

var _ = Suite(&CleanerSuite{})

type CleanerSuite struct {
	client      *dockerapi.Client
	sync        *Synchronizer
	registry    *registryHelper
	registryDir string
}

func (s *CleanerSuite) SetUpTest(c *C) {
	var err error
	s.client, err = NewClientFromEnv()
	c.Assert(err, IsNil)
	s.sync = NewSynchronizer(logrus.New(), s.client, utils.DiscardProgress)
	s.registryDir = c.MkDir()
	s.registry = newRegistry(s.registryDir, s.sync, c)
}

func (s *CleanerSuite) TearDownTest(*C) {
	_ = s.registry.r.Close()
}

func (s *CleanerSuite) removeImages(images []loc.DockerImage) {
	for _, image := range images {
		// Error is ignored since this is a best-effort cleanup
		_ = s.client.RemoveImage(image.String())
	}
}

func (s *CleanerSuite) generateImages(c *C) ([]loc.DockerImage, []loc.DockerImage, []loc.DockerImage) {
	cleanImages := generateDockerImages(s.client, "test/clean", 5, c)
	validImages := generateDockerImages(s.client, "test/valid", 5, c)
	invalidImages := generateDockerImages(s.client, "test/invalid", 6, c)

	allImages := make([]loc.DockerImage, 0)
	allImages = append(allImages, cleanImages...)
	allImages = append(allImages, validImages...)
	allImages = append(allImages, invalidImages...)

	requiredImages := make([]loc.DockerImage, 0)
	requiredImages = append(requiredImages, validImages...)
	requiredImages = append(requiredImages, invalidImages[3:]...)

	expectDeletedImages := make([]loc.DockerImage, 0)
	expectDeletedImages = append(expectDeletedImages, cleanImages...)
	expectDeletedImages = append(expectDeletedImages, invalidImages[:3]...)

	return allImages, requiredImages, expectDeletedImages
}

func (s *CleanerSuite) TestCleanRegistry(c *C) {
	allImages, requiredImages, expectDeletedImages := s.generateImages(c)

	defer s.removeImages(allImages)

	s.registry.pushImages(allImages, c)
	// http server of registry must be closed, CleanRegistry is not using http protocol
	_ = s.registry.r.Close()

	requiredImgs := make([]string, 0)
	for _, i := range requiredImages {
		requiredImgs = append(requiredImgs, i.String())
	}
	ctx := context.Background()

	// delete unnecessary images
	err := CleanRegistry(ctx, s.registryDir, requiredImgs)
	c.Assert(err, IsNil)

	// re-create the http registry to make sure all the required images are there
	s.registry = newRegistry(s.registryDir, s.sync, c)

	for _, image := range requiredImages {
		exists, err := s.sync.ImageExists(ctx, s.registry.info.GetURL(), image.Repository, image.Tag)
		c.Assert(err, IsNil)
		c.Assert(exists, Equals, true)
	}
	for _, image := range expectDeletedImages {
		exists, err := s.sync.ImageExists(ctx, s.registry.info.GetURL(), image.Repository, image.Tag)
		c.Assert(err, IsNil)
		c.Assert(exists, Equals, false)
	}
	validTags, err := s.sync.ImageTags(ctx, s.registry.info.GetURL(), "test/valid")
	c.Assert(err, IsNil)
	c.Assert(len(validTags), Equals, 5)
	invalidTags, err := s.sync.ImageTags(ctx, s.registry.info.GetURL(), "test/invalid")
	c.Assert(err, IsNil)
	c.Assert(len(invalidTags), Equals, 3)
	cleanedTags, err := s.sync.ImageTags(ctx, s.registry.info.GetURL(), "test/clean")
	c.Assert(err, IsNil)
	c.Assert(len(cleanedTags), Equals, 0)
}
