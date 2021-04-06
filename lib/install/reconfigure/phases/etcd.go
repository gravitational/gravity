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

	"github.com/gravitational/gravity/lib/clients"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/state"

	etcd "github.com/coreos/etcd/client"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// NewEtcd returns executor that updates etcd member with new advertise URL.
func NewEtcd(p fsm.ExecutorParams, operator ops.Operator) (*etcdExecutor, error) {
	logger := &fsm.Logger{
		FieldLogger: logrus.WithField(constants.FieldPhase, p.Phase.ID),
		Key:         p.Key(),
		Operator:    operator,
		Server:      p.Phase.Data.Server,
	}
	// Connect to the local etcd server.
	stateDir, err := state.GetStateDir()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	etcdClient, err := clients.EtcdMembers(&clients.EtcdConfig{
		SecretsDir: state.SecretDir(stateDir),
	})
	if err != nil {
		return nil, trace.Wrap(err)
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

// Execute updates local etcd member with a new advertise URL.
func (p *etcdExecutor) Execute(ctx context.Context) error {
	p.Progress.NextStep("Updating etcd member with a new advertise URL")
	// Find the member. Should be just 1.
	members, err := p.Etcd.List(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	if len(members) != 1 {
		return trace.BadParameter("expected 1 etcd member, got: %v", members)
	}
	// Update the member's advertise URL.
	member := members[0]
	err = p.Etcd.Update(ctx, member.ID, []string{p.Phase.Data.Server.EtcdPeerURL()})
	if err != nil {
		return trace.Wrap(err)
	}
	p.Infof("Updated etcd member %v with advertise URL %v.", member, p.Phase.Data.Server.EtcdPeerURL())
	return nil
}

// Rollback is no-op for this phase.
func (*etcdExecutor) Rollback(ctx context.Context) error {
	return nil
}

// PreCheck is no-op for this phase.
func (*etcdExecutor) PreCheck(ctx context.Context) error {
	return nil
}

// PostCheck is no-op for this phase.
func (*etcdExecutor) PostCheck(ctx context.Context) error {
	return nil
}
