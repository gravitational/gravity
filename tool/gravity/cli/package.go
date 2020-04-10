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

package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/gravitational/gravity/lib/app/service"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/httplib"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/utils"
	"github.com/gravitational/gravity/tool/common"
	"github.com/opencontainers/selinux/go-selinux"

	"github.com/docker/docker/pkg/archive"
	"github.com/gravitational/configure"
	"github.com/gravitational/trace"
)

func importPackage(env *localenv.LocalEnvironment, path string, loc loc.Locator, checkManifest bool, opsCenterURL string,
	labels map[string]string) error {
	var file io.ReadCloser

	fileInfo, err := os.Stat(path)
	if err != nil {
		return trace.Wrap(err)
	}
	if fileInfo.IsDir() {
		file, err = pack.Tar(path, checkManifest)
		if err != nil {
			return trace.Wrap(err)
		}
	} else {
		file, err = os.Open(path)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	defer file.Close()

	packages, err := env.PackageService(opsCenterURL)
	if err != nil {
		return trace.Wrap(err)
	}

	err = packages.UpsertRepository(loc.Repository, time.Time{})
	if err != nil {
		return trace.Wrap(err)
	}

	var opts []pack.PackageOption
	if len(labels) != 0 {
		opts = append(opts, pack.WithLabels(labels))
	}

	envelope, err := packages.CreatePackage(loc, file, opts...)
	if err != nil {
		return trace.Wrap(err)
	}
	env.Printf("%v imported: %v\n", loc, envelope)
	return nil
}

func unpackPackage(env *localenv.LocalEnvironment, loc loc.Locator, dir, opsCenterURL string, tarOptions *archive.TarOptions) error {
	packageService, err := env.PackageService(opsCenterURL, httplib.WithDialTimeout(dialTimeout))
	if err != nil {
		return trace.Wrap(err)
	}

	if dir == "" {
		unpackedDir := filepath.Join(env.StateDir, defaults.PackagesDir, defaults.UnpackedDir)
		dir = pack.PackagePath(unpackedDir, loc)
	}

	isDir, err := utils.IsDirectory(dir)
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	} else {
		if !isDir {
			return trace.BadParameter("%v is not directory", dir)
		}
	}

	locPtr, err := pack.ProcessMetadata(packageService, &loc)
	if err != nil {
		return trace.Wrap(err)
	}
	loc = *locPtr

	err = pack.Unpack(packageService, loc, dir, tarOptions)
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("%v unpacked at %v\n", loc, dir)
	return nil
}

func exportPackage(env *localenv.LocalEnvironment, loc loc.Locator, opsCenterURL, targetPath string, mode os.FileMode, label string) error {
	packageService, err := env.PackageService(opsCenterURL)
	if err != nil {
		return trace.Wrap(err)
	}

	locPtr, err := pack.ProcessMetadata(packageService, &loc)
	if err != nil {
		return trace.Wrap(err)
	}
	loc = *locPtr

	ctx, cancel := context.WithTimeout(context.Background(), defaults.TransientErrorTimeout)
	defer cancel()
	err = utils.CopyWithRetries(ctx, targetPath, func() (io.ReadCloser, error) {
		_, rc, err := packageService.ReadPackage(loc)
		return rc, trace.Wrap(err)
	}, utils.PermOption(mode))
	if err != nil {
		return trace.Wrap(err)
	}

	if selinux.GetEnabled() && label != "" {
		if err := selinux.SetFileLabel(targetPath, label); err != nil {
			return trace.Wrap(err)
		}
	}

	env.Printf("%v exported to file %v\n", loc, targetPath)
	return nil
}

func listPackages(app *localenv.LocalEnvironment, repositoryFilter string, opsCenterURL string) error {
	var repository string
	return foreachPackage(app, repositoryFilter, opsCenterURL, func(env pack.PackageEnvelope) error {
		if repository != env.Locator.Repository {
			repository = env.Locator.Repository
			common.PrintHeader(repository)
			app.Println("")
		}
		if len(env.RuntimeLabels) != 0 {
			kv := configure.KeyVal(env.RuntimeLabels)
			app.Printf("* %v %v\n", env, kv.String())
		} else {
			app.Printf("* %v\n", env)
		}
		return nil
	})
}

