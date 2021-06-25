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

package schema

import (
	"reflect"

	"github.com/gravitational/trace"

	"github.com/ghodss/yaml"
	v1 "k8s.io/api/batch/v1"
)

// Hooks defines all supported application lifecycle hooks
type Hooks struct {
	// ClusterProvision provisions new cluster
	ClusterProvision *Hook `json:"clusterProvision,omitempty"`
	// ClusterDeprovision deprovisions cluster
	ClusterDeprovision *Hook `json:"clusterDeprovision,omitempty"`
	// NodesProvision provisions new nodes to the existing cluster
	NodesProvision *Hook `json:"nodesProvision,omitempty"`
	// NodesDeprovision deprovisions nodes
	NodesDeprovision *Hook `json:"nodesDeprovision,omitempty"`
	// Install installs the application
	Install *Hook `json:"install,omitempty"`
	// Installed is called after the application has been installed
	Installed *Hook `json:"postInstall,omitempty"`
	// Uninstall uninstalls the application
	Uninstall *Hook `json:"uninstall,omitempty"`
	// Uninstalling is called before uninstall
	Uninstalling *Hook `json:"preUninstall,omitempty"`
	// NodeAdding is called before expansion
	NodeAdding *Hook `json:"preNodeAdd,omitempty"`
	// NodeAdded is called after expansion
	NodeAdded *Hook `json:"postNodeAdd,omitempty"`
	// NodeRemoving is called before shrink
	NodeRemoving *Hook `json:"preNodeRemove,omitempty"`
	// NodeRemoved is called after shrink
	NodeRemoved *Hook `json:"postNodeRemove,omitempty"`
	// BeforeUpdate is executed before the application is updated
	BeforeUpdate *Hook `json:"preUpdate,omitempty"`
	// Updating performs application update
	Updating *Hook `json:"update,omitempty"`
	// Updated is called after successful update
	Updated *Hook `json:"postUpdate,omitempty"`
	// Rollback performs application rollback after an unsuccessful update
	Rollback *Hook `json:"rollback,omitempty"`
	// RolledBack is called after successful rollback
	RolledBack *Hook `json:"postRollback,omitempty"`
	// Status is called every minute to check application status
	Status *Hook `json:"status,omitempty"`
	// Info is used to obtain application information
	Info *Hook `json:"info,omitempty"`
	// LicenseUpdated is called after license update
	LicenseUpdated *Hook `json:"licenseUpdated,omitempty"`
	// Start starts the application
	Start *Hook `json:"start,omitempty"`
	// Stop stops the application
	Stop *Hook `json:"stop,omitempty"`
	// Dump is used to retrieve application-specific dumps for debug reports
	Dump *Hook `json:"dump,omitempty"`
	// Backup triggers application data backup
	Backup *Hook `json:"backup,omitempty"`
	// Restore restores application state from a backup
	Restore *Hook `json:"restore,omitempty"`
	// NetworkInstall is a hook for installing a custom overlay network
	NetworkInstall *Hook `json:"networkInstall,omitempty"`
	// NetworkUpdate is a hook for updating a custom overlay network
	NetworkUpdate *Hook `json:"networkUpdate,omitempty"`
	// NetworkRollback is a hook for rolling back a custom overlay network
	NetworkRollback *Hook `json:"networkRollback,omitempty"`
}

// AllHooks returns all non-nil hooks.
//nolint:stylecheck // TODO: receiver names 'in' (in auto-generated code) vs 'h'
func (h Hooks) AllHooks() (all []*Hook) {
	value := reflect.ValueOf(h)
	for i := 0; i < value.NumField(); i++ {
		field := value.Field(i)
		if field.IsNil() {
			continue
		}
		all = append(all, field.Interface().(*Hook))
	}
	return all
}

// Hook defines a hook as either a shell script run in the context
// of the automatically created job or the raw job spec
type Hook struct {
	// Type is a hook type
	Type HookType `json:"type,omitempty"`
	// Job is a URL of (file:// or http://) or a literal value of a k8s job
	Job string `json:"job,omitempty"`
}

// Empty determines if the hook set is empty
func (h Hook) Empty() bool {
	return h.Job == ""
}

// GetJob parses the hook's string with job spec and returns a job object
func (h Hook) GetJob() (*v1.Job, error) {
	if h.Job == "" {
		return nil, trace.NotFound("hook %q does not have job spec", h.Type)
	}
	var job v1.Job
	if err := yaml.Unmarshal([]byte(h.Job), &job); err != nil {
		return nil, trace.Wrap(err)
	}
	return &job, nil
}

// SetJob updates the hook's job spec with the provided job object
func (h *Hook) SetJob(job v1.Job) error {
	bytes, err := yaml.Marshal(job)
	if err != nil {
		return trace.Wrap(err)
	}
	h.Job = string(bytes)
	return nil
}

// HookType defines the application hook type
type HookType string

