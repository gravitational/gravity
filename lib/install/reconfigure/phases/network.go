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

package phases

import (
	"context"
	"net"
	"os"
	"os/exec"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// NewNetwork returns executor that cleans up network interfaces on the node.
func NewNetwork(p fsm.ExecutorParams, operator ops.Operator) (*networkExecutor, error) {
	logger := &fsm.Logger{
		FieldLogger: logrus.WithField(constants.FieldPhase, p.Phase.ID),
		Key:         p.Key(),
		Operator:    operator,
		Server:      p.Phase.Data.Server,
	}
	return &networkExecutor{
		FieldLogger:    logger,
		ExecutorParams: p,
	}, nil
}

type networkExecutor struct {
	// FieldLogger is used for logging.
	logrus.FieldLogger
	// ExecutorParams are common executor parameters.
	fsm.ExecutorParams
}

// Execute removes old CNI interfaces.
func (p *networkExecutor) Execute(ctx context.Context) error {
	p.Progress.NextStep("Cleaning up network interfaces")
	ifaces, err := net.Interfaces()
	if err != nil {
		return trace.Wrap(err)
	}
	for _, iface := range ifaces {
		if utils.HasOneOfPrefixes(iface.Name, defaults.NetworkInterfacePrefixes...) {
			err := utils.Exec(exec.Command("ip", "link", "del", iface.Name), os.Stdout)
			if err != nil {
				return trace.Wrap(err)
			}
			p.Infof("Removed inteface %v", iface.Name)
		}
	}
	return nil
}

// Rollback is no-op for this phase.
func (*networkExecutor) Rollback(ctx context.Context) error {
	return nil
}

// PreCheck is no-op for this phase.
func (*networkExecutor) PreCheck(ctx context.Context) error {
	return nil
}

// PostCheck is no-op for this phase.
func (*networkExecutor) PostCheck(ctx context.Context) error {
	return nil
}
