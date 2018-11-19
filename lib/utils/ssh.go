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
	"text/template"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/trace"

	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

type OutputParseFn func(r *bufio.Reader) error

const (
	exitStatusUndefined = -1
	exitCode            = "exit"
)

type sshCommand struct {
	command      string
	abortOnError bool
	env          map[string]string
}

type SSHCommands interface {
	// adds new command with default policy
	C(format string, a ...interface{}) SSHCommands
	// adds new command which will tolerate any error occured
	IgnoreError(format string, a ...interface{}) SSHCommands
	// WithRetries executes the specified command as a script,
	// retrying several times upon failure
	WithRetries(format string, a ...interface{}) SSHCommands
	// WithLogger sets logger
	WithLogger(logrus.FieldLogger) SSHCommands
	// executes sequence
	Run(ctx context.Context) error
}

type sshCommands struct {
	client   *ssh.Client
	logger   logrus.FieldLogger
	commands []sshCommand
	err      error
}

func NewSSHCommands(client *ssh.Client) SSHCommands {
	return &sshCommands{
		client: client,
		logger: logrus.StandardLogger()}
}

func (c *sshCommands) C(format string, args ...interface{}) SSHCommands {
	if c.err != nil {
		return c
	}
	c.commands = append(c.commands, sshCommand{
		command:      fmt.Sprintf(format, args...),
		abortOnError: true,
		env: map[string]string{
			defaults.PathEnv: defaults.PathEnvVal,
		}})
	return c
}

func (c *sshCommands) WithRetries(format string, args ...interface{}) SSHCommands {
	if c.err != nil {
		return c
	}
	var cmd bytes.Buffer
	err := scriptTemplate.Execute(&cmd, map[string]string{
		"command": fmt.Sprintf(format, args...),
	})
	if err != nil {
		c.err = err
	}
	c.commands = append(c.commands, sshCommand{
		command:      cmd.String(),
		abortOnError: true,
		env: map[string]string{
			defaults.PathEnv: defaults.PathEnvVal,
		}})
	return c
}

func (c *sshCommands) IgnoreError(format string, args ...interface{}) SSHCommands {
	if c.err != nil {
		return c
	}
	c.commands = append(c.commands, sshCommand{
		command:      fmt.Sprintf(format, args...),
		abortOnError: false,
		env: map[string]string{
			defaults.PathEnv: defaults.PathEnvVal,
		}})
	return c
}

func (c *sshCommands) WithLogger(logger logrus.FieldLogger) SSHCommands {
	c.logger = logger
	return c
}

// RunCommands executes commands sequentially
func (c *sshCommands) Run(ctx context.Context) error {
	if c.err != nil {
		return trace.Wrap(c.err)
	}
	for _, cmd := range c.commands {
		_, err := SSHRunAndParse(ctx,
			c.client, c.logger,
			cmd.command, cmd.env, ParseDiscard)
		if err != nil {
			log := c.logger.WithFields(logrus.Fields{
				"error":   err,
				"command": cmd,
			})

			if cmd.abortOnError {
				log.Error("subcommand failed, sequence interrupted")
				return trace.Wrap(err, cmd)
			}
			log.Warn("ignoring failed subcommand")
		}
	}
	return nil
}

// Run is a simple method to run external program and don't care about its output or exit status
func SSHRun(ctx context.Context, client *ssh.Client, log logrus.FieldLogger, cmd string, env map[string]string) error {
	exit, err := SSHRunAndParse(ctx, client, log, cmd, env, ParseDiscard)
	if err != nil {
		return trace.Wrap(err, cmd)
	}

	if exit != 0 {
		return trace.Errorf("%s returned %d", cmd, exit)
	}

	return nil
}

// RunAndParse runs remote SSH command with environment variables set by `env`
// exitStatus is -1 if undefined
func SSHRunAndParse(ctx context.Context, client *ssh.Client, log logrus.FieldLogger, cmd string, env map[string]string, parse OutputParseFn) (exitStatus int, err error) {
	session, err := client.NewSession()
	if err != nil {
		return exitStatusUndefined, trace.Wrap(err)
	}
	defer session.Close()

	envStrings := []string{}
	if env != nil {
		for k, v := range env {
			envStrings = append(envStrings, fmt.Sprintf("%s=%s", k, v))
		}
	}

	session.Stdin = new(bytes.Buffer)

	stdout, err := session.StdoutPipe()
	if err != nil {
		return exitStatusUndefined, trace.Wrap(err)
	}

	stderr, err := session.StderrPipe()
	if err != nil {
		return exitStatusUndefined, trace.Wrap(err)
	}

	log = log.WithField("cmd", cmd)

	errCh := make(chan error, 2)
	expectErrs := 1
	if parse != nil {
		expectErrs++
		go func() {
			err := parse(bufio.NewReader(
				&readLogger{log.WithField("stream", "stdout"), stdout}))
			errCh <- trace.Wrap(err)
		}()
	}

	go func() {
		log.Debug("")
		errCh <- session.Run(fmt.Sprintf("%s %s", strings.Join(envStrings, " "), cmd))
	}()

	go func() {
		r := bufio.NewReader(stderr)
		stderrLog := log.WithField("stream", "stderr")
		for {
			line, err := r.ReadString('\n')
			if line != "" {
				stderrLog.Debug(line)
			}
			if parse == nil {
				session.Close()
				errCh <- nil // FIXME : this is a hack; session closure does not unblock session.Run() wonder if there's a better way
				return
			}
			if err != nil {
				return
			}
		}
	}()

	for i := 0; i < expectErrs; i++ {
		select {
		case <-ctx.Done():
			session.Signal(ssh.SIGTERM)
			log.WithError(ctx.Err()).Debug("context terminated, sent SIGTERM")
			return exitStatusUndefined, err
		case err = <-errCh:
			if exitErr, isExitErr := err.(*ssh.ExitError); isExitErr {
				err = trace.Wrap(exitErr)
				log.WithError(err).Debug("")
				return exitErr.ExitStatus(), err
			}
			if err != nil {
				err = trace.Wrap(err)
				log.WithError(err).Debug("unexpected error")
				return exitStatusUndefined, err
			}
		}
	}
	return 0, nil
}

func ParseDiscard(r *bufio.Reader) error {
	io.Copy(ioutil.Discard, r)
	return nil
}

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
		l.log.WithError(err).Debug("unexpected I/O error")
	} else if n > 0 {
		l.log.Debug(string(p[0:n]))
	}
	return n, err
}

type stderrLogger struct {
	log logrus.FieldLogger
}

// StderrWriter returns io.Writer which would log with log.Warn
func StderrLogger(log logrus.FieldLogger) io.Writer {
	return &stderrLogger{log}
}

func (w *stderrLogger) Write(p []byte) (n int, err error) {
	w.log.Warn(string(p))
	return len(p), nil
}

var scriptTemplate = template.Must(template.New("script").
	Parse(`/bin/bash -c 'for i in $(seq 1 20); do {{.command}} && break || sleep 5; done'`))
