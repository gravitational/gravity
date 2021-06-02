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
	"github.com/gravitational/gravity/lib/rpc"
	rpcclient "github.com/gravitational/gravity/lib/rpc/client"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/state"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/cenkalti/backoff"
	etcd "github.com/coreos/etcd/client"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// NewEtcd returns executor that adds a new etcd member to the cluster
func NewEtcd(p fsm.ExecutorParams, operator ops.Operator, runner rpc.AgentRepository) (fsm.PhaseExecutor, error) {
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
			constants.FieldPhase: p.Phase.ID,
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
		etcdPeerURL:    p.Phase.Data.Server.EtcdPeerURL(),
	}, nil
}

type etcdExecutor struct {
	// FieldLogger is used for logging
	logrus.FieldLogger
	// Etcd is client to the cluster's etcd members API
	Etcd etcd.MembersAPI
	// Runner is used to run remote commands
	Runner rpc.AgentRepository
	// Master is one of the master nodes
	Master storage.Server
	// ExecutorParams is common executor params
	fsm.ExecutorParams
	// etcdPeerURL specifies this node's etcd peer URL
	etcdPeerURL string
}

// Execute adds the joining node to the cluster's etcd cluster
func (p *etcdExecutor) Execute(ctx context.Context) error {
	p.Progress.NextStep("Adding etcd member")
	member, err := p.addEtcdMember(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	p.Infof("Added etcd member: %v.", member)
	return nil
}

// Rollback removes the joined node from the cluster's etcd cluster
func (p *etcdExecutor) Rollback(ctx context.Context) error {
	p.Progress.NextStep("Restoring etcd data")
	backupPath := getBackupPath(p.Plan.OperationID)
	agentClient, err := p.Runner.GetClient(ctx, p.Master.AdvertiseIP)
	if err != nil {
		return trace.Wrap(err)
	}
	err = p.checkBackup(ctx, agentClient, backupPath)
	if err != nil {
		return trace.Wrap(err)
	}
	err = p.stopEtcd(ctx, agentClient)
	if err != nil {
		return trace.Wrap(err)
	}
	err = p.wipeEtcd(ctx, agentClient)
	if err != nil {
		return trace.Wrap(err)
	}
	err = p.startEtcd(ctx, agentClient)
	if err != nil {
		return trace.Wrap(err)
	}
	err = p.restoreEtcd(ctx, agentClient, backupPath)
	if err != nil {
		return trace.Wrap(err)
	}
	p.Infof("Restored etcd data from %v.", backupPath)
	return nil
}

func (p *etcdExecutor) addEtcdMember(ctx context.Context) (member *etcd.Member, err error) {
	boff := backoff.NewExponentialBackOff()
	boff.MaxElapsedTime = defaults.TransientErrorTimeout
	err = utils.RetryTransient(ctx, boff, func() error {
		peers, err := p.Etcd.List(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
		if p.hasSelfAsMember(peers) {
			p.WithField("peerURL", p.etcdPeerURL).Info("Node is already etcd peer.")
			return nil
		}
		member, err = p.Etcd.Add(ctx, p.etcdPeerURL)
		return trace.Wrap(err)
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return member, nil
}

func (p *etcdExecutor) hasSelfAsMember(peers []etcd.Member) bool {
	return utils.EtcdHasMember(peers, p.etcdPeerURL) != nil
}

func (p *etcdExecutor) checkBackup(ctx context.Context, agent rpcclient.Client, backupPath string) error {
	var out bytes.Buffer
	err := agent.Command(ctx, p.FieldLogger, &out, &out, utils.PlanetEnterCommand(
		defaults.StatBin, backupPath)...)
	if err != nil {
		return trace.Wrap(err, "failed to check backup file %v: %s", backupPath, out.String())
	}
	return nil
}

func (p *etcdExecutor) stopEtcd(ctx context.Context, agent rpcclient.Client) error {
	var out bytes.Buffer
	err := agent.Command(ctx, p.FieldLogger, &out, &out, utils.PlanetEnterCommand(
		defaults.SystemctlBin, "stop", "etcd")...)
	if err != nil {
		return trace.Wrap(err, "failed to stop etcd: %s", out.String())
	}
	return nil
}

func (p *etcdExecutor) wipeEtcd(ctx context.Context, agent rpcclient.Client) error {
	var out bytes.Buffer
	err := agent.Command(ctx, p.FieldLogger, &out, &out, utils.PlanetEnterCommand(
		defaults.PlanetBin, "etcd", "wipe", "--confirm")...)
	if err != nil {
		return trace.Wrap(err, "failed to wipe out etcd data: %s", out.String())
	}
	return nil
}

func (p *etcdExecutor) startEtcd(ctx context.Context, agent rpcclient.Client) error {
	var out bytes.Buffer
	err := agent.Command(ctx, p.FieldLogger, &out, &out, utils.PlanetEnterCommand(
		defaults.SystemctlBin, "start", "etcd")...)
	if err != nil {
		return trace.Wrap(err, "failed to start etcd: %s", out.String())
	}
	return nil
}

func (p *etcdExecutor) restoreEtcd(ctx context.Context, agent rpcclient.Client, backupPath string) error {
	var out bytes.Buffer
	err := utils.Retry(defaults.RetryInterval, defaults.RetryLessAttempts, func() error {
		return agent.Command(ctx, p.FieldLogger, &out, &out, utils.PlanetEnterCommand(
			defaults.PlanetBin, "etcd", "restore", backupPath)...)
	})
	if err != nil {
		return trace.Wrap(err, "failed to restore etcd data: %s", out.String())
	}
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
func NewEtcdBackup(p fsm.ExecutorParams, operator ops.Operator, runner rpc.AgentRepository) (fsm.PhaseExecutor, error) {
	logger := &fsm.Logger{
		FieldLogger: logrus.WithFields(logrus.Fields{
			constants.FieldPhase: p.Phase.ID,
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
	Runner rpc.AgentRepository
	// ExecutorParams is common executor params
	fsm.ExecutorParams
}

// Execute backs up etcd data on the node
func (p *etcdBackupExecutor) Execute(ctx context.Context) error {
	p.Progress.NextStep("Backing up etcd data")
	backupPath := getBackupPath(p.Plan.OperationID)
	agentClient, err := p.Runner.GetClient(ctx, p.Master.AdvertiseIP)
	if err != nil {
		return trace.Wrap(err)
	}
	err = p.backupEtcd(ctx, agentClient, backupPath)
	if err != nil {
		return trace.Wrap(err)
	}
	p.Infof("Backed up etcd data to %v.", backupPath)
	return nil
}

func (p *etcdBackupExecutor) backupEtcd(ctx context.Context, agent rpcclient.Client, backupPath string) error {
	var out bytes.Buffer
	err := agent.Command(ctx, p.FieldLogger, &out, &out, utils.PlanetEnterCommand(
		defaults.PlanetBin, "etcd", "backup", backupPath)...)
	if err != nil {
		return trace.Wrap(err, "failed to backup etcd data: %s", out.String())
	}
	return nil
}

// Rollback is no-op for this phase
func (*etcdBackupExecutor) Rollback(ctx context.Context) error {
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

// getBackupPath returns in-planet etcd data backup path for the provided
// operation
func getBackupPath(operationID string) string {
	return filepath.Join(defaults.GravityDir, defaults.PlanetDir,
		fmt.Sprintf("join-%v.backup", operationID))
}

func opKey(plan storage.OperationPlan) ops.SiteOperationKey {
	return ops.SiteOperationKey{
		AccountID:   plan.AccountID,
		SiteDomain:  plan.ClusterName,
		OperationID: plan.OperationID,
	}
}
