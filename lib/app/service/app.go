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
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	appservice "github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/app/hooks"
	"github.com/gravitational/gravity/lib/archive"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/docker"
	"github.com/gravitational/gravity/lib/helm"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/users"
	"github.com/gravitational/gravity/lib/utils"

	dockerarchive "github.com/docker/docker/pkg/archive"
	"github.com/gravitational/rigging"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Config defines the set of configuration attributes for an application interface
type Config struct {
	// Backend defines the backend used for persistency
	Backend storage.Backend
	// Packages defines the package service to use to query / create application
	// package
	Packages pack.PackageService
	// DockerClient defines the interface to the docker.
	// The client is used to manage container images obtained from the application
	// manifest.  It is used to check image presence, pull images from a remote
	// registry and start temporary registry container.
	DockerClient docker.DockerInterface
	// ImageService defines the interface to the private docker registry running inside
	// the cluster.
	// It is used to sync local container images during installation
	// and to push images to another instance during application export.
	ImageService docker.ImageService
	// StateDir defines the directory used to keep intermediate state.
	// This is the location for log files, for instance.
	StateDir string
	// Devmode sets/removes some insecure flags acceptable for development
	Devmode bool
	// Users provides access to user management interface
	Users users.Users
	// CacheResources sets precomputed cache for resources
	CacheResources bool
	// UnpackedDir is the dir where packages are unpacked
	UnpackedDir string
	// ExcludeDeps defines a list of dependencies that will be excluded for the app image
	ExcludeDeps []loc.Locator
	// GetClient constructs kubernetes clients.
	// Either this or Client must be set to use the kubernetes API.
	GetClient func() (*kubernetes.Clientset, error)
	// Client is an optional kubernetes client
	Client *kubernetes.Clientset
	// FieldLogger specifies the optional logger
	log.FieldLogger
	// Charts provides chart repository methods.
	Charts helm.Repository
}

// New creates a new instance of the application manager
func New(conf Config) (*applications, error) {
	if err := conf.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	apps := &applications{
		Config: conf,
	}

	return apps, nil
}

func (r *Config) checkAndSetDefaults() error {
	if r.Backend == nil {
		return trace.BadParameter("Backend is required")
	}
	if r.Packages == nil {
		return trace.BadParameter("Package service is required")
	}
	if r.FieldLogger == nil {
		r.FieldLogger = log.WithFields(log.Fields{
			trace.Component: "apps",
			"devmode":       r.Devmode,
			"unpackedDir":   r.UnpackedDir,
			"stateDir":      r.StateDir,
		})
	}
	return nil
}

