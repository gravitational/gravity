package cli

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"
	"time"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/install"
	libinstall "github.com/gravitational/gravity/lib/install"
	"github.com/gravitational/gravity/lib/install/engine"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/state"
	systemstate "github.com/gravitational/gravity/lib/system/state"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/fatih/color"
	"github.com/gravitational/trace"
	"github.com/kardianos/osext"
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
	if r.Printer == nil {
		r.Printer = utils.DiscardPrinter
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
	install.Planner
	// Operator specifies the service operator
	ops.Operator
	// ExcludeHostFromCluster specifies whether the host should not be part of the cluster
	ExcludeHostFromCluster bool
}

// Execute executes the installer steps
func (r *Engine) Execute(ctx context.Context, installer install.Installer) (err error) {
	err := installBinary(installer.ServiceUser.UID, installer.ServiceUser.GID, r.FieldLogger)
	if err != nil {
		return trace.Wrap(err, "failed to install binary")
	}
	err = r.configureStateDirectory(installer.SystemDevice)
	if err != nil {
		return trace.Wrap(err, "failed to configure state directory")
	}
	err = install.ExportRPCCredentials(ctx, installer.Packages, r.FieldLogger)
	if err != nil {
		return trace.Wrap(err, "failed to export RPC credentials")
	}
	operation, err := r.upsertClusterAndOperation(r.Operator, installer.Config)
	if err != nil {
		return trace.Wrap(err, "failed to export RPC credentials")
	}
	if !r.ExcludeHostFromCluster {
		agent, err := r.startAgent()
		if err != nil {
			return trace.Wrap(err, "failed to start installer agent")
		}
		defer agent.Close()
	}
	err = r.waitForAgents(ctx, installer, operation)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := engine.ExecuteOperation(ctx, r.Planner, r.StateMachineFactory, operator, operation.Key()); err != nil {
		return trace.Wrap(err)
	}
	installer.PrintPostInstallBanner()
	return nil
}

func (r *Engine) upsertClusterAndOperation(operator ops.Operator, config install.Config) (*ops.SiteOperation, error) {
	clusters, err := operator.GetSites(config.AccountID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var cluster *ops.Site
	if len(clusters) == 0 {
		cluster, err = r.NewCluster(operator)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		cluster = &clusters[0]
	}
	operations, err := operator.GetSiteOperations(cluster.Key())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var operation *ops.SiteOperation
	if len(operations) == 0 {
		operation, err = operator.CreateSiteInstallOperation(ops.CreateSiteInstallOperationRequest{
			SiteDomain: cluster.Domain,
			AccountID:  cluster.AccountID,
			// With CLI install flow we always rely on external provisioner
			Provisioner: schema.ProvisionerOnPrem,
			Variables: storage.OperationVariables{
				System: storage.SystemVariables{
					Docker: config.Docker,
				},
				OnPrem: storage.OnPremVariables{
					PodCIDR:     config.PodCIDR,
					ServiceCIDR: config.ServiceCIDR,
					VxlanPort:   config.VxlanPort,
				},
			},
			Profiles: ServerRequirements(*config.Flavor),
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		operation = &operations[0]
	}
	return operation, nil
}

func (r *Engine) waitForAgents(ctx context.Context, installer install.Installer, operation ops.SiteOperation) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	b := utils.NewUnlimitedExponentialBackOff()
	b.MaxInterval = 5 * time.Second
	var oldReport *ops.AgentReport
	err = utils.RetryWithInterval(ctx, b, func() error {
		report, err := r.Operator.GetSiteInstallOperationAgentReport(operation.Key())
		if err != nil {
			return trace.Wrap(err, "failed to get agent report")
		}
		old := oldReport
		oldReport = report
		if !r.canContinue(old, report, installer.Config) {
			return trace.BadParameter("cannot continue")
		}
		r.WithField("report", report).Info("Installation can proceed.")
		err = libinstall.UpdateOperationState(*report, r.Operator, operation)
		return trace.Wrap(err)
	})
	return trace.Wrap(err)
}

// canContinue returns true if the installation can commence based on the
// provided agent report and false if not all agents have joined yet.
func (r *Engine) canContinue(old, new *ops.AgentReport, config install.Config) bool {
	// See if any new nodes have joined or left since previous agent report.
	joined, left := new.Diff(old)
	for _, server := range joined {
		config.PrintStep(color.GreenString("Successfully added %q node on %v",
			server.Role, utils.ExtractHost(server.AdvertiseAddr)))
	}
	for _, server := range left {
		config.PrintStep(color.YellowString("Node %q on %v has left",
			server.Role, utils.ExtractHost(server.AdvertiseAddr)))
	}
	// Save the current agent report so we can compare against it on next iteration.
	// i.agentReport = report
	// See if the current agent report satisfies the selected flavor.
	needed, extra := new.MatchFlavor(*config.Flavor)
	if len(needed) == 0 && len(extra) == 0 {
		config.PrintStep(color.GreenString("All agents have connected!"))
		return true
	}
	// If there were no changes compared to previous report, do not
	// output anything.
	if len(joined) == 0 && len(left) == 0 {
		return false
	}
	// Dump the table with remaining nodes that need to join.
	r.PrintStep(fmt.Sprintf("Please execute the following join commands on target nodes:\n%v",
		formatProfiles(needed, config.AdvertiseAddr, config.Token.Token)))
	// If there are any extra agents with roles we don't expect for
	// the selected flavor, they need to leave.
	for _, server := range extra {
		config.PrintStep(color.RedString("Node %q on %v is not a part of the flavor, shut it down",
			server.Role, utils.ExtractHost(server.AdvertiseAddr)))
	}
	return false
}

func (r *Engine) startAgent(installer install.Installer) (*rpcserver.Agent, error) {
	profile, ok := r.Operation.InstallExpand.Agents[installer.Role]
	if !ok {
		return nil, trace.NotFound("agent profile not found for %v", installer.Role)
	}
	agent, err := installer.NewAgent(profile.AgentURL)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	go agent.Serve()
	return agent, nil
}

type Engine struct {
	Config
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

// installBinary places the system binary into the proper binary directory
// depending on the distribution.
// The specified uid/gid pair is used to set user/group permissions on the
// resulting binary
func installBinary(uid, gid int, logger log.FieldLogger) (err error) {
	for _, targetPath := range state.GravityBinPaths {
		err = tryInstallBinary(targetPath, uid, gid, logger)
		if err == nil {
			break
		}
	}
	if err != nil {
		return trace.Wrap(err, "failed to install gravity binary in any of %v",
			state.GravityBinPaths)
	}
	return nil
}

func tryInstallBinary(targetPath string, uid, gid int, logger log.FieldLogger) error {
	path, err := osext.Executable()
	if err != nil {
		return trace.Wrap(err, "failed to determine path to binary")
	}
	err = os.MkdirAll(filepath.Dir(targetPath), defaults.SharedDirMask)
	if err != nil {
		return trace.Wrap(err)
	}
	err = utils.CopyFileWithPerms(targetPath, path, defaults.SharedExecutableMask)
	if err != nil {
		return trace.Wrap(err)
	}
	err = os.Chown(targetPath, uid, gid)
	if err != nil {
		return trace.Wrap(trace.ConvertSystemError(err),
			"failed to change ownership on %v", targetPath)
	}
	logger.WithField("path", targetPath).Info("Installed gravity binary.")
	return nil
}
