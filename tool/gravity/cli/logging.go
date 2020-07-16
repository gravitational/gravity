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

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/utils"
	"github.com/gravitational/gravity/lib/utils/cli"

	"github.com/gravitational/trace"
)

// LogCLIRunning writes the running cmd as a log entry into the system journal
// with the gravity-cli tag.
func LogCLIRunning(cmd string) {
	entry := fmt.Sprintf("[RUNNING]: %s", cmd)
	if err := utils.SyslogWrite(syslog.LOG_INFO, entry, constants.GravityCLITag); err != nil {
		log.WithError(err).Warn("Failed to write to system logs.")
	}
}

// LogCLICompleted writes the completed cmd as a log entry into the system journal
// with the gravity-cli tag. Failed commands will be logged with the returned
// error.
func LogCLICompleted(cmd, err string) {
	var entry string
	if err != "" {
		entry = fmt.Sprintf("[FAILURE]: %s: [ERROR]: %s", cmd, err)
	} else {
		entry = fmt.Sprintf("[SUCCESS]: %s", cmd)
	}
	if err := utils.SyslogWrite(syslog.LOG_INFO, entry, constants.GravityCLITag); err != nil {
		log.WithError(err).Warn("Failed to write to system logs.")
	}
}

// RedactCmd removes potentially sensitive data from the args and returns the
// sanitized cmd as a list of strings.
func RedactCmd(args ...string) (cmd []string, err error) {
	commandArgs := cli.CommandArgs{
		Parser: cli.ArgsParserFunc(parseArgs),
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
	args, err = commandArgs.Update(args)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return append([]string{utils.Exe.Path}, args...), nil
}
