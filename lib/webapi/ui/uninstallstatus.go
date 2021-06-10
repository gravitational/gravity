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

package ui

import (
	"github.com/gravitational/gravity/lib/ops"

	"github.com/gravitational/trace"
)

// UninstallStatus describes the status of uninstall operation
type UninstallStatus struct {
	// ClusterName is cluster name
	ClusterName string `json:"siteDomain"`
	// State is a state of uninstall operation
	State string `json:"state"`
	// Step is a step of uninstall operation
	Step int `json:"step"`
	// Message is a message of uninstall operation
	Message string `json:"message"`
	// OperationID is ID of uninstall operation
	OperationID string `json:"operationId"`
}

// GetUninstallStatus returns a status of uninstall operation. Since 'not-found' cluster indicates that
// a cluster has been successfully deleted, it's to be treated as such.
func GetUninstallStatus(accountID string, clusterName string, operator ops.Operator) (*UninstallStatus, error) {
	uninstallStatus := &UninstallStatus{
		ClusterName: clusterName,
		State:       ops.OperationStateCompleted,
	}

	siteKey := ops.SiteKey{
		AccountID:  accountID,
		SiteDomain: clusterName,
	}

	_, progressEntry, err := ops.GetLastUninstallOperation(siteKey, operator)
	if err != nil && trace.IsNotFound(err) {
		// not found indicates that uninstall operation has been completed
		return uninstallStatus, nil
	}

	if err != nil {
		return nil, trace.Wrap(err)
	}

	if progressEntry != nil {
		uninstallStatus.State = progressEntry.State
		uninstallStatus.Message = progressEntry.Message
		uninstallStatus.Step = progressEntry.Step
		uninstallStatus.OperationID = progressEntry.OperationID
	}

	return uninstallStatus, nil
}
