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
	"path/filepath"
	"time"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/trace"

	"github.com/docker/docker/pkg/archive"
)

// importApp imports an application from the specified directory
func (r *Applications) importApp(op storage.AppOperation, request *app.ImportRequest, unpackedDir string) (err error) {
	var ctx *operationContext
	ctx, err = newOperationContext(&importOperation{op: &op, progressc: request.ProgressC}, r.config.StateDir, r.config.Backend)
	if err != nil {
		return trace.Wrap(err)
	}
	defer ctx.Close()

	if err = r.importAppWithContext(ctx, request, unpackedDir); err != nil {
		err = trace.Wrap(err)
		_ = ctx.update(app.ProgressFailure(err.Error()))
		return err
	}
	return trace.Wrap(ctx.update(app.ProgressStateCompleted))
}

func (r *Applications) importAppWithContext(ctx *operationContext, request *app.ImportRequest, unpackedDir string) error {
	manifestBytes, err := manifestFromDir(unpackedDir)
	if err != nil {
		return trace.Wrap(err)
	}

	locator, err := loc.NewLocator(request.Repository, request.PackageName, request.PackageVersion)
	if err != nil {
		return trace.Wrap(err)
	}

	archiveOptions := &archive.TarOptions{
		Compression:     archive.Gzip,
		ExcludePatterns: request.ExcludePatterns,
		IncludeFiles:    request.IncludePaths,
	}

	packageBytes, err := archive.TarWithOptions(unpackedDir, archiveOptions)
	if err != nil {
		return trace.Wrap(err)
	}
	defer packageBytes.Close()

	if err = ctx.update(app.ImportStateCreatingPackage); err != nil {
		return trace.Wrap(err)
	}

	// delete precomputed resources package
	if err := r.deleteResourcesPackage(*locator); err != nil {
		return trace.Wrap(err)
	}

	ctx.Infof("creating application package")

	var labels map[string]string
	_, err = r.createApp(*locator, packageBytes, manifestBytes, labels, request.Email, request.Force)
	return trace.Wrap(err)
}

// importOperation implements operation interface
type importOperation struct {
	op        *storage.AppOperation
	progressc chan *app.ProgressEntry
}

// update advances this import operation to the step given with operationStep.
// It will update the state of the import operation in the backend as well as
// create the corresponding import progress entry
func (r *importOperation) update(backend storage.Backend, step operationStep) (err error) {
	r.op.State = step.State()
	r.op.Updated = time.Now().UTC()

	op, err := backend.UpdateAppOperation(*r.op)
	if err != nil {
		return trace.Wrap(err)
	}
	progress, err := backend.CreateAppProgressEntry(storage.AppProgressEntry{
		Repository:     r.op.Repository,
		PackageName:    r.op.PackageName,
		PackageVersion: r.op.PackageVersion,
		OperationID:    r.op.ID,
		Completion:     step.Completion(),
		Created:        time.Now().UTC(),
		State:          r.op.State,
		Message:        step.Message(),
	})
	if err != nil {
		return trace.Wrap(err)
	}

	if r.progressc != nil {
		r.progressc <- (*app.ProgressEntry)(progress)
	}
	r.op = op
	return nil
}

// logPath obtains the path to the log file used by this import operation
func (r importOperation) logPath() string {
	return filepath.Join(r.op.ID, "operation.log")
}

// applicationType determines the type of application based on specified manifest
func applicationType(manifest *schema.Manifest) (storage.AppType, error) {
	switch manifest.Kind {
	case schema.KindSystemApplication:
		return storage.AppService, nil
	case schema.KindRuntime:
		return storage.AppRuntime, nil
	case schema.KindBundle, schema.KindCluster, schema.KindApplication:
		return storage.AppUser, nil
	}
	return "", trace.BadParameter("unknown application type: %v", manifest.Kind)
}
