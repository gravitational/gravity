/*
Copyright 2019 Gravitational, Inc.

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

package catalog

import (
	"sort"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/gravity/lib/compare"

	check "gopkg.in/check.v1"
)

type listerSuite struct{}

var _ = check.Suite(&listerSuite{})

func (s *listerSuite) TestLatestAndSorting(c *check.C) {
	items := ListItems{
		listItem{Name: "alpine", Version: v("1.0.0"), Type: "Application"},
		listItem{Name: "nginx", Version: v("0.0.1"), Type: "Application"},
		listItem{Name: "nginx", Version: v("1.2.3"), Type: "Application"},
		listItem{Name: "alpine", Version: v("1.0.0-rc.3"), Type: "Application"},
		listItem{Name: "nginx", Version: v("2.0.0-alpha.1"), Type: "Application"},
		listItem{Name: "kafka", Version: v("1.0.1"), Type: "Application"},
		listItem{Name: "zookeeper", Version: v("1.0.0-beta.2"), Type: "Application"},
		listItem{Name: "kubernetes", Version: v("13.0.0"), Type: "Cluster"},
		listItem{Name: "kubernetes", Version: v("14.0.0-beta.1"), Type: "Cluster"},
	}
	latest, err := items.Latest()
	c.Assert(err, check.IsNil)
	sort.Sort(latest)
	c.Assert(latest, compare.DeepEquals, ListItems{
		listItem{Name: "kubernetes", Version: v("13.0.0"), Type: "Cluster"},
		listItem{Name: "alpine", Version: v("1.0.0"), Type: "Application"},
		listItem{Name: "kafka", Version: v("1.0.1"), Type: "Application"},
		listItem{Name: "nginx", Version: v("1.2.3"), Type: "Application"},
	})
	// Verify sort order of all items.
	sort.Sort(items)
	c.Assert(items, compare.DeepEquals, ListItems{
		listItem{Name: "kubernetes", Version: v("14.0.0-beta.1"), Type: "Cluster"},
		listItem{Name: "kubernetes", Version: v("13.0.0"), Type: "Cluster"},
		listItem{Name: "alpine", Version: v("1.0.0"), Type: "Application"},
		listItem{Name: "alpine", Version: v("1.0.0-rc.3"), Type: "Application"},
		listItem{Name: "kafka", Version: v("1.0.1"), Type: "Application"},
		listItem{Name: "nginx", Version: v("2.0.0-alpha.1"), Type: "Application"},
		listItem{Name: "nginx", Version: v("1.2.3"), Type: "Application"},
		listItem{Name: "nginx", Version: v("0.0.1"), Type: "Application"},
		listItem{Name: "zookeeper", Version: v("1.0.0-beta.2"), Type: "Application"},
	})
}

func v(ver string) semver.Version {
	return *semver.New(ver)
}
