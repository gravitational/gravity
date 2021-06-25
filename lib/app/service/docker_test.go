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

package service

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/gravitational/gravity/lib/compare"
	"github.com/gravitational/gravity/lib/loc"
	. "gopkg.in/check.v1"
)

func (s *VendorSuite) TestGeneratesProperPackageName(c *C) {
	var testCases = []struct {
		image        string
		result       loc.Locator
		visited      map[string]loc.Locator
		randomSuffix func(string) string
		comment      string
	}{
		{
			image:   "foo:5.1.0",
			result:  loc.MustParseLocator("gravitational.io/foo:5.1.0"),
			comment: "image reference w/o repository",
		},
		{
			image:   "repo/foo:1.0.0",
			result:  loc.MustParseLocator("gravitational.io/repo-foo:1.0.0"),
			comment: "image reference with repository",
		},
		{
			image:   "repo.io/subrepo/foo:0.0.1",
			result:  loc.MustParseLocator("gravitational.io/repo.io-subrepo-foo:0.0.1"),
			comment: "nested repositories",
		},
		{
			image:   "repo.io:123/subrepo/foo:0.0.1",
			result:  loc.MustParseLocator("gravitational.io/repo.io-123-subrepo-foo:0.0.1"),
			comment: "repository with a port",
		},
		{
			image:  "repo.io:123/subrepo/foo:0.0.1",
			result: loc.MustParseLocator("foo/bar:0.0.1"),
			visited: map[string]loc.Locator{
				"repo.io:123/subrepo/foo:0.0.1": loc.MustParseLocator("foo/bar:0.0.1"),
			},
			comment: "uses cached value",
		},
		{
			image:  "planet-master:0.0.1",
			result: loc.MustParseLocator("gravitational.io/planet-master-qux:0.0.1"),
			randomSuffix: func(name string) string {
				return fmt.Sprintf("%v-qux", name)
			},
			comment: "avoids collision with legacy name",
		},
	}

	for _, testCase := range testCases {
		comment := Commentf(testCase.comment)
		visited := testCase.visited
		if visited == nil {
			visited = make(map[string]loc.Locator)
		}
		generate := newRuntimePackage(visited, testCase.randomSuffix)
		runtimePackage, err := generate(testCase.image)
		c.Assert(err, IsNil, comment)
		c.Assert(*runtimePackage, compare.DeepEquals, testCase.result, comment)
	}
}

func Test_excludeImagesStartingWith(t *testing.T) {
	type args struct {
		images []string
		prefix string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "empty input - no changes",
			args: args{
				images: []string{},
				prefix: "123",
			},
			want: []string{},
		},
		{
			name: "empty pattern - nothing is excluded",
			args: args{
				images: []string{"name1", "name2"},
				prefix: "",
			},
			want: []string{"name1", "name2"},
		},
		{
			name: "exclude all images. result is empty slice",
			args: args{
				images: []string{"registry1/name1", "registry1/name2"},
				prefix: "registry1",
			},
			want: []string{},
		},
		{
			name: "doesn't exclude images since the pattern does not match",
			args: args{
				images: []string{"registry1/name1", "registry1/name2", "registry2/name2"},
				prefix: "registry3",
			},
			want: []string{"registry1/name1", "registry1/name2", "registry2/name2"},
		},
		{
			name: "search pattern at the start of slice",
			args: args{
				images: []string{"registry1/image1", "registry1/image2", "registry2/image3", "registry2/image4"},
				prefix: "registry1",
			},
			want: []string{"registry2/image3", "registry2/image4"},
		},
		{
			name: "search pattern in the middle of slice",
			args: args{
				images: []string{"registry1/image1", "registry2/image2", "registry2/image3", "registry3/image4"},
				prefix: "registry2",
			},
			want: []string{"registry1/image1", "registry3/image4"},
		},
		{
			name: "search pattern at the end of slice",
			args: args{
				images: []string{"registry1/name1", "registry1/name2", "registry2/name2", "registry2/name3"},
				prefix: "registry2",
			},
			want: []string{"registry1/name1", "registry1/name2"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := excludeImagesStartingWith(tt.args.images, tt.args.prefix); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("excludeImagesStartingWith() = %v, want %v", got, tt.want)
			}
		})
	}
}
