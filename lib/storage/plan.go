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
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	v1 "k8s.io/api/core/v1"
)

// OperationPlan represents a plan of an operation as a collection of phases
type OperationPlan struct {
	// OperationID is the ID of the operation the plan belongs to
	OperationID string `json:"operation_id"`
	// OperationType is the type of the operation the plan belongs to
	OperationType string `json:"operation_type"`
	// AccountID is the ID of the account initiated the operation
	AccountID string `json:"account_id"`
	// ClusterName is the name of the cluster for the operation
	ClusterName string `json:"cluster_name"`
	// Phases is the list of phases the plan consists of
	Phases []OperationPhase `json:"phases"`
	// Servers is the list of all cluster servers
	Servers []Server `json:"servers"`
	// OfflineCoordinator is the server leading/coordinating the upgrade across the cluster, and will have a local copy
	// of completed plan phases if the underlying state sync (etcd) is offline
	OfflineCoordinator *Server `json:"lead_master"`
	// GravityPackage is the gravity package locator to update to
	GravityPackage loc.Locator `json:"gravity_package"`
	// CreatedAt is the plan creation timestamp
	CreatedAt time.Time `json:"created_at"`
	// DNSConfig specifies cluster DNS configuration
	DNSConfig DNSConfig `json:"dns_config"`
}

// Check makes sure operation plan is valid
func (p OperationPlan) Check() error {
	if p.OperationID == "" {
		return trace.BadParameter("missing OperationID")
	}
	if p.OperationType == "" {
		return trace.BadParameter("missing OperationType")
	}
	if p.ClusterName == "" {
		return trace.BadParameter("missing ClusterName")
	}
	return nil
}

// GetLeafPhases flattens the plan and returns all phases that do not have
// any subphases in the order they appear in the plan.
//
// For instance, for the following plan
//
//  * /init
//    * /node-1
//    * /node-2
//  * /checks
//
// it will return ["/init/node-1", "/init/node-2", "/checks"].
func (p *OperationPlan) GetLeafPhases() (result []OperationPhase) {
	for _, phase := range p.Phases {
		result = append(result, getLeafPhases(phase)...)
	}
	return result
}

func getLeafPhases(phase OperationPhase) (result []OperationPhase) {
	if len(phase.Phases) == 0 {
		result = append(result, phase)
	} else {
		for _, sub := range phase.Phases {
			result = append(result, getLeafPhases(sub)...)
		}
	}
	return result
}

// OperationPhase represents a single operation plan phase
type OperationPhase struct {
	// ID is the ID of the phase within operation
	ID string `json:"id"`
	// Executor is function which should execute this phase
	Executor string `json:"executor"`
	// Description is verbose description of the phase
	Description string `json:"description,omitepty" yaml:"description,omitempty"`
	// State is the current phase state
	State string `json:"state,omitempty" yaml:"state,omitempty"`
	// Step maps the phase to its corresponding step on the UI progress screen
	Step int `json:"step"`
	// Phases is the list of sub-phases the phase consists of
	Phases []OperationPhase `json:"phases,omitempty" yaml:"phases,omitempty"`
	// Requires is a list of phase names that need to be
	// completed before this phase can be executed
	Requires []string `json:"requires,omitempty" yaml:"requires,omitempty"`
	// Parallel enables parallel execution of sub-phases
	Parallel bool `json:"parallel"`
	// Updated is the last phase update time
	Updated time.Time `json:"updated,omitempty" yaml:"updated,omitempty"`
	// Data is optional phase-specific data attached to the phase
	Data *OperationPhaseData `json:"data,omitempty" yaml:"data,omitempty"`
	// Error is the error that happened during phase execution
	Error *trace.RawTrace `json:"error,omitempty"`
}

// OperationPhaseData represents data attached to an operation phase
type OperationPhaseData struct {
	// Server is the server the phase operates on
	Server *Server `json:"server,omitempty" yaml:"server,omitempty"`
	// ExecServer is an optional server the phase is supposed to be executed on.
	// If unspecified, the Server is used
	ExecServer *Server `json:"exec_server,omitempty" yaml:"exec_server,omitempty"`
	// Master is the selected master node the phase needs access to
	Master *Server `json:"master,omitempty" yaml:"master,omitempty"`
	// Package is the package locator for the phase, e.g. update package
	Package *loc.Locator `json:"package,omitempty" yaml:"package,omitempty"`
	// Labels can optionally identify the package
	Labels map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	// InstalledPackage references the installed application package
	InstalledPackage *loc.Locator `json:"installed_package,omitempty" yaml:"installed_package,omitempty"`
	// RuntimePackage references the update runtime package
	RuntimePackage *loc.Locator `json:"runtime_package,omitempty" yaml:"runtime_package,omitempty"`
	// ElectionChange describes changes to make to cluster elections
	ElectionChange *ElectionChange `json:"election_status,omitempty" yaml:"election_status,omitempty"`
	// Agent is the credentials of the agent that should be logged in
	Agent *LoginEntry `json:"agent,omitempty" yaml:"agent,omitempty"`
	// License is the cluster license
	License []byte `json:"license,omitempty" yaml:"license,omitempty"`
	// TrustedCluster is the resource data for a trusted cluster representing an Ops Center
	TrustedCluster []byte `json:"trusted_cluster_resource,omitempty" yaml:"trusted_cluster_resource,omitempty"`
	// ServiceUser specifies the optional service user to use as a context
	// for file operations
	ServiceUser *OSUser `json:"service_user,omitempty" yaml:"service_user,omitempty"`
	// Data is arbitrary text data to provide to a phase executor
	Data string `json:"data,omitempty" yaml:"data,omitempty"`
	// GarbageCollect specifies configuration specific to garbage collect operation
	GarbageCollect *GarbageCollectOperationData `json:"garbage_collect,omitempty" yaml:"garbage_collect,omitempty"`
	// Update specifies configuration specific to update operations
	Update *UpdateOperationData `json:"update,omitempty" yaml:"update,omitempty"`
	// Install specifies configuration specific to install operation
	Install *InstallOperationData `json:"install,omitempty" yaml:"install,omitempty"`
}

