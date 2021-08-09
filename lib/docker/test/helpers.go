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
	"fmt"
	"os"

	dockerapi "github.com/fsouza/go-dockerclient"
	"github.com/gravitational/gravity/lib/archive"
	"github.com/gravitational/gravity/lib/docker"
	"github.com/gravitational/gravity/lib/loc"

	"gopkg.in/check.v1"
)

// NewRegistry returns a new started docker registry
func NewRegistry(dir string, s *docker.Synchronizer, c *check.C) *Registry {
	config := docker.BasicConfiguration("127.0.0.1:0", dir)
	r, err := docker.NewRegistry(config)
	c.Assert(err, check.IsNil)
	c.Assert(r.Start(), check.IsNil)
	return &Registry{
		r:   r,
		dir: dir,
		info: docker.RegistryInfo{
			Address:  r.Addr(),
			Protocol: "http",
		},
		helper: s,
	}
}

// GenerateDockerImages generates a set of docker image (subject to size)
func GenerateDockerImages(client *dockerapi.Client, repository string, numImages int, c *check.C) []loc.DockerImage {
	// Use a predictable tagging scheme
	imageTag := func(i int) string {
		return fmt.Sprintf("v0.0.%d", i)
	}
	images := make([]loc.DockerImage, 0, numImages)
	for i := 0; i < numImages; i++ {
		image := GenerateDockerImage(client, repository, imageTag(i), c)
		images = append(images, image)
	}
	return images
}

// GenerateDockerImage generates a test docker image with unique contents
// in the specified repository and given a tag
func GenerateDockerImage(client *dockerapi.Client, repository, tag string, c *check.C) loc.DockerImage {
	image := loc.DockerImage{
		Repository: repository,
		Tag:        tag,
	}
	imageName := image.String()
	files := make([]*archive.Item, 0)
	files = append(files, archive.ItemFromStringMode("version.txt", tag, 0666))
	dockerFile := "FROM scratch\nCOPY version.txt .\n"
	files = append(files, archive.ItemFromStringMode("Dockerfile", dockerFile, 0666))
	r := archive.MustCreateMemArchive(files)
	c.Assert(client.BuildImage(dockerapi.BuildImageOptions{
		Name:         imageName,
		InputStream:  r,
		OutputStream: os.Stdout,
	}), check.IsNil)
	return image
}

func (r Registry) Addr() string {
	return r.r.Addr()
}

// Push pushes the set of images to the underlying registry
func (r *Registry) Push(c *check.C, images ...loc.DockerImage) {
	for _, image := range images {
		c.Assert(r.helper.Push(image.String(), r.r.Addr()), check.IsNil)
	}
}

// Registry is a test docker registry instance
type Registry struct {
	dir    string
	r      *docker.Registry
	info   docker.RegistryInfo
	helper *docker.Synchronizer
}
