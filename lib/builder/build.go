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

package builder

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"runtime"

	"github.com/gravitational/gravity/lib/docker"
	"github.com/gravitational/gravity/lib/schema"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// Build builds the standalone application installer using the provided builder
func (b *Builder) Build(ctx context.Context) error {
	err := checkBuildEnv()
	if err != nil {
		return trace.Wrap(err)
	}

	locator := b.Locator()
	if b.OutPath == "" {
		b.OutPath = fmt.Sprintf("%v-%v.tar", locator.Name, locator.Version)
		if _, err := os.Stat(b.OutPath); err == nil && !b.Overwrite {
			return trace.BadParameter("tarball %v already exists, please remove "+
				"it first or provide -f (--overwrite) flag to overwrite it", b.OutPath)
		}
	}

	switch b.Manifest.Kind {
	case schema.KindBundle, schema.KindCluster:
		b.NextStep("Building cluster image %v %v",
			locator.Name, locator.Version)
	case schema.KindApplication:
		b.NextStep("Building application image %v %v",
			locator.Name, locator.Version)
	default:
		return trace.BadParameter("unknown manifest kind %q",
			b.Manifest.Kind)
	}

	switch b.Manifest.Kind {
	case schema.KindBundle, schema.KindCluster:
		b.NextStep("Selecting base image version")
		runtimeVersion, err := b.SelectRuntime()
		if err != nil {
			return trace.Wrap(err)
		}
		err = b.checkVersion(runtimeVersion)
		if err != nil {
			return trace.Wrap(err)
		}
		err = b.SyncPackageCache(ctx, *runtimeVersion, b.UpgradeVia...)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	b.NextStep("Embedding application container images")
	vendorDir, err := ioutil.TempDir("", "vendor")
	if err != nil {
		return trace.Wrap(err)
	}
	defer os.RemoveAll(vendorDir)
	stream, err := b.Vendor(ctx, vendorDir)
	if err != nil {
		return trace.Wrap(err)
	}
	defer stream.Close()

	b.NextStep("Creating application")
	application, err := b.CreateApplication(stream)
	if err != nil {
		return trace.Wrap(err)
	}

	b.NextStep("Generating the cluster image")
	installer, err := b.GenerateInstaller(*application)
	if err != nil {
		return trace.Wrap(err)
	}
	defer installer.Close()

	b.NextStep("Saving the image as %v", b.OutPath)
	err = b.WriteInstaller(installer)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// checkBuildEnv makes sure that the environment "tele build" is invoked in is
// suitable, for example, OS is supported and Docker is running
func checkBuildEnv() error {
	if runtime.GOOS != "linux" {
		return trace.BadParameter("tele build is not supported on %v, only "+
			"Linux is supported", runtime.GOOS)
	}
	client, err := docker.NewDefaultClient()
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = client.Version()
	if err != nil {
		logrus.Error(trace.DebugReport(err))
		return trace.BadParameter("docker is not running on this machine, " +
			"please install it (https://docs.docker.com/engine/installation/) " +
			"and make sure it can be used by a non-root user " +
			"(https://docs.docker.com/install/linux/linux-postinstall/)")
	}
	return nil
}
