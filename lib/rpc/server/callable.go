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

package server

import (
	"os/exec"
	"sync/atomic"
	"syscall"

	pb "github.com/gravitational/gravity/lib/rpc/proto"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
)

const (
	// ExitCodeUndefined specifies the value of the exit code when the real exit code is unknown
	ExitCodeUndefined = -1
)

func osExec(ctx context.Context, stream pb.OutgoingMessageStream, args []string, log log.FieldLogger) error {
	cmd := &osCommand{}
	return trace.Wrap(cmd.exec(ctx, stream, args, log))
}

// exec executes the command specified with args streaming stdout/stderr to stream
// TODO: separate RPC failures (like failure to send messages to the stream) from command errors
func (c *osCommand) exec(ctx context.Context, stream pb.OutgoingMessageStream, args []string, log log.FieldLogger) error {
	seq := atomic.AddInt32(&c.seq, 1)
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Stdout = &streamWriter{stream, pb.ExecOutput_STDOUT, seq}
	cmd.Stderr = &streamWriter{stream, pb.ExecOutput_STDERR, seq}

	err := cmd.Start()
	if err != nil {
		return trace.Wrap(err, "failed to start").AddField("path", cmd.Path)
	}

	err = stream.Send(&pb.Message{Element: &pb.Message_ExecStarted{ExecStarted: &pb.ExecStarted{
		Args: args,
		Seq:  seq,
	}}})
	if err != nil {
		log.WithError(err).Warn("Failed to notify stream of command start.")
	}
	err = cmd.Wait()
	if err == nil {
		if err := stream.Send(&pb.Message{
			Element: &pb.Message_ExecCompleted{
				ExecCompleted: &pb.ExecCompleted{
					Seq: seq,
				},
			},
		}); err != nil {
			log.WithError(err).Warn("Failed to notify stream of command completion.")
		}
		return nil
	}

	exitCode := ExitCodeUndefined
	if errExit, ok := err.(*exec.ExitError); ok {
		if status, ok := errExit.Sys().(syscall.WaitStatus); ok {
			exitCode = status.ExitStatus()
		}
	}

	if err := stream.Send(&pb.Message{Element: &pb.Message_ExecCompleted{ExecCompleted: &pb.ExecCompleted{
		Seq:      seq,
		ExitCode: int32(exitCode),
		Error:    pb.EncodeError(trace.Wrap(err)),
	}}}); err != nil {
		log.WithError(err).Warn("Failed to notify stream of command completion with error.")
	}
	return utils.NewExitCodeError(exitCode)
}

type osCommand struct {
	seq int32
}

// streamWriter implements io.Writer and forwards the data to the underlying stream
type streamWriter struct {
	stream pb.OutgoingMessageStream
	fd     pb.ExecOutput_FD
	seq    int32
}

func (s *streamWriter) Write(p []byte) (n int, err error) {
	data := &pb.ExecOutput{
		Fd:   s.fd,
		Data: p,
		Seq:  s.seq,
	}

	err = s.stream.Send(&pb.Message{Element: &pb.Message_ExecOutput{ExecOutput: data}})
	if err != nil {
		return 0, err
	}

	return len(p), nil
}

func (r execFunc) exec(ctx context.Context, stream pb.OutgoingMessageStream, args []string, logger log.FieldLogger) error {
	return r(ctx, stream, args, logger)
}

type execFunc func(ctx context.Context, stream pb.OutgoingMessageStream, args []string, logger log.FieldLogger) error

type commandExecutor interface {
	// exec executes a local command specified with args and streams
	// output into the specified stream.
	// Returns an error if the command execution was insuccessful
	exec(ctx context.Context, stream pb.OutgoingMessageStream, args []string, logger log.FieldLogger) error
}
