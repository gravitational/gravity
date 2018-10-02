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

// Package app implements gravity application support for import and configuration and management
package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/trace"
	"k8s.io/api/core/v1"
)

const (
	// DefaultNamespace is the name of the default application namespace.
	DefaultNamespace = "default"

	// LabelName defines the name label every application resource is annotated with
	LabelName = "g-app-name"
	// LabelVersion defines the version label every application resource is annotated with
	LabelVersion = "g-app-ver"

	// AppOperationImport defines an application import operation type
	AppOperationImport = "operation_app_import"
)

// Applications manages a collection of applications
type Applications interface {
	Operations

	// ListApps lists applications of given type in the specified repository.
	// If the repository is empty, all applications are listed.
	ListApps(ListAppsRequest) ([]Application, error)

	// GetApp obtains an application specified with locator.
	// If no application can be found, nil is returned.
	GetApp(locator loc.Locator) (*Application, error)

	// GetAppResources obtains application resources,
	// if the app does not exist, returns NotFound
	GetAppResources(locator loc.Locator) (io.ReadCloser, error)

	// ExportApp exports the containers of an existing application
	// to the docker registry specified in the request
	ExportApp(ExportAppRequest) error

	// UninstallApp uninstalls a running application from the local cluster
	UninstallApp(locator loc.Locator) (*Application, error)

	// StatusApp retrieves the status of a running application
	StatusApp(locator loc.Locator) (*Status, error)

	// CreateApp creates a new application from the specified package bytes (reader)
	// and an optional set of package labels using locator as destination for the
	// resulting package
	CreateApp(locator loc.Locator, reader io.Reader, labels map[string]string) (*Application, error)

	// CreateAppWithManifest creates a new application from the specified package bytes (reader)
	// and an optional set of package labels using locator as destination for the
	// resulting package, with supplied manifest
	CreateAppWithManifest(locator loc.Locator, manifest []byte, reader io.Reader, labels map[string]string) (*Application, error)

	// UpsertApp creates a new application or updates the existing one
	UpsertApp(locator loc.Locator, reader io.Reader, labels map[string]string) (*Application, error)

	// DeleteApp deletes an application record
	DeleteApp(DeleteRequest) error

	// GetAppManifest returns a reader to the application
	// manifest
	GetAppManifest(locator loc.Locator) (io.ReadCloser, error)

	// GetAppInstaller builds an installer package for the
	// specified application and returns a reader to the package
	// compressed as gzipped tar archive
	GetAppInstaller(InstallerRequest) (io.ReadCloser, error)

	// StartAppHook starts application hook specified with req asynchronously
	StartAppHook(ctx context.Context, req HookRunRequest) (*HookRef, error)

	// WaitAppHook waits for app hook to complete or fail
	WaitAppHook(ctx context.Context, ref HookRef) error

	// DeleteAppHookJob deletes app hook job
	DeleteAppHookJob(ctx context.Context, ref HookRef) error

	// StreamAppHookLogs streams app hook logs to output writer, this is a blocking call
	StreamAppHookLogs(ctx context.Context, ref HookRef, out io.Writer) error
}

// ListAppsRequest is a request to show applications in a repository
type ListAppsRequest struct {
	// Repository is repository to show apps for
	Repository string `json:"repository"`
	// Type is the app type to filter by
	Type storage.AppType `json:"type"`
	// ExcludeHidden is whether to exclude apps marked as 'hidden' from the output
	ExcludeHidden bool `json:"exclude_hidden"`
}

