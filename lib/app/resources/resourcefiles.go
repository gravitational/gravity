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

package resources

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/schema"

	clusterv1beta1 "github.com/gravitational/gravity/lib/apis/cluster/v1beta1"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	admissionv1beta1 "k8s.io/api/admissionregistration/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	appsv1beta1 "k8s.io/api/apps/v1beta1"
	appsv1beta2 "k8s.io/api/apps/v1beta2"
	batchv1 "k8s.io/api/batch/v1"
	batchv1beta1 "k8s.io/api/batch/v1beta1"
	batchv2alpha1 "k8s.io/api/batch/v2alpha1"
	certificatesv1beta1 "k8s.io/api/certificates/v1beta1"
	corev1 "k8s.io/api/core/v1"
	extensions "k8s.io/api/extensions/v1beta1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1beta1 "k8s.io/api/policy/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	rbacv1alpha1 "k8s.io/api/rbac/v1alpha1"
	rbacv1beta1 "k8s.io/api/rbac/v1beta1"
	settingsv1alpha1 "k8s.io/api/settings/v1alpha1"
	storagev1 "k8s.io/api/storage/v1"
	storagev1beta1 "k8s.io/api/storage/v1beta1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	apiregistrationv1beta1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1beta1"
)

// Resources is a list of resource objects
type Resources []Resource

// Images returns images found in resources
func (resources Resources) Images() ([]string, error) {
	var objects []runtime.Object
	for _, r := range resources {
		objects = append(objects, r.Objects...)
	}
	extractedImages, err := extractImages(objects)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return extractedImages.Images, nil
}

// resourceFiles is a collection of resource files
type ResourceFiles []ResourceFile

// resourceFile represents a file with resources to process
type ResourceFile struct {
	Resource
	path string
	kind string
}

const (
	// KindResourceFile represents a generic Kubernetes resource spec file.
	KindResourceFile = "Resource file"
	// KindManifestFile represents a cluster/app image manifest file.
	KindManifestFile = "Manifest file"
	// KindHelmTemplate represents a Helm template file.
	KindHelmTemplate = "Helm template"
)

// Path returns path to resource
func (r ResourceFile) Path() string {
	return r.path
}

// Kind returns the kind of the resource this file represents.
func (r ResourceFile) Kind() string {
	if r.kind != "" {
		return r.kind
	}
	return KindResourceFile
}

// Images returns a list of Docker images in this resource file
func (r ResourceFile) Images() (*ExtractedImages, error) {
	return extractImages(r.Objects)
}

// String implements fmt.Stringer for this resource file
func (r ResourceFile) String() string {
	return fmt.Sprintf("ResourceFile(%v)", r.path)
}

// NewResourceFileObject returns new resource file created from resource
func NewResourceFileObject(path, kind string, resource Resource) ResourceFile {
	return ResourceFile{path: path, kind: kind, Resource: resource}
}

// NewResourceFile parses the file at path and returns
// all kubernetes objects as a single ResourceFile instance.
// The object can then serialize itself to the original location.
func NewResourceFile(path string) (*ResourceFile, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer file.Close()

	// decode the contents of the resource file into an object
	resource, err := Decode(file)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	kind := KindResourceFile
	if filepath.Base(path) == defaults.ManifestFileName {
		kind = KindManifestFile
	}

	return &ResourceFile{
		Resource: *resource,
		path:     path,
		kind:     kind,
	}, nil
}

// Images accumulates container image references over the specified range of
// resource files and returns them as a flat list w/o duplicates.
func (r ResourceFiles) Images() ([]string, error) {
	var objects []runtime.Object
	for _, file := range r {
		objects = append(objects, file.Objects...)
	}
	extractedImages, err := extractImages(objects)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return extractedImages.Images, nil
}

// ManifestRewriteFunc defines an interface for functions that can update parts of an application manifest
type ManifestRewriteFunc func(m *schema.Manifest) error

// RewriteManifest rewrites parts of the application manifest resources by application
// of the specified set of rewrite functions
func (r *ResourceFiles) RewriteManifest(rewrites ...ManifestRewriteFunc) error {
	for _, file := range *r {
		for _, object := range file.Objects {
			switch manifest := object.(type) {
			case *schema.Manifest:
				for _, rewrite := range rewrites {
					if err := rewrite(manifest); err != nil {
						return trace.Wrap(err)
					}
				}
			}
		}
	}
	return nil
}