// ElectionChange describes changes to make to cluster elections
type ElectionChange struct {
	// EnableServers is a list of servers that we should enable elections on
	EnableServers []Server `json:"enable_server,omitempty" yaml:"enable_server,omitempty"`
	// DisableServers is a list of servers that we should disable elections on
	DisableServers []Server `json:"disable_servers,omitempty" yaml:"disable_servers,omitempty"`
}

// GarbageCollectOperationData describes configuration for the garbage collect operation
type GarbageCollectOperationData struct {
	// RemoteApps lists remote applications known to cluster
	RemoteApps []Application `json:"remote_apps,omitempty" yaml:"remote_apps,omitempty"`
}

// UpdateOperationData describes configuration for update operations
type UpdateOperationData struct {
	// Servers lists the cluster servers to use for the configuration update step.
	// The list might be a subset of all cluster servers in case
	// the operation only operates on a specific part
	Servers []UpdateServer `json:"updates,omitempty"`
	// ClusterConfig optionally specifies data specific to cluster configuration operation
	ClusterConfig *ClusterConfigData `json:"cluster_config,omitempty"`
}

// ClusterConfigData describes the configuration specific to cluster configuration update operation
type ClusterConfigData struct {
	// ServiceSuffix specifies the suffix of the temporary DNS services with a ClusterIP
	// from a new service subnet when updating cluster service CIDR
	ServiceSuffix string `json:"service_suffix,omitempty"`
	// ServiceCIDR specifies the service IP range
	ServiceCIDR string `json:"service_cidr,omitempty"`
	// Services lists original service definitions as captured
	// prior to update
	Services []v1.Service `json:"services,omitempty"`
}

// UpdateServer describes an intent to update runtime/teleport configuration
// packages on a specific cluster node
type UpdateServer struct {
	// Server identifies the server for the configuration package update
	Server `json:"server"`
	// Runtime defines the runtime update
	Runtime RuntimePackage `json:"runtime"`
	// Teleport defines the optional teleport update
	Teleport TeleportPackage `json:"teleport"`
}

// RuntimePackage describes the state of the runtime package during update
type RuntimePackage struct {
	// Installed identifies the installed version of the runtime package
	Installed loc.Locator `json:"installed"`
	// SecretsPackage specifies the new secrets package
	SecretsPackage *loc.Locator `json:"secrets_package,omitempty"`
	// Update describes an update to the runtime package
	Update *RuntimeUpdate `json:"update,omitempty"`
}

// RuntimeUpdate describes an update to the runtime package
type RuntimeUpdate struct {
	// Package identifies the package to update to.
	// This can be the same as Installed in which case no update is performed
	Package loc.Locator `json:"package"`
	// ConfigPackage identifies the new configuration package
	ConfigPackage loc.Locator `json:"config_package"`
}

// TeleportPackage describes the state of the teleport package during update
type TeleportPackage struct {
	// Installed identifies the installed version of the teleport package
	Installed loc.Locator `json:"installed"`
	// Update describes an update to the runtime package
	Update *TeleportUpdate `json:"update,omitempty"`
}

// TeleportUpdate describes an update to the teleport package
type TeleportUpdate struct {
	// Package identifies the package to update to.
	// This can be the same as Installed in which case no update is performed
	Package loc.Locator `json:"package"`
	// NodeConfigPackage identifies the new host teleport configuration package.
	// If nil, no changes to configuration package required
	NodeConfigPackage *loc.Locator `json:"node_config_package,omitempty"`
}

// InstallOperationData describes configuration for the install operation
type InstallOperationData struct {
	// Env specifies optional cluster environment variables to add
	Env map[string]string `json:"env,omitempty"`
	// Config specifies optional cluster configuration resource
	Config []byte `json:"config,omitempty"`
	// Resources specifies optional Kubernetes resources to create
	Resources []byte `json:"resources,omitempty"`
	// GravityResources specifies optional Gravity resources to create upon successful installation
	GravityResources []UnknownResource `json:"gravity_resources,omitempty"`
}

