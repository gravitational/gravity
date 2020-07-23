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

package opsservice

import (
	"context"
	"sync"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/ops/events"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// operationGroup provides means for synchronizing simultaneous cluster operations
//
// For example it makes sure that only a certain number of concurrent operations is
// permitted, or that a cluster transitions into a proper state in the face of multiple
// operations being launched/finished.
//
// It serves as a sort of a critical section every cluster/operation state transition
// should go through.
type operationGroup struct {
	sync.Mutex
	operator *Operator
	siteKey  ops.SiteKey
}

// swap represents an operation state transition
type swap struct {
	// key is the key of the operation that changes the state
	key ops.SiteOperationKey
	// expectedStates is an optional list of states the operation is expected to be in
	expectedStates []string
	// newOpState is the state to move the operation into
	newOpState string
}

// Check makes sure that the swap object is valid
func (s swap) Check() error {
	if s.newOpState == "" {
		return trace.BadParameter("missing newOpState")
	}
	return nil
}

// createSiteOperation creates the provided operation if the checks allow it to be created
func (g *operationGroup) createSiteOperation(operation ops.SiteOperation) (*ops.SiteOperationKey, error) {
	return g.createSiteOperationWithOptions(operation, createOperationOptions{})
}

type createOperationOptions struct {
	force bool
}

func (g *operationGroup) createSiteOperationWithOptions(operation ops.SiteOperation, options createOperationOptions) (*ops.SiteOperationKey, error) {
	g.Lock()
	defer g.Unlock()

	err := g.canCreateOperation(operation, options.force)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	site, err := g.operator.openSite(g.siteKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	op, err := site.createSiteOperation(&operation)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	state, err := operation.ClusterState()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = site.setSiteState(state)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = g.emitAuditEvent(context.TODO(), *op)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	key := op.Key()
	return &key, nil
}

func (g *operationGroup) emitAuditEvent(ctx context.Context, operation ops.SiteOperation) error {
	// Audit events for the following operations are emitted by their agents.
	switch operation.Type {
	case ops.OperationInstall, ops.OperationUpdate, ops.OperationUpdateConfig, ops.OperationUpdateRuntimeEnviron:
		return nil
	}
	// Expand operation start event is emitted by the joining agent.
	if operation.Type == ops.OperationExpand && !operation.IsFinished() {
		return nil
	}
	return events.EmitForOperation(ctx, g.operator, operation)
}

// canCreateOperation checks if the provided operation is allowed to be created
//
// In case of failed checks returns trace.CompareFailed error to indicate that
// the cluster is not in the appropriate state.
func (g *operationGroup) canCreateOperation(operation ops.SiteOperation, force bool) error {
	cluster, err := g.operator.GetSite(g.siteKey)
	if err != nil {
		return trace.Wrap(err)
	}

	switch operation.Type {
	case ops.OperationInstall, ops.OperationUninstall, ops.OperationReconfigure:
		// no special checks for install/uninstall are needed
	case ops.OperationExpand:
		// expand has to undergo some checks
		err := g.canCreateExpandOperation(*cluster, operation)
		if err != nil {
			return trace.Wrap(err)
		}
	case ops.OperationUpdate:
		err := g.canCreateUpgradeOperation(*cluster, operation, force)
		if err != nil {
			return trace.Wrap(err)
		}
	case ops.OperationShrink, ops.OperationGarbageCollect, ops.OperationUpdateRuntimeEnviron:
		// shrink, gc and updating environment are allowed for degraded clusters
		switch cluster.State {
		case ops.SiteStateActive, ops.SiteStateDegraded:
		default:
			return trace.CompareFailed("the cluster is %v", cluster.State)
		}
	default:
		// other operation can be performed by active clusters only
		switch cluster.State {
		case ops.SiteStateActive:
		default:
			return trace.CompareFailed("the cluster is %v", cluster.State)
		}
	}

	return nil
}

func (g *operationGroup) canCreateUpgradeOperation(cluster ops.Site, operation ops.SiteOperation, force bool) error {
	// Upgrade is only allowed for active and healthy clusters.
	if cluster.State != ops.SiteStateActive {
		return trace.CompareFailed(
			`Upgrade operation can only be triggered for active clusters. This cluster is currently %v.
Use "gravity status" to see the cluster status and make sure that the cluster is active and healthy before retrying.`, cluster.State)
	}
	// Even if the cluster is active, run a few checks on the last upgrade
	// operation to make sure it was completed/rolled back properly, to
	// protect against cases when cluster state is force-reset midway.
	lastUpgrade, err := ops.GetLastUpgradeOperation(cluster.Key(), g.operator)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	if trace.IsNotFound(err) {
		return nil
	}
	err = g.checkLastOperation(*lastUpgrade)
	if err != nil {
		if force { // Last operation checks didn't pass but force flag was provided.
			log.WithError(err).Warn("Force-creating upgrade operation.")
			return nil
		}
		return trace.Wrap(err)
	}
	// All last operation checks passed.
	return nil
}

func (g *operationGroup) checkLastOperation(operation ops.SiteOperation) error {
	// Check if the operation is in a terminal state.
	if !operation.IsFinished() {
		return trace.CompareFailed(
			`The last %v operation (%v) is still in progress.
Use "gravity plan" to view the operation plan, and either resume or rollback it before attempting to start another operation.
You can provide --force flag to override this check.`,
			operation.TypeString(),
			operation.ID)
	}
	// Last upgrade completed fine.
	if operation.IsCompleted() {
		return nil
	}
	// Otherwise the operation is failed - check its plan and make sure it was
	// fully rolled back.
	plan, err := g.operator.GetOperationPlan(operation.Key())
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	if trace.IsNotFound(err) {
		return nil
	}
	if !fsm.IsRolledBack(plan) {
		return trace.CompareFailed(
			`The last %v operation (%v) is in a %v state but its operation plan isn't fully rolled back.
Use "gravity plan" to view the operation plan, and either resume it or roll it back before attempting to start another operation.
You can provide --force flag to override this check.`,
			operation.TypeString(),
			operation.ID,
			operation.State)
	}
	return nil
}

// canCreateExpandOperation runs expand-specific checks
//
// In case of failed checks returns trace.CompareFailed error to indicate that
// the cluster is not in the appropriate state.
func (g *operationGroup) canCreateExpandOperation(site ops.Site, operation ops.SiteOperation) error {
	if site.State == ops.SiteStateActive {
		return nil
	}

	operations, err := ops.GetActiveOperationsByType(g.siteKey, g.operator, ops.OperationExpand)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}

	// cluster is not active, but there are no expand operations so there is either
	// other type of operation in progress, or it's degraded
	if len(operations) == 0 {
		if site.State == ops.SiteStateDegraded {
			return utils.ClusterDegradedError{}
		}
		return trace.CompareFailed("cannot expand %v cluster", site.State)
	}

	if len(operations) >= defaults.MaxExpandConcurrency {
		return trace.CompareFailed("at most %v nodes can be joining simultaneously",
			defaults.MaxExpandConcurrency)
	}

	// if an expand operation that's adding master node is currently running,
	// it has to finish before another expand can be started
	for _, op := range operations {
		for _, node := range op.Servers {
			if node.ClusterRole == string(schema.ServiceRoleMaster) {
				return trace.CompareFailed("can't launch another expand while master node %v is joining",
					node.AdvertiseIP)
			}
		}
	}

	// now check the opposite use-case: if we're about to add a master,
	// it has to be the only operation running
	for _, profile := range operation.InstallExpand.Profiles {
		switch profile.ServiceRole {
		case string(schema.ServiceRoleMaster):
			// the joining node wants to be a master
			return trace.CompareFailed("can't join master node while another node is joining")
		case "":
			// cluster role was not set explicitly on the joining node, so
			// it will be auto-set to master if the max number of masters
			// haven't been reached yet
			if len(site.Masters()) < defaults.MaxMasterNodes {
				return trace.CompareFailed("can't join master node while another node is joining")
			}
		}
	}

	// if we've reached here, we're about to join a regular node, there are
	// other regular nodes joining right now too and the total number of join
	// operations is under the maximum
	return nil
}

// compareAndSwapOperationState changes the operation state according to the provided spec
//
// In the case the operation moves to its final state, it also updates the cluster
// state accordingly (e.g. moves the cluster from 'expanding' to 'active' if no other
// expand operations are running).
func (g *operationGroup) compareAndSwapOperationState(swap swap) (*ops.SiteOperation, error) {
	g.Lock()
	defer g.Unlock()

	err := swap.Check()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	operation, err := g.operator.GetSiteOperation(swap.key)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(swap.expectedStates) != 0 && !utils.StringInSlice(swap.expectedStates, operation.State) {
		return nil, trace.CompareFailed(
			"operation %v is not in %v", operation, swap.expectedStates)
	}

	site, err := g.operator.openSite(g.siteKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	operation, err = site.setOperationState(operation.Key(), swap.newOpState)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// if we've just moved the operation to one of the final states (completed/failed),
	// see if we also need to update the site state
	if operation.IsFinished() {
		err = g.emitAuditEvent(context.TODO(), *operation)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		err = g.onSiteOperationComplete(swap.key)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	return operation, nil
}

// onSiteOperationComplete is called upon operation completion and possibly updates
// the cluster state
func (g *operationGroup) onSiteOperationComplete(key ops.SiteOperationKey) error {
	operation, err := g.operator.GetSiteOperation(key)
	if err != nil {
		return trace.Wrap(err)
	}

	operations, err := ops.GetActiveOperationsByType(g.siteKey, g.operator, operation.Type)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}

	if len(operations) > 0 {
		log.Debugf("%v more %q operation(-s) in progress for %v: %#v %#v",
			len(operations), operation.Type, key.SiteDomain, key, operations)
		return nil
	}

	site, err := g.operator.openSite(g.siteKey)
	if err != nil {
		return trace.Wrap(err)
	}

	state, err := operation.ClusterState()
	if err != nil {
		return trace.Wrap(err)
	}

	err = site.setSiteState(state)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// addClusterStateServers adds the provided servers to the cluster state
func (g *operationGroup) addClusterStateServers(servers []storage.Server) error {
	g.Lock()
	defer g.Unlock()

	site, err := g.operator.backend().GetSite(g.siteKey.SiteDomain)
	if err != nil {
		return trace.Wrap(err)
	}

	// add provided servers one-by-one making sure they're not already present
	for _, server := range servers {
		if site.ClusterState.HasServer(server.Hostname) {
			return trace.AlreadyExists(
				"node %[1]v is already registered, remove it using 'gravity remove %[1]v --force' first",
				server.Hostname)
		}
		site.ClusterState.Servers = append(site.ClusterState.Servers, server)
	}

	if _, err = g.operator.backend().UpdateSite(*site); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// removeClusterStateServers removes servers with the specified hostnames from the cluster state
func (g *operationGroup) removeClusterStateServers(hostnames []string) error {
	g.Lock()
	defer g.Unlock()

	site, err := g.operator.backend().GetSite(g.siteKey.SiteDomain)
	if err != nil {
		return trace.Wrap(err)
	}

	var servers []storage.Server
	for _, server := range site.ClusterState.Servers {
		if !utils.StringInSlice(hostnames, server.Hostname) {
			servers = append(servers, server)
		}
	}

	site.ClusterState.Servers = servers
	if _, err = g.operator.backend().UpdateSite(*site); err != nil {
		return trace.Wrap(err)
	}

	return nil
}
