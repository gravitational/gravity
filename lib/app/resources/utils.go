package resources

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/systeminfo"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	appsv1beta1 "k8s.io/api/apps/v1beta1"
	appsv1beta2 "k8s.io/api/apps/v1beta2"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	batchv2alpha1 "k8s.io/api/batch/v2alpha1"
	v1 "k8s.io/api/core/v1"
	extensions "k8s.io/api/extensions/v1beta1"
)

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

	res, err := Decode(in)
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
		case *appsv1beta1.Deployment:
			spec = &resource.Spec.Template.Spec
		case *appsv1beta2.Deployment:
			spec = &resource.Spec.Template.Spec
		case *appsv1.Deployment:
			spec = &resource.Spec.Template.Spec
		case *extensions.DaemonSet:
			spec = &resource.Spec.Template.Spec
		case *appsv1.DaemonSet:
			spec = &resource.Spec.Template.Spec
		case *appsv1beta2.DaemonSet:
			spec = &resource.Spec.Template.Spec
		case *extensions.ReplicaSet:
			spec = &resource.Spec.Template.Spec
		case *appsv1.ReplicaSet:
			spec = &resource.Spec.Template.Spec
		case *appsv1beta2.ReplicaSet:
			spec = &resource.Spec.Template.Spec
		case *appsv1.StatefulSet:
			spec = &resource.Spec.Template.Spec
		case *appsv1beta1.StatefulSet:
			spec = &resource.Spec.Template.Spec
		case *appsv1beta2.StatefulSet:
			spec = &resource.Spec.Template.Spec
		case *batchv1.Job:
			spec = &resource.Spec.Template.Spec
		case *batchv2alpha1.CronJob:
			spec = &resource.Spec.JobTemplate.Spec.Template.Spec
		case *batchv1beta1.CronJob:
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
