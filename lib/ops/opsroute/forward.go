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

package opsroute

import (
	"context"
	"fmt"
	"io"
	"net/url"

	"github.com/gravitational/gravity/lib/clients"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/modules"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/ops/opsservice"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/storage/clusterconfig"

	teleservices "github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
)

// RouterConfig specifies config parameters for Router
type RouterConfig struct {
	// Backend is a storage backend
	Backend storage.Backend
	// Local is local ops service
	Local *opsservice.Operator
	// Wizard is true if this is an install wizard process
	Wizard bool
	// Clients provides access to clients for remote clusters such as operator or apps
	Clients *clients.ClusterClients
}

// NewRouter returns new router instance
func NewRouter(conf RouterConfig) (*Router, error) {
	if conf.Backend == nil {
		return nil, trace.BadParameter("missing parameter Backend")
	}
	if conf.Local == nil {
		return nil, trace.BadParameter("missing parameter Local")
	}
	if conf.Clients == nil {
		return nil, trace.BadParameter("missing parameter Clients")
	}
	return &Router{
		RouterConfig: conf,
	}, nil
}

// Router routes requests either to a local ops center
// or remote link based on the site status
// it is used in ops center mode to make sure we are using local gravity site
// state when possible
type Router struct {
	RouterConfig
}

