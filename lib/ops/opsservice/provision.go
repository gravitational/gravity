package opsservice

import (
	"context"
	"time"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/trace"
)

func (s *site) verifyPermissionsAWS(ctx *operationContext) (err error) {
	aws := s.cloudProvider().(*aws)
	deniedActions, err := aws.verifyPermissions(context.TODO(), &s.app.Manifest)
	if err != nil {
		return trace.Wrap(err)
	}
	if len(deniedActions) > 0 {
		const deniedActionsMessage = "Your AWS API key is missing permissions required to complete the operation:\n%v"
		policy, err := deniedActions.AsPolicy(s.app.Manifest.Providers.AWS.IAMPolicy.Version)
		if err != nil {
			return trace.Wrap(err, "failed to format AWS permissions as policy")
		}
		ctx.Errorf(deniedActionsMessage, policy)
		return trace.AccessDenied(deniedActionsMessage, policy)
	}
	return nil
}

func (s *site) connectAndConfigureServers(ctx *operationContext) error {
	s.reportProgress(ctx, ops.ProgressEntry{
		State:      ops.ProgressStateInProgress,
		Completion: 15,
		Message:    "connecting to servers",
	})

	localCtx, cancel := defaults.WithTimeout(context.TODO())
	agentReport, err := s.waitForAgents(localCtx, ctx)
	cancel()
	if err != nil {
		if agentReport != nil {
			ctx.RecordError("failed to connect to servers: %v", agentReport.Message)
		} else {
			ctx.RecordError("failed to connect to servers")
		}
		return trace.Wrap(err)
	}

	s.reportProgress(ctx, ops.ProgressEntry{
		State:      ops.ProgressStateInProgress,
		Completion: 20,
		Message:    "connected to servers",
	})

	agents, err := s.prepareForProvisioning(ctx, agentReport)
	if err != nil {
		return trace.Wrap(err)
	}

	servers := make([]storage.Server, 0, len(agentReport.Servers))
	counter := 0
	for _, profile := range ctx.profiles() {
		for i := 0; i < profile.Request.Count; i++ {
			info := agentReport.Servers[counter]
			agent := agents[counter]
			counter++
			if info.CloudMetadata == nil {
				return trace.BadParameter("failed to fetch AWS metadata for %v", info)
			}
			// ServerProfile is either set explicitly by agent
			// or is set via legacy `KubernetesRole` aws tag
			// that we are going to deprecate
			var serverProfile string
			if info.Role != "" && info.Role != ops.AgentAutoRole {
				serverProfile = info.Role
				// FIXME: is this still relevant?
				// } else if info.CloudMetadata.Role != "" {
				// 	serverProfile = info.AWSMeta.Role
			} else {
				return trace.BadParameter("no server profile set for node %v", agent.Hostname)
			}

			servers = append(servers, storage.Server{
				Hostname:     agent.Hostname,
				Nodename:     info.CloudMetadata.NodeName,
				InstanceType: info.CloudMetadata.InstanceType,
				InstanceID:   info.CloudMetadata.InstanceId,
				AdvertiseIP:  agent.AdvertiseIP,
				Role:         serverProfile,
				User:         info.GetUser(),
				Provisioner:  schema.ProvisionerAWSTerraform,
				Created:      time.Now().UTC(),
			})
		}
	}

	err = s.setOperationServers(ctx.key(), servers)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// prepareForProvisioning obtains remote system details from the specified agent report
// as a list of ops.Server instances and updates the state of the current operation
func (s *site) prepareForProvisioning(ctx *operationContext, agentReport *ops.AgentReport) (remoteServers []storage.Server, err error) {
	remoteServers = make([]storage.Server, 0, len(agentReport.Servers))
	for _, info := range agentReport.Servers {
		systemDevice := info.GetDevices().GetByName(storage.DeviceName(info.SystemDevice))
		dockerDevice := info.GetDevices().GetByName(storage.DeviceName(info.DockerDevice))
		mounts := make([]storage.Mount, 0, len(info.Mounts))
		for _, mount := range info.Mounts {
			mounts = append(mounts, storage.Mount{Name: mount.Name, Source: mount.Source})
		}
		remoteServers = append(remoteServers, storage.Server{
			Hostname:    info.GetHostname(),
			AdvertiseIP: info.AdvertiseAddr,
			OSInfo:      info.GetOS(),
			SystemState: storage.SystemState{
				Device:   systemDevice,
				StateDir: info.StateDir,
			},
			Docker: storage.Docker{
				Device:             dockerDevice,
				LVMSystemDirectory: info.GetLVMSystemDirectory(),
			},
			Mounts: mounts,
			User:   info.GetUser(),
		})
	}
	if ctx.operation.InstallExpand != nil {
		// save servers in operation state to mimick behavior
		// of an onprem install
		operation, err := s.getSiteOperation(ctx.operation.ID)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		operation.InstallExpand.Servers = remoteServers
		updated, err := s.updateSiteOperation(operation)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		ctx.operation = *updated
	}
	return remoteServers, nil
}
