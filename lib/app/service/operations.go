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

package service

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// operationStep describes a single progress within a complex operation.
// It represents a numeric completion value as well as textual message
// describing the current state
type operationStep interface {
	Completion() int
	State() string
	Message() string
	String() string
}

// operation represents a complex operation consisting of several progressions
// and a log file.
// An operation is capable of persisting its state into the specified backend
// and obtain the current state from the specified operationStep
type operation interface {
	logPath() string
	update(storage.Backend, operationStep) error
}

// operationContext wraps an operation:
//
//  - it serves as a logging endpoint (implements io.WriteCloser)
//  - is capable of progressing the said operation
type operationContext struct {
	io.WriteCloser

	stateDir string
	op       operation
	backend  storage.Backend
}

// newOperationContext creates a new operationContext for the specified operation
// using the given stateDir for temporary storage and the specified backend
// for persistency
func newOperationContext(op operation, stateDir string, backend storage.Backend) (*operationContext, error) {
	logPath := filepath.Join(stateDir, op.logPath())
	err := os.MkdirAll(filepath.Dir(logPath), defaults.SharedDirMask)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	recorder, err := os.OpenFile(logPath, os.O_CREATE|os.O_RDWR|os.O_APPEND, defaults.SharedReadWriteMask)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &operationContext{
		backend:     backend,
		op:          op,
		stateDir:    stateDir,
		WriteCloser: recorder,
	}, nil
}

// operationLogTailReader returns a tail reader to the underlined log file
func (r *operationContext) operationLogTailReader() (io.ReadCloser, error) {
	logPath := filepath.Join(r.stateDir, r.op.logPath())
	reader, err := utils.NewTailReader(logPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return reader, nil
}

// operationLogReader returns a reader to the underlined log file
func (r *operationContext) operationLogReader() (io.ReadCloser, int64, error) {
	logPath := filepath.Join(r.stateDir, r.op.logPath())
	fi, err := os.Stat(logPath)
	if err != nil {
		return nil, 0, trace.Wrap(err)
	}
	reader, err := os.Open(logPath)
	if err != nil {
		return nil, 0, trace.Wrap(err)
	}
	return reader, fi.Size(), nil
}

// update advances the wrapped operation to the specified operationStep
func (r *operationContext) update(step operationStep) error {
	r.Infof("step: `%v`", step)
	return r.op.update(r.backend, step)
}

// Infof implements the app.Logger interface and writes the specified
// arguments into the underlines io.WriteCloser
func (r *operationContext) Infof(format string, args ...interface{}) {
	log.Infof(format, args...)
	fmt.Fprintf(r.WriteCloser, format, args...)
	_, _ = r.Write([]byte{'\n'})
}
