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

package ops

import (
	"fmt"
	"strings"
	"time"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/pack/encryptedpack"

	licenseapi "github.com/gravitational/license"
	"github.com/gravitational/trace"
)

// GetInstallOperation returns an install operation for the specified siteKey
func GetInstallOperation(siteKey SiteKey, operator Operator) (op *SiteOperation, progress *ProgressEntry, err error) {
	operations, err := operator.GetSiteOperations(siteKey)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	for _, op := range operations {
		if op.Type != OperationInstall {
			continue
		}
		operation := (*SiteOperation)(&op)
		entry, err := operator.GetSiteOperationProgress(operation.Key())
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		return operation, entry, nil
	}
	return nil, nil, trace.NotFound("no install operation for %v found", siteKey)
}

// GetLastUninstallOperation returns the last uninstall operation for the specified siteKey
func GetLastUninstallOperation(siteKey SiteKey, operator Operator) (op *SiteOperation, progress *ProgressEntry, err error) {
	operations, err := operator.GetSiteOperations(siteKey)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	for _, op := range operations {
		if op.Type != OperationUninstall {
			continue
		}
		operation := (*SiteOperation)(&op)
		entry, err := operator.GetSiteOperationProgress(operation.Key())
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		return operation, entry, nil
	}
	return nil, nil, trace.NotFound("no uninstall operation for %v found", siteKey)
}

// GetCompletedInstallOperation returns a completed install operation for the specified site
func GetCompletedInstallOperation(siteKey SiteKey, operator Operator) (*SiteOperation, error) {
	op, entry, err := GetInstallOperation(siteKey, operator)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if entry.IsCompleted() {
		return op, nil
	}
	return nil, trace.NotFound("no completed install operation for %v found", siteKey)
}

// GetLastOperation returns the most recent operation and its progress for the specified site
func GetLastOperation(siteKey SiteKey, operator Operator) (*SiteOperation, *ProgressEntry, error) {
	operations, err := operator.GetSiteOperations(siteKey)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	if len(operations) == 0 {
		return nil, nil, trace.NotFound("no operations found for %v", siteKey)
	}
	// backend is guaranteed to return operations in the last-to-first order
	lastOperation := (*SiteOperation)(&operations[0])
	progress, err := operator.GetSiteOperationProgress(lastOperation.Key())
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return lastOperation, progress, nil
}

// GetLastUpdateOperation returns the last update operation
//
// If there're no operations or the last operation is not of type 'update', returns NotFound error
func GetLastUpdateOperation(siteKey SiteKey, operator Operator) (*SiteOperation, error) {
	lastOperation, _, err := GetLastOperation(siteKey, operator)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if lastOperation.Type != OperationUpdate {
		return nil, trace.NotFound("the last operation is not update: %v", lastOperation)
	}
	return lastOperation, nil
}

// GetLastShrinkOperation returns the last shrink operation
//
// If there're no operations or the last operation is not of type 'shrink', returns NotFound error
func GetLastShrinkOperation(siteKey SiteKey, operator Operator) (*SiteOperation, error) {
	lastOperation, _, err := GetLastOperation(siteKey, operator)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if lastOperation.Type != OperationShrink {
		return nil, trace.NotFound("the last operation is not shrink: %v", lastOperation)
	}
	return lastOperation, nil
}

// GetOperationWithProgress returns the operation and its progress for the provided operation key
func GetOperationWithProgress(opKey SiteOperationKey, operator Operator) (*SiteOperation, *ProgressEntry, error) {
	operation, err := operator.GetSiteOperation(opKey)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	progress, err := operator.GetSiteOperationProgress(opKey)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return operation, progress, nil
}

// GetActiveOperations returns a list of cluster operations that are currently in progress
func GetActiveOperations(key SiteKey, operator Operator, opType string) ([]SiteOperation, error) {
	all, err := operator.GetSiteOperations(key)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var ongoing []SiteOperation
	for _, op := range all {
		if op.Type != opType {
			continue
		}
		operation := (*SiteOperation)(&op)
		if !operation.IsFinished() {
			ongoing = append(ongoing, *operation)
		}
	}
	if len(ongoing) == 0 {
		return nil, trace.NotFound("no operations in progress for %v", key)
	}
	return ongoing, nil
}

// GetWizardOperation returns the install operation assuming that the
// provided operator talks to an install wizard process
func GetWizardOperation(operator Operator) (*SiteOperation, error) {
	// in wizard mode there is only 1 cluster
	clusters, err := operator.GetSites(defaults.SystemAccountID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(clusters) != 1 {
		return nil, trace.BadParameter("expected 1 cluster, got: %v", clusters)
	}
	op, _, err := GetInstallOperation(clusters[0].Key(), operator)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return op, nil
}

// GetWizardCluster returns the cluster created by wizard install process
func GetWizardCluster(operator Operator) (*Site, error) {
	// in wizard mode there is only 1 cluster
	clusters, err := operator.GetSites(defaults.SystemAccountID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(clusters) != 1 {
		return nil, trace.BadParameter("expected 1 cluster, got: %v", clusters)
	}
	return &clusters[0], nil
}

// CompleteOperation marks the specified operation as completed
func CompleteOperation(key SiteOperationKey, operator Operator) error {
	return operator.SetOperationState(key, SetOperationStateRequest{
		State: OperationStateCompleted,
		Progress: &ProgressEntry{
			SiteDomain:  key.SiteDomain,
			OperationID: key.OperationID,
			Step:        constants.FinalStep,
			Completion:  constants.Completed,
			State:       ProgressStateCompleted,
			Message:     "Operation has completed",
			Created:     time.Now().UTC(),
		},
	})
}

// FailOperation marks the specified operation as failed
func FailOperation(key SiteOperationKey, operator Operator, message string) error {
	if message != "" {
		message = fmt.Sprintf("Operation failure: %v", message)
	} else {
		message = "Operation failure"
	}
	return operator.SetOperationState(key, SetOperationStateRequest{
		State: OperationStateFailed,
		Progress: &ProgressEntry{
			SiteDomain:  key.SiteDomain,
			OperationID: key.OperationID,
			Step:        constants.FinalStep,
			Completion:  constants.Completed,
			State:       ProgressStateFailed,
			Message:     strings.TrimSpace(message),
			Created:     time.Now().UTC(),
		},
	})
}

// VerifyLicense verifies the provided license
func VerifyLicense(packages pack.PackageService, license string) error {
	parsed, err := licenseapi.ParseLicense(license)
	if err != nil {
		return trace.Wrap(err)
	}
	if len(parsed.GetPayload().EncryptionKey) != 0 {
		packages = encryptedpack.New(packages, string(
			parsed.GetPayload().EncryptionKey))
	}
	ca, err := pack.ReadCertificateAuthority(packages)
	if err != nil {
		return trace.Wrap(err)
	}
	return parsed.Verify(ca.CertPEM)
}
