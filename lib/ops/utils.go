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
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/pack/encryptedpack"
	"github.com/gravitational/gravity/lib/storage"

	licenseapi "github.com/gravitational/license"
	"github.com/gravitational/trace"
)

// GetInstallOperation returns an install operation for the specified siteKey
func GetInstallOperation(siteKey SiteKey, operator Operator) (op *SiteOperation, progress *ProgressEntry, err error) {
	operations, err := operator.GetSiteOperations(siteKey, OperationsFilter{
		Type: OperationInstall,
	})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	if len(operations) == 0 {
		return nil, nil, trace.NotFound("no install operation found for %v", siteKey)
	}

	op = (*SiteOperation)(&operations[0])
	progress, err = operator.GetSiteOperationProgress(op.Key())
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return op, progress, nil
}

// GetLastUninstallOperation returns the last uninstall operation for the specified siteKey
func GetLastUninstallOperation(siteKey SiteKey, operator Operator) (op *SiteOperation, progress *ProgressEntry, err error) {
	operations, err := operator.GetSiteOperations(siteKey, OperationsFilter{
		Type: OperationInstall,
		Last: true,
	})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	if len(operations) == 0 {
		return nil, nil, trace.NotFound("no uninstall operation found for %v", siteKey)
	}

	// backend is guaranteed to return operations in the last-to-first order
	op = (*SiteOperation)(&operations[0])
	progress, err = operator.GetSiteOperationProgress(op.Key())
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return op, progress, nil
}

