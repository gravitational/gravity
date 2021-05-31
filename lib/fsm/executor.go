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
//nolint:revive // TODO: rename to SpecFunc
type FSMSpecFunc func(ExecutorParams, Remote) (PhaseExecutor, error)
