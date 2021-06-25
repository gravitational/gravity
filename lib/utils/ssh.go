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
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	"github.com/gravitational/gravity/lib/defaults"

	"github.com/cenkalti/backoff"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

// OutputParseFn defines a parser function for arbitrary input r
type OutputParseFn func(r *bufio.Reader) error

type sshCommand struct {
	command      string
	abortOnError bool
	env          map[string]string
	withRetries  bool
}

// SSHCommands abstracts a way of executing a set of remote commands
type SSHCommands interface {
	// adds new command with default policy
	C(format string, a ...interface{}) SSHCommands
	// adds new command which will tolerate any error occurred
	IgnoreError(format string, a ...interface{}) SSHCommands
	// WithRetries executes the specified command as a script,
	// retrying several times upon failure
	WithRetries(format string, a ...interface{}) SSHCommands
	// WithLogger sets logger
	WithLogger(logrus.FieldLogger) SSHCommands
	// WithOutput sets the output sink
	WithOutput(io.Writer) SSHCommands
	// executes sequence
	Run(ctx context.Context) error
}

type sshCommands struct {
	client   *ssh.Client
	logger   logrus.FieldLogger
	commands []sshCommand
	output   io.Writer
}

// NewSSHCommands returns a new remote command executor
// that will use the specified runner to execute commands
func NewSSHCommands(client *ssh.Client) SSHCommands {
	return &sshCommands{
		client: client,
		logger: logrus.StandardLogger(),
		output: ioutil.Discard,
	}
}

// C adds a new command specified with format/args
func (c *sshCommands) C(format string, args ...interface{}) SSHCommands {
	c.commands = append(c.commands, sshCommand{
		command:      fmt.Sprintf(format, args...),
		abortOnError: true,
		env: map[string]string{
			defaults.PathEnv: defaults.PathEnvVal,
		}})
	return c
}

// WithRetries adds a new command specified with format/args.
// The command will be retried in a loop as long as it is failed
// or the number of attempts has exceeded
func (c *sshCommands) WithRetries(format string, args ...interface{}) SSHCommands {
	c.commands = append(c.commands, sshCommand{
		command:      fmt.Sprintf(format, args...),
		withRetries:  true,
		abortOnError: true,
		env: map[string]string{
			defaults.PathEnv: defaults.PathEnvVal,
		}})
	return c
}

// IgnoreError adds a new command specified with format/args.
// If the command returns an error, it will be ignored
func (c *sshCommands) IgnoreError(format string, args ...interface{}) SSHCommands {
	c.commands = append(c.commands, sshCommand{
		command:      fmt.Sprintf(format, args...),
		abortOnError: false,
		env: map[string]string{
			defaults.PathEnv: defaults.PathEnvVal,
		}})
	return c
}

// WithLogger configures the logger
func (c *sshCommands) WithLogger(logger logrus.FieldLogger) SSHCommands {
	c.logger = logger
	return c
}

// WithOutput configures the output writer for the command contents
func (c *sshCommands) WithOutput(w io.Writer) SSHCommands {
	c.output = w
	return c
}

// RunCommands executes commands sequentially
func (c *sshCommands) Run(ctx context.Context) (err error) {
	for _, cmd := range c.commands {
		if cmd.withRetries {
			err = c.runWithRetries(ctx, cmd)
		} else {
			err = c.run(ctx, cmd)
		}
		if err != nil {
			log := c.logger.WithFields(logrus.Fields{
				"error":   err,
				"command": cmd,
			})

			if cmd.abortOnError {
				log.Warn("Subcommand failed, sequence interrupted.")
				return trace.Wrap(err, cmd)
			}
			log.Warn("ignoring failed subcommand")
		}
	}
	return nil
}

func (c *sshCommands) runWithRetries(ctx context.Context, cmd sshCommand) error {
	ctx, cancel := context.WithTimeout(ctx, defaults.TransientErrorTimeout)
	defer cancel()
	b := backoff.NewConstantBackOff(defaults.RetryInterval)
	err := RetryWithInterval(ctx, b, func() error {
		err := SSHRunAndParse(ctx,
			c.client, c.logger,
			cmd.command, cmd.env, c.output, ParseDiscard)
		return trace.Wrap(err)
	})
	return trace.Wrap(err)
}

func (c *sshCommands) run(ctx context.Context, cmd sshCommand) error {
	err := SSHRunAndParse(ctx,
		c.client, c.logger,
		cmd.command, cmd.env, c.output, ParseDiscard)
	return trace.Wrap(err)
}

