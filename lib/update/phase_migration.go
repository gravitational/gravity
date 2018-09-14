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

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	libkubernetes "github.com/gravitational/gravity/lib/kubernetes"
	"github.com/gravitational/gravity/lib/storage"

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
	err = p.Backend.UpsertTrustedCluster(trustedCluster)
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
	// Entry is used for logging
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
