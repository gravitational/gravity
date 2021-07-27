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
	"os"
	"runtime"

	"github.com/gravitational/magnet"
	"github.com/gravitational/trace"
	"github.com/magefile/mage/mg"
)

type Test mg.Namespace

func (Test) All(ctx context.Context) {
	mg.CtxDeps(ctx, Test.Unit, Test.Lint)
}

// Lint runs golangci linter against the repo.
func (Test) Lint(ctx context.Context) (err error) {
	mg.CtxDeps(ctx, Build.BuildContainer)

	m := root.Target("test:lint")
	defer func() { m.Complete(err) }()

	m.Printlnf("Running golangci-lint")
	m.Println("  Linter: ", golangciVersion)

	wd, err := os.Getwd()
	if err != nil {
		return trace.Wrap(err)
	}

	cacheDir := root.Config.AbsCacheDir()
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return trace.Wrap(trace.ConvertSystemError(err))
	}

	err = m.DockerRun().
		SetRemove(true).
		SetUID(fmt.Sprint(os.Getuid())).
		SetGID(fmt.Sprint(os.Getgid())).
		AddVolume(magnet.DockerBindMount{
			Source:      wd,
			Destination: "/host",
			Readonly:    true,
			Consistency: "cached",
		}).
		AddVolume(magnet.DockerBindMount{
			Source:      cacheDir,
			Destination: "/cache",
			Consistency: "cached",
		}).
		SetEnv("XDG_CACHE_HOME", "/cache").
		SetEnv("GOCACHE", "/cache/go").
		SetEnv("GO111MODULE", "on").
		SetWorkDir("/host").
		Run(ctx, buildBoxName(),
			"/usr/bin/dumb-init",
			"bash", "-c",
			"golangci-lint run --config .golangci.yml",
		)

	return trace.Wrap(err)
}

// Unit runs unit tests with the race detector enabled.
func (Test) Unit(ctx context.Context, pkg string) (err error) {
	mg.CtxDeps(ctx, Build.BuildContainer)

	m := root.Target("test:unit")
	defer func() { m.Complete(err) }()

	m.Println("Running unit tests")

	var packages = []string{pkg}
	if pkg == "" {
		packages = []string{
			"./lib/...",
			"./tool/...",
			"./e/lib/...",
			"./e/tool/...",
		}
	}

	tasks := runtime.NumCPU()
	if runtime.GOOS == "darwin" {
		// TODO(dima): arbitrary upper bound which seems to avoid
		// the race with docker-for-mac and cache mount triggering
		// input/output errors on parallel link attempts
		tasks = 4
	}
	err = m.GolangTest().
		SetRace(true).
		SetCacheResults(false).
		// Enable the use of docker inside the test container
		AddVolumes(magnet.DockerBindMount{
			Source:      "/var/run/docker.sock",
			Destination: "/var/run/docker.sock",
			Consistency: "cached",
		}).
		SetNetwork("host").
		SetUser(magnet.User{ID: "root"}).
		SetBuildContainerConfig(magnet.BuildContainer{
			Name:          buildBoxName(),
			ContainerPath: "/host",
		}).
		SetParallelTasks(tasks).
		SetEnv("GO111MODULE", "on").
		SetMod("vendor").
		SetCoverProfile("coverage.out").
		Test(ctx, packages...)
	return trace.Wrap(err)
}

// Cover converts the coverage profile to HTML.
func (Test) Cover(ctx context.Context) (err error) {
	mg.CtxDeps(ctx, Build.BuildContainer)

	m := root.Target("test:cover")
	defer func() { m.Complete(err) }()

	m.Println("Converting coverage profile to HTML")

	return trace.Wrap(m.GolangCover().
		SetBuildContainerConfig(magnet.BuildContainer{
			Name:          buildBoxName(),
			ContainerPath: "/host",
		}).
		SetEnv("GO111MODULE", "on").
		SetMod("vendor").
		SetProfile("coverage.out").
		SetOutput("coverage.html").
		Run(ctx))
}
