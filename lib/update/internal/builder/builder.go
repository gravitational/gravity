/*
Copyright 2019 Gravitational, Inc.

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

// This file implements richer support for working with operation phases
package builder

import (
	"path"

	"github.com/gravitational/gravity/lib/storage"
)


// DependencyForServer looks up a dependency in the list of sub-phases of the give phase
// that references the specified server and returns a reference to it.
// If no server has been found, it retruns the reference to the phase itself
func DependencyForServer(phase *Phase, server storage.Server) *Phase {
	for _, phase := range phase.phases {
		if phase.p.Data.Server.AdvertiseIP == server.AdvertiseIP {
			return phase
		}
	}
	// FIXME: check this use-case
	return nil
}

// ResolveInline embeds the phases of the specified root without the root itself.
// Returns a new plan using the specified plan template
func ResolveInline(root *Phase, planTemplate storage.OperationPlan) *storage.OperationPlan {
	return Resolve(root.phases, planTemplate)
}

// Resolve resolves dependencies between phases and renders phase IDs as absolute.
// Returns a new plan using the specified plan template
func Resolve(phases []*Phase, planTemplate storage.OperationPlan) *storage.OperationPlan {
	resolveIDs(nil, phases)
	resolveRequirements(nil, phases)
	result := make([]storage.OperationPhase, len(phases))
	render(result, phases)
	plan := planTemplate
	plan.Phases = result
	return &plan
}

// New returns a new phase using the specified phase as a template
func New(phase storage.OperationPhase) *Phase {
	return &Phase{
		p: phase,
	}
}

// HasSubphases returns true if this phase has sub-phases
func (p *Phase) HasSubphases() bool {
	return len(p.phases) != 0
}

// AddSequential will append sub-phases which depend one upon another
func (p *Phase) AddSequential(subs ...*Phase) {
	for i := range subs {
		if len(p.phases) != 0 {
			subs[i].Require(p.phases[len(p.phases)-1])
		}
		p.phases = append(p.phases, subs[i])
	}
}

// AddParallel will append sub-phases which depend on parent only
func (p *Phase) AddParallel(subs ...*Phase) {
	p.phases = append(p.phases, subs...)
}

// AddParallelRaw will append sub-phases which depend on parent only
func (p *Phase) AddParallelRaw(subs ...storage.OperationPhase) {
	for _, sub := range subs {
		phase := New(sub)
		p.phases = append(p.phases, phase)
	}
}

// AddSequentialRaw will append sub-phases which depend one upon another
func (p *Phase) AddSequentialRaw(subs ...storage.OperationPhase) {
	for _, sub := range subs {
		phase := New(sub)
		if len(p.phases) != 0 {
			phase.Require(p.phases[len(p.phases)-1])
		}
		p.phases = append(p.phases, phase)
	}
}

// AddWithDependency sets phase as explicit dependency on subs
func (p *Phase) AddWithDependency(dep *Phase, subs ...*Phase) {
	for i := range subs {
		subs[i].Require(dep)
		p.phases = append(p.phases, subs[i])
	}
}

// Required adds the specified phases reqs as requirements for this phase
func (p *Phase) Require(reqs ...*Phase) *Phase {
	for _, req := range reqs {
		p.requires = append(p.requires, req)
	}
	return p
}

// Phases aliases the operation phase object from lib/storage
type Phase struct {
	p        storage.OperationPhase
	phases   []*Phase
	requires []*Phase
}

// child formats sub as a child of this phase and returns the path
func (p *Phase) child(sub *Phase) string {
	return p.childLiteral(sub.p.ID)
}

// childLiteral formats sub as a child of this phase and returns the path
func (p *Phase) childLiteral(sub string) string {
	if p == nil {
		return path.Join("/", sub)
	}
	return path.Join(p.p.ID, sub)
}

// resolveIDs traverses the phase tree and turns relative IDs into absolute
func resolveIDs(parent *Phase, phases []*Phase) {
	for i, phase := range phases {
		if !path.IsAbs(phases[i].p.ID) {
			phases[i].p.ID = parent.child(phase)
		}
		resolveIDs(phases[i], phases[i].phases)
	}
}

// resolveRequirements traverses the phase tree and resolves relative IDs in requirements into absolute
func resolveRequirements(parent *Phase, phases []*Phase) {
	for i := range phases {
		var requires []string
		for _, req := range phases[i].requires {
			if path.IsAbs(req.p.ID) {
				requires = append(requires, req.p.ID)
			} else {
				requires = append(requires, parent.child(req))
			}
		}
		phases[i].p.Requires = requires
		resolveRequirements(phases[i], phases[i].phases)
	}
}

// expects len(result) == len(phases)
func render(result []storage.OperationPhase, phases []*Phase) {
	for i, phase := range phases {
		result[i] = phase.p
		if len(phase.phases) == 0 {
			continue
		}
		result[i].Phases = make([]storage.OperationPhase, len(phase.phases))
		render(result[i].Phases, phase.phases)
	}
}
