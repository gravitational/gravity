package cli

import (
	"bytes"
	"context"
	"fmt"
	"text/tabwriter"
	"time"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/install"
	libinstall "github.com/gravitational/gravity/lib/install"
	"github.com/gravitational/gravity/lib/install/engine"
	"github.com/gravitational/gravity/lib/ops"
	rpcserver "github.com/gravitational/gravity/lib/rpc/server"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/state"
	"github.com/gravitational/gravity/lib/storage"
	systemstate "github.com/gravitational/gravity/lib/system/state"
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
	if r.StateMachineFactory == nil {
		return trace.BadParameter("StateMachineFactory required")
	}
	if r.ClusterFactory == nil {
		return trace.BadParameter("ClusterFactory is required")
	}
	if r.Planner == nil {
		return trace.BadParameter("Planner is required")
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
	// StateMachineFactory is a factory for creating installer state machines
	engine.StateMachineFactory
	// ClusterFactory is a factory for creating cluster records
	engine.ClusterFactory
	// Planner creates a plan for the operation
	engine.Planner
	// Operator specifies the service operator
	ops.Operator
	// ExcludeHostFromCluster specifies whether the host should not be part of the cluster
	ExcludeHostFromCluster bool
	// Manual specifies whether the operation should be automatically executed.
	// If false, only the cluster/operation/plan are created
	Manual bool
}

func (r *Engine) Validate(ctx context.Context, config install.Config) (err error) {
	return trace.Wrap(config.RunLocalChecks(ctx))
}

// Execute executes the installer steps
func (r *Engine) Execute(ctx context.Context, installer install.Interface, config install.Config) (err error) {
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
		return trace.Wrap(err, "failed to create cluster/operation")
	}
	if err := installer.NotifyOperationAvailable(operation.Key()); err != nil {
		return trace.Wrap(err)
	}
	if !r.ExcludeHostFromCluster {
		profile, ok := operation.InstallExpand.Agents[config.Role]
		if !ok {
			return trace.NotFound("agent profile not found for %v", config.Role)
		}
		agent, err := e.startAgent(profile)
		if err != nil {
			return trace.Wrap(err, "failed to start installer agent")
		}
		defer agent.Stop(ctx)
	}
	err = e.waitForAgents(*operation)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := engine.ExecuteOperation(ctx, r.Planner, r.StateMachineFactory,
		r.Operator, operation.Key(), r.FieldLogger); err != nil {
		return trace.Wrap(err)
	}
	if err := installer.CompleteFinalInstallStep(0); err != nil {
		r.WithError(err).Warn("Failed to complete final install step.")
	}
	if err := installer.Finalize(*operation); err != nil {
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
	err = install.ExportRPCCredentials(r.ctx, r.config.Packages, r.FieldLogger)
	if err != nil {
		return trace.Wrap(err, "failed to export RPC credentials")
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
			return nil, trace.Wrap(err)
		}
	} else {
		cluster = &clusters[0]
	}
	operations, err := r.Operator.GetSiteOperations(cluster.Key())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var operation *ops.SiteOperation
	if len(operations) == 0 {
		operation, err = r.createOperation()
		if err != nil {
			return nil, trace.Wrap(err)
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
				PodCIDR:     r.config.PodCIDR,
				ServiceCIDR: r.config.ServiceCIDR,
				VxlanPort:   r.config.VxlanPort,
			},
		},
		Profiles: install.ServerRequirements(*r.config.Flavor),
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
	ctx, cancel := context.WithTimeout(r.ctx, 5*time.Minute)
	defer cancel()
	b := utils.NewUnlimitedExponentialBackOff()
	b.MaxInterval = 5 * time.Second
	var oldReport *ops.AgentReport
	err := utils.RetryWithInterval(ctx, b, func() error {
		report, err := r.Operator.GetSiteInstallOperationAgentReport(operation.Key())
		if err != nil {
			return trace.Wrap(err, "failed to get agent report")
		}
		old := oldReport
		oldReport = report
		if !r.canContinue(old, report) {
			return trace.BadParameter("cannot continue")
		}
		r.WithField("report", report).Info("Installation can proceed.")
		err = libinstall.UpdateOperationState(r.Operator, operation, *report)
		return trace.Wrap(err)
	})
	return trace.Wrap(err)
}

// canContinue returns true if the installation can commence based on the
// provided agent report and false if not all agents have joined yet.
func (r *executor) canContinue(old, new *ops.AgentReport) bool {
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
	// Save the current agent report so we can compare against it on next iteration.
	// i.agentReport = report
	// See if the current agent report satisfies the selected flavor.
	needed, extra := new.MatchFlavor(r.config.Flavor)
	if len(needed) == 0 && len(extra) == 0 {
		r.PrintStep(color.GreenString("All agents have connected!"))
		return true
	}
	// If there were no changes compared to previous report, do not
	// output anything.
	if len(joined) == 0 && len(left) == 0 {
		return false
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
	return false
}

func (r *executor) startAgent(profile storage.AgentProfile) (rpcserver.Server, error) {
	agent, err := r.NewAgent(profile.AgentURL)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	go agent.Serve()
	return agent, nil
}

type Engine struct {
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
			fmt.Sprintf("gravity join %v --token=%v --role=%v",
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
	err = systemstate.ConfigureStateDirectory(stateDir, systemDevice)
	return trace.Wrap(err)
}
