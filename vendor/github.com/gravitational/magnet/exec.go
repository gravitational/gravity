package magnet

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/gravitational/magnet/pkg/progressui"
	"github.com/gravitational/trace"
	"github.com/opencontainers/go-digest"
)

type ExecConfig struct {
	target *MagnetTarget
	env    map[string]string
	wd     string
}

// Exec is used to build and run a command on the system.
func (m *MagnetTarget) Exec() *ExecConfig {
	return &ExecConfig{
		target: m,
	}
}

// SetEnv is used to add environment variables to the exec'd commands environment.
func (e *ExecConfig) SetEnv(key, value string) *ExecConfig {
	if e.env == nil {
		e.env = make(map[string]string)
	}

	e.env[key] = value

	return e
}

// SetEnvs is used to add environment variables to the execed commands environment.
func (e *ExecConfig) SetEnvs(env map[string]string) *ExecConfig {
	if e.env == nil {
		e.env = make(map[string]string)
	}
	for key, value := range env {
		e.env[key] = value
	}

	return e
}

// SetWD is used to set the working directory of the command to be executed
func (e *ExecConfig) SetWD(wd string) *ExecConfig {
	e.wd = wd

	return e
}

// Run runs the provided command
// based on https://github.com/magefile/mage/blob/310e198ebd9303cd2c876d96e79de954915f60a7/sh/cmd.go#L92
func (e *ExecConfig) Run(ctx context.Context, cmd string, args ...string) (bool, error) {
	expand := func(s string) string {
		s2, ok := e.env[s]
		if ok {
			return s2
		}

		return os.Getenv(s)
	}

	cmd = os.Expand(cmd, expand)

	for i := range args {
		args[i] = os.Expand(args[i], expand)
	}

	stdout, stderr := outStreams(e.target.vertex.Digest, e.target.root.status)

	if len(e.env) > 0 {
		e.target.Println("Env: ", e.env, " Exec: ", fmt.Sprint(cmd, " ", strings.Join(args, " ")))
	} else {
		e.target.Println("Exec: ", fmt.Sprint(cmd, " ", strings.Join(args, " ")))
	}

	ran, err := run(ctx, e.env, stdout, stderr, e.wd, cmd, args...)

	return ran, trace.Wrap(err)
}

// Outout runs the provided command, returning the output
// Note: output / trace won't be present in magnet logs
func Output(ctx context.Context, cmd string, args ...string) (string, error) {
	buf := &bytes.Buffer{}
	_, err := run(ctx, nil, buf, buf, "", cmd, args...)
	return strings.TrimSuffix(buf.String(), "\n"), err
}

// based on https://github.com/magefile/mage/blob/310e198ebd9303cd2c876d96e79de954915f60a7/sh/cmd.go#L126
func run(ctx context.Context, env map[string]string, stdout, stderr io.Writer, wd, cmd string, args ...string) (ran bool, err error) {
	c := exec.CommandContext(ctx, cmd, args...)
	c.Env = os.Environ()

	for k, v := range env {
		c.Env = append(c.Env, k+"="+v)
	}

	c.Stderr = stderr
	c.Stdout = stdout
	c.Stdin = os.Stdin
	c.Dir = wd

	err = c.Run()

	if err == nil {
		return true, nil
	}

	if CmdRan(err) {
		return true, trace.ConvertSystemError(err)
	}

	return false, trace.ConvertSystemError(err)
}

// CmdRan examines the error to determine if it was generated as a result of a
// command running via os/exec.Command.  If the error is nil, or the command ran
// (even if it exited with a non-zero exit code), CmdRan reports true.  If the
// error is an unrecognized type, or it is an error from exec.Command that says
// the command failed to run (usually due to the command not existing or not
// being executable), it reports false.
// based on https://github.com/magefile/mage/blob/310e198ebd9303cd2c876d96e79de954915f60a7/sh/cmd.go#L140
func CmdRan(err error) bool {
	if err == nil {
		return true
	}

	ee, ok := err.(*exec.ExitError)
	if ok {
		return ee.Exited()
	}

	return false
}

type exitStatus interface {
	ExitStatus() int
}

// ExitStatus returns the exit status of the error if it is an exec.ExitError
// or if it implements ExitStatus() int.
// 0 if it is nil or 1 if it is a different error.
// based on https://github.com/magefile/mage/blob/310e198ebd9303cd2c876d96e79de954915f60a7/sh/cmd.go#L161
func ExitStatus(err error) int {
	if err == nil {
		return 0
	}

	if e, ok := err.(exitStatus); ok {
		return e.ExitStatus()
	}

	if e, ok := err.(*exec.ExitError); ok {
		if ex, ok := e.Sys().(exitStatus); ok {
			return ex.ExitStatus()
		}
	}

	return 1
}

const STDOUT = 1
const STDERR = 2

type streamWriter struct {
	vertex digest.Digest
	stream int //1 = stdout, 2 = stderr
	status chan *progressui.SolveStatus
}

func outStreams(d digest.Digest, status chan *progressui.SolveStatus) (stdout io.WriteCloser, stderr io.WriteCloser) {
	return &streamWriter{
			stream: STDOUT,
			vertex: d,
			status: status,
		}, &streamWriter{
			stream: STDERR,
			vertex: d,
			status: status,
		}
}

// Write implementation for WriteCloser.
func (sw *streamWriter) Write(dt []byte) (int, error) {
	sw.status <- &progressui.SolveStatus{
		Logs: []*progressui.VertexLog{
			{
				Vertex:    sw.vertex,
				Stream:    sw.stream,
				Data:      append([]byte{}, dt...),
				Timestamp: time.Now(),
			},
		},
	}

	return len(dt), nil
}

// Close implementation for WriteCloser.
func (sw *streamWriter) Close() error {
	return nil
}
