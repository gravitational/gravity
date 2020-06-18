/*
Copyright 2019 Gravitational, Inc.

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

package phases

import (
	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/loc"
)

const (
	// UpdateConfig defines the phase to update runtime configuration package
	UpdateConfig = "update-config"
	// RestartContainer defines the phase to restart runtime container to make the
	// configuration package effective
	RestartContainer = "restart"
	// Elections defines the phase to control election / leadership on
	// a set of nodes
	Elections = "elections"
	// Taint defines the phase to add a taint to a node
	Taint = "taint"
	// Custom defines the phase that runs custom tasks before the node accepts
	// generic workloads during upgrade
	Custom = "custom"
	// Untaint defines the phase to remove the previously added taint from a node
	Untaint = "untaint"
	// Drain defines the phase to drain a node
	Drain = "drain"
	// Uncordon defines the phase to uncordon a node
	Uncordon = "uncordon"
	// Endpoints defines the phase to wait for endpoints on a node to be become active
	Endpoints = "endpoints"
)

type appGetter interface {
	GetApp(loc.Locator) (*app.Application, error)
}
