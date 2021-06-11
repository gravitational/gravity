/*
Copyright 2018-2019 Gravitational, Inc.

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
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/gravitational/gravity/lib/app/hooks"
	"github.com/gravitational/gravity/lib/app/resources"
	"github.com/gravitational/gravity/lib/archive"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/docker"
	"github.com/gravitational/gravity/lib/helm"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/run"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/utils"

	dockerarchive "github.com/docker/docker/pkg/archive"
	"github.com/ghodss/yaml"
	teleutils "github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
)

// VendorerConfig is configuration for vendorer
type VendorerConfig struct {
	// DockerClient is the docker client to use to manage images
	DockerClient docker.DockerInterface
	// ImageService is the docker registry service
	docker.ImageService
	// RegistryURL is the URL of the active docker registry to use
	RegistryURL string
	// Packages is the pack service
	Packages pack.PackageService
}

// NewVendorer creates a new vendorer instance.
//nolint:revive // will be exported in a separate PR
func NewVendorer(config VendorerConfig) (*vendorer, error) {
	dockerPuller := docker.NewDockerPuller(config.DockerClient)
	v := &vendorer{
		dockerClient: config.DockerClient,
		imageService: config.ImageService,
		dockerPuller: dockerPuller,
		registryURL:  config.RegistryURL,
		packages:     config.Packages,
	}
	return v, nil
}

// Vendorer is an interface for interacting with the vendoring helper.
type Vendorer interface {
	// VendorDir takes information from an app vendor request, imports missing docker images if necessary,
	// rewrites image names in the app's resources and returns a path to the directory containing ready
	// to be imported app.
	VendorDir(ctx context.Context, dir string, req VendorRequest) error
	// VendorTarball is the same as VendorDir but accepts a tarball stream and unpacks it before vendoring
	VendorTarball(ctx context.Context, tarball io.ReadCloser, req VendorRequest) (string, error)
}

// VendorRequest combined various vendoring options
type VendorRequest struct {
	// Repository is the name of app repository
	Repository string
	// PackageName is the app name
	PackageName string
	// PackageVersion is the app version
	PackageVersion string
	// ManifestPath is the path to app manifest
	ManifestPath string
	// ResourcePatterns is a list of file path patterns to search for container images
	ResourcePatterns []string
	// IgnoreResourcePatterns is a list of file path patterns to ignore when searching for images
	IgnoreResourcePatterns []string
	// SetImages is a list of images to rewrite to new versions
	SetImages []loc.DockerImage
	// SetDeps is a list of app dependencies to rewrite to new versions
	SetDeps []loc.Locator
	// VendorRuntime specifies whether to translate runtime images into packages.
	// The vendoring of the runtime package is a multi-step process which also requires
	// access to the package store used for building the final application installer
	// tarball.
	// During the vendoring of individual package tarballs, it is not feasible to also translate
	// the runtime docker image into a telekube package - hence this is initially false.
	VendorRuntime bool
	// Parallel defines the number of tasks to run in parallel.
	// If < 0, the number of tasks is unrestricted.
	// If in [0,1], the tasks are executed sequentially.
	Parallel int
	// ProgressReporter is a special writer, if set, vendorer will output user-friendly
	// information during vendoring
	ProgressReporter utils.Progress
	// Helm contains parameters for rendering Helm charts.
	Helm helm.RenderParameters
	// Pull allows to force-pull Docker images even if they're already present.
	Pull bool
}

// vendorer is a helper struct that encapsulates all services needed to vendor/rewrite images in
// the application being imported.
type vendorer struct {
	dockerClient docker.DockerInterface
	imageService docker.ImageService
	dockerPuller docker.DockerPuller
	registryURL  string
	packages     pack.PackageService
}

// VendorTarball is the same as VendorDir but accepts a tarball stream and unpacks it before vendoring.
//
// It is the caller's responsibility to delete the temporary directory containing the vendored app.
func (v *vendorer) VendorTarball(ctx context.Context, tarball io.ReadCloser, req VendorRequest) (string, error) {
	unpackedDir, err := ioutil.TempDir(os.TempDir(), "gravity-import")
	if err != nil {
		return "", trace.Wrap(err)
	}

	if err = dockerarchive.Untar(tarball, unpackedDir, archive.DefaultOptions()); err != nil {
		return "", trace.Wrap(err)
	}
	req.ManifestPath = filepath.Join(unpackedDir, defaults.ResourcesDir, defaults.ManifestFileName)

	return unpackedDir, trace.Wrap(v.VendorDir(ctx, unpackedDir, req))
}

// VendorDir creates an application tarball from the unpackedDir using configuration specified in req.
//
// It will detect and import missing docker images and rewrite image references in all resource files
// to point to a fixed docker registry address.
func (v *vendorer) VendorDir(ctx context.Context, unpackedDir string, req VendorRequest) error {
	if req.ProgressReporter == nil {
		req.ProgressReporter = utils.DiscardProgress
	}
	// before parsing resources apply basic transformations on manifest, e.g. environment
	// variables interpolation
	if req.ManifestPath != "" {
		if err := expandEnvVars(req.ManifestPath); err != nil {
			return trace.Wrap(err)
		}
	}

	// parse all resources
	resourceFiles, chartResources, err := resourcesFromPath(unpackedDir, req)
	if err != nil {
		return trace.Wrap(err)
	}

	// first, rewrite all "multi-source" values that might refer to files and replace them
	// with literal values, since some of them may also contain docker image references
	// and generate overlay network jobs
	err = resourceFiles.RewriteManifest(ctx,
		makeRewriteMultiSourceFunc(req.ManifestPath),
		makeRewriteWormholeJobFunc())
	if err != nil {
		return trace.Wrap(err)
	}

	// next, rewrite images that were specified by `--set-image` since the
	// original image tag might not actually exist.
	err = resourceFiles.RewriteImages(makeRewriteSetImagesFunc(req.SetImages))
	if err != nil {
		return trace.Wrap(err)
	}

	err = analyzeResources(resourceFiles, chartResources, req)
	if err != nil {
		return trace.Wrap(err)
	}

	images, err := resourceFiles.Images()
	if err != nil {
		return trace.Wrap(err)
	}
	log.Infof("Images: %v.", images)

	// vendor chart images as well
	chartImages, err := chartResources.Images()
	if err != nil {
		return trace.Wrap(err)
	}
	log.Infof("Chart images: %v.", chartImages)

	images = append(images, chartImages...)

	// Now that we have all referenced images in our local registry, and can find them without
	// a registry prefix, rewrite our resource files to vendor the images.
	if err = resourceFiles.RewriteImages(v.imageService.Wrap); err != nil {
		return trace.Wrap(err)
	}

	var runtimeImages []string
	manifestRewrites := []resources.ManifestRewriteFunc{
		makeRewriteDepsFunc(req.SetDeps),
		makeRewritePackagesMetadataFunc(v.packages),
		makeRewriteAppMetadataFunc(req.Repository, req.PackageName, req.PackageVersion),
	}
	if req.VendorRuntime {
		manifestRewrites = append(manifestRewrites, fetchRuntimeImages(&runtimeImages))
	}

	err = resourceFiles.RewriteManifest(ctx, manifestRewrites...)
	if err != nil {
		return trace.Wrap(err)
	}

	// pull the default container image along with the rest of images
	imagesToPull := append(images, defaults.ContainerImage)
	imagesToPull = append(imagesToPull, runtimeImages...)

	group, groupCtx := run.WithContext(ctx, run.WithParallel(req.Parallel))
	for _, image := range imagesToPull {
		log := log.WithField("image", image)
		if strings.HasPrefix(image, v.registryURL) {
			// image has already been vendored
			continue
		}
		image := image // create new variable for go routine below
		group.Go(groupCtx, func() error {
			// pull all missing images (this will correctly fail for images without a remote
			// registry that do not exist i.e. due to failed image build)
			if err := pullMissingRemoteImage(image, v.dockerPuller, log, req); err != nil {
				return trace.Wrap(err)
			}

			// tag all images without their registry, so that we can find them later after
			// stripping remote registries
			if err := tagImageWithoutRegistry(image, v.dockerClient, log); err != nil {
				return trace.Wrap(err)
			}
			return nil
		})
	}
	if err := group.Wait(); err != nil {
		return trace.Wrap(err)
	}

	if req.VendorRuntime {
		err = resourceFiles.RewriteManifest(ctx, v.translateRuntimeImages)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	if err = resourceFiles.Write(); err != nil {
		return trace.Wrap(err)
	}

	if ok, _ := utils.IsDirectory(filepath.Join(unpackedDir, defaults.RegistryDir)); ok {
		log.Debug("Registry layers are present.")
		return nil
	}

	// if the application package does not contain the dump of docker images of the referenced
	// containers, pull all the necessary images, then export those images to disk
	images, err = resourceFiles.Images()
	if err != nil {
		return trace.Wrap(err)
	}

	images = append(images, hooks.InitContainerImage)
	for i, image := range images {
		images[i] = v.imageService.Unwrap(image)
	}

	log.Infof("No registry layers found, will pull and export images %q.", images)
	if err = v.pullAndExportImages(ctx, teleutils.Deduplicate(images), unpackedDir, req.Parallel, req.ProgressReporter); err != nil {
		return trace.Wrap(err)
	}

	if err = v.pullAndExportImages(ctx, teleutils.Deduplicate(chartImages), unpackedDir, req.Parallel, req.ProgressReporter); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// analyzeResources looks at the parsed Kubernetes/Helm resource files and
// prints some helpful information about them to the user.
func analyzeResources(resourceFiles, chartFiles resources.ResourceFiles, req VendorRequest) error {
	for _, resourceFile := range append(resourceFiles, chartFiles...) {
		images, err := resourceFile.Images()
		if err != nil {
			return trace.Wrap(err)
		}
		if len(images.UnrecognizedObjects) > 0 {
			req.ProgressReporter.PrintSubWarn("Some resources weren't recognized, run with --verbose (-v) for more details")
			break
		}
	}
	log.Infof("Detected resource files: %v.", resourceFiles)
	for _, resourceFile := range resourceFiles {
		err := printResourceStatus(resourceFile, req)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	log.Infof("Detected Helm templates: %v.", chartFiles)
	for _, resourceFile := range chartFiles {
		err := printResourceStatus(resourceFile, req)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// printResourceStatus prints a user-friendly status message about the provided
// resource file which gives the user a high-level visibility into the process
// of discovering images from resources
//
// The following information is printed:
//  - the fact that this resource file has been detected and is being processed
//  - an info message in case no Docker images could be extracted from the file
//    which might help the user spot mistakes in their resource file / chart
//  - a debug message in case an object definition of an unknown version / kind
//    has been detected
func printResourceStatus(resourceFile resources.ResourceFile, req VendorRequest) error {
	relPath := utils.TrimPathPrefix(resourceFile.Path(), filepath.Dir(req.ManifestPath))
	extractedImages, err := resourceFile.Images()
	if err != nil {
		return trace.Wrap(err)
	}
	if len(extractedImages.Images) == 0 {
		req.ProgressReporter.PrintSubDebug("%v %v:\n\t\tNo images to vendor", resourceFile.Kind(), relPath)
	} else {
		req.ProgressReporter.PrintSubDebug("%v %v:", resourceFile.Kind(), relPath)
		for _, image := range extractedImages.Images {
			req.ProgressReporter.PrintSubDebug(color.GreenString("\t%v", image))
		}
	}
	for _, o := range extractedImages.UnrecognizedObjects {
		gvk := o.GetObjectKind().GroupVersionKind()
		unk, err := resources.ToUnknown(o)
		if err != nil {
			log.WithError(err).WithFields(log.Fields{
				"apiVersion": fmt.Sprintf("%v/%v", gvk.Group, gvk.Version),
				"kind":       gvk.Kind,
			}).Warn("Failed to convert object to unknown resource.")
		} else {
			log.WithFields(log.Fields{
				"apiVersion": fmt.Sprintf("%v/%v", gvk.Group, gvk.Version),
				"kind":       gvk.Kind,
				"name":       unk.Metadata.Name,
			}).Info("Skip unrecognized object.")
			req.ProgressReporter.PrintSubDebug(color.BlueString("\tUnrecognized object: apiVersion=%v; kind=%v; name=%v",
				fmt.Sprintf("%v/%v", gvk.Group, gvk.Version), gvk.Kind, unk.Metadata.Name))
		}
	}
	return nil
}

// pullAndExportImages pulls the docker images of all referenced container images (if not yet
// present locally), pushes them into an instance of a private docker registry and then
// dumps the contents of this private registry into the specified directory
func (v *vendorer) pullAndExportImages(ctx context.Context, images []string, exportDir string, parallel int, progress utils.Progress) error {
	resourcesDir := filepath.Join(exportDir, "resources")
	if err := os.MkdirAll(resourcesDir, defaults.PrivateDirMask); err != nil {
		return trace.Wrap(trace.ConvertSystemError(err),
			"failed to create %q", resourcesDir)
	}

	if len(images) == 0 {
		// nothing to do
		return nil
	}

	layersDir := filepath.Join(exportDir, "registry")
	if err := os.MkdirAll(layersDir, defaults.PrivateDirMask); err != nil {
		return trace.Wrap(trace.ConvertSystemError(err),
			"failed to create %q", layersDir)
	}

	if err := exportLayers(ctx, exportDir, images, v.dockerClient,
		log.WithField("export-directory", exportDir), parallel, progress); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (v *vendorer) translateRuntimeImages(ctx context.Context, m *schema.Manifest) error {
	if m.SystemOptions != nil && m.SystemOptions.BaseImage != "" {
		_, tag, err := parseImageNameTag(m.SystemOptions.BaseImage)
		if err != nil {
			return trace.Wrap(err, "invalid base image format %q",
				m.SystemOptions.BaseImage)
		}
		runtimePackage, err := loc.NewLocator(defaults.SystemAccountOrg,
			constants.PlanetPackage, tag)
		if err != nil {
			return trace.Wrap(err, "invalid package format %v/%v:%v",
				defaults.SystemAccountOrg, constants.PlanetPackage, tag)
		}
		req := docker.TranslateImageRequest{
			Image:           m.SystemOptions.BaseImage,
			Package:         *runtimePackage,
			DockerInterface: v.dockerClient,
			PackageService:  v.packages,
		}
		if err := docker.TranslateRuntimeImage(req); err != nil {
			return trace.Wrap(err)
		}
		if m.SystemOptions.Dependencies.Runtime == nil {
			m.SystemOptions.Dependencies.Runtime = &schema.Dependency{}
		}
		m.SystemOptions.Dependencies.Runtime.Locator = *runtimePackage
	}
	imageToPackage := make(map[string]loc.Locator)
	newPackageName := newRuntimePackage(imageToPackage, nil)
	for i, profile := range m.NodeProfiles {
		if profile.SystemOptions == nil || profile.SystemOptions.BaseImage == "" {
			continue
		}
		var runtimePackage loc.Locator
		var exists bool
		if runtimePackage, exists = imageToPackage[profile.SystemOptions.BaseImage]; !exists {
			newPackage, err := newPackageName(profile.SystemOptions.BaseImage)
			if err != nil {
				return trace.Wrap(err, "invalid package format")
			}
			runtimePackage = *newPackage
			req := docker.TranslateImageRequest{
				Image:           profile.SystemOptions.BaseImage,
				Package:         runtimePackage,
				DockerInterface: v.dockerClient,
				PackageService:  v.packages,
			}
			if err := docker.TranslateRuntimeImage(req); err != nil {
				return trace.Wrap(err)
			}
		}
		if profile.SystemOptions.Dependencies.Runtime == nil {
			m.NodeProfiles[i].SystemOptions.Dependencies.Runtime = &schema.Dependency{}
		}
		m.NodeProfiles[i].SystemOptions.Dependencies.Runtime.Locator = runtimePackage
	}
	return nil
}

// newRuntimePackage returns a generator to generate package names.
// Generated package names are guaranteed to not collide with legacy runtime
// package names and be unique within a single generator.
func newRuntimePackage(imageToPackage map[string]loc.Locator, randomSuffix randomSuffix) packageNameGeneratorFunc {
	generatedNames := make(map[string]struct{})
	nonUnique := func(name string) bool {
		_, exists := generatedNames[name]
		return exists
	}
	if randomSuffix == nil {
		randomSuffix = func(name string) string {
			return fmt.Sprintf("%v-%v", name, utilrand.String(4))
		}
	}
	var legacyPackages = []string{loc.LegacyPlanetMaster.Name, loc.LegacyPlanetNode.Name}
	return func(image string) (runtimePackage *loc.Locator, err error) {
		name, tag, err := parseImageNameTag(image)
		if err != nil || name == "" {
			return nil, trace.Wrap(err, "invalid base image format %q", image)
		}

		if loc, exists := imageToPackage[image]; exists {
			return &loc, nil
		}

		// Update the name if it matches any of the legacy package names to avoid collision
		if utils.StringInSlice(legacyPackages, name) {
			newName := randomSuffix(name)
			for nonUnique(newName) {
				newName = randomSuffix(name)
			}
			name = newName
		}

		runtimePackage, err = loc.NewLocator(defaults.SystemAccountOrg, name, tag)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		imageToPackage[image] = *runtimePackage
		generatedNames[name] = struct{}{}
		return runtimePackage, nil
	}
}

type randomSuffix func(string) string

type packageNameGeneratorFunc func(image string) (runtimePackage *loc.Locator, err error)

// expandEnvVars performs environment variables interpolation on manifest raw source data
func expandEnvVars(path string) error {
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		return trace.Wrap(err)
	}
	err = ioutil.WriteFile(path, schema.ExpandEnvVars(bytes), defaults.SharedReadMask)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

type rewriteFunc func(string) string

// makeRewriteSetImagesFunc prepares a function for rewriting images in resource files
// if it finds a matching image name, it overrides image with the version set
// in the request
func makeRewriteSetImagesFunc(setImages []loc.DockerImage) rewriteFunc {
	return func(image string) string {
		origImage, err := loc.ParseDockerImage(image)
		if err != nil {
			log.Warningf("Failed to rewrite %v: %v.", image, trace.DebugReport(err))
			return image
		}
		for _, newImage := range setImages {
			if origImage.Repository == newImage.Repository {
				log.Infof("Image %v rewritten to %v.", image, newImage.String())
				image = newImage.String()
				break
			}
		}
		return image
	}
}

// makeRewriteAppMetadataFunc returns a function to rewrite application metadata: repository, name or version
func makeRewriteAppMetadataFunc(setRepository, setName, setVersion string) resources.ManifestRewriteFunc {
	return func(ctx context.Context, m *schema.Manifest) error {
		if setRepository != "" {
			m.Metadata.Repository = setRepository
		}
		if setName != "" {
			m.Metadata.Name = setName
		}
		if setVersion != "" {
			m.Metadata.ResourceVersion = setVersion
		}
		m.Metadata.CreatedTimestamp = time.Now().UTC()
		return nil
	}
}

// makeRewriteMultiSourceFunc returns a function that rewrites "multi-source" values in manifest
// with their literal values
func makeRewriteMultiSourceFunc(manifestPath string) resources.ManifestRewriteFunc {
	return func(ctx context.Context, m *schema.Manifest) error {
		return trace.Wrap(schema.ProcessMultiSourceValues(ctx, m, manifestPath))
	}
}

func makeRewriteWormholeJobFunc() resources.ManifestRewriteFunc {
	return func(ctx context.Context, m *schema.Manifest) error {
		if m.Providers != nil && m.Providers.Generic.Networking.Type == constants.WireguardNetworkType {
			if m.Hooks == nil {
				m.Hooks = &schema.Hooks{}
			}

			var err error
			m.Hooks.NetworkInstall, err = generateWormholeHook(schema.HookNetworkInstall)
			if err != nil {
				return trace.Wrap(err)
			}

			m.Hooks.NetworkUpdate, err = generateWormholeHook(schema.HookNetworkUpdate)
			if err != nil {
				return trace.Wrap(err)
			}

			m.Hooks.NetworkRollback, err = generateWormholeHook(schema.HookNetworkRollback)
			if err != nil {
				return trace.Wrap(err)
			}
		}
		return nil
	}
}

// generateWormholeHook generates a gravity hook for installing wormhole encrypted network plugin
func generateWormholeHook(hook schema.HookType) (*schema.Hook, error) {
	script := ""

	switch hook {
	case schema.HookNetworkInstall:
		script = "/gravity/gravity-install.sh"
	case schema.HookNetworkUpdate:
		script = "/gravity/gravity-upgrade.sh"
	case schema.HookNetworkRollback:
		script = "/gravity/gravity-rollback.sh"
	default:
		return nil, trace.BadParameter("unsupported hook: %v", hook)
	}

	job := batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: constants.WireguardNetworkType,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyOnFailure,
					Containers: []corev1.Container{
						{
							Name:    "hook",
							Image:   defaults.WormholeImg,
							Command: []string{script},
						},
					},
				},
			},
		},
	}
	y, err := yaml.Marshal(job)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &schema.Hook{
		Type: hook,
		Job:  string(y),
	}, nil
}

// makeRewriteDepsFunc returns a function to update package dependencies - for each dependency
// matching the repository/name pair from the specified set, the package is updated to the one from the list.
func makeRewriteDepsFunc(setPackages []loc.Locator) resources.ManifestRewriteFunc {
	rewrite := func(l loc.Locator) loc.Locator {
		for _, newPackage := range setPackages {
			if l.Repository == newPackage.Repository && l.Name == newPackage.Name {
				log.Infof("Dependency %v rewritten to %v.", l, newPackage.String())
				return newPackage
			}
		}
		return l
	}
	return func(ctx context.Context, m *schema.Manifest) error {
		for i := range m.Dependencies.Packages {
			m.Dependencies.Packages[i].Locator = rewrite(m.Dependencies.Packages[i].Locator)
		}
		for i := range m.Dependencies.Apps {
			m.Dependencies.Apps[i].Locator = rewrite(m.Dependencies.Apps[i].Locator)
		}
		if m.SystemOptions != nil && m.SystemOptions.Dependencies.Runtime != nil {
			m.SystemOptions.Dependencies.Runtime.Locator =
				rewrite(m.SystemOptions.Dependencies.Runtime.Locator)
		}
		base := m.Base()
		if base != nil {
			m.SetBase(rewrite(*base))
		}
		return nil
	}
}

// makeRewritePackagesMetadataFunc returns a function that processes metadata for the app's dependency
// packages (base, packages, apps) and rewrites versions accordingly
func makeRewritePackagesMetadataFunc(packages pack.PackageService) resources.ManifestRewriteFunc {
	return func(ctx context.Context, m *schema.Manifest) error {
		base := m.Base()
		if base != nil {
			newLoc, err := pack.ProcessMetadata(packages, base)
			if err != nil {
				return trace.Wrap(err)
			}
			log.Infof("Rewritten: %v -> %v.", base, newLoc)
			m.SetBase(*newLoc)
		}
		for i, dep := range m.Dependencies.Packages {
			newLoc, err := pack.ProcessMetadata(packages, &dep.Locator)
			if err != nil {
				return trace.Wrap(err)
			}
			log.Infof("Rewritten: %v -> %v.", dep, newLoc)
			m.Dependencies.Packages[i].Locator = *newLoc
		}
		for i, dep := range m.Dependencies.Apps {
			newLoc, err := pack.ProcessMetadata(packages, &dep.Locator)
			if err != nil {
				return trace.Wrap(err)
			}
			log.Infof("Rewritten: %v -> %v.", dep, newLoc)
			m.Dependencies.Apps[i].Locator = *newLoc
		}
		return nil
	}
}

func fetchRuntimeImages(images *[]string) resources.ManifestRewriteFunc {
	return func(ctx context.Context, m *schema.Manifest) error {
		*images = m.RuntimeImages()
		return nil
	}
}

func isChartDirectory(path string) (bool, error) {
	fi, err := os.Stat(filepath.Join(path, constants.HelmChartFile))
	err = trace.ConvertSystemError(err)
	if err != nil {
		if trace.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return !fi.IsDir(), nil
}

func resourcesFromChart(path string, req VendorRequest) (resources.ResourceFiles, error) {
	rendered, err := helm.Render(helm.RenderParameters{
		Path:   path,
		Values: req.Helm.Values,
		Set:    req.Helm.Set,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var result resources.ResourceFiles
	for k, v := range rendered {
		resource, err := resources.Decode(strings.NewReader(v))
		if err != nil {
			return nil, trace.Wrap(err, "failed to decode: %v", k)
		}
		result = append(result, resources.NewResourceFileObject(
			k, resources.KindHelmTemplate, *resource))
	}
	return result, nil
}

// resourcesFromPath collects resource files in root for further processing.
// It will search for files starting with root and matching a set of file path patterns
// specified with patterns.
// Returns a list of collected resource files upon success.
func resourcesFromPath(root string, req VendorRequest) (result resources.ResourceFiles, chartResources resources.ResourceFiles, err error) {
	err = filepath.Walk(root, func(path string, fileInfo os.FileInfo, err error) error {
		if err != nil {
			return trace.Wrap(err)
		}

		var matched bool
		for _, pattern := range req.ResourcePatterns {
			matched, _ = archive.PathMatch(archive.PathPattern(pattern), path)
			if matched {
				break
			}
		}

		if matched {
			for _, ignorePattern := range req.IgnoreResourcePatterns {
				matchedIgnore, _ := regexp.MatchString(ignorePattern, path)
				if matchedIgnore {
					matched = false
					break
				}
			}
		}

		if fileInfo.IsDir() {
			isChartDir, err := isChartDirectory(path)
			if err != nil {
				return trace.Wrap(err)
			}
			if !isChartDir {
				return nil
			}
			log.Infof("Extracting images from Helm chart directory %v.", path)
			resource, err := resourcesFromChart(path, req)
			if err != nil {
				return trace.Wrap(err, "failed to parse resources in Helm chart: %v",
					utils.TrimPathPrefix(path, root, defaults.ResourcesDir))
			}
			chartResources = append(chartResources, resource...)
			// Chart dir can also contain manifest file.
			if _, err := utils.StatFile(filepath.Join(path, defaults.ManifestFileName)); err == nil {
				resourceFile, err := resources.NewResourceFile(filepath.Join(path, defaults.ManifestFileName))
				if err != nil {
					return trace.Wrap(err)
				}
				result = append(result, *resourceFile)
			}
			return filepath.SkipDir
		}

		if !matched {
			log.Debugf("Skipping not matching file %v.", path)
			return nil
		}
		resourceFile, err := resources.NewResourceFile(path)
		if err != nil {
			return trace.Wrap(err, "failed to parse resource file: %v",
				utils.TrimPathPrefix(path, root, defaults.ResourcesDir))
		}
		result = append(result, *resourceFile)
		return nil
	})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return result, chartResources, nil
}
