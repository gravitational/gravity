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

package mage

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gravitational/magnet"
	"github.com/gravitational/magnet/pkg/cp"
	"github.com/gravitational/trace"
	"github.com/magefile/mage/mg"
)

type Package mg.Namespace

type gravityPackage struct {
	repository string
	name       string
	version    string

	// env are any env variables that need to be passed to the underlying build commands
	env map[string]string

	// apps subcommands
	// apps from local repository
	setImage []string
	include  []string
	exclude  []string
	deps     []string

	// srcDir is the relative path within the local git repo to find the application
	srcDir string

	// gitRepo is the public git repository to use to build the application
	gitRepo string
	// gitBranch is the branch or tag of the repository to build
	gitBranch string

	// force indicates that we should always import this asset and ignore caches
	force bool
}

var sharedStateMutex sync.Mutex

var (
	pkgGravity = gravityPackage{
		repository: "gravitational.io",
		name:       "gravity",
		version:    buildVersion,
		force:      true,
	}

	pkgTeleport = gravityPackage{repository: "gravitational.io", name: "teleport", version: teleportTag}
	pkgFio      = gravityPackage{repository: "gravitational.io", name: "fio", version: fioPkgTag}
	pkgSelinux  = gravityPackage{repository: "gravitational.io", name: "selinux", version: selinuxVersion}

	pkgPlanet = gravityPackage{
		repository: "gravitational.io",
		name:       "planet",
		version:    planetVersion,
		gitBranch:  planetBranch,
		gitRepo:    "https://github.com/gravitational/planet",
		env: map[string]string{
			"PLANET_BUILD_TAG": planetVersion,
		},
	}

	pkgWebAssets = gravityPackage{
		repository: "gravitational.io",
		name:       "web-assets",
		version:    buildVersion,
	}

	pkgSiteApp = gravityPackage{
		repository: "gravitational.io",
		name:       "site",
		version:    buildVersion,
		setImage: []string{
			fmt.Sprint("site-app-hook:", buildVersion),
			fmt.Sprint("gravity-site:", buildVersion),
		},
		include: []string{"resources", "registry"},
		srcDir:  "assets/site-app",
	}

	pkgMonitoringApp = gravityPackage{
		repository: "gravitational.io",
		name:       "monitoring-app",
		version:    appMonitoringVersion,
		gitBranch:  appMonitoringBranch,
		gitRepo:    appMonitoringRepo,
	}

	pkgLoggingApp = gravityPackage{
		repository: "gravitational.io",
		name:       "logging-app",
		version:    appLoggingVersion,
		gitBranch:  appLoggingBranch,
		gitRepo:    appLoggingRepo,
	}

	pkgIngressApp = gravityPackage{
		repository: "gravitational.io",
		name:       "ingress-app",
		version:    appIngressVersion,
		gitBranch:  appIngressBranch,
		gitRepo:    appIngressRepo,
	}

	pkgStorageApp = gravityPackage{
		repository: "gravitational.io",
		name:       "storage-app",
		version:    appStorageVersion,
		gitBranch:  appStorageBranch,
		gitRepo:    appStorageRepo,
	}

	pkgTillerApp = gravityPackage{
		repository: "gravitational.io",
		name:       "tiller-app",
		version:    appTillerVersion,
		setImage:   []string{fmt.Sprint("gcr.io/kubernetes-helm/tiller:v", tillerVersion)},
		include:    []string{"resources", "registry"},
		srcDir:     "assets/tiller-app",
	}

	pkgRBAC = gravityPackage{
		repository: "gravitational.io",
		name:       "rbac-app",
		version:    appRBACVersion,
		include:    []string{"resources", "registry"},
		srcDir:     "assets/rbac-app",
	}

	pkgDNSApp = gravityPackage{
		repository: "gravitational.io",
		name:       "dns-app",
		version:    appDNSVersion,
		setImage:   []string{fmt.Sprint("dns-app-hooks:", appDNSVersion)},
		include:    []string{"resources", "registry"},
		srcDir:     "assets/dns-app",
	}

	pkgBandwagonApp = gravityPackage{
		repository: "gravitational.io",
		name:       "bandwagon",
		version:    appBandwagonVersion,
		gitBranch:  appBandwagonBranch,
		gitRepo:    appBandwagonRepo,
	}

	pkgTelekube = gravityPackage{
		repository: "gravitational.io",
		name:       "telekube",
		version:    buildVersion,
		srcDir:     "assets/telekube",
		env: map[string]string{
			"GRAVITY_K8S_VERSION": k8sVersion,
		},
		force: true,
	}

	pkgKubernetes = gravityPackage{
		repository: "gravitational.io",
		name:       "kubernetes",
		version:    buildVersion,
		exclude:    []string{"**/*.tf"},
		srcDir:     "assets/kubernetes",
		deps: []string{
			pkgGravity.Locator(),
			pkgTeleport.Locator(),
			pkgFio.Locator(),
			pkgPlanet.Locator(),
			pkgWebAssets.Locator(),
			pkgRBAC.Locator(),
			pkgDNSApp.Locator(),
			pkgLoggingApp.Locator(),
			pkgMonitoringApp.Locator(),
			pkgIngressApp.Locator(),
			pkgStorageApp.Locator(),
			pkgBandwagonApp.Locator(),
			pkgTillerApp.Locator(),
			pkgSiteApp.Locator(),
		},
		force: true,
	}
)

