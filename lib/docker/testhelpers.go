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
	"fmt"
	"os"

	"github.com/gravitational/gravity/lib/archive"
	"github.com/gravitational/gravity/lib/loc"

	dockerapi "github.com/fsouza/go-dockerclient"
	"gopkg.in/check.v1"
)

// GenerateTestDockerImage generates a test docker image in the specified repository and with the given
// tag
func GenerateTestDockerImage(client *dockerapi.Client, repoName, tag string, c *check.C) loc.DockerImage {
	image := loc.DockerImage{
		Repository: repoName,
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

// GenerateTestDockerImages generates the requested number of docker images in the specified repository
func GenerateTestDockerImages(client *dockerapi.Client, repoName string, size int, c *check.C) []loc.DockerImage {
	// Use a predictable tagging scheme
	imageTag := func(i int) string {
		return fmt.Sprintf("v0.0.%d", i)
	}
	images := make([]loc.DockerImage, 0, size)
	for i := 0; i < size; i++ {
		image := GenerateTestDockerImage(client, repoName, imageTag(i), c)
		images = append(images, image)
	}
	return images
}

// Close stops this registry instance and removes its resources
func (r *TestRegistry) Close() error {
	return r.r.Close()
}

// Push pushes the specified images to the underlying registry
func (r *TestRegistry) Push(c *check.C, images ...loc.DockerImage) {
	for _, image := range images {
		c.Assert(r.helper.Push(image.String(), r.r.Addr()), check.IsNil)
	}
}

// TestRegistry represents an instance of a docker registry for testing purposes
type TestRegistry struct {
	dir    string
	r      *Registry
	info   RegistryInfo
	helper *Synchronizer
}
