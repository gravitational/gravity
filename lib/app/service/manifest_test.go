package service

import (
	"os"
	"testing"

	"github.com/gravitational/gravity/lib/archive"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	. "gopkg.in/check.v1"
)

func init() {
	if testing.Verbose() {
		log.SetOutput(os.Stderr)
		log.SetLevel(log.InfoLevel)
	}
}

func TestService(t *testing.T) { TestingT(t) }

type ManifestSuite struct{}

var _ = Suite(&ManifestSuite{})

func (r *ManifestSuite) TestReadsManifestFromTarball(c *C) {
	files := []*archive.Item{
		archive.DirItem("resources"),
		archive.ItemFromString("resources/app.yaml", manifestBytes),
	}
	input := archive.MustCreateMemArchive(files)
	manifest, err := manifestFromSource(input)
	c.Assert(err, IsNil)
	c.Assert(manifest, NotNil)
}

func (r *ManifestSuite) TestReadsManifestFromUnpacked(c *C) {
	files := []*archive.Item{
		archive.DirItem("resources"),
		archive.ItemFromString("resources/app.yaml", manifestBytes),
	}
	input := archive.MustCreateMemArchive(files)
	manifest, _, cleanup, err := manifestFromUnpackedSource(input)
	defer cleanup()
	c.Assert(err, IsNil)
	c.Assert(manifest, NotNil)
}

func (r *ManifestSuite) TestRequiresApplicationManifest(c *C) {
	input := archive.MustCreateMemArchive(nil)
	_, err := manifestFromSource(input)
	c.Assert(err, NotNil)
	c.Assert(trace.IsNotFound(err), Equals, true, Commentf("%T", err))
}

const manifestBytes = `apiVersion: bundle.gravitational.io/v2
kind: Bundle
metadata:
  name: sample
  resourceVersion: "0.0.1"
installer:
  flavors:
    prompt: "Test flavors"
    items:
      - name: flavor1
        nodes:
          - profile: master
            count: 1
nodeProfiles:
  - name: master
	description: "control plane server"
	labels:
	  role: master`
