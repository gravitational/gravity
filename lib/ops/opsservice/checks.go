package opsservice

import (
	"context"

	"github.com/gravitational/gravity/lib/ops"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// ValidateServers runs preflight checks before the installation
func (o *Operator) ValidateServers(req ops.ValidateServersRequest) error {
	log.Infof("Validating servers: %#v.", req)

	op, err := o.GetSiteOperation(req.OperationKey())
	if err != nil {
		return trace.Wrap(err)
	}

	cluster, err := o.openSite(req.SiteKey())
	if err != nil {
		return trace.Wrap(err)
	}

	infos, err := cluster.agentService().GetServerInfos(context.TODO(), op.Key())
	if err != nil {
		return trace.Wrap(err)
	}

	err = ops.CheckServers(context.TODO(), op.Key(), infos, req.Servers,
		cluster.agentService(), cluster.app.Manifest)
	if err != nil {
		return trace.Wrap(ops.FormatValidationError(err))
	}

	return nil
}
