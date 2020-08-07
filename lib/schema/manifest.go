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

package schema

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubeschema "k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
)

// Manifest represents an application manifest that describes a Gravity application or cluster image
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type Manifest struct {
	// Header provides basic information about application
	Header
	// BaseImage specifies the cluster image that's used as a base image
	BaseImage *BaseImage `json:"baseImage,omitempty"`
	// Logo is the application logo; can be either a filename (file://) or
	// an HTTP address (http://) or base64 encoded image data in the format
	// that can be used in a web page
	Logo string `json:"logo,omitempty"`
	// ReleaseNotes is the application's release notes; can be either a filename
	// (file://) or an HTTP address (http://) or plain text
	ReleaseNotes string `json:"releaseNotes,omitempty"`
	// Endpoints is a list of application endpoints
	Endpoints []Endpoint `json:"endpoints,omitempty"`
	// Dependencies is other apps/packages the application depends on
	Dependencies Dependencies `json:"dependencies,omitempty"`
	// Installer customizes the installer behavior
	Installer *Installer `json:"installer,omitempty"`
	// NodeProfiles describes types of nodes the application supports
	NodeProfiles NodeProfiles `json:"nodeProfiles,omitempty"`
	// Providers contains settings specific to different providers (e.g. cloud)
	Providers *Providers `json:"providers,omitempty"`
	// Storage configures persistent storage providers.
	Storage *Storage `json:"storage,omitempty"`
	// License allows to turn on/off license mode for the application
	License *License `json:"license,omitempty"`
	// Hooks contains application-defined hooks
	Hooks *Hooks `json:"hooks,omitempty"`
	// SystemOptions contains various global settings
	SystemOptions *SystemOptions `json:"systemOptions,omitempty"`
	// Extensions allows to enable/disable various custom features
	Extensions *Extensions `json:"extensions,omitempty"`
	// WebConfig allows to specify config.js used by UI to customize installer
	WebConfig string `json:"webConfig,omitempty"`
}

// BaseImage defines a base image type which is basically a locator with
// custom marshal/unmarshal.
type BaseImage struct {
	// Locator is the base image locator.
	Locator loc.Locator
}

// MarshalJSON marshals base image into a JSON string.
func (b *BaseImage) MarshalJSON() ([]byte, error) {
	if b == nil || b.Locator.IsEmpty() {
		return nil, nil
	}
	return json.Marshal(b.Locator.String())
}

// UnmarshalJSON unmarshals base image from a JSON string.
func (b *BaseImage) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	var str string
	err := json.Unmarshal(data, &str)
	if err != nil {
		return trace.Wrap(err)
	}
	locator, err := loc.MakeLocator(str)
	if err != nil {
		return trace.Wrap(err)
	}
	*b = BaseImage{Locator: *locator}
	return nil
}

// GetObjectKind returns the manifest header
func (m Manifest) GetObjectKind() kubeschema.ObjectKind {
	return &m.Header.TypeMeta
}

// Base returns a locator of a base application (runtime) the application
// depends on.
//
// Only cluster images can have runtimes.
func (m Manifest) Base() *loc.Locator {
	switch m.Kind {
	case KindBundle, KindCluster:
	default:
		return nil
	}
	if m.BaseImage != nil && !m.BaseImage.Locator.IsEmpty() {
		locator := m.BaseImage.Locator
		// When specifying base image in manifest, users use "gravity" but
		// the actual runtime app is called "kubernetes" so translate the
		// name here.
		if locator.Name == constants.BaseImageName {
			locator.Name = defaults.Runtime
		}
		return &locator
	}
	if m.SystemOptions == nil || m.SystemOptions.Runtime == nil {
		return &loc.Runtime
	}
	return &m.SystemOptions.Runtime.Locator
}

// Locator returns locator for this manifest's app
func (m Manifest) Locator() loc.Locator {
	return loc.Locator{
		Repository: m.Metadata.Repository,
		Name:       m.Metadata.Name,
		Version:    m.Metadata.ResourceVersion,
	}
}

// SetBase sets a runtime application to the provided locator
func (m *Manifest) SetBase(locator loc.Locator) {
	if m.SystemOptions == nil {
		m.SystemOptions = &SystemOptions{}
	}
	m.SystemOptions.Runtime = &Runtime{
		Locator: locator,
	}
	// Only update baseImage property if it was originally set to preserve
	// backward compatibility - otherwise older clusters will reject the
	// manifest due to additional property restriction.
	if m.BaseImage != nil {
		m.BaseImage.Locator = locator
	}
}

// WithBase returns a copy of this manifest with the specified base application
func (m *Manifest) WithBase(locator loc.Locator) Manifest {
	result := *m
	result.SetBase(locator)
	return result
}

// FindFlavor returns a flavor by the provided name
func (m Manifest) FindFlavor(name string) *Flavor {
	if m.Installer != nil {
		for _, flavor := range m.Installer.Flavors.Items {
			if flavor.Name == name {
				return &flavor
			}
		}
	}
	return nil
}

// FlavorNames returns a list of all defined flavors
func (m Manifest) FlavorNames() []string {
	var names []string
	if m.Installer != nil {
		for _, flavor := range m.Installer.Flavors.Items {
			names = append(names, flavor.Name)
		}
	}
	return names
}

