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

// package suite contains a backend-independent application service acceptance test suite
package suite

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/app/service"
	apptest "github.com/gravitational/gravity/lib/app/service/test"
	"github.com/gravitational/gravity/lib/archive"
	"github.com/gravitational/gravity/lib/compare"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/docker"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/cloudflare/cfssl/csr"
	dockerarchive "github.com/docker/docker/pkg/archive"
	dockertest "github.com/fsouza/go-dockerclient/testing"
	"github.com/ghodss/yaml"
	"github.com/gravitational/license/authority"
	"github.com/gravitational/rigging"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	. "gopkg.in/check.v1"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/repo"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type AppsSuite struct {
	NewService func(*C, docker.DockerInterface, docker.ImageService) app.Applications
	Packages   pack.PackageService
	ca         authority.TLSKeyPair
}

func (r *AppsSuite) SetUpSuite(c *C) {
	trace.SetDebug(true)
	log.SetOutput(os.Stderr)

	// initialize certificate authority
	ca, err := authority.GenerateSelfSignedCA(csr.CertificateRequest{
		CN: constants.OpsCenterKeyPair,
	})
	c.Assert(err, IsNil)
	r.ca = *ca
}

func (r *AppsSuite) SetUpTest(c *C) {
	c.Assert(pack.CreateCertificateAuthority(pack.CreateCAParams{
		Packages: r.Packages,
		KeyPair:  r.ca,
	}), IsNil)
}

func (r *AppsSuite) ValidatesManifest(c *C) {
	var emptyResources string
	files := []*archive.Item{
		archive.DirItem("resources"),
		archive.ItemFromString("resources/app.yaml", "$?@"),
		archive.ItemFromString("resources/resources.yaml", emptyResources),
	}
	input := apptest.CreatePackageData(files, c)
	apps := r.NewService(c, nil, nil)

	errorc := make(chan error, 1)
	progressc := make(chan *app.ProgressEntry)
	op, err := apps.CreateImportOperation(&app.ImportRequest{
		Source:         ioutil.NopCloser(&input),
		Repository:     "example.com",
		PackageName:    "app",
		PackageVersion: "0.0.1",
		ErrorC:         errorc,
		ProgressC:      progressc,
	})
	c.Assert(err, NotNil)
	c.Assert(op, IsNil)
}

func (r *AppsSuite) ImportsApplication(c *C) {
	dockerClient, err := docker.NewClientFromEnv()
	if err != nil {
		c.Skip("this test requires docker")
	}

	apps := r.NewService(c, dockerClient, nil)
	r.importApplication(apps, nil, c)
}

