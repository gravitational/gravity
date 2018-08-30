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

package utils

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// CommandOptionSetter defines a type for a functional option setter for exec.Cmd
type CommandOptionSetter func(cmd *exec.Cmd)

// Dir sets the command's working dir
func Dir(dir string) CommandOptionSetter {
	return func(cmd *exec.Cmd) {
		cmd.Dir = dir
	}
}

// Stderr redirects the command's stderr to the specified writer
func Stderr(w io.Writer) CommandOptionSetter {
	return func(cmd *exec.Cmd) {
		// preserve existing stderr
		if cmd.Stderr != nil {
			cmd.Stderr = io.MultiWriter(cmd.Stderr, w)
		} else {
			cmd.Stderr = w
		}
	}
}

// Stdout redirects the command's stdout to the specified writer
func Stdout(w io.Writer) CommandOptionSetter {
	return func(cmd *exec.Cmd) {
		// preserve existing stdout
		if cmd.Stdout != nil {
			cmd.Stdout = io.MultiWriter(cmd.Stdout, w)
		} else {
			cmd.Stdout = w
		}
	}
}

// RunGravityCommand executes the command specified with args with the current process binary
func RunGravityCommand(ctx context.Context, log log.FieldLogger, args ...string) ([]byte, error) {
	args = append([]string{Exe.Path}, args...)
	return RunCommand(ctx, log, args...)
}

// RunPlanetCommand executes the command specified with args as a planet command inside the container
func RunPlanetCommand(ctx context.Context, log log.FieldLogger, args ...string) ([]byte, error) {
	args = PlanetCommandArgs(append([]string{defaults.PlanetBin}, args...)...)
	return RunCommand(ctx, log, args...)
}

// RunCommand executes the command specified with args
func RunCommand(ctx context.Context, log log.FieldLogger, args ...string) ([]byte, error) {
	r := NewRunnerWithContext(ctx, log)
	var out bytes.Buffer
	if err := r.RunStream(&out, args...); err != nil {
		return out.Bytes(), trace.Wrap(err)
	}
	return out.Bytes(), nil
}

// NewRunnerWithContext creates a new CommandRunner using the specified context
func NewRunnerWithContext(ctx context.Context, log log.FieldLogger, setters ...CommandOptionSetter) *runner {
	runner := NewRunner(log, setters...)
	runner.ctx = ctx
	return runner
}

// NewRunner creates a new CommandRunner using ExecX APIs
func NewRunner(logger log.FieldLogger, setters ...CommandOptionSetter) *runner {
	if logger == nil {
		logger = log.StandardLogger()
	}
	return &runner{
		setters:     setters,
		FieldLogger: logger,
	}
}

// CommandRunner abstracts command execution.
// w specifies the sink for command's output.
// The command is given with args
type CommandRunner interface {
	// RunStream executes a command specified with args and streams
	// output to w
	RunStream(w io.Writer, args ...string) error
}

// RunStream executes a command specified with args and streams
// output to w
// Implements CommandRunner
func (r *runner) RunStream(w io.Writer, args ...string) error {
	name := args[0]
	args = args[1:]

	var cmd *exec.Cmd
	if r.ctx != nil {
		cmd = exec.CommandContext(r.ctx, name, args...)
	} else {
		cmd = exec.Command(name, args...)
	}
	return ExecL(cmd, w, r.FieldLogger, r.setters...)
}

type runner struct {
	setters []CommandOptionSetter
	ctx     context.Context
	log.FieldLogger
}

// ExecL executes the specified cmd and logs the command line to the specified entry
func ExecL(cmd *exec.Cmd, out io.Writer, logger log.FieldLogger, setters ...CommandOptionSetter) error {
	var stderr, stdout bytes.Buffer
	err := Exec(cmd, out, append(setters, Stderr(&stderr), Stdout(&stdout))...)
	fields := log.Fields{
		constants.FieldCommandError:       (err != nil),
		constants.FieldCommandErrorReport: trace.UserMessage(err),
		constants.FieldCommandStderr:      stderr.String(),
		constants.FieldCommandStdout:      stdout.String(),
	}
	logger.WithFields(fields).Info(strings.Join(cmd.Args, " "))
	return err
}

func Exec(cmd *exec.Cmd, out io.Writer, setters ...CommandOptionSetter) error {
	return ExecWithInput(cmd, "", out, setters...)
}

func ExecWithInput(cmd *exec.Cmd, input string, out io.Writer, setters ...CommandOptionSetter) error {
	execPath, err := exec.LookPath(cmd.Path)
	if err != nil {
		return trace.Wrap(err)
	}
	cmd.Path = execPath
	cmd.Stdout = out
	cmd.Stderr = out

	for _, s := range setters {
		s(cmd)
	}

	var stdin io.WriteCloser
	if input != "" {
		stdin, err = cmd.StdinPipe()
		if err != nil {
			return trace.Wrap(err)
		}
		defer stdin.Close()
	}

	if err := cmd.Start(); err != nil {
		return trace.Wrap(err)
	}

	if stdin != nil {
		io.WriteString(stdin, input)
	}

	if err := cmd.Wait(); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func CombinedOutput(cmd *exec.Cmd, out io.Writer) (string, error) {
	buf := &SafeByteBuffer{}
	err := Exec(cmd, io.MultiWriter(buf, out))
	return buf.String(), err
}

func ExecuteWithDelay(args []string, delay time.Duration) error {
	dir, err := ioutil.TempDir("", "")
	if err != nil {
		return trace.Wrap(err)
	}
	script := fmt.Sprintf(`#!/bin/bash
if [ -n "$1" ]; then
  echo "sleeping!"
  sleep %v
  %v
else
  nohup $0 call 2> error.log > output.log &
fi

`, int(delay/time.Second), strings.Join(args, " "))

	scriptPath := filepath.Join(dir, "script.sh")
	if err := ioutil.WriteFile(scriptPath, []byte(script), 0777); err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(
		Exec(exec.Command(scriptPath),
			log.StandardLogger().Writer(),
			Dir(dir),
		),
	)
}

// Command abstracts a CLI command
type Command interface {
	// Args returns the complete command line of this command
	Args() []string
}
