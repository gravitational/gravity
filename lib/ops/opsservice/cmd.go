package opsservice

import (
	"fmt"

	"github.com/gravitational/trace"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/utils"
)

// Command is a command to execute, includes arguments
// and user-facing description
type Command struct {
	// Args is a slice of command arguments
	Args []string
	// Description is a user friendly description of this command
	Description string
	// Retry indicates whether the command should be retried upon failure
	Retry bool
}

// Cmd returns new instance of command with user facing description
func Cmd(args []string, format string, formatArgs ...interface{}) Command {
	return Command{Args: args, Description: fmt.Sprintf(format, formatArgs...)}
}

// RetryCmd returns a new command that should be retried upon failure
func RetryCmd(args []string, format string, formatArgs ...interface{}) Command {
	return Command{Args: args, Description: fmt.Sprintf(format, formatArgs...), Retry: true}
}

// Run runs the command on the specified server using the provided runner
func (c Command) Run(ctx operationContext, runner remoteRunner, server remoteServer) (out []byte, err error) {
	if c.Retry {
		err = utils.Retry(defaults.RetryInterval, defaults.RetryLessAttempts, func() error {
			out, err = runner.Run(server, c.Args...)
			if err != nil {
				ctx.RecordWarn("[%v] retrying: %v", server.Address(), c.Description)
				return trace.Wrap(err)
			}
			return nil
		})
	} else {
		out, err = runner.Run(server, c.Args...)
	}
	if err != nil {
		ctx.RecordError("[%v] %v", server.Address(), c.Description)
		ctx.Errorf("%v: %s %v", c.Args, out, trace.DebugReport(err))
		return nil, trace.Wrap(err)
	}
	ctx.RecordInfo("[%v] %v", server.Address(), c.Description)
	ctx.Infof("%v: %s", c.Args, out)
	return out, nil
}
