package app

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/gravitational/gravity/lib/app/resources"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/systeminfo"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	apps "k8s.io/api/apps/v1beta1"
	batchv1 "k8s.io/api/batch/v1"
	batchv2alpha1 "k8s.io/api/batch/v2alpha1"
	"k8s.io/api/core/v1"
	extensions "k8s.io/api/extensions/v1beta1"
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

// GetUpdatedDependencies compares dependencies of the "installed" and "update" apps and
// returns locators of updated (or new) dependencies.
//
// Only direct dependencies are compared, without base app resolution.
func GetUpdatedDependencies(installed, update Application) ([]loc.Locator, error) {
	if installed.Package.IsEqualTo(update.Package) {
		return nil, trace.NotFound("no update for %v", update)
	}

	installedDeps, err := GetDirectDeps(installed)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	updateDeps, err := GetDirectDeps(update)
	if err != nil {
		return nil, trace.Wrap(err)
	}

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
func GetDirectDeps(app Application) ([]loc.Locator, error) {
	manifest, err := schema.ParseManifestYAMLNoValidate(app.PackageEnvelope.Manifest)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return append(manifest.Dependencies.GetApps(), app.Package), nil
}

// UpdateSecurityContextInDir updates all application resources in the specified directory
// with securityContext using the given service user
func UpdateSecurityContextInDir(dir string, serviceUser systeminfo.User) error {
	if serviceUser.UID == defaults.PlaceholderUserID {
		// No need for transformation
		return nil
	}

	err := filepath.Walk(dir, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return trace.ConvertSystemError(err)
		}
		if fi.IsDir() {
			// Descend into directory
			return nil
		}
		if filepath.Ext(path) == ".yaml" && filepath.Base(path) != defaults.ManifestFileName {
			err = renderResourceTemplate(path, serviceUser)
			if err != nil {
				log.Warnf("Failed to render resources at %v: %v.", path, trace.DebugReport(err))
			}
		}
		return nil
	})
	return trace.Wrap(err)
}

// UpdateSecurityContext updates the security context for the given Pod (including
// security contexts of all containers) using the specified service user.
// Only the security contexts using a special defaults.PlaceholderServiceUserID
// are updated.
func UpdateSecurityContext(pod *v1.PodSpec, serviceUser systeminfo.User) (updated bool) {
	updateContext := func(securityCtx *v1.SecurityContext) (updated bool) {
		if securityCtx.RunAsUser != nil && *securityCtx.RunAsUser == defaults.PlaceholderUserID {
			*securityCtx.RunAsUser = int64(serviceUser.UID)
			return true
		}
		return false
	}
	updatePodContext := func(securityCtx *v1.PodSecurityContext) (updated bool) {
		if securityCtx.RunAsUser != nil && *securityCtx.RunAsUser == defaults.PlaceholderUserID {
			*securityCtx.RunAsUser = int64(serviceUser.UID)
			return true
		}
		return false
	}
	if pod.SecurityContext != nil {
		if updatePodContext(pod.SecurityContext) {
			updated = true
		}
	}
	for _, container := range pod.Containers {
		if container.SecurityContext != nil && updateContext(container.SecurityContext) {
			updated = true
		}
	}
	return updated
}

func renderResourceTemplate(path string, serviceUser systeminfo.User) error {
	in, err := os.Open(path)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	defer in.Close()

	dir := filepath.Dir(path)
	tmp, err := ioutil.TempFile(dir, "render")
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	defer func() {
		tmp.Close()
		// Ignore the error as the file might not be at this path
		// after move
		os.Remove(tmp.Name())
	}()

	res, err := resources.Decode(in)
	if err != nil {
		return trace.Wrap(err)
	}

	var updated bool
	for _, object := range res.Objects {
		var spec *v1.PodSpec
		switch resource := object.(type) {
		case *v1.Pod:
			spec = &resource.Spec
		case *v1.ReplicationController:
			spec = &resource.Spec.Template.Spec
		case *extensions.Deployment:
			spec = &resource.Spec.Template.Spec
		case *extensions.DaemonSet:
			spec = &resource.Spec.Template.Spec
		case *extensions.ReplicaSet:
			spec = &resource.Spec.Template.Spec
		case *apps.StatefulSet:
			spec = &resource.Spec.Template.Spec
		case *batchv1.Job:
			spec = &resource.Spec.Template.Spec
		case *batchv2alpha1.CronJob:
			spec = &resource.Spec.JobTemplate.Spec.Template.Spec
		}
		if spec != nil && UpdateSecurityContext(spec, serviceUser) {
			updated = true
		}
	}

	if !updated {
		log.Debugf("Skip rewriting %v as it has not changed.", path)
		return nil
	}

	log.Debugf("Rewrite %v with %v.", path, serviceUser)
	err = res.Encode(tmp)
	if err != nil {
		return trace.Wrap(err)
	}

	err = os.Rename(tmp.Name(), path)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}
