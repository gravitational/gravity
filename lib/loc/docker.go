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
	"strings"

	"github.com/gravitational/trace"
)

type DockerImage struct {
	Registry   string `json:"registry"`
	Repository string `json:"repository"`
	Tag        string `json:"tag"`
}

func (d *DockerImage) String() string {
	out := d.Repository
	if d.Registry != "" {
		out = d.Registry + "/" + d.Repository
	}
	if d.Tag != "" {
		if strings.HasPrefix(d.Tag, "sha256:") {
			out = out + "@" + d.Tag
		} else {
			out = out + ":" + d.Tag
		}
	}
	return out
}

// ParseDockerImage parses docker image
func ParseDockerImage(image string) (*DockerImage, error) {
	if image == "" {
		return nil, trace.BadParameter("image name can not be empty")
	}
	remote, tag := ParseRepositoryTag(image)
	parts := strings.SplitN(remote, "/", 2)

	if len(parts) == 1 {
		return &DockerImage{Registry: "", Repository: parts[0], Tag: tag}, nil
	} else if !strings.Contains(parts[0], ".") && !strings.Contains(parts[0], ":") && parts[0] != "localhost" {
		return &DockerImage{Registry: "", Repository: strings.Join(parts, "/"), Tag: tag}, nil
	}
	return &DockerImage{Registry: parts[0], Repository: strings.Join(parts[1:], "/"), Tag: tag}, nil
}

// ParseRepositoryTag returns the name of repository + tag|digest.
// The tag can be confusing because of a port in a repository name.
//     Ex: localhost.localdomain:5000/samalba/hipache:latest
//     Digest ex: localhost:5000/foo/bar@sha256:bc8813ea7b3603864987522f02a76101c17ad122e1c46d790efc0fca78ca7bfb
func ParseRepositoryTag(repos string) (string, string) {
	n := strings.Index(repos, "@")
	if n >= 0 {
		parts := strings.Split(repos, "@")
		return parts[0], parts[1]
	}
	n = strings.LastIndex(repos, ":")
	if n < 0 {
		return repos, ""
	}
	if tag := repos[n+1:]; !strings.Contains(tag, "/") {
		return repos[:n], tag
	}
	return repos, ""
}
