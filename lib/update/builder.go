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
package update

import (
	"path"

	"github.com/gravitational/gravity/lib/storage"
)

// AddSequential will append sub-phases which depend one upon another
func (p *Phase) AddSequential(subs ...Phase) {
	for i := range subs {
		if len(p.Phases) != 0 {
			subs[i].RequireIDs(p.Phases[len(p.Phases)-1].ID)
		}
		p.Phases = append(p.Phases, storage.OperationPhase(subs[i]))
	}
}

// AddParallel will append sub-phases which depend on parent only
func (p *Phase) AddParallel(subs ...Phase) {
	p.Add(subs...)
}

// Add adds the specified sub-phases without dependency
func (p *Phase) Add(subs ...Phase) {
	p.Phases = append(p.Phases, Phases(subs).AsPhases()...)
}

// AddWithDependency sets phase as explicit dependency on subs
func (p *Phase) AddWithDependency(dep PhaseIder, subs ...Phase) {
	for i := range subs {
		subs[i].Require(dep)
		p.Phases = append(p.Phases, storage.OperationPhase(subs[i]))
	}
}

// Child formats sub as a child of this phase and returns the path
func (p *Phase) Child(sub Phase) string {
	return p.ChildLiteral(sub.ID)
}

// ChildLiteral formats sub as a child of this phase and returns the path
func (p *Phase) ChildLiteral(sub string) string {
	if p == nil {
		return path.Join("/", sub)
	}
	return path.Join(p.ID, sub)
}

// Require adds the specified phases reqs as requirements for this phase
func (p *Phase) Require(reqs ...PhaseIder) *Phase {
	for _, req := range reqs {
		p.Requires = append(p.Requires, req.GetID())
	}
	return p
}

// RequireIDs adds the specified phase IDs as requirements for this phase
func (p *Phase) RequireIDs(ids ...string) *Phase {
	for _, id := range ids {
		p.Requires = append(p.Requires, id)
	}
	return p
}

// RequireLiteral adds the specified phase IDs as requirements for this phase
func (p *Phase) RequireLiteral(ids ...string) *Phase {
	p.Requires = append(p.Requires, ids...)
	return p
}

// GetID returns this phase's ID.
// implements PhaseDependency
func (p Phase) GetID() string {
	return p.ID
}

// RootPhase makes the specified phase root
func RootPhase(sub Phase) Phase {
	sub.ID = path.Join("/", sub.ID)
	return sub
}

// GetID returns the ID of the phase.
// implements PhaseIder
func (r PhaseRef) GetID() string {
	return string(r)
}

// PhaseRef refers to a phase by ID
type PhaseRef string

// Phases aliases the operation phase object from lib/storage
type Phase storage.OperationPhase

type ParentPhase interface {
	ChildLiteral(sub string) string
}

// PhaseIder is an interface to identify phases
type PhaseIder interface {
	// GetID identifies the phase by ID
	GetID() string
}

// AsPhases converts this list to a slice of storate.OperationPhase
func (r Phases) AsPhases() (result []storage.OperationPhase) {
	result = make([]storage.OperationPhase, 0, len(r))
	for _, phase := range r {
		result = append(result, storage.OperationPhase(phase))
	}
	return result
}

// Phases is a list of Phase
type Phases []Phase

// AsPhases converts this list to a slice of storate.OperationPhase
func (r PhasePtrs) AsPhases() (result []storage.OperationPhase) {
	result = make([]storage.OperationPhase, 0, len(r))
	for _, phase := range r {
		result = append(result, storage.OperationPhase(*phase))
	}
	return result
}

// PhasePtrs is a list of Phase
type PhasePtrs []*Phase

// DependencyForServer looks up a dependency in the list of sub-phases of the give phase
// that references the specified server and returns a reference to it.
// If no server has been found, it retruns the reference to the phase itself
func DependencyForServer(phase Phase, server storage.Server) PhaseRef {
	for _, p := range phase.Phases {
		if p.Data.Server.AdvertiseIP == server.AdvertiseIP {
			return PhaseRef(p.ID)
		}
	}
	return PhaseRef(phase.ID)
}

// ResolvePlan resolves dependencies between phases
// and renders IDs as absolute in the specified plan
func ResolvePlan(plan *storage.OperationPlan) {
	resolveIDs(nil, plan.Phases)
	resolveRequirementIDs(nil, plan.Phases)
}

// resolveIDs travels the phase tree and turns relative IDs into absolute
func resolveIDs(parent *Phase, phases []storage.OperationPhase) {
	for i := range phases {
		if !path.IsAbs(phases[i].ID) {
			phases[i].ID = parent.Child(Phase(phases[i]))
		}
		resolveIDs((*Phase)(&phases[i]), phases[i].Phases)
	}
}

// resolveRequirementIDs travels the phase tree and resolves relative IDs in requirements into absolute
func resolveRequirementIDs(parent *Phase, phases []storage.OperationPhase) {
	for i := range phases {
		var requires []string
		for _, req := range phases[i].Requires {
			if path.IsAbs(req) {
				requires = append(requires, req)
			} else {
				requires = append(requires, parent.ChildLiteral(req))
			}
		}
		phases[i].Requires = requires
		resolveRequirementIDs((*Phase)(&phases[i]), phases[i].Phases)
	}
}
