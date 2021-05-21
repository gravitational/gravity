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

package report

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
)

// NewFileWriter creates a Writer that writes to a file in the specified directory dir
func NewFileWriter(dir string) FileWriterFunc {
	return FileWriterFunc(func(name string) (io.WriteCloser, error) {
		fileName := filepath.Join(dir, name)
		return NewPendingFileWriter(fileName), nil
	})
}

// NewPendingFileWriter creates a new instance of the pendingWriter
// for the specified path
func NewPendingFileWriter(path string) *PendingWriter {
	return &PendingWriter{path: path}
}

// Write forwards specified data to the underlying file which
// is created at this point if not yet existing.
// It implements io.Writer
func (r *PendingWriter) Write(data []byte) (n int, err error) {
	if len(data) == 0 {
		return 0, nil
	}
	if r.file == nil {
		var err error
		r.file, err = os.OpenFile(r.path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC,
			defaults.SharedReadWriteMask)
		if err != nil {
			return 0, err
		}
	}
	return r.file.Write(data)
}

// Close closes the underlying file if it has been created.
// It implements io.Closer
func (r *PendingWriter) Close() error {
	if r.file == nil {
		return nil
	}
	err := r.file.Close()
	r.file = nil
	return err
}

// PendingWriter forwards data to the underlying file.
// It only creates a file if there's data to forward.
type PendingWriter struct {
	path string
	file io.WriteCloser
}

// FileWriter is a factory for creating named file writers
type FileWriter interface {
	// NewWriter creates a new report writer with the specified name.
	NewWriter(name string) (io.WriteCloser, error)
}

// NewWriter creates a new writer to write to a file with the specified name
func (r FileWriterFunc) NewWriter(name string) (io.WriteCloser, error) {
	return r(name)
}

// FileWriterFunc is a functional wrapper for NamedWriter
type FileWriterFunc func(name string) (io.WriteCloser, error)

// Collector defines an interface to collect diagnostic information
type Collector interface {
	// Collect collects diagnostics using CommandRunner and serializes
	// them using specified Writer
	Collect(context.Context, FileWriter, utils.CommandRunner) error
}

// Collectors is a list of Collectors
type Collectors []Collector

// Collect implements Collector for a list of Collectors
func (r Collectors) Collect(ctx context.Context, reportWriter FileWriter, runner utils.CommandRunner) error {
	var errors []error
	for _, collector := range r {
		err := collector.Collect(ctx, reportWriter, runner)
		if err != nil {
			errors = append(errors, err)
		}
	}
	return trace.NewAggregate(errors...)
}

// Collect invokes this function with the specified parameters.
// Implements Collector
func (r CollectorFunc) Collect(ctx context.Context, w FileWriter, runner utils.CommandRunner) error {
	return r(ctx, w, runner)
}

// CollectorFunc allows ordinary functions as Collectors
type CollectorFunc func(context.Context, FileWriter, utils.CommandRunner) error

// Cmd creates a new Command with the given name and command line
func Cmd(name string, args ...string) Command {
	cmd := args[0]
	args = args[1:]
	return Command{name: name, cmd: cmd, args: args}
}

// Self returns a reference to this binary.
// name names the resulting output file.
// args is the list of optional command line arguments
func Self(name string, args ...string) Command {
	return Command{name: name, cmd: utils.Exe.Path, args: args}
}

// Command defines a generic command with a name and a list of arguments
type Command struct {
	name             string
	cmd              string
	args             []string
	successExitCodes []int
}

// Collect implements Collector for this Command
func (r Command) Collect(ctx context.Context, reportWriter FileWriter, runner utils.CommandRunner) error {
	w, err := reportWriter.NewWriter(r.name)
	if err != nil {
		return trace.Wrap(err)
	}
	defer w.Close()

	var stderr bytes.Buffer
	args := []string{r.cmd}
	args = append(args, r.args...)
	if err := runner.RunStream(ctx, w, &stderr, args...); err != nil && r.isExitCodeFailed(err) {
		return trace.Wrap(err, "failed to execute %v: %s", r, stderr.String())
	}
	return nil
}

func (r Command) isExitCodeFailed(err error) bool {
	exitCode := utils.ExitStatusFromError(err)
	if exitCode == nil {
		return false
	}
	return *exitCode != 0 && !exitCodeOneOf(*exitCode, r.successExitCodes...)
}

// String returns a text representation of this command
func (r Command) String() string {
	return fmt.Sprintf("%v(cmd=%v, args=%v)", r.name, r.cmd, r.args)
}

// Script creates a new script collector
func Script(name, script string) ScriptCollector {
	return ScriptCollector{name: name, script: script}
}

// Collect implements Collector using a bash script
func (r ScriptCollector) Collect(ctx context.Context, reportWriter FileWriter, runner utils.CommandRunner) error {
	args := []string{"/bin/bash", "-c", r.script}
	w, err := reportWriter.NewWriter(r.name)
	if err != nil {
		return trace.Wrap(err)
	}
	defer w.Close()

	var stderr bytes.Buffer
	if err := runner.RunStream(ctx, w, &stderr, args...); err != nil {
		return trace.Wrap(err, "failed to execute script %v: %s", r, stderr.String())
	}
	return nil
}

// String returns a text representation of this script
func (r ScriptCollector) String() string {
	return fmt.Sprintf("script(%v)", r.name)
}

// ScriptCollector is a convenience Collector to execute bash scripts
type ScriptCollector struct {
	name   string
	script string
}

func exitCodeOneOf(exitCode int, exitCodes ...int) bool {
	if len(exitCodes) == 0 {
		return false
	}
	for _, code := range exitCodes {
		if code == exitCode {
			return true
		}
	}
	return false
}
