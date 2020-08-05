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

package app

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/app/docker"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/cenkalti/backoff"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// SyncApp syncs the specified application and all its dependencies with registry
func (r Syncer) SyncApp(ctx context.Context, loc loc.Locator) error {
	err := r.checkAndSetDefaults()
	if err != nil {
		return trace.Wrap(err)
	}
	deps, err := GetAppDependencies(loc, r.AppService, r.PackService)
	if err != nil {
		return trace.Wrap(err)
	}
	return r.Sync(ctx, *deps)
}

// Sync syncs the specified dependencies with the configured registry
func (r Syncer) Sync(ctx context.Context, deps Dependencies) error {
	err := r.checkAndSetDefaults()
	if err != nil {
		return trace.Wrap(err)
	}
	return r.sync(ctx, deps)
}

// GetAppDependencies collects dependencies for the specified application
// (including the application package itself)
func GetAppDependencies(loc loc.Locator, apps Applications, packages pack.PackageService) (deps *Dependencies, err error) {
	app, err := apps.GetApp(loc)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	deps, err = GetDependencies(GetDependenciesRequest{
		App:  *app,
		Apps: apps,
		Pack: packages,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	deps.Apps = append(deps.Apps, *app)
	return deps, nil
}

func (r Syncer) sync(ctx context.Context, deps Dependencies) error {
	for _, app := range deps.Apps {
		if err := r.syncApp(ctx, app.Package); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// Syncer syncs an application with a registry
type Syncer struct {
	// FieldLogger specifies the logger
	log.FieldLogger
	// PackService specifies the package service
	PackService pack.PackageService
	// AppService specifies the application service
	AppService Applications
	// ImageService specifies the docker registry service
	ImageService docker.ImageService
	// Progress specifies the optional progress message printer
	Progress utils.Printer
}

func (r Syncer) syncApp(ctx context.Context, loc loc.Locator) error {
	dir, err := ioutil.TempDir("", "sync")
	if err != nil {
		return trace.Wrap(err)
	}
	defer func() {
		err := os.RemoveAll(dir)
		if err != nil {
			log.WithFields(log.Fields{
				log.ErrorKey: err,
				"dir":        dir,
			}).Warn("Failed to remove directory.")
		}
	}()

	// unpack the app and sync its registry with the local registry
	unpackedPath := pack.PackagePath(dir, loc)
	ctx, cancel := context.WithTimeout(ctx, defaults.TransientErrorTimeout)
	defer cancel()
	err = r.unpackRemotePackage(ctx, loc, unpackedPath)
	if err != nil {
		return trace.Wrap(err)
	}

	syncPath := filepath.Join(unpackedPath, "registry")
	logger := r.WithFields(log.Fields{
		"package": loc,
		"dir":     syncPath,
	})

	// skip sync if no registry directory exists
	if exists, _ := utils.IsDirectory(syncPath); !exists {
		logger.Warn("Registry directory does not exist, skipping sync.")
		return nil
	}

	empty, err := utils.IsDirectoryEmpty(syncPath)
	if err != nil {
		return trace.Wrap(err)
	}
	if empty {
		logger.Warn("Registry directory is empty, skipping sync.")
		return nil
	}

	logger.Info("Sync.")

	_, err = r.ImageService.Sync(ctx, syncPath, r.Progress)
	return trace.Wrap(err)
}

// checkAndSetDefaults validates the request and sets some defaults.
func (r *Syncer) checkAndSetDefaults() error {
	if r.Progress == nil {
		r.Progress = utils.DiscardPrinter
	}
	if r.FieldLogger == nil {
		r.FieldLogger = log.WithField(trace.Component, "sync")
	}
	return nil
}

func (r Syncer) unpackRemotePackage(ctx context.Context, loc loc.Locator, unpackPath string) error {
	b := backoff.NewConstantBackOff(defaults.RetryInterval)
	err := utils.RetryTransient(ctx, b, func() error {
		err := pack.Unpack(r.PackService, loc, unpackPath, nil)
		if err == nil {
			return nil
		}
		return trace.Wrap(err)
	})
	if err != nil {
		r.WithFields(log.Fields{
			"package":    loc,
			log.ErrorKey: err,
		}).Warn("Failed to unpack package.")
	}
	return trace.Wrap(err)
}
