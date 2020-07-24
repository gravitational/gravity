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

package client

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"

	validationpb "github.com/gravitational/gravity/lib/network/validation/proto"
	pb "github.com/gravitational/gravity/lib/rpc/proto"

	"github.com/gravitational/satellite/agent/proto/agentpb"
	"github.com/gravitational/trace"

	"github.com/gogo/protobuf/types"
	"github.com/sirupsen/logrus"
)

// Command executes the command specified with args on remote node
func (c *client) Command(ctx context.Context, log logrus.FieldLogger, stdout, stderr io.Writer, args ...string) error {
	err := c.command(ctx, log, stdout, stderr, &pb.CommandArgs{
		Args: args,
	})
	return trace.Wrap(err)
}

// GravityCommand executes the gravity command specified with args on remote node.
// The command uses the same gravity binary that runs the agent.
func (c *client) GravityCommand(ctx context.Context, log logrus.FieldLogger, stdout, stderr io.Writer, args ...string) error {
	err := c.command(ctx, log, stdout, stderr, &pb.CommandArgs{
		SelfCommand: true,
		Args:        args,
	})
	return trace.Wrap(err)
}

// Validate validates the node against the specified manifest and profile.
// Returns the list of failed probes
func (c *client) Validate(ctx context.Context, req *validationpb.ValidateRequest) ([]*agentpb.Probe, error) {
	resp, err := c.validation.Validate(ctx, req)
	if resp != nil {
		return resp.Failed, trace.Wrap(err)
	}
	return nil, trace.Wrap(err)
}

// Shutdown requests remote agent to quit
func (c *client) Shutdown(ctx context.Context, req *pb.ShutdownRequest) error {
	_, err := c.agent.Shutdown(ctx, req)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(c.Close())
}

// Abort requests remote agent to abort operation
func (c *client) Abort(ctx context.Context) error {
	_, err := c.agent.Abort(ctx, &types.Empty{})
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(c.Close())
}

func (c *client) command(ctx context.Context, log logrus.FieldLogger, stdout, stderr io.Writer, args *pb.CommandArgs) error {
	if len(args.Args) < 1 {
		return trace.BadParameter("at least one argument is required")
	}

	out, err := c.agent.Command(ctx, args)
	if err != nil {
		return trace.Wrap(err)
	}

	err = processStream(out, log, stdout, stderr)
	return trace.Wrap(err)
}

type streamContext struct {
	commands map[int32][]string
	log      logrus.FieldLogger
}

func processStream(stream pb.IncomingMessageStream, log logrus.FieldLogger, stdout, stderr io.Writer) error {
	streamCtx := &streamContext{map[int32][]string{}, log}
	if stdout == nil {
		stdout = ioutil.Discard
	}
	if stderr == nil {
		stderr = ioutil.Discard
	}
	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return trace.Wrap(err)
		}

		switch elem := msg.Element.(type) {
		case *pb.Message_ExecOutput:
			err = trace.Wrap(streamCtx.processExecOutput(elem.ExecOutput, stdout, stderr))
		case *pb.Message_ExecStarted:
			err = trace.Wrap(streamCtx.processExecStarted(elem.ExecStarted))
		case *pb.Message_ExecCompleted:
			err = trace.Wrap(streamCtx.processExecCompleted(elem.ExecCompleted))
		case *pb.Message_LogEntry:
			err = trace.Wrap(streamCtx.processLogEntry(elem.LogEntry))
		case *pb.Message_Error:
			err = trace.Wrap(streamCtx.processError(elem.Error))
		default:
			err = trace.BadParameter("unexpected message %+v", msg.Element)
		}

		if err != nil {
			log.WithError(err).Error("error processing stream")
		}
	}
}

func (s *streamContext) processExecOutput(msg *pb.ExecOutput, stdout, stderr io.Writer) error {
	entry := s.log

	args, ok := s.commands[msg.Seq]
	if ok && len(args) > 0 {
		entry = s.log.WithField("CMD", fmt.Sprintf("%s#%d", args[0], msg.Seq))
	}

	switch msg.Fd {
	case pb.ExecOutput_STDOUT:
		if _, err := stdout.Write(msg.Data); err != nil {
			entry.WithError(err).Warn("failed to output to stdout")
		}
		entry.Infof("%q", msg.Data)
	case pb.ExecOutput_STDERR:
		if _, err := stderr.Write(msg.Data); err != nil {
			entry.WithError(err).Warn("failed to output to stderr")
		}
		entry.Warnf("%q", msg.Data)
	default:
		return trace.BadParameter("unexpected output descriptor value %v", msg.Fd)
	}
	return nil
}

func (s *streamContext) processExecStarted(msg *pb.ExecStarted) error {
	s.commands[msg.Seq] = msg.Args
	s.log.WithFields(logrus.Fields{trace.Component: "rpc",
		"seq": msg.Seq,
	}).Debugf("Run %q.", msg.Args)
	return nil
}

func (s *streamContext) processExecCompleted(msg *pb.ExecCompleted) error {
	s.log.WithFields(logrus.Fields{trace.Component: "rpc",
		"seq":  msg.Seq,
		"exit": msg.ExitCode,
	}).Debug("Completed.")
	return nil
}

func (s *streamContext) processLogEntry(msg *pb.LogEntry) error {
	fields := logrus.Fields{}
	for k, v := range msg.Fields {
		fields[k] = v
	}
	if len(msg.Traces) > 0 {
		fields["FILE"] = msg.Traces[0]
	}

	entry := s.log.WithFields(fields)

	switch msg.Level {
	case pb.LogEntry_Debug:
		entry.Debug(msg.Message)
	case pb.LogEntry_Info:
		entry.Info(msg.Message)
	case pb.LogEntry_Warn:
		entry.Warning(msg.Message)
	case pb.LogEntry_Error:
		entry.Error(msg.Message)
	default:
		entry.Error(msg.Message)
	}

	return nil
}

func (s *streamContext) processError(msg *pb.Error) error {
	s.log.Error(msg.Message)
	return nil
}
