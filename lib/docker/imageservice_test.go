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

package docker

import (
	"context"
	"io"

	registryauth "github.com/docker/distribution/registry/client/auth"

	. "gopkg.in/check.v1"
)

type ImageServiceSuite struct{}

var _ = Suite(&ImageServiceSuite{})

func (r *ImageServiceSuite) TestRewritesImage(c *C) {
	var testCases = []struct {
		image   string
		rewrite string
	}{
		{
			image:   "foobar.com/dummy:0.0.1",
			rewrite: "apiserver:5000/dummy:0.0.1",
		},
		{
			image:   "apiserver:5000/dummy:0.0.1",
			rewrite: "apiserver:5000/dummy:0.0.1",
		},
		{
			image:   "private.repo:1234/dummy:0.0.1",
			rewrite: "apiserver:5000/dummy:0.0.1",
		},
		{
			image:   "log-collector:latest",
			rewrite: "apiserver:5000/log-collector:latest",
		},
		{
			image:   "planet/base:latest",
			rewrite: "apiserver:5000/planet/base:latest",
		},
		{
			image:   "docker.io/gravitational/debian-tall",
			rewrite: "apiserver:5000/gravitational/debian-tall",
		},
	}
	service, err := NewImageService(RegistryConnectionRequest{
		RegistryAddress: "apiserver:5000",
	})
	c.Assert(err, IsNil)

	for _, testCase := range testCases {
		localImage := service.Wrap(testCase.image)
		c.Assert(localImage, DeepEquals, testCase.rewrite)
	}
}

func (r *ImageServiceSuite) TestReportsEOFForEmptyRepository(c *C) {
	registry := &registry{repos: []string{}}
	_, err := ListRepos(context.Background(), registry)
	c.Assert(err, ErrorMatches, "EOF")
}

func (r *ImageServiceSuite) TestListsRepos(c *C) {
	registry := &registry{repos: []string{"a", "b", "c", "d", "e"}}
	repos, err := ListRepos(context.Background(), registry)
	c.Assert(err, IsNil)
	c.Assert(repos, DeepEquals, []string{"a", "b", "c", "d", "e"})
}

func (r *ImageServiceSuite) TestMergingScopeActions(c *C) {
	cases := []struct {
		comment  string
		scopes   []registryauth.RepositoryScope
		expected []registryauth.RepositoryScope
	}{
		{
			comment: "merging multiple actions into the same scope",
			scopes: []registryauth.RepositoryScope{
				{
					Repository: "a",
					Class:      "a",
					Actions:    []string{"push"},
				},
				{
					Repository: "a",
					Class:      "a",
					Actions:    []string{"pull"},
				},
			},
			expected: []registryauth.RepositoryScope{
				{
					Repository: "a",
					Class:      "a",
					Actions:    []string{"push", "pull"},
				},
			},
		},
		{
			comment: "more that one scope added to the list without merging",
			scopes: []registryauth.RepositoryScope{
				{
					Repository: "a",
					Class:      "a",
					Actions:    []string{"push"},
				},
				{
					Repository: "b",
					Class:      "b",
					Actions:    []string{"pull"},
				},
			},
			expected: []registryauth.RepositoryScope{
				{
					Repository: "a",
					Class:      "a",
					Actions:    []string{"push"},
				},
				{
					Repository: "b",
					Class:      "b",
					Actions:    []string{"pull"},
				},
			},
		},
		{
			comment: "adding the same scope more than once should only include it once",
			scopes: []registryauth.RepositoryScope{
				{
					Repository: "a",
					Class:      "a",
					Actions:    []string{"push"},
				},
				{
					Repository: "a",
					Class:      "a",
					Actions:    []string{"push"},
				},
			},
			expected: []registryauth.RepositoryScope{
				{
					Repository: "a",
					Class:      "a",
					Actions:    []string{"push"},
				},
			},
		},
	}

	for _, tt := range cases {
		th := &multiScopeTokenHandler{}
		for _, scope := range tt.scopes {
			th.AddScope(scope)
		}

		c.Assert(th.scopes, DeepEquals, tt.expected, Commentf(tt.comment))
	}
}

type registry struct {
	repos []string
	n     int
}

func (r *registry) Repositories(ctx context.Context, repos []string, last string) (n int, err error) {
	n = min(5, len(r.repos))
	copy(repos, r.repos[:n])
	r.repos = r.repos[n:]
	r.n += n
	if n == 0 {
		err = io.EOF
	}
	return n, err
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
