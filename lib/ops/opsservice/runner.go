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

package opsservice

import (
	"bytes"
	"context"
	"io"
	"os/exec"
	"strings"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

type remoteRunner interface {
	// Run runs the provided command on the specified server
	Run(server remoteServer, args ...string) ([]byte, error)
	// RunStream runs the provided command on the specified server and streams output to w
	RunStream(server remoteServer, stdout, stderr io.Writer, args ...string) error
	// RunCmd runs the provided command on the specified server and logs
	// its results into the operation context
	RunCmd(operationContext, remoteServer, Command) ([]byte, error)
}

type remoteServer interface {
	Address() string
	HostName() string
	Debug() string
}

type teleportRunner struct {
	log.FieldLogger
	domainName string
	ops.TeleportProxyService
}

// RunStream runs the provided command on the specified server and streams output to w
func (r *teleportRunner) RunStream(server remoteServer, stdout, stderr io.Writer, args ...string) error {
	command := strings.Join(args, " ")
	err := r.ExecuteCommand(context.TODO(), r.domainName, server.Address(), command, stdout, stderr)

	logger := r.WithFields(log.Fields{
		constants.FieldServer:             server.Address(),
		constants.FieldCommandError:       (err != nil),
		constants.FieldCommandErrorReport: trace.UserMessage(err),
	})
	logger.Info(command)

	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// Run runs the provided command on the specified server
func (r *teleportRunner) Run(server remoteServer, args ...string) ([]byte, error) {
	var out bytes.Buffer
	err := r.RunStream(server, &out, &out, args...)
	if err != nil {
		return out.Bytes(), trace.Wrap(err, out.String())
	}
	return out.Bytes(), nil
}

// RunCmd runs the provided command on the specified server and logs
// its results into the operation context
func (r *teleportRunner) RunCmd(ctx operationContext, server remoteServer, cmd Command) ([]byte, error) {
	out, err := cmd.Run(ctx, r, server)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return out, nil
}

type agentRunner struct {
	ctx *operationContext
	ops.AgentService
}

// RunStream runs the provided command on the specified server and streams output to w
func (r *agentRunner) RunStream(server remoteServer, stdout, stderr io.Writer, args ...string) error {
	err := r.AgentService.ExecNoLog(context.TODO(), r.ctx.key(), server.Address(), args, stdout, stderr)

	entry := r.ctx.WithFields(log.Fields{
		constants.FieldServer:             server.Address(),
		constants.FieldCommandError:       (err != nil),
		constants.FieldCommandErrorReport: trace.UserMessage(err),
	})
	command := strings.Join(args, " ")
	entry.Info(command)

	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// Run runs the provided command on the specified server
func (r *agentRunner) Run(server remoteServer, args ...string) ([]byte, error) {
	var out bytes.Buffer
	err := r.RunStream(server, &out, &out, args...)
	if err != nil {
		return out.Bytes(), trace.Wrap(err)
	}
	return out.Bytes(), nil
}

// RunCmd runs the provided command on the specified server and logs
// its results into the operation context
func (r *agentRunner) RunCmd(ctx operationContext, server remoteServer, cmd Command) ([]byte, error) {
	out, err := cmd.Run(ctx, r, server)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return out, nil
}

// serverRunner runs commands on the server it was initialized with,
type serverRunner struct {
	server remoteServer
	runner remoteRunner
}

// RunStream runs the provided command and streams output to w
func (r *serverRunner) RunStream(stdout, stderr io.Writer, args ...string) error {
	return r.runner.RunStream(r.server, stdout, stderr, args...)
}

// Run runs the provided command
func (r *serverRunner) Run(args ...string) ([]byte, error) {
	return r.runner.Run(r.server, args...)
}

// RunCmd runs the provided command and logs its results into the operation context
func (r *serverRunner) RunCmd(ctx operationContext, cmd Command) ([]byte, error) {
	return r.runner.RunCmd(ctx, r.server, cmd)
}

// localRunner runs commands locally, implements commandRunner
type localRunner struct {
}

func (r *localRunner) RunStream(w io.Writer, args ...string) error {
	var cmd *exec.Cmd
	if len(args) > 1 {
		cmd = exec.Command(args[0], args[1:]...)
	} else {
		cmd = exec.Command(args[0])
	}
	return trace.Wrap(utils.Exec(cmd, w))
}

func (r *localRunner) Run(args ...string) ([]byte, error) {
	var b bytes.Buffer
	err := r.RunStream(&b, args...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return b.Bytes(), nil
}

func (r *localRunner) RunCmd(ctx operationContext, cmd Command) ([]byte, error) {
	out, err := r.Run(cmd.Args...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return out, nil
}
