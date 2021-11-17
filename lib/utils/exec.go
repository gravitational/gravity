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
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
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
		cmd.Stderr = w
	}
}

// Stdout redirects the command's stdout to the specified writer
func Stdout(w io.Writer) CommandOptionSetter {
	return func(cmd *exec.Cmd) {
		cmd.Stdout = w
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

// RunInPlanetCommand executes the command specified with args inside planet container
func RunInPlanetCommand(ctx context.Context, log log.FieldLogger, args ...string) ([]byte, error) {
	return RunCommand(ctx, log, PlanetCommandArgs(args...)...)
}

// RunCommand executes the command specified with args
func RunCommand(ctx context.Context, logger log.FieldLogger, args ...string) ([]byte, error) {
	var out bytes.Buffer
	if logger == nil {
		logger = log.WithField(trace.Component, "utils")
	}
	logger.WithField("args", args).Debug("Run command.")
	if err := RunStream(ctx, &out, &out, args...); err != nil {
		return out.Bytes(), trace.Wrap(err)
	}
	return out.Bytes(), nil
}

// Runner is the default CommandRunner
var Runner CommandRunner = CommandRunnerFunc(RunStream)

// CommandRunner abstracts command execution.
// w specifies the sink for command's output.
// The command is given with args
type CommandRunner interface {
	// RunStream executes a command specified with args and streams
	// output to w using ctx for cancellation
	RunStream(ctx context.Context, stdout, stderr io.Writer, args ...string) error
}

// RunStream invokes r with the specified arguments.
// Implements CommandRunner
func (r CommandRunnerFunc) RunStream(ctx context.Context, stdout, stderr io.Writer, args ...string) error {
	return r(ctx, stdout, stderr, args...)
}

// CommandRunnerFunc is the wrapper that allows standalone functions
// to act as CommandRunners
type CommandRunnerFunc func(ctx context.Context, stdout, stderr io.Writer, args ...string) error

// RunStream executes a command specified with args and streams output to w
func RunStream(ctx context.Context, stdout, stderr io.Writer, args ...string) error {
	name := args[0]
	args = args[1:]
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	log.WithField("cmd", cmd.Args).Debug("Execute.")
	if err := cmd.Start(); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(cmd.Wait())
}

// ExecUnprivileged executes the specified command as unprivileged user
func ExecUnprivileged(ctx context.Context, command string, args []string, opts ...CommandOptionSetter) error {
	nobody, err := user.Lookup("nobody")
	if err != nil {
		return trace.ConvertSystemError(err)
	}
	cmd := exec.CommandContext(ctx, command, args...)
	uid, err := getUID(*nobody)
	if err != nil {
		return trace.Wrap(err)
	}
	gid, err := getGID(*nobody)
	if err != nil {
		return trace.Wrap(err)
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{
			Uid: uid,
			Gid: gid,
		},
	}
	for _, opt := range opts {
		opt(cmd)
	}
	return cmd.Run()
}

// ExecL executes the specified cmd and logs the command line to the specified entry
func ExecL(cmd *exec.Cmd, out io.Writer, logger log.FieldLogger, setters ...CommandOptionSetter) error {
	var stderr, stdout bytes.Buffer
	// Since we split the Stderr/Stdout into separate sinks
	// but want to capture to out as required by the ExecL interface,
	// we need to use a concurrency-safe version of the out buffer due
	// to the os.exec.Command contract (only equal values of Stderr/Stdout
	// are guaranteed to receive Writes from a single goroutine).
	outBuf := NewSyncBufferWithWriter(out)
	err := Exec(cmd, out, append(setters,
		Stdout(io.MultiWriter(outBuf, &stdout)),
		Stderr(io.MultiWriter(outBuf, &stderr)))...)
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
		io.WriteString(stdin, input) //nolint:errcheck
	}

	if err := cmd.Wait(); err != nil {
		return trace.Wrap(err)
	}

	return nil
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
	if err := ioutil.WriteFile(scriptPath, []byte(script), defaults.PrivateExecutableFileMask); err != nil {
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

func getUID(u user.User) (uid uint32, err error) {
	id, err := strconv.Atoi(u.Uid)
	if err != nil {
		return 0, trace.BadParameter("invalid UID for user %v: %v", u.Username, u.Uid)
	}
	return uint32(id), nil
}

func getGID(u user.User) (gid uint32, err error) {
	id, err := strconv.Atoi(u.Gid)
	if err != nil {
		return 0, trace.BadParameter("invalid GID for user %v: %v", u.Username, u.Gid)
	}
	return uint32(id), nil
}
