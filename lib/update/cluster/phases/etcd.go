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

package phases

import (
	"context"
	"path/filepath"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/kubernetes"
	"github.com/gravitational/gravity/lib/state"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/update"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	kubeapi "k8s.io/client-go/kubernetes"
)

// Upgrade ETCD
// Upgrading etcd to etcd 3 is a somewhat complicated process.
// According to the etcd documentation, upgrades of a cluster are only supported one release at a time. Since we are
// several versions behind, coordinate several upgrades in succession has a certain amount of risk and may also be
// time consuming.
//
// The chosen approach to upgrades of etcd is as follows
// 1. Planet will ship with each version of etcd we support upgrades from
// 2. Planet when started, will determine the version of etcd to use (planet etcd init)
//      This is done by assuming the oldest possible etcd release
//      During an upgrade, the verison of etcd to use is written to the etcd data directory
// 3. Backup all etcd data via API (optional)
// 4. Shutdown etcd (all servers) // API outage starts
// 5. Move old data directory to the new location
// 6. Restart etcd on the correct ports// API outage ends
// 7. Restart gravity-site to fix elections
//
//
// Rollback
// Stop etcd (all servers)
// Set the version to use to be the previous version
// Restart etcd using the old version, pointed at the old data directory
// Start etcd (all servers)

// PhaseUpgradeEtcdBackup backs up etcd data on all servers
type PhaseUpgradeEtcdBackup struct {
	log.FieldLogger
}

func NewPhaseUpgradeEtcdBackup(logger log.FieldLogger) (fsm.PhaseExecutor, error) {
	return &PhaseUpgradeEtcdBackup{
		FieldLogger: logger,
	}, nil
}

func (p *PhaseUpgradeEtcdBackup) Execute(ctx context.Context) error {
	p.Info("Backup etcd.")
	out, err := utils.RunPlanetCommand(ctx, p.FieldLogger, "etcd", "backup", backupFile())
	if err != nil {
		return trace.Wrap(err, "failed to backup etcd").AddField("output", string(out))
	}
	return nil
}

func (p *PhaseUpgradeEtcdBackup) Rollback(context.Context) error {
	// NOOP, don't clean up the backup file during rollback, incase we still need it
	return nil
}

func (*PhaseUpgradeEtcdBackup) PreCheck(context.Context) error {
	// TODO(knisbet) should we check that there is enough free space available to hold the backup?
	return nil
}

func (*PhaseUpgradeEtcdBackup) PostCheck(context.Context) error {
	// NOOP
	return nil
}

// PhaseUpgradeEtcdShutdown shuts down etcd across the cluster
type PhaseUpgradeEtcdShutdown struct {
	log.FieldLogger
	Client   *kubeapi.Clientset
	isLeader bool
}

// NewPhaseUpgradeEtcdShutdown creates a phase for shutting down etcd across the cluster
// 4. Shutdown etcd (all servers) // API outage starts
func NewPhaseUpgradeEtcdShutdown(phase storage.OperationPhase, client *kubeapi.Clientset, logger log.FieldLogger) (fsm.PhaseExecutor, error) {
	return &PhaseUpgradeEtcdShutdown{
		FieldLogger: logger,
		Client:      client,
		isLeader:    phase.Data.Data == "true",
	}, nil
}

func (p *PhaseUpgradeEtcdShutdown) Execute(ctx context.Context) error {
	p.Info("Shutdown etcd.")
	out, err := utils.RunPlanetCommand(ctx, p.FieldLogger, "etcd", "disable", "--stop-api")
	if err != nil {
		return trace.Wrap(err).AddField("output", string(out))
	}
	p.Info("Command output: ", string(out))
	return nil
}

func (p *PhaseUpgradeEtcdShutdown) Rollback(ctx context.Context) error {
	p.Info("Enable etcd.")
	out, err := utils.RunPlanetCommand(ctx, p.FieldLogger, "etcd", "enable")
	if err != nil {
		return trace.Wrap(err).AddField("output", string(out))
	}
	p.Info("Command output: ", string(out))

	if p.isLeader {
		return trace.Wrap(restartGravitySite(ctx, p.Client, p.FieldLogger))
	}
	return nil
}

func (p *PhaseUpgradeEtcdShutdown) PreCheck(ctx context.Context) error {
	return nil
}

func (*PhaseUpgradeEtcdShutdown) PostCheck(context.Context) error {
	return nil
}

// PhaseUpgradeEtcd upgrades etcd specifically on the leader
type PhaseUpgradeEtcd struct {
	log.FieldLogger
}

func NewPhaseUpgradeEtcd(phase storage.OperationPhase, logger log.FieldLogger) (fsm.PhaseExecutor, error) {
	return &PhaseUpgradeEtcd{
		FieldLogger: logger,
	}, nil
}

// Execute upgrades the leader
// Upgrade etcd by changing the launch version and data directory
func (p *PhaseUpgradeEtcd) Execute(ctx context.Context) error {
	p.Info("Upgrade etcd.")
	// TODO(knisbet) only wipe the etcd database when required
	out, err := utils.RunPlanetCommand(ctx, p.FieldLogger, "etcd", "upgrade")
	if err != nil {
		return trace.Wrap(err).AddField("output", string(out))
	}
	p.Info("Command output: ", string(out))

	return nil
}

func (p *PhaseUpgradeEtcd) Rollback(ctx context.Context) error {
	out, err := utils.RunPlanetCommand(ctx, p.FieldLogger, "etcd", "rollback")
	if err != nil {
		return trace.Wrap(err).AddField("output", string(out))
	}
	p.Info("Command output: ", string(out))

	return nil
}

func (*PhaseUpgradeEtcd) PreCheck(context.Context) error {
	return nil
}

