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

package storage

import (
	"time"

	"github.com/gravitational/gravity/lib/loc"

	teledefaults "github.com/gravitational/teleport/lib/defaults"
	teleservices "github.com/gravitational/teleport/lib/services"
	"github.com/jonboulle/clockwork"
)

// App defines an app resource
type App interface {
	teleservices.Resource
	// GetRepository returns app repository
	GetRepository() string
}

// NewApp creates a new app from the provided locator
func NewApp(locator loc.Locator) *AppV2 {
	return &AppV2{
		Kind:    KindApp,
		Version: locator.Version,
		Metadata: teleservices.Metadata{
			Name:      locator.Name,
			Namespace: teledefaults.Namespace,
		},
		Spec: AppSpecV2{
			Repository: locator.Repository,
		},
	}
}

// AppV2 represents an app resource format
type AppV2 struct {
	// Kind is resource kind, should be "app"
	Kind string `json:"kind"`
	// Version is the app version
	Version string `json:"version"`
	// Metadata is resource metadata
	Metadata teleservices.Metadata `json:"metadata"`
	// Spec is the app spec
	Spec AppSpecV2 `json:"spec"`
}

// AppSpecV2 represents an app resource spec
type AppSpecV2 struct {
	// Repository is repository app belongs to
	Repository string `json:"repository"`
}

// GetName returns the app name
func (a *AppV2) GetName() string {
	return a.Metadata.Name
}

// SetName sets the app name
func (a *AppV2) SetName(name string) {
	a.Metadata.Name = name
}

// GetMetadata returns the app metadata
func (a *AppV2) GetMetadata() teleservices.Metadata {
	return a.Metadata
}

// Expiry returns the resource expiration time
func (a *AppV2) Expiry() time.Time {
	return a.Metadata.Expiry()
}

// SetExpiry sets the resource expiration time
func (a *AppV2) SetExpiry(expires time.Time) {
	a.Metadata.SetExpiry(expires)
}

// SetTTL sets the resource TTL
func (a *AppV2) SetTTL(clock clockwork.Clock, ttl time.Duration) {
	a.Metadata.SetTTL(clock, ttl)
}

// GetRepository returns repository the app belongs to
func (a *AppV2) GetRepository() string {
	return a.Spec.Repository
}
