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

package phases

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	libkubernetes "github.com/gravitational/gravity/lib/kubernetes"
	"github.com/gravitational/gravity/lib/state"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/storage/keyval"
	"github.com/gravitational/gravity/lib/users"

	teleservices "github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

// phaseMigrateLinks migrates remote Ops Center links to trusted clusters
type phaseMigrateLinks struct {
	// FieldLogger is used for logging
	log.FieldLogger
	// Backend is the cluster backend
	Backend storage.Backend
	// ClusterName is the name of the cluster performing the operation
	ClusterName string
}

// NewPhaseMigrateLinks returns a new links migration executor
func NewPhaseMigrateLinks(plan storage.OperationPlan, backend storage.Backend, logger log.FieldLogger) (fsm.PhaseExecutor, error) {
	return &phaseMigrateLinks{
		FieldLogger: logger,
		Backend:     backend,
		ClusterName: plan.ClusterName,
	}, nil
}

// Execute creates trusted clusters out of Ops Center links
func (p *phaseMigrateLinks) Execute(context.Context) error {
	links, err := p.Backend.GetOpsCenterLinks(p.ClusterName)
	if err != nil {
		return trace.Wrap(err)
	}
	// sort out all links into remote access vs update
	remoteLinks, updateLinks := p.sortOutLinks(links)
	if len(remoteLinks) == 0 {
		p.Debugf("Cluster %q does not have links to migrate.", p.ClusterName)
		return nil
	}
	p.Debugf("Found links to migrate: %v, %v.", remoteLinks, updateLinks)
	// we only support a simultaneous connection to a single Ops Center but in
	// case some cluster has more than one remote support link, consider only
	// the first one
	if len(remoteLinks) > 1 {
		p.Warnf("Only the 1st link will be migrated: %v.", remoteLinks)
	}
	remoteLink := remoteLinks[0]
	// find the corresponding update link
	var updateLink *storage.OpsCenterLink
	for i, link := range updateLinks {
		if link.Hostname == remoteLink.Hostname {
			updateLink = &updateLinks[i]
			break
		}
	}
	// update link *should* be present but in case of some broken configuration
	// let's tolerate its absence
	if updateLink == nil {
		p.Warnf("Could not find update link for remote support link %v: %v %v.",
			remoteLink, remoteLinks, updateLinks)
	}
	// now that we've found a remote support link and (possibly) its update
	// counterpart, convert them to a trusted cluster
	trustedCluster, err := storage.NewTrustedClusterFromLinks(remoteLink, updateLink)
	if err != nil {
		return trace.Wrap(err)
	}
	p.Debugf("Creating trusted cluster: %s.", trustedCluster)
	_, err = p.Backend.UpsertTrustedCluster(trustedCluster)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// Rollback deletes trusted clusters created during phase execution
func (p *phaseMigrateLinks) Rollback(context.Context) error {
	clusters, err := p.Backend.GetTrustedClusters()
	if err != nil {
		return trace.Wrap(err)
	}
	for _, cluster := range clusters {
		err := p.Backend.DeleteTrustedCluster(cluster.GetName())
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// PreCheck is no-op for this phase
func (*phaseMigrateLinks) PreCheck(context.Context) error {
	return nil
}

// PostCheck is no-op for this phase
func (*phaseMigrateLinks) PostCheck(context.Context) error {
	return nil
}

// sortOutLinks sorts the provided links out by type into remote access
// and update links
//
// Wizard links or links with unknown type are skipped.
func (p *phaseMigrateLinks) sortOutLinks(links []storage.OpsCenterLink) (remoteLinks, updateLinks []storage.OpsCenterLink) {
	for _, link := range links {
		if link.Wizard {
			p.Debugf("Skipping wizard link: %v.", link)
			continue
		}
		switch link.Type {
		case storage.OpsCenterRemoteAccessLink:
			remoteLinks = append(remoteLinks, link)
		case storage.OpsCenterUpdateLink:
			updateLinks = append(updateLinks, link)
		default:
			p.Warnf("Unknown link type %q, skipping.", link.Type)
		}
	}
	return remoteLinks, updateLinks
}

// phaseUpdateLabels adds / updates labels as required
type phaseUpdateLabels struct {
	// FieldLogger is used for logging
	log.FieldLogger
	// Servers is the list of servers to update
	Servers []storage.Server
	// Client is an API client to the kubernetes API
	Client *kubernetes.Clientset
}

// NewPhaseUpdateLabels updates labels during an upgrade
func NewPhaseUpdateLabels(plan storage.OperationPlan, client *kubernetes.Clientset, logger log.FieldLogger) (fsm.PhaseExecutor, error) {
	return &phaseUpdateLabels{
		FieldLogger: logger,
		Servers:     plan.Servers,
		Client:      client,
	}, nil
}

// Execute updates the labels on each node
func (p *phaseUpdateLabels) Execute(ctx context.Context) error {
	for _, server := range p.Servers {
		labels := map[string]string{
			defaults.KubernetesAdvertiseIPLabel: server.AdvertiseIP,
			v1.LabelHostname:                    server.KubeNodeID(),
			v1.LabelArchStable:                  "amd64", // Only amd64 is currently supported
			v1.LabelOSStable:                    "linux", // Only linux is currently supported
		}
		p.WithField("labels", labels).Infof("Update labels on %v.", server)
		err := libkubernetes.UpdateLabels(ctx, p.Client.CoreV1().Nodes(), server.KubeNodeID(), labels)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// Rollback does nothing
func (p *phaseUpdateLabels) Rollback(context.Context) error {
	return nil
}

// PreCheck makes sure this phase is being executed on a master node
func (p *phaseUpdateLabels) PreCheck(context.Context) error {
	return trace.Wrap(fsm.CheckMasterServer(p.Servers))
}

// PostCheck is no-op for this phase
func (*phaseUpdateLabels) PostCheck(context.Context) error {
	return nil
}

// phaseMigrateRoles migrates cluster roles to a new format
type phaseMigrateRoles struct {
	// FieldLogger is used for logging
	log.FieldLogger
	// Backend is the cluster backend
	Backend storage.Backend
	// ClusterName is the name of the cluster performing the operation
	ClusterName string
	// OperationID is the current operation ID
	OperationID string
	// getBackupBackend returns backend for backup data, overridden in tests
	getBackupBackend func(operationID string) (storage.Backend, error)
}

// NewPhaseMigrateRoles returns a new roles migration executor
func NewPhaseMigrateRoles(plan storage.OperationPlan, backend storage.Backend, logger log.FieldLogger) (fsm.PhaseExecutor, error) {
	return &phaseMigrateRoles{
		FieldLogger:      logger,
		Backend:          backend,
		ClusterName:      plan.ClusterName,
		OperationID:      plan.OperationID,
		getBackupBackend: getBackupBackend,
	}, nil
}

// Execute migrates cluster roles to a new format
func (p *phaseMigrateRoles) Execute(context.Context) error {
	roles, err := p.Backend.GetRoles()
	if err != nil {
		return trace.Wrap(err)
	}
	for _, role := range roles {
		if needMigrateRole(role) {
			p.Infof("Migrating role %q.", role.GetName())
			if err := p.migrateRole(role); err != nil {
				return trace.Wrap(err)
			}
		}
	}
	return nil
}

// Rollback rolls back role migration changes
func (p *phaseMigrateRoles) Rollback(context.Context) error {
	roles, err := p.Backend.GetRoles()
	if err != nil {
		return trace.Wrap(err)
	}
	for _, role := range roles {
		if len(role.GetKubeGroups(teleservices.Allow))+len(role.GetKubeGroups(teleservices.Deny)) != 0 {
			p.Infof("Rolling back role %q.", role.GetName())
			if err := p.restoreRole(role.GetName()); err != nil {
				return trace.Wrap(err)
			}
		}
	}
	return nil
}

// NeedMigrateRoles returns true if the provided cluster roles need to be
// migrated to a new format
func NeedMigrateRoles(roles []teleservices.Role) bool {
	for _, role := range roles {
		if needMigrateRole(role) {
			return true
		}
	}
	return false
}

// migrateRole migrates obsolete "assignKubernetesGroups" rule action
// to the KubeGroups rule property
func (p *phaseMigrateRoles) migrateRole(role teleservices.Role) error {
	allowKubeGroups, allowRules, err := p.rewriteRole(role, teleservices.Allow)
	if err != nil {
		return trace.Wrap(err)
	}

	denyKubeGroups, denyRules, err := p.rewriteRole(role, teleservices.Deny)
	if err != nil {
		return trace.Wrap(err)
	}

	err = p.backupRole(role)
	if err != nil {
		return trace.Wrap(err)
	}

	role.SetRules(teleservices.Allow, allowRules)
	role.SetKubeGroups(teleservices.Allow, allowKubeGroups)
	role.SetRules(teleservices.Deny, denyRules)
	role.SetKubeGroups(teleservices.Deny, denyKubeGroups)

	err = p.Backend.UpsertRole(role, storage.Forever)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// rewriteRole returns kube groups from the legacy "assignKubernetesGroups"
// role action and the rewritten rules without this action
func (p *phaseMigrateRoles) rewriteRole(role teleservices.Role, condition teleservices.RoleConditionType) (kubeGroups []string, rules []teleservices.Rule, err error) {
	for _, rule := range role.GetRules(condition) {
		var actions []string
		for _, action := range rule.Actions {
			if !strings.HasPrefix(action, constants.AssignKubernetesGroupsFnName) {
				actions = append(actions, action)
				continue
			}
			groups, err := users.ExtractKubeGroups(action)
			if err != nil {
				return nil, nil, trace.Wrap(err)
			}
			kubeGroups = append(kubeGroups, groups...)
		}
		rule.Actions = actions
		rules = append(rules, rule)
	}
	return kubeGroups, rules, nil
}

// backupRole saves the provided role in the operation's backup backend
func (p *phaseMigrateRoles) backupRole(role teleservices.Role) error {
	backend, err := p.getBackupBackend(p.OperationID)
	if err != nil {
		return trace.Wrap(err)
	}
	defer backend.Close()
	err = backend.UpsertRole(role, storage.Forever)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// restoreRole restores role with the provided name from the operation's backup backend
func (p *phaseMigrateRoles) restoreRole(name string) error {
	backend, err := p.getBackupBackend(p.OperationID)
	if err != nil {
		return trace.Wrap(err)
	}
	defer backend.Close()
	role, err := backend.GetRole(name)
	if err != nil {
		return trace.Wrap(err)
	}
	err = p.Backend.UpsertRole(role, storage.Forever)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// PreCheck is no-op for this phase
func (*phaseMigrateRoles) PreCheck(context.Context) error {
	return nil
}

// PostCheck is no-op for this phase
func (*phaseMigrateRoles) PostCheck(context.Context) error {
	return nil
}

// getBackupBackend returns a new local backend specific to the current operation
//
// It is the caller's responsibility to close the backend.
func getBackupBackend(operationID string) (storage.Backend, error) {
	stateDir, err := state.GetStateDir()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = os.MkdirAll(filepath.Join(stateDir, defaults.BackupDir, operationID), defaults.SharedDirMask)
	if err != nil {
		return nil, trace.ConvertSystemError(err)
	}
	backend, err := keyval.NewBolt(keyval.BoltConfig{
		Path: filepath.Join(stateDir, defaults.BackupDir, operationID, "backup.db"),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return backend, nil
}

// needMigrateRole returns true if the provided cluster role needs to be
// migrated to a new format
func needMigrateRole(role teleservices.Role) bool {
	// if the role has "assignKubernetesGroups" action, it needs to
	// be migrated to the new KubeGroups property
	for _, rule := range append(role.GetRules(teleservices.Allow), role.GetRules(teleservices.Deny)...) {
		for _, action := range rule.Actions {
			if strings.HasPrefix(action, constants.AssignKubernetesGroupsFnName) {
				return true
			}
		}
	}
	return false
}
