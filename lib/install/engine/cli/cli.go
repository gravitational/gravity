/*
Copyright 2019 Gravitational, Inc.

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

// package cli implements command line installer workflow
package cli

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/gravitational/gravity/lib/checks"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/install"
	"github.com/gravitational/gravity/lib/install/dispatcher"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/state"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/system/environ"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/fatih/color"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// New returns a new installer that implements non-interactive installation
// workflow.
//
// The installer can optionally run an agent to include the host node
// in the resulting cluster
func New(config Config) (*Engine, error) {
	if err := config.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &Engine{
		Config: config,
	}, nil
}

func (r *Config) checkAndSetDefaults() error {
	if r.FieldLogger == nil {
		return trace.BadParameter("FieldLogger is required")
	}
	if r.Operator == nil {
		return trace.BadParameter("Operator is required")
	}
	return nil
}

// Config defines the installer configuration
type Config struct {
	// FieldLogger is the logger for the installer
	log.FieldLogger
	// Operator specifies the service operator
	ops.Operator
}

// Execute executes the installer steps.
// Implements installer.Engine
func (r *Engine) Execute(ctx context.Context, installer install.Interface, config install.Config) (dispatcher.Status, error) {
	err := r.execute(ctx, installer, config)
	if err != nil {
		return dispatcher.StatusUnknown, trace.Wrap(err)
	}
	return dispatcher.StatusCompleted, nil
}

func (r *Engine) execute(ctx context.Context, installer install.Interface, config install.Config) (err error) {
	e := executor{
		Config:    r.Config,
		Interface: installer,
		ctx:       ctx,
		config:    config,
	}
	if err := e.bootstrap(); err != nil {
		return trace.Wrap(err)
	}
	operation, err := e.upsertClusterAndOperation()
	if err != nil {
		return trace.Wrap(err)
	}
	if err := installer.NotifyOperationAvailable(*operation); err != nil {
		return trace.Wrap(err)
	}
	err = e.waitForAgents(*operation)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := installer.ExecuteOperation(operation.Key()); err != nil {
		return trace.Wrap(err)
	}
	if err := installer.CompleteFinalInstallStep(operation.Key(), 0); err != nil {
		r.WithError(err).Warn("Failed to complete final install step.")
	}
	if err := installer.CompleteOperation(*operation); err != nil {
		r.WithError(err).Warn("Failed to finalize install.")
	}
	return nil
}

// bootstrap prepares for the installation
func (r *executor) bootstrap() error {
	err := install.InstallBinary(r.config.ServiceUser.UID, r.config.ServiceUser.GID, r.FieldLogger)
	if err != nil {
		return trace.Wrap(err, "failed to install binary")
	}
	err = configureStateDirectory(r.config.SystemDevice)
	if err != nil {
		return trace.Wrap(err, "failed to configure state directory")
	}
	return nil
}

func (r *executor) upsertClusterAndOperation() (*ops.SiteOperation, error) {
	clusters, err := r.Operator.GetSites(defaults.SystemAccountID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var cluster *ops.Site
	if len(clusters) == 0 {
		cluster, err = r.Operator.CreateSite(r.NewCluster())
		if err != nil {
			return nil, trace.Wrap(err, "failed to create cluster")
		}
	} else {
		cluster = &clusters[0]
	}
	operations, err := r.Operator.GetSiteOperations(cluster.Key(), ops.OperationsFilter{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var operation *ops.SiteOperation
	if len(operations) == 0 {
		operation, err = r.createOperation()
		if err != nil {
			return nil, trace.Wrap(err, "failed to create install operation")
		}
	} else {
		operation = (*ops.SiteOperation)(&operations[0])
	}
	return operation, nil
}

func (r *executor) createOperation() (*ops.SiteOperation, error) {
	key, err := r.Operator.CreateSiteInstallOperation(r.ctx, ops.CreateSiteInstallOperationRequest{
		SiteDomain: r.config.SiteDomain,
		AccountID:  defaults.SystemAccountID,
		// With CLI install flow we always rely on external provisioner
		Provisioner: schema.ProvisionerOnPrem,
		Variables: storage.OperationVariables{
			System: storage.SystemVariables{
				Docker: r.config.Docker,
			},
			OnPrem: storage.OnPremVariables{
				VxlanPort: r.config.VxlanPort,
			},
			Values: r.config.Values,
		},
		Profiles: install.ServerRequirements(r.config.Flavor),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	operation, err := r.Operator.GetSiteOperation(*key)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return operation, nil
}

func (r *executor) waitForAgents(operation ops.SiteOperation) error {
	ctx, cancel := context.WithTimeout(r.ctx, defaults.AgentWaitTimeout)
	defer cancel()
	b := utils.NewUnlimitedExponentialBackOff()
	b.MaxInterval = 5 * time.Second
	var report *ops.AgentReport
	err := utils.RetryWithInterval(ctx, b, func() error {
		newReport, err := r.Operator.GetSiteInstallOperationAgentReport(ctx, operation.Key())
		if err != nil {
			return trace.Wrap(err, "failed to get agent report")
		}
		oldReport := report
		report = newReport
		if err := r.canContinue(oldReport, newReport); err != nil {
			return trace.Wrap(err)
		}
		r.WithField("report", report).Info("Installation can proceed.")
		err = install.UpdateOperationState(r.Operator, operation, *report)
		return trace.Wrap(err)
	})
	return trace.Wrap(err)
}

// canContinue returns true if all agents have joined and the installation can start
func (r *executor) canContinue(old, new *ops.AgentReport) error {
	// See if any new nodes have joined or left since previous agent report.
	joined, left := new.Diff(old)
	for _, server := range joined {
		r.PrintStep(color.GreenString("Successfully added %q node on %v",
			server.Role, utils.ExtractHost(server.AdvertiseAddr)))
	}
	for _, server := range left {
		r.PrintStep(color.YellowString("Node %q on %v has left",
			server.Role, utils.ExtractHost(server.AdvertiseAddr)))
	}
	// See if the current agent report satisfies the selected flavor.
	needed, extra := new.MatchFlavor(r.config.Flavor)
	if len(needed) == 0 && len(extra) == 0 {
		r.PrintStep(color.GreenString("All agents have connected!"))
		return nil
	}
	// If there were no changes compared to previous report, do not
	// output anything.
	if len(joined) == 0 && len(left) == 0 {
		return trace.Errorf("waiting for agents to join")
	}
	// Dump the table with remaining nodes that need to join.
	r.PrintStep("Please execute the following join commands on target nodes:\n%v",
		formatProfiles(needed, r.config.AdvertiseAddr, r.config.Token.Token))
	// If there are any extra agents with roles we don't expect for
	// the selected flavor, they need to leave.
	for _, server := range extra {
		r.PrintStep(color.RedString("Node %q on %v is not a part of the flavor, shut it down",
			server.Role, utils.ExtractHost(server.AdvertiseAddr)))
	}
	return trace.Errorf(formatNeededAndExtra(needed, extra))
}

// Engine implements command line-driven installation workflow
type Engine struct {
	// Config specifies the engine's configuration
	Config
}

type executor struct {
	Config
	install.Interface
	config install.Config
	ctx    context.Context
}

// formatProfiles outputs a table with information about node profiles
// that need to join in order for installation to proceed.
func formatProfiles(profiles map[string]int, addr, token string) string {
	var buf bytes.Buffer
	w := new(tabwriter.Writer)
	w.Init(&buf, 0, 8, 1, '\t', 0)
	fmt.Fprintf(w, "Role\tNodes\tCommand\n")
	fmt.Fprintf(w, "----\t-----\t-------\n")
	for role, nodes := range profiles {
		fmt.Fprintf(w, "%v\t%v\t%v\n", role, nodes,
			fmt.Sprintf("./gravity join %v --token=%v --role=%v",
				addr, token, role))
	}
	w.Flush()
	return buf.String()
}

// configureStateDirectory configures local gravity state directory
func configureStateDirectory(systemDevice string) error {
	stateDir, err := state.GetStateDir()
	if err != nil {
		return trace.Wrap(err)
	}
	err = environ.ConfigureStateDirectory(stateDir, systemDevice)
	return trace.Wrap(err)
}

func formatNeededAndExtra(needed map[string]int, extra []checks.ServerInfo) string {
	var buf bytes.Buffer
	fmt.Fprint(&buf, "still requires:[")
	var nodes []string
	for role, amount := range needed {
		nodes = append(nodes, fmt.Sprintf("%v nodes of role %q", amount, role))
	}
	fmt.Fprint(&buf, strings.Join(nodes, ","))
	fmt.Fprint(&buf, "]")
	if len(extra) == 0 {
		return buf.String()
	}
	fmt.Fprint(&buf, ", following nodes are unexpected:[")
	nodes = nodes[:0]
	for _, n := range extra {
		nodes = append(nodes, fmt.Sprintf("%v(%v)",
			n.Role, utils.ExtractHost(n.AdvertiseAddr)))
	}
	fmt.Fprint(&buf, strings.Join(nodes, ","))
	fmt.Fprint(&buf, "]")
	return buf.String()
}