func (*PhaseUpgradeEtcd) PostCheck(context.Context) error {
	return nil
}

// PhaseUpgradeEtcdMigrate moves etcd data to the new version
type PhaseUpgradeEtcdMigrate struct {
	log.FieldLogger
	fromVersion, toVersion string
}

// NewPhaseUpgradeEtcdMigrate creates a new phase to move the data from the old version to the new version
func NewPhaseUpgradeEtcdMigrate(phase storage.OperationPhase, logger log.FieldLogger) (fsm.PhaseExecutor, error) {
	return &PhaseUpgradeEtcdMigrate{
		FieldLogger: logger,
		fromVersion: phase.Data.Update.Etcd.From,
		toVersion:   phase.Data.Update.Etcd.To,
	}, nil
}

// Execute moves the data from the old version to the new version
func (p *PhaseUpgradeEtcdMigrate) Execute(ctx context.Context) error {
	p.Info("Migrate etcd data.")
	gravityPath := filepath.Join(defaults.GravityRPCAgentDir, constants.GravityBin)
	out, err := utils.RunCommand(ctx, p.FieldLogger,
		utils.PlanetCommandArgs(gravityPath,
			"system", "etcd", "migrate",
			"--from", p.fromVersion,
			"--to", p.toVersion)...)
	if err != nil {
		return trace.Wrap(err).AddField("output", string(out))
	}
	return nil
}

func (*PhaseUpgradeEtcdMigrate) Rollback(ctx context.Context) error {
	return nil
}

func (p *PhaseUpgradeEtcdMigrate) PreCheck(ctx context.Context) error {
	// wait for etcd to form a cluster
	out, err := utils.RunCommand(ctx, p.FieldLogger,
		utils.PlanetCommandArgs(defaults.WaitForEtcdScript, "https://127.0.0.2:2379")...)
	if err != nil {
		return trace.Wrap(err).AddField("output", string(out))
	}
	p.Info("Command output: ", string(out))
	return nil
}

func (*PhaseUpgradeEtcdMigrate) PostCheck(context.Context) error {
	return nil
}

// PhaseUpgradeEtcdRestart disables the etcd-upgrade service, and starts the etcd service
type PhaseUpgradeEtcdRestart struct {
	log.FieldLogger
	Server storage.Server
	Master storage.Server
}

func NewPhaseUpgradeEtcdRestart(phase storage.OperationPhase, logger log.FieldLogger) (fsm.PhaseExecutor, error) {
	return &PhaseUpgradeEtcdRestart{
		FieldLogger: logger,
		Server:      *phase.Data.Server,
	}, nil
}

func (p *PhaseUpgradeEtcdRestart) Execute(ctx context.Context) error {
	p.Info("Restart etcd after upgrade.")
	out, err := utils.RunPlanetCommand(ctx, p.FieldLogger, "etcd", "enable")
	if err != nil {
		return trace.Wrap(err).AddField("output", string(out))
	}
	return nil
}

func (p *PhaseUpgradeEtcdRestart) Rollback(ctx context.Context) error {
	p.Info("Reenable etcd upgrade service.")
	out, err := utils.RunPlanetCommand(ctx, p.FieldLogger, "etcd", "disable", "--stop-api")
	if err != nil {
		return trace.Wrap(err).AddField("output", string(out))
	}
	return nil
}

func (*PhaseUpgradeEtcdRestart) PreCheck(context.Context) error {
	return nil
}

func (*PhaseUpgradeEtcdRestart) PostCheck(context.Context) error {
	// NOOP
	return nil
}

// PhaseUpgradeGravitySiteRestart restarts gravity-site pod
type PhaseUpgradeGravitySiteRestart struct {
	log.FieldLogger
	Client *kubeapi.Clientset
}

func NewPhaseUpgradeGravitySiteRestart(phase storage.OperationPhase, client *kubeapi.Clientset, logger log.FieldLogger) (fsm.PhaseExecutor, error) {
	if client == nil {
		return nil, trace.BadParameter("phase %q must be run from a master node (requires kubernetes client)", phase.ID)
	}

	return &PhaseUpgradeGravitySiteRestart{
		FieldLogger: logger,
		Client:      client,
	}, nil
}

func (p *PhaseUpgradeGravitySiteRestart) Execute(ctx context.Context) error {
	return trace.Wrap(restartGravitySite(ctx, p.Client, p.FieldLogger))
}

func (p *PhaseUpgradeGravitySiteRestart) Rollback(context.Context) error {
	return nil
}

func (*PhaseUpgradeGravitySiteRestart) PreCheck(context.Context) error {
	return nil
}

func (*PhaseUpgradeGravitySiteRestart) PostCheck(context.Context) error {
	return nil
}

func restartGravitySite(ctx context.Context, client *kubeapi.Clientset, logger log.FieldLogger) error {
	logger.Info("Restart cluster controller.")
	// wait for etcd to form a cluster
	out, err := utils.RunCommand(ctx, logger, utils.PlanetCommandArgs(defaults.WaitForEtcdScript)...)
	if err != nil {
		return trace.Wrap(err).AddField("output", string(out))
	}
	logger.Info("Command output: ", string(out))

	// delete the gravity-site pods, in order to force them to restart
	// This is because the leader election process seems to break during the etcd upgrade
	label := map[string]string{"app": constants.GravityServiceName}
	logger.Infof("Deleting pods with label %v.", label)
	err = update.Retry(ctx, func() error {
		return trace.Wrap(kubernetes.DeletePods(client, constants.KubeSystemNamespace, label))
	}, defaults.DrainErrorTimeout)
	return trace.Wrap(err)
}

func backupFile() (path string) {
	return filepath.Join(state.GravityUpdateDir(defaults.GravityDir), defaults.EtcdUpgradeBackupFile)
}
