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
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
)

// NewFileWriter creates a Writer that writes to a file
func NewFileWriter(dir string) Writer {
	return func(name string) (io.WriteCloser, error) {
		fileName := filepath.Join(dir, name)
		return NewPendingFileWriter(fileName), nil
	}
}

// NewPendingFileWriter creates a new instance of the pendingWriter
// for the specified path
func NewPendingFileWriter(path string) *pendingWriter {
	return &pendingWriter{path: path}
}

// Write forwards specified data to the underlying file which
// is created at this point if not yet existing.
// It implements io.Writer
func (r *pendingWriter) Write(data []byte) (n int, err error) {
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
func (r *pendingWriter) Close() error {
	if r.file == nil {
		return nil
	}
	err := r.file.Close()
	r.file = nil
	return err
}

// pendingWriter forwards data to the underlying file.
// It only creates a file if there's data to forward.
type pendingWriter struct {
	path string
	file io.WriteCloser
}

// Writer defines an interface for collectors to serialize
// data with a name.
type Writer func(name string) (io.WriteCloser, error)

// Collector defines an interface to collect diagnostic information
type Collector interface {
	// Collect collects diagnostics using CommandRunner and serializes
	// them using specified Writer
	Collect(context.Context, Writer, utils.CommandRunner) error
}

// Collectors is a list of Collectors
type Collectors []Collector

// Collect implements Collector for a list of Collectors
func (r Collectors) Collect(ctx context.Context, reportWriter Writer, runner utils.CommandRunner) error {
	var errors []error
	for _, collector := range r {
		err := collector.Collect(ctx, reportWriter, runner)
		if err != nil {
			errors = append(errors, err)
		}
	}
	return trace.NewAggregate(errors...)
}

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
	name string
	cmd  string
	args []string
}

// Collect implements Collector for this Command
func (r Command) Collect(ctx context.Context, reportWriter Writer, runner utils.CommandRunner) error {
	w, err := reportWriter(r.name)
	if err != nil {
		return trace.Wrap(err)
	}
	defer w.Close()

	args := []string{r.cmd}
	args = append(args, r.args...)
	return runner.RunStream(ctx, w, args...)
}

// Script creates a new script collector
func Script(name, script string) ScriptCollector {
	return ScriptCollector{name: name, script: script}
}

// Collect implements Collector using a bash script
func (r ScriptCollector) Collect(ctx context.Context, reportWriter Writer, runner utils.CommandRunner) error {
	args := []string{"/bin/bash", "-c", r.script}
	w, err := reportWriter(r.name)
	if err != nil {
		return trace.Wrap(err)
	}
	defer w.Close()

	return runner.RunStream(ctx, w, args...)
}

// ScriptCollector is a convenience Collector to execute bash scripts
type ScriptCollector struct {
	name   string
	script string
}

func tarball(pattern string) string {
	return fmt.Sprintf(`
#!/bin/bash
/bin/tar cz --ignore-failed-read --ignore-command-error -f /dev/stdout -C / $(readlink -e %v) -P 2> /dev/null
`, pattern)
}
