package service

import (
	"time"

	"github.com/gravitational/gravity/lib/app"
	apptest "github.com/gravitational/gravity/lib/app/service/test"
	"github.com/gravitational/gravity/lib/archive"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/pack"

	. "gopkg.in/check.v1"
)

type ResourceSuite struct {
	pack pack.PackageService
	apps app.Applications
}

var _ = Suite(&ResourceSuite{})

func (r *ResourceSuite) SetUpTest(c *C) {
	_, r.pack, r.apps = setupServices(c)
	err := r.pack.UpsertRepository("example.com", time.Time{})
	c.Assert(err, IsNil)
}

func (r *ResourceSuite) TestTranslatesResources(c *C) {
	appPackage := loc.MustParseLocator("gravitational.io/app:1.0.0")
	apptest.CreateApplication(apptest.AppRequest{
		App:      apptest.DefaultClusterApplication(appPackage).Build(),
		Packages: r.pack,
		Apps:     r.apps,
	}, c)

	_, reader, err := r.pack.ReadPackage(appPackage)
	c.Assert(err, IsNil)

	rc, err := unpackedResources(reader)
	c.Assert(err, IsNil)
	defer rc.Close()

	archive.AssertArchiveHasFiles(c, rc, nil,
		"resources/app.yaml",
		"resources/resources.yaml",
		"resources/config/config.yaml",
	)
}