// skipRewrite skips rewrite of the provisioning hooks,
// these ones will be pulled from the internet
func skipRewrite(hook *schema.Hook, job *batchv1.Job) bool {
	switch hook.Type {
	case schema.HookClusterProvision, schema.HookClusterDeprovision, schema.HookNodesProvision, schema.HookNodesDeprovision:
		return true
	}
	return false
}

// RewriteImages rewrites container image references in all resource files part
// of this collection using the specified imageService.
func (r *ResourceFiles) RewriteImages(rewriteFunc func(string) string) error {
	rewrite := func(spec *corev1.PodSpec) {
		for i := range spec.Containers {
			container := &spec.Containers[i]
			spec.Containers[i].Image = rewriteFunc(container.Image)
		}
		for i := range spec.InitContainers {
			container := &spec.InitContainers[i]
			spec.InitContainers[i].Image = rewriteFunc(container.Image)
		}
	}

	// rewriteInHook takes a hook, parses its job specification (if present),
	// rewrites images in it according to the provided rewrite function and
	// updates the job spec in the hook back
	rewriteInHook := func(hook *schema.Hook) error {
		job, err := hook.GetJob()
		if err != nil && !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		if job == nil {
			return nil
		}
		if skipRewrite(hook, job) {
			return nil
		}
		rewrite(&job.Spec.Template.Spec)
		if err = hook.SetJob(*job); err != nil {
			return trace.Wrap(err)
		}
		return nil
	}

	for _, file := range *r {
		for _, object := range file.Objects {
			switch resource := object.(type) {
			case *schema.Manifest:
				log.Infof("Rewriting images in app manifest %q.", resource.Metadata.Name)
				if resource.Hooks != nil {
					for _, hook := range resource.Hooks.AllHooks() {
						if err := rewriteInHook(hook); err != nil {
							return trace.Wrap(err)
						}
					}
				}
			case *clusterv1beta1.ImageSet:
				log.Infof("Rewriting images in ImageSet %q.", resource.Name)
				for i := range resource.Spec.Images {
					resource.Spec.Images[i].Image = rewriteFunc(
						resource.Spec.Images[i].Image)
				}
			case *corev1.Pod:
				log.Infof("Rewriting images in Pod %q.", resource.Name)
				rewrite(&resource.Spec)
			case *corev1.ReplicationController:
				log.Infof("Rewriting images in ReplicationController %q.", resource.Name)
				if resource.Spec.Template != nil {
					rewrite(&resource.Spec.Template.Spec)
				}
			case *extensions.Deployment:
				log.Infof("Rewriting images in Deployment %q.", resource.Name)
				rewrite(&resource.Spec.Template.Spec)
			case *appsv1beta1.Deployment:
				log.Infof("Rewriting images in Deployment %q.", resource.Name)
				rewrite(&resource.Spec.Template.Spec)
			case *appsv1beta2.Deployment:
				log.Infof("Rewriting images in Deployment %q.", resource.Name)
				rewrite(&resource.Spec.Template.Spec)
			case *appsv1.Deployment:
				log.Infof("Rewriting images in Deployment %q.", resource.Name)
				rewrite(&resource.Spec.Template.Spec)
			case *extensions.DaemonSet:
				log.Infof("Rewriting images in DaemonSet %q.", resource.Name)
				rewrite(&resource.Spec.Template.Spec)
			case *appsv1beta2.DaemonSet:
				log.Infof("Rewriting images in DaemonSet %q.", resource.Name)
				rewrite(&resource.Spec.Template.Spec)
			case *appsv1.DaemonSet:
				log.Infof("Rewriting images in DaemonSet %q.", resource.Name)
				rewrite(&resource.Spec.Template.Spec)
			case *extensions.ReplicaSet:
				log.Infof("Rewriting images in ReplicaSet %q.", resource.Name)
				rewrite(&resource.Spec.Template.Spec)
			case *appsv1beta2.ReplicaSet:
				log.Infof("Rewriting images in ReplicaSet %q.", resource.Name)
				rewrite(&resource.Spec.Template.Spec)
			case *appsv1.ReplicaSet:
				log.Infof("Rewriting images in ReplicaSet %q.", resource.Name)
				rewrite(&resource.Spec.Template.Spec)
			case *appsv1beta1.StatefulSet:
				log.Infof("Rewriting images in StatefulSet %q.", resource.Name)
				rewrite(&resource.Spec.Template.Spec)
			case *appsv1beta2.StatefulSet:
				log.Infof("Rewriting images in StatefulSet %q.", resource.Name)
				rewrite(&resource.Spec.Template.Spec)
			case *appsv1.StatefulSet:
				log.Infof("Rewriting images in StatefulSet %q.", resource.Name)
				rewrite(&resource.Spec.Template.Spec)
			case *batchv1.Job:
				log.Infof("Rewriting images in Job %q.", resource.Name)
				rewrite(&resource.Spec.Template.Spec)
			case *batchv2alpha1.CronJob:
				log.Infof("Rewriting images in CronJob %q.", resource.Name)
				rewrite(&resource.Spec.JobTemplate.Spec.Template.Spec)
			case *batchv1beta1.CronJob:
				log.Infof("Rewriting images in CronJob %q.", resource.Name)
				rewrite(&resource.Spec.JobTemplate.Spec.Template.Spec)
			}
		}
	}
	return nil
}

