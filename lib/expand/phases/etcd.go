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
	"fmt"

	"github.com/gravitational/gravity/lib/clients"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/state"
	"github.com/gravitational/gravity/lib/storage"

	etcd "github.com/coreos/etcd/client"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// NewEtcd returns executor that adds a new etcd member to the cluster
func NewEtcd(p fsm.ExecutorParams, operator ops.Operator) (*etcdExecutor, error) {
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
	}
	return &etcdExecutor{
		FieldLogger:    logger,
		Etcd:           etcdClient,
		ExecutorParams: p,
	}, nil
}

type etcdExecutor struct {
	// FieldLogger is used for logging
	logrus.FieldLogger
	// Etcd is client to the cluster's etcd members API
	Etcd etcd.MembersAPI
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
	// TODO Implement rollback
	return trace.NotImplemented("implement me!")
}

// PreCheck is no-op for this phase
func (*etcdExecutor) PreCheck(ctx context.Context) error {
	return nil
}

// PostCheck is no-op for this phase
func (*etcdExecutor) PostCheck(ctx context.Context) error {
	return nil
}

func opKey(plan storage.OperationPlan) ops.SiteOperationKey {
	return ops.SiteOperationKey{
		AccountID:   plan.AccountID,
		SiteDomain:  plan.ClusterName,
		OperationID: plan.OperationID,
	}
}
