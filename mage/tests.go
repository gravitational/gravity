package mage

import (
	"context"
	"fmt"
	"os"

	"github.com/gravitational/magnet"
	"github.com/gravitational/trace"
	"github.com/magefile/mage/mg"
)

type Test mg.Namespace

func (Test) All() {
	//mg.SerialDeps(Build.Go, Build.BuildContainer)
	mg.Deps(Test.Unit, Test.Lint)
}

// Lint runs golangci linter against the repo.
func (Test) Lint() (err error) {
	mg.Deps(Build.BuildContainer)

	m := root.Target("test:lint")
	defer func() { m.Complete(err) }()

	m.Printlnf("Running golangci-lint")

	wd, _ := os.Getwd()

	err = m.DockerRun().
		SetRemove(true).
		SetUID(fmt.Sprint(os.Getuid())).
		SetGID(fmt.Sprint(os.Getgid())).
		//AddVolume(fmt.Sprint(wd, ":/gopath/src/github.com/gravitational/gravity:ro,cached")).
		//AddVolume(fmt.Sprint(localCacheDir, ":/gopath/src/github.com/gravitational/gravity/build/cache:cached")).
		AddVolume(magnet.DockerBindMount{
			Source:      wd,
			Destination: "/gopath/src/github.com/gravitational/gravity",
			Readonly:    true,
			Consistency: "cached",
		}).
		AddVolume(magnet.DockerBindMount{
			Source:      magnet.AbsCacheDir(),
			Destination: "/cache",
			Consistency: "cached",
		}).
		SetEnv("XDG_CACHE_HOME", "/cache").
		SetEnv("GOCACHE", "/cache/go").
		Run(context.TODO(), buildBoxName(),
			"bash",
			"-c",
			"cd /gopath/src/github.com/gravitational/gravity && golangci-lint run --deadline=30m --enable-all "+
				"--modules-download-mode=vendor --print-resources-usage --new -D gochecknoglobals -D gochecknoinits",
		)

	return trace.Wrap(err)
}

// Unit runs unit tests with the race detector enabled.
func (Test) Unit() (err error) {
	mg.Deps(Build.BuildContainer)

	m := root.Target("test:unit")
	defer func() { m.Complete(err) }()

	m.Println("Running unit tests")

	err = m.GolangTest().
		SetRace(true).
		SetBuildContainer(buildBoxName()).
		SetMod("vendor").
		Test(context.TODO(), "./...")
	return
}
