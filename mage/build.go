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
	"os"
	"runtime"

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

// Go builds go binaries using consistent build environment.
func (Build) Go() (err error) {
	mg.Deps(Build.BuildContainer, Build.Selinux)

	m := root.Target("build:go")
	defer func() { m.Complete(err) }()

	packages := []string{"github.com/gravitational/gravity/tool/gravity", "github.com/gravitational/gravity/tool/tele"}
	if enterprise != "" {
		if err = hasE(); err != nil {
			return trace.Wrap(err)
		}

		packages = []string{"github.com/gravitational/gravity/e/tool/gravity", "github.com/gravitational/gravity/e/tool/tele"}
	}

	err = m.GolangBuild().
		SetGOOS("linux").
		SetGOARCH("amd64").
		SetEnv("GO111MODULE", "on").
		SetMod("vendor").
		AddTag("selinux").
		AddTag("selinux_embed").
		SetBuildContainer(fmt.Sprint("gravity-build:", buildVersion)).
		SetOutputPath(consistentBinDir()).
		AddLDFlags(buildFlags()).
		Build(context.TODO(), packages...)

	return trace.Wrap(err)
}

// Darwin builds go binaries on darwin platform (doesn't support cross compile).
func (Build) Darwin() (err error) {
	m := root.Target("build:darwin")
	defer func() { m.Complete(err) }()

	if runtime.GOOS != "darwin" {
		return trace.BadParameter("Cross-compile not currently supported, darwin builds need to be run on darwin OS")
	}

	err = m.GolangBuild().
		SetGOOS("darwin").
		SetGOARCH("amd64").
		SetEnv("GO111MODULE", "on").
		SetMod("vendor").
		AddLDFlags(buildFlags()).
		Build(context.TODO(), "github.com/gravitational/gravity/tool/tele")

	return trace.Wrap(err)
}

// BuildContainer creates a docker container as a consistent golang environment to use for software builds.
func (Build) BuildContainer() (err error) {
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
		SetDockerfile("build.assets/Dockerfile").
		Build(context.TODO(), "./build.assets")

	return trace.Wrap(err)
}

// Selinux builds internal selinux code
func (Build) Selinux() (err error) {
	if runtime.GOOS != "linux" {
		return
	}

	m := root.Target("build:selinux")
	defer func() { m.Complete(err) }()

	_, err = m.Exec().Run(context.TODO(), "make", "selinux")
	return trace.Wrap(err)
}
