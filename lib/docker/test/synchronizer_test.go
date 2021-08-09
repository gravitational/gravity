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

package test

import (
	"context"
	"fmt"
	"sort"
	"testing"

	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/utils"

	dockerapi "github.com/fsouza/go-dockerclient"
	"github.com/sirupsen/logrus"
)

func TestDocker(t *testing.T) { TestingT(t) }

var _ = Suite(&DockerSuite{})

// Set up a separate Suite for this test so we can use SetUp/TearDown phases
type DockerSuite struct {
	client   *dockerapi.Client
	helper   *Synchronizer
	src, dst *TestRegistry
}

func (s *DockerSuite) SetUpTest(c *check.C) {
	var err error
	s.client, err = NewClientFromEnv()
	c.Assert(err, check.IsNil)
	s.helper = NewSynchronizer(logrus.New(), s.client, utils.DiscardProgress)
	// Set up source and destination registries
	s.src = NewTestRegistry(c.MkDir(), s.helper, c)
	s.dst = NewTestRegistry(c.MkDir(), s.helper, c)
}

func (s *DockerSuite) TearDownTest(*check.C) {
	s.src.Close()
	s.dst.Close()
}

func (s *DockerSuite) listTags(repository string, c *check.C) (tags map[string]bool) {
	opts := dockerapi.ListImagesOptions{Filters: map[string][]string{
		"reference": {repository},
	}}
	images, err := s.client.ListImages(opts)
	c.Assert(err, check.IsNil)
	tags = make(map[string]bool)
	for _, image := range images {
		for _, imageName := range image.RepoTags {
			dockerImage, err := loc.ParseDockerImage(imageName)
			c.Assert(err, check.IsNil)
			tags[dockerImage.Tag] = true
		}
	}
	return tags
}

// NewTestRegistry returns a new started docker registry
func NewTestRegistry(dir string, s *Synchronizer, c *check.C) *TestRegistry {
	config := BasicConfiguration("127.0.0.1:0", dir)
	r, err := NewRegistry(config)
	c.Assert(err, check.IsNil)
	c.Assert(r.Start(), check.IsNil)
	return &TestRegistry{
		r:   r,
		dir: dir,
		info: RegistryInfo{
			Address:  r.Addr(),
			Protocol: "http",
		},
		helper: s,
	}
}

func splitAsTagsAndImages(images []loc.DockerImage, regAddr string) (tags, exportedImages []string) {
	for _, image := range images {
		tags = append(tags, image.Tag)

		exportedImage := image
		exportedImage.Registry = regAddr
		exportedImages = append(exportedImages, exportedImage.String())
	}
	sort.Strings(tags)
	return tags, exportedImages
}

const imageRepository = "test/image"

var _ = check.Suite(&DockerSuite{})

func (s *DockerSuite) TestPullAndPushImages(c *check.C) {
	// Setup
	const dockerImageSize = 6

	dockerImages := GenerateTestDockerImages(s.client, imageRepository, dockerImageSize, c)
	defer s.removeImages(dockerImages)
	defer s.removeTaggedImages(s.src.info.Address, dockerImages)

	// the first 3 docker images are pushed to both registries
	var pushedDockerTags []string
	for _, image := range dockerImages[:3] {
		s.src.Push(c, image)
		s.dst.Push(c, image)
		pushedDockerTags = append(pushedDockerTags, image.Tag)
	}
	sort.Strings(pushedDockerTags)

	// the last docker images are pushed only to the source registry
	var unpushedDockerTags []string
	for _, image := range dockerImages[3:] {
		s.src.Push(c, image)
		unpushedDockerTags = append(unpushedDockerTags, image.Tag)
	}

	allDockerTags, exportedImages := splitAsTagsAndImages(dockerImages, s.src.r.Addr())
	srcImageRepository := fmt.Sprintf("%s/%s", s.src.r.Addr(), imageRepository)
	localImageTags := s.listTags(srcImageRepository, c)

	// generated docker images should not be in the local docker registry
	for _, tag := range allDockerTags {
		if localImageTags[tag] {
			c.Errorf("image %s:%s should not be in the local docker registry", imageRepository, tag)
		}
	}

	// all docker images should be in the source docker registry
	srcTags, err := s.helper.ImageTags(context.Background(), s.src.info.GetURL(), imageRepository)
	c.Assert(err, check.IsNil)
	sort.Strings(srcTags)
	c.Assert(srcTags, check.DeepEquals, allDockerTags)

	// only pushed docker images should be in the target docker registry
	dstTags, err := s.helper.ImageTags(context.Background(), s.dst.info.GetURL(), imageRepository)
	c.Assert(err, check.IsNil)
	sort.Strings(dstTags)
	c.Assert(dstTags, check.DeepEquals, pushedDockerTags)

	// export images
	err = s.helper.PullAndExportImages(context.Background(), exportedImages, s.dst.info, false, dockerImageSize)
	c.Assert(err, check.IsNil)

	// Validate: this is where actual validation starts
	// relist tags
	localImageTags = s.listTags(srcImageRepository, c)

	// only unpushed docker images should not be in the local docker registry
	for _, tag := range unpushedDockerTags {
		if !localImageTags[tag] {
			c.Errorf("image %s:%s should be in the local docker registry", srcImageRepository, tag)
		}
	}
	for _, tag := range pushedDockerTags {
		if localImageTags[tag] {
			c.Errorf("image %s:%s should not be in the local docker registry", srcImageRepository, tag)
		}
	}

	// all docker images should be in the target docker registry
	dstTags, err = s.helper.ImageTags(context.Background(), s.dst.info.GetURL(), imageRepository)
	c.Assert(err, check.IsNil)
	sort.Strings(dstTags)
	c.Assert(dstTags, check.DeepEquals, allDockerTags)
}

func (s *DockerSuite) removeImages(images []loc.DockerImage) {
	for _, image := range images {
		// Error is ignored since this is a best-effort cleanup
		_ = s.client.RemoveImage(image.String())
	}
}

func (s *DockerSuite) removeTaggedImages(registryAddr string, images []loc.DockerImage) {
	for _, image := range images {
		// Error is ignored since this is a best-effort cleanup
		image.Registry = registryAddr
		_ = s.client.RemoveImage(image.String())
	}
}
