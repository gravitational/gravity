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
	"gopkg.in/check.v1"
)

type DiffSuite struct{}

var _ = check.Suite(&DiffSuite{})

func (s *DiffSuite) TestDiffDockerImages(c *check.C) {
	c.Assert(DiffDockerImages(DockerImages{
		{
			Repository: "gravitational/debian-tall",
			Tag:        "0.0.1",
		},
		{
			Repository: "redis",
			Tag:        "4.0.0",
		},
		{
			Repository: "mongodb",
			Tag:        "6.0.0",
		},
		{
			Repository: "robotshop/rs-user",
			Tag:        "latest",
		},
	}, DockerImages{
		{
			Repository: "gravitational/debian-tall",
			Tag:        "0.0.1",
		},
		{
			Repository: "gravitational/debian-tall",
			Tag:        "buster",
		},
		{
			Repository: "redis",
			Tag:        "5.0.0",
		},
		{
			Repository: "postgresql",
			Tag:        "11.0.0",
		},
		{
			Repository: "robotshop/rs-user",
			Tag:        "latest",
		},
	}), check.DeepEquals, []RepositoryDiff{
		{
			Repository: "gravitational/debian-tall",
			Tags: []TagDiff{
				{Tag: "0.0.1", Left: true, Right: true},
				{Tag: "buster", Left: false, Right: true},
			},
		},
		{
			Repository: "mongodb",
			Tags: []TagDiff{
				{Tag: "6.0.0", Left: true, Right: false},
			},
		},
		{
			Repository: "postgresql",
			Tags: []TagDiff{
				{Tag: "11.0.0", Left: false, Right: true},
			},
		},
		{
			Repository: "redis",
			Tags: []TagDiff{
				{Tag: "4.0.0", Left: true, Right: false},
				{Tag: "5.0.0", Left: false, Right: true},
			},
		},
		{
			Repository: "robotshop/rs-user",
			Tags: []TagDiff{
				{Tag: "latest", Left: true, Right: true},
			},
		},
	})
}