func (r *Router) RemoteClient(siteName string) (ops.Operator, error) {
	site, err := r.Backend.GetSite(siteName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if site.Local {
		return r.Local, nil
	}
	client, err := r.Clients.OpsClient(siteName)
	return client, trace.Wrap(err)
}

// WizardClient returns the operator client for an install wizard process that
// keeps a reverse tunnel to this Ops Center
//
// If this process is install wizard itself, then local operator is returned.
func (r *Router) WizardClient(clusterName string) (ops.Operator, error) {
	if r.Wizard {
		return r.Local, nil
	}
	client, err := r.Clients.OpsClient(fmt.Sprintf("%v%v",
		constants.InstallerTunnelPrefix, clusterName))
	return client, trace.Wrap(err)
}

// PickClient picks active client based on its state - if the site is installed,
// it picks remote tunnel HTTP client, otherwise it picks local ops center
// service
func (r *Router) PickClient(siteName string) (ops.Operator, error) {
	site, err := r.Backend.GetSite(siteName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if site.Local {
		return r.Local, nil
	}
	if !ops.IsInstalledState(site.State) {
		return r.Local, nil
	}
	return r.RemoteClient(siteName)
}

// PickOperationClient selects an appropriate operator service to perform a site operation
func (r *Router) PickOperationClient(siteName string) (ops.Operator, error) {
	return r.PickClient(siteName)
}

func (r *Router) GetLocalOperator() ops.Operator {
	return r.Local
}

func (r *Router) GetCurrentUser() (storage.User, error) {
	return r.Local.GetCurrentUser()
}

func (r *Router) GetCurrentUserInfo() (*ops.UserInfo, error) {
	return r.Local.GetCurrentUserInfo()
}

func (r *Router) GetAccount(accountID string) (*ops.Account, error) {
	return r.Local.GetAccount(accountID)
}

func (r *Router) CreateAccount(req ops.NewAccountRequest) (*ops.Account, error) {
	return r.Local.CreateAccount(req)
}

func (r *Router) GetAccounts() ([]ops.Account, error) {
	return r.Local.GetAccounts()
}

func (r *Router) CreateUser(req ops.NewUserRequest) error {
	return r.Local.CreateUser(req)
}

// UpdateUser updates the specified user information.
func (r *Router) UpdateUser(ctx context.Context, req ops.UpdateUserRequest) error {
	client, err := r.PickClient(req.SiteDomain)
	if err != nil {
		return trace.Wrap(err)
	}
	return client.UpdateUser(ctx, req)
}

func (r *Router) DeleteLocalUser(name string) error {
	return r.Local.DeleteLocalUser(name)
}

func (r *Router) GetLocalUser(key ops.SiteKey) (storage.User, error) {
	client, err := r.PickClient(key.SiteDomain)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return client.GetLocalUser(key)
}

func (r *Router) GetClusterAgent(req ops.ClusterAgentRequest) (*storage.LoginEntry, error) {
	client, err := r.PickClient(req.ClusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return client.GetClusterAgent(req)
}

// GetClusterNodes returns a real-time information about cluster nodes
func (r *Router) GetClusterNodes(key ops.SiteKey) ([]ops.Node, error) {
	client, err := r.PickClient(key.SiteDomain)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return client.GetClusterNodes(key)
}

func (r *Router) ResetUserPassword(req ops.ResetUserPasswordRequest) (string, error) {
	client, err := r.PickClient(req.SiteDomain)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return client.ResetUserPassword(req)
}

func (r *Router) CreateAPIKey(ctx context.Context, req ops.NewAPIKeyRequest) (*storage.APIKey, error) {
	return r.Local.CreateAPIKey(ctx, req)
}

func (r *Router) GetAPIKeys(userEmail string) ([]storage.APIKey, error) {
	return r.Local.GetAPIKeys(userEmail)
}

func (r *Router) DeleteAPIKey(ctx context.Context, userEmail, token string) error {
	return r.Local.DeleteAPIKey(ctx, userEmail, token)
}

func (r *Router) CreateSite(req ops.NewSiteRequest) (*ops.Site, error) {
	return r.Local.CreateSite(req)
}

func (r *Router) GetSites(accountID string) ([]ops.Site, error) {
	return r.Local.GetSites(accountID)
}

func (r *Router) DeleteSite(siteKey ops.SiteKey) error {
	return r.Local.DeleteSite(siteKey)
}

func (r *Router) GetSiteByDomain(domainName string) (*ops.Site, error) {
	client, err := r.PickClient(domainName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return client.GetSiteByDomain(domainName)
}

func (r *Router) GetSite(siteKey ops.SiteKey) (*ops.Site, error) {
	client, err := r.PickClient(siteKey.SiteDomain)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return client.GetSite(siteKey)
}

func (r *Router) GetLocalSite() (*ops.Site, error) {
	return r.Local.GetLocalSite()
}

func (r *Router) DeactivateSite(req ops.DeactivateSiteRequest) error {
	client, err := r.RemoteClient(req.SiteDomain)
	if err != nil {
		return trace.Wrap(err)
	}
	return client.DeactivateSite(req)
}

func (r *Router) ActivateSite(req ops.ActivateSiteRequest) error {
	client, err := r.RemoteClient(req.SiteDomain)
	if err != nil {
		return trace.Wrap(err)
	}
	return client.ActivateSite(req)
}

func (r *Router) CompleteFinalInstallStep(req ops.CompleteFinalInstallStepRequest) error {
	client, err := r.RemoteClient(req.SiteDomain)
	if err != nil {
		return trace.Wrap(err)
	}
	return client.CompleteFinalInstallStep(req)
}

// CheckSiteStatus runs app status hook and updates site status appropriately
func (r *Router) CheckSiteStatus(ctx context.Context, key ops.SiteKey) error {
	client, err := r.RemoteClient(key.SiteDomain)
	if err != nil {
		return trace.Wrap(err)
	}
	return client.CheckSiteStatus(ctx, key)
}

func (r *Router) GetSiteInstructions(tokenID string, serverProfile string, params url.Values) (string, error) {
	token, err := r.Backend.GetProvisioningToken(tokenID)
	if err != nil {
		return "", trace.Wrap(err)
	}
	client, err := r.PickOperationClient(token.SiteDomain)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return client.GetSiteInstructions(tokenID, serverProfile, params)
}

func (r *Router) GetSiteOperations(key ops.SiteKey) (ops.SiteOperations, error) {
	client, err := r.PickOperationClient(key.SiteDomain)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return client.GetSiteOperations(key)
}

func (r *Router) GetSiteOperation(key ops.SiteOperationKey) (*ops.SiteOperation, error) {
	client, err := r.PickOperationClient(key.SiteDomain)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return client.GetSiteOperation(key)
}

func (r *Router) CreateSiteInstallOperation(ctx context.Context, req ops.CreateSiteInstallOperationRequest) (*ops.SiteOperationKey, error) {
	return r.Local.CreateSiteInstallOperation(ctx, req)
}

func (r *Router) ResumeShrink(key ops.SiteKey) (*ops.SiteOperationKey, error) {
	return r.Local.ResumeShrink(key)
}

func (r *Router) CreateSiteExpandOperation(ctx context.Context, req ops.CreateSiteExpandOperationRequest) (*ops.SiteOperationKey, error) {
	client, err := r.PickOperationClient(req.SiteDomain)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return client.CreateSiteExpandOperation(ctx, req)
}

func (r *Router) CreateSiteShrinkOperation(ctx context.Context, req ops.CreateSiteShrinkOperationRequest) (*ops.SiteOperationKey, error) {
	client, err := r.PickOperationClient(req.SiteDomain)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return client.CreateSiteShrinkOperation(ctx, req)
}

func (r *Router) CreateSiteAppUpdateOperation(ctx context.Context, req ops.CreateSiteAppUpdateOperationRequest) (*ops.SiteOperationKey, error) {
	client, err := r.RemoteClient(req.SiteDomain)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return client.CreateSiteAppUpdateOperation(ctx, req)
}

func (r *Router) GetSiteInstallOperationAgentReport(key ops.SiteOperationKey) (*ops.AgentReport, error) {
	client, err := r.WizardClient(key.SiteDomain)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return client.GetSiteInstallOperationAgentReport(key)
}

func (r *Router) SiteInstallOperationStart(key ops.SiteOperationKey) error {
	return r.Local.SiteInstallOperationStart(key)
}

func (r *Router) CreateSiteUninstallOperation(ctx context.Context, req ops.CreateSiteUninstallOperationRequest) (*ops.SiteOperationKey, error) {
	return r.Local.CreateSiteUninstallOperation(ctx, req)
}

// CreateClusterGarbageCollectOperation creates a new garbage collection operation in the cluster
func (r *Router) CreateClusterGarbageCollectOperation(ctx context.Context, req ops.CreateClusterGarbageCollectOperationRequest) (*ops.SiteOperationKey, error) {
	return r.Local.CreateClusterGarbageCollectOperation(ctx, req)
}

// CreateClusterReconfigureOperation creates a new cluster reconfiguration operation.
func (r *Router) CreateClusterReconfigureOperation(ctx context.Context, req ops.CreateClusterReconfigureOperationRequest) (*ops.SiteOperationKey, error) {
	return r.Local.CreateClusterReconfigureOperation(ctx, req)
}

// CreateUpdateEnvarsOperation creates a new operation to update cluster runtime environment variables
func (r *Router) CreateUpdateEnvarsOperation(ctx context.Context, req ops.CreateUpdateEnvarsOperationRequest) (*ops.SiteOperationKey, error) {
	return r.Local.CreateUpdateEnvarsOperation(ctx, req)
}

// CreateUpdateConfigOperation creates a new operation to update cluster configuration
func (r *Router) CreateUpdateConfigOperation(ctx context.Context, req ops.CreateUpdateConfigOperationRequest) (*ops.SiteOperationKey, error) {
	return r.Local.CreateUpdateConfigOperation(ctx, req)
}

func (r *Router) GetSiteOperationLogs(key ops.SiteOperationKey) (io.ReadCloser, error) {
	client, err := r.PickOperationClient(key.SiteDomain)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return client.GetSiteOperationLogs(key)
}

func (r *Router) CreateLogEntry(key ops.SiteOperationKey, entry ops.LogEntry) error {
	client, err := r.PickOperationClient(key.SiteDomain)
	if err != nil {
		return trace.Wrap(err)
	}
	return client.CreateLogEntry(key, entry)
}

// StreamOperationLogs appends the logs from the provided reader to the
// specified operation (user-facing) log file
func (r *Router) StreamOperationLogs(key ops.SiteOperationKey, reader io.Reader) error {
	client, err := r.PickOperationClient(key.SiteDomain)
	if err != nil {
		return trace.Wrap(err)
	}
	return client.StreamOperationLogs(key, reader)
}

func (r *Router) GetSiteExpandOperationAgentReport(key ops.SiteOperationKey) (*ops.AgentReport, error) {
	client, err := r.PickOperationClient(key.SiteDomain)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return client.GetSiteExpandOperationAgentReport(key)
}

func (r *Router) SiteExpandOperationStart(key ops.SiteOperationKey) error {
	client, err := r.PickOperationClient(key.SiteDomain)
	if err != nil {
		return trace.Wrap(err)
	}
	return client.SiteExpandOperationStart(key)
}

func (r *Router) GetSiteOperationProgress(key ops.SiteOperationKey) (*ops.ProgressEntry, error) {
	client, err := r.PickOperationClient(key.SiteDomain)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return client.GetSiteOperationProgress(key)
}

func (r *Router) CreateProgressEntry(key ops.SiteOperationKey, entry ops.ProgressEntry) error {
	client, err := r.PickOperationClient(key.SiteDomain)
	if err != nil {
		return trace.Wrap(err)
	}
	return client.CreateProgressEntry(key, entry)
}

func (r *Router) GetSiteReport(req ops.GetClusterReportRequest) (io.ReadCloser, error) {
	client, err := r.PickClient(req.SiteDomain)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return client.GetSiteReport(req)
}

// ValidateServers runs pre-installation checks
func (r *Router) ValidateServers(ctx context.Context, req ops.ValidateServersRequest) (*ops.ValidateServersResponse, error) {
	client, err := r.WizardClient(req.SiteDomain)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return client.ValidateServers(ctx, req)
}

func (r *Router) ValidateDomainName(domainName string) error {
	return r.Local.ValidateDomainName(domainName)
}

func (r *Router) ValidateRemoteAccess(req ops.ValidateRemoteAccessRequest) (*ops.ValidateRemoteAccessResponse, error) {
	return r.Local.ValidateRemoteAccess(req)
}

func (r *Router) UpdateInstallOperationState(key ops.SiteOperationKey, req ops.OperationUpdateRequest) (err error) {
	// in the cloud provisioner use-case update the requested server profiles
	// in the Ops Center since there are no remote servers yet
	if len(req.Servers) == 0 {
		return r.Local.UpdateInstallOperationState(key, req)
	}
	// in the onprem use-case update the servers directly in the installer
	client, err := r.WizardClient(key.SiteDomain)
	if err != nil {
		return trace.Wrap(err)
	}
	return client.UpdateInstallOperationState(key, req)
}

func (r *Router) UpdateExpandOperationState(key ops.SiteOperationKey, req ops.OperationUpdateRequest) error {
	client, err := r.PickOperationClient(key.SiteDomain)
	if err != nil {
		return trace.Wrap(err)
	}
	return client.UpdateExpandOperationState(key, req)
}

func (r *Router) DeleteSiteOperation(key ops.SiteOperationKey) error {
	client, err := r.PickOperationClient(key.SiteDomain)
	if err != nil {
		return trace.Wrap(err)
	}
	return client.DeleteSiteOperation(key)
}

func (r *Router) SetOperationState(key ops.SiteOperationKey, req ops.SetOperationStateRequest) error {
	client, err := r.PickOperationClient(key.SiteDomain)
	if err != nil {
		return trace.Wrap(err)
	}
	return client.SetOperationState(key, req)
}

// CreateOperationPlan saves the provided operation plan
func (r *Router) CreateOperationPlan(key ops.SiteOperationKey, plan storage.OperationPlan) error {
	client, err := r.PickOperationClient(key.SiteDomain)
	if err != nil {
		return trace.Wrap(err)
	}
	return client.CreateOperationPlan(key, plan)
}

// CreateOperationPlanChange creates a new changelog entry for a plan
func (r *Router) CreateOperationPlanChange(key ops.SiteOperationKey, change storage.PlanChange) error {
	client, err := r.PickOperationClient(key.SiteDomain)
	if err != nil {
		return trace.Wrap(err)
	}
	return client.CreateOperationPlanChange(key, change)
}

// GetOperationPlan returns plan for the specified operation
func (r *Router) GetOperationPlan(key ops.SiteOperationKey) (*storage.OperationPlan, error) {
	client, err := r.PickOperationClient(key.SiteDomain)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return client.GetOperationPlan(key)
}

// Configure packages configures packages for the specified install operation
func (r *Router) ConfigurePackages(req ops.ConfigurePackagesRequest) error {
	client, err := r.PickOperationClient(req.SiteDomain)
	if err != nil {
		return trace.Wrap(err)
	}
	return client.ConfigurePackages(req)
}

func (r *Router) RotateSecrets(req ops.RotateSecretsRequest) (*ops.RotatePackageResponse, error) {
	return r.Local.RotateSecrets(req)
}

func (r *Router) RotatePlanetConfig(req ops.RotatePlanetConfigRequest) (*ops.RotatePackageResponse, error) {
	return r.Local.RotatePlanetConfig(req)
}

func (r *Router) RotateTeleportConfig(req ops.RotateTeleportConfigRequest) (*ops.RotatePackageResponse, *ops.RotatePackageResponse, error) {
	return r.Local.RotateTeleportConfig(req)
}

func (r *Router) ConfigureNode(req ops.ConfigureNodeRequest) error {
	return r.Local.ConfigureNode(req)
}

// GetLogForwarders returns a list of configured log forwarders
func (r *Router) GetLogForwarders(key ops.SiteKey) ([]storage.LogForwarder, error) {
	client, err := r.RemoteClient(key.SiteDomain)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return client.GetLogForwarders(key)
}

// CreateLogForwarder creates a new log forwarder
func (r *Router) CreateLogForwarder(ctx context.Context, key ops.SiteKey, forwarder storage.LogForwarder) error {
	client, err := r.RemoteClient(key.SiteDomain)
	if err != nil {
		return trace.Wrap(err)
	}
	return client.CreateLogForwarder(ctx, key, forwarder)
}

// UpdateLogForwarder updates an existing log forwarder
func (r *Router) UpdateLogForwarder(ctx context.Context, key ops.SiteKey, forwarder storage.LogForwarder) error {
	client, err := r.RemoteClient(key.SiteDomain)
	if err != nil {
		return trace.Wrap(err)
	}
	return client.UpdateLogForwarder(ctx, key, forwarder)
}

// DeleteLogForwarder deletes a log forwarder
func (r *Router) DeleteLogForwarder(ctx context.Context, key ops.SiteKey, forwarderName string) error {
	client, err := r.RemoteClient(key.SiteDomain)
	if err != nil {
		return trace.Wrap(err)
	}
	return client.DeleteLogForwarder(ctx, key, forwarderName)
}

// GetClusterMetrics returns basic CPU/RAM metrics for the specified cluster.
func (r *Router) GetClusterMetrics(ctx context.Context, req ops.ClusterMetricsRequest) (*ops.ClusterMetricsResponse, error) {
	client, err := r.PickClient(req.SiteDomain)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return client.GetClusterMetrics(ctx, req)
}

// GetSMTPConfig returns the cluster SMTP configuration
func (r *Router) GetSMTPConfig(key ops.SiteKey) (storage.SMTPConfig, error) {
	client, err := r.RemoteClient(key.SiteDomain)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return client.GetSMTPConfig(key)
}

// UpdateSMTPConfig updates the cluster SMTP configuration
func (r *Router) UpdateSMTPConfig(ctx context.Context, key ops.SiteKey, config storage.SMTPConfig) error {
	client, err := r.RemoteClient(key.SiteDomain)
	if err != nil {
		return trace.Wrap(err)
	}
	return client.UpdateSMTPConfig(ctx, key, config)
}

// DeleteSMTPConfig deletes the cluster SMTP configuration
func (r *Router) DeleteSMTPConfig(ctx context.Context, key ops.SiteKey) error {
	client, err := r.RemoteClient(key.SiteDomain)
	if err != nil {
		return trace.Wrap(err)
	}
	return client.DeleteSMTPConfig(ctx, key)
}

// GetAlerts returns a list of monitoring alerts
func (r *Router) GetAlerts(key ops.SiteKey) ([]storage.Alert, error) {
	client, err := r.RemoteClient(key.SiteDomain)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return client.GetAlerts(key)
}

// UpdateAlert updates the specified monitoring alert
func (r *Router) UpdateAlert(ctx context.Context, key ops.SiteKey, alert storage.Alert) error {
	client, err := r.RemoteClient(key.SiteDomain)
	if err != nil {
		return trace.Wrap(err)
	}
	return client.UpdateAlert(ctx, key, alert)
}

// DeleteAlert deletes the monitoring alert specified with name
func (r *Router) DeleteAlert(ctx context.Context, key ops.SiteKey, name string) error {
	client, err := r.RemoteClient(key.SiteDomain)
	if err != nil {
		return trace.Wrap(err)
	}
	return client.DeleteAlert(ctx, key, name)
}

// GetAlertTargets returns a list of monitoring alert targets
func (r *Router) GetAlertTargets(key ops.SiteKey) ([]storage.AlertTarget, error) {
	client, err := r.RemoteClient(key.SiteDomain)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return client.GetAlertTargets(key)
}

// UpdateAlertTarget updates the cluster monitoring alert target
func (r *Router) UpdateAlertTarget(ctx context.Context, key ops.SiteKey, target storage.AlertTarget) error {
	client, err := r.RemoteClient(key.SiteDomain)
	if err != nil {
		return trace.Wrap(err)
	}
	return client.UpdateAlertTarget(ctx, key, target)
}

// DeleteAlertTarget deletes the cluster monitoring alert target
func (r *Router) DeleteAlertTarget(ctx context.Context, key ops.SiteKey) error {
	client, err := r.RemoteClient(key.SiteDomain)
	if err != nil {
		return trace.Wrap(err)
	}
	return client.DeleteAlertTarget(ctx, key)
}

// GetClusterEnvironmentVariables retrieves the cluster runtime environment variables
func (r *Router) GetClusterEnvironmentVariables(key ops.SiteKey) (storage.EnvironmentVariables, error) {
	client, err := r.RemoteClient(key.SiteDomain)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return client.GetClusterEnvironmentVariables(key)
}

// UpdateClusterEnvironmentVariables updates the cluster runtime environment variables
// from the specified request
func (r *Router) UpdateClusterEnvironmentVariables(req ops.UpdateClusterEnvironRequest) error {
	client, err := r.RemoteClient(req.ClusterKey.SiteDomain)
	if err != nil {
		return trace.Wrap(err)
	}
	return client.UpdateClusterEnvironmentVariables(req)
}

// GetClusterConfiguration retrieves the cluster configuration
func (r *Router) GetClusterConfiguration(key ops.SiteKey) (clusterconfig.Interface, error) {
	client, err := r.RemoteClient(key.SiteDomain)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return client.GetClusterConfiguration(key)
}

// UpdateClusterConfiguration updates the cluster configuration from the specified request
func (r *Router) UpdateClusterConfiguration(req ops.UpdateClusterConfigRequest) error {
	client, err := r.RemoteClient(req.ClusterKey.SiteDomain)
	if err != nil {
		return trace.Wrap(err)
	}
	return client.UpdateClusterConfiguration(req)
}

// GetPersistentStorage retrieves cluster persistent storage configuration.
func (r *Router) GetPersistentStorage(ctx context.Context, key ops.SiteKey) (storage.PersistentStorage, error) {
	client, err := r.PickClient(key.SiteDomain)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return client.GetPersistentStorage(ctx, key)
}

// UpdatePersistentStorage updates persistent storage configuration.
func (r *Router) UpdatePersistentStorage(ctx context.Context, req ops.UpdatePersistentStorageRequest) error {
	client, err := r.PickClient(req.SiteDomain)
	if err != nil {
		return trace.Wrap(err)
	}
	return client.UpdatePersistentStorage(ctx, req)
}

func (r *Router) GetApplicationEndpoints(key ops.SiteKey) ([]ops.Endpoint, error) {
	client, err := r.RemoteClient(key.SiteDomain)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return client.GetApplicationEndpoints(key)
}

func (r *Router) CreateInstallToken(req ops.NewInstallTokenRequest) (*storage.InstallToken, error) {
	return r.Local.CreateInstallToken(req)
}

func (r *Router) CreateProvisioningToken(token storage.ProvisioningToken) error {
	return r.Local.CreateProvisioningToken(token)
}

func (r *Router) GetExpandToken(key ops.SiteKey) (*storage.ProvisioningToken, error) {
	client, err := r.PickClient(key.SiteDomain)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return client.GetExpandToken(key)
}

func (r *Router) GetTrustedClusterToken(key ops.SiteKey) (storage.Token, error) {
	return r.Local.GetTrustedClusterToken(key)
}

// SignTLSKey signs X509 Public Key with X509 certificate authority of this site
func (r *Router) SignTLSKey(req ops.TLSSignRequest) (*ops.TLSSignResponse, error) {
	return r.Local.SignTLSKey(req)
}

// SignSSHKey signs SSH Public Key with SSH user certificate authority of this site
func (r *Router) SignSSHKey(req ops.SSHSignRequest) (*ops.SSHSignResponse, error) {
	return r.Local.SignSSHKey(req)
}

func (r *Router) GetAppInstaller(req ops.AppInstallerRequest) (io.ReadCloser, error) {
	return r.Local.GetAppInstaller(req)
}

// GetClusterCertificate returns the cluster certificate
func (r *Router) GetClusterCertificate(key ops.SiteKey, withSecrets bool) (*ops.ClusterCertificate, error) {
	client, err := r.RemoteClient(key.SiteDomain)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return client.GetClusterCertificate(key, withSecrets)
}

// UpdateClusterCertificate updates the cluster certificate
func (r *Router) UpdateClusterCertificate(ctx context.Context, req ops.UpdateCertificateRequest) (*ops.ClusterCertificate, error) {
	client, err := r.RemoteClient(req.SiteDomain)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return client.UpdateClusterCertificate(ctx, req)
}

// DeleteClusterCertificate deletes the cluster certificate
func (r *Router) DeleteClusterCertificate(ctx context.Context, key ops.SiteKey) error {
	client, err := r.RemoteClient(key.SiteDomain)
	if err != nil {
		return trace.Wrap(err)
	}
	return client.DeleteClusterCertificate(ctx, key)
}

// StepDown asks the process to pause its leader election heartbeat so it can
// give up its leadership
func (r *Router) StepDown(key ops.SiteKey) error {
	return r.Local.StepDown(key)
}

// UpsertUser creates or updates a user
func (r *Router) UpsertUser(ctx context.Context, key ops.SiteKey, user teleservices.User) error {
	client, err := r.PickClient(key.SiteDomain)
	if err != nil {
		return trace.Wrap(err)
	}
	return client.UpsertUser(ctx, key, user)
}

// GetUser returns a user by name
func (r *Router) GetUser(key ops.SiteKey, name string) (teleservices.User, error) {
	client, err := r.PickClient(key.SiteDomain)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return client.GetUser(key, name)
}

// GetUsers returns all users
func (r *Router) GetUsers(key ops.SiteKey) ([]teleservices.User, error) {
	client, err := r.PickClient(key.SiteDomain)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return client.GetUsers(key)
}

// DeleteUser deletes a user by name
func (r *Router) DeleteUser(ctx context.Context, key ops.SiteKey, name string) error {
	client, err := r.PickClient(key.SiteDomain)
	if err != nil {
		return trace.Wrap(err)
	}
	return client.DeleteUser(ctx, key, name)
}

// UpsertClusterAuthPreference updates cluster authentication preference
func (r *Router) UpsertClusterAuthPreference(ctx context.Context, key ops.SiteKey, auth teleservices.AuthPreference) error {
	client, err := r.PickClient(key.SiteDomain)
	if err != nil {
		return trace.Wrap(err)
	}
	return client.UpsertClusterAuthPreference(ctx, key, auth)
}

// GetClusterAuthPreference returns cluster authentication preference
func (r *Router) GetClusterAuthPreference(key ops.SiteKey) (teleservices.AuthPreference, error) {
	client, err := r.PickClient(key.SiteDomain)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return client.GetClusterAuthPreference(key)
}

// UpsertGithubConnector creates or updates a Github connector
func (r *Router) UpsertGithubConnector(ctx context.Context, key ops.SiteKey, connector teleservices.GithubConnector) error {
	client, err := r.PickClient(key.SiteDomain)
	if err != nil {
		return trace.Wrap(err)
	}
	return client.UpsertGithubConnector(ctx, key, connector)
}

// GetGithubConnector returns a Github connector by name
//
// Returned connector exclude client secret unless withSecrets is true.
func (r *Router) GetGithubConnector(key ops.SiteKey, name string, withSecrets bool) (teleservices.GithubConnector, error) {
	client, err := r.PickClient(key.SiteDomain)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return client.GetGithubConnector(key, name, withSecrets)
}

// GetGithubConnectors returns all Github connectors
//
// Returned connectors exclude client secret unless withSecrets is true.
func (r *Router) GetGithubConnectors(key ops.SiteKey, withSecrets bool) ([]teleservices.GithubConnector, error) {
	client, err := r.PickClient(key.SiteDomain)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return client.GetGithubConnectors(key, withSecrets)
}

// DeleteGithubConnector deletes a Github connector by name
func (r *Router) DeleteGithubConnector(ctx context.Context, key ops.SiteKey, name string) error {
	client, err := r.PickClient(key.SiteDomain)
	if err != nil {
		return trace.Wrap(err)
	}
	return client.DeleteGithubConnector(ctx, key, name)
}

// UpsertAuthGateway updates auth gateway configuration.
func (r *Router) UpsertAuthGateway(ctx context.Context, key ops.SiteKey, gw storage.AuthGateway) error {
	return r.Local.UpsertAuthGateway(ctx, key, gw)
}

// GetAuthGateway returns auth gateway configuration.
func (r *Router) GetAuthGateway(key ops.SiteKey) (storage.AuthGateway, error) {
	client, err := r.PickClient(key.SiteDomain)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return client.GetAuthGateway(key)
}

// ListReleases returns all currently installed application releases in a cluster.
func (r *Router) ListReleases(req ops.ListReleasesRequest) ([]storage.Release, error) {
	client, err := r.PickClient(req.SiteDomain)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return client.ListReleases(req)
}

// EmitAuditEvent saves the provided event in the audit log.
func (r *Router) EmitAuditEvent(ctx context.Context, req ops.AuditEventRequest) error {
	return r.Local.EmitAuditEvent(ctx, req)
}

// GetVersion returns the gravity binary version information.
func (r *Router) GetVersion(ctx context.Context) (*modules.Version, error) {
	return r.Local.GetVersion(ctx)
}

// CreateUserInvite creates a new invite token for a user.
func (r *Router) CreateUserInvite(ctx context.Context, req ops.CreateUserInviteRequest) (*storage.UserToken, error) {
	client, err := r.PickClient(req.SiteDomain)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return client.CreateUserInvite(ctx, req)
}

// GetUserInvites returns all active user invites.
func (r *Router) GetUserInvites(ctx context.Context, key ops.SiteKey) ([]storage.UserInvite, error) {
	client, err := r.PickClient(key.SiteDomain)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return client.GetUserInvites(ctx, key)
}

// DeleteUserInvite deletes the specified user invite.
func (r *Router) DeleteUserInvite(ctx context.Context, req ops.DeleteUserInviteRequest) error {
	client, err := r.PickClient(req.SiteDomain)
	if err != nil {
		return trace.Wrap(err)
	}
	return client.DeleteUserInvite(ctx, req)
}

// CreateUserInvite creates a new reset token for a user.
func (r *Router) CreateUserReset(ctx context.Context, req ops.CreateUserResetRequest) (*storage.UserToken, error) {
	client, err := r.PickClient(req.SiteDomain)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return client.CreateUserReset(ctx, req)
}