func (Package) Telekube() (err error) {
	mg.Deps(Build.Go, Package.K8s)

	m := root.Target("package:telekube")
	defer func() { m.Complete(err) }()

	return trace.Wrap(pkgTelekube.localAppImport(m, consistentStateDir()))
}

func (Package) K8s() (err error) {
	mg.Deps(Build.Go, Package.GravityPackage, Package.Teleport, Package.Fio, Package.Planet, Package.Web,
		Package.Site, Package.Monitoring, Package.Logging, Package.Ingress, Package.Storage, Package.Tiller,
		Package.Rbac, Package.DNS, Package.Bandwagon)

	m := root.Target("package:k8s")
	defer func() { m.Complete(err) }()

	return trace.Wrap(pkgKubernetes.localAppImport(m, consistentStateDir()))
}

func (Package) GravityPackage() (err error) {
	mg.Deps(Build.Go)

	m := root.Target("package:gravity-package")
	defer func() { m.Complete(err) }()

	// the gravity package operates directly on the version state directory, so use the shared lock
	sharedStateMutex.Lock()
	defer sharedStateMutex.Unlock()

	gravityPackage := fmt.Sprint("gravitational.io/gravity:", buildVersion)

	_, err = m.Exec().Run(context.TODO(),
		consistentGravityBin(),
		"--state-dir", consistentStateDir(),
		"package",
		"delete",
		gravityPackage,
		"--force",
	)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = m.Exec().Run(context.TODO(),
		consistentGravityBin(),
		"--state-dir", consistentStateDir(),
		"package",
		"import",
		consistentGravityBin(),
		gravityPackage,
	)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (Package) Teleport() (err error) {
	mg.Deps(Build.Go)

	m := root.Target("package:teleport")
	defer func() { m.Complete(err) }()

	cachePath := filepath.Join("build/apps", fmt.Sprint("teleport.", pkgTeleport.version, ".tar.gz"))

	_, err = os.Stat(cachePath)
	if !os.IsNotExist(err) {
		m.SetCached(true)
		return trace.Wrap(pkgTeleport.ImportPackage(m, cachePath))
	}

	tmpDir, err := ioutil.TempDir("", "build-teleport")
	if err != nil {
		return trace.ConvertSystemError(err)
	}

	defer os.RemoveAll(tmpDir)

	m.Println("  tmpDir:", tmpDir)
	m.Println()

	buildDir := filepath.Join(tmpDir, "build")

	for _, dir := range []string{
		filepath.Join(buildDir, "/rootfs/usr/bin"),
		filepath.Join(buildDir, "/rootfs/usr/share/teleport"),
		tmpDir,
	} {
		err = os.MkdirAll(dir, 0755)
		if err != nil {
			return trace.ConvertSystemError(err)
		}
	}

	_, err = m.Exec().SetWD(tmpDir).Run(context.TODO(),
		"git",
		"clone",
		"https://github.com/gravitational/teleport",
		"--branch", teleportRepoTag,
		"--depth=1",
		"./src",
	)
	if err != nil {
		return trace.Wrap(err)
	}

	buildAssets := filepath.Join(tmpDir, "src/build.assets")

	_, err = m.Exec().SetWD(buildAssets).
		Run(context.TODO(), "make", "build-binaries")
	if err != nil {
		return trace.Wrap(err)
	}

	err = os.MkdirAll(filepath.Join(buildDir), 0755)
	if err != nil {
		return trace.Wrap(err)
	}

	err = cp.Copy(cp.Config{
		Source:      filepath.Join("build.assets", "teleport.manifest.json"),
		Destination: filepath.Join(buildDir, "orbit.manifest.json"),
	})
	if err != nil {
		return trace.Wrap(err)
	}

	err = cp.Copy(cp.Config{
		Source:      filepath.Join(tmpDir, "src/build/"),
		Destination: filepath.Join(buildDir, "/rootfs/usr/bin/"),
	})
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = m.Exec().
		Run(context.TODO(), "tar", "-C", buildDir, "-czf", cachePath, ".")
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(pkgTeleport.ImportPackage(m, cachePath))
}

func (Package) Fio() (err error) {
	mg.Deps(Build.BuildContainer, Build.Go)

	m := root.Target("package:fio")
	defer func() { m.Complete(err) }()

	fioImage := fmt.Sprint("fio:", fioTag)

	err = m.DockerBuild().
		SetBuildArg("BUILD_BOX", buildBoxName()).
		SetBuildArg("FIO_BRANCH", fioTag).
		SetPull(false).
		AddTag(fioImage).
		Build(context.TODO(), "assets/fio")
	if err != nil {
		return trace.Wrap(err)
	}

	wd, _ := os.Getwd()

	err = m.DockerRun().
		SetRemove(true).
		//AddVolume(fmt.Sprint(filepath.Join(wd, "build/"), ":/local")).
		AddVolume(magnet.DockerBindMount{
			Source:      filepath.Join(wd, "build/"),
			Destination: "/local",
		}).
		Run(context.TODO(), fioImage, "cp", "/gopath/native/fio/fio", filepath.Join("/local", "fio"))
	if err != nil {
		return trace.Wrap(err)
	}

	defer os.Remove("build/fio")

	return trace.Wrap(pkgFio.ImportPackage(m, "build/fio"))
}

func (Package) Selinux() (err error) {
	mg.Deps(Build.BuildContainer, Build.Go)

	m := root.Target("package:selinux")
	defer func() { m.Complete(err) }()

	cachePath := filepath.Join("build/apps", fmt.Sprint("selinux.", pkgSelinux.version, ".tar.gz"))

	_, err = os.Stat(cachePath)
	if !os.IsNotExist(err) {
		m.SetCached(true)
		return trace.Wrap(pkgSelinux.ImportPackage(m, cachePath))
	}

	tmpDir, err := ioutil.TempDir("", "build-selinux")
	if err != nil {
		return trace.ConvertSystemError(err)
	}

	defer os.RemoveAll(tmpDir)

	m.Println("  tmpDir:", tmpDir)
	m.Println()

	err = os.MkdirAll(tmpDir, 0755)
	if err != nil {
		return trace.ConvertSystemError(err)
	}

	_, err = m.Exec().SetWD(tmpDir).Run(context.TODO(),
		"git",
		"clone",
		selinuxRepo,
		"--branch", selinuxBranch,
		"--depth=1",
		"./",
	)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = m.Exec().SetWD(tmpDir).Run(context.TODO(),
		"git",
		"submodule",
		"update",
		"--init",
		"--recursive",
	)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = m.Exec().SetWD(tmpDir).
		Run(context.TODO(), "make", "BUILDBOX_INSTANCE=")
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = m.Exec().SetWD(filepath.Join(tmpDir, "selinux/output")).
		Run(context.TODO(),
			"tar",
			"czf", cachePath,
			"gravity.pp.bz2", "container.pp.bz2", "gravity.statedir.fc.template",
		)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(pkgFio.ImportPackage(m, cachePath))
}

func (Package) Planet() (err error) {
	mg.Deps(Build.Go)

	m := root.Target("package:planet")
	defer func() { m.Complete(err) }()

	packageList, err := magnet.Output(context.TODO(),
		consistentGravityBin(),
		"--state-dir", consistentStateDir(),
		"package",
		"list",
	)
	if err != nil {
		return trace.Wrap(err)
	}

	if strings.Contains(packageList, pkgPlanet.Locator()) {
		m.SetCached(true)
		return nil
	}

	err = pkgPlanet.BuildApp()
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(pkgPlanet.ImportPackage(m, pkgPlanet.DefaultCachePath()))
}

func (Package) Web() (err error) {
	mg.Deps(Build.Go)

	m := root.Target("package:web-assets")
	defer func() { m.Complete(err) }()

	webImage := fmt.Sprint("telekube-oss-web:", pkgWebAssets.version)

	err = m.DockerBuild().AddTag(webImage).Build(context.TODO(), "web/")
	if err != nil {
		return trace.Wrap(err)
	}

	wd, _ := os.Getwd()

	err = m.DockerRun().
		SetRemove(true).
		SetUID(fmt.Sprint(os.Getuid())).
		SetGID(fmt.Sprint(os.Getgid())).
		//AddVolume(fmt.Sprint(filepath.Join(wd, "build"), ":/local")).
		AddVolume(magnet.DockerBindMount{
			Source:      filepath.Join(wd, "build"),
			Destination: "/local",
		}).
		Run(context.TODO(), webImage, "cp", "-r", "/web-assets.tar.gz", "/local/")
	if err != nil {
		return trace.Wrap(err)
	}

	defer os.Remove("build/web-assets.tar.gz")

	return trace.Wrap(pkgWebAssets.ImportPackage(m, "build/web-assets.tar.gz"))
}

func (Package) Site() (err error) {
	mg.Deps(Build.Go)

	m := root.Target("package:site")
	defer func() { m.Complete(err) }()

	err = m.DockerBuild().
		SetBuildArg("CHANGESET", fmt.Sprint("site-", buildVersion)).
		SetPull(true).
		AddTag(fmt.Sprint("site-app-hook:", buildVersion)).
		Build(context.TODO(), "assets/site-app/images/hook")
	if err != nil {
		return trace.Wrap(err)
	}

	err = m.DockerBuild().
		SetPull(true).
		AddTag(fmt.Sprint("gravity-site:", buildVersion)).
		CopyToContext(consistentGravityBin(), "/gravity", nil, nil).
		CopyToContext("assets/site-app/images/site", "/", nil, nil).
		Build(context.TODO(), "assets/site-app/images/site")
	if err != nil {
		m.Println(trace.DebugReport(err))
		return trace.Wrap(err)
	}

	return trace.Wrap(pkgSiteApp.BuildApp())
}

func (Package) Monitoring() (err error) {
	return trace.Wrap(pkgMonitoringApp.BuildApp())
}

func (Package) Logging() (err error) {
	return trace.Wrap(pkgLoggingApp.BuildApp())
}

func (Package) Ingress() (err error) {
	return trace.Wrap(pkgIngressApp.BuildApp())
}

func (Package) Storage() (err error) {
	return trace.Wrap(pkgStorageApp.BuildApp())
}

func (Package) Tiller() (err error) {
	return trace.Wrap(pkgTillerApp.BuildApp())
}

func (Package) Rbac() (err error) {
	return trace.Wrap(pkgRBAC.BuildApp())
}

func (Package) DNS() (err error) {
	m := root.Target("package:dns:containers")
	defer func() { m.Complete(err) }()

	err = m.DockerBuild().
		SetBuildArg("CHANGESET", fmt.Sprint("dns-app-", pkgDNSApp.version)).
		SetPull(true).
		AddTag(fmt.Sprint("dns-app-hooks:", pkgDNSApp.version)).
		Build(context.TODO(), "assets/dns-app/hooks")
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(pkgDNSApp.BuildApp())
}

func (Package) Bandwagon() (err error) {
	return trace.Wrap(pkgBandwagonApp.BuildApp())
}

func consistentStateDir() string {
	path := filepath.Join("build", root.Version, "state")

	err := os.MkdirAll(path, 0755)
	if err != nil {
		panic(trace.DebugReport(err))
	}

	return path
}

func consistentBinDir() string {
	path := filepath.Join("build", root.Version, "bin")

	err := os.MkdirAll(path, 0755)
	if err != nil {
		panic(trace.DebugReport(err))
	}

	return path
}

func consistentBuildDir() string {
	path := filepath.Join("build", root.Version)
	err := os.MkdirAll(path, 0755)
	if err != nil {
		panic(trace.DebugReport(err))
	}

	return path
}

func consistentGravityBin() string {
	return filepath.Join("build", root.Version, "bin/gravity")
}

func (p gravityPackage) Locator() string {
	return fmt.Sprint(p.repository, "/", p.name, ":", p.version)
}

func (p gravityPackage) BuildApp() (err error) {
	mg.Deps(Build.Go)

	m := root.Target(fmt.Sprint("package:", p.name, ":app"))
	defer func() { m.Complete(err) }()

	if !p.force {
		var cached bool
		cached, err = p.IsAppCachedAndSync(m, "")
		if err != nil {
			return trace.Wrap(err)
		}
		if cached {
			m.SetCached(true)
			return
		}
	}

	m.Println("Building App:")
	m.Println("  repository:", p.repository)
	m.Println("  name:", p.name)
	m.Println("  version:", p.version)
	m.Println("  setImage:", p.setImage)
	m.Println("  include:", p.include)
	m.Println("  exclude: ", p.exclude)
	m.Println("  deps: ", p.deps)
	m.Println("  gitRepo: ", p.gitRepo)
	m.Println("  gitBranch: ", p.gitBranch)
	m.Println("  cachePath: ", p.DefaultCachePath())
	m.Println("  env: ", p.env)

	if p.gitRepo != "" {
		err = p.buildGit(m)
		if err != nil {
			return trace.Wrap(err)
		}
	} else {
		err = p.buildLocal(m)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	// Don't sync planet, it's not an application
	if p.name == pkgPlanet.name {
		return
	}

	return trace.Wrap(p.SyncAppToStateDir(m, p.DefaultCachePath()))
}

func (p gravityPackage) DefaultCachePath() string {
	return filepath.Join("build/apps", p.repository, fmt.Sprint(p.name, ".", p.version, ".tar.gz"))
}

// IsAppCachedAndSync checks whether the package is available in the cache and if missing syncs to the active state
// directory.
// path allows overiding the binary path from the default location.
func (p gravityPackage) IsAppCachedAndSync(m *magnet.Magnet, path string) (bool, error) {
	if path == "" {
		path = p.DefaultCachePath()
	}

	// don't sync planet
	if p.name == pkgPlanet.name {
		return false, nil
	}

	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false, nil
	}

	if err != nil {
		return false, trace.Wrap(err)
	}

	// we found the cache file, so try and make sure it's in our state directory
	err = p.SyncAppToStateDir(m, path)

	return true, trace.Wrap(err)
}

func (p gravityPackage) SyncAppToStateDir(m *magnet.Magnet, path string) error {
	if !p.force {
		packageList, err := magnet.Output(context.TODO(),
			consistentGravityBin(),
			"--state-dir", consistentStateDir(),
			"package",
			"list",
		)
		if err != nil {
			return trace.Wrap(err)
		}

		if strings.Contains(packageList, p.Locator()) {
			return nil
		}
	}

	// I'm not sure the gravity package store is really protected against concurrent access
	// so until we're sure, have operations take a lock
	sharedStateMutex.Lock()
	defer sharedStateMutex.Unlock()

	_, err := m.Exec().Run(
		context.TODO(),
		consistentGravityBin(),
		"--state-dir", consistentStateDir(),
		"app", "delete",
		p.Locator(),
		"--force",
	)
	if err != nil {
		return trace.Wrap(err)
	}

	args := []string{
		"--state-dir", consistentStateDir(),
		"app", "import",
		path,
		"--vendor",
		"--repository=gravitational.io",
	}
	if p.name == pkgPlanet.name {
		args = append(args, "--labels=purpose:runtime")
	}

	_, err = m.Exec().Run(
		context.TODO(),
		consistentGravityBin(),
		args...,
	)

	return trace.Wrap(err)
}

func (p gravityPackage) buildLocal(m *magnet.Magnet) error {
	stateDir, err := ioutil.TempDir("", fmt.Sprint("build-app-", p.name))
	if err != nil {
		return trace.ConvertSystemError(err)
	}

	defer os.RemoveAll(stateDir)

	err = p.localAppImport(m, stateDir)
	if err != nil {
		return trace.Wrap(err)
	}

	err = os.MkdirAll(filepath.Dir(p.DefaultCachePath()), 0755)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = m.Exec().Run(
		context.TODO(),
		consistentGravityBin(),
		"--state-dir", stateDir,
		"--debug",
		"package", "export",
		p.Locator(),
		p.DefaultCachePath(),
	)

	return trace.Wrap(err)
}

func (p gravityPackage) buildGit(m *magnet.Magnet) error {
	tmpDir, err := ioutil.TempDir("", fmt.Sprint("build-app-", p.name))
	if err != nil {
		return trace.ConvertSystemError(err)
	}

	defer os.RemoveAll(tmpDir)

	m.Println("  tmpDir:", tmpDir)
	m.Println()

	srcDir := filepath.Join(tmpDir, "src")
	stateDir := filepath.Join(tmpDir, "state")

	for _, dir := range []string{srcDir, stateDir, filepath.Dir(p.DefaultCachePath())} {
		err = os.MkdirAll(dir, 0755)
		if err != nil {
			return trace.ConvertSystemError(err)
		}
	}

	_, err = m.Exec().SetWD(srcDir).Run(context.TODO(),
		"git",
		"clone",
		p.gitRepo,
		"--branch", p.gitBranch,
		"--depth=1",
		".",
	)
	if err != nil {
		return trace.Wrap(err)
	}

	wd, err := os.Getwd()
	if err != nil {
		return trace.Wrap(err)
	}

	// planet has a different target than other repos
	target := "import"
	if p.name == pkgPlanet.name {
		target = "production"
	}

	envs := map[string]string{
		"GRAVITY":  fmt.Sprint(filepath.Join(wd, consistentGravityBin()), " --state-dir ", stateDir),
		"VERSION":  p.version,
		"OPS_URL":  "",
		"BUILDDIR": stateDir,
		"USER":     "jenkins",
	}
	for k, v := range p.env {
		envs[k] = v
	}

	_, err = m.Exec().SetWD(srcDir).SetEnvs(envs).Run(context.TODO(), "make", target)
	if err != nil {
		return trace.Wrap(err)
	}

	// planet builds a bit differently, so copy/cache the build and exit
	if p.name == pkgPlanet.name {
		return trace.Wrap(cp.Copy(cp.Config{
			Source:      filepath.Join(stateDir, "planet.tar.gz"),
			Destination: p.DefaultCachePath(),
		}))
	}

	_, err = m.Exec().Run(
		context.TODO(),
		consistentGravityBin(),
		"--state-dir", stateDir,
		"--debug",
		"package", "export",
		p.Locator(),
		p.DefaultCachePath(),
	)
	return trace.Wrap(err)
}

func (p gravityPackage) localAppImport(m *magnet.Magnet, stateDir string) error {
	_, err := m.Exec().Run(
		context.TODO(),
		consistentGravityBin(),
		"--state-dir", stateDir,
		"app", "delete",
		p.Locator(),
		"--force",
	)
	if err != nil {
		return trace.Wrap(err)
	}

	args := []string{
		"--state-dir", stateDir,
		"app", "import",
		"--vendor",
		"--repository", p.repository,
		"--name", p.name,
		"--version", p.version,
	}

	for _, i := range p.setImage {
		args = append(args, "--set-image", i)
	}

	for _, i := range p.include {
		args = append(args, "--include", i)
	}

	for _, i := range p.exclude {
		args = append(args, "--exclude", i)
	}

	for _, i := range p.deps {
		args = append(args, "--set-dep", i)
	}

	args = append(args, p.srcDir)

	_, err = m.Exec().SetEnvs(p.env).Run(
		context.TODO(),
		consistentGravityBin(),
		args...,
	)

	return trace.Wrap(err)
}

func (p gravityPackage) ImportPackage(m *magnet.Magnet, path string) error {
	// I'm not sure the gravity package store is really protected against concurrent access
	// so until we're sure, have operations take a lock
	sharedStateMutex.Lock()
	defer sharedStateMutex.Unlock()

	_, err := m.Exec().Run(
		context.TODO(),
		consistentGravityBin(),
		"--state-dir", consistentStateDir(),
		"package", "delete",
		p.Locator(),
		"--force",
	)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = m.Exec().Run(
		context.TODO(),
		consistentGravityBin(),
		"--state-dir", consistentStateDir(),
		"package", "import",
		path,
		p.Locator(),
	)
	return trace.Wrap(err)
}