// SSHRunAndParse runs remote SSH command cmd with environment variables set with env.
// parse if set, will be provided the reader that consumes stdout of the command.
// Returns *ssh.ExitError if the command has completed with a non-0 exit code,
// *ssh.ExitMissingError if the other side has terminated the session without providing
// the exit code and nil for no error
func SSHRunAndParse(
	ctx context.Context,
	client *ssh.Client,
	log logrus.FieldLogger,
	cmd string,
	env map[string]string,
	output io.Writer,
	parse OutputParseFn,
) (err error) {
	log = log.WithField("cmd", cmd)

	session, err := client.NewSession()
	if err != nil {
		return trace.Wrap(err)
	}
	defer session.Close()

	var envStrings []string
	for k, v := range env {
		envStrings = append(envStrings, fmt.Sprintf("%v=%v", k, v))
	}

	session.Stdin = new(bytes.Buffer)

	var stdout io.Reader
	if parse != nil {
		// Only create a pipe to remote command's stdout if it's going to be
		// processed, otherwise the remote command might block
		stdout, err = session.StdoutPipe()
		if err != nil {
			return trace.Wrap(err)
		}
	}

	stderr, err := session.StderrPipe()
	if err != nil {
		return trace.Wrap(err)
	}

	sessionCommand := fmt.Sprintf("%s %s", strings.Join(envStrings, " "), cmd)
	err = session.Start(sessionCommand)
	if err != nil {
		return trace.Wrap(err)
	}

	errCh := make(chan error, 2)
	expectErrs := 2
	go func() {
		err := parse(bufio.NewReader(
			io.TeeReader(
				&readLogger{
					log: log.WithField("stream", "stdout"),
					r:   stdout,
				},
				output),
		),
		)
		err = trace.Wrap(err)
		errCh <- err
	}()

	go func() {
		logger := log.WithField("stream", "stderr")
		_, _ = io.Copy(io.MultiWriter(output, NewStderrLogger(logger)), stderr)
	}()

	go func() {
		err := trace.Wrap(session.Wait())
		errCh <- err
	}()

	for i := 0; i < expectErrs; i++ {
		select {
		case <-ctx.Done():
			_ = session.Signal(ssh.SIGTERM)
			log.WithError(ctx.Err()).Debug("Context terminated, sent SIGTERM.")
			return trace.Wrap(ctx.Err())
		case err = <-errCh:
			switch sshError := trace.Unwrap(err).(type) {
			case *ssh.ExitError:
				err = trace.Wrap(sshError)
				log.WithError(err).Debugf("Command %v failed: %v", cmd, sshError.Error())
				return err
			case *ssh.ExitMissingError:
				err = trace.Wrap(sshError)
				log.WithError(err).Debug("Session aborted unexpectedly.")
				return err
			}
			if err != nil {
				err = trace.Wrap(err)
				log.WithError(err).Debug("Unexpected error.")
				return err
			}
		}
	}

	return nil
}

// ParseDiscard returns a no-op parser function that discards the input
func ParseDiscard(r *bufio.Reader) error {
	io.Copy(ioutil.Discard, r) //nolint:errcheck
	return nil
}

// ParseAsString returns a parser function that extracts the stream contents
// as a string
func ParseAsString(out *string) OutputParseFn {
	return func(r *bufio.Reader) error {
		b, err := ioutil.ReadAll(r)
		if err != nil {
			return trace.Wrap(err)
		}
		*out = string(b)
		return nil
	}
}

type readLogger struct {
	log logrus.FieldLogger
	r   io.Reader
}

func (l *readLogger) Read(p []byte) (n int, err error) {
	n, err = l.r.Read(p)
	if err != nil && err != io.EOF {
		l.log.WithError(err).Debug("Unexpected I/O error.")
	} else if n > 0 {
		l.log.Info(string(p[0:n]))
	}
	return n, err
}

// NewStderrLogger returns a new io.Writer that logs its input
// with log.Warn
func NewStderrLogger(log logrus.FieldLogger) *StderrLogger {
	return &StderrLogger{log: log}
}

// StderrLogger is a stdlib-compatible logger that logs to the underlying
// logger at Warn level
type StderrLogger struct {
	log logrus.FieldLogger
}

func (w *StderrLogger) Write(p []byte) (n int, err error) {
	w.log.Warn(string(p))
	return len(p), nil
}
