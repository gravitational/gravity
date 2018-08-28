package update

import (
	"context"
	"strings"
	"testing"

	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/ops/opsservice"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"gopkg.in/check.v1"
)

func TestUpdate(t *testing.T) { check.TestingT(t) }

type FSMSuite struct {
	engine *fsmUpdateEngine
	fsm    *fsm.FSM
}

var _ = check.Suite(&FSMSuite{})

func (s *FSMSuite) SetUpTest(c *check.C) {
	services := opsservice.SetupTestServices(c)
	s.engine = &fsmUpdateEngine{
		FSMConfig: FSMConfig{
			LocalBackend: services.Backend,
			Packages:     services.Packages,
			Apps:         services.Apps,
			Operator:     services.Operator,
			Spec:         getTestExecutor(),
		},
		FieldLogger: logrus.WithField(trace.Component, "fsm-suite"),
	}
	s.fsm = &fsm.FSM{
		Config:      fsm.Config{Engine: s.engine},
		FieldLogger: logrus.WithField(trace.Component, "fsm-suite"),
	}
}

func (s *FSMSuite) TestFSMBasic(c *check.C) {
	plan := storage.OperationPlan{
		OperationID:   "operation-1",
		OperationType: "test_operation",
		ClusterName:   "example.com",
		Phases: []storage.OperationPhase{
			{ID: "/phase1"},
			{ID: "/phase2", Requires: []string{"/phase1"}},
		},
	}

	checkStates(c, s.resolvePlan(c, plan), map[string]string{
		"/phase1": storage.OperationPhaseStateUnstarted,
		"/phase2": storage.OperationPhaseStateUnstarted,
	})

	s.engine.plan = &plan
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
		OperationID:   "operation-1",
		OperationType: "test_operation",
		ClusterName:   "example.com",
		Phases: []storage.OperationPhase{
			{ID: "/phase1", Phases: []storage.OperationPhase{
				{ID: "/phase1/sub1"},
				{ID: "/phase1/sub2"},
			}},
		},
	}

	s.engine.plan = &plan

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
		OperationID:   "operation-1",
		OperationType: "test_operation",
		ClusterName:   "example.com",
		Phases: []storage.OperationPhase{
			{ID: "/phase1", Phases: []storage.OperationPhase{
				{ID: "/phase1/sub1"},
				{ID: "/phase1/sub2"},
			}},
		},
	}

	s.engine.plan = &plan

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

func (s *FSMSuite) resolvePlan(c *check.C, plan storage.OperationPlan) *storage.OperationPlan {
	changelog, err := s.engine.LocalBackend.GetOperationPlanChangelog(plan.ClusterName, plan.OperationID)
	c.Assert(err, check.IsNil)
	return fsm.ResolvePlan(plan, changelog)
}

func checkStates(c *check.C, plan *storage.OperationPlan, states map[string]string) {
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
