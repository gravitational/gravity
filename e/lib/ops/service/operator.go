// Copyright 2021 Gravitational Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package service

import (
	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/ops/opsservice"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/users"
	"github.com/gravitational/teleport/lib/events"

	"github.com/mailgun/timetools"
)

// Operator extends the open-source operator service
type Operator struct {
	// Operator is the open-source operator
	*opsservice.Operator
	// installGroups are used to keep track of the nodes of the clusters
	// that are currently being installed via this Ops Center
	installGroups map[ops.SiteOperationKey]*installGroup
	// services is a set of goroutines this operator is maintaining
	services map[ops.SiteKey]map[string]service
}

// New returns an new enterprise operator
func New(ossOperator *opsservice.Operator) *Operator {
	return &Operator{
		Operator:      ossOperator,
		services:      map[ops.SiteKey]map[string]service{},
		installGroups: map[ops.SiteOperationKey]*installGroup{},
	}
}

// isOpsCenter returns true if this process is an Ops Center (i.e. not
// standalone installer and not a cluster)
func (o *Operator) isOpsCenter() bool {
	return !o.GetConfig().Wizard && !o.GetConfig().Local
}

// isInstaller returns true if this process is running in the installer mode
func (o *Operator) isInstaller() bool {
	return o.GetConfig().Wizard
}

func (o *Operator) backend() storage.Backend {
	return o.GetConfig().Backend
}

func (o *Operator) packages() pack.PackageService {
	return o.GetConfig().Packages
}

func (o *Operator) apps() app.Applications {
	return o.GetConfig().Apps
}

func (o *Operator) users() users.Identity {
	return o.GetConfig().Users
}

func (o *Operator) teleport() ops.TeleportProxyService {
	return o.GetConfig().TeleportProxy
}

func (o *Operator) clock() timetools.TimeProvider {
	return o.GetConfig().Clock
}

func (o *Operator) auditLog() events.IAuditLog {
	return o.GetConfig().AuditLog
}
