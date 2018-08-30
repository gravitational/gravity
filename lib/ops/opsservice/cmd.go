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
