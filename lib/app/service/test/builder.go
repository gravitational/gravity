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

package test

import (
	"bytes"
	"fmt"
	"time"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/archive"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/pack"

	. "gopkg.in/check.v1"
)

// CreateDummyPackage creates package with fake contents
func CreateDummyPackage(locator loc.Locator, packages pack.PackageService, c *C) {
	files := []*archive.Item{
		archive.ItemFromString("manifest.json", "{}"),
	}
	CreateDummyPackageWithContents(locator, files, packages, c)
}

// CreateDummyPackageWithContents creates package with specified contents
func CreateDummyPackageWithContents(locator loc.Locator, items []*archive.Item, packages pack.PackageService, c *C) {
	data := CreatePackageData(items, c)
	err := packages.UpsertRepository(locator.Repository, time.Time{})
	c.Assert(err, IsNil)
	_, err = packages.CreatePackage(locator, &data)
	c.Assert(err, IsNil)
}

// CreateDummyApplication creates app with valid app manifest, but fake content.
// It returns the application created in the last service specified with services
func CreateDummyApplication(locator loc.Locator, c *C, services ...app.Applications) (app *app.Application) {
	data := createAppData(locator, "", c)
	for _, service := range services {
		app = CreateApplicationFromBinaryData(service, locator, data, c)
	}
	return app
}

// CreateDummyApplicationWithDependencies creates a test application with a valid manifest
// and specified dependencies
func CreateDummyApplicationWithDependencies(apps app.Applications, loc loc.Locator, dependencies string, c *C) *app.Application {
	return createApp(loc, dependencies, apps, c)
}

// CreateAppWithDeps creates app with valid app manifest and all proper dependency packages
// although it has fake contents
func CreateAppWithDeps(apps app.Applications, packages pack.PackageService, c *C) *app.Application {
	packageList := []string{
		"gravitational.io/gravity:0.0.1",
		"gravitational.io/web-assets:0.0.1",
		"gravitational.io/teleport:0.0.4",
		"gravitational.io/planet:0.0.1",
	}

	appList := []string{
		"gravitational.io/rbac-app:0.0.1",
		"gravitational.io/dns-app:0.0.1",
		"gravitational.io/logging-app:0.0.1",
		"gravitational.io/monitoring-app:0.0.1",
		"gravitational.io/site:0.0.1",
	}

	for _, p := range packageList {
		CreateDummyPackage(loc.MustParseLocator(p), packages, c)
	}

	for _, a := range appList {
		createApp(loc.MustParseLocator(a), "", apps, c)
	}

	deps := `dependencies:
  packages:
  - gravitational.io/gravity:0.0.1
  - gravitational.io/web-assets:0.0.1
  - gravitational.io/teleport:0.0.4
  apps:
  - gravitational.io/rbac-app:0.0.1
  - gravitational.io/dns-app:0.0.1
  - gravitational.io/logging-app:0.0.1
  - gravitational.io/monitoring-app:0.0.1
  - gravitational.io/site:0.0.1`

	return createApp(loc.MustParseLocator("gravitational.io/dummy:0.0.1"), deps, apps, c)
}

func createApp(loc loc.Locator, dependencies string, apps app.Applications, c *C) *app.Application {
	data := createAppData(loc, dependencies, c)
	return CreateApplicationFromBinaryData(apps, loc, data, c)
}

func createAppData(loc loc.Locator, dependencies string, c *C) bytes.Buffer {
	const manifestTemplate = `
apiVersion: bundle.gravitational.io/v2
kind: Bundle
metadata:
  name: %v
  resourceVersion: "%v"
installer:
  flavors:
    items:
      - name: "one"
        nodes:
          - profile: node
            count: 1
nodeProfiles:
  - name: node
  - name: kmaster
    labels:
      node-role.kubernetes.io/master: "true"
  - name: knode
    labels:
      node-role.kubernetes.io/node: "true"
systemOptions:
  runtime:
    version: 0.0.1
%v
%v`

	manifestBytes := fmt.Sprintf(manifestTemplate, loc.Name, loc.Version, hooks, dependencies)

	const resourceBytes = `
apiVersion: v1
kind: Pod
metadata:
  name: webserver
  labels:
    app: sample-application
    role: webserver
spec:
  containers:
  - name: webserver
    image: alpine:edge
    ports:
      - containerPort: 80
  nodeSelector:
    role: webserver
---
apiVersion: v1
kind: Pod
metadata:
  name: platform
  labels:
    app: sample-application
    role: server
spec:
  containers:
  - name: platform
    image: busybox:1
    ports:
      - containerPort: 50001
  nodeSelector:
    role: server`
	files := []*archive.Item{
		archive.DirItem("resources"),
		archive.DirItem("resources/config"),
		archive.ItemFromString("resources/app.yaml", manifestBytes),
		archive.ItemFromString("resources/resources.yaml", resourceBytes),
		archive.ItemFromString("resources/config/config.yaml", "configuration"),
		archive.DirItem("registry"),
		archive.DirItem("registry/docker"),
		archive.ItemFromString("registry/docker/TODO", ""),
	}
	return CreatePackageData(files, c)
}

