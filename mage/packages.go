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

package mage

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
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
		gitBranch:  planetVersion,
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
		force:   true,
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

func (Package) Telekube(ctx context.Context) (err error) {
	mg.CtxDeps(ctx, Build.Go, Package.K8s, Mkdir(consistentStateDir()))

	m := root.Target("package:telekube")
	defer func() { m.Complete(err) }()

	return trace.Wrap(pkgTelekube.localAppImport(ctx, m, consistentStateDir()))
}

func (Package) K8s(ctx context.Context) (err error) {
	mg.CtxDeps(ctx, Mkdir(consistentStateDir()),
		Build.Go, Package.Gravity, Package.Teleport,
		Package.Fio, Package.Planet, Package.Web,
		Package.Site, Package.Monitoring, Package.Logging,
		Package.Ingress, Package.Storage, Package.Tiller,
		Package.Rbac, Package.DNS, Package.Bandwagon)

	m := root.Target("package:k8s")
	defer func() { m.Complete(err) }()

	return trace.Wrap(pkgKubernetes.localAppImport(ctx, m, consistentStateDir()))
}

func (Package) Gravity(ctx context.Context) (err error) {
	if runtime.GOOS == "darwin" {
		// Build linux/amd64 gravity binary additionally for packaging
		mg.CtxDeps(ctx, Mkdir(consistentStateDir()), Build.Go, Build.Linux)
	} else {
		mg.CtxDeps(ctx, Mkdir(consistentStateDir()), Build.Go)
	}

	m := root.Target("package:gravity")
	defer func() { m.Complete(err) }()

	// the gravity package operates directly on the version state directory, so use the shared lock
	sharedStateMutex.Lock()
	defer sharedStateMutex.Unlock()

	gravityPackage := fmt.Sprint("gravitational.io/gravity:", buildVersion)

	_, err = m.Exec().Run(ctx,
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

	_, err = m.Exec().Run(ctx,
		consistentGravityBin(),
		"--state-dir", consistentStateDir(),
		"package",
		"import",
		osArchGravityBin("linux", "amd64"),
		gravityPackage,
	)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (Package) Teleport(ctx context.Context) (err error) {
	mg.CtxDeps(ctx, Build.Go, Mkdir(root.inBuildDir("apps")))

	m := root.Target("package:teleport")
	defer func() { m.Complete(err) }()

	cachePath := root.inBuildDir("apps", fmt.Sprint("teleport.", pkgTeleport.version, ".tar.gz"))

	_, err = os.Stat(cachePath)
	if !os.IsNotExist(err) {
		m.SetCached(true)
		return trace.Wrap(pkgTeleport.ImportPackage(ctx, m, cachePath))
	}

	tmpDir, err := ioutil.TempDir("", "build-teleport")
	if err != nil {
		return trace.ConvertSystemError(err)
	}

	defer os.RemoveAll(tmpDir)

	m.Println("  tmpDir:", tmpDir)
	m.Println()

	buildDir := filepath.Join(tmpDir, "build")
	mg.Deps(Mkdir(buildDir))

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

	_, err = m.Exec().SetWD(tmpDir).Run(ctx,
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
		Run(ctx, "make", "build-binaries")
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
		Run(ctx, "tar", "-C", buildDir, "-czf", cachePath, ".")
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(pkgTeleport.ImportPackage(ctx, m, cachePath))
}

func (Package) Fio(ctx context.Context) (err error) {
	mg.CtxDeps(ctx, Build.BuildContainer, Build.Go)

	m := root.Target("package:fio")
	defer func() { m.Complete(err) }()

	fioImage := fmt.Sprint("fio:", fioTag)

	err = m.DockerBuild().
		SetBuildArg("BUILD_BOX", buildBoxName()).
		SetBuildArg("FIO_BRANCH", fioTag).
		SetPull(false).
		AddTag(fioImage).
		Build(ctx, "assets/fio")
	if err != nil {
		return trace.Wrap(err)
	}

	err = m.DockerRun().
		SetRemove(true).
		AddVolume(magnet.DockerBindMount{
			Source:      root.buildDir,
			Destination: "/local",
		}).
		Run(ctx, fioImage, "cp", "/gopath/native/fio/fio", filepath.Join("/local", "fio"))
	if err != nil {
		return trace.Wrap(err)
	}

	defer os.Remove(root.inBuildDir("fio"))

	return trace.Wrap(pkgFio.ImportPackage(ctx, m, root.inBuildDir("fio")))
}

func (Package) Planet(ctx context.Context) (err error) {
	mg.CtxDeps(ctx, Mkdir(consistentStateDir()), Build.Go)

	m := root.Target("package:planet")
	defer func() { m.Complete(err) }()

	packageList, err := magnet.Output(ctx,
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

	labels := []string{"purpose", "runtime"}
	if _, err := os.Stat(pkgPlanet.defaultCachePath()); !os.IsNotExist(err) {
		m.SetCached(true)
		return trace.Wrap(pkgPlanet.ImportPackage(ctx, m, pkgPlanet.defaultCachePath(), labels...))
	}

	err = pkgPlanet.BuildApp(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(pkgPlanet.ImportPackage(ctx, m, pkgPlanet.defaultCachePath(), labels...))
}

func (Package) Web(ctx context.Context) (err error) {
	mg.CtxDeps(ctx, Build.Go)

	m := root.Target("package:web-assets")
	defer func() { m.Complete(err) }()

	webImage := fmt.Sprint("telekube-oss-web:", pkgWebAssets.version)
	contextPath := "web/"

	err = m.DockerBuild().
		AddTag(webImage).
		SetBuildArg("NODE_VER", "12.18.3-buster").
		SetDockerfile("web/Dockerfile.buildx").
		Build(ctx, contextPath)
	if err != nil {
		return trace.Wrap(err)
	}

	err = m.DockerRun().
		SetRemove(true).
		SetUID(fmt.Sprint(os.Getuid())).
		SetGID(fmt.Sprint(os.Getgid())).
		AddVolume(magnet.DockerBindMount{
			Source:      root.buildDir,
			Destination: "/local",
		}).
		Run(ctx, webImage, "cp", "-r", "/web-assets.tar.gz", "/local/")
	if err != nil {
		return trace.Wrap(err)
	}

	defer os.Remove(root.inBuildDir("web-assets.tar.gz"))

	return trace.Wrap(pkgWebAssets.ImportPackage(ctx, m, root.inBuildDir("web-assets.tar.gz")))
}

func (Package) Site(ctx context.Context) (err error) {
	mg.CtxDeps(ctx, Build.Linux)

	m := root.Target("package:site")
	defer func() { m.Complete(err) }()

	err = m.DockerBuild().
		SetBuildArg("CHANGESET", fmt.Sprint("site-", buildVersion)).
		SetPull(true).
		AddTag(fmt.Sprint("site-app-hook:", buildVersion)).
		Build(ctx, "assets/site-app/images/hook")
	if err != nil {
		return trace.Wrap(err)
	}

	err = m.DockerBuild().
		SetPull(true).
		AddTag(fmt.Sprint("gravity-site:", buildVersion)).
		CopyToContext(osArchGravityBin("linux", "amd64"), "/gravity", nil, nil).
		CopyToContext("assets/site-app/images/site", "/", nil, nil).
		Build(ctx, "assets/site-app/images/site")
	if err != nil {
		m.Println(trace.DebugReport(err))
		return trace.Wrap(err)
	}

	return trace.Wrap(pkgSiteApp.BuildApp(ctx))
}

func (Package) Monitoring(ctx context.Context) (err error) {
	return trace.Wrap(pkgMonitoringApp.BuildApp(ctx))
}

func (Package) Logging(ctx context.Context) (err error) {
	return trace.Wrap(pkgLoggingApp.BuildApp(ctx))
}

func (Package) Ingress(ctx context.Context) (err error) {
	return trace.Wrap(pkgIngressApp.BuildApp(ctx))
}

func (Package) Storage(ctx context.Context) (err error) {
	return trace.Wrap(pkgStorageApp.BuildApp(ctx))
}

func (Package) Tiller(ctx context.Context) (err error) {
	return trace.Wrap(pkgTillerApp.BuildApp(ctx))
}

func (Package) Rbac(ctx context.Context) (err error) {
	return trace.Wrap(pkgRBAC.BuildApp(ctx))
}

func (Package) DNS(ctx context.Context) (err error) {
	m := root.Target("package:dns:containers")
	defer func() { m.Complete(err) }()

	err = m.DockerBuild().
		SetBuildArg("CHANGESET", fmt.Sprint("dns-app-", pkgDNSApp.version)).
		SetPull(true).
		AddTag(fmt.Sprint("dns-app-hooks:", pkgDNSApp.version)).
		Build(ctx, "assets/dns-app/hooks")
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(pkgDNSApp.BuildApp(ctx))
}

func (Package) Bandwagon(ctx context.Context) (err error) {
	return trace.Wrap(pkgBandwagonApp.BuildApp(ctx))
}

func consistentStateDir() string {
	return root.inVersionedBuildDir("state")
}

func inOsArchBinDir(targetOS, targetArch string, subelems ...string) string {
	elems := append([]string{"bin"}, fmt.Sprint(targetOS, "-", targetArch))
	return root.inVersionedBuildDir(append(elems, subelems...)...)
}

func inOsArchContainerBinDir(targetOS, targetArch string, subelems ...string) string {
	elems := append([]string{"bin"}, fmt.Sprint(targetOS, "-", targetArch))
	return root.inVersionedContainerBuildDir(append(elems, subelems...)...)
}

func consistentContainerBinDir(elems ...string) string {
	return root.inVersionedContainerBuildDir(append([]string{"bin"}, elems...)...)
}

func consistentBinDir(elems ...string) string {
	return root.inVersionedBuildDir(append([]string{"bin"}, elems...)...)
}

func consistentBuildDir() string {
	return root.inVersionedBuildDir()
}

func osArchGravityBin(os, arch string) string {
	return inOsArchBinDir(os, arch, "gravity")
}

func consistentGravityBin() string {
	return root.inVersionedBuildDir("bin", "gravity")
}

func (p gravityPackage) Locator() string {
	return fmt.Sprint(p.repository, "/", p.name, ":", p.version)
}

func (p gravityPackage) BuildApp(ctx context.Context) (err error) {
	mg.CtxDeps(ctx, Build.Go)

	m := root.Target(fmt.Sprint("package:", p.name, ":app"))
	defer func() { m.Complete(err) }()

	if !p.force {
		var cached bool
		cached, err = p.IsAppCachedAndSync(ctx, m, "")
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
	m.Println("  cachePath: ", p.defaultCachePath())
	m.Println("  env: ", p.env)

	if p.gitRepo != "" {
		err = p.buildGit(ctx, m)
		if err != nil {
			return trace.Wrap(err)
		}
	} else {
		err = p.buildLocal(ctx, m)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	// Don't sync planet, it's not an application
	if p.name == pkgPlanet.name {
		return
	}

	return trace.Wrap(p.SyncAppToStateDir(ctx, m, p.defaultCachePath()))
}

func (p gravityPackage) defaultCacheDir() string {
	return root.inBuildDir("apps", p.repository)
}

func (p gravityPackage) defaultCachePath() string {
	return root.inBuildDir("apps", p.repository, fmt.Sprint(p.name, ".", p.version, ".tar.gz"))
}

// IsAppCachedAndSync checks whether the package is available in the cache and if missing syncs to the active state
// directory.
// path allows overiding the binary path from the default location.
func (p gravityPackage) IsAppCachedAndSync(ctx context.Context, m *magnet.MagnetTarget, path string) (bool, error) {
	if path == "" {
		path = p.defaultCachePath()
	}

	// don't sync planet
	//  unknown long flag '--labels'
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
	err = p.SyncAppToStateDir(ctx, m, path)

	return true, trace.Wrap(err)
}

func (p gravityPackage) SyncAppToStateDir(ctx context.Context, m *magnet.MagnetTarget, path string) error {
	mg.Deps(Mkdir(consistentStateDir()))
	if !p.force {
		packageList, err := magnet.Output(ctx,
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
		ctx,
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
		ctx,
		consistentGravityBin(),
		args...,
	)

	return trace.Wrap(err)
}

func (p gravityPackage) buildLocal(ctx context.Context, m *magnet.MagnetTarget) error {
	mg.Deps(Mkdir(p.defaultCacheDir()))

	stateDir, err := ioutil.TempDir("", fmt.Sprint("build-app-", p.name))
	if err != nil {
		return trace.ConvertSystemError(err)
	}

	defer os.RemoveAll(stateDir)

	err = p.localAppImport(ctx, m, stateDir)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = m.Exec().Run(
		ctx,
		consistentGravityBin(),
		"--state-dir", stateDir,
		"--debug",
		"package", "export",
		p.Locator(),
		p.defaultCachePath(),
	)

	return trace.Wrap(err)
}

func (p gravityPackage) buildGit(ctx context.Context, m *magnet.MagnetTarget) error {
	tmpDir, err := ioutil.TempDir("", fmt.Sprint("build-app-", p.name))
	if err != nil {
		return trace.ConvertSystemError(err)
	}

	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			m.Println("Failed to clean up temporary directory %q: %v", tmpDir, err)
		}
	}()

	m.Println("  tmpDir:", tmpDir)
	m.Println()

	srcDir := filepath.Join(tmpDir, "src")
	stateDir := filepath.Join(tmpDir, "state")

	for _, dir := range []string{srcDir, stateDir, filepath.Dir(p.defaultCachePath())} {
		err = os.MkdirAll(dir, 0755)
		if err != nil {
			return trace.ConvertSystemError(err)
		}
	}

	_, err = m.Exec().SetWD(srcDir).Run(ctx,
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

	envs := map[string]string{
		"GRAVITY": fmt.Sprint(consistentGravityBin(), " --state-dir ", stateDir),
		"VERSION": p.version,
		"OPS_URL": "",
		"USER":    "jenkins",
	}
	if buildkitHost, ok := os.LookupEnv("BUILDKIT_HOST"); ok {
		envs["BUILDKIT_HOST"] = buildkitHost
	}
	for k, v := range p.env {
		envs[k] = v
	}

	if p.name == pkgPlanet.name {
		envs["OUTPUTDIR"] = stateDir
		_, err = m.Exec().SetWD(srcDir).SetEnvs(envs).Run(ctx, "make", "-f", "Makefile.buildx", "tarball")
	} else {
		envs["BUILDDIR"] = stateDir
		_, err = m.Exec().SetWD(srcDir).SetEnvs(envs).Run(ctx, "make", "import")
	}
	if err != nil {
		return trace.Wrap(err)
	}

	// planet builds a bit differently, so copy/cache the build and exit
	if p.name == pkgPlanet.name {
		return trace.Wrap(cp.Copy(cp.Config{
			Source:      filepath.Join(stateDir, "planet.tar.gz"),
			Destination: p.defaultCachePath(),
		}))
	}

	_, err = m.Exec().Run(
		ctx,
		consistentGravityBin(),
		"--state-dir", stateDir,
		"--debug",
		"package", "export",
		p.Locator(),
		p.defaultCachePath(),
	)
	return trace.Wrap(err)
}

func (p gravityPackage) localAppImport(ctx context.Context, m *magnet.MagnetTarget, stateDir string) error {
	_, err := m.Exec().Run(
		ctx,
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
		"--debug",
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
		ctx,
		consistentGravityBin(),
		args...,
	)

	return trace.Wrap(err)
}

func (p gravityPackage) ImportPackage(ctx context.Context, m *magnet.MagnetTarget, path string, labelPairs ...string) error {
	mg.Deps(Mkdir(consistentStateDir()))
	// I'm not sure the gravity package store is really protected against concurrent access
	// so until we're sure, have operations take a lock
	sharedStateMutex.Lock()
	defer sharedStateMutex.Unlock()

	if len(labelPairs) != 0 && len(labelPairs)%2 != 0 {
		return trace.BadParameter("invalid label set: %q", labelPairs)
	}

	var labels []string
	for i := 0; i < len(labelPairs); i += 2 {
		labels = append(labels, fmt.Sprint(labelPairs[i], ":", labelPairs[i+1]))
	}

	_, err := m.Exec().Run(
		ctx,
		consistentGravityBin(),
		"--state-dir", consistentStateDir(),
		"package", "delete",
		p.Locator(),
		"--force",
	)
	if err != nil {
		return trace.Wrap(err)
	}

	args := []string{
		"--state-dir", consistentStateDir(),
		"package", "import",
	}
	if len(labels) != 0 {
		args = append(args, "--labels", strings.Join(labels, ","))
	}
	_, err = m.Exec().Run(
		ctx,
		consistentGravityBin(),
		append(args, path, p.Locator())...,
	)
	return trace.Wrap(err)
}
