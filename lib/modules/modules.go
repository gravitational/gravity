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
	"fmt"
	"sync"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/teleport"
	teleservices "github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/version"
)

// Modules allows to customize certain behavioral aspects of Telekube
type Modules interface {
	// ProcessModes returns a list of modes gravity process can run in
	ProcessModes() []string
	// InstallModes returns a list of modes gravity install supports
	InstallModes() []string
	// DefaultAuthPreference returns default authentication preference based on process mode
	DefaultAuthPreference(processMode string) (teleservices.AuthPreference, error)
	// SupportedResources returns a list of resources that can be created/viewed
	SupportedResources() []string
	// SupportedResourcesToRemoves returns a list of resources that can be removed
	SupportedResourcesToRemove() []string
	// SupportedConnectors returns a list of supported auth connector kinds
	SupportedConnectors() []string
	// Version returns the gravity version
	Version() Version
	// TeleRepository returns the default repository for tele package cache
	TeleRepository() string
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

// SupportedResources returns a list of resources that can be created/viewed
func (m *defaultModules) SupportedResources() []string {
	return storage.SupportedGravityResources
}

// SupportedResourcesToRemoves returns a list of resources that can be removed
func (m *defaultModules) SupportedResourcesToRemove() []string {
	return storage.SupportedGravityResourcesToRemove
}

// SupportedConnectors returns a list of supported auth connector kinds
func (m *defaultModules) SupportedConnectors() []string {
	return []string{
		teleservices.KindOIDCConnector,
		teleservices.KindGithubConnector,
	}
}

// Version returns the gravity version
func (m *defaultModules) Version() Version {
	ver := version.Get()
	return Version{
		Edition:   "open-source",
		Version:   ver.Version,
		GitCommit: ver.GitCommit,
	}
}

// TeleRepository returns the default repository for tele package cache
func (m *defaultModules) TeleRepository() string {
	return fmt.Sprintf("s3://%v", defaults.HubBucket)
}

// Version represents gravity version
type Version struct {
	// Edition is the gravity edition, e.g. open-source
	Edition string `json:"edition"`
	// Version is the gravity semantic version
	Version string `json:"version"`
	// GitCommit is the git commit hash
	GitCommit string `json:"gitCommit"`
}

var (
	mutex           = sync.Mutex{}
	modules Modules = &defaultModules{}
)
