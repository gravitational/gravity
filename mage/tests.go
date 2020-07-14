package mage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

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

	m := root.Clone("test:lint")
	defer func() { m.Complete(false, err) }()

	m.Printlnf("Running golangci-lint")

	wd, _ := os.Getwd()
	localCacheDir := filepath.Join(wd, "build/cache")

	err = m.DockerRun().
		SetRemove(true).
		SetUID(fmt.Sprint(os.Getuid())).
		SetGID(fmt.Sprint(os.Getgid())).
		AddVolume(fmt.Sprint(wd, ":/gopath/src/github.com/gravitational/gravity:ro,cached")).
		AddVolume(fmt.Sprint(localCacheDir, ":/gopath/src/github.com/gravitational/gravity/build/cache:cached")).
		SetEnv("XDG_CACHE_HOME", "/gopath/src/github.com/gravitational/gravity/build/cache").
		SetEnv("GOCACHE", "/gopath/src/github.com/gravitational/gravity/build/cache/go").
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

	m := root.Clone("test:unit")
	defer func() { m.Complete(false, err) }()

	m.Println("Running unit tests")

	err = m.GolangTest().
		SetRace(true).
		SetBuildContainer(buildBoxName()).
		SetMod("vendor").
		Test(context.TODO(), "./...")
	return
}