func (r *AppsSuite) Resources(c *C) {
	dockerClient, err := docker.NewClientFromEnv()
	if err != nil {
		c.Skip("this test requires docker")
	}

	apps := r.NewService(c, dockerClient, nil)
	a := r.importApplication(apps, nil, c)

	reader, err := apps.GetAppResources(a.Package)
	c.Assert(err, IsNil)
	defer reader.Close()

	archive.AssertArchiveHasFiles(c, reader, nil, "resources/resource-0.yaml", "resources/app.yaml")

	// import another version and check that resources have been updated
	const manifestBytes = `apiVersion: bundle.gravitational.io/v2
kind: SystemApplication
metadata:
  name: app
  resourceVersion: "0.0.1"`

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
    image: alpine:latest
    ports:
      - containerPort: 80
  nodeSelector:
    role: webserver`

	resources := []*archive.Item{
		archive.DirItem("resources"),
		archive.ItemFromString("resources/app.yaml", manifestBytes),
		archive.ItemFromString("resources/resource-1.yaml", resourceBytes),
	}

	err = apps.DeleteApp(app.DeleteRequest{Package: a.Package})
	c.Assert(err, IsNil)

	_, reader2 := r.importApplicationWithResources(apps, nil, manifestBytes, resources, c)
	defer reader2.Close()
	archive.AssertArchiveHasFiles(c, reader2, []string{"registry"},
		"resources", "resources/resource-1.yaml", "resources/app.yaml")
}

func (r *AppsSuite) RetrievesLogsForFailedImport(c *C) {
	const manifestBytes = `
apiVersion: bundle.gravitational.io/v2
kind: SystemApplication
metadata:
  name: broken-sample
  resourceVersion: 0.0.1`

	const resourceBytes = `
# sample pod pointing to a non-existent image
apiVersion: v1
kind: Pod
metadata:
  name: broken-sample
  labels:
    app: broken
    role: worker
spec:
  containers:
  - name: application
    image: non-existent-app:0.0.1
    ports:
      - containerPort: 50001
  nodeSelector:
    role: worker`

	server, err := dockertest.NewServer("127.0.0.1:0", nil, nil)
	c.Assert(err, IsNil)
	defer server.Stop()

	server.PrepareFailure("no such image", `/images/create`)

	dockerClient, err := docker.NewClient(server.URL())
	c.Assert(err, IsNil)

	imageService, err := docker.NewImageService(docker.RegistryConnectionRequest{
		RegistryAddress: defaults.DockerRegistry,
	})
	c.Assert(err, IsNil)

	apps := r.NewService(c, dockerClient, imageService)

	files := []*archive.Item{
		archive.DirItem("resources"),
		archive.ItemFromString("resources/app.yaml", manifestBytes),
		archive.ItemFromString("resources/resources.yaml", resourceBytes),
	}

	f := apptest.CreatePackageData(files, c)

	errorc := make(chan error, 1)
	progressc := make(chan *app.ProgressEntry)
	op, err := apps.CreateImportOperation(&app.ImportRequest{
		Source:           ioutil.NopCloser(&f),
		Repository:       "invalid---", // specify invalid name intentionally so import fails
		PackageName:      "app",
		PackageVersion:   "0.0.1",
		ErrorC:           errorc,
		ProgressC:        progressc,
		Vendor:           true,
		ResourcePatterns: []string{"**/*.yaml"},
	})
	c.Assert(err, IsNil)
	c.Assert(op, NotNil)

	var progressEntry *app.ProgressEntry
	for progressEntry = range progressc {
	}
	c.Assert(<-errorc, NotNil)
	c.Assert(progressEntry.IsCompleted(), Equals, true)
	c.Assert(progressEntry.State, Equals, app.ProgressStateFailed.State())

	logs, err := apps.GetOperationCrashReport(*op)
	c.Assert(err, IsNil)
	c.Assert(logs, NotNil)
	defer logs.Close()

	archive.AssertArchiveHasFiles(c, logs, nil, "operation.log")
}

func (r *AppsSuite) ExportsApplication(c *C) {
	dockerClient, err := docker.NewClientFromEnv()
	if err != nil {
		c.Skip("this test requires docker")
	}

	dir := c.MkDir()
	config := docker.BasicConfiguration("127.0.0.1:0", dir)
	registry, err := docker.NewRegistry(config)
	c.Assert(err, IsNil)

	err = registry.Start()
	c.Assert(err, IsNil)
	defer registry.Close()

	imageService, err := docker.NewImageService(docker.RegistryConnectionRequest{
		RegistryAddress: defaults.DockerRegistry,
	})
	c.Assert(err, IsNil)
	apps := r.NewService(c, dockerClient, imageService)

	vendorer, err := service.NewVendorer(service.VendorerConfig{
		DockerClient: dockerClient,
		ImageService: imageService,
		RegistryURL:  registry.Addr(),
		Packages:     r.Packages,
	})
	c.Assert(err, IsNil)
	application := r.importApplication(apps, vendorer, c)

	registryAddr := registryAddr(registry.Addr())
	c.Assert(apps.ExportApp(app.ExportAppRequest{
		Package:         application.Package,
		RegistryAddress: registryAddr,
	}), IsNil)

	remoteRegistry, err := docker.ConnectRegistry(context.TODO(),
		docker.RegistryConnectionRequest{
			RegistryAddress: registryAddr,
		})
	c.Assert(err, IsNil)
	repos := make([]string, 3)
	_, err = remoteRegistry.Repositories(context.TODO(), repos, "")
	if err != io.EOF {
		c.Assert(err, IsNil)
	}
	sort.Strings(repos)
	c.Assert(repos, DeepEquals, []string{"alpine", "busybox", "gravitational/debian-tall"})
}

func (r *AppsSuite) CreatesApplicationInstaller(c *C) {
	const manifestBytesMain = `
apiVersion: bundle.gravitational.io/v2
kind: Bundle
metadata:
  name: sample
  resourceVersion: "0.0.1"
systemOptions:
  runtime:
    name: app-dep
    version: 0.0.1
dependencies:
  packages:
  - gravitational.io/gravity:0.0.1
  apps:
  - gravitational.io/app-dep:0.0.1`

	const manifestBytesDependency = `
apiVersion: bundle.gravitational.io/v2
kind: Runtime
metadata:
  name: sample-dependency
  resourceVersion: "0.0.1"
systemOptions:
  dependencies:
    runtimePackage: gravitational.io/planet:0.0.1
`

	apps := r.NewService(c, nil, nil)
	mainApp := loc.MustParseLocator("gravitational.io/app-main:0.0.1")
	dependencyApp := loc.MustParseLocator("gravitational.io/app-dep:0.0.1")
	dependencyPackage := loc.MustParseLocator("gravitational.io/gravity:0.0.1")
	runtimePackage := loc.MustParseLocator("gravitational.io/planet:0.0.1")

	var emptyResources string
	mainFiles := []*archive.Item{
		archive.DirItem("resources"),
		archive.ItemFromString("resources/app.yaml", manifestBytesMain),
		archive.ItemFromString("resources/resources.yaml", emptyResources),
	}
	dependencyFiles := []*archive.Item{
		archive.DirItem("resources"),
		archive.ItemFromString("resources/app.yaml", manifestBytesDependency),
		archive.ItemFromString("resources/resources.yaml", emptyResources),
	}
	packageFiles := []*archive.Item{archive.ItemFromString("./data", "hello")}

	apptest.CreatePackage(r.Packages, dependencyPackage, packageFiles, c)
	apptest.CreatePackage(r.Packages, runtimePackage, packageFiles, c)
	apptest.CreateApplicationFromData(apps, dependencyApp, dependencyFiles, c)
	apptest.CreateApplicationFromData(apps, mainApp, mainFiles, c)

	req := app.InstallerRequest{
		Application: mainApp,
		Account: storage.Account{
			ID:  "12345",
			Org: "acme",
		},
		TrustedCluster: storage.NewTrustedCluster("example.com",
			storage.TrustedClusterSpecV2{
				Enabled:              true,
				ProxyAddress:         "example.com:32009",
				ReverseTunnelAddress: "example.com:32024",
				Token:                "secret",
				Roles:                []string{constants.RoleAdmin},
			}),
	}
	installerTarball, err := apps.GetAppInstaller(req)
	c.Assert(err, IsNil)
	defer installerTarball.Close()
	n, err := io.Copy(ioutil.Discard, installerTarball)
	c.Assert(err, IsNil)
	c.Assert(n, Not(Equals), 0)
	c.Logf("%d bytes transferred", n)
}

func (r *AppsSuite) CreatesApplicationWithManifest(c *C) {
	apps := r.NewService(c, nil, nil)
	manifest := `apiVersion: bundle.gravitational.io/v2
kind: Runtime
metadata:
  name: kubernetes
  resourceVersion: 0.0.1`
	locator := loc.MustParseLocator(
		fmt.Sprintf("%v/%v:0.0.1", defaults.SystemAccountOrg, defaults.Runtime))
	items := []*archive.Item{
		archive.DirItem("resources"),
		archive.ItemFromString("resources/app.yaml", manifest),
	}

	data := apptest.CreatePackageData(items, c)

	var labels map[string]string
	app, err := apps.CreateAppWithManifest(locator, []byte(manifest), ioutil.NopCloser(&data), labels)
	c.Assert(err, IsNil)
	c.Assert(app, NotNil)

	app, err = apps.GetApp(locator)
	c.Assert(err, IsNil)
	c.Assert(app, NotNil)
	c.Assert(app.Manifest.Metadata.Name, Equals, "kubernetes")
	c.Assert(app.Manifest.Metadata.ResourceVersion, Equals, "0.0.1")
	c.Logf("created %v", app)
}

func (r *AppsSuite) CreatesApplication(c *C) {
	apps := r.NewService(c, nil, nil)
	apptest.CreateRuntimeApplication(apps, c)
	app := loc.MustParseLocator("example.com/example-app:0.0.1")
	apptest.CreateDummyApplication(app, c, apps)
}

func (r *AppsSuite) DeletesApplication(c *C) {
	apps := r.NewService(c, nil, nil)
	apptest.CreateRuntimeApplication(apps, c)
	loc := loc.MustParseLocator("example.com/example-app:0.0.1")
	application := apptest.CreateDummyApplication(loc, c, apps)

	c.Assert(apps.DeleteApp(app.DeleteRequest{Package: application.Package}), IsNil)

	// make sure both package and application records are gone
	_, err := r.Packages.ReadPackageEnvelope(application.Package)
	c.Assert(trace.IsNotFound(err), Equals, true, Commentf("%T", err))

	_, err = apps.GetAppManifest(application.Package)
	c.Assert(trace.IsNotFound(err), Equals, true, Commentf("%T", err))
}

func (r *AppsSuite) ResolvesManifest(c *C) {
	const manifestBytesTemplate = `
apiVersion: bundle.gravitational.io/v2
kind: Runtime
metadata:
  name: template-app
  resourceVersion: "0.0.1"
systemOptions:
  dependencies:
    runtimePackage: gravitational.io/planet:0.0.1
dependencies:
  packages:
    - gravitational.io/package:0.0.1`

	const manifestBytesApp = `
apiVersion: bundle.gravitational.io/v2
kind: Bundle
metadata:
  name: sample
  resourceVersion: "0.0.1"
systemOptions:
  runtime:
    name: app-template
    version: 0.0.1
installer:
  flavors:
    prompt: Test flavors
    items:
      - name: flavor1
        nodes:
          - profile: worker
            count: 1
nodeProfiles:
  - name: worker
    description: "worker node"
    labels:
      node-role.kubernetes.io/master: "true"
      role: worker`

	apps := r.NewService(c, nil, nil)

	mainAppPackage := loc.MustParseLocator("gravitational.io/sample:0.0.1")
	templateAppPackage := loc.MustParseLocator("gravitational.io/app-template:0.0.1")
	dependencyPackage := loc.MustParseLocator("gravitational.io/package:0.0.1")
	runtimePackage := loc.MustParseLocator("gravitational.io/planet:0.0.1")

	packageFiles := []*archive.Item{archive.ItemFromString("./data", "hello")}
	apptest.CreatePackage(r.Packages, dependencyPackage, packageFiles, c)
	apptest.CreatePackage(r.Packages, runtimePackage, packageFiles, c)

	var emptyResources string
	mainFiles := []*archive.Item{
		archive.DirItem("resources"),
		archive.ItemFromString("resources/app.yaml", manifestBytesApp),
		archive.ItemFromString("resources/resources.yaml", emptyResources),
	}
	templateFiles := []*archive.Item{
		archive.DirItem("resources"),
		archive.ItemFromString("resources/app.yaml", manifestBytesTemplate),
		archive.ItemFromString("resources/resources.yaml", emptyResources),
	}

	apptest.CreateApplicationFromData(apps, templateAppPackage, templateFiles, c)
	mainApp := apptest.CreateApplicationFromData(apps, mainAppPackage, mainFiles, c)

	c.Assert(mainApp.Manifest.NodeProfiles, HasLen, 1)
	c.Assert(mainApp.Manifest.Dependencies, DeepEquals, schema.Dependencies{
		Packages: []schema.Dependency{
			{Locator: dependencyPackage},
		},
	})

	worker, err := mainApp.Manifest.NodeProfiles.ByName("worker")
	c.Assert(err, IsNil)
	c.Assert(worker.ServiceRole, Equals, schema.ServiceRoleMaster)
	c.Assert(worker.Labels, DeepEquals, map[string]string{
		schema.LabelRole:        "worker",
		constants.MasterLabel:   constants.True,
		schema.ServiceLabelRole: string(schema.ServiceRoleMaster),
	})
}

func (r *AppsSuite) AppHookCycle(c *C) {
	apps := r.NewService(c, nil, nil)
	runtimeManifest := `apiVersion: bundle.gravitational.io/v2
kind: Runtime
metadata:
  name: telekube
  resourceVersion: 0.0.1`
	runtimeLocator := loc.MustParseLocator(
		fmt.Sprintf("%v/%v:0.0.1", defaults.SystemAccountOrg, defaults.Runtime))
	runtimeItems := []*archive.Item{
		archive.DirItem("resources"),
		archive.ItemFromString("resources/app.yaml", runtimeManifest),
	}
	apptest.CreateApplicationFromData(apps, runtimeLocator, runtimeItems, c)
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "kube-system",
		},
		Spec: batchv1.JobSpec{
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					RestartPolicy: v1.RestartPolicyOnFailure,
					Containers: []v1.Container{
						{
							Name:            "hello-1",
							Image:           "quay.io/gravitational/debian-grande:buster",
							Command:         []string{"/bin/bash", "-c", "echo 'hello, world app hook'; sleep 1;"},
							ImagePullPolicy: v1.PullIfNotPresent,
						},
					},
				},
			},
		},
	}
	job.APIVersion = batchv1.SchemeGroupVersion.String()
	job.Kind = rigging.KindJob

	jobBytes, err := yaml.Marshal(job)
	c.Assert(err, IsNil)

	manifest := schema.Manifest{}
	manifest.APIVersion = "bundle.gravitational.io/v2"
	manifest.Kind = "Bundle"
	manifest.Metadata.Name = "hook"
	manifest.Metadata.ResourceVersion = "0.0.1"
	manifest.Hooks = &schema.Hooks{
		ClusterProvision: &schema.Hook{
			Job: string(jobBytes),
		},
	}

	manifestBytes, err := yaml.Marshal(manifest)
	c.Assert(err, IsNil)

	locator := loc.MustParseLocator(
		fmt.Sprintf("%v/%v:0.0.1", defaults.SystemAccountOrg, "hook"))
	items := []*archive.Item{
		archive.DirItem("resources"),
		archive.ItemFromString("resources/app.yaml", string(manifestBytes)),
	}

	data := apptest.CreatePackageData(items, c)

	var labels map[string]string
	application, err := apps.CreateAppWithManifest(locator, manifestBytes, ioutil.NopCloser(&data), labels)
	c.Assert(err, IsNil)
	c.Assert(application, NotNil)

	ctx, cancel := context.WithTimeout(context.TODO(), 60*time.Second)
	defer cancel()
	ref, err := apps.StartAppHook(ctx, app.HookRunRequest{
		Application: application.Package,
		Hook:        schema.HookClusterProvision,
		// Init containers try to pull package that does not exist
		// on remote cluster most likely
		SkipInitContainers: true,
	})
	c.Assert(err, IsNil)
	c.Assert(ref, NotNil)

	out := utils.NewSyncBuffer()
	go func() {
		err := apps.StreamAppHookLogs(ctx, *ref, writer(out))
		c.Assert(err, IsNil)
	}()

	err = apps.WaitAppHook(ctx, *ref)
	c.Assert(err, IsNil)
	c.Assert(utils.RemoveNewlines(out.String()), Matches, ".*hello, world app hook.*")

	err = apps.DeleteAppHookJob(ctx, app.DeleteAppHookJobRequest{HookRef: *ref})
	c.Assert(err, IsNil)

	err = apps.DeleteAppHookJob(ctx, app.DeleteAppHookJobRequest{HookRef: *ref})
	c.Assert(trace.IsNotFound(err), Equals, true, Commentf("expected not found, got %T", err))
}

// Charts tests support for vendoring helm charts
func (r *AppsSuite) Charts(c *C) {
	dockerClient, err := docker.NewClientFromEnv()
	if err != nil {
		c.Skip("this test requires docker")
	}

	dir := c.MkDir()
	config := docker.BasicConfiguration("127.0.0.1:0", dir)
	registry, err := docker.NewRegistry(config)
	c.Assert(err, IsNil)

	err = registry.Start()
	c.Assert(err, IsNil)
	defer registry.Close()

	imageService, err := docker.NewImageService(docker.RegistryConnectionRequest{
		RegistryAddress: defaults.DockerRegistry,
	})
	c.Assert(err, IsNil)

	vendorer, err := service.NewVendorer(service.VendorerConfig{
		DockerClient: dockerClient,
		ImageService: imageService,
		RegistryURL:  registry.Addr(),
		Packages:     r.Packages,
	})
	c.Assert(err, IsNil)

	apps := r.NewService(c, dockerClient, imageService)

	// import another version and check that resources have been updated
	const manifestBytes = `apiVersion: bundle.gravitational.io/v2
kind: SystemApplication
metadata:
  name: app
  resourceVersion: "0.0.1"`

	const chartManifestBytes = `name: mattermost
version: 2.2.1
description: Mattermost Example
keywords:
  - Demo of Mattermost chart with Gravity
tillerVersion: ">=2.8.0"
`

	const templateBytes = `
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
    image: {{.Values.registry}}alpine:latest
    ports:
      - containerPort: 80
  nodeSelector:
    role: webserver`

	const valuesBytes = `
registry:
`
	resources := []*archive.Item{
		archive.DirItem("resources"),
		archive.DirItem("resources/charts"),
		archive.DirItem("resources/charts/mattermost"),
		archive.DirItem("resources/charts/mattermost/templates"),
		archive.ItemFromString("resources/app.yaml", manifestBytes),
		archive.ItemFromString("resources/charts/mattermost/Chart.yaml", chartManifestBytes),
		archive.ItemFromString("resources/charts/mattermost/values.yaml", valuesBytes),
		archive.ItemFromString("resources/charts/mattermost/templates/pod.yaml", templateBytes),
	}

	application, reader := r.importApplicationWithResources(apps, vendorer, manifestBytes, resources, c)
	defer reader.Close()
	files, err := archive.FetchFiles(reader, []string{"registry"})
	c.Assert(err, IsNil)

	// files were not changed
	c.Assert(files["resources/charts/mattermost/Chart.yaml"], Equals, chartManifestBytes)
	c.Assert(files["resources/charts/mattermost/values.yaml"], Equals, valuesBytes)
	c.Assert(files["resources/charts/mattermost/templates/pod.yaml"], Equals, templateBytes)

	registryAddr := registryAddr(registry.Addr())
	// alpine was referenced as a part of helm template,
	// but make sure helm template has captured it
	// and added to the list of vendored apps
	c.Assert(apps.ExportApp(app.ExportAppRequest{
		Package:         application.Package,
		RegistryAddress: registryAddr,
	}), IsNil)

	ctx := context.Background()
	remoteRegistry, err := docker.ConnectRegistry(
		ctx, docker.RegistryConnectionRequest{
			RegistryAddress: registryAddr,
		})
	c.Assert(err, IsNil)
	repos := make([]string, 2)
	_, err = remoteRegistry.Repositories(ctx, repos, "")
	if err != io.EOF {
		c.Assert(err, IsNil)
	}
	sort.Strings(repos)
	c.Assert(repos, DeepEquals, []string{"alpine", "gravitational/debian-tall"})
}

func (r *AppsSuite) FetchChart(c *C) {
	apps := r.NewService(c, nil, nil)

	// Create a test Helm-based application.
	alpine := loc.MustParseLocator("gravitational.io/alpine:0.1.0")
	apptest.CreateHelmChartApp(c, apps, alpine)

	// Fetch it using the app service client.
	reader, err := apps.FetchChart(alpine)
	c.Assert(err, IsNil)
	defer reader.Close()

	// Load the chart archive and make sure it's valid.
	chart, err := loader.LoadArchive(reader)

	c.Assert(err, IsNil)
	compare.DeepCompare(c, chart, apptest.Chart(alpine))
}

func (r *AppsSuite) FetchIndexFile(c *C) {
	apps := r.NewService(c, nil, nil)

	// Create a test Helm-based application.
	alpine := loc.MustParseLocator("gravitational.io/alpine:0.1.0")
	apptest.CreateHelmChartApp(c, apps, alpine)

	// Fetch and parse the index file.
	reader, err := apps.FetchIndexFile()
	c.Assert(err, IsNil)
	indexFileBytes, err := ioutil.ReadAll(reader)
	c.Assert(err, IsNil)
	var indexFile repo.IndexFile
	err = yaml.Unmarshal(indexFileBytes, &indexFile)
	c.Assert(err, IsNil)

	// Make sure the application is there.
	c.Assert(len(indexFile.Entries), Equals, 1)
	c.Assert(indexFile.Has(alpine.Name, alpine.Version), Equals, true)
}

func writer(in io.Writer) io.Writer {
	if testing.Verbose() {
		return io.MultiWriter(in, os.Stdout)
	}
	return in
}

func (r *AppsSuite) importApplicationWithResources(apps app.Applications, vendorer service.Vendorer, manifestBytes string, resources []*archive.Item, c *C) (*app.Application, io.ReadCloser) {
	locator := loc.MustParseLocator("gravitational.io/app:0.0.1")
	manifest, err := schema.ParseManifestYAML([]byte(manifestBytes))
	c.Assert(err, IsNil)
	application := &app.Application{
		Package: locator,
		PackageEnvelope: pack.PackageEnvelope{
			Locator:   locator,
			SizeBytes: 0,
			SHA512:    "",
			Type:      string(storage.AppService),
			Manifest:  []byte(manifestBytes),
		},
		Manifest: *manifest,
	}
	service.PostProcessManifest(manifest)

	buf := apptest.CreatePackageData(resources, c)
	input := ioutil.NopCloser(&buf)

	errorc := make(chan error, 1)
	progressc := make(chan *app.ProgressEntry)
	req := &app.ImportRequest{
		Source:           input,
		Repository:       "gravitational.io",
		PackageName:      "app",
		PackageVersion:   "0.0.1",
		ErrorC:           errorc,
		ProgressC:        progressc,
		ResourcePatterns: []string{"**/*.yaml"},
	}

	if vendorer != nil {
		dir, err := vendorer.VendorTarball(
			context.Background(),
			input, service.VendorRequest{
				Repository:       req.Repository,
				PackageName:      req.PackageName,
				PackageVersion:   req.PackageVersion,
				ResourcePatterns: req.ResourcePatterns,
			})
		c.Assert(err, IsNil)
		defer os.RemoveAll(dir)
		req.Source, err = dockerarchive.Tar(dir, dockerarchive.Uncompressed)
		c.Assert(err, IsNil)
	}
	op, err := apps.CreateImportOperation(req)
	c.Assert(err, IsNil)
	c.Assert(op, NotNil)

	var progressEntry *app.ProgressEntry
	for progressEntry = range progressc {
	}
	c.Assert(<-errorc, IsNil)
	c.Assert(progressEntry.IsCompleted(), Equals, true)
	c.Assert(progressEntry.State, Equals, string(app.ProgressStateCompleted))

	importedApplication, err := apps.GetImportedApplication(*op)
	importedApplication.Manifest.Metadata.CreatedTimestamp = time.Time{}

	c.Assert(err, IsNil)
	// Adopt attributes that were not known a-priori
	application.PackageEnvelope.SizeBytes = importedApplication.PackageEnvelope.SizeBytes
	application.PackageEnvelope.SHA512 = importedApplication.PackageEnvelope.SHA512
	application.PackageEnvelope.Created = importedApplication.PackageEnvelope.Created
	// As the manifest is rewritten during import we cannot rely on textual comparision
	// TODO: this step requires a call to resolveManifest to update manifest
	// importedManifest, err := schema.ParseManifestYAML([]byte(importedApplication.PackageEnvelope.Manifest))
	// c.Assert(importedApplication.Manifest, DeepEquals, manifest)
	importedApplication.PackageEnvelope.Manifest = nil
	application.PackageEnvelope.Manifest = nil
	c.Assert(importedApplication, DeepEquals, application)

	_, reader, err := r.Packages.ReadPackage(importedApplication.Package)
	c.Assert(err, IsNil)

	return importedApplication, reader
}

func (r *AppsSuite) importApplication(apps app.Applications, vendorer service.Vendorer, c *C) *app.Application {
	const manifestBytes = `apiVersion: bundle.gravitational.io/v2
kind: SystemApplication
metadata:
  name: app
  resourceVersion: "0.0.1"`

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
    image: alpine:latest
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

	resources := []*archive.Item{
		archive.DirItem("resources"),
		archive.ItemFromString("resources/app.yaml", manifestBytes),
		archive.ItemFromString("resources/resource-0.yaml", resourceBytes),
	}

	importedApplication, reader := r.importApplicationWithResources(apps, vendorer, manifestBytes, resources, c)
	defer reader.Close()
	archive.AssertArchiveHasFiles(c, reader, []string{"registry"}, "resources", "resources/resource-0.yaml", "resources/app.yaml")
	return importedApplication
}

func registryAddr(addr string) string {
	return fmt.Sprintf("http://%v", addr)
}
