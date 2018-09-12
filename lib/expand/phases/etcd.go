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
	"bytes"
	"context"
	"fmt"
	"path/filepath"

	"github.com/gravitational/gravity/lib/clients"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/state"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	etcd "github.com/coreos/etcd/client"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// NewEtcd returns executor that adds a new etcd member to the cluster
func NewEtcd(p fsm.ExecutorParams, operator ops.Operator, runner fsm.AgentRepository) (*etcdExecutor, error) {
	// create etcd client that's talking to members running on master nodes
	var masters []storage.Server
	for _, node := range p.Plan.Servers {
		if node.ClusterRole == string(schema.ServiceRoleMaster) {
			masters = append(masters, node)
		}
	}
	var endpoints []string
	for _, master := range masters {
		endpoints = append(endpoints, fmt.Sprintf("https://%v:%v",
			master.AdvertiseIP, defaults.EtcdAPIPort))
	}
	stateDir, err := state.GetStateDir()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	etcdClient, err := clients.EtcdMembers(&clients.EtcdConfig{
		Endpoints:  endpoints,
		SecretsDir: state.SecretDir(stateDir),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	logger := &fsm.Logger{
		FieldLogger: logrus.WithFields(logrus.Fields{
			constants.FieldInstallPhase: p.Phase.ID,
		}),
		Key:      opKey(p.Plan),
		Operator: operator,
		Server:   p.Phase.Data.Server,
	}
	return &etcdExecutor{
		FieldLogger:    logger,
		Etcd:           etcdClient,
		Runner:         runner,
		Master:         *p.Phase.Data.Master,
		ExecutorParams: p,
	}, nil
}

type etcdExecutor struct {
	// FieldLogger is used for logging
	logrus.FieldLogger
	// Etcd is client to the cluster's etcd members API
	Etcd etcd.MembersAPI
	// Runner is used to run remote commands
	Runner fsm.AgentRepository
	// Master is one of the master nodes
	Master storage.Server
	// ExecutorParams is common executor params
	fsm.ExecutorParams
}

// Execute adds the joining node to the cluster's etcd cluster
func (p *etcdExecutor) Execute(ctx context.Context) error {
	p.Progress.NextStep("Adding etcd member")
	member, err := p.Etcd.Add(ctx, fmt.Sprintf("https://%v:%v",
		p.Phase.Data.Server.AdvertiseIP, defaults.EtcdPeerPort))
	if err != nil {
		return trace.Wrap(err)
	}
	p.Infof("Added etcd member: %v.", member)
	return nil
}

// Rollback removes the joined node from the cluster's etcd cluster
func (p *etcdExecutor) Rollback(ctx context.Context) error {
	p.Progress.NextStep("Restoring etcd data")
	backupPath, err := getBackupPath(p.Plan.OperationID)
	if err != nil {
		return trace.Wrap(err)
	}
	// _, err = utils.StatFile(backupPath) // make sure backup exists
	// if err != nil {
	// 	return trace.Wrap(err, "etcd backup %v does not exist", backupPath)
	// }
	agentClient, err := p.Runner.GetClient(ctx, p.Master.AdvertiseIP)
	if err != nil {
		return trace.Wrap(err)
	}
	// TODO take another backup just in case?
	var out bytes.Buffer
	err = agentClient.Command(ctx, p.FieldLogger, &out, utils.PlanetEnterCommand(
		defaults.SystemctlBin, "stop", "etcd")...)
	if err != nil {
		return trace.Wrap(err, "failed to stop etcd: %s", out.String())
	}
	err = agentClient.Command(ctx, p.FieldLogger, &out, utils.PlanetEnterCommand(
		defaults.PlanetBin, "etcd", "wipe", "--confirm")...)
	if err != nil {
		return trace.Wrap(err, "failed to wipe out etcd data: %s", out.String())
	}
	err = agentClient.Command(ctx, p.FieldLogger, &out, utils.PlanetEnterCommand(
		defaults.SystemctlBin, "start", "etcd")...)
	if err != nil {
		return trace.Wrap(err, "failed to start etcd: %s", out.String())
	}
	err = utils.Retry(defaults.RetryInterval, defaults.RetryLessAttempts, func() error {
		return agentClient.Command(ctx, p.FieldLogger, &out, utils.PlanetEnterCommand(
			defaults.PlanetBin, "etcd", "restore", backupPath)...)
	})
	if err != nil {
		return trace.Wrap(err, "failed to restore etcd data: %s", out.String())
	}
	p.Infof("Restored etcd data from %v.", backupPath)
	return nil
}

// PreCheck is no-op for this phase
func (*etcdExecutor) PreCheck(ctx context.Context) error {
	return nil
}

// PostCheck is no-op for this phase
func (*etcdExecutor) PostCheck(ctx context.Context) error {
	return nil
}

// NewEtcdBackup returns executor that backs up etcd data
func NewEtcdBackup(p fsm.ExecutorParams, operator ops.Operator, runner fsm.AgentRepository) (*etcdBackupExecutor, error) {
	logger := &fsm.Logger{
		FieldLogger: logrus.WithFields(logrus.Fields{
			constants.FieldInstallPhase: p.Phase.ID,
		}),
		Key:      opKey(p.Plan),
		Operator: operator,
		Server:   p.Phase.Data.Server,
	}
	return &etcdBackupExecutor{
		FieldLogger:    logger,
		Master:         *p.Phase.Data.Server,
		Runner:         runner,
		ExecutorParams: p,
	}, nil
}

type etcdBackupExecutor struct {
	// FieldLogger is used for logging
	logrus.FieldLogger
	// Master is the master server where backup should be taken
	Master storage.Server
	// Runner is used to run remote commands
	Runner fsm.AgentRepository
	// ExecutorParams is common executor params
	fsm.ExecutorParams
}

// Execute backs up etcd data on the node
func (p *etcdBackupExecutor) Execute(ctx context.Context) error {
	p.Progress.NextStep("Backing up etcd data")
	backupPath, err := getBackupPath(p.Plan.OperationID)
	if err != nil {
		return trace.Wrap(err)
	}
	agentClient, err := p.Runner.GetClient(ctx, p.Master.AdvertiseIP)
	if err != nil {
		return trace.Wrap(err)
	}
	var out bytes.Buffer
	err = agentClient.Command(ctx, p.FieldLogger, &out, utils.PlanetEnterCommand(
		defaults.PlanetBin, "etcd", "backup", backupPath)...)
	if err != nil {
		return trace.Wrap(err, "failed to backup etcd data: %s", out.String())
	}
	p.Infof("Backed up etcd data to %v.", backupPath)
	return nil
}

// Rollback is no-op for this phase
func (p *etcdBackupExecutor) Rollback(ctx context.Context) error {
	return nil
}

// PreCheck is no-op for this phase
func (*etcdBackupExecutor) PreCheck(ctx context.Context) error {
	return nil
}

// PostCheck is no-op for this phase
func (*etcdBackupExecutor) PostCheck(ctx context.Context) error {
	return nil
}

// getBackupPath returns etcd data backup path for the provided operation
// making sure that the directory where it's located exists
func getBackupPath(operationID string) (string, error) {
	stateDir, err := state.GetStateDir()
	if err != nil {
		return "", trace.Wrap(err)
	}
	return filepath.Join(stateDir, defaults.PlanetDir,
		fmt.Sprintf("join-%v.backup", operationID)), nil
}

func opKey(plan storage.OperationPlan) ops.SiteOperationKey {
	return ops.SiteOperationKey{
		AccountID:   plan.AccountID,
		SiteDomain:  plan.ClusterName,
		OperationID: plan.OperationID,
	}
}
