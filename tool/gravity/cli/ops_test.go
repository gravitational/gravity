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

package cli

/*
import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"

	libapp "github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/app/service"
	apptest "github.com/gravitational/gravity/lib/app/service/test"
	"github.com/gravitational/gravity/lib/archive"
	"github.com/gravitational/gravity/lib/compare"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/docker"
	dockertest "github.com/gravitational/gravity/lib/docker/test"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/localenv"
	packtest "github.com/gravitational/gravity/lib/pack/test"
	"github.com/gravitational/gravity/lib/utils"
	"github.com/gravitational/trace"

	dockerapi "github.com/fsouza/go-dockerclient"
	"github.com/sirupsen/logrus"
	"gopkg.in/check.v1"
)

type OpsSuite struct{}

var _ = check.Suite(&OpsSuite{})

func (*OpsSuite) TestUploadsUpdate(c *check.C) {
	client, err := docker.NewClientFromEnv()
	if err != nil {
		c.Skip(fmt.Sprint("This test requires docker: ", err))
	}

	// setup
	from, to := service.NewTestServices(c.MkDir(), c), service.NewTestServices(c.MkDir(), c)
	depPackageLoc := loc.MustParseLocator("example.com/package:1.0.0")
	depAppLoc := loc.MustParseLocator("gravitational.io/dep-app:1.0.0")
	appLoc := loc.MustParseLocator("gravitational.io/app:1.0.0")
	depApp := apptest.SystemApplication(depAppLoc).
		WithSchemaPackageDependencies(depPackageLoc).
		WithItems(generateDockerImage(client, loc.DockerImage{Repository: "depimage", Tag: "0.0.1"}, c)...).
		Build()
	clusterApp := apptest.DefaultClusterApplication(appLoc).
		WithAppDependencies(depApp).
		WithItems(generateDockerImage(client, loc.DockerImage{Repository: "testimage", Tag: "1.0.0"}, c)...).
		Build()
	app := apptest.CreateApplication(apptest.AppRequest{
		App:      clusterApp,
		Apps:     from.Apps,
		Packages: from.Packages,
	}, c)

	logger := logrus.WithField("test", "TestUploadsUpdate")
	synchronizer := docker.NewSynchronizer(logger, client, utils.DiscardProgress)
	registry := dockertest.NewRegistry(c.MkDir(), synchronizer, c)
	imageService, err := docker.NewImageService(docker.RegistryConnectionRequest{
		RegistryAddress: registry.Addr(),
		Insecure:        true,
	})
	c.Assert(err, check.IsNil)
	puller := libapp.Puller{
		SrcPack: from.Packages,
		SrcApp:  from.Apps,
		DstPack: to.Packages,
		DstApp:  to.Apps,
	}
	syncer := libapp.Syncer{
		PackService:  from.Packages,
		AppService:   from.Apps,
		ImageService: imageService,
		Progress:     localenv.Silent(true),
	}

	// exercise
	err = uploadApplicationUpdate(context.TODO(), puller, syncer, []docker.ImageService{imageService}, *app)

	// verify
	c.Assert(err, check.IsNil)
	verifyRegistry(context.TODO(), c, imageService, "testimage:1.0.0", "depimage:0.0.1")
	packtest.VerifyPackages(to.Packages, []loc.Locator{
		depAppLoc,
		depPackageLoc,
		appLoc,
		apptest.RuntimeApplicationLoc,
		apptest.RuntimePackageLoc,
	}, c)
}

func verifyRegistry(ctx context.Context, c *check.C, service docker.ImageService, expectedImages ...string) {
	images, err := service.List(ctx)
	c.Assert(err, check.IsNil)
	refs := make([]string, 0, len(images))
	for _, image := range images {
		for _, tag := range image.Tags {
			refs = append(refs, fmt.Sprint(image.Repository, ":", tag))
		}
	}
	c.Assert(sort.StringSlice(refs), compare.SortedSliceEquals, sort.StringSlice(expectedImages))
}

func generateDockerImage(client *dockerapi.Client, image loc.DockerImage, c *check.C) []*archive.Item {
	synchronizer := docker.NewSynchronizer(logrus.New(), client, utils.DiscardProgress)
	dockerImage := dockertest.GenerateDockerImage(client, image.Repository, image.Tag, c)
	dir := filepath.Join(c.MkDir(), defaults.RegistryDir)
	err := os.MkdirAll(dir, defaults.SharedDirMask)
	c.Assert(err, check.IsNil)
	registry := dockertest.NewRegistry(dir, synchronizer, c)
	registry.Push(c, dockerImage)
	return snapshotRegistryDirectory(dir, c)
}

func snapshotRegistryDirectory(root string, c *check.C) (result []*archive.Item) {
	parent := filepath.Dir(root)
	err := filepath.Walk(root, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return trace.Wrap(err)
		}
		rel, err := filepath.Rel(parent, path)
		if err != nil {
			return trace.Wrap(err)
		}
		if fi.IsDir() {
			result = append(result, archive.DirItem(rel))
			return nil
		}
		b, err := ioutil.ReadFile(path)
		c.Assert(err, check.IsNil)
		rc := ioutil.NopCloser(bytes.NewReader(b))
		result = append(result, archive.ItemFromStream(rel, rc, int64(len(b)), defaults.SharedReadMask))
		return nil
	})
	c.Assert(err, check.IsNil)
	return result
}
*/
