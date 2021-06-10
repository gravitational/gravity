/*
Copyright 2020 Gravitational, Inc.

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
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gravitational/gravity/lib/rpc"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
)

func newTestEngine(getPlan func() storage.OperationPlan) *testEngine {
	return &testEngine{
		getPlan: getPlan,
		clock:   clockwork.NewFakeClock(),
	}
}

// testEngine is fsm engine used in tests. Keeps its changelog in memory.
type testEngine struct {
	getPlan   func() storage.OperationPlan
	changelog storage.PlanChangelog
	clock     clockwork.Clock
}

// GetExecutor returns one of the test executors depending on the specified phase.
func (t *testEngine) GetExecutor(p ExecutorParams, r Remote) (PhaseExecutor, error) {
	if strings.HasPrefix(p.Phase.ID, "/init") {
		return newInit(), nil
	} else if strings.HasPrefix(p.Phase.ID, "/bootstrap") {
		return newBootstrap(), nil
	} else if strings.HasPrefix(p.Phase.ID, "/upgrade") {
		return newUpgrade(), nil
	}
	return nil, trace.BadParameter("unknown phase %q", p.Phase.ID)
}

// ChangePhaseState records the provided phase state change in the test engine.
func (t *testEngine) ChangePhaseState(ctx context.Context, ch StateChange) error {
	t.changelog = append(t.changelog, storage.PlanChange{
		PhaseID:  ch.Phase,
		NewState: ch.State,
		// Make sure that new changelog entries get the most recent timestamp.
		Created: t.clock.Now().Add(time.Duration(len(t.changelog)) * time.Minute),
	})
	return nil
}

// changePhaseStateWithTimestamp records the provided phase state change in the
// test engine with the specified timestamp.
func (t *testEngine) changePhaseStateWithTimestamp(ch StateChange, created time.Time) {
	t.changelog = append(t.changelog, storage.PlanChange{
		PhaseID:  ch.Phase,
		NewState: ch.State,
		Created:  created,
	})
}

// GetPlan returns the test plan with the changelog applied.
func (t *testEngine) GetPlan() (*storage.OperationPlan, error) {
	return ResolvePlan(t.getPlan(), t.changelog), nil
}

// RunCommand is not implemented by the test engine.
func (t *testEngine) RunCommand(ctx context.Context, r rpc.RemoteRunner, s storage.Server, p Params) error {
	return trace.NotImplemented("test engine cannot execute remote commands")
}

// Complete is not implemented by the test engine.
func (t *testEngine) Complete(ctx context.Context, err error) error {
	return trace.NotImplemented("test engine cannot complete operations")
}

func newInit() *initExecutor {
	return &initExecutor{newTest()}
}

type initExecutor struct {
	*testExecutor
}

func newBootstrap() *bootstrapExecutor {
	return &bootstrapExecutor{newTest()}
}

type bootstrapExecutor struct {
	*testExecutor
}

func newUpgrade() *upgradeExecutor {
	return &upgradeExecutor{newTest()}
}

type upgradeExecutor struct {
	*testExecutor
}

func newTest() *testExecutor {
	return &testExecutor{FieldLogger: logrus.WithField(trace.Component, "test")}
}

// testExecutor serves as a base for executors used in tests.
type testExecutor struct {
	logrus.FieldLogger
}

func (e *testExecutor) Execute(ctx context.Context) error   { return nil }
func (e *testExecutor) Rollback(ctx context.Context) error  { return nil }
func (e *testExecutor) PreCheck(ctx context.Context) error  { return nil }
func (e *testExecutor) PostCheck(ctx context.Context) error { return nil }

// testPlanner knows how to generate plans and changelogs used in fsm tests.
type testPlanner struct{}

func (p *testPlanner) newPlan(phases ...storage.OperationPhase) *storage.OperationPlan {
	return &storage.OperationPlan{Phases: phases}
}

func (p *testPlanner) newChangelog(changes ...storage.PlanChange) storage.PlanChangelog {
	return storage.PlanChangelog(changes)
}

func (p *testPlanner) initPhase(state string) storage.OperationPhase {
	return storage.OperationPhase{ID: "/init", State: state}
}

func (p *testPlanner) initChange(state string) storage.PlanChange {
	return storage.PlanChange{PhaseID: "/init", NewState: state}
}

func (p *testPlanner) bootstrapPhase(phases ...storage.OperationPhase) storage.OperationPhase {
	return storage.OperationPhase{ID: "/bootstrap", Phases: phases}
}

func (p *testPlanner) bootstrapSubPhase(node, state string) storage.OperationPhase {
	return storage.OperationPhase{ID: fmt.Sprintf("/bootstrap/%v", node), State: state}
}

func (p *testPlanner) bootstrapSubChange(node, state string) storage.PlanChange {
	return storage.PlanChange{PhaseID: fmt.Sprintf("/bootstrap/%v", node), NewState: state}
}

func (p *testPlanner) upgradePhase(state string) storage.OperationPhase {
	return storage.OperationPhase{ID: "/upgrade", State: state}
}

func (p *testPlanner) upgradeChange(state string) storage.PlanChange {
	return storage.PlanChange{PhaseID: "/upgrade", NewState: state}
}
