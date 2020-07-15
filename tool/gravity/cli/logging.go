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
func LogCLICompleted(cmd string, err error) {
	var entry string
	if err != nil {
		entry = fmt.Sprintf("[FAILURE]: %s: [ERROR]: %s", cmd, trace.UserMessage(err))
	} else {
		entry = fmt.Sprintf("[SUCCESS]: %s", cmd)
	}
	if err := utils.SyslogWrite(syslog.LOG_INFO, entry, constants.GravityCLITag); err != nil {
		log.WithError(err).Warn("Failed to write to system logs.")
	}
}

// SanitizeCmd removes potentially sensitive data from the cmdString.
func SanitizeCmd(g *Application, cmd, cmdString string) string {
	switch cmd {
	case g.InstallCmd.FullCommand():
		if *g.InstallCmd.Token != "" {
			cmdString = Redact(cmdString, *g.InstallCmd.Token)
		}
	case g.JoinCmd.FullCommand():
		if *g.JoinCmd.Token != "" {
			cmdString = Redact(cmdString, *g.JoinCmd.Token)
		}
	case g.AutoJoinCmd.FullCommand():
		if *g.AutoJoinCmd.Token != "" {
			cmdString = Redact(cmdString, *g.AutoJoinCmd.Token)
		}
	case g.WizardCmd.FullCommand():
		if *g.WizardCmd.Token != "" {
			cmdString = Redact(cmdString, *g.WizardCmd.Token)
		}
	case g.OpsAgentCmd.FullCommand():
		if *g.OpsAgentCmd.Token != "" {
			cmdString = Redact(cmdString, *g.OpsAgentCmd.Token)
		}
	case g.APIKeyDeleteCmd.FullCommand():
		if *g.APIKeyDeleteCmd.Token != "" {
			cmdString = Redact(cmdString, *g.APIKeyDeleteCmd.Token)
		}
	case g.AppInstallCmd.FullCommand():
		if *g.AppInstallCmd.RegistryPassword != "" {
			cmdString = Redact(cmdString, *g.AppInstallCmd.RegistryPassword)
		}
	case g.AppUpgradeCmd.FullCommand():
		if *g.AppUpgradeCmd.RegistryPassword != "" {
			cmdString = Redact(cmdString, *g.AppUpgradeCmd.RegistryPassword)
		}
	case g.AppSyncCmd.FullCommand():
		if *g.AppSyncCmd.RegistryPassword != "" {
			cmdString = Redact(cmdString, *g.AppSyncCmd.RegistryPassword)
		}
	case g.OpsConnectCmd.FullCommand():
		if *g.OpsConnectCmd.Password != "" {
			cmdString = Redact(cmdString, *g.OpsConnectCmd.Password)
		}
	case g.UserCreateCmd.FullCommand():
		if *g.UserCreateCmd.Password != "" {
			cmdString = Redact(cmdString, *g.UserCreateCmd.Password)
		}
	}

	return cmdString
}

// Redact replaces any instances of value found in cmd with the redacted string.
func Redact(cmd, value string) string {
	return strings.ReplaceAll(cmd, value, constants.Redacted)
}