// SetupEndpoint returns the endpoint that is used at the post-installation step
//
// Currently only one setup endpoint is supported, so if multiple
// are defines in the manifest, only the first one is returned
func (m Manifest) SetupEndpoint() *Endpoint {
	if m.Installer != nil && len(m.Installer.SetupEndpoints) != 0 {
		name := m.Installer.SetupEndpoints[0]
		for _, endpoint := range m.Endpoints {
			if endpoint.Name == name {
				return &endpoint
			}
		}
	}
	return nil
}

// GetNetworkType looks up network type for the specified provider / provisioner pair
func (m Manifest) GetNetworkType(provider, provisioner string) string {
	switch provider {
	case ProviderAWS, ProvisionerAWSTerraform:
		return m.Providers.AWS.Networking.Type
	}
	return m.Providers.Generic.Networking.Type
}

// HasHook returns true if manifest defines hook of the specified type
func (m Manifest) HasHook(hook HookType) bool {
	_, err := HookFromString(hook, m)
	return err == nil
}

// Docker returns docker configuration for the specified node profile.
// With no explicit configuration, default docker configuration is returned
func (m Manifest) Docker(profile NodeProfile) Docker {
	config := profile.SystemOptions.DockerConfig()
	if config == nil {
		config = m.SystemOptions.DockerConfig()
	}
	return dockerConfigWithDefaults(config)
}

// SystemDocker returns global docker configuration
func (m Manifest) SystemDocker() Docker {
	return dockerConfigWithDefaults(m.SystemOptions.DockerConfig())
}

// DescribeKind returns a human-friendly short description of the manifest kind.
func (m Manifest) DescribeKind() string {
	switch m.Kind {
	case KindBundle, KindCluster:
		return "Cluster"
	case KindApplication:
		return "Application"
	case KindSystemApplication:
		return "System application"
	case KindRuntime:
		return "Runtime"
	default:
		return m.Kind
	}
}

// ImageType returns the image type this manifest represents, cluster or application.
func (m Manifest) ImageType() string {
	switch m.Kind {
	case KindBundle, KindCluster:
		return KindCluster
	case KindApplication:
		return KindApplication
	default:
		return ""
	}
}

// EULA returns the end-user license agreement text.
func (m Manifest) EULA() string {
	if m.Installer != nil {
		return m.Installer.EULA.Source
	}
	return ""
}

func dockerConfigWithDefaults(config *Docker) Docker {
	if config == nil {
		return defaultDocker
	}
	docker := *config
	if docker.StorageDriver == "" {
		docker.StorageDriver = constants.DockerStorageDriverOverlay2
	}
	if docker.Capacity == 0 {
		docker.Capacity = defaultDockerCapacity
	}
	return docker
}

// EtcdArgs returns the list of additional etcd arguments for the specified node profile
func (m Manifest) EtcdArgs(profile NodeProfile) []string {
	args := profile.SystemOptions.EtcdArgs()
	if len(args) != 0 {
		return args
	}
	return append(args, m.SystemOptions.EtcdArgs()...)
}

// KubeletArgs returns the list of additional kubelet arguments for the specified node profile
func (m Manifest) KubeletArgs(profile NodeProfile) []string {
	args := profile.SystemOptions.KubeletArgs()
	if len(args) != 0 {
		return args
	}
	return append(args, m.SystemOptions.KubeletArgs()...)
}

// RuntimeArgs returns the list of additional runtime arguments for the specified node profile
func (m Manifest) RuntimeArgs(profile NodeProfile) []string {
	args := profile.SystemOptions.RuntimeArgs()
	if len(args) != 0 {
		return args
	}
	return append(args, m.SystemOptions.RuntimeArgs()...)
}

