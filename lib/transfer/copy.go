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

package transfer

import (
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/storage"

	teleservices "github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
)

// copySite copies site-related information from one backend to another
func copySite(site *storage.Site, dst storage.Backend, src ExportBackend, clusters []storage.TrustedCluster) error {
	// this site will become local for the target host
	site.Local = true

	account, err := src.GetAccount(site.AccountID)
	if err != nil {
		return trace.Wrap(err)
	}

	if _, err := dst.CreateAccount(*account); err != nil && !trace.IsAlreadyExists(err) {
		return trace.Wrap(err)
	}

	users, err := src.GetSiteUsers(site.Domain)
	if err != nil {
		return trace.Wrap(err)
	}

	a := site.App
	pkg, err := src.GetPackage(a.Repository, a.Name, a.Version)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = dst.CreateRepository(storage.NewRepository(pkg.Repository))
	if err != nil && !trace.IsAlreadyExists(err) {
		return trace.Wrap(err)
	}

	_, err = dst.CreatePackage(*pkg)
	if err != nil && !trace.IsAlreadyExists(err) {
		return trace.Wrap(err)
	}

	site.State = ops.SiteStateActive
	if _, err := dst.CreateSite(*site); err != nil && !trace.IsAlreadyExists(err) {
		return trace.Wrap(err)
	}

	// if clusters are supplied, they will be used to populate dst
	// backend, otherwise they will be taken from the source backend
	var trustedClusters []teleservices.TrustedCluster
	if len(clusters) == 0 {
		trustedClusters, err = src.GetTrustedClusters()
		if err != nil {
			return trace.Wrap(err)
		}
	} else {
		for _, cluster := range clusters {
			trustedClusters = append(trustedClusters, cluster)
		}
	}
	for _, cluster := range trustedClusters {
		if _, err := dst.UpsertTrustedCluster(cluster); err != nil {
			return trace.Wrap(err)
		}
	}

	for _, user := range users {
		// Only copy agent users because sites will have their local user
		// hierarchy, although agent users are robots used for
		// updates/install/pulling packages
		if user.GetType() != storage.AgentUser {
			continue
		}

		if _, err := dst.CreateUser(user); err != nil && !trace.IsAlreadyExists(err) {
			return trace.Wrap(err)
		}

		keys, err := src.GetAPIKeys(user.GetName())
		if err != nil {
			return trace.Wrap(err)
		}

		for _, key := range keys {
			if _, err := dst.CreateAPIKey(key); err != nil && !trace.IsAlreadyExists(err) {
				return trace.Wrap(err)
			}
		}

		roles, err := src.GetUserRoles(user.GetName())
		if err != nil {
			return trace.Wrap(err)
		}

		for _, r := range roles {
			if err := dst.UpsertRole(r, storage.Forever); err != nil {
				return trace.Wrap(err)
			}
		}
	}

	tokens, err := src.GetSiteProvisioningTokens(site.Domain)
	if err != nil {
		return trace.Wrap(err)
	}

	for _, token := range tokens {
		_, err = dst.CreateProvisioningToken(token)
		if err != nil && !trace.IsAlreadyExists(err) {
			return trace.Wrap(err)
		}
	}

	operations, err := src.GetSiteOperations(site.Domain)
	if err != nil {
		return trace.Wrap(err)
	}

	for _, op := range operations {
		if op.Type != ops.OperationInstall {
			continue
		}
		op.State = ops.OperationStateCompleted
		_, err := dst.CreateSiteOperation(op)
		if err != nil && !trace.IsAlreadyExists(err) {
			return trace.Wrap(err)
		}
		entry, err := src.GetLastProgressEntry(site.Domain, op.ID)
		if err != nil {
			return trace.Wrap(err)
		}
		entry.Completion = constants.Completed
		entry.Step = constants.FinalStep
		entry.State = ops.ProgressStateCompleted
		entry.Message = "Operation has completed"
		_, err = dst.CreateProgressEntry(*entry)
		if err != nil && !trace.IsAlreadyExists(err) {
			return trace.Wrap(err)
		}
		plan, err := src.GetOperationPlan(site.Domain, op.ID)
		if err != nil {
			return trace.Wrap(err)
		}
		_, err = dst.CreateOperationPlan(fsm.MarkCompleted(*plan))
		if err != nil && !trace.IsAlreadyExists(err) {
			return trace.Wrap(err)
		}
	}

	return trace.Wrap(err)
}

// ExportBackend exposes a subset of storage.Backend to perform site export.
//
// This interface defines a facade to provide alternate implementations of
// (some of) the Backend methods used in export.
type ExportBackend interface {
	GetAccount(accountID string) (*storage.Account, error)
	GetSiteUsers(domain string) ([]storage.User, error)
	GetPackage(repository, packageName, packageVersion string) (*storage.Package, error)
	GetAPIKeys(email string) ([]storage.APIKey, error)
	GetUserRoles(email string) ([]teleservices.Role, error)
	GetSiteOperations(domain string) ([]storage.SiteOperation, error)
	GetTrustedClusters() ([]teleservices.TrustedCluster, error)
	GetSiteProvisioningTokens(domain string) ([]storage.ProvisioningToken, error)
	GetLastProgressEntry(domain, operationID string) (*storage.ProgressEntry, error)
	GetOperationPlan(domain, operationID string) (*storage.OperationPlan, error)
}