// GetLastCompletedUpdateOperation returns the last completed update operation
func GetLastCompletedUpdateOperation(siteKey SiteKey, operator Operator) (op *SiteOperation, err error) {
	operations, err := operator.GetSiteOperations(siteKey, OperationsFilter{
		Type:     OperationUpdate,
		Last:     true,
		Complete: true,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(operations) == 0 {
		return nil, trace.NotFound("no completed update operation found for %v", siteKey)
	}

	return (*SiteOperation)(&operations[0]), nil
}

// GetCompletedInstallOperation returns a completed install operation for the specified site
func GetCompletedInstallOperation(siteKey SiteKey, operator Operator) (*SiteOperation, error) {
	operations, err := operator.GetSiteOperations(siteKey, OperationsFilter{
		Type:     OperationInstall,
		Last:     true,
		Complete: true,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(operations) == 0 {
		return nil, trace.NotFound("no completed install operation found for %v", siteKey)
	}

	return (*SiteOperation)(&operations[0]), nil
}

// GetLastOperation returns the most recent operation and its progress for the specified site
func GetLastOperation(siteKey SiteKey, operator Operator) (op *SiteOperation, progress *ProgressEntry, err error) {
	operations, err := operator.GetSiteOperations(siteKey, OperationsFilter{
		Last: true,
	})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	if len(operations) == 0 {
		return nil, nil, trace.NotFound("no operation found for %v", siteKey)
	}

	// backend is guaranteed to return operations in the last-to-first order
	op = (*SiteOperation)(&operations[0])
	progress, err = operator.GetSiteOperationProgress(op.Key())
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return op, progress, nil
}

// GetLastCompletedOperations returns the cluster's last completed operation
func GetLastCompletedOperation(siteKey SiteKey, operator Operator) (op *SiteOperation, progress *ProgressEntry, err error) {
	operations, err := operator.GetSiteOperations(siteKey, OperationsFilter{
		Last:     true,
		Finished: true,
	})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	if len(operations) == 0 {
		return nil, nil, trace.NotFound("no completed operation found for %v", siteKey)
	}

	// backend is guaranteed to return operations in the last-to-first order
	op = (*SiteOperation)(&operations[0])
	progress, err = operator.GetSiteOperationProgress(op.Key())
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return op, progress, nil
}

// GetLastUpgradeOperation returns the most recent upgrade operation or NotFound.
func GetLastUpgradeOperation(siteKey SiteKey, operator Operator) (*SiteOperation, error) {
	operations, err := operator.GetSiteOperations(siteKey, OperationsFilter{
		Type: OperationUpdate,
		Last: true,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(operations) == 0 {
		return nil, trace.NotFound("no upgrade operation found for %v", siteKey)
	}

	return (*SiteOperation)(&operations[0]), nil
}

// GetLastShrinkOperation returns the last shrink operation
//
// If there're no operations or the last operation is not of type 'shrink', returns NotFound error
func GetLastShrinkOperation(siteKey SiteKey, operator Operator) (*SiteOperation, error) {
	operations, err := operator.GetSiteOperations(siteKey, OperationsFilter{
		Type: OperationShrink,
		Last: true,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(operations) == 0 {
		return nil, trace.NotFound("no shrink operation found for %v", siteKey)
	}

	return (*SiteOperation)(&operations[0]), nil
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

// GetActiveOperations returns a list of currently active cluster operations
func GetActiveOperations(siteKey SiteKey, operator Operator) (active []SiteOperation, err error) {
	operations, err := operator.GetSiteOperations(siteKey, OperationsFilter{
		Active: true,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(operations) == 0 {
		return nil, trace.NotFound("no active operation found for %v", siteKey)
	}

	for _, op := range operations {
		active = append(active, SiteOperation(op))
	}

	return active, nil
}

// GetActiveOperationsByType returns a list of cluster operations of the specified
// type that are currently in progress
func GetActiveOperationsByType(siteKey SiteKey, operator Operator, opType string) (active []SiteOperation, err error) {
	operations, err := operator.GetSiteOperations(siteKey, OperationsFilter{
		Type:   opType,
		Active: true,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(operations) == 0 {
		return nil, trace.NotFound("no shrink operation found for %v", siteKey)
	}

	for _, op := range operations {
		active = append(active, SiteOperation(op))
	}

	return active, nil
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

// FailOperationAndResetCluster completes the specified operation and resets
// cluster state to active
func FailOperationAndResetCluster(ctx context.Context, key SiteOperationKey, operator Operator, message string) error {
	err := FailOperation(ctx, key, operator, message)
	if err != nil {
		return trace.Wrap(err)
	}
	err = operator.ActivateSite(ActivateSiteRequest{
		AccountID:  key.AccountID,
		SiteDomain: key.SiteDomain,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// CompleteOperation marks the specified operation as completed
func CompleteOperation(ctx context.Context, key SiteOperationKey, operator OperationStateSetter) error {
	return operator.SetOperationState(ctx, key, SetOperationStateRequest{
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
func FailOperation(ctx context.Context, key SiteOperationKey, operator OperationStateSetter, message string) error {
	if message != "" {
		message = fmt.Sprintf("Operation failure: %v", message)
	} else {
		message = "Operation failure"
	}
	return operator.SetOperationState(ctx, key, SetOperationStateRequest{
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

// OperationStateSetter defines an interface to set/update operation state
type OperationStateSetter interface {
	// SetOperationState updates state of the operation
	// specified with given operation key
	SetOperationState(context.Context, SiteOperationKey, SetOperationStateRequest) error
}

// SetOperationState implements the OperationStateSetter by invoking this handler
func (r OperationStateFunc) SetOperationState(ctx context.Context, key SiteOperationKey, req SetOperationStateRequest) error {
	return r(ctx, key, req)
}

// OperationStateFunc is a function handler for setting the operation state
type OperationStateFunc func(context.Context, SiteOperationKey, SetOperationStateRequest) error

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

// GetExpandOperation returns the first available expand operation from
// the provided backend
func GetExpandOperation(backend storage.Backend) (*SiteOperation, error) {
	cluster, err := backend.GetLocalSite(defaults.SystemAccountID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	operations, err := backend.GetSiteOperations(cluster.Domain)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, operation := range operations {
		if operation.Type == OperationExpand {
			return (*SiteOperation)(&operation), nil
		}
	}
	return nil, trace.NotFound("expand operation not found")
}

// UpsertSystemAccount creates a new system account if one has not been created.
// Returns the system account
func UpsertSystemAccount(operator Operator) (*Account, error) {
	accounts, err := operator.GetAccounts()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for i := range accounts {
		if accounts[i].Org == defaults.SystemAccountOrg {
			return &accounts[i], nil
		}
	}
	account, err := operator.CreateAccount(NewAccountRequest{
		ID:  defaults.SystemAccountID,
		Org: defaults.SystemAccountOrg,
	})
	return account, trace.Wrap(err)
}

// MatchByType returns an OperationMatcher to match operations by type
func MatchByType(opType string) OperationMatcher {
	return func(op SiteOperation) bool {
		return op.Type == opType
	}
}

// OperationMatcher is a function type that matches the given operation
type OperationMatcher func(SiteOperation) bool