// RuntimePackageForProfile returns the planet package for the specified profile
func (m Manifest) RuntimePackageForProfile(profileName string) (*loc.Locator, error) {
	profile, err := m.NodeProfiles.ByName(profileName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return m.RuntimePackage(*profile)
}

// RuntimePackage returns the planet package for the specified profile.
// If the profile does not specify a runtime package, the default runtime
// package is returned
func (m Manifest) RuntimePackage(profile NodeProfile) (*loc.Locator, error) {
	if profile.SystemOptions != nil && profile.SystemOptions.Dependencies.Runtime != nil {
		return &profile.SystemOptions.Dependencies.Runtime.Locator, nil
	}
	return m.DefaultRuntimePackage()
}

// DefaultRuntimePackage returns the default runtime package
func (m Manifest) DefaultRuntimePackage() (*loc.Locator, error) {
	if m.SystemOptions == nil || m.SystemOptions.Dependencies.Runtime == nil {
		return nil, trace.NotFound("no runtime specified in manifest")
	}
	return &m.SystemOptions.Dependencies.Runtime.Locator, nil
}

// RuntimeImages returns the list of all runtime images.
func (m Manifest) RuntimeImages() (images []string) {
	if m.SystemOptions != nil && m.SystemOptions.BaseImage != "" {
		images = append(images, m.SystemOptions.BaseImage)
	}
	for _, profile := range m.NodeProfiles {
		if profile.SystemOptions != nil && profile.SystemOptions.BaseImage != "" {
			images = append(images, profile.SystemOptions.BaseImage)
		}
	}
	return images
}

// AllPackageDependencies returns the list of all available package dependencies
func (m Manifest) AllPackageDependencies() (deps []loc.Locator) {
	if m.SystemOptions != nil && m.SystemOptions.Dependencies.Runtime != nil {
		deps = append(deps, m.SystemOptions.Dependencies.Runtime.Locator)
	}
	deps = append(deps, m.NodeProfiles.RuntimePackages()...)
	return loc.Deduplicate(append(m.Dependencies.GetPackages(), deps...))
}

// PackageDependencies returns the list of package dependencies
// for the specified profile
func (m Manifest) PackageDependencies(profile string) (deps []loc.Locator, err error) {
	runtimePackage, err := m.RuntimePackageForProfile(profile)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return loc.Deduplicate(append(m.Dependencies.GetPackages(), *runtimePackage)), nil
}

// DefaultProvider returns the default cloud provider or an empty string.
func (m Manifest) DefaultProvider() string {
	if m.Providers != nil {
		return m.Providers.Default
	}
	return ""
}

// FilterDisabledDependencies filters the provided list of application locators and
// returns only those that are enabled based on the manifest settings.
func (m Manifest) FilterDisabledDependencies(apps []loc.Locator) (result []loc.Locator) {
	for _, app := range apps {
		if !ShouldSkipApp(m, app.Name) {
			result = append(result, app)
		}
	}
	return result
}

// SystemSettingsChanged returns true if system settings in this manifest
// changed compared to the provided manifest.
func (m Manifest) SystemSettingsChanged(other Manifest) bool {
	return m.PrivilegedEnabled() != other.PrivilegedEnabled()
}

// FirstNodeProfile returns the first available node profile.
func (m Manifest) FirstNodeProfile() (*NodeProfile, error) {
	if len(m.NodeProfiles) == 0 {
		return nil, trace.NotFound("manifest does not define any node profiles")
	}
	return &m.NodeProfiles[0], nil
}

// FirstNodeProfileName returns the name of the first available node profile.
func (m Manifest) FirstNodeProfileName() (string, error) {
	profile, err := m.FirstNodeProfile()
	if err != nil {
		return "", trace.Wrap(err)
	}
	return profile.Name, nil
}

// OpenEBSEnabled returns true if OpenEBS storage provider is enabled.
func (m Manifest) OpenEBSEnabled() bool {
	return m.Storage != nil && m.Storage.OpenEBS != nil && m.Storage.OpenEBS.Enabled
}

// PrivilegedEnabled returns true if privileged containers should be allowed.
func (m Manifest) PrivilegedEnabled() bool {
	return m.SystemOptions != nil && m.SystemOptions.AllowPrivileged || m.OpenEBSEnabled()
}

// CatalogDisabled returns true if the application catalog feature is disabled.
func (m Manifest) CatalogDisabled() bool {
	return m.Extensions != nil && m.Extensions.Catalog != nil && m.Extensions.Catalog.Disabled
}

// Header is manifest header
type Header struct {
	metav1.TypeMeta
	// Metadata is the application metadata
	Metadata Metadata `json:"metadata,omitempty"`
}

// GetVersion returns the manifest version
func (h Header) GetVersion() string {
	return h.APIVersion
}

// Metadata is the application metadata
type Metadata struct {
	// Name is the application name
	Name string `json:"name,omitempty"`
	// ResourceVersion is the application version in semver format
	ResourceVersion string `json:"resourceVersion,omitempty"`
	// Namespace is the application namespace
	Namespace string `json:"namespace,omitempty"`
	// Repository is the repository where application package resides;
	// it is normally not used and always equals to "gravitational.io"
	Repository string `json:"repository,omitempty"`
	// Description is the application description
	Description string `json:"description,omitempty"`
	// Author is the application author
	Author string `json:"author,omitempty"`
	// CreatedTimestamp is a timestamp the application package was built at
	CreatedTimestamp time.Time `json:"createdTimestamp,omitempty"`
	// Hidden allows to hide the app from a list of apps visible in Ops Center
	Hidden bool `json:"hidden,omitempty"`
	// Labels is labels attached to the manifest
	Labels map[string]string `json:"labels,omitempty"`
}

// GetName returns an application name
func (m Metadata) GetName() string {
	return m.Name
}

// Endpoint describes an application endpoint
type Endpoint struct {
	// Name is the endpoint short name
	Name string `json:"name,omitempty"`
	// Description is the endpoint verbose description
	Description string `json:"description,omitempty"`
	// Selector is the endpoint's k8s service selector
	Selector map[string]string `json:"selector,omitempty"`
	// ServiceName is the endpoint's k8s service name
	ServiceName string `json:"serviceName,omitempty"`
	// Namespace is the endpoint's k8s namespace
	Namespace string `json:"namespace,omitempty"`
	// Protocol is the endpoint protocol
	Protocol string `json:"protocol,omitempty"`
	// Port is the endpoint port
	Port int `json:"port,omitempty"`
	// Hidden is whether to hide the endpoint from the UI
	Hidden bool `json:"hidden,omitempty"`
}

// Dependencies describes application dependencies
type Dependencies struct {
	// Packages is a list of dependencies-packages
	Packages []Dependency `json:"packages,omitempty"`
	// Apps is a list of dependencies-apps
	Apps []Dependency `json:"apps,omitempty"`
}

// ByName returns a dependency package locator by its name
func (d Dependencies) ByName(names ...string) (*loc.Locator, error) {
	for _, dep := range append(d.Packages, d.Apps...) {
		if utils.StringInSlice(names, dep.Locator.Name) {
			return &dep.Locator, nil
		}
	}
	return nil, trace.NotFound("dependencies %q are not defined in the manifest", names)
}

// GetPackages returns a list of all package dependencies except the runtime
// package which is described in systemOptions
func (d Dependencies) GetPackages() []loc.Locator {
	packages := make([]loc.Locator, 0, len(d.Packages))
	for _, dep := range d.Packages {
		if loc.IsPlanetPackage(dep.Locator) {
			continue
		}
		packages = append(packages, dep.Locator)
	}
	return packages
}

// GetApps returns a list of all application dependencies
func (d Dependencies) GetApps() []loc.Locator {
	apps := make([]loc.Locator, 0, len(d.Apps))
	for _, app := range d.Apps {
		apps = append(apps, app.Locator)
	}
	return loc.Deduplicate(apps)
}

// Dependency represents a package or app dependency
type Dependency struct {
	// Locator is dependency package locator
	Locator loc.Locator
}

// MarshalJSON marshals dependency into a JSON string
func (d *Dependency) MarshalJSON() ([]byte, error) {
	bytes, err := json.Marshal(d.Locator.String())
	return bytes, trace.Wrap(err)
}

// UnmarshalJSON unmarshals dependency from a JSON string
func (d *Dependency) UnmarshalJSON(data []byte) error {
	var locator string
	if err := json.Unmarshal(data, &locator); err != nil {
		return trace.Wrap(err)
	}
	parsed, err := loc.ParseLocator(locator)
	if err != nil {
		return trace.Wrap(err)
	}
	*d = Dependency{Locator: *parsed}
	return nil
}

// Installer contains installer customizations
type Installer struct {
	// EULA describes the application end user license agreement
	EULA EULA `json:"eula,omitempty"`
	// SetupEndpoints is a list of names of endpoints to use in
	// post installation; only one is actually supported for now
	SetupEndpoints []string `json:"setupEndpoints,omitempty"`
	// Flavors defines application flavors
	Flavors Flavors `json:"flavors,omitempty"`
}

// EULA describes the application end user license agreement
type EULA struct {
	// Source is the license text URL (file:// or http://) or literal text
	Source string `json:"source,omitempty"`
}

// Flavors describes the application flavors
type Flavors struct {
	// Default is the name of the default application flavor
	Default string `json:"default,omitempty"`
	// Prompt is a phrase or a question describing the criteria that
	// should be considered when picking a flavor (e.g. "How many
	// requests per second do you want to process?")
	Prompt string `json:"prompt,omitempty"`
	// Description is a general description for the application flavors
	Description string `json:"description,omitempty"`
	// Items is a list of flavors
	Items []Flavor `json:"items,omitempty"`
}

// Flavor describes a single application flavor
type Flavor struct {
	// Name is the flavor name
	Name string `json:"name,omitempty"`
	// Description is verbose flavor description
	Description string `json:"description,omitempty"`
	// Nodes defines a list of node profiles composing the flavor
	Nodes []FlavorNode `json:"nodes,omitempty"`
}

// FlavorNode describes a single node profile for a flavor
type FlavorNode struct {
	// Profile is a node profile name
	Profile string `json:"profile,omitempty"`
	// Count is number of nodes of this profile
	Count int `json:"count,omitempty"`
}

// NodeProfiles is a list of node profiles
type NodeProfiles []NodeProfile

// ByName returns a node profile by its name
func (p NodeProfiles) ByName(name string) (*NodeProfile, error) {
	for _, profile := range p {
		if profile.Name == name {
			return &profile, nil
		}
	}
	return nil, trace.NotFound("node profile %q not found", name)
}

// RuntimePackages returns the list of runtime package dependencies for all node profiles
func (p NodeProfiles) RuntimePackages() (deps []loc.Locator) {
	for _, profile := range p {
		if profile.SystemOptions != nil && profile.SystemOptions.Dependencies.Runtime != nil {
			deps = append(deps, profile.SystemOptions.Dependencies.Runtime.Locator)
		}
	}
	return deps
}

// NodeProfile describes a single node
type NodeProfile struct {
	// Name is the profile name (role), e.g. "db"
	Name string `json:"name,omitempty"`
	// Description is a verbose profile description
	Description string `json:"description,omitempty"`
	// Requirements is a list of requirements the servers of this profile
	// should satisty
	Requirements Requirements `json:"requirements,omitempty"`
	// Labels is a list of labels nodes of this profile will be marked with
	Labels map[string]string `json:"labels,omitempty"`
	// Tains is a list of taints to apply to this profile
	Taints []corev1.Taint `json:"taints,omitempty"`
	// Providers contains some cloud provider specific settings
	Providers NodeProviders `json:"providers,omitempty"`
	// ExpandPolicy specifies whether nodes of this profile can
	// be resized
	ExpandPolicy string `json:"expandPolicy,omitempty"`
	// ServiceRole is the node's system role ("master" or "node")
	ServiceRole ServiceRole `json:"serviceRole,omitempty"`
	// SystemOptions defines optional system configuration for the node
	SystemOptions *SystemOptions `json:"systemOptions,omitempty"`
}

// Mounts returns a list of mounts
func (p NodeProfile) Mounts() []Volume {
	var mounts []Volume
	for _, v := range p.Requirements.Volumes {
		if v.TargetPath != "" {
			mounts = append(mounts, v)
		}
	}
	return mounts
}

// LabelValues returns a list of all labels in this server profile as "key=value"
func (p NodeProfile) LabelValues() []string {
	if len(p.Labels) == 0 {
		return []string{}
	}
	labels := make([]string, 0, len(p.Labels))
	for key, val := range p.Labels {
		labels = append(labels, fmt.Sprintf("%v=%v", key, val))
	}
	return labels
}

// TaintValues returns a list of all taints in this server profile as "key=value"
func (p NodeProfile) TaintValues() []string {
	if len(p.Taints) == 0 {
		return nil
	}
	taints := make([]string, 0, len(p.Taints))
	for _, taint := range p.Taints {
		taints = append(taints, fmt.Sprintf("%v=%v:%v", taint.Key, taint.Value, taint.Effect))
	}
	return taints
}

// Ports parses ports ranges from the node profile.
func (p NodeProfile) Ports() (tcp, udp []int, err error) {
	for _, ports := range p.Requirements.Network.Ports {
		for _, portRange := range ports.Ranges {
			parsed, err := utils.ParsePorts(portRange)
			if err != nil {
				return nil, nil, trace.Wrap(err)
			}
			switch ports.Protocol {
			case "tcp":
				tcp = append(tcp, parsed...)
			case "udp":
				udp = append(udp, parsed...)
			default:
				return nil, nil, trace.BadParameter("unknown protocol for port: %q", ports.Protocol)
			}
		}
	}
	return tcp, udp, nil
}

// Requirements defines a set of requirements for a node profile
type Requirements struct {
	// CPU describes CPU requirements
	CPU CPU `json:"cpu,omitempty"`
	// RAM describes RAM requirements
	RAM RAM `json:"ram,omitempty"`
	// OS describes OS requirements
	OS []OS `json:"os,omitempty"`
	// Network describes network requirements
	Network Network `json:"network,omitempty"`
	// Volumes describes volumes requirements
	Volumes []Volume `json:"volumes,omitempty"`
	// Devices describes devices that should be created inside container
	Devices []Device `json:"devices,omitempty"`
	// CustomChecks lists additional preflight checks as inline scripts
	CustomChecks []CustomCheck `json:"customChecks,omitempty"`
}

// Device describes a device that should be created inside container
type Device struct {
	// Path is the device path, treated as a glob, e.g. /dev/nvidia*
	Path string `json:"path"`
	// Permissions is the device permissions, a composition
	// of 'r' (read), 'w' (write) and 'm' (mknod)
	Permissions string `json:"permissions,omitempty"`
	// FileMode is the permission bits for the device
	FileMode string `json:"fileMode,omitempty"`
	// UID is the device user ID
	UID *int `json:"uid,omitempty"`
	// GID is the device group ID
	GID *int `json:"gid,omitempty"`
}

// Check makes sure all device parameters are correct
func (d Device) Check() error {
	if d.Path == "" {
		return trace.BadParameter("device path cannot be empty")
	}
	if d.Permissions != "" {
		if len(d.Permissions) > 3 || !devicePermsRegex.MatchString(d.Permissions) {
			return trace.BadParameter("invalid permissions %q for device %q: "+
				"must be a composition of 'r' (read), 'w' (write) and 'm' "+
				`(mknod), e.g. "rwm"`, d.Permissions, d.Path)
		}
	}
	if d.FileMode != "" {
		if _, err := strconv.ParseInt(d.FileMode, 8, 32); err != nil {
			return trace.BadParameter("invalid file mode %q for device %q: "+
				`must be an octal number, e.g. "0666"`, d.FileMode, d.Path)
		}
	}
	if d.UID != nil {
		if *d.UID < 0 {
			return trace.BadParameter("invalid UID %v for device %q: must "+
				"be >= 0", *d.UID, d.Path)
		}
	}
	if d.GID != nil {
		if *d.GID < 0 {
			return trace.BadParameter("invalid GID %v for device %q: must "+
				"be >= 0", *d.GID, d.Path)
		}
	}
	return nil
}

// Format formats the device to a string so it can be parsed later
func (d Device) Format() string {
	parts := []string{fmt.Sprintf("path=%v", d.Path)}
	if d.Permissions != "" {
		parts = append(parts, fmt.Sprintf("permissions=%v", d.Permissions))
	}
	if d.FileMode != "" {
		parts = append(parts, fmt.Sprintf("fileMode=%v", d.FileMode))
	}
	if d.UID != nil {
		parts = append(parts, fmt.Sprintf("uid=%v", *d.UID))
	}
	if d.GID != nil {
		parts = append(parts, fmt.Sprintf("gid=%v", *d.GID))
	}
	return strings.Join(parts, ";")
}

// CustomCheck defines a script that runs a custom preflight check
type CustomCheck struct {
	// Description provides a readable description for the check
	Description string `json:"description,omitempty"`
	// Script defines the contents of the check script.
	// It is provided to the shell verbatim in a temporary file
	Script string `json:"script,omitempty"`
}

// DevicesForProfile returns a list of required devices for the specified profile
func (m Manifest) DevicesForProfile(profileName string) ([]Device, error) {
	profile, err := m.NodeProfiles.ByName(profileName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return profile.Requirements.Devices, nil
}

// CPU describes CPU requirements
type CPU struct {
	// Min is minimum required amount of CPUs
	Min int `json:"min,omitempty"`
	// Max is maximum supported amount of CPUs
	Max int `json:"max,omitempty"`
}

// RAM describes RAM requirements
type RAM struct {
	// Min is minimum required amount of RAM
	Min utils.Capacity `json:"min,omitempty"`
	// Max is maximum supported amount of RAM
	Max utils.Capacity `json:"max,omitempty"`
}

// OS describes OS requirements
type OS struct {
	// Name is the required OS name (e.g. "centos", "redhat")
	Name string `json:"name,omitempty"`
	// Versions is supported OS versions
	Versions []string `json:"versions,omitempty"`
}

// Network describes network requirements
type Network struct {
	// MinTransferRate is minimum required transfer rate
	MinTransferRate utils.TransferRate `json:"minTransferRate,omitempty"`
	// Ports specifies port ranges that should be available on the server
	Ports []Port `json:"ports,omitempty"`
}

// Port describes port ranges
type Port struct {
	// Protocol is port protocol ("tcp", "udp")
	Protocol string `json:"protocol,omitempty"`
	// Ranges is a list of port ranges to check
	Ranges []string `json:"ranges,omitempty"`
}

// Volume describes a volume requirement
type Volume struct {
	// Name is a volume short name for referencing
	Name string `json:"name,omitempty"`
	// Path is a volume path on host
	Path string `json:"path,omitempty"`
	// TargetPath is a volume mount path inside planet
	TargetPath string `json:"targetPath,omitempty"`
	// Capacity is required capacity
	Capacity utils.Capacity `json:"capacity,omitempty"`
	// Filesystems is a list of supported filesystems
	Filesystems []string `json:"filesystems,omitempty"`
	// CreateIfMissing is whether to create directory on host if it's missing
	CreateIfMissing *bool `json:"createIfMissing,omitempty"`
	// SkipIfMissing avoids mounting a directory inside a container if it's missing on host.
	// This flag implies CreateIfMissing == false.
	// The rationale for this flag is to be able to define mounts optimistically
	// for multiple disparate locations (subject to specific OS distributions)
	// and only actually create mounts for existing directories.
	//
	// For a configuration with two mounts: /path/to/dir1 (as found on OS1)
	// and /path/to/dir2 (as found on OS2) to some location inside the container,
	// installing on OS1 will use /path/to/dir1 as a mount source,
	// while installing on OS2 will use /path/to/dir2.
	SkipIfMissing *bool `json:"skipIfMissing,omitempty"`
	// MinTransferRate is required disk speed
	MinTransferRate utils.TransferRate `json:"minTransferRate,omitempty"`
	// Hidden applies to mounts and means that the mount is not shown to a user in installer UI
	Hidden bool `json:"hidden,omitempty"`
	// UID sets UID for a volume path on the host
	UID *int `json:"uid,omitempty"`
	// GID sets GID for a volume path on the host
	GID *int `json:"gid,omitempty"`
	// Mode sets file mode for a volume path on the host
	// accepts octal format
	Mode string `json:"mode,omitempty"`
	// Recursive means that all mount points inside this mount should also be mounted
	Recursive bool `json:"recursive,omitempty"`
	// SELinuxLabel specifies the SELinux label.
	// If left unspecified, the default.ContainerFileLabel will be used to label the directory.
	// If a special value SELinuxLabelNone is specified - no labeling is performed
	SELinuxLabel string `json:"seLinuxLabel,omitempty"`
}

// CheckAndSetDefaults checks and sets defaults
func (v *Volume) CheckAndSetDefaults() error {
	if v.UID != nil {
		if *v.UID < 0 {
			return trace.BadParameter("uid should be >= 0")
		}
	}
	if v.GID != nil {
		if *v.GID < 0 {
			return trace.BadParameter("gid should be >= 0")
		}
	}
	if v.Mode != "" {
		_, err := v.FileMode()
		if err != nil {
			return trace.Wrap(err)
		}
	}
	if utils.BoolValue(v.SkipIfMissing) {
		// Turn off automatic directory creation for optimistic mounts
		v.CreateIfMissing = utils.BoolPtr(false)
	}
	return nil
}

// FileMode parses mode from octal string representation
// and returns os.FileMode instead
func (v Volume) FileMode() (os.FileMode, error) {
	if v.Mode == "" {
		return 0, trace.BadParameter("volume mode is not specified")
	}
	mode, err := strconv.ParseInt(v.Mode, 8, 32)
	if err != nil {
		return 0, trace.BadParameter("volume mode %q is not in valid format, expected '0755'", v.Mode)
	}
	return os.FileMode(mode), nil
}

// IsMount returns true if the volume defines a mount point
func (v Volume) IsMount() bool {
	return v.TargetPath != ""
}

// NodeProviders contains provider-specific node settings
type NodeProviders struct {
	// AWS contains AWS-specific settings
	AWS NodeProviderAWS `json:"aws,omitempty"`
}

// NodeProviderAWS describes AWS-specific node settings
type NodeProviderAWS struct {
	// InstanceTypes is a list of supported instance types
	InstanceTypes []string `json:"instanceTypes,omitempty"`
}

// Storage represents persistent storage configuration.
type Storage struct {
	// OpenEBS is the OpenEBS storage provider configuration.
	OpenEBS *OpenEBS `json:"openebs,omitempty"`
}

// OpenEBS represents OpenEBS configuration.
type OpenEBS struct {
	// Enabled indicates whether OpenEBS is enabled.
	Enabled bool `json:"enabled,omitempty"`
}

// Providers defines global provider-specific settings
type Providers struct {
	// Default specifies the default provider.
	Default string `json:"default,omitempty"`
	// AWS defines AWS-specific settings
	AWS AWS `json:"aws,omitempty"`
	// Azure defines Azure-specific settings
	Azure Azure `json:"azure,omitempty"`
	// Generic defines settings for a generic provider (e.g. onprem)
	Generic Generic `json:"generic,omitempty"`
}

// AWS defines AWS-specific settings
type AWS struct {
	// Networking describes networking configuration
	Networking Networking `json:"network,omitempty"`
	// Regions is a list of supported regions
	Regions []string `json:"regions,omitempty"`
	// IAMPolicy is a list of permissions
	IAMPolicy IAMPolicy `json:"iamPolicy,omitempty"`
	// Disabled is whether this provider should be disabled
	Disabled bool `json:"disabled,omitempty"`
}

// IAMPolicy defines a list of AWS permissions
type IAMPolicy struct {
	// Version is the policy version
	Version string `json:"version,omitempty"`
	// Actions is a list of IAM permissions (e.g. "ec2:CreateVolume")
	Actions []string `json:"actions,omitempty"`
}

// Azure defines Azure-specific settings
type Azure struct {
	// Disabled is whether this provider should be disabled
	Disabled bool `json:"disabled,omitempty"`
}

// Generic defines generic provider settings
type Generic struct {
	// Networking describes networking configuration
	Networking Networking `json:"network,omitempty"`
	// Disabled is whether this provider should be disabled
	Disabled bool `json:"disabled,omitempty"`
}

// Networking describes networking configuration
type Networking struct {
	// Type is networking type
	Type string `json:"type,omitempty"`
}

// License describes an application license
type License struct {
	// Enabled is whether licensing is enabled/disabled
	Enabled bool `json:"enabled,omitempty"`
	// Type is the license type
	Type string `json:"type,omitempty"`
}

// EtcdArgs returns a list of additional etcd arguments
func (r *SystemOptions) EtcdArgs() []string {
	if r == nil || r.Etcd == nil {
		return nil
	}
	return r.Etcd.Args
}

// DockerConfig returns docker configuration for this options object
func (r *SystemOptions) DockerConfig() *Docker {
	if r == nil {
		return nil
	}
	return r.Docker
}

// KubeletArgs returns a list of additional kubelet arguments
func (r *SystemOptions) KubeletArgs() []string {
	if r == nil || r.Kubelet == nil {
		return nil
	}
	return r.Kubelet.Args
}

// RuntimeArgs returns a list of additional runtime arguments
func (r *SystemOptions) RuntimeArgs() []string {
	if r == nil {
		return nil
	}
	return r.Args
}

// SystemOptions defines various global settings
type SystemOptions struct {
	// ExternalService specifies additional configuration for the runtime package
	ExternalService
	// Runtime describes the runtime the application is based on
	Runtime *Runtime `json:"runtime,omitempty"`
	// Docker describes docker options
	Docker *Docker `json:"docker,omitempty"`
	// Etcd describes etcd options
	Etcd *Etcd `json:"etcd,omitempty"`
	// Kubelet describes kubelet options
	Kubelet *Kubelet `json:"kubelet,omitempty"`
	// BaseImage optionally overrides the planet image.
	// If specified, the image is vendored locally and translated into a runtime package.
	// If this is specified for a specific node profile, only nodes with the profile
	// will have that runtime package installed.
	BaseImage string `json:"baseImage,omitempty"`
	// Dependencies defines additional package dependencies
	Dependencies SystemDependencies `json:"dependencies"`
	// AllowPrivileged controls whether privileged containers will be allowed
	// in the cluster.
	AllowPrivileged bool `json:"allowPrivileged,omitempty"`
}

// Runtime describes the application runtime
type Runtime struct {
	// Locator is the runtime package locator
	Locator loc.Locator
}

// MarshalJSON marshals runtime package
func (r Runtime) MarshalJSON() ([]byte, error) {
	bytes, err := json.Marshal(r.Locator)
	return bytes, trace.Wrap(err)
}

// UnmarshalJSON unmarshals runtime package
func (r *Runtime) UnmarshalJSON(data []byte) error {
	var l loc.Locator
	if err := json.Unmarshal(data, &l); err != nil {
		return trace.Wrap(err)
	}
	if l.Repository == "" {
		l.Repository = defaults.SystemAccountOrg
	}
	if l.Name == "" {
		l.Name = defaults.Runtime
	}
	if l.Version == "" {
		l.Version = loc.LatestVersion
	}
	locator, err := loc.NewLocator(l.Repository, l.Name, l.Version)
	if err != nil {
		return trace.Wrap(err)
	}
	*r = Runtime{Locator: *locator}
	return nil
}

// SystemDependencies defines additional system-level package dependencies
type SystemDependencies struct {
	// Runtime describes the runtime package
	Runtime *Dependency `json:"runtimePackage,omitempty"`
}

// Docker describes docker options
type Docker struct {
	// ExternalService defines additional configuration for the docker service
	ExternalService
	// StorageDriver is the docker storage driver to use
	StorageDriver string `json:"storageDriver,omitempty"`
	// Capacity is required docker device capacity
	Capacity utils.Capacity `json:"capacity,omitempty"`
}

// IsEmpty returns true if docker configuration is empty
func (d Docker) IsEmpty() bool {
	return d.StorageDriver == "" && d.Capacity == 0
}

// Etcd describes etcd options
type Etcd struct {
	// ExternalService defines additional configuration for the etcd service
	ExternalService
}

// Kubelet describes kubelet options
type Kubelet struct {
	// ExternalService defines additional configuration for kubelet
	ExternalService
	// HairpinMode is deprecated and no longer used
	HairpinMode string `json:"hairpinMode,omitempty"`
}

// ExternalService defines configuration for an external service.
type ExternalService struct {
	// Args is a list of extra arguments to provide to the service
	Args []string `json:"args,omitempty"`
}

// Extensions defines various custom application features
type Extensions struct {
	// Encryption allows to encrypt installer packages
	Encryption *EncryptionExtension `json:"encryption,omitempty"`
	// Logs allows to customize logs feature
	Logs *LogsExtension `json:"logs,omitempty"`
	// Monitoring allows to customize monitoring feature
	Monitoring *MonitoringExtension `json:"monitoring,omitempty"`
	// Kubernetes allows to customize kubernetes feature
	Kubernetes *KubernetesExtension `json:"kubernetes,omitempty"`
	// Configuration allows to customize configuration feature
	Configuration *ConfigurationExtension `json:"configuration,omitempty"`
	// Catalog allows to customize application catalog feature
	Catalog *CatalogExtension `json:"catalog,omitempty"`
}

// EncryptionExtension describes installer encryption extension
type EncryptionExtension struct {
	// EncryptionKey is the passphrase for installer encryption
	EncryptionKey string `json:"encryptionKey,omitempty"`
	// CACert is the certificate authority certificate
	CACert string `json:"caCert,omitempty"`
}

// Logs allows to customize logs feature
type LogsExtension struct {
	// Disabled allows to disable Logs tab
	Disabled bool `json:"disabled,omitempty"`
}

// Monitoring allows to customize monitoring feature
type MonitoringExtension struct {
	// Disabled allows to disable Monitoring tab
	Disabled bool `json:"disabled,omitempty"`
}

// CatalogExtension allows to customize application catalog feature
type CatalogExtension struct {
	// Disabled disables application catalog and tiller
	Disabled bool `json:"disabled,omitempty"`
}

// Kubernetes allows to customize kubernetes feature
type KubernetesExtension struct {
	// Disabled allows to disable Kubernetes tab
	Disabled bool `json:"disabled,omitempty"`
}

// Configuration allows to customize configuration feature
type ConfigurationExtension struct {
	// Disabled allows to disable Configuration tab
	Disabled bool `json:"disabled,omitempty"`
}

func init() {
	addKnownTypes(scheme.Scheme)
}

// addKnownTypes adds the list of known types to the given scheme.
func addKnownTypes(scheme *runtime.Scheme) {
	scheme.AddKnownTypeWithName(SchemeGroupVersion.WithKind(KindSystemApplication), &Manifest{})
	scheme.AddKnownTypeWithName(SchemeGroupVersion.WithKind(KindBundle), &Manifest{})
	scheme.AddKnownTypeWithName(SchemeGroupVersion.WithKind(KindRuntime), &Manifest{})
	scheme.AddKnownTypeWithName(SchemeGroupVersion.WithKind(KindCluster), &Manifest{})
	scheme.AddKnownTypeWithName(SchemeGroupVersion.WithKind(KindApplication), &Manifest{})
	scheme.AddKnownTypeWithName(ClusterGroupVersion.WithKind(KindCluster), &Manifest{})
	scheme.AddKnownTypeWithName(AppGroupVersion.WithKind(KindApplication), &Manifest{})
}

var (
	// SchemeGroupVersion defines group and version for the application manifest type in the kubernetes
	// resource scheme
	SchemeGroupVersion = kubeschema.GroupVersion{Group: GroupName, Version: Version}
	// ClusterGroupVersion defines group/version for the cluster image manifest
	ClusterGroupVersion = kubeschema.GroupVersion{Group: ClusterGroupName, Version: Version}
	// AppGroupVersion defines group/version for the app image manifest
	AppGroupVersion = kubeschema.GroupVersion{Group: AppGroupName, Version: Version}

	// defaultDockerCapacity is the default capacity for a docker device
	defaultDockerCapacity = utils.MustParseCapacity(defaults.DockerDeviceCapacity)
	// defaultDocker is the default docker settings
	defaultDocker = Docker{
		StorageDriver: constants.DockerStorageDriverOverlay2,
		Capacity:      defaultDockerCapacity,
	}

	// devicePermsRegex is a regular expression used to validate device permissions
	devicePermsRegex = regexp.MustCompile("^[rwm]+$")
)

// ShouldSkipApp returns true if the application given with appName should not be
// installed in the cluster described by the provided manifest.
func ShouldSkipApp(manifest Manifest, appName string) bool {
	switch appName {
	case defaults.BandwagonPackageName:
		// do not install bandwagon unless the app uses it in its post-install
		setup := manifest.SetupEndpoint()
		if setup == nil || setup.ServiceName != defaults.BandwagonServiceName {
			return true
		}
	case defaults.LoggingAppName:
		// do not install logging-app if logs feature is disabled
		ext := manifest.Extensions
		if ext != nil && ext.Logs != nil && ext.Logs.Disabled {
			return true
		}
	case defaults.MonitoringAppName:
		// do not install monitoring-app if logs feature is disabled
		ext := manifest.Extensions
		if ext != nil && ext.Monitoring != nil && ext.Monitoring.Disabled {
			return true
		}
	case defaults.TillerAppName:
		// do not install tiller-app if catalog feature is disabled
		ext := manifest.Extensions
		if ext != nil && ext.Catalog != nil && ext.Catalog.Disabled {
			return true
		}
	case defaults.StorageAppName:
		// do not install storage-app if no storage providers are enabled
		return !manifest.OpenEBSEnabled()
	}
	return false
}
