// Copyright 2021 Gravitational Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package install

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/gravitational/gravity/e/lib/ops"
	ossops "github.com/gravitational/gravity/lib/ops"

	"github.com/fatih/color"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// RegisterAgentParams combines parameters needed for registering
// agent with an Ops Center
type RegisterAgentParams struct {
	// Context can be used to cancel registration loop
	Context context.Context
	// Request is the registration request to send to Ops Center
	Request ops.RegisterAgentRequest
	// OriginalResponse is the response to the first registration attempt
	OriginalResponse ops.RegisterAgentResponse
	// Operator is the Ops Center ops service
	Operator ops.Operator
}

// RegisterAgentLoop makes a request to an Ops Center periodically to register
// this agent process in an install group.
//
// It is used in Ops Center initiated installations to help install agents
// distribute the roles of installer/joining agents between themselves. The
// registration loop is canceled when the operation starts.
func RegisterAgentLoop(p RegisterAgentParams) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	logrus.Info("Agent register loop started.")
	for {
		select {
		case <-ticker.C:
			response, err := p.Operator.RegisterAgent(p.Request)
			if err != nil {
				logrus.Errorf("Failed to register agent: %v.",
					trace.DebugReport(err))
				continue
			}
			logrus.Debugf("%s.", response)
			// see if the operation has started yet, and if it has,
			// then cancel the registration loop
			op, err := p.Operator.GetSiteOperation(p.Request.SiteOperationKey())
			if err != nil {
				logrus.Errorf("Failed to get operation: %v.",
					trace.DebugReport(err))
				continue
			} else if op.State == ossops.OperationStateInstallDeploying {
				logrus.Info("Operation has started, canceling registration loop.")
				return
			}
			// if the installer ID has changed, it means the original
			// installer agent has exited and since we cannot easily
			// reconnect, just exit
			if response.InstallerID != p.OriginalResponse.InstallerID {
				fmt.Println(color.RedString("Installer agent at %v has exited.",
					p.OriginalResponse.InstallerIP))
				os.Exit(255)
			}
		case <-p.Context.Done():
			logrus.Debug("Agent register loop exited.")
			return
		}
	}
}