// DeleteApp deletes an application record and the underlying package
func (r *applications) DeleteApp(req appservice.DeleteRequest) error {
	if err := r.canDelete(req.Package); err != nil {
		if !req.Force {
			return trace.Wrap(err)
		}
		r.Warnf("Force deleting app %v: %v.", req.Package, err)
	}
	if r.Charts != nil {
		if err := r.Charts.RemoveFromIndex(req.Package); err != nil {
			return trace.Wrap(err)
		}
	}
	if err := r.deleteResourcesPackage(req.Package); err != nil {
		return trace.Wrap(err)
	}
	if err := r.Packages.DeletePackage(req.Package); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// UninstallApp uninstalls the specified application from the runtime, with all its dependencies
func (r *applications) UninstallApp(locator loc.Locator) (*appservice.Application, error) {
	app, err := r.withApp(locator, func(dir string, app *appservice.Application) error {
		// first uninstall the app
		err := r.uninstallApp(locator)
		if err != nil {
			return trace.Wrap(err)
		}
		// then the app's dependencies
		for _, dependency := range app.Manifest.Dependencies.Apps {
			err = r.uninstallApp(dependency.Locator)
			if err != nil {
				return trace.Wrap(err)
			}
		}
		return nil
	})
	return app, trace.Wrap(err)
}

// ExportApp exports containers of the specified application and its dependencies into the specified
// docker registry
func (r *applications) ExportApp(req appservice.ExportAppRequest) error {
	imageService, err := docker.NewImageService(docker.RegistryConnectionRequest{
		RegistryAddress: req.RegistryAddress,
		CertName:        req.CertName,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	ctx := context.TODO()
	_, err = r.withApp(req.Package, func(dir string, app *appservice.Application) error {
		for _, dependency := range app.Manifest.Dependencies.Apps {
			_, err := r.withApp(dependency.Locator, func(dir string, app *appservice.Application) error {
				return r.exportApp(ctx, dir, imageService)
			})
			if err != nil {
				return trace.Wrap(err)
			}
		}
		err := r.exportApp(ctx, dir, imageService)
		return trace.Wrap(err)
	})
	return trace.Wrap(err)
}

// processAppFn defines an interface to handle an application object using dir as a directory
// with all application assets statically available.
// After processing, dir is automatically removed.
type processAppFn func(dir string, app *appservice.Application) error

func (r *applications) withApp(locator loc.Locator, process processAppFn) (*appservice.Application, error) {
	envelope, err := r.Packages.ReadPackageEnvelope(locator)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	app, err := newApp(envelope, envelope.Manifest, locator, r)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	packagePath := pack.PackagePath(r.UnpackedDir, locator)

	err = pack.UnpackIfNotUnpacked(r.Packages, locator, packagePath, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = process(packagePath, app)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return app, nil
}

func syncWithRegistry(ctx context.Context, registryDir string, imageService docker.ImageService, log log.FieldLogger) error {
	if ok, _ := utils.IsDirectory(registryDir); !ok {
		log.Infof("No registry directory is present - skipping registry sync.")
		return nil
	}
	empty, err := utils.IsDirectoryEmpty(registryDir)
	if err != nil {
		return trace.Wrap(err)
	}
	if empty {
		return trace.BadParameter("registry directory %v is empty", registryDir)
	}
	if _, err = imageService.Sync(ctx, registryDir, utils.DiscardPrinter); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// runAppHook executes the hook specified by the request if the app has it
func (r *applications) runAppHook(ctx context.Context, req appservice.HookRunRequest) error {
	// before launching the hook check if the app has it at all
	hook, err := appservice.CheckHasAppHook(r, req)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	if hook == nil {
		r.Debugf(err.Error())
		return nil
	}
	_, out, err := appservice.RunAppHook(ctx, r, req)
	if err != nil {
		return trace.Wrap(err, "failed to run hook %v: %s", req, out)
	}
	return nil
}

func (r *applications) exportApp(ctx context.Context, dir string, imageService docker.ImageService) error {
	dir = filepath.Join(dir, defaults.RegistryDir)
	return syncWithRegistry(ctx, dir, imageService, r.FieldLogger)
}

// uninstallApp calls "pre-uninstall" and "uninstall" hooks for the specified app
func (r *applications) uninstallApp(locator loc.Locator) error {
	r.Infof("Uninstalling %v.", locator)
	err := r.runAppHook(context.TODO(), appservice.HookRunRequest{
		Application: locator,
		Hook:        schema.HookUninstalling,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	err = r.runAppHook(context.TODO(), appservice.HookRunRequest{
		Application: locator,
		Hook:        schema.HookUninstall,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// StartAppHook starts app hook in async mode
func (r *applications) StartAppHook(ctx context.Context, req appservice.HookRunRequest) (*appservice.HookRef, error) {
	hook, err := appservice.CheckHasAppHook(r, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	client, err := r.getKubeClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = r.injectEnvVars(&req, client)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	runner, err := hooks.NewRunner(client)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	creds, err := storage.GetClusterLoginEntry(r.Backend)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	params := hooks.Params{
		Hook:               hook,
		Locator:            req.Application,
		Env:                req.Env,
		Volumes:            req.Volumes,
		Mounts:             req.VolumeMounts,
		NodeSelector:       req.NodeSelector,
		SkipInitContainers: req.SkipInitContainers,
		HostNetwork:        req.HostNetwork,
		JobDeadline:        req.Timeout,
		AgentUser:          creds.Email,
		AgentPassword:      creds.Password,
		GravityPackage:     req.GravityPackage,
		ServiceUser:        req.ServiceUser,
		Values:             req.Values,
	}

	ref, err := runner.Start(ctx, params)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &appservice.HookRef{
		Name:        ref.Name,
		Namespace:   ref.Namespace,
		Application: req.Application,
		Hook:        req.Hook,
	}, nil
}

// injectEnvVars updates the provided hook run request with additional
// environment variables such as cluster information.
func (r *applications) injectEnvVars(req *appservice.HookRunRequest, client *kubernetes.Clientset) error {
	configMap, err := client.CoreV1().ConfigMaps(metav1.NamespaceSystem).
		Get(context.TODO(), constants.ClusterInfoMap, metav1.GetOptions{})
	if err != nil {
		return rigging.ConvertError(err)
	}
	if req.Env == nil {
		req.Env = make(map[string]string)
	}
	req.Env[constants.DevmodeEnvVar] = strconv.FormatBool(r.Devmode)
	for name, value := range configMap.Data {
		req.Env[name] = value
	}
	return nil
}

// WaitAppHook waits for app hook to complete or fail
func (r *applications) WaitAppHook(ctx context.Context, ref appservice.HookRef) error {
	client, err := r.getKubeClient()
	if err != nil {
		return trace.Wrap(err)
	}
	return appservice.WaitAppHook(ctx, client, ref)
}

// StreamAppHookLogs streams app hook logs to output writer, this is a blocking call
func (r *applications) StreamAppHookLogs(ctx context.Context, ref appservice.HookRef, out io.Writer) error {
	client, err := r.getKubeClient()
	if err != nil {
		return trace.Wrap(err)
	}
	return appservice.StreamAppHookLogs(ctx, client, ref, out)
}

// DeleteAppHookJob deletes app hook job
func (r *applications) DeleteAppHookJob(ctx context.Context, req appservice.DeleteAppHookJobRequest) error {
	client, err := r.getKubeClient()
	if err != nil {
		return trace.Wrap(err)
	}
	return appservice.DeleteAppHookJob(ctx, client, req)
}

// StatusApp retrieves the status of a running application
func (r *applications) StatusApp(locator loc.Locator) (*appservice.Status, error) {
	err := r.runAppHook(context.TODO(), appservice.HookRunRequest{
		Application: locator,
		Hook:        schema.HookStatus,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// TODO: decode application status
	return &appservice.Status{}, nil
}

// ListApps lists currently installed applications from the specified repository of the given type
func (r *applications) ListApps(req appservice.ListAppsRequest) (apps []appservice.Application, err error) {
	if err := req.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	batch, err := r.Packages.GetPackages(req.Repository)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, item := range batch {
		if item.Manifest == nil { // app packages have non-nil manifests
			continue
		}
		if item.Hidden && req.ExcludeHidden {
			continue
		}
		if string(req.Type) != "" && item.Type != string(req.Type) {
			continue
		}
		if req.Pattern != "" && !strings.Contains(item.Locator.Name, req.Pattern) {
			continue
		}
		app, err := toApp(&item, r)
		if err != nil {
			// just skip the app if we failed to resolve its manifest to prevent OpsCenter from
			// breaking when deploying backward-incompatible manifest changes
			r.Errorf("Failed to resolve manifest for %v: %v.",
				item.Locator.String(), trace.DebugReport(err))
			continue
		}
		apps = append(apps, *app)
	}
	return apps, nil
}

func resourcesPackageFor(locator loc.Locator) loc.Locator {
	locator.Name = fmt.Sprintf("%v-resources", locator.Name)
	return locator
}

func (r *applications) setResourcesPackage(locator loc.Locator, reader io.Reader) error {
	if !r.CacheResources {
		return trace.BadParameter("cache is off")
	}
	_, err := r.Packages.UpsertPackage(resourcesPackageFor(locator), reader)
	return trace.Wrap(err)
}

func (r *applications) getResourcesPackage(locator loc.Locator) (io.ReadCloser, error) {
	if !r.CacheResources {
		return nil, trace.NotFound("cache is off")
	}
	_, reader, err := r.Packages.ReadPackage(resourcesPackageFor(locator))
	if err != nil {
		r.Debugf("Cache miss %v.", locator)
	} else {
		r.Debugf("Cache hit %v.", locator)
	}
	return reader, err
}

func (r *applications) deleteResourcesPackage(locator loc.Locator) error {
	if !r.CacheResources {
		return nil
	}
	err := r.Packages.DeletePackage(resourcesPackageFor(locator))
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}
	r.Debugf("Cache invalidated for %v.", locator)
	return nil
}

// GetAppResources retrieves an application resources specified with locator
func (r *applications) GetAppResources(locator loc.Locator) (io.ReadCloser, error) {
	if !r.CacheResources {
		return r.getAppResources(locator)
	}
	resourcesPackage, err := r.getResourcesPackage(locator)
	if err == nil {
		return resourcesPackage, nil
	}
	if !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	reader, err := r.getAppResources(locator)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer reader.Close()
	if err := r.setResourcesPackage(locator, reader); err != nil {
		return nil, trace.Wrap(err)
	}
	return r.getResourcesPackage(locator)
}

func (r *applications) getAppResources(locator loc.Locator) (io.ReadCloser, error) {
	locator, err := r.processMetadata(locator)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	_, reader, err := r.Packages.ReadPackage(locator)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	stream, err := unpackedResources(reader)
	if err != nil {
		reader.Close()
		return nil, trace.Wrap(err)
	}
	return &utils.CleanupReadCloser{
		ReadCloser: stream,
		Cleanup: func() {
			reader.Close()
		},
	}, nil
}

// GetApp retrieves an application specified with locator
func (r *applications) GetApp(locator loc.Locator) (*appservice.Application, error) {
	if locator.IsEqualTo(appservice.Phony.Package) {
		return appservice.Phony, nil
	}

	locator, err := r.processMetadata(locator)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	envelope, err := r.Packages.ReadPackageEnvelope(locator)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return toApp(envelope, r)
}

func (r *applications) UpsertApp(locator loc.Locator, reader io.Reader, labels map[string]string) (*appservice.Application, error) {
	manifest, tempDir, cleanup, err := manifestFromUnpackedSource(reader)
	defer cleanup()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	packageBytes, err := dockerarchive.Tar(tempDir, dockerarchive.Gzip)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer packageBytes.Close()

	return r.createApp(locator, packageBytes, manifest, labels, "", true)
}

// CreateAppWithManifest new application from the specified package bytes (reader)
// and an optional set of package labels using locator as destination for the
// resulting package, with supplied manifest
func (r *applications) CreateAppWithManifest(locator loc.Locator, manifest []byte, reader io.Reader, labels map[string]string) (*appservice.Application, error) {
	return r.createApp(locator, reader, manifest, labels, "", false)
}

// CreateApp creates a new application from the specified package bytes (reader)
// and an optional set of package labels using locator as destination for the
// resulting package
func (r *applications) CreateApp(locator loc.Locator, reader io.Reader, labels map[string]string) (*appservice.Application, error) {
	envelope, err := r.Packages.ReadPackageEnvelope(locator)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	if envelope != nil {
		return nil, trace.AlreadyExists("package %v already exists", locator)
	}

	manifest, tempDir, cleanup, err := manifestFromUnpackedSource(reader)
	defer cleanup()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	packageBytes, err := dockerarchive.Tar(tempDir, dockerarchive.Gzip)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer packageBytes.Close()

	return r.CreateAppWithManifest(locator, manifest, packageBytes, labels)
}

func (r *applications) createApp(locator loc.Locator, packageBytes io.Reader, manifestBytes []byte, labels map[string]string, email string, upsert bool) (*appservice.Application, error) {
	manifest, err := r.resolveManifest(manifestBytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = r.Packages.UpsertRepository(locator.Repository, time.Time{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	appType, err := applicationType(manifest)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	options := []pack.PackageOption{
		pack.WithLabels(labels),
		pack.WithManifest(string(appType), manifestBytes),
		pack.WithCreatedBy(email),
		pack.WithHidden(manifest.Metadata.Hidden),
	}

	var envelope *pack.PackageEnvelope
	if upsert {
		envelope, err = r.Packages.UpsertPackage(locator, packageBytes, options...)
	} else {
		envelope, err = r.Packages.CreatePackage(locator, packageBytes, options...)
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if manifest.Kind == schema.KindApplication && r.Charts != nil {
		err = r.Charts.AddToIndex(locator, upsert)
		if err != nil && !trace.IsAlreadyExists(err) {
			return nil, trace.Wrap(err)
		}
	}

	return &appservice.Application{
		Package:         locator,
		PackageEnvelope: *envelope,
		Manifest:        *manifest,
	}, nil
}

// CreateImportOperation initiates import for an application specified with req.
// Returns the import operation to keep track of the import progress.
func (r *applications) CreateImportOperation(req *appservice.ImportRequest) (*storage.AppOperation, error) {
	unpackedDir, cleanup, err := unpackedSource(req.Source)
	if err != nil {
		cleanup()
		return nil, trace.Wrap(err)
	}

	manifestBytes, err := manifestFromDir(unpackedDir)
	if err != nil {
		cleanup()
		return nil, trace.Wrap(err)
	}

	m, err := schema.ParseManifestYAMLNoValidate(manifestBytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	r.Config.ExcludeDeps = appservice.AppsToExclude(*m)

	manifest, err := r.resolveManifest(manifestBytes)
	if err != nil {
		cleanup()
		return nil, trace.Wrap(err)
	}

	// ensure the import request is sound and valid
	if err = r.checkImportRequirements(manifest, req); err != nil {
		cleanup()
		return nil, trace.Wrap(err)
	}

	op := &storage.AppOperation{
		Repository:     req.Repository,
		PackageName:    req.PackageName,
		PackageVersion: req.PackageVersion,
		Type:           appservice.AppOperationImport,
		Created:        time.Now().UTC(),
	}

	err = (func(b storage.Backend) error {
		var err error
		op, err = b.CreateAppOperation(*op)
		if err != nil {
			return trace.Wrap(err)
		}
		_, err = b.CreateAppProgressEntry(storage.AppProgressEntry{
			Repository:     req.Repository,
			PackageName:    req.PackageName,
			PackageVersion: req.PackageVersion,
			OperationID:    op.ID,
			Created:        time.Now().UTC(),
			Completion:     0,
			State:          appservice.ProgressStateInProgress.State(),
		})
		return trace.Wrap(err)
	})(r.Backend)

	if err != nil {
		cleanup()
		return nil, trace.Wrap(err)
	}

	go func() {
		err := r.importApp(*op, req, unpackedDir)
		cleanup()
		req.Source.Close()
		if err != nil && req.ErrorC != nil {
			req.ErrorC <- err
		}
		if req.ErrorC != nil {
			close(req.ErrorC)
		}
		if req.ProgressC != nil {
			close(req.ProgressC)
		}
	}()

	return op, nil
}

// GetAppManifest returns a reader to the application manifest
func (r *applications) GetAppManifest(locator loc.Locator) (io.ReadCloser, error) {
	locator, err := r.processMetadata(locator)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	envelope, err := r.Packages.ReadPackageEnvelope(locator)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return ioutil.NopCloser(bytes.NewReader(envelope.Manifest)), nil
}

// GetOperationProgress returns the last progress record for the specified operation
func (r *applications) GetOperationProgress(op storage.AppOperation) (*appservice.ProgressEntry, error) {
	progress, err := r.Backend.GetLastAppProgressEntry(op.ID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return (*appservice.ProgressEntry)(progress), nil
}

// GetImportedApplication returns the imported application identified by the specified import operation
func (r *applications) GetImportedApplication(op storage.AppOperation) (*appservice.Application, error) {
	if op.ID == "" {
		return nil, trace.BadParameter("missing parameter OperationID")
	}
	operation, err := r.Backend.GetAppOperation(op.ID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	locator, err := loc.NewLocator(operation.Repository, operation.PackageName, operation.PackageVersion)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	pkg, err := r.Packages.ReadPackageEnvelope(*locator)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	app, err := toApp(pkg, r)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return app, nil
}

// GetOperationLogs returns the reader to the logs of the specified operation
func (r *applications) GetOperationLogs(op storage.AppOperation) (io.ReadCloser, error) {
	ctx, err := newOperationContext(&importOperation{op: &op}, r.StateDir, r.Backend)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return ctx.operationLogTailReader()
}

// GetOperationCrashReport returns crash report of the specified operation
func (r *applications) GetOperationCrashReport(op storage.AppOperation) (io.ReadCloser, error) {
	// TODO: importOperation -> operationContext
	ctx, err := newOperationContext(&importOperation{op: &op}, r.StateDir, r.Backend)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	logs, logFileSize, err := ctx.operationLogReader()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	item := archive.ItemFromStream("operation.log", logs, logFileSize, defaults.SharedReadMask)

	reader, writer := io.Pipe()
	go func() {
		tarball := archive.NewTarAppender(writer)
		defer tarball.Close()
		if errClose := writer.CloseWithError(tarball.Add(item)); errClose != nil {
			r.Warnf("Failed to close writer: %v.", errClose)
		}
	}()

	return reader, nil
}

// FetchChart returns Helm chart package with the specified application.
func (r *applications) FetchChart(locator loc.Locator) (io.ReadCloser, error) {
	return r.Charts.FetchChart(locator)
}

// FetchIndexFile returns Helm chart repository index file data.
func (r *applications) FetchIndexFile() (io.Reader, error) {
	return r.Charts.GetIndexFile()
}

func (r *applications) resolveManifest(manifestBytes []byte) (*schema.Manifest, error) {
	manifest, err := schema.ParseManifestYAMLNoValidate(manifestBytes)
	if err != nil {
		r.Warnf("Failed to parse: %s.\n", manifestBytes)
		return nil, trace.Wrap(err, "failed to parse application manifest")
	}
	baseLocator := manifest.Base()
	if baseLocator != nil {
		baseApp, err := r.GetApp(*baseLocator)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		err = mergeManifests(manifest, baseApp.Manifest)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	message := "Dependency %v excluded from manifest"
	manifest.Dependencies.Apps = appservice.Wrap(loc.Filter(appservice.Unwrap(manifest.Dependencies.Apps), r.Config.ExcludeDeps, message))

	if err = schema.CheckAndSetDefaults(manifest); err != nil {
		return nil, trace.Wrap(err)
	}
	PostProcessManifest(manifest)
	return manifest, nil
}

// canDelete makes sure that it is safe to delete an app:
//  - the app exists
//  - no other app depends on it
//  - it is not deployed on any site
func (r *applications) canDelete(locator loc.Locator) error {
	// ensure the app exists
	_, err := r.GetApp(locator)
	if err != nil {
		return trace.Wrap(err)
	}
	// ensure no other app depends on it
	apps, err := r.ListApps(appservice.ListAppsRequest{
		Repository: locator.Repository,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	for _, app := range apps {
		base := app.Manifest.Base()
		if base != nil && base.IsEqualTo(locator) {
			return trace.BadParameter("%v is a base app for %v", locator, app)
		}
		for _, dep := range app.Manifest.Dependencies.Apps {
			if dep.Locator.IsEqualTo(locator) {
				return trace.BadParameter("%v depends on %v", app, locator)
			}
		}
	}
	// ensure it is not deployed on any site
	accounts, err := r.Backend.GetAccounts()
	if err != nil {
		return trace.Wrap(err)
	}
	for _, account := range accounts {
		sites, err := r.Backend.GetSites(account.ID)
		if err != nil {
			return trace.Wrap(err)
		}
		for _, site := range sites {
			p := loc.Locator{
				Repository: site.App.Repository,
				Name:       site.App.Name,
				Version:    site.App.Version,
			}
			if p.IsEqualTo(locator) {
				return trace.BadParameter("%v is used on site %v", locator, site.Domain)
			}
		}
	}
	return nil
}

// checkImportRequirements verifies that the provided app import request is valid.
//
// In particular it checks that:
//
//  - The app's repository, name and version are either present in the app manifest or provided in
//    the import request. The provided request is updated with the proper values.
//
//  - The app's dependencies can be satisfied.
func (r *applications) checkImportRequirements(manifest *schema.Manifest, req *appservice.ImportRequest) error {
	// check whether we need to take repository, package and version from the manifest
	if req.Repository == "" {
		if manifest.Metadata.Repository == "" {
			return trace.BadParameter("repository name must be either present in the app manifest or provided on the command line")
		}
		req.Repository = manifest.Metadata.Repository
	}

	if req.PackageName == "" {
		if manifest.Metadata.Name == "" {
			return trace.BadParameter("app name must be either present in the app manifest or provided on the command line")
		}
		req.PackageName = manifest.Metadata.Name
	}

	if req.PackageVersion == "" {
		if manifest.Metadata.ResourceVersion == "" {
			return trace.BadParameter("app version must be either present in the app manifest or provided on the command line")
		}
		req.PackageVersion = manifest.Metadata.ResourceVersion
	}

	// check dependencies-packages
	for _, dep := range manifest.Dependencies.Packages {
		locator, err := r.processMetadata(dep.Locator)
		if err != nil {
			return trace.Wrap(err)
		}
		p, err := r.Packages.ReadPackageEnvelope(locator)
		if err != nil && !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		if p == nil {
			return trace.BadParameter("missing dependency: package %v", dep)
		}
	}

	// check dependencies-apps
	for _, dep := range manifest.Dependencies.Apps {
		locator, err := r.processMetadata(dep.Locator)
		if err != nil {
			return trace.Wrap(err)
		}
		envelope, err := r.Packages.ReadPackageEnvelope(locator)
		if err != nil && !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		if envelope == nil {
			return trace.BadParameter("missing dependency: app %v", dep)
		}
	}

	return nil
}

func (r *applications) processMetadata(locator loc.Locator) (loc.Locator, error) {
	locatorPtr, err := pack.ProcessMetadata(r.Packages, &locator)
	if err != nil {
		return locator, trace.Wrap(err)
	}
	return *locatorPtr, nil
}

func (r *applications) getKubeClient() (_ *kubernetes.Clientset, err error) {
	r.Lock()
	defer r.Unlock()
	if r.Client == nil {
		r.Client, err = r.GetClient()
		if err != nil {
			return nil, trace.Wrap(err, "failed to create Kubernetes client")
		}
	}
	return r.Client, nil
}

type applications struct {
	// Mutex guards Client
	sync.Mutex
	Config
}
