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

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/ops/opsclient"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/trace"
	"github.com/pborman/uuid"
	log "github.com/sirupsen/logrus"
)

// requestUninstall is called by cluster and makes a request to the remote
// Ops Center the cluster is connected to to initiate the uninstall operation
func (s *site) requestUninstall(ctx context.Context, req ops.CreateSiteUninstallOperationRequest) (*ops.SiteOperationKey, error) {
	// check the status of remote access - we can request uninstall only if
	// the remote support to Ops Center is turned on
	clusters, err := s.service.cfg.Users.GetTrustedClusters()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var client *opsclient.Client
	for _, cluster := range clusters {
		if !cluster.GetEnabled() {
			continue
		}
		client, err = s.service.RemoteOpsClient(cluster)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		break
	}

	if client == nil {
		return nil, trace.BadParameter(
			"this cluster is not connected to a Gravity Hub")
	}

	// create an uninstall operation for ourselves in the remote Ops Center
	key, err := client.CreateSiteUninstallOperation(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// set the local cluster state to "uninstalling"
	err = s.setSiteState(ops.SiteStateUninstalling)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return key, nil
}

// createUninstallOperation initiates uninstall operation and starts it right away
func (s *site) createUninstallOperation(context context.Context, req ops.CreateSiteUninstallOperationRequest) (*ops.SiteOperationKey, error) {
	opInstall, _, err := ops.GetInstallOperation(s.key, s.service)
	if err != nil {
		return nil, trace.Wrap(err, "failed to query install operation")
	}

	op := &ops.SiteOperation{
		ID:          uuid.New(),
		AccountID:   s.key.AccountID,
		SiteDomain:  s.key.SiteDomain,
		Type:        ops.OperationUninstall,
		Created:     s.clock().UtcNow(),
		CreatedBy:   ops.UserFromContext(context),
		Updated:     s.clock().UtcNow(),
		State:       ops.OperationStateUninstallInProgress,
		Provisioner: opInstall.Provisioner,
		Uninstall: &storage.UninstallOperationState{
			Force: req.Force,
			Vars:  opInstall.InstallExpand.Vars,
		},
	}

	ctx, err := s.newOperationContext(*op)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer ctx.Close()

	err = s.updateRequestVars(ctx, &req.Variables, op)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = s.service.setCloudProviderFromRequest(s.key, op.Provisioner, &req.Variables)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if isAWSProvisioner(op.Provisioner) {
		// verify the provided keys are valid
		err = s.verifyPermissionsAWS(ctx)
		if err != nil {
			return nil, trace.BadParameter("invalid AWS credentials")
		}
	}

	key, err := s.getOperationGroup().createSiteOperationWithOptions(*op,
		createOperationOptions{force: req.Force})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	s.reportProgress(ctx, ops.ProgressEntry{
		State:      ops.ProgressStateInProgress,
		Completion: 0,
		Message:    "initializing the operation",
	})

	err = s.executeOperation(*key, s.uninstallOperationStart)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return key, nil
}

// uninstallOperationStart kicks off actuall uninstall process:
// deprovisions servers, deletes packages
func (s *site) uninstallOperationStart(ctx *operationContext) error {
	state := ctx.operation.Uninstall

	s.reportProgress(ctx, ops.ProgressEntry{
		State:      ops.ProgressStateInProgress,
		Completion: 10,
		Message:    "uninstalling applications",
	})

	// execute commands attempting to run uninstall hooks
	err := s.uninstallUserApp(ctx)
	if err != nil {
		if !state.Force {
			return trace.Wrap(err, "failed to uninstall user app")
		}
		log.Warningf("forced uninstall: failed to uninstall user app: %v",
			trace.DebugReport(err))
	}

	// if user has supplied custom hooks, call them
	if isAWSProvisioner(ctx.operation.Provisioner) {
		if !s.app.Manifest.HasHook(schema.HookClusterDeprovision) {
			return trace.BadParameter("%v hook is not defined",
				schema.HookClusterDeprovision)
		}
		err := s.runClusterDeprovisionHook(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	s.reportProgress(ctx, ops.ProgressEntry{
		State:      ops.ProgressStateCompleted,
		Completion: constants.Completed,
		Message:    "uninstall completed",
	})

	return trace.Wrap(s.deleteSite())
}

func (s *site) uninstallUserApp(ctx *operationContext) error {
	log.Infof("uninstallUserApp")

	s.reportProgress(ctx, ops.ProgressEntry{
		State:      ops.ProgressStateInProgress,
		Completion: 20,
		Message:    "uninstalling user application",
	})

	runner := &teleportRunner{ctx, s.domainName, s.teleport()}

	master, err := s.getTeleportServerNoRetry(schema.ServiceLabelRole,
		string(schema.ServiceRoleMaster))
	if err != nil {
		return trace.Wrap(err)
	}

	userAppPackage, err := s.appPackage()
	if err != nil {
		return trace.Wrap(err)
	}

	command := s.planetGravityCommand("app", "package-uninstall", userAppPackage.String())
	out, err := runner.Run(master, command...)
	if err != nil {
		return trace.Wrap(err)
	}

	log.Infof("app uninstall output: %s", out)

	return nil
}
