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

package v1

import (
	v1 "k8s.io/api/batch/v1"
)

// Hooks defines a set of application lifecycle hooks in effect
type Hooks struct {
	// Install hook executes an installation script
	Install *HooksBase `json:"install,omitempty"`
	// Installed hook executes script after install
	Installed *HooksBase `json:"post_install,omitempty"`
	// Uninstall hook executes an uninstallation script
	Uninstall *HooksBase `json:"uninstall,omitempty"`
	// Uninstalling hook runs just before the uninstall operation commences
	Uninstalling *HooksBase `json:"pre_uninstall,omitempty"`
	// NodeAdding hook runs before the expand operation starts
	NodeAdding *HooksBase `json:"pre_node_add,omitempty"`
	// NodeAdded hook runs after the expand operation has completed
	NodeAdded *HooksBase `json:"post_node_add,omitempty"`
	// NodeRemoving hook runs before the shrink operation starts
	NodeRemoving *HooksBase `json:"pre_node_remove,omitempty"`
	// NodeRemoved hook runs after the shrink operation has completed
	NodeRemoved *HooksBase `json:"post_node_remove,omitempty"`
	// Updating hook runs before the update operation starts
	Updating *HooksBase `json:"update,omitempty"`
	// Updated hook runs after the update operation has completed
	Updated *HooksBase `json:"post_update,omitempty"`
	// Rollback hook runs when update operation fails
	Rollback *HooksBase `json:"rollback,omitempty"`
	// RolledBack hook runs after rollback has been finished
	RolledBack *HooksBase `json:"post_rollback,omitempty"`
	// Status hook reports the status of the application
	Status *HooksBase `json:"status,omitempty"`
	// Info hook serves as an information endpoint for an installed application.
	// It is used to obtain the details of the application service(s)
	Info *HooksBase `json:"info,omitempty"`
	// LicenseUpdated hook is triggered when a site license has been updated.
	LicenseUpdated *HooksBase `json:"license_updated,omitempty"`
	// Start hook provides a way to start a stopped application.
	Start *HooksBase `json:"start,omitempty"`
	// Stop hook provides a way to stop a running application.
	Stop *HooksBase `json:"stop,omitempty"`
	// Dump hook is used for retrieving application-specific dumps for debug reports
	Dump *HooksBase `json:"dump,omitempty"`
	// HookBackup defines a hook to trigger backup of application data
	Backup *HooksBase `json:"backup,omitempty"`
	// HookRestore defines a hook to restore application state from a previously
	// created backup
	Restore *HooksBase `json:"restore,omitempty"`
}

// AllHooks returns all non-nil hooks.
//nolint:stylecheck // TODO: receiver names 'in' (in auto-generated code) vs 'h'
func (h Hooks) AllHooks() []*HooksBase {
	all := []*HooksBase{h.Install, h.Uninstall, h.Installed, h.Uninstalling, h.NodeAdding, h.NodeAdded,
		h.NodeRemoving, h.NodeRemoved, h.Updating, h.Updated, h.Status, h.Info,
		h.LicenseUpdated, h.Start, h.Stop, h.Dump, h.Backup, h.Restore, h.Rollback,
		h.RolledBack}
	var result []*HooksBase
	for _, hook := range all {
		if hook != nil {
			result = append(result, hook)
		}
	}
	return result
}

// HooksBase defines a hook as either a shell script run in the context
// of the automatically created job or the raw job spec
type HooksBase struct {
	// Type denotes the type of this hook
	Type HookType `json:"type,omitempty"`
	// JobSpec defines the raw kubernetes job resource template executed
	// verbatim
	JobSpec v1.Job `json:"spec,omitempty"`
}

// HookType defines the application hook type
type HookType string

const (
	// HookInstall defines the installation hook
	HookInstall HookType = "install"
	// HookUninstall defines the installation hook
	HookUninstall HookType = "uninstall"
	// HookInstalled defines the post install hook
	HookInstalled HookType = "post_install"
	// HookUninstalling defines the before uninstall hook
	HookUninstalling HookType = "pre_uninstall"
	// HookUpdate defines the application update hook
	HookUpdate HookType = "update"
	// HookUpdated defines the post application update hook
	HookUpdated HookType = "post_update"
	// HookRollback defines the application rollback hook
	HookRollback HookType = "rollback"
	// HookRolledBack defines the application post rollback hook
	HookRolledBack HookType = "post_rollback"
	// HookNodeAdding defines the before expand hook
	HookNodeAdding HookType = "pre_node_add"
	// HookNodeAdded defines the post expand hook
	HookNodeAdded HookType = "post_node_add"
	// HookNodeRemoving defines the before shrink hook
	HookNodeRemoving HookType = "pre_node_remove"
	// HookNodeRemoved defines the post shrink hook
	HookNodeRemoved HookType = "post_node_remove"
	// HookStatus defines the application status hook
	HookStatus HookType = "status"
	// HookInfo defines the application service info hook
	HookInfo HookType = "info"
	// HookLicenseUpdated defines the license update hook
	HookLicenseUpdated HookType = "license_updated"
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
)
