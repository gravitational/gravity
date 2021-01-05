// Copyright 2021 Gravitational Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package install

import (
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/gravitational/gravity/e/lib/ops"
	"github.com/gravitational/gravity/lib/checks"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/httplib"
	"github.com/gravitational/gravity/lib/install"
	validationpb "github.com/gravitational/gravity/lib/network/validation/proto"
	ossops "github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/cenkalti/backoff"
	teledefaults "github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/trace"
)

// StartOpsCenterInstall starts installation that was initiated by an
// Ops Center
func (i *Installer) StartOpsCenterInstall() error {
	// fetch operation details from the remote Ops Center
	err := i.Operator.RequestClusterCopy(ops.ClusterCopyRequest{
		AccountID:   defaults.SystemAccountID,
		ClusterName: i.SiteDomain,
		OperationID: i.Config.OperationID,
		OpsURL:      i.Config.RemoteOpsURL,
		OpsToken:    i.Config.RemoteOpsToken,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	i.Cluster, err = i.Operator.GetSiteByDomain(i.SiteDomain)
	if err != nil {
		return trace.Wrap(err)
	}
	// perform local checks on the installer agent
	err = checks.RunLocalChecks(checks.LocalChecksRequest{
		Context:  i.Context,
		Manifest: i.Cluster.App.Manifest,
		Role:     i.Role,
		Docker:   i.Docker,
		Options: &validationpb.ValidateOptions{
			VxlanPort: int32(i.VxlanPort),
			DnsAddrs:  i.DNSConfig.Addrs,
			DnsPort:   int32(i.DNSConfig.Port),
		},
		AutoFix: true,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	// establish a reverse tunnel back to the remote Ops Center for the
	// duration of installation so the Ops Center can call our API
	err = i.establishOpsCenterTrust()
	if err != nil {
		return trace.Wrap(err)
	}
	// provisioning token is required so connecting RPC agents can be
	// authenticated by the agent server
	err = i.createProvisioningToken()
	if err != nil {
		return trace.Wrap(err)
	}
	i.OperationKey = ossops.SiteOperationKey{
		AccountID:   defaults.SystemAccountID,
		SiteDomain:  i.SiteDomain,
		OperationID: i.Config.OperationID,
	}
	// start RPC agent
	agentURL, err := i.agentURL()
	if err != nil {
		return trace.Wrap(err)
	}
	agent, err := i.StartAgent(agentURL)
	if err != nil {
		return trace.Wrap(err)
	}
	go agent.Serve()
	ticker := backoff.NewTicker(backoff.NewConstantBackOff(1 * time.Second))
	defer ticker.Stop()
	for {
		select {
		case <-i.Context.Done():
			return trace.Wrap(i.Context.Err())
		case <-ticker.C:
			op, err := i.Operator.GetSiteOperation(i.OperationKey)
			if err != nil {
				i.Errorf("Failed to query operation: %v.", trace.DebugReport(err))
				continue
			}
			if op.State != ossops.OperationStateReady {
				i.Debug("Operation is not ready yet.")
				continue
			}
			i.Info("Operation is ready!")
			if op.IsAWS() {
				err = i.UpdateOperationState()
				if err != nil {
					return trace.Wrap(err)
				}
			}
			err = i.StartOperation()
			if err != nil {
				return trace.Wrap(err, "failed to kick off installation")
			}
			go i.PollProgress(agent.Done())
			return nil
		}
	}
}

// NewClusterRequest constructs a request to create a new cluster
func (i *Installer) NewClusterRequest() ossops.NewSiteRequest {
	return ossops.NewSiteRequest{
		AppPackage:   i.AppPackage.String(),
		AccountID:    i.AccountID,
		Email:        fmt.Sprintf("installer@%v", i.SiteDomain),
		Provider:     i.CloudProvider,
		DomainName:   i.SiteDomain,
		License:      i.Config.License,
		InstallToken: i.Config.Token,
		ServiceUser: storage.OSUser{
			Name: i.Config.ServiceUser.Name,
			UID:  strconv.Itoa(i.Config.ServiceUser.UID),
			GID:  strconv.Itoa(i.Config.ServiceUser.GID),
		},
		CloudConfig: storage.CloudConfig{
			GCENodeTags: i.Config.GCENodeTags,
		},
		DNSOverrides: i.DNSOverrides,
		DNSConfig:    i.DNSConfig,
		Docker:       i.Docker,
	}
}

// GetFSM returns the installer FSM engine
func (i *Installer) GetFSM() (*fsm.FSM, error) {
	fsmConfig := install.FSMConfig{
		OperationKey:       i.OperationKey,
		Packages:           i.Packages,
		Apps:               i.Apps,
		Operator:           i.Operator,
		LocalClusterClient: i.Config.LocalClusterClient,
		LocalPackages:      i.LocalPackages,
		LocalApps:          i.LocalApps,
		LocalBackend:       i.LocalBackend,
		RemoteOpsURL:       i.Config.RemoteOpsURL,
		RemoteOpsToken:     i.Config.RemoteOpsToken,
		Insecure:           i.Insecure,
		UserLogFile:        i.UserLogFile,
		ReportProgress:     true,
	}
	fsmConfig.Spec = FSMSpec(fsmConfig)
	return install.NewFSM(fsmConfig)
}

// establishOpsCenterTrust creates a trusted cluster for the remote Ops Center
func (i *Installer) establishOpsCenterTrust() error {
	opsHost, opsPort, err := utils.URLSplitHostPort(
		i.Config.RemoteOpsURL, defaults.HTTPSPort)
	if err != nil {
		return trace.Wrap(err)
	}
	trustedCluster := storage.NewTrustedCluster(opsHost,
		storage.TrustedClusterSpecV2{
			Enabled:      true,
			Token:        i.Config.OpsTunnelToken,
			ProxyAddress: fmt.Sprintf("%v:%v", opsHost, opsPort),
			ReverseTunnelAddress: fmt.Sprintf("%v:%v", opsHost,
				teledefaults.SSHProxyTunnelListenPort),
			Roles: []string{constants.RoleAdmin},
		})
	trustedCluster.SetSystem(true)
	return i.Operator.UpsertTrustedCluster(ossops.SiteKey{
		AccountID:  defaults.SystemAccountID,
		SiteDomain: i.SiteDomain,
	}, trustedCluster)
}

// createProvisioningToken creates a new install token which agent server
// uses to authenticate connecting agents
func (i *Installer) createProvisioningToken() error {
	return i.Operator.CreateProvisioningToken(storage.ProvisioningToken{
		AccountID:   defaults.SystemAccountID,
		SiteDomain:  i.SiteDomain,
		OperationID: i.Config.OperationID,
		Token:       i.Token.Token,
		Type:        storage.ProvisioningTokenTypeInstall,
		UserEmail:   defaults.WizardUser,
		Expires:     time.Now().UTC().Add(defaults.InstallTokenTTL),
	})
}

// agentURL returns the agent server URL
func (i *Installer) agentURL() (string, error) {
	agentURL, err := url.Parse(fmt.Sprintf("agent://%v/%v",
		i.Process.AgentService().ServerAddr(), i.Role))
	if err != nil {
		return "", trace.Wrap(err)
	}
	query := agentURL.Query()
	query.Set(httplib.AccessTokenQueryParam, i.Token.Token)
	if i.CloudProvider == schema.ProviderAWS {
		query.Set(ossops.AgentProvisioner, schema.ProvisionerAWSTerraform)
	}
	agentURL.RawQuery = query.Encode()
	return agentURL.String(), nil
}
