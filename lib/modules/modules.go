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

package modules

import (
	"sync"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/rpc/proto"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/teleport"
	teleservices "github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/version"
	helm "k8s.io/helm/pkg/version"
)

// Modules allows to customize certain behavioral aspects of Gravity
type Modules interface {
	// ProcessModes returns a list of modes gravity process can run in
	ProcessModes() []string
	// InstallModes returns a list of modes gravity install supports
	InstallModes() []string
	// DefaultAuthPreference returns default authentication preference based on process mode
	DefaultAuthPreference(processMode string) (teleservices.AuthPreference, error)
	// SupportedConnectors returns a list of supported auth connector kinds
	SupportedConnectors() []string
	// Version returns the tool version
	Version() proto.Version
	// TeleRepository returns the default repository for tele package cache
	TeleRepository() string
}

// Resources defines the interface to query tool resource support
type Resources interface {
	// SupportedResources returns a list of resources that can be created/viewed
	SupportedResources() []string
	// SupportedResourcesToRemove returns a list of resources that can be removed
	SupportedResourcesToRemove() []string
	// CanonicalKind translates the specified kind to canonical form.
	// Returns an empty string if no canonical form exists
	CanonicalKind(kind string) string
}

// Messager provides methods for various informational messages
type Messager interface {
	// PostInstallMessage returns a message that gets printed to console after successful installation
	PostInstallMessage() string
}

// Set sets the modules interface
func Set(m Modules) {
	mutex.Lock()
	defer mutex.Unlock()
	modules = m
}

// Get returns the modules interface
func Get() Modules {
	mutex.Lock()
	defer mutex.Unlock()
	return modules
}

// GetResources returns the resources interface
func GetResources() Resources {
	mutex.Lock()
	defer mutex.Unlock()
	return resources
}

// SetResources sets the resources interface
func SetResources(r Resources) {
	mutex.Lock()
	defer mutex.Unlock()
	resources = r
}

type defaultModules struct{}

// ProcessModes returns a list of modes gravity process can run in
func (m *defaultModules) ProcessModes() []string {
	return []string{
		constants.ComponentSite,
		constants.ComponentInstaller,
	}
}

// InstallModes returns a list of modes gravity install supports
func (m *defaultModules) InstallModes() []string {
	return []string{
		constants.InstallModeInteractive,
		constants.InstallModeCLI,
	}
}

// DefaultAuthPreference returns default auth preference based on run mode
func (m *defaultModules) DefaultAuthPreference(string) (teleservices.AuthPreference, error) {
	return teleservices.NewAuthPreference(
		teleservices.AuthPreferenceSpecV2{
			Type:         teleport.Local,
			SecondFactor: teleport.OFF,
		})
}

// SupportedConnectors returns a list of supported auth connector kinds
func (m *defaultModules) SupportedConnectors() []string {
	return []string{
		teleservices.KindOIDCConnector,
		teleservices.KindGithubConnector,
	}
}

// Version returns the gravity version
func (m *defaultModules) Version() proto.Version {
	ver := version.Get()
	return proto.Version{
		Edition:   "open-source",
		Version:   ver.Version,
		GitCommit: ver.GitCommit,
		Helm:      helm.Version,
	}
}

// TeleRepository returns the default repository for tele package cache
func (m *defaultModules) TeleRepository() string {
	return fmt.Sprintf("s3://%v", defaults.HubBucket)
}

// PostInstallMessage returns message that gets printed to console after
// successful installation.
func (m *defaultModules) PostInstallMessage() string {
	return `Congratulations!
The cluster is up and running. Please take a look at "cluster management" section:
https://gravitational.com/gravity/docs/cluster/`
}

type defaultResources struct{}

// SupportedResources returns a list of resources that can be created/viewed
func (*defaultResources) SupportedResources() []string {
	return storage.SupportedGravityResources
}

// SupportedResourcesToRemove returns a list of resources that can be removed
func (*defaultResources) SupportedResourcesToRemove() []string {
	return storage.SupportedGravityResourcesToRemove
}

// CanonicalKind translates the specified kind to canonical form.
// Returns an empty string if no canonical form exists
func (*defaultResources) CanonicalKind(kind string) string {
	return storage.CanonicalKind(kind)
}

var (
	mutex               = sync.Mutex{}
	modules   Modules   = &defaultModules{}
	resources Resources = &defaultResources{}
)
