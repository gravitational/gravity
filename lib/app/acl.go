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
	"io"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/users"

	teledefaults "github.com/gravitational/teleport/lib/defaults"
	teleservices "github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
)

// ApplicationsWithACL returns an instance of the Applications interface
// with the specified security context
func ApplicationsWithACL(applications Applications, users users.Users, user storage.User, checker teleservices.AccessChecker) Applications {
	return &ApplicationsACL{
		applications: applications,
		users:        users,
		user:         user,
		checker:      checker,
	}
}

// ApplicationsACL defines a security aware wrapper around Applications
type ApplicationsACL struct {
	applications Applications
	users        users.Users
	user         storage.User
	checker      teleservices.AccessChecker
}

func (r *ApplicationsACL) repoContext(repoName string) *users.Context {
	return &users.Context{
		Context: teleservices.Context{
			User:     r.user,
			Resource: storage.NewRepository(repoName),
		},
	}
}

func (r *ApplicationsACL) appContext(locator loc.Locator) *users.Context {
	return &users.Context{
		Context: teleservices.Context{
			User:     r.user,
			Resource: storage.NewApp(locator),
		},
	}
}

func (r *ApplicationsACL) CreateImportOperation(req *ImportRequest) (*storage.AppOperation, error) {
	if err := r.check(req.Repository, teleservices.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}
	return r.applications.CreateImportOperation(req)
}

func (r *ApplicationsACL) GetOperationProgress(op storage.AppOperation) (*ProgressEntry, error) {
	if err := r.check(op.Repository, teleservices.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return r.applications.GetOperationProgress(op)
}

func (r *ApplicationsACL) GetOperationLogs(op storage.AppOperation) (io.ReadCloser, error) {
	if err := r.check(op.Repository, teleservices.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return r.applications.GetOperationLogs(op)
}

func (r *ApplicationsACL) GetOperationCrashReport(op storage.AppOperation) (io.ReadCloser, error) {
	if err := r.check(op.Repository, teleservices.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return r.applications.GetOperationCrashReport(op)
}

func (r *ApplicationsACL) GetImportedApplication(op storage.AppOperation) (*Application, error) {
	if err := r.check(op.Repository, teleservices.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return r.applications.GetImportedApplication(op)
}

func (r *ApplicationsACL) GetApp(locator loc.Locator) (*Application, error) {
	if err := r.checkApp(locator, teleservices.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return r.applications.GetApp(locator)
}

func (r *ApplicationsACL) GetAppManifest(locator loc.Locator) (io.ReadCloser, error) {
	if err := r.checkApp(locator, teleservices.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return r.applications.GetAppManifest(locator)
}

func (r *ApplicationsACL) GetAppResources(locator loc.Locator) (io.ReadCloser, error) {
	if err := r.checkApp(locator, teleservices.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return r.applications.GetAppResources(locator)
}

func (r *ApplicationsACL) GetAppInstaller(req InstallerRequest) (io.ReadCloser, error) {
	if err := r.checkApp(req.Application, teleservices.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return r.applications.GetAppInstaller(req)
}

func (r *ApplicationsACL) ListApps(req ListAppsRequest) ([]Application, error) {
	if req.Repository == "" {
		return nil, trace.BadParameter("missing parameter repository")
	}
	if err := r.check(req.Repository, teleservices.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}
	return r.applications.ListApps(req)
}

func (r *ApplicationsACL) ExportApp(req ExportAppRequest) error {
	if err := r.checkApp(req.Package, teleservices.VerbRead); err != nil {
		return trace.Wrap(err)
	}
	return r.applications.ExportApp(req)
}

func (r *ApplicationsACL) UninstallApp(locator loc.Locator) (*Application, error) {
	if err := r.checkApp(locator, teleservices.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return r.applications.UninstallApp(locator)
}

func (r *ApplicationsACL) StatusApp(locator loc.Locator) (*Status, error) {
	if err := r.checkApp(locator, teleservices.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return r.applications.StatusApp(locator)
}

func (r *ApplicationsACL) DeleteApp(req DeleteRequest) error {
	if err := r.check(req.Package.Repository, teleservices.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return r.applications.DeleteApp(req)
}

func (r *ApplicationsACL) CreateApp(locator loc.Locator, reader io.Reader, labels map[string]string) (*Application, error) {
	if err := r.check(locator.Repository, teleservices.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}
	return r.applications.CreateApp(locator, reader, labels)
}

// CreateAppWithManifest creates a new application from the specified package bytes (reader)
// and an optional set of package labels using locator as destination for the
// resulting package, with supplied manifest
func (r *ApplicationsACL) CreateAppWithManifest(locator loc.Locator, manifest []byte, reader io.Reader, labels map[string]string) (*Application, error) {
	if err := r.check(locator.Repository, teleservices.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}
	return r.applications.CreateAppWithManifest(locator, manifest, reader, labels)
}

func (r *ApplicationsACL) UpsertApp(locator loc.Locator, reader io.Reader, labels map[string]string) (*Application, error) {
	if err := r.check(locator.Repository, teleservices.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := r.check(locator.Repository, teleservices.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	return r.applications.UpsertApp(locator, reader, labels)
}

// StartAppHook starts application hook specified with req asynchronously
func (r *ApplicationsACL) StartAppHook(ctx context.Context, req HookRunRequest) (*HookRef, error) {
	if err := r.check(req.Application.Repository, teleservices.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return r.applications.StartAppHook(ctx, req)
}

// WaitAppHook waits for app hook to complete or fail
func (r *ApplicationsACL) WaitAppHook(ctx context.Context, ref HookRef) error {
	if err := r.check(ref.Application.Repository, teleservices.VerbRead); err != nil {
		return trace.Wrap(err)
	}
	return r.applications.WaitAppHook(ctx, ref)
}

// DeleteAppHookJob deletes app hook job to complete or fail
func (r *ApplicationsACL) DeleteAppHookJob(ctx context.Context, req DeleteAppHookJobRequest) error {
	if err := r.check(req.Application.Repository, teleservices.VerbRead); err != nil {
		return trace.Wrap(err)
	}
	return r.applications.DeleteAppHookJob(ctx, req)
}

// StreamAppHookLogs streams app hook logs to output writer, this is a blocking call
func (r *ApplicationsACL) StreamAppHookLogs(ctx context.Context, ref HookRef, out io.Writer) error {
	if err := r.check(ref.Application.Repository, teleservices.VerbRead); err != nil {
		return trace.Wrap(err)
	}
	return r.applications.StreamAppHookLogs(ctx, ref, out)
}

// FetchChart returns Helm chart package with the specified application.
func (r *ApplicationsACL) FetchChart(locator loc.Locator) (io.ReadCloser, error) {
	if err := r.checkApp(locator, teleservices.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return r.applications.FetchChart(locator)
}

// FetchIndexFile returns Helm chart repository index file data.
func (r *ApplicationsACL) FetchIndexFile() (io.Reader, error) {
	if err := r.check(defaults.SystemAccountOrg, teleservices.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return r.applications.FetchIndexFile()
}

// check checks whether the user has the requested permissions to read write apps
func (r *ApplicationsACL) check(repoName, verb string) error {
	return r.checker.CheckAccessToRule(r.repoContext(repoName), teledefaults.Namespace, storage.KindApp, verb, false)
}

// checkApp checks whether the user has the requested permissions to the specified app
func (r *ApplicationsACL) checkApp(locator loc.Locator, verb string) error {
	return r.checker.CheckAccessToRule(r.appContext(locator),
		teledefaults.Namespace, storage.KindApp, verb, false)
}
