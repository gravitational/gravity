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

package service

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/docker"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/cenkalti/backoff"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// SyncRequest describes a request to sync an application with registry
type SyncRequest struct {
	PackService  pack.PackageService
	AppService   app.Applications
	ImageService docker.ImageService
	Package      loc.Locator
	Progress     utils.Printer
	ScanConfig   *docker.ScanConfig
}

// CheckAndSetDefaults validates the request and sets some defaults.
func (r *SyncRequest) CheckAndSetDefaults() error {
	if r.Progress == nil {
		r.Progress = utils.DiscardPrinter
	}
	return nil
}

// SyncApp syncs an application and all its dependencies with registry
func SyncApp(ctx context.Context, req SyncRequest) error {
	err := req.CheckAndSetDefaults()
	if err != nil {
		return trace.Wrap(err)
	}

	application, err := req.AppService.GetApp(req.Package)
	if err != nil {
		return trace.Wrap(err)
	}

	// sync base app
	base := application.Manifest.Base()
	if base != nil {
		err = SyncApp(ctx, SyncRequest{
			PackService:  req.PackService,
			AppService:   req.AppService,
			ImageService: req.ImageService,
			Package:      *base,
			Progress:     req.Progress,
			ScanConfig:   req.ScanConfig,
		})
		if err != nil {
			return trace.Wrap(err)
		}
	}

	// sync dependencies
	for _, dep := range application.Manifest.Dependencies.Apps {
		err = SyncApp(ctx, SyncRequest{
			PackService:  req.PackService,
			AppService:   req.AppService,
			ImageService: req.ImageService,
			Package:      dep.Locator,
			Progress:     req.Progress,
			ScanConfig:   req.ScanConfig,
		})
		if err != nil {
			return trace.Wrap(err)
		}
	}

	// the app will be unpacked at this dir
	dir, err := ioutil.TempDir("", "sync")
	if err != nil {
		return trace.Wrap(err)
	}
	defer func() {
		err := os.RemoveAll(dir)
		if err != nil {
			log.Warningf("failed to remove %v: %v", dir, trace.DebugReport(err))
		}
	}()

	// unpack the app and sync its registry with the local registry
	unpackedPath := pack.PackagePath(dir, req.Package)
	ctx, cancel := context.WithTimeout(context.Background(), defaults.TransientErrorTimeout)
	defer cancel()
	err = unpackRemotePackage(ctx, req.PackService, req.Package, unpackedPath)
	if err != nil {
		return trace.Wrap(err)
	}

	syncPath := filepath.Join(unpackedPath, "registry")

	// check if the registry dir exists at all
	if exists, _ := utils.IsDirectory(syncPath); !exists {
		log.Warningf("registry dir does not exist, skipping sync: %v", syncPath)
		return nil
	}

	// registry dir exists, check if it has any contents
	empty, err := utils.IsDirectoryEmpty(syncPath)
	if err != nil {
		return trace.Wrap(err)
	}
	if empty {
		log.Warningf("registry directory is empty, skipping sync: %v", syncPath)
		return nil
	}

	log.Infof("Syncing %v.", req.Package)

	if _, err = req.ImageService.Sync(ctx, syncPath, req.Progress); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func unpackRemotePackage(ctx context.Context, packages pack.PackageService, pkg loc.Locator, unpackPath string) error {
	b := backoff.NewConstantBackOff(defaults.RetryInterval)
	err := utils.RetryTransient(ctx, b, func() error {
		err := pack.Unpack(packages, pkg, unpackPath, nil)
		if err == nil {
			return nil
		}
		log.Warnf("Failed to unpack package %v: %v.", pkg, err)
		return trace.Wrap(err)
	})
	return trace.Wrap(err)
}
