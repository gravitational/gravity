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
