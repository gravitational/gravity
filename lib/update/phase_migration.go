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

package update

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
func NewPhaseMigrateLinks(c FSMConfig, plan storage.OperationPlan, phase storage.OperationPhase) (*phaseMigrateLinks, error) {
	return &phaseMigrateLinks{
		FieldLogger: log.WithFields(log.Fields{
			trace.Component: "migrate-links",
		}),
		Backend:     c.Backend,
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
		p.Debugf("cluster %q does not have links to migrate", p.ClusterName)
		return nil
	}
	p.Debugf("found links to migrate: %v, %v", remoteLinks, updateLinks)
	// we only support a simultaneous connection to a single Ops Center but in
	// case some cluster has more than one remote support link, consider only
	// the first one
	if len(remoteLinks) > 1 {
		p.Warnf("only the 1st link will be migrated: %v", remoteLinks)
	}
	remoteLink := remoteLinks[0]
	// find the corresponding update link
	var updateLink *storage.OpsCenterLink
	for _, link := range updateLinks {
		if link.Hostname == remoteLink.Hostname {
			updateLink = &link
			break
		}
	}
	// update link *should* be present but in case of some broken configuration
	// let's tolerate its absense
	if updateLink == nil {
		p.Warnf("could not find update link for remote support link %v: %v %v",
			remoteLink, remoteLinks, updateLinks)
	}
	// now that we've found a remote support link and (possibly) its update
	// counterpart, convert them to a trusted cluster
	trustedCluster, err := storage.NewTrustedClusterFromLinks(remoteLink, updateLink)
	if err != nil {
		return trace.Wrap(err)
	}
	p.Debugf("creating trusted cluster: %s", trustedCluster)
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
			p.Debugf("skipping wizard link: %v", link)
			continue
		}
		switch link.Type {
		case storage.OpsCenterRemoteAccessLink:
			remoteLinks = append(remoteLinks, link)
		case storage.OpsCenterUpdateLink:
			updateLinks = append(updateLinks, link)
		default:
			p.Warnf("unknown link type %q, skipping", link.Type)
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
func NewPhaseUpdateLabels(c FSMConfig, plan storage.OperationPlan, phase storage.OperationPhase) (*phaseUpdateLabels, error) {
	return &phaseUpdateLabels{
		FieldLogger: log.WithFields(log.Fields{
			trace.Component: "update-labels",
		}),
		Servers: plan.Servers,
		Client:  c.Client,
	}, nil
}

// Execute updates the labels on each node
func (p *phaseUpdateLabels) Execute(ctx context.Context) error {
	for _, server := range p.Servers {
		labels := map[string]string{
			defaults.KubernetesAdvertiseIPLabel: server.AdvertiseIP,
		}
		err := libkubernetes.UpdateLabels(ctx, p.Client.Core().Nodes(), server.KubeNodeID(), labels)
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
func NewPhaseMigrateRoles(c FSMConfig, plan storage.OperationPlan, phase storage.OperationPhase) (*phaseMigrateRoles, error) {
	return &phaseMigrateRoles{
		FieldLogger: log.WithFields(log.Fields{
			constants.FieldPhase: phase.ID,
		}),
		Backend:          c.Backend,
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

// migrateRole migrates obsolete "assignKubernetesGroups" rule action
// to the KubeGroups rule property
func (p *phaseMigrateRoles) migrateRole(role teleservices.Role) error {
	var allowKubeGroups []string
	var allowRules []teleservices.Rule
	for _, rule := range role.GetRules(teleservices.Allow) {
		var actions []string
		for _, action := range rule.Actions {
			if !strings.HasPrefix(action, constants.AssignKubernetesGroupsFnName) {
				actions = append(actions, action)
				continue
			}
			groups, err := users.ExtractKubeGroups(action)
			if err != nil {
				return trace.Wrap(err)
			}
			allowKubeGroups = append(allowKubeGroups, groups...)
		}
		rule.Actions = actions
		allowRules = append(allowRules, rule)
	}

	var denyKubeGroups []string
	var denyRules []teleservices.Rule
	for _, rule := range role.GetRules(teleservices.Deny) {
		var actions []string
		for _, action := range rule.Actions {
			if !strings.HasPrefix(action, constants.AssignKubernetesGroupsFnName) {
				actions = append(actions, action)
				continue
			}
			groups, err := users.ExtractKubeGroups(action)
			if err != nil {
				return trace.Wrap(err)
			}
			denyKubeGroups = append(denyKubeGroups, groups...)
		}
		rule.Actions = actions
		denyRules = append(denyRules, rule)
	}

	err := p.backupRole(role)
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

func (p *phaseMigrateRoles) extractKubeGroups(rules []teleservices.Rule) (result []string, err error) {
	for _, rule := range rules {
		for _, action := range rule.Actions {
			if strings.HasPrefix(action, constants.AssignKubernetesGroupsFnName) {
				groups, err := users.ExtractKubeGroups(action)
				if err != nil {
					return nil, trace.Wrap(err)
				}
				result = append(result, groups...)
			}
		}
	}
	return result, nil
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

// getBackupBackend returns a new local backend specific for current operation
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
