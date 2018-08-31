package phases

import (
	"context"
	"fmt"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/trace"

	etcd "github.com/coreos/etcd/client"
	"github.com/gravitational/logrus"
)

// NewEtcd returns executor that adds a new etcd member to the cluster
func NewEtcd(p fsm.ExecutorParams, operator ops.Operator, etcd etcd.MembersAPI) (*etcdExecutor, error) {
	logger := &fsm.Logger{
		FieldLogger: logrus.WithFields(logrus.Fields{
			constants.FieldInstallPhase: p.Phase.ID,
		}),
		Key:      opKey(p.Plan),
		Operator: operator,
	}
	return &etcdExecutior{
		FieldLogger:    logger,
		Operator:       operator,
		Etcd:           etcd,
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
