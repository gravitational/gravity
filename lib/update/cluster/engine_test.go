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

package cluster

import (
	"context"
	"strings"
	"testing"

	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/ops/opsservice"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/update"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"gopkg.in/check.v1"
)

func TestUpdate(t *testing.T) { check.TestingT(t) }

type FSMSuite struct {
	engine *engine
	fsm    *fsm.FSM
}

var _ = check.Suite(&FSMSuite{})

const (
	operationID = "operation-1"
	clusterName = "example.com"
)

func (s *FSMSuite) SetUpTest(c *check.C) {
	services := opsservice.SetupTestServices(c)
	logger := logrus.WithField(trace.Component, "fsm-suite")
	s.engine = &engine{
		Config: Config{
			Config: update.Config{
				LocalBackend: services.Backend,
				Operator:     services.Operator,
			},
			Packages: services.Packages,
			Apps:     services.Apps,
			Spec:     getTestExecutor(),
		},
		FieldLogger: logger,
		reconciler: &testReconciler{
			backend: services.Backend,
		},
	}
	s.fsm = &fsm.FSM{
		Config:      fsm.Config{Engine: s.engine},
		FieldLogger: logger,
	}
}

func (s *FSMSuite) TestFSMBasic(c *check.C) {
	plan := storage.OperationPlan{
		OperationID:   operationID,
		OperationType: "test_operation",
		ClusterName:   clusterName,
		Phases: []storage.OperationPhase{
			{ID: "/phase1"},
			{ID: "/phase2", Requires: []string{"/phase1"}},
		},
	}

	checkStates(c, s.resolvePlan(c, plan), map[string]string{
		"/phase1": storage.OperationPhaseStateUnstarted,
		"/phase2": storage.OperationPhaseStateUnstarted,
	})

	s.engine.plan = plan
	ctx := context.TODO()

	// phase2 requires phase1 to be completed first
	err := s.fsm.ExecutePhase(ctx, fsm.Params{
		PhaseID: "/phase2",
	})
	c.Assert(err, check.NotNil)

	err = s.fsm.ExecutePhase(ctx, fsm.Params{
		PhaseID: "/phase1",
	})
	c.Assert(err, check.IsNil)

	checkStates(c, s.resolvePlan(c, plan), map[string]string{
		"/phase1": storage.OperationPhaseStateCompleted,
		"/phase2": storage.OperationPhaseStateUnstarted,
	})

	err = s.fsm.ExecutePhase(ctx, fsm.Params{
		PhaseID: "/phase2",
	})
	c.Assert(err, check.IsNil)

	checkStates(c, s.resolvePlan(c, plan), map[string]string{
		"/phase1": storage.OperationPhaseStateCompleted,
		"/phase2": storage.OperationPhaseStateCompleted,
	})
}

func (s *FSMSuite) TestFSMExecuteSubphase(c *check.C) {
	plan := storage.OperationPlan{
		OperationID:   operationID,
		OperationType: "test_operation",
		ClusterName:   clusterName,
		Phases: []storage.OperationPhase{
			{ID: "/phase1", Phases: []storage.OperationPhase{
				{ID: "/phase1/sub1"},
				{ID: "/phase1/sub2"},
			}},
		},
	}

	s.engine.plan = plan

	err := s.fsm.ExecutePhase(context.TODO(), fsm.Params{
		PhaseID: "/phase1/sub1",
	})
	c.Assert(err, check.IsNil)

	checkStates(c, s.resolvePlan(c, plan), map[string]string{
		"/phase1":      storage.OperationPhaseStateInProgress,
		"/phase1/sub1": storage.OperationPhaseStateCompleted,
		"/phase1/sub2": storage.OperationPhaseStateUnstarted,
	})
}

func (s *FSMSuite) TestFSMExecutePhaseWithSubphases(c *check.C) {
	plan := storage.OperationPlan{
		OperationID:   operationID,
		OperationType: "test_operation",
		ClusterName:   clusterName,
		Phases: []storage.OperationPhase{
			{ID: "/phase1", Phases: []storage.OperationPhase{
				{ID: "/phase1/sub1"},
				{ID: "/phase1/sub2"},
			}},
		},
	}

	s.engine.plan = plan

	err := s.fsm.ExecutePhase(context.TODO(), fsm.Params{
		PhaseID: "/phase1",
	})
	c.Assert(err, check.IsNil)

	checkStates(c, s.resolvePlan(c, plan), map[string]string{
		"/phase1":      storage.OperationPhaseStateCompleted,
		"/phase1/sub1": storage.OperationPhaseStateCompleted,
		"/phase1/sub2": storage.OperationPhaseStateCompleted,
	})
}

func (s *FSMSuite) resolvePlan(c *check.C, plan storage.OperationPlan) storage.OperationPlan {
	changelog, err := s.engine.LocalBackend.GetOperationPlanChangelog(plan.ClusterName, plan.OperationID)
	c.Assert(err, check.IsNil)
	return fsm.ResolvePlan(plan, changelog)
}

func checkStates(c *check.C, plan storage.OperationPlan, states map[string]string) {
	for name, state := range states {
		phase, err := fsm.FindPhase(plan, name)
		c.Assert(err, check.IsNil)
		c.Assert(phase.GetState(), check.Equals, state)
	}
}

func getTestExecutor() fsm.FSMSpecFunc {
	return func(p fsm.ExecutorParams, remote fsm.Remote) (fsm.PhaseExecutor, error) {
		if strings.HasPrefix(p.Phase.ID, "/phase1") {
			return &testPhase1{
				FieldLogger: logrus.NewEntry(logrus.New()),
			}, nil
		}
		if p.Phase.ID == "/phase2" {
			return &testPhase2{
				FieldLogger: logrus.NewEntry(logrus.New()),
			}, nil
		}
		return nil, trace.BadParameter("unsupported phase %q", p.Phase.ID)
	}
}

type testPhase1 struct {
	logrus.FieldLogger
}

func (p *testPhase1) PreCheck(context.Context) error {
	return nil
}
func (p *testPhase1) PostCheck(context.Context) error {
	return nil
}
func (p *testPhase1) Execute(context.Context) error {
	return nil
}
func (p *testPhase1) Rollback(context.Context) error {
	return nil
}

type testPhase2 struct {
	logrus.FieldLogger
}

func (p *testPhase2) PreCheck(context.Context) error {
	return nil
}
func (p *testPhase2) PostCheck(context.Context) error {
	return nil
}
func (p *testPhase2) Execute(context.Context) error {
	return nil
}
func (p *testPhase2) Rollback(context.Context) error {
	return nil
}

func (r *testReconciler) ReconcilePlan(ctx context.Context, plan storage.OperationPlan) (*storage.OperationPlan, error) {
	changes, err := r.backend.GetOperationPlanChangelog(plan.ClusterName, plan.OperationID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	plan = fsm.ResolvePlan(plan, changes)
	return &plan, nil
}

type testReconciler struct {
	backend storage.Backend
}
