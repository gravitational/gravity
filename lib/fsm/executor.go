package fsm

import (
	"context"

	"github.com/sirupsen/logrus"
)

// PhaseExecutor defines an operation plan phase executor
//
// An executor performs a (compound) operation that it can then roll back
// in case any of the intermediate steps fail.
//
// An executor can optionally validate its requirements and outcomes by
// implementing non-trivial PreCheck/PostCheck APIs.
type PhaseExecutor interface {
	// PreCheck is called before phase execution
	PreCheck(context.Context) error
	// PostCheck is called after successful phase execution
	PostCheck(context.Context) error
	// Execute executes phase
	Execute(context.Context) error
	// Rollback performs phase rollback
	Rollback(context.Context) error
	// FieldLogger allows FSM engine to use phase-specific logger
	logrus.FieldLogger
}

// FSMSpecFunc defines a function that returns an appropriate executor for
// the specified operation phase
type FSMSpecFunc func(ExecutorParams, Remote) (PhaseExecutor, error)
