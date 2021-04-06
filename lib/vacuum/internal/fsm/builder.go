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

package fsm

import (
	"fmt"
	"path"

	libfsm "github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/storage"
	libphase "github.com/gravitational/gravity/lib/vacuum/internal/phases"

	"github.com/gravitational/trace"
)

// NewOperationPlan returns a new plan for the specified operation
// and the given set of servers
func NewOperationPlan(operation ops.SiteOperation, servers []storage.Server, remoteApps []storage.Application) (*storage.OperationPlan, error) {
	masters, _ := libfsm.SplitServers(servers)
	if len(masters) == 0 {
		return nil, trace.NotFound("no master servers found in cluster state")
	}

	builder := phaseBuilder{remoteApps: remoteApps}

	registry := *builder.registry(masters)
	packages := *builder.packages(servers)
	journals := *builder.journals(servers)
	phases := phases{registry, packages, journals}

	plan := &storage.OperationPlan{
		OperationID:   operation.ID,
		OperationType: operation.Type,
		AccountID:     operation.AccountID,
		ClusterName:   operation.SiteDomain,
		Phases:        phases.asPhases(),
		Servers:       servers,
	}

	return plan, nil
}

func (r phaseBuilder) registry(masters []storage.Server) *phase {
	root := root(phase{
		ID:          libphase.Registry,
		Description: "Prune unused docker images",
	})

	for i, master := range masters {
		node := r.node(master, root, "Prune unused docker images on node %q")
		node.Data = &storage.OperationPhaseData{
			Server: &masters[i],
		}
		root.AddSequential(node)
	}
	return &root
}

func (r phaseBuilder) packages(servers []storage.Server) *phase {
	root := root(phase{
		ID:          libphase.Packages,
		Description: "Prune unused packages",
	})

	root.AddParallel(r.clusterPackages(root))
	for i, server := range servers {
		node := r.node(server, root, "Prune unused packages on node %q")
		node.Data = &storage.OperationPhaseData{
			Server: &servers[i],
		}
		root.AddParallel(node)
	}
	return &root
}

func (r phaseBuilder) clusterPackages(parent phase) phase {
	return phase{
		ID:          parent.ChildLiteral("cluster"),
		Description: "Prune unused cluster packages",
		Data: &storage.OperationPhaseData{
			GarbageCollect: &storage.GarbageCollectOperationData{
				RemoteApps: r.remoteApps,
			},
		},
	}
}

func (r phaseBuilder) journals(servers []storage.Server) *phase {
	root := root(phase{
		ID:          libphase.Journal,
		Description: "Prune obsolete systemd journal directories",
	})

	for i, server := range servers {
		node := r.node(server, root, "Prune journal directories on node %q")
		node.Data = &storage.OperationPhaseData{
			Server: &servers[i],
		}
		root.AddParallel(node)
	}
	return &root
}

func (r phaseBuilder) node(server storage.Server, parent phase, format string) phase {
	return phase{
		ID:          parent.ChildLiteral(server.Hostname),
		Description: fmt.Sprintf(format, server.Hostname),
	}
}

type phaseBuilder struct {
	remoteApps []storage.Application
}

// AddSequential will append sub-phases which depend one upon another
func (p *phase) AddSequential(sub ...phase) {
	for i := range sub {
		if len(p.Phases) > 0 {
			sub[i].Require(phase(p.Phases[len(p.Phases)-1]))
		}
		p.Phases = append(p.Phases, storage.OperationPhase(sub[i]))
	}
}

// AddParallel will append sub-phases which depend on parent only
func (p *phase) AddParallel(sub ...phase) {
	p.Phases = append(p.Phases, phases(sub).asPhases()...)
}

// Required adds the specified phases reqs as requirements for this phase
func (p *phase) Require(reqs ...phase) *phase {
	for _, req := range reqs {
		p.Requires = append(p.Requires, req.ID)
	}
	return p
}

// ChildLiteral adds the specified sub phase ID as a child of this phase
// and returns the resulting path
func (p *phase) ChildLiteral(sub string) string {
	if p == nil {
		return path.Join("/", sub)
	}
	return path.Join(p.ID, sub)
}

// Root makes the specified phase root
func root(sub phase) phase {
	sub.ID = path.Join("/", sub.ID)
	return sub
}

type phase storage.OperationPhase

func (r phases) asPhases() (result []storage.OperationPhase) {
	result = make([]storage.OperationPhase, 0, len(r))
	for _, phase := range r {
		result = append(result, storage.OperationPhase(phase))
	}
	return result
}

type phases []phase