// Write serializes the resource files to disk back to the locations they were read from.
func (r ResourceFiles) Write() (err error) {
	for _, res := range r {
		file, err := os.Create(res.path)
		if err != nil {
			return trace.Wrap(err)
		}
		defer file.Close()
		if err = res.Encode(file); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// ExtractedImages is the results of image extraction from a list of objects
type ExtractedImages struct {
	// Images contains extracted image names
	Images []string
	// UnrecognizedObjects contains objects which could not be recognized
	UnrecognizedObjects []runtime.Object
}

// extractImages collects container images from supported kubernetes resource types
func extractImages(objects []runtime.Object) (*ExtractedImages, error) {
	imagesMap := make(map[string]struct{})
	var unrecognizedObjects []runtime.Object
	for _, obj := range objects {
		var containers []corev1.Container
		switch resource := obj.(type) {
		case *schema.Manifest:
			if resource.Hooks != nil {
				for _, hook := range resource.Hooks.AllHooks() {
					job, err := hook.GetJob()
					if err != nil && !trace.IsNotFound(err) {
						return nil, trace.Wrap(err)
					}
					if job == nil {
						continue
					}
					if skipRewrite(hook, job) {
						continue
					}
					containers = append(containers, job.Spec.Template.Spec.Containers...)
					containers = append(containers, job.Spec.Template.Spec.InitContainers...)
				}
			}
		case *clusterv1beta1.ImageSet:
			for _, spec := range resource.Spec.Images {
				imagesMap[spec.Image] = struct{}{}
			}
		case *corev1.Pod:
			containers = append(resource.Spec.Containers, resource.Spec.InitContainers...)
		case *corev1.ReplicationController:
			if resource.Spec.Template != nil {
				containers = append(resource.Spec.Template.Spec.Containers,
					resource.Spec.Template.Spec.InitContainers...)
			}
		case *batchv1.Job:
			containers = append(resource.Spec.Template.Spec.Containers,
				resource.Spec.Template.Spec.InitContainers...)
		case *extensions.Deployment:
			containers = append(resource.Spec.Template.Spec.Containers,
				resource.Spec.Template.Spec.InitContainers...)
		case *appsv1beta1.Deployment:
			containers = append(resource.Spec.Template.Spec.Containers,
				resource.Spec.Template.Spec.InitContainers...)
		case *appsv1beta2.Deployment:
			containers = append(resource.Spec.Template.Spec.Containers,
				resource.Spec.Template.Spec.InitContainers...)
		case *appsv1.Deployment:
			containers = append(resource.Spec.Template.Spec.Containers,
				resource.Spec.Template.Spec.InitContainers...)
		case *extensions.DaemonSet:
			containers = append(resource.Spec.Template.Spec.Containers,
				resource.Spec.Template.Spec.InitContainers...)
		case *appsv1beta2.DaemonSet:
			containers = append(resource.Spec.Template.Spec.Containers,
				resource.Spec.Template.Spec.InitContainers...)
		case *appsv1.DaemonSet:
			containers = append(resource.Spec.Template.Spec.Containers,
				resource.Spec.Template.Spec.InitContainers...)
		case *extensions.ReplicaSet:
			containers = append(resource.Spec.Template.Spec.Containers,
				resource.Spec.Template.Spec.InitContainers...)
		case *appsv1beta2.ReplicaSet:
			containers = append(resource.Spec.Template.Spec.Containers,
				resource.Spec.Template.Spec.InitContainers...)
		case *appsv1.ReplicaSet:
			containers = append(resource.Spec.Template.Spec.Containers,
				resource.Spec.Template.Spec.InitContainers...)
		case *appsv1beta1.StatefulSet:
			containers = append(resource.Spec.Template.Spec.Containers,
				resource.Spec.Template.Spec.InitContainers...)
		case *appsv1beta2.StatefulSet:
			containers = append(resource.Spec.Template.Spec.Containers,
				resource.Spec.Template.Spec.InitContainers...)
		case *appsv1.StatefulSet:
			containers = append(resource.Spec.Template.Spec.Containers,
				resource.Spec.Template.Spec.InitContainers...)
		case *batchv2alpha1.CronJob:
			containers = append(resource.Spec.JobTemplate.Spec.Template.Spec.Containers,
				resource.Spec.JobTemplate.Spec.Template.Spec.InitContainers...)
		case *batchv1beta1.CronJob:
			containers = append(resource.Spec.JobTemplate.Spec.Template.Spec.Containers,
				resource.Spec.JobTemplate.Spec.Template.Spec.InitContainers...)
		default:
			if !isKnownNonPodObject(obj) {
				unrecognizedObjects = append(unrecognizedObjects, obj)
			}
		}
		for _, container := range containers {
			imagesMap[container.Image] = struct{}{}
		}
	}
	images := make([]string, 0, len(imagesMap))
	for image := range imagesMap {
		images = append(images, image)
	}
	return &ExtractedImages{
		Images:              images,
		UnrecognizedObjects: unrecognizedObjects,
	}, nil
}

// isKnownObject returns true if the provided object is one of the recognized
// Kubernetes object types that are not pod-based, i.e. do not contain Docker
// image in their spec
//
// The list of recognized types is roughly based on the list of resources
// supported by kubectl:
//
// https://kubernetes.io/docs/reference/kubectl/overview/#resource-types
func isKnownNonPodObject(object runtime.Object) bool {
	switch object.(type) {
	case *apiregistrationv1.APIService,
		*apiregistrationv1beta1.APIService,
		*certificatesv1beta1.CertificateSigningRequest,
		*rbacv1.ClusterRole,
		*rbacv1.ClusterRoleBinding,
		*rbacv1.Role,
		*rbacv1.RoleBinding,
		*rbacv1alpha1.ClusterRole,
		*rbacv1alpha1.ClusterRoleBinding,
		*rbacv1alpha1.Role,
		*rbacv1alpha1.RoleBinding,
		*rbacv1beta1.ClusterRole,
		*rbacv1beta1.ClusterRoleBinding,
		*rbacv1beta1.Role,
		*rbacv1beta1.RoleBinding,
		*corev1.ConfigMap,
		*corev1.Endpoints,
		*extensions.Ingress,
		*corev1.LimitRange,
		*corev1.Namespace,
		*extensions.NetworkPolicy,
		*networkingv1.NetworkPolicy,
		*corev1.PersistentVolume,
		*corev1.PersistentVolumeClaim,
		*policyv1beta1.PodDisruptionBudget,
		*policyv1beta1.PodSecurityPolicy,
		*settingsv1alpha1.PodPreset,
		*extensions.PodSecurityPolicy,
		*corev1.ResourceQuota,
		*corev1.Secret,
		*corev1.ServiceAccount,
		*corev1.Service,
		*storagev1.StorageClass,
		*storagev1beta1.StorageClass,
		*v1beta1.CustomResourceDefinition,
		*admissionv1beta1.ValidatingWebhookConfiguration,
		*admissionv1beta1.MutatingWebhookConfiguration:
		return true
	}
	return false
}

// ResourceFunc is the function that operates on an arbitrary Kubernetes object
type ResourceFunc func(object runtime.Object) error

// ForEachObjectInFile decodes the file at specified path as Kubernetes spec
// and calls the provided function for each of the decoded objects
func ForEachObjectInFile(path string, fn ResourceFunc) error {
	file, err := os.Open(path)
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	defer file.Close()
	err = ForEachObject(file, fn)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// ForEachObject decodes Kubernetes objects from the provided reader and
// calls the provided function for each of them
func ForEachObject(reader io.Reader, fn ResourceFunc) error {
	resourceFile, err := Decode(reader)
	if err != nil {
		return trace.Wrap(err)
	}
	for _, object := range resourceFile.Objects {
		err := fn(object)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}