func CreateApplication(apps app.Applications, locator loc.Locator, files []*archive.Item, c *C) *app.Application {
	return CreateApplicationFromData(apps, locator, files, c)
}

func CreateApplicationFromData(apps app.Applications, locator loc.Locator, files []*archive.Item, c *C) *app.Application {
	data := CreatePackageData(files, c)
	return CreateApplicationFromBinaryData(apps, locator, data, c)
}

func CreateApplicationFromBinaryData(apps app.Applications, locator loc.Locator, data bytes.Buffer, c *C) *app.Application {
	var labels map[string]string
	app, err := apps.CreateApp(locator, &data, labels)
	c.Assert(err, IsNil)
	c.Assert(app, NotNil)
	return app
}

// CreateRuntimeApplication creates a default runtime application in the provided app service
func CreateRuntimeApplication(apps app.Applications, c *C) {
	manifest := `apiVersion: bundle.gravitational.io/v2
kind: Runtime
metadata:
  name: kubernetes
  resourceVersion: 0.0.1
systemOptions:
  dependencies:
    runtimePackage: gravitational.io/planet:0.0.1
`
	locator := loc.MustParseLocator(
		fmt.Sprintf("%v/%v:0.0.1", defaults.SystemAccountOrg, defaults.Runtime))
	items := []*archive.Item{
		archive.DirItem("resources"),
		archive.ItemFromString("resources/app.yaml", manifest),
	}
	CreateApplicationFromData(apps, locator, items, c)
}

func CreatePackage(packages pack.PackageService, locator loc.Locator, files []*archive.Item, c *C) *pack.PackageEnvelope {
	input := CreatePackageData(files, c)

	c.Assert(packages.UpsertRepository(locator.Repository, time.Time{}), IsNil)

	app, err := packages.CreatePackage(locator, &input)
	c.Assert(err, IsNil)
	c.Assert(app, NotNil)

	envelope, err := packages.ReadPackageEnvelope(locator)
	c.Assert(err, IsNil)
	c.Assert(envelope, NotNil)
	c.Assert(envelope.SizeBytes, Not(Equals), int64(0))
	c.Logf("created package %v", envelope)
	return envelope
}

func CreatePackageData(items []*archive.Item, c *C) bytes.Buffer {
	var buf bytes.Buffer
	archive := archive.NewTarAppender(&buf)

	c.Assert(archive.Add(items...), IsNil)
	archive.Close()

	return buf
}

const hooks = `hooks:
  preNodeAdd:
    job: |
      apiVersion: batch/v1
      kind: Job
      metadata:
        name: pre-join
      spec:
        template:
          spec:
            containers:
              - name: hook
                image: quay.io/gravitational/debian-tall:buster
                command: ["/bin/echo", "Pre-join hook"]
  postNodeAdd:
    job: |
      apiVersion: batch/v1
      kind: Job
      metadata:
        name: post-join
      spec:
        template:
          spec:
            containers:
              - name: hook
                image: quay.io/gravitational/debian-tall:buster
                command: ["/bin/echo", "Post-join hook"]
  networkInstall:
    job: |
      apiVersion: batch/v1
      kind: Job
      metadata:
        name: post-join
      spec:
        template:
          spec:
            containers:
            - name: hook
              image: quay.io/gravitational/debian-tall:buster
              command: ["/bin/echo", "Install overlay network hook"]`
