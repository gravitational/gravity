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

	"github.com/gravitational/gravity/lib/compare"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/utils"

	teleservices "github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	check "gopkg.in/check.v1"
)

// GetClusterAgentCreds returns credentials for cluster agent
//
//  - for regular nodes, this is unprivileged cluster agent that can pull updates
//  - for master nodes, this is privileged agent, that can also do some cluster administration
func GetClusterAgentCreds(backend Backend, clusterName string, needAdmin bool) (*LoginEntry, error) {
	users, err := backend.GetSiteUsers(clusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var user User
	for i := range users {
		if users[i].GetType() == AgentUser {
			hasAdminRole := utils.StringInSlice(users[i].GetRoles(), constants.RoleAdmin)
			if (needAdmin && hasAdminRole) || (!needAdmin && !hasAdminRole) {
				user = users[i]
				break
			}
		}
	}

	if user == nil {
		return nil, trace.NotFound("cluster agent user not found")
	}

	keys, err := backend.GetAPIKeys(user.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(keys) == 0 {
		return nil, trace.NotFound("no API keys found for user %v", user.GetName())
	}

	return &LoginEntry{
		OpsCenterURL: defaults.GravityServiceURL,
		Email:        user.GetName(),
		Password:     keys[0].Token,
	}, nil
}

// GetClusterLoginEntry returns login entry for the local cluster
func GetClusterLoginEntry(backend Backend) (*LoginEntry, error) {
	// first try to find out if we're logged in
	entry, err := backend.GetLoginEntry(defaults.GravityServiceURL)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}

	if entry != nil {
		return entry, nil
	}

	// otherwise search for agent user and return its creds
	cluster, err := backend.GetLocalSite(defaults.SystemAccountID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	entry, err = GetClusterAgentCreds(backend, cluster.Domain, true)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return entry, nil
}

// UpsertCluster creates or updates cluster in the provided backend.
func UpsertCluster(backend Backend, cluster Site) error {
	_, err := backend.UpdateSite(cluster)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	if trace.IsNotFound(err) {
		_, err := backend.CreateSite(cluster)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// UpsertOperation creates or updates operation in the provided backend.
func UpsertOperation(backend Backend, operation SiteOperation) error {
	_, err := backend.UpdateSiteOperation(operation)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	if trace.IsNotFound(err) {
		_, err := backend.CreateSiteOperation(operation)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// GetLastOperation returns the last operation for the local cluster
func GetLastOperation(backend Backend) (*SiteOperation, error) {
	operations, err := GetOperations(backend)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(operations) == 0 {
		return nil, trace.NotFound("no operations found")
	}
	return &(operations[0]), nil
}

// GetOperations returns all operations for the local cluster
// sorted by time in descending order (with most recent operation first)
func GetOperations(backend Backend) ([]SiteOperation, error) {
	cluster, err := backend.GetLocalSite(defaults.SystemAccountID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	operations, err := backend.GetSiteOperations(cluster.Domain)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return operations, nil
}

// GetLastOperationForCluster returns the last operation for the specified cluster
func GetLastOperationForCluster(backend Backend, clusterName string) (*SiteOperation, error) {
	operations, err := GetOperationsForCluster(backend, clusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(operations) == 0 {
		return nil, trace.NotFound("no operations found")
	}
	return &(operations[0]), nil
}

// GetOperationsForCluster returns all operations for the specified cluster
// sorted by time in descending order (with most recent operation first)
func GetOperationsForCluster(backend Backend, clusterName string) ([]SiteOperation, error) {
	operations, err := backend.GetSiteOperations(clusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return operations, nil
}

// GetOperationByID returns the operation with the given ID for the local cluster
func GetOperationByID(backend Backend, operationID string) (*SiteOperation, error) {
	cluster, err := backend.GetLocalSite(defaults.SystemAccountID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	operation, err := backend.GetSiteOperation(cluster.Domain, operationID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return operation, nil
}

// GetLocalServers returns local cluster state servers
func GetLocalServers(backend Backend) ([]Server, error) {
	cluster, err := backend.GetLocalSite(defaults.SystemAccountID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return cluster.ClusterState.Servers, nil
}

// GetLocalPackage returns the local cluster application package
func GetLocalPackage(backend Backend) (*loc.Locator, error) {
	cluster, err := backend.GetLocalSite(defaults.SystemAccountID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	locator, err := loc.NewLocator(cluster.App.Repository, cluster.App.Name, cluster.App.Version)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return locator, nil
}

// GetTrustedCluster returns a trusted cluster representing the Ops Center
// the cluster is connected to, currently only 1 is supported
func GetTrustedCluster(backend Backend) (TrustedCluster, error) {
	clusters, err := backend.GetTrustedClusters()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, cluster := range clusters {
		clusterS, ok := cluster.(TrustedCluster)
		if !ok {
			log.Warnf("Unexpected trusted cluster type: %T.", cluster)
			continue
		}
		if !clusterS.GetWizard() {
			return clusterS, nil
		}
	}
	return nil, trace.NotFound("trusted cluster not found")
}

// GetWizardTrustedCluster returns a trusted cluster representing the wizard
// Ops Center the specified site is connected to
func GetWizardTrustedCluster(backend Backend) (TrustedCluster, error) {
	clusters, err := backend.GetTrustedClusters()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, cluster := range clusters {
		clusterS, ok := cluster.(TrustedCluster)
		if !ok {
			log.Warnf("Unexpected trusted cluster type: %T.", cluster)
			continue
		}
		if clusterS.GetWizard() {
			return clusterS, nil
		}
	}
	return nil, trace.NotFound("wizard trusted cluster not found")
}

// DisableAccess disables access for the remote Teleport cluster (Ops Center
// or installer wizard) with the specified name.
//
// All objects that comprise remote access such as reverse tunnels, trusted
// clusters and certificate authorities are deleted from backend.
//
// If non-0 delay is specified, the access is scheduled to be removed after
// the specified interval.
func DisableAccess(backend Backend, name string, delay time.Duration) error {
	log.Infof("Disabling access for %v with delay %v.", name, delay)
	tunnels, err := backend.GetReverseTunnels()
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	for _, tunnel := range tunnels {
		if tunnel.GetClusterName() == name {
			tunnel.SetTTL(backend, delay)
			if err := backend.UpsertReverseTunnel(tunnel); err != nil {
				return trace.Wrap(err)
			}
		}
	}
	ca, err := backend.GetCertAuthority(teleservices.CertAuthID{
		Type:       teleservices.UserCA,
		DomainName: name,
	}, true)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	// User authority may have already been deleted if one of the calls below
	// failed and this is being retried.
	if trace.IsNotFound(err) {
		log.WithField("name", name).Warn("User authority not found.")
	}
	if ca != nil {
		ca.SetTTL(backend, delay)
		if err := backend.UpsertCertAuthority(ca); err != nil {
			return trace.Wrap(err)
		}
	}
	ca, err = backend.GetCertAuthority(teleservices.CertAuthID{
		Type:       teleservices.HostCA,
		DomainName: name,
	}, true)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	// Host authority may have already been deleted if one of the calls below
	// failed and this is being retried.
	if trace.IsNotFound(err) {
		log.WithField("name", name).Warn("Host authority not found.")
	}
	if ca != nil {
		ca.SetTTL(backend, delay)
		if err := backend.UpsertCertAuthority(ca); err != nil {
			return trace.Wrap(err)
		}
	}
	cluster, err := backend.GetTrustedCluster(name)
	if err != nil {
		return trace.Wrap(err)
	}
	cluster.SetTTL(backend, delay)
	if _, err := backend.UpsertTrustedCluster(cluster); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// GetDNSConfig returns the DNS configuration from the backend using fallback
// if no configuration is available
func GetDNSConfig(backend Backend, fallback DNSConfig) (config *DNSConfig, err error) {
	config, err = backend.GetDNSConfig()
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	if config == nil {
		config = &fallback
	}
	return config, nil
}

// DeepComparePhases compares the actual phase to the expected phase omitting
// some insignificant fields like description or UI step number
func DeepComparePhases(c *check.C, expected, actual OperationPhase) {
	c.Assert(expected.ID, check.Equals, actual.ID,
		check.Commentf("phase ID does not match"))
	c.Assert(expected.Requires, check.DeepEquals, actual.Requires,
		check.Commentf("field Requires on phase %v does not match", expected.ID))
	c.Assert(expected.Parallel, check.Equals, actual.Parallel,
		check.Commentf("field Parallel on phase %v does not match", expected.ID))
	c.Assert(expected.Data, check.DeepEquals, actual.Data,
		check.Commentf("field Data on phase %v does not match: %v", expected.ID,
			compare.Diff(expected.Data, actual.Data)))
	c.Assert(len(expected.Phases), check.Equals, len(actual.Phases),
		check.Commentf("number of subphases on phase %v does not match: %v", expected.ID,
			compare.Diff(expected.Phases, actual.Phases)))
	for i := range expected.Phases {
		DeepComparePhases(c, expected.Phases[i], actual.Phases[i])
	}
}
