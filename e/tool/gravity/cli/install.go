package cli

import (
	"context"
	"fmt"

	"github.com/gravitational/gravity/e/lib/environment"
	"github.com/gravitational/gravity/e/lib/install"
	"github.com/gravitational/gravity/e/lib/ops"
	"github.com/gravitational/gravity/e/lib/ops/client"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/httplib"
	ossinstall "github.com/gravitational/gravity/lib/install"
	"github.com/gravitational/gravity/lib/localenv"
	ossops "github.com/gravitational/gravity/lib/ops"
	opsclient "github.com/gravitational/gravity/lib/ops/opsclient"
	"github.com/gravitational/gravity/lib/utils"
	"github.com/gravitational/gravity/tool/gravity/cli"

	"github.com/gravitational/trace"
	"github.com/pborman/uuid"
	"github.com/sirupsen/logrus"
)

func startInstall(env *environment.Local, i InstallConfig) error {
	env.PrintStep("Starting enterprise installer")

	err := cli.CheckLocalState(env.LocalEnvironment)
	if err != nil {
		return trace.Wrap(err)
	}

	err = i.CheckAndSetDefaults()
	if err != nil {
		return trace.Wrap(err)
	}

	installerConfig, err := i.ToInstallerConfig(env)
	if err != nil {
		return trace.Wrap(err)
	}

	processConfig, err := install.MakeProcessConfig(*installerConfig)
	if err != nil {
		return trace.Wrap(err)
	}

	env.PrintStep("Preparing for installation...")

	installerConfig.Process, err = ossinstall.InitProcess(context.TODO(),
		installerConfig.Config, *processConfig)
	if err != nil {
		return trace.Wrap(err)
	}

	installer, err := install.Init(context.TODO(), *installerConfig)
	if err != nil {
		return trace.Wrap(err)
	}

	err = installer.Start()
	if err != nil {
		return trace.Wrap(err)
	}

	err = installer.Wait()
	if utils.IsContextCancelledError(err) {
		return nil
	}
	return trace.Wrap(err)
}

func installOrJoin(env, joinEnv *environment.Local, config InstallConfig) error {
	// when installing using Ops Center instructions, no advertise IP is
	// specified so it needs to be determined automatically
	if config.AdvertiseAddr == "" {
		advertiseIP, err := utils.PickAdvertiseIP()
		if err != nil {
			return trace.Wrap(err)
		}
		config.AdvertiseAddr = advertiseIP
	}
	// generate a unique process ID that will allow this agent to identify
	// itself with other agents, so for example they can detect if the process
	// on the same machine shut down and started again
	id := uuid.New()
	// register ourselves with the Ops Center provided in the config - it
	// will tell us whether we are the installer or a joining agent
	operator, err := client.NewBearerClient(config.OpsURL, config.OpsToken,
		opsclient.HTTPClient(httplib.GetClient(config.Insecure)))
	if err != nil {
		return trace.Wrap(err)
	}
	req := ops.RegisterAgentRequest{
		AccountID:   defaults.SystemAccountID,
		ClusterName: config.SiteDomain,
		OperationID: config.OperationID,
		AgentID:     id,
		AdvertiseIP: config.AdvertiseAddr,
	}
	response, err := operator.RegisterAgent(req)
	if err != nil {
		return trace.Wrap(err)
	}
	logrus.WithFields(logrus.Fields{
		"advertise-ip": config.AdvertiseAddr,
	}).Debugf("%s.", response)
	// keep heartbeating back to Ops Center until the installation start
	go install.RegisterAgentLoop(install.RegisterAgentParams{
		Context:          context.TODO(),
		Request:          req,
		OriginalResponse: *response,
		Operator:         operator,
	})
	// this agent will be running the installer process
	// (effectively, running "gravity install")
	if response.InstallerID == id {
		err := startInstall(env, config)
		if err != nil {
			return trace.Wrap(err)
		}
		return nil
	}
	// this agent will be joining the installer process
	// (effectively, running "gravity join")
	env.PrintStep(fmt.Sprintf("Joining installer at %v", response.InstallerIP))
	err = cli.Join(env.LocalEnvironment, joinEnv.LocalEnvironment, cli.JoinConfig{
		SystemLogFile: config.SystemLogFile,
		UserLogFile:   config.UserLogFile,
		AdvertiseAddr: config.AdvertiseAddr,
		ServerAddr:    fmt.Sprintf("%v:%v", response.InstallerIP, defaults.WizardPackServerPort),
		PeerAddrs:     response.InstallerIP,
		Token:         config.InstallToken,
		Role:          config.Role,
		SystemDevice:  config.SystemDevice,
		DockerDevice:  config.DockerDevice,
		Mounts:        config.Mounts,
		OperationID:   config.OperationID,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// InstallPhaseParams is a set of parameters for a single phase execution
type InstallPhaseParams struct {
	cli.PhaseParams
	// OpsURL is an optional address of Ops Center to report progress to
	OpsURL string
	// OpsToken is the auth token for the above Ops Center
	OpsToken string
}

func executeInstallPhase(localEnv *environment.Local, p InstallPhaseParams) error {
	wizardEnv, err := localenv.NewRemoteEnvironment()
	if err != nil {
		return trace.Wrap(err)
	}

	if wizardEnv.Operator == nil {
		return trace.BadParameter(cli.NoOperationStateBanner)
	}

	op, err := ossops.GetWizardOperation(wizardEnv.Operator)
	if err != nil {
		if trace.IsConnectionProblem(err) {
			if err2 := cli.CheckInstallOperationComplete(localEnv.LocalEnvironment); err2 != nil {
				return trace.Wrap(err, "unable to connect to installer. Is the installer process running?")
			}
			return trace.BadParameter("installation already completed")
		}
		return trace.Wrap(err)
	}

	var operator ops.Operator = client.New(wizardEnv.Operator)
	if p.OpsURL != "" && p.OpsToken != "" {
		opsOperator, err := client.NewBearerClient(p.OpsURL, p.OpsToken,
			opsclient.HTTPClient(localEnv.HTTPClient()))
		if err != nil {
			return trace.Wrap(err)
		}
		operator = install.NewFanoutOperator(operator, opsOperator)
	}

	localApps, err := localEnv.AppServiceLocal(localenv.AppConfig{})
	if err != nil {
		return trace.Wrap(err)
	}

	fsmConfig := ossinstall.FSMConfig{
		OperationKey:       op.Key(),
		Packages:           wizardEnv.Packages,
		Apps:               wizardEnv.Apps,
		Operator:           operator,
		LocalClusterClient: localEnv.SiteOperator,
		LocalPackages:      localEnv.Packages,
		LocalApps:          localApps,
		LocalBackend:       localEnv.Backend,
		Insecure:           localEnv.Insecure,
	}
	fsmConfig.Spec = install.FSMSpec(fsmConfig)
	installFSM, err := ossinstall.NewFSM(fsmConfig)
	if err != nil {
		return trace.Wrap(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), p.Timeout)
	defer cancel()
	progress := utils.NewProgress(ctx, fmt.Sprintf("Executing install phase %q", p.PhaseID), -1, false)
	defer progress.Stop()

	if p.PhaseID == fsm.RootPhase {
		return trace.Wrap(cli.ResumeInstall(ctx, installFSM, progress))
	}

	err = installFSM.ExecutePhase(ctx, fsm.Params{
		PhaseID:  p.PhaseID,
		Force:    p.Force,
		Progress: progress,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}
