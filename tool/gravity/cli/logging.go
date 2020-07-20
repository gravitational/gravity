/*
Copyright 2020 Gravitational, Inc.

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

package cli

import (
	"fmt"
	"log/syslog"
	"strings"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/utils"
	"github.com/gravitational/gravity/lib/utils/cli"

	"github.com/gravitational/trace"
)

// Executable executes a gravity command.
type Executable func() error

// CmdExecer handles execution of a gravity command.
type CmdExecer struct {
	// Exe specifies an executable gravity command.
	Exe Executable
	// Parser specifies the gravity arguments parser function.
	Parser cli.ArgsParserFunc
	// Args specifies the provided gravity command arguments.
	Args []string
	// ExtraArgs specifies the provided extra arguments.
	ExtraArgs []string
}

// Execute executes the gravity command while logging the start and completion
// of the command.
func (r *CmdExecer) Execute() (err error) {
	sanitizedCmd, err := r.getRedactedCmd()
	if err != nil {
		return trace.Wrap(err)
	}
	cmdString := strings.Join(sanitizedCmd, " ")
	if len(r.ExtraArgs) > 0 {
		cmdString = fmt.Sprintf("%s -- %s", cmdString, strings.Join(r.ExtraArgs, " "))
	}

	logEntry(fmt.Sprintf("[RUNNING]: %s", cmdString))
	defer func() {
		if r := recover(); r != nil {
			logEntry(fmt.Sprintf("[FAILURE]: %s: [PANIC]: %v", cmdString, r))
			panic(r)
		}
		if err != nil {
			logEntry(fmt.Sprintf("[FAILURE]: %s: [ERROR]: %s", cmdString, trace.UserMessage(err)))
			return
		}
		logEntry(fmt.Sprintf("[SUCCESS]: %s", cmdString))
	}()

	err = r.Exe()
	return trace.Wrap(err)
}

// getRedactedCmd removes potentially sensitive data from the args and returns
// the sanitized cmd as a list of strings.
func (r *CmdExecer) getRedactedCmd() (cmd []string, err error) {
	commandArgs := cli.CommandArgs{
		Parser: r.Parser,
		FlagsToReplace: []cli.Flag{
			cli.NewFlag("token", constants.Redacted),
			cli.NewFlag("registry-password", constants.Redacted),
			cli.NewFlag("password", constants.Redacted),
			cli.NewFlag("license", constants.Redacted),
			cli.NewFlag("ops-token", constants.Redacted),
			cli.NewFlag("ops-tunnel-token", constants.Redacted),
			cli.NewFlag("encryption-key", constants.Redacted),
		},
	}
	args, err := commandArgs.Update(r.Args)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return append([]string{utils.Exe.Path}, args...), nil
}

// logEntry writes the provided entry into the system journal with the
// gravity-cli tag.
func logEntry(entry string) {
	if err := utils.SyslogWrite(syslog.LOG_INFO, entry, constants.GravityCLITag); err != nil {
		log.WithError(err).Warn("Failed to write to system logs.")
	}
}
