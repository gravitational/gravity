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

	"github.com/gravitational/magnet"
	"github.com/gravitational/trace"

	"github.com/magefile/mage/mg"
)

type Build mg.Namespace

func (Build) All() {
	mg.SerialDeps(Build.Go)
}

func buildBoxName() string {
	return fmt.Sprint("gravity-build:", buildVersion)
}

// Go builds platform-native binaries
func (r Build) Go(ctx context.Context) (err error) {
	m := root.Target("build:go")
	defer func() { m.Complete(err) }()

	mg.Deps(Mkdir(consistentBinDir()))

	if runtime.GOOS == "darwin" {
		mg.CtxDeps(ctx, Build.Darwin)
		for _, binary := range []string{"gravity", "tele"} {
			err := relink(
				inOsArchBinDir("darwin", "amd64", binary),
				consistentBinDir(binary),
			)
			if err != nil {
				return trace.Wrap(err)
			}
		}
		return nil
	}

	mg.CtxDeps(ctx, Build.Linux)
	for _, binary := range []string{"gravity", "tele"} {
		err := relink(
			inOsArchBinDir("linux", "amd64", binary),
			consistentBinDir(binary),
		)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// Linux builds Go Linux binaries using consistent build environment.
func (Build) Linux(ctx context.Context) (err error) {
	mg.CtxDeps(ctx, Build.BuildContainer, Build.Selinux)

	m := root.Target("build:linux")
	defer func() { m.Complete(err) }()

	packages := []string{
		"github.com/gravitational/gravity/tool/gravity",
		"github.com/gravitational/gravity/tool/tele",
	}

	// TODO(dima): avoid hard-coding platform
	outputPath := inOsArchBinDir("linux", "amd64")
	mg.Deps(Mkdir(outputPath))

	// TODO(dima): use buildkit's multiarch support
	err = m.GolangBuild().
		SetGOOS("linux").
		SetGOARCH("amd64").
		SetEnv("GO111MODULE", "on").
		SetMod("vendor").
		AddTag("selinux", "selinux_embed").
		SetBuildContainerConfig(magnet.BuildContainer{
			Name: fmt.Sprint("gravity-build:", buildVersion),
			// In Go module mode we don't need to be in a specific directory
			ContainerPath: "/host",
		}).
		SetOutputPath(inOsArchContainerBinDir("linux", "amd64")).
		AddLDFlags(buildFlags()).
		Build(ctx, packages...)

	return trace.Wrap(err)
}

// Darwin builds Go binaries on the Darwin platform (doesn't support cross compile).
func (Build) Darwin(ctx context.Context) (err error) {
	m := root.Target("build:darwin")
	defer func() { m.Complete(err) }()

	// TODO(dima): avoid hard-coding platform
	outputPath := inOsArchBinDir("darwin", "amd64")
	mg.Deps(Mkdir(outputPath))

	// TODO(dima): use buildkit's multiarch support and build in container
	err = m.GolangBuild().
		SetGOOS("darwin").
		SetGOARCH("amd64").
		SetEnv("GO111MODULE", "on").
		SetMod("vendor").
		SetOutputPath(outputPath).
		AddLDFlags(buildFlags()).
		Build(ctx,
			"github.com/gravitational/gravity/tool/gravity",
			"github.com/gravitational/gravity/tool/tele",
		)

	return trace.Wrap(err)
}

// BuildContainer creates a docker container as a consistent Go environment to use for software builds.
func (Build) BuildContainer(ctx context.Context) (err error) {
	m := root.Target("build:buildContainer")
	defer func() { m.Complete(err) }()

	err = m.DockerBuild().
		AddTag(buildBoxName()).
		SetPull(true).
		SetBuildArg("GOLANG_VER", golangVersion).
		SetBuildArg("PROTOC_VER", grpcProtocVersion).
		SetBuildArg("PROTOC_PLATFORM", grpcProtocPlatform).
		SetBuildArg("GOGO_PROTO_TAG", grpcGoGoTag).
		SetBuildArg("GRPC_GATEWAY_TAG", grpcGatewayTag).
		SetBuildArg("GOLANGCI_VER", golangciVersion).
		SetBuildArg("UID", fmt.Sprint(os.Getuid())).
		SetBuildArg("GID", fmt.Sprint(os.Getgid())).
		SetDockerfile("build.assets/Dockerfile.buildx").
		Build(ctx, "./build.assets")

	return trace.Wrap(err)
}

// Selinux builds internal selinux code
func (Build) Selinux(ctx context.Context) (err error) {
	mg.CtxDeps(ctx, Build.SelinuxPolicy)

	m := root.Target("build:selinux")
	defer func() { m.Complete(err) }()

	uptodate := IsUpToDate("lib/system/selinux/internal/policy/policy_embed.go",
		"lib/system/selinux/internal/policy/policy.go",
		"lib/system/selinux/internal/policy/assets",
	)
	if uptodate {
		return nil
	}

	_, err = m.Exec().Run(ctx, "make", "-C", "lib/system/selinux")
	return trace.Wrap(err)
}

func (Build) SelinuxPolicy(ctx context.Context) (err error) {
	mg.Deps(Mkdir(root.inBuildDir("apps")))

	m := root.Target("build:selinuxpolicy")
	defer func() { m.Complete(err) }()

	tmpDir, err := ioutil.TempDir("", "build-selinux")
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	defer os.RemoveAll(tmpDir)

	cachePath := root.inBuildDir("apps", fmt.Sprint("selinux.", pkgSelinux.version, ".tar.gz"))

	if _, err := os.Stat(cachePath); err == nil {
		m.SetCached(true)
		_, err := m.Exec().
			// TODO(dima): have selinux makefile output to a subdirectory per
			// supported OS distribution instead of hardcoding it
			Run(ctx, "tar", "xf", cachePath, "-C", "lib/system/selinux/internal/policy/assets/centos")
		return trace.Wrap(err)
	}

	_, err = m.Exec().SetWD(tmpDir).Run(ctx,
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

	_, err = m.Exec().SetWD(tmpDir).Run(ctx,
		"git",
		"submodule",
		"update",
		"--init",
		"--recursive",
	)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = m.Exec().SetWD(tmpDir).Run(ctx, "make", "BUILDBOX_INSTANCE=")
	if err != nil {
		return trace.Wrap(err)
	}

	outputDir := filepath.Join(tmpDir, "output")
	mg.Deps(Mkdir(outputDir))

	_, err = m.Exec().
		Run(ctx, "tar", "czf", cachePath,
			"-C", outputDir,
			"gravity.pp.bz2",
			"container.pp.bz2",
			"gravity.statedir.fc.template",
		)
	return trace.Wrap(err)
}

func relink(old, new string) error {
	if err := os.Remove(new); err != nil && !os.IsNotExist(err) {
		return trace.ConvertSystemError(err)
	}
	return trace.ConvertSystemError(os.Symlink(old, new))
}