// Application describes an application for the package cleaner
type Application struct {
	// Locator references the application package
	loc.Locator
	// Manifest is the application's manifest
	schema.Manifest
}

// PlanChange represents a single operation plan state change
type PlanChange struct {
	// ID is the change ID
	ID string `json:"id"`
	// ClusterName is the name of the cluster for the operation
	ClusterName string `json:"cluster_name"`
	// OperationID is the ID of the operation this change is for
	OperationID string `json:"operation_id"`
	// PhaseID is the ID of the phase the change refers to
	PhaseID string `json:"phase_id"`
	// NewState is the state the phase moved into
	NewState string `json:"new_state"`
	// Created is the change timestamp
	Created time.Time `json:"created"`
	// Error is the error that happened during phase execution
	Error *trace.RawTrace `json:"error"`
}

// PlanChangelog is a list of plan state changes
type PlanChangelog []PlanChange

// Latest returns the most recent plan change entry for the specified phase
func (c PlanChangelog) Latest(phaseID string) *PlanChange {
	var latest *PlanChange
	for i, change := range c {
		if change.PhaseID != phaseID {
			continue
		}
		if latest == nil || change.Created.After(latest.Created) {
			latest = &(c[i])
		}
	}
	return latest
}

// HasSubphases returns true if the phase has 1 or more subphases
func (p OperationPhase) HasSubphases() bool {
	return len(p.Phases) > 0
}

// IsUnstarted returns true if the phase is in "unstarted" state
func (p OperationPhase) IsUnstarted() bool {
	return p.GetState() == OperationPhaseStateUnstarted
}

// IsInProgress returns true if the phase is in "in progress" state
func (p OperationPhase) IsInProgress() bool {
	return p.GetState() == OperationPhaseStateInProgress
}

// IsCompleted returns true if the phase is in "completed" state
func (p OperationPhase) IsCompleted() bool {
	return p.GetState() == OperationPhaseStateCompleted
}

// IsFailed returns true if the phase is in "failed" state
func (p OperationPhase) IsFailed() bool {
	return p.GetState() == OperationPhaseStateFailed
}

// IsRolledBack returns true if the phase is in "rolled back" state
func (p OperationPhase) IsRolledBack() bool {
	return p.GetState() == OperationPhaseStateRolledBack
}

// GetLastUpdateTime returns the phase last updated time
func (p OperationPhase) GetLastUpdateTime() time.Time {
	if len(p.Phases) == 0 {
		return p.Updated
	}
	last := p.Phases[0].GetLastUpdateTime()
	for _, phase := range p.Phases[1:] {
		if phase.GetLastUpdateTime().After(last) {
			last = phase.GetLastUpdateTime()
		}
	}
	return last
}

// GetState returns the phase state based on the states of all its subphases
func (p OperationPhase) GetState() string {
	// if the phase doesn't have subphases, then just return its state from property
	if len(p.Phases) == 0 {
		if p.State == "" {
			return OperationPhaseStateUnstarted
		}
		return p.State
	}
	// otherwise collect states of all subphases
	states := utils.NewStringSet()
	for _, phase := range p.Phases {
		states.Add(phase.GetState())
	}
	// if all subphases are in the same state, then this phase is in this state as well
	if len(states) == 1 {
		return states.Slice()[0]
	}
	// if any of the subphases is failed or rolled back then this phase is failed
	if states.Has(OperationPhaseStateFailed) || states.Has(OperationPhaseStateRolledBack) {
		return OperationPhaseStateFailed
	}
	// otherwise we consider the whole phase to be in progress because it hasn't
	// converged to a single state yet
	return OperationPhaseStateInProgress
}

const (
	// OperationPhaseStateUnstarted means that the phase or all of its subphases haven't started executing yet
	OperationPhaseStateUnstarted = "unstarted"
	// OperationPhaseStateInProgress means that the phase or any of its subphases haven't reached any of the final states yet
	OperationPhaseStateInProgress = "in_progress"
	// OperationPhaseStateCompleted means that the phase or all of its subphases have been completed
	OperationPhaseStateCompleted = "completed"
	// OperationPhaseStateFailed means that the phase or all of its subphases have failed
	OperationPhaseStateFailed = "failed"
	// OperationPhaseStateRolledBack means that the phase or all of its subphases have been rolled back
	OperationPhaseStateRolledBack = "rolled_back"
)

// IsValidOperationPhaseState returns true if the provided phase state is valid.
func IsValidOperationPhaseState(state string) bool {
	return utils.StringInSlice(OperationPhaseStates, state)
}

// OperationPhaseStates is a list of all supported phase states.
var OperationPhaseStates = []string{
	OperationPhaseStateUnstarted,
	OperationPhaseStateInProgress,
	OperationPhaseStateCompleted,
	OperationPhaseStateFailed,
	OperationPhaseStateRolledBack,
}
