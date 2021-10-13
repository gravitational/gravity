/*
Copyright 2018 Gravitational, Inc.

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
	"fmt"
	"strings"

	"github.com/gravitational/trace"
)

type DockerImage struct {
	Registry   string `json:"registry"`
	Repository string `json:"repository"`
	Tag        string `json:"tag"`
	Digest     string `json:"digest"`
}

func (d *DockerImage) String() string {
	out := d.Repository
	if d.Registry != "" {
		out = fmt.Sprintf("%s/%s", d.Registry, out)
	}
	if d.Tag != "" {
		out = fmt.Sprintf("%s:%s", out, d.Tag)
	}
	if d.Digest != "" {
		out = fmt.Sprintf("%s@%s", out, d.Digest)
	}
	return out
}

func ParseDockerImage(image string) (*DockerImage, error) {
	if image == "" {
		return nil, trace.BadParameter("image name can not be empty")
	}
	return &DockerImage{
		Registry:   parseRegistry(image),
		Repository: parseRepository(image),
		Tag:        parseTag(image),
		Digest:     parseDigest(image),
	}, nil
}

func parseRegistry(image string) string {
	parts := strings.SplitN(image, "/", 2)
	if len(parts) == 1 {
		return ""
	}
	if isRegistry(parts[0]) {
		return parts[0]
	}
	return ""
}

func parseRepository(image string) string {
	image = strings.Split(image, "@")[0]
	parts := strings.SplitN(image, "/", 2)
	if len(parts) == 1 {
		return strings.Split(image, ":")[0]
	}
	if isRegistry(parts[0]) {
		return strings.Split(parts[1], ":")[0]
	}
	return strings.Split(image, ":")[0]
}

func parseTag(image string) string {
	image = strings.Split(image, "@")[0]
	n := strings.LastIndex(image, ":")
	if n < 0 {
		return ""
	}
	afterColon := image[n+1:]
	if strings.Contains(afterColon, "/") {
		return ""
	}
	return afterColon
}

func parseDigest(image string) string {
	parts := strings.Split(image, "@")
	if len(parts) == 1 {
		return ""
	}
	return parts[1]
}

func isRegistry(str string) bool {
	return strings.Contains(str, ".") ||
		strings.Contains(str, ":") ||
		strings.Contains(str, "localhost")
}