func foreachPackage(app *localenv.LocalEnvironment, repositoryFilter string, opsCenterURL string, fn func(env pack.PackageEnvelope) error) error {
	packageService, err := app.PackageService(opsCenterURL)
	if err != nil {
		return trace.Wrap(err)
	}

	if err = foreachRepository(repositoryFilter, packageService, func(repository string) error {
		envelopes, err := packageService.GetPackages(repository)
		if err != nil {
			return trace.Wrap(err)
		}
		for _, env := range envelopes {
			err = fn(env)
			if err != nil {
				return trace.Wrap(err)
			}
		}
		return nil
	}); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func deletePackage(app *localenv.LocalEnvironment, loc loc.Locator, force bool, opsCenterURL string) error {
	packageService, err := app.PackageService(opsCenterURL)
	if err != nil {
		return trace.Wrap(err)
	}

	locPtr, err := pack.ProcessMetadata(packageService, &loc)
	if err != nil {
		return trace.Wrap(err)
	}
	loc = *locPtr

	if err := packageService.DeletePackage(loc); err != nil {
		if force && trace.IsNotFound(err) {
			return nil
		}
		return trace.Wrap(err)
	}

	fmt.Printf("%v deleted\n", loc)
	return nil
}

func configurePackage(s *localenv.LocalEnvironment, loc loc.Locator, confLoc loc.Locator, args []string) error {
	log.Infof("configure %v into %v", loc, confLoc)

	if len(args) == 0 {
		fmt.Println(
			"Configuring package using default configuration. " +
				"Provide some args after 'args' separator to configure it with some variables.",
		)
	}

	if err := s.Packages.ConfigurePackage(loc, confLoc, args); err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf(
		"configuration parameters for %v captured in %v\n", loc, confLoc)

	return nil
}

func updatePackageLabels(s *localenv.LocalEnvironment, loc loc.Locator, opsCenterURL string,
	addLabels map[string]string, removeLabels []string) error {
	packages, err := s.PackageService(opsCenterURL)
	if err != nil {
		return trace.Wrap(err)
	}
	err = packages.UpdatePackageLabels(loc, addLabels, removeLabels)
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("%v labels updated\n", loc)
	return nil
}

func executePackageCommand(s *localenv.LocalEnvironment, cmd string, loc loc.Locator, confLoc *loc.Locator, execArgs []string) error {
	log.Infof("exec with config %v %v", loc, confLoc)

	// in case if user supplies "+installed" we provide a special treatment,
	// using currently installed version of the package and configuration
	ver, err := loc.SemVer()
	if err != nil {
		return trace.Wrap(err)
	}
	log.Infof("metadata: %v", ver.Metadata)
	if ver.Metadata == pack.InstalledLabel {
		ploc, pconfLoc, err := pack.FindInstalledPackageWithConfig(s.Packages, loc)
		if err != nil {
			return trace.Wrap(err)
		}
		loc = *ploc
		if confLoc == nil {
			confLoc = pconfLoc
		}
	}

	manifest, err := s.Packages.GetPackageManifest(loc)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := s.Packages.Unpack(loc, ""); err != nil {
		return trace.Wrap(err)
	}

	command, err := manifest.Command(cmd)
	if err != nil {
		return err
	}

	env := []string{fmt.Sprintf("PATH=%v", os.Getenv("PATH"))}
	// read package with configuration if it's provided
	if confLoc.Name != "" {
		_, reader, err := s.Packages.ReadPackage(*confLoc)
		if err != nil {
			return trace.Wrap(err)
		}
		defer reader.Close()

		vars, err := pack.ReadConfigPackage(reader)
		if err != nil {
			return trace.Wrap(err)
		}
		for k, v := range vars {
			env = append(env, fmt.Sprintf("%v=%v", k, v))
		}
	}

	args := append(command.Args, execArgs...)
	log.Infof("calling: %v with env %v", args, env)
	path, err := s.Packages.UnpackedPath(loc)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := os.Chdir(path); err != nil {
		return trace.Wrap(err)
	}
	return syscall.Exec(command.Args[0], args, env)
}

func pushPackage(app *localenv.LocalEnvironment, loc loc.Locator, opsCenterURL string) error {
	dstPackages, err := app.PackageService(opsCenterURL)
	if err != nil {
		return trace.Wrap(err)
	}

	req := service.PackagePullRequest{
		SrcPack:  app.Packages,
		DstPack:  dstPackages,
		Package:  loc,
		Progress: app.Reporter,
	}

	if _, err = service.PullPackage(req); err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("%v pushed to %v\n", loc, opsCenterURL)
	return nil
}

func pullPackage(app *localenv.LocalEnvironment, loc loc.Locator, opsCenterURL string, labels map[string]string, force bool) error {
	log.Infof("start download: %v from %v", loc, opsCenterURL)

	sourcePackages, err := app.PackageService(opsCenterURL)
	if err != nil {
		return trace.Wrap(err)
	}

	req := service.PackagePullRequest{
		SrcPack:  sourcePackages,
		DstPack:  app.Packages,
		Package:  loc,
		Labels:   labels,
		Progress: app.Reporter,
		Upsert:   force,
	}

	if _, err = service.PullPackage(req); err != nil {
		return trace.Wrap(err)
	}

	fmt.Printf("%v pulled from %v\n", loc, opsCenterURL)
	return nil
}

func foreachRepository(repository string, packageService pack.PackageService, fn func(repository string) error) (err error) {
	var repositories []string
	if repository != "" {
		repositories = []string{repository}
	} else {
		repositories, err = packageService.GetRepositories()
		if err != nil {
			return trace.Wrap(err)
		}
	}
	for _, repository := range repositories {
		if err := fn(repository); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// dialTimeout is used in calls to some APIs
const dialTimeout = 10 * time.Second
