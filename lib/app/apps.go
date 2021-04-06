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
	"fmt"
	"io"

	"github.com/gravitational/gravity/lib/app/hooks"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
)

// RunAppHook launches the specified hook, waits for its completion
// and returns its output and job reference
func RunAppHook(ctx context.Context, apps Applications, req HookRunRequest) (*HookRef, []byte, error) {
	buf := utils.NewSyncBuffer()
	ref, err := StreamAppHook(ctx, apps, req, buf)
	return ref, buf.Bytes(), trace.Wrap(err)
}

// StreamAppHook launches the specified hook and starts streaming its
// output into the provided writer until the job completes
func StreamAppHook(ctx context.Context, apps Applications, req HookRunRequest, wc io.WriteCloser) (*HookRef, error) {
	ref, err := apps.StartAppHook(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	localCtx, localCancel := context.WithCancel(ctx)
	defer localCancel()

	go func() {
		defer localCancel()
		err := apps.StreamAppHookLogs(ctx, *ref, wc)
		wc.Close()
		if err != nil && !trace.IsEOF(err) {
			log.Warnf("Failed to stream logs for hook %v: %v",
				ref, trace.DebugReport(err))
		}
	}()

	err = utils.Retry(defaults.RetryInterval, defaults.RetryLessAttempts, func() error {
		err := apps.WaitAppHook(ctx, *ref)
		if err != nil {
			if trace.IsConnectionProblem(err) {
				return utils.Continue(fmt.Sprintf(
					"resuming wait on connection error for hook %v", ref))
			}
			return utils.Abort(err)
		}
		return nil
	})
	if err != nil {
		log.Warnf("Hook %v failed: %v.", ref, trace.DebugReport(err))
	}

	// wait for the logs to finish streaming before returning
	select {
	case <-localCtx.Done():
	case <-ctx.Done():
	}
	return ref, trace.Wrap(err)
}

// CheckHasAppHook checks if the app has specified hook
func CheckHasAppHook(apps Applications, req HookRunRequest) (*schema.Hook, error) {
	app, err := apps.GetApp(req.Application)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if app.Manifest.Hooks == nil {
		return nil, trace.NotFound("%v:%v does not have hooks",
			req.Application.Name, req.Application.Version)
	}

	hook, err := schema.HookFromString(req.Hook, app.Manifest)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return hook, nil
}

// WaitAppHook waits for app hook to complete or fail
func WaitAppHook(ctx context.Context, client *kubernetes.Clientset, ref HookRef) error {
	runner, err := hooks.NewRunner(client)
	if err != nil {
		return trace.Wrap(err)
	}
	return runner.Wait(ctx, hooks.JobRef{Name: ref.Name, Namespace: ref.Namespace})
}

// StreamAppHookLogs streams app hook logs to output writer, this is a blocking call
func StreamAppHookLogs(ctx context.Context, client *kubernetes.Clientset, ref HookRef, out io.Writer) error {
	runner, err := hooks.NewRunner(client)
	if err != nil {
		return trace.Wrap(err)
	}
	return runner.StreamLogs(ctx, hooks.JobRef{Name: ref.Name, Namespace: ref.Namespace}, out)
}

// DeleteAppHookJob deletes app hook job
func DeleteAppHookJob(ctx context.Context, client *kubernetes.Clientset, req DeleteAppHookJobRequest) error {
	runner, err := hooks.NewRunner(client)
	if err != nil {
		return trace.Wrap(err)
	}
	return runner.DeleteJob(ctx, hooks.DeleteJobRequest{
		JobRef:  hooks.JobRef{Name: req.Name, Namespace: req.Namespace},
		Cascade: req.Cascade,
	})
}

// GetUpdatedDependencies compares dependencies of the "installed" and "update"
// apps and returns locators of updated (or new) dependencies.
//
// Only direct dependencies are compared, without base app resolution.
func GetUpdatedDependencies(installed, update Application, installedManifest, updateManifest schema.Manifest) ([]loc.Locator, error) {
	installedDeps, err := GetDirectDeps(installed)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	installedDeps = installedManifest.FilterDependencies(installedDeps)

	updateDeps, err := GetDirectDeps(update)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	updateDeps = updateManifest.FilterDependencies(updateDeps)

	var updates []loc.Locator
	for _, update := range updateDeps {
		isUpdate, err := loc.IsUpdate(update, installedDeps)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if !isUpdate {
			continue
		}
		updates = append(updates, update)
	}

	return updates, nil
}

// GetDirectDeps returns the direct application dependencies, without
// base app resolution
func GetDirectDeps(app Application) (deps []loc.Locator, err error) {
	manifest, err := schema.ParseManifestYAMLNoValidate(app.PackageEnvelope.Manifest)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return append(manifest.Dependencies.GetApps(), app.Package), nil
}