// Check validates the request
func (r ListAppsRequest) Check() error {
	if r.Repository == "" {
		return trace.BadParameter("repository must not be empty")
	}
	if err := r.Type.Check(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// ExportAppRequest defines a set of parameters for exporting an application to a docker registry
type ExportAppRequest struct {
	// Package represents the package name and version to be exported
	Package loc.Locator

	// RegistryAddress represents the registry to export the application images to
	RegistryAddress string

	// CertName represents the name of the certificate to use for TLS connecting to the registry
	CertName string
}

// Operations defines a set of operations on applications
type Operations interface {
	// CreateImportOperation initiates application import and returns the operation
	// key to use to query the operation progress
	CreateImportOperation(*ImportRequest) (*storage.AppOperation, error)

	// GetOperationProgress retrieves the progress of the application import
	// operation
	GetOperationProgress(storage.AppOperation) (*ProgressEntry, error)

	// GetOperationLogs retrieves the log for the specified import operation
	GetOperationLogs(storage.AppOperation) (io.ReadCloser, error)

	// GetImportedApplication queries the application data for the specified import operation
	GetImportedApplication(storage.AppOperation) (*Application, error)

	// GetOperationCrashReport retrieves the crash report for a failed import attempt
	GetOperationCrashReport(storage.AppOperation) (io.ReadCloser, error)
}

// InstallAppRequest defines a request to install an application
type InstallAppRequest struct {
	// Locator identifies the application package to install
	Locator loc.Locator `json:"locator"`
	// Env specifies optional environment varibales to provide to the install hook
	Env map[string]string `json:"env"`
}

// UpdateAppRequest defines a request to update an application
type UpdateAppRequest struct {
	// Locator identifies the application package to update
	Locator loc.Locator `json:"locator"`
	// Env specifies optional environment varibales to provide to the update hook
	Env map[string]string `json:"env"`
}

// HookRunRequest defines a request to run a specific application hook
type HookRunRequest struct {
	// Application identifies the application package to run hook for
	Application loc.Locator `json:"application"`
	// Hook defines the hook to run
	Hook schema.HookType `json:"hook"`
	// Volumes lists additional volumes to add inside a hook's job container
	Volumes []v1.Volume `json:"volumes"`
	// VolumeMounts lists additional volume mounts to create inside a hook's job container
	VolumeMounts []v1.VolumeMount `json:"volume_mounts"`
	// NodeSelector defines a set of labels to use to select a node for this hook's job.
	// If unspecified - the default behavior is to schedule job on a master node
	NodeSelector map[string]string `json:"node_selector"`
	// Env defines additional environment variables to pass to the hook job
	Env map[string]string `json:"env"`
	// SkipInitContainers skips injection of init containers
	SkipInitContainers bool `json:"skip_init_containers"`
	// Timeout allows to set timeout for the hook job to override default value
	// or value from manifest
	Timeout time.Duration `json:"timeout"`
	// GravityPackage specifies the gravity binary package to run hook with.
	// If empty, the hooks will use the binary from host.
	GravityPackage loc.Locator `json:"gravity_package"`
	// ServiceUser specifies a user for system services inside the planet container.
	// Also some system kubernetes resources are executed in the context of the
	// service user unless permission elevation is necessary.
	//
	// The application resources that do not require explicit permission elevation
	// should use a special placeholder user ID (defaults.PlaceholderServiceUserID)
	// which will be replaced with the ID of the effective service user during
	// installation and when running application hooks.
	ServiceUser storage.OSUser
}

// Check validates this request
func (r HookRunRequest) Check() error {
	if string(r.Hook) == "" {
		return trace.BadParameter("missing parameter Hook")
	}
	return nil
}

// HookRefis a reference to a hook running as a kubernetes job
type HookRef struct {
	// Application identifies the application package to run hook for
	Application loc.Locator `json:"application"`
	// Hook identifies the hook type to run
	Hook schema.HookType `json:"hook"`
	// Namespace is a resource namespace
	Namespace string `json:"namespace"`
	// Name is a resource name
	Name string `json:"name"`
}

// InstallerRequest defines a request to create an installer tarball
type InstallerRequest struct {
	// Account specifies which account the local site entries will use
	// when the remote sites `dial back` on the initial OpsCenter connection
	// handshake (AcceptRemoteCluster)
	Account storage.Account `json:"account_id"`
	// Application identifies the application package to create installer for
	Application loc.Locator `json:"application"`
	// TrustedCluster is used to preserve information about Ops Center
	// installer is being downloaded from so clusters can connect back
	TrustedCluster storage.TrustedCluster `json:"trusted_cluster"`
	// CACert is certificate authority cert to package with installer
	CACert string `json:"ca_cert,omitempty"`
	// EncryptionKey is encryption key to encrypt installer packages with
	EncryptionKey string `json:"encryption_key,omitempty"`
}

// Check validates this request
func (r InstallerRequest) Check() error {
	if r.Application.IsEmpty() {
		return trace.BadParameter("missing Application")
	}
	if r.EncryptionKey != "" && r.CACert == "" {
		return trace.BadParameter("CACert is required when EncryptionKey is provided")
	}
	return nil
}

// ToRaw converts the request to the format understood by API calls
func (r InstallerRequest) ToRaw() (*InstallerRequestRaw, error) {
	bytes, err := storage.MarshalTrustedCluster(r.TrustedCluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &InstallerRequestRaw{
		Account:        r.Account,
		Application:    r.Application,
		TrustedCluster: json.RawMessage(bytes),
		CACert:         r.CACert,
		EncryptionKey:  r.EncryptionKey,
	}, nil
}

// InstallerRequestRaw is the same as InstallerRequest but can be marshaled
// and unmarshaled when passed in API call
type InstallerRequestRaw struct {
	// Account is the request account
	Account storage.Account `json:"account_id"`
	// Application is the application to make installer for
	Application loc.Locator `json:"application"`
	// TrustedCluster preserves original Ops Center information
	TrustedCluster json.RawMessage `json:"trusted_cluster"`
	// CACert is certificate authority cert to package with installer
	CACert string `json:"ca_cert,omitempty"`
	// EncryptionKey is encryption key to encrypt installer packages with
	EncryptionKey string `json:"encryption_key,omitempty"`
}

// ToNative converts the request from API-friendly to its regular format
func (r InstallerRequestRaw) ToNative() (*InstallerRequest, error) {
	cluster, err := storage.UnmarshalTrustedCluster([]byte(r.TrustedCluster))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &InstallerRequest{
		Account:        r.Account,
		Application:    r.Application,
		TrustedCluster: cluster,
		CACert:         r.CACert,
		EncryptionKey:  r.EncryptionKey,
	}, nil
}

// ImportRequest is a request to import an application
type ImportRequest struct {
	// Source defines the source of the application data
	Source io.ReadCloser `json:"-"`
	// Repository is an optional repository name, overrides the one specified in the app manifest
	Repository string `json:"repository"`
	// PackageName is an optional app name, overrides the one specified in the app manifest
	PackageName string `json:"package_name"`
	// PackageVersion is an optional app version, overrides the one specified in the app manifest
	PackageVersion string `json:"package_version"`
	// Email is email address of a user who imported the application
	Email string `json:"email"`
	// ProgressC is an chan to receive progress events on
	// Callers are expected to receive on this chan to avoid
	// blocking the import operation.
	// Note, that ProgressC can closed before producing a single value
	ProgressC chan *ProgressEntry `json:"-"`
	// ErrorC is a chan to receive errors on
	// It is expected to be buffered to receive at least a single error
	ErrorC chan error `json:"-"`
	// Vendor defines whether the import operation will rewrite container images to point
	// to the private docker registry
	Vendor bool `json:"vendor"`
	// Force flag allows to overwrite existing application
	Force bool `json:"force"`
	// ResourcePatterns defines a list of file path patterns to consider for container
	// image references.
	ResourcePatterns []string `json:"resource_directories"`
	// IgnoreResourcePatterns defines a list of regular expressions to exclude files from searching
	// for container image references.
	IgnoreResourcePatterns []string `json:"ignore_resource_patterns"`
	// ExcludePatterns defines a set of file path patterns to exclude from the resulting
	// tarball. The patterns use the extended path pattern syntax (incl. double-star (`**`)
	// to denote arbitrary sub-directories)
	ExcludePatterns []string `json:"exclude_patterns"`
	// IncludePaths defines a list of directories to consider when creating a resulting
	// application tarball. If unspecified, all directories will be considered.
	IncludePaths []string `json:"include_paths"`
	// SetImages defines the list of docker image references with each item defining
	// a new tag to update for an image matching the specified registry/repository
	SetImages []loc.DockerImage `json:"set_images"`
	// SetDeps defines a list of package dependencies that will be set to the specified version
	SetDeps []loc.Locator `json:"set_deps"`
}

// DeleteRequest describes a request to delete an application
type DeleteRequest struct {
	// Package is the app package
	Package loc.Locator
	// Force allows to ignore constraints during deletion
	Force bool
}

// Application defines a Gravity application
type Application struct {
	// Package identifies the application package
	Package loc.Locator `json:"package"`
	// PackageEnvelope is the complete underlying package information
	PackageEnvelope pack.PackageEnvelope `json:"envelope"`
	// Manifest defines the application install configuration
	Manifest schema.Manifest `json:"manifest"`
}

// String formats the specified application for output
func (a Application) String() string {
	return fmt.Sprintf("%v:%v", a.Name(), a.Package.Version)
}

// Namespace returns the application namespace
func (a Application) Namespace() string {
	return DefaultNamespace
}

// Name returns the application name
func (a Application) Name() string {
	return a.Manifest.Metadata.Name
}

// RequiresLicense returns true/false depending on whether the app allows
// some or all of its flavors to work without a license
func (a Application) RequiresLicense() bool {
	if a.Manifest.License == nil || !a.Manifest.License.Enabled {
		return false
	}
	return true
}

// Logger defines an interface to log messages
type Logger interface {
	io.Writer

	Infof(format string, arg ...interface{})
}

type Status struct {
	Endpoints []struct {
		Protocol string `json:"protocol"`
		NodePort string `json:"node_port"`
	} `json:"endpoints"`
}

// ProgressEntry defines a single progress step in a complex operation
// (i.e. import)
type ProgressEntry storage.AppProgressEntry

// IsCompleted returns whether this progress entry identifies a completed
// (successful or failed) operation
func (r ProgressEntry) IsCompleted() bool {
	return r.Completion == constants.Completed
}