const (
	// HookClusterProvision used to provision new cluster
	HookClusterProvision HookType = "clusterProvision"
	// HookClusterDeprovision used to deprovision existing cluster
	HookClusterDeprovision HookType = "clusterDeprovision"
	// HookNodesProvision used to provision new nodes in the cluster
	HookNodesProvision HookType = "nodesProvision"
	// HookNodesDeprovision used to deprovision existing nodes
	HookNodesDeprovision HookType = "nodesDeprovision"
	// HookInstall defines the installation hook
	HookInstall HookType = "install"
	// HookInstalled defines the post install hook
	HookInstalled HookType = "postInstall"
	// HookUninstall defines the installation hook
	HookUninstall HookType = "uninstall"
	// HookUninstalling defines the before uninstall hook
	HookUninstalling HookType = "preUninstall"
	// HookBeforeUpdate defines the application hook that runs before the update
	HookBeforeUpdate HookType = "preUpdate"
	// HookUpdate defines the application update hook
	HookUpdate HookType = "update"
	// HookUpdated defines the post application update hook
	HookUpdated HookType = "postUpdate"
	// HookRollback defines the application rollback hook
	HookRollback HookType = "rollback"
	// HookRolledBack defines the application post rollback hook
	HookRolledBack HookType = "postRollback"
	// HookNodeAdding defines the before expand hook
	HookNodeAdding HookType = "preNodeAdd"
	// HookNodeAdded defines the post expand hook
	HookNodeAdded HookType = "postNodeAdd"
	// HookNodeRemoving defines the before shrink hook
	HookNodeRemoving HookType = "preNodeRemove"
	// HookNodeRemoved defines the post shrink hook
	HookNodeRemoved HookType = "postNodeRemove"
	// HookStatus defines the application status hook
	HookStatus HookType = "status"
	// HookInfo defines the application service info hook
	HookInfo HookType = "info"
	// HookLicenseUpdated defines the license update hook
	HookLicenseUpdated HookType = "licenseUpdated"
	// HookStart defines the application start hook
	HookStart HookType = "start"
	// HookStop defines the application stop hook
	HookStop HookType = "stop"
	// HookDump defines the application dump hook
	HookDump HookType = "dump"
	// HookBackup defines a hook to trigger backup of application data.
	// The hook is scheduled on the same node where the backup command runs
	HookBackup HookType = "backup"
	// HookRestore defines a hook to restore application state from a previously
	// created backup.
	// The hook is scheduled on the same node where the restore command runs
	HookRestore HookType = "restore"
	// HookNetworkInstall defines a hook used to install a custom overlay network
	HookNetworkInstall = "networkInstall"
	// HookNetworkUpdate defines a hook to update the overlay network
	HookNetworkUpdate = "networkUpdate"
	// HookNetworkRollback defines a hook to rollback the overlay network
	HookNetworkRollback = "networkRollback"
)

// String implements Stringer
func (h HookType) String() string {
	return string(h)
}

// AllHooks obtains the list of all hook types
func AllHooks() []HookType {
	return []HookType{
		HookClusterProvision,
		HookClusterDeprovision,
		HookNodesProvision,
		HookNodesDeprovision,
		HookInstall,
		HookUninstall,
		HookInstalled,
		HookUninstalling,
		HookBeforeUpdate,
		HookUpdate,
		HookUpdated,
		HookRollback,
		HookRolledBack,
		HookNodeAdding,
		HookNodeAdded,
		HookNodeRemoving,
		HookNodeRemoved,
		HookStatus,
		HookInfo,
		HookLicenseUpdated,
		HookStart,
		HookStop,
		HookDump,
		HookBackup,
		HookRestore,
		HookNetworkInstall,
		HookNetworkUpdate,
		HookNetworkRollback,
	}
}

// HookFromString returns an application hook specified with hookType
func HookFromString(hookType HookType, manifest Manifest) (*Hook, error) {
	if manifest.Hooks == nil {
		return nil, trace.NotFound("%v:%v does not have hooks",
			manifest.Metadata.Name, manifest.Metadata.ResourceVersion)
	}
	var hook *Hook
	switch hookType {
	case HookClusterProvision:
		hook = manifest.Hooks.ClusterProvision
	case HookClusterDeprovision:
		hook = manifest.Hooks.ClusterDeprovision
	case HookNodesProvision:
		hook = manifest.Hooks.NodesProvision
	case HookNodesDeprovision:
		hook = manifest.Hooks.NodesDeprovision
	case HookInstall:
		hook = manifest.Hooks.Install
	case HookInstalled:
		hook = manifest.Hooks.Installed
	case HookUninstall:
		hook = manifest.Hooks.Uninstall
	case HookUninstalling:
		hook = manifest.Hooks.Uninstalling
	case HookBeforeUpdate:
		hook = manifest.Hooks.BeforeUpdate
	case HookUpdate:
		hook = manifest.Hooks.Updating
	case HookUpdated:
		hook = manifest.Hooks.Updated
	case HookRollback:
		hook = manifest.Hooks.Rollback
	case HookRolledBack:
		hook = manifest.Hooks.RolledBack
	case HookNodeAdding:
		hook = manifest.Hooks.NodeAdding
	case HookNodeAdded:
		hook = manifest.Hooks.NodeAdded
	case HookNodeRemoving:
		hook = manifest.Hooks.NodeRemoving
	case HookNodeRemoved:
		hook = manifest.Hooks.NodeRemoved
	case HookStatus:
		hook = manifest.Hooks.Status
	case HookInfo:
		hook = manifest.Hooks.Info
	case HookLicenseUpdated:
		hook = manifest.Hooks.LicenseUpdated
	case HookStart:
		hook = manifest.Hooks.Start
	case HookStop:
		hook = manifest.Hooks.Stop
	case HookDump:
		hook = manifest.Hooks.Dump
	case HookBackup:
		hook = manifest.Hooks.Backup
	case HookRestore:
		hook = manifest.Hooks.Restore
	case HookNetworkInstall:
		hook = manifest.Hooks.NetworkInstall
	case HookNetworkUpdate:
		hook = manifest.Hooks.NetworkUpdate
	case HookNetworkRollback:
		hook = manifest.Hooks.NetworkRollback
	default:
		return nil, trace.BadParameter("unknown hook %q", hookType)
	}
	if hook == nil || hook.Empty() {
		return nil, trace.NotFound("%v:%v does not have %v hook",
			manifest.Metadata.Name, manifest.Metadata.ResourceVersion, hookType)
	}
	return hook, nil
}
