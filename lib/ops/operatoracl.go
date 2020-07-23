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

package ops

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"io"
	"net/url"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/modules"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/storage/clusterconfig"
	"github.com/gravitational/gravity/lib/users"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/cloudflare/cfssl/csr"
	"github.com/cloudflare/cfssl/signer"
	teledefaults "github.com/gravitational/teleport/lib/defaults"
	teleservices "github.com/gravitational/teleport/lib/services"
	teleutils "github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// OperatorWithACL retruns new instance of the Operator interface
// that is checking every action against this username privileges
func OperatorWithACL(operator Operator, users users.Identity, user storage.User, checker teleservices.AccessChecker) *OperatorACL {
	return &OperatorACL{
		operator:    operator,
		users:       users,
		user:        user,
		username:    user.GetName(),
		checker:     checker,
		FieldLogger: log.WithField(trace.Component, "acl"),
	}
}

// OperatorACL is a wrapper around any Operator service that
// implements ACLs - access control lists for every operation
type OperatorACL struct {
	operator Operator
	users    users.Identity
	username string
	checker  teleservices.AccessChecker
	user     storage.User
	log.FieldLogger
}

type localOperator interface {
	// GetLocalOperator retrieves the local operator from the opsrouter
	GetLocalOperator() Operator
}

func (o *OperatorACL) context() *users.Context {
	return &users.Context{Context: teleservices.Context{User: o.user}}
}

// resourceContext returns context for the provided resource.
func (o *OperatorACL) resourceContext(resource teleservices.Resource) *users.Context {
	return &users.Context{
		Context: teleservices.Context{
			User:     o.user,
			Resource: resource,
		},
	}
}

func (o *OperatorACL) clusterContext(clusterName string) (*users.Context, storage.Cluster, error) {
	site, err := o.operator.GetSiteByDomain(clusterName)
	if err != nil {
		o.Warnf("Falling back to local operator: %v.", err)
		localOperator, ok := o.operator.(localOperator)
		if !ok {
			return nil, nil, trace.Wrap(err)
		}
		site, err = localOperator.GetLocalOperator().GetSiteByDomain(clusterName)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
	}
	cluster := NewClusterFromSite(*site)
	return o.resourceContext(cluster), cluster, nil
}

// Action checks access to the specified action on the specified resource kind
func (o *OperatorACL) Action(resourceKind, action string) error {
	return o.checker.CheckAccessToRule(o.context(), defaults.Namespace,
		resourceKind, action, false)
}

func (o *OperatorACL) ClusterAction(clusterName, resourceKind, action string) error {
	ctx, cluster, err := o.clusterContext(clusterName)
	if err != nil {
		return trace.Wrap(err)
	}
	return o.checker.CheckAccessToRule(ctx, cluster.GetMetadata().Namespace, resourceKind, action, false)
}

func (o *OperatorACL) repoContext(repoName string) *users.Context {
	return o.resourceContext(storage.NewRepository(repoName))
}

// currentUserAction is a special checker that allows certain actions for users
// even if they are not admins, e.g. update their own passwords,
// or generate certificates, otherwise it will require admin privileges
func (o *OperatorACL) currentUserActions(username string, actions ...string) error {
	if username == o.username {
		return nil
	}
	return o.userActions(actions...)
}

// userActions checks access to the specified actions on the "user" resource
func (o *OperatorACL) userActions(actions ...string) error {
	for _, action := range actions {
		if err := o.Action(teleservices.KindUser, action); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// authPreferenceActions checks access to the specified actions on the "cluster
// auth preference" resource
func (o *OperatorACL) authPreferenceActions(actions ...string) error {
	for _, action := range actions {
		if err := o.Action(teleservices.KindClusterAuthPreference, action); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// AuthConnectorActions checks access to the specified actions on the "auth
// connector" resource
//
// First, access to the provided specific connector type is checked, e.g.
// "oidc" or "saml". If that fails, then access to a generic "auth_connector"
// resource type (that encompasses all kinds of connectors) is checked.
func (o *OperatorACL) AuthConnectorActions(connectorKind string, actions ...string) error {
	if !utils.StringInSlice(modules.Get().SupportedConnectors(), connectorKind) {
		return trace.BadParameter("expected one of %v connector kinds, got: %v",
			modules.Get().SupportedConnectors(), connectorKind)
	}
	for _, action := range actions {
		if err := o.Action(connectorKind, action); err != nil {
			if err := o.Action(teleservices.KindAuthConnector, action); err != nil {
				return trace.Wrap(err)
			}
		}
	}
	return nil
}

func (o *OperatorACL) GetAccount(accountID string) (*Account, error) {
	if err := o.Action(storage.KindCluster, teleservices.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return o.operator.GetAccount(accountID)
}

func (o *OperatorACL) CreateAccount(req NewAccountRequest) (*Account, error) {
	if err := o.Action(storage.KindCluster, teleservices.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}
	return o.operator.CreateAccount(req)
}

func (o *OperatorACL) GetAccounts() ([]Account, error) {
	if err := o.Action(storage.KindCluster, teleservices.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}
	return o.operator.GetAccounts()
}

func (o *OperatorACL) CreateUser(req NewUserRequest) error {
	if err := o.Action(teleservices.KindUser, teleservices.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	return o.operator.CreateUser(req)
}

// UpdateUser updates the specified user information.
func (o *OperatorACL) UpdateUser(ctx context.Context, req UpdateUserRequest) error {
	if err := o.Action(teleservices.KindUser, teleservices.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return o.operator.UpdateUser(ctx, req)
}

func (o *OperatorACL) DeleteLocalUser(name string) error {
	if err := o.Action(teleservices.KindUser, teleservices.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return o.operator.DeleteLocalUser(name)
}

func (o *OperatorACL) GetLocalUser(key SiteKey) (storage.User, error) {
	if err := o.Action(teleservices.KindUser, teleservices.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return o.operator.GetLocalUser(key)
}

func (o *OperatorACL) GetClusterAgent(req ClusterAgentRequest) (*storage.LoginEntry, error) {
	if err := o.ClusterAction(req.ClusterName, storage.KindCluster, teleservices.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return o.operator.GetClusterAgent(req)
}

// GetClusterNodes returns a real-time information about cluster nodes
func (o *OperatorACL) GetClusterNodes(key SiteKey) ([]Node, error) {
	if err := o.ClusterAction(key.SiteDomain, storage.KindCluster, teleservices.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return o.operator.GetClusterNodes(key)
}

func (o *OperatorACL) ResetUserPassword(req ResetUserPasswordRequest) (string, error) {
	if err := o.Action(teleservices.KindUser, teleservices.VerbUpdate); err != nil {
		return "", trace.Wrap(err)
	}
	return o.operator.ResetUserPassword(req)
}

func (o *OperatorACL) CreateAPIKey(ctx context.Context, req NewAPIKeyRequest) (*storage.APIKey, error) {
	if err := o.currentUserActions(req.UserEmail, teleservices.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}
	return o.operator.CreateAPIKey(ctx, req)
}

func (o *OperatorACL) GetAPIKeys(userEmail string) ([]storage.APIKey, error) {
	if err := o.currentUserActions(userEmail, teleservices.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return o.operator.GetAPIKeys(userEmail)
}

func (o *OperatorACL) DeleteAPIKey(ctx context.Context, userEmail, token string) error {
	if err := o.currentUserActions(userEmail, teleservices.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return o.operator.DeleteAPIKey(ctx, userEmail, token)
}

func (o *OperatorACL) CreateInstallToken(req NewInstallTokenRequest) (*storage.InstallToken, error) {
	// TODO(klizhentas) introduce more fine grained RBAC, right now
	// we use this Update requirement to limit access to admin only users
	if err := o.Action(storage.KindCluster, teleservices.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	return o.operator.CreateInstallToken(req)
}

func (o *OperatorACL) CreateProvisioningToken(token storage.ProvisioningToken) error {
	// TODO(klizhentas) introduce more fine grained RBAC, right now
	// we use this Update requirement to limit access to admin only users
	if err := o.Action(storage.KindCluster, teleservices.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return o.operator.CreateProvisioningToken(token)
}

func (o *OperatorACL) GetExpandToken(key SiteKey) (*storage.ProvisioningToken, error) {
	// TODO(klizhentas) introduce more fine grained RBAC, right now
	// we use this Update requirement to limit access to admin only users
	if err := o.ClusterAction(key.SiteDomain, storage.KindCluster, teleservices.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	return o.operator.GetExpandToken(key)
}

func (o *OperatorACL) GetTrustedClusterToken(key SiteKey) (storage.Token, error) {
	if err := o.ClusterAction(key.SiteDomain, storage.KindCluster, teleservices.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return o.operator.GetTrustedClusterToken(key)
}

func (o *OperatorACL) CreateSite(req NewSiteRequest) (*Site, error) {
	err := o.Action(storage.KindCluster, teleservices.VerbCreate)
	if err == nil {
		return o.operator.CreateSite(req)
	}
	// 1st case is when there's a special one-time install token
	token, err := o.users.GetInstallTokenByUser(o.username)
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.AccessDenied("user %v can not create clusters", o.username)
		}
		return nil, trace.BadParameter("internal server error")
	}
	l, err := loc.ParseLocator(req.AppPackage)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// we are going to update the install token to reduce the
	// scope of the token, it will return a new role
	_, role, err := o.users.UpdateInstallToken(users.InstallTokenUpdateRequest{
		Token:      token.Token,
		SiteDomain: req.DomainName,
		Repository: l.Repository,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// update checker to use new role that has extended permissions
	o.checker = teleservices.NewRoleSet(role)

	return o.operator.CreateSite(req)
}

func (o *OperatorACL) GetSites(accountID string) ([]Site, error) {
	allClusters, err := o.operator.GetSites(accountID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// return only the clusters we have access to
	var clusters []Site
	for _, cluster := range allClusters {
		if err := o.ClusterAction(cluster.Domain, storage.KindCluster, teleservices.VerbRead); err != nil {
			continue
		}
		clusters = append(clusters, cluster)
	}
	return clusters, nil
}

func (o *OperatorACL) GetLocalSite() (*Site, error) {
	return o.operator.GetLocalSite()
}

func (o *OperatorACL) DeleteSite(siteKey SiteKey) error {
	if err := o.ClusterAction(siteKey.SiteDomain, storage.KindCluster, teleservices.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return o.operator.DeleteSite(siteKey)
}

func (o *OperatorACL) GetSiteByDomain(domainName string) (*Site, error) {
	if err := o.ClusterAction(domainName, storage.KindCluster, teleservices.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return o.operator.GetSiteByDomain(domainName)
}

func (o *OperatorACL) GetSite(siteKey SiteKey) (site *Site, err error) {
	if err := o.ClusterAction(siteKey.SiteDomain, storage.KindCluster, teleservices.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return o.operator.GetSite(siteKey)
}

func (o *OperatorACL) GetAppInstaller(req AppInstallerRequest) (io.ReadCloser, error) {
	if err := o.checker.CheckAccessToRule(o.repoContext(req.Application.Repository), teledefaults.Namespace, storage.KindApp, teleservices.VerbRead, false); err != nil {
		return nil, trace.Wrap(err)
	}
	return o.operator.GetAppInstaller(req)
}

func (o *OperatorACL) DeactivateSite(req DeactivateSiteRequest) error {
	if err := o.ClusterAction(req.SiteDomain, storage.KindCluster, teleservices.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return o.operator.DeactivateSite(req)
}

func (o *OperatorACL) ActivateSite(req ActivateSiteRequest) error {
	if err := o.ClusterAction(req.SiteDomain, storage.KindCluster, teleservices.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return o.operator.ActivateSite(req)
}

func (o *OperatorACL) CompleteFinalInstallStep(req CompleteFinalInstallStepRequest) error {
	if err := o.ClusterAction(req.SiteDomain, storage.KindCluster, teleservices.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return o.operator.CompleteFinalInstallStep(req)
}

func (o *OperatorACL) CheckSiteStatus(ctx context.Context, key SiteKey) error {
	// TODO(klizhentas) introduce more fine grained RBAC, right now
	// we use this Update requirement to limit access to admin only users
	// as this can modify cluster state
	if err := o.ClusterAction(key.SiteDomain, storage.KindCluster, teleservices.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return o.operator.CheckSiteStatus(ctx, key)
}

// ValidateServers runs pre-installation checks
func (o *OperatorACL) ValidateServers(ctx context.Context, req ValidateServersRequest) (*ValidateServersResponse, error) {
	if err := o.ClusterAction(req.SiteDomain, storage.KindCluster, teleservices.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return o.operator.ValidateServers(ctx, req)
}

func (o *OperatorACL) GetSiteInstructions(tokenID string, serverProfile string, params url.Values) (string, error) {
	// tokenID is the private token that grants access by itself to the site
	// so no extra checks are necessary
	return o.operator.GetSiteInstructions(tokenID, serverProfile, params)
}

func (o *OperatorACL) GetSiteOperations(key SiteKey) (SiteOperations, error) {
	if err := o.ClusterAction(key.SiteDomain, storage.KindCluster, teleservices.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return o.operator.GetSiteOperations(key)
}

func (o *OperatorACL) GetSiteOperation(key SiteOperationKey) (*SiteOperation, error) {
	if err := o.ClusterAction(key.SiteDomain, storage.KindCluster, teleservices.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return o.operator.GetSiteOperation(key)
}

func (o *OperatorACL) CreateSiteInstallOperation(ctx context.Context, req CreateSiteInstallOperationRequest) (*SiteOperationKey, error) {
	if err := o.ClusterAction(req.SiteDomain, storage.KindCluster, teleservices.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	return o.operator.CreateSiteInstallOperation(ctx, req)
}

func (o *OperatorACL) ResumeShrink(key SiteKey) (*SiteOperationKey, error) {
	if err := o.ClusterAction(key.SiteDomain, storage.KindCluster, teleservices.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	return o.operator.ResumeShrink(key)
}

func (o *OperatorACL) CreateSiteExpandOperation(ctx context.Context, req CreateSiteExpandOperationRequest) (*SiteOperationKey, error) {
	if err := o.ClusterAction(req.SiteDomain, storage.KindCluster, teleservices.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	return o.operator.CreateSiteExpandOperation(ctx, req)
}

func (o *OperatorACL) CreateSiteShrinkOperation(ctx context.Context, req CreateSiteShrinkOperationRequest) (*SiteOperationKey, error) {
	if err := o.ClusterAction(req.SiteDomain, storage.KindCluster, teleservices.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	return o.operator.CreateSiteShrinkOperation(ctx, req)
}

func (o *OperatorACL) CreateSiteAppUpdateOperation(ctx context.Context, req CreateSiteAppUpdateOperationRequest) (*SiteOperationKey, error) {
	if err := o.ClusterAction(req.SiteDomain, storage.KindCluster, teleservices.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	return o.operator.CreateSiteAppUpdateOperation(ctx, req)
}

func (o *OperatorACL) GetSiteInstallOperationAgentReport(key SiteOperationKey) (*AgentReport, error) {
	if err := o.ClusterAction(key.SiteDomain, storage.KindCluster, teleservices.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return o.operator.GetSiteInstallOperationAgentReport(key)
}

func (o *OperatorACL) SiteInstallOperationStart(key SiteOperationKey) error {
	if err := o.ClusterAction(key.SiteDomain, storage.KindCluster, teleservices.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return o.operator.SiteInstallOperationStart(key)
}

func (o *OperatorACL) CreateSiteUninstallOperation(ctx context.Context, req CreateSiteUninstallOperationRequest) (*SiteOperationKey, error) {
	if err := o.ClusterAction(req.SiteDomain, storage.KindCluster, teleservices.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	return o.operator.CreateSiteUninstallOperation(ctx, req)
}

// CreateClusterGarbageCollectOperation creates a new garbage collection operation in the cluster
func (o *OperatorACL) CreateClusterGarbageCollectOperation(ctx context.Context, req CreateClusterGarbageCollectOperationRequest) (*SiteOperationKey, error) {
	if err := o.ClusterAction(req.ClusterName, storage.KindCluster, teleservices.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	return o.operator.CreateClusterGarbageCollectOperation(ctx, req)
}

// CreateClusterReconfigureOperation creates a new cluster reconfiguration operation.
func (o *OperatorACL) CreateClusterReconfigureOperation(ctx context.Context, req CreateClusterReconfigureOperationRequest) (*SiteOperationKey, error) {
	if err := o.ClusterAction(req.SiteDomain, storage.KindCluster, teleservices.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	return o.operator.CreateClusterReconfigureOperation(ctx, req)
}

// CreateUpdateEnvarsOperation creates a new operation to update cluster environment variables
func (o *OperatorACL) CreateUpdateEnvarsOperation(ctx context.Context, req CreateUpdateEnvarsOperationRequest) (*SiteOperationKey, error) {
	if err := o.ClusterAction(req.ClusterKey.SiteDomain, storage.KindCluster, teleservices.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	return o.operator.CreateUpdateEnvarsOperation(ctx, req)
}

// CreateUpdateConfigOperation creates a new operation to update cluster configuration
func (o *OperatorACL) CreateUpdateConfigOperation(ctx context.Context, req CreateUpdateConfigOperationRequest) (*SiteOperationKey, error) {
	if err := o.ClusterAction(req.ClusterKey.SiteDomain, storage.KindCluster, teleservices.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	return o.operator.CreateUpdateConfigOperation(ctx, req)
}

func (o *OperatorACL) GetSiteOperationLogs(key SiteOperationKey) (io.ReadCloser, error) {
	if err := o.ClusterAction(key.SiteDomain, storage.KindCluster, teleservices.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return o.operator.GetSiteOperationLogs(key)
}

func (o *OperatorACL) CreateLogEntry(key SiteOperationKey, entry LogEntry) error {
	if err := o.ClusterAction(key.SiteDomain, storage.KindCluster, teleservices.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return o.operator.CreateLogEntry(key, entry)
}

// StreamOperationLogs appends the logs from the provided reader to the
// specified operation (user-facing) log file
func (o *OperatorACL) StreamOperationLogs(key SiteOperationKey, reader io.Reader) error {
	if err := o.ClusterAction(key.SiteDomain, storage.KindCluster, teleservices.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return o.operator.StreamOperationLogs(key, reader)
}

func (o *OperatorACL) GetSiteExpandOperationAgentReport(key SiteOperationKey) (*AgentReport, error) {
	if err := o.ClusterAction(key.SiteDomain, storage.KindCluster, teleservices.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return o.operator.GetSiteExpandOperationAgentReport(key)
}

func (o *OperatorACL) SiteExpandOperationStart(key SiteOperationKey) error {
	if err := o.ClusterAction(key.SiteDomain, storage.KindCluster, teleservices.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return o.operator.SiteExpandOperationStart(key)
}

func (o *OperatorACL) GetSiteOperationProgress(key SiteOperationKey) (*ProgressEntry, error) {
	if err := o.ClusterAction(key.SiteDomain, storage.KindCluster, teleservices.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return o.operator.GetSiteOperationProgress(key)
}

func (o *OperatorACL) CreateProgressEntry(key SiteOperationKey, entry ProgressEntry) error {
	if err := o.ClusterAction(key.SiteDomain, storage.KindCluster, teleservices.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return o.operator.CreateProgressEntry(key, entry)
}

func (o *OperatorACL) GetSiteReport(req GetClusterReportRequest) (io.ReadCloser, error) {
	if err := o.ClusterAction(req.SiteDomain, storage.KindCluster, teleservices.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return o.operator.GetSiteReport(req)
}

func (o *OperatorACL) ValidateDomainName(domainName string) error {
	if err := o.ClusterAction(domainName, storage.KindCluster, teleservices.VerbRead); err != nil {
		// when installing via a one-time install link, the token does not have
		// any cluster access yet but we need to let it validate the domain name
		// which happens before creating a cluster
		if teleutils.SliceContainsStr(o.user.GetRoles(), constants.RoleOneTimeLink) {
			return trace.Wrap(err)
		}
	}
	return o.operator.ValidateDomainName(domainName)
}

func (o *OperatorACL) ValidateRemoteAccess(req ValidateRemoteAccessRequest) (*ValidateRemoteAccessResponse, error) {
	if err := o.ClusterAction(req.SiteDomain, storage.KindCluster, teleservices.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	return o.operator.ValidateRemoteAccess(req)
}

func (o *OperatorACL) UpdateInstallOperationState(key SiteOperationKey, req OperationUpdateRequest) error {
	if err := o.ClusterAction(key.SiteDomain, storage.KindCluster, teleservices.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return o.operator.UpdateInstallOperationState(key, req)
}

func (o *OperatorACL) UpdateExpandOperationState(key SiteOperationKey, req OperationUpdateRequest) error {
	if err := o.ClusterAction(key.SiteDomain, storage.KindCluster, teleservices.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return o.operator.UpdateExpandOperationState(key, req)
}

func (o *OperatorACL) DeleteSiteOperation(key SiteOperationKey) error {
	if err := o.ClusterAction(key.SiteDomain, storage.KindCluster, teleservices.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return o.operator.DeleteSiteOperation(key)
}

func (o *OperatorACL) SetOperationState(key SiteOperationKey, req SetOperationStateRequest) error {
	if err := o.ClusterAction(key.SiteDomain, storage.KindCluster, teleservices.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return o.operator.SetOperationState(key, req)
}

// CreateOperationPlan saves the provided operation plan
func (o *OperatorACL) CreateOperationPlan(key SiteOperationKey, plan storage.OperationPlan) error {
	if err := o.ClusterAction(key.SiteDomain, storage.KindCluster, teleservices.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return o.operator.CreateOperationPlan(key, plan)
}

// CreateOperationPlanChange creates a new changelog entry for a plan
func (o *OperatorACL) CreateOperationPlanChange(key SiteOperationKey, change storage.PlanChange) error {
	if err := o.ClusterAction(key.SiteDomain, storage.KindCluster, teleservices.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return o.operator.CreateOperationPlanChange(key, change)
}

// GetOperationPlan returns plan for the specified operation
func (o *OperatorACL) GetOperationPlan(key SiteOperationKey) (*storage.OperationPlan, error) {
	if err := o.ClusterAction(key.SiteDomain, storage.KindCluster, teleservices.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return o.operator.GetOperationPlan(key)
}

// Configure packages configures packages for the specified operation
func (o *OperatorACL) ConfigurePackages(req ConfigurePackagesRequest) error {
	if err := o.ClusterAction(req.SiteDomain, storage.KindCluster, teleservices.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return o.operator.ConfigurePackages(req)
}

func (o *OperatorACL) RotateSecrets(req RotateSecretsRequest) (*RotatePackageResponse, error) {
	if err := o.ClusterAction(req.Key.SiteDomain, storage.KindCluster, teleservices.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	return o.operator.RotateSecrets(req)
}

func (o *OperatorACL) RotatePlanetConfig(req RotatePlanetConfigRequest) (*RotatePackageResponse, error) {
	if err := o.ClusterAction(req.Key.SiteDomain, storage.KindCluster, teleservices.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	return o.operator.RotatePlanetConfig(req)
}

func (o *OperatorACL) RotateTeleportConfig(req RotateTeleportConfigRequest) (*RotatePackageResponse, *RotatePackageResponse, error) {
	if err := o.ClusterAction(req.Key.SiteDomain, storage.KindCluster, teleservices.VerbUpdate); err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return o.operator.RotateTeleportConfig(req)
}

func (o *OperatorACL) ConfigureNode(req ConfigureNodeRequest) error {
	if err := o.ClusterAction(req.ClusterName, storage.KindCluster, teleservices.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return o.operator.ConfigureNode(req)
}

// GetLogForwarders returns a list of configured log forwarders
func (o *OperatorACL) GetLogForwarders(key SiteKey) ([]storage.LogForwarder, error) {
	if err := o.ClusterAction(key.SiteDomain, storage.KindLogForwarder, teleservices.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}
	return o.operator.GetLogForwarders(key)
}

// CreateLogForwarder creates a new log forwarder
func (o *OperatorACL) CreateLogForwarder(ctx context.Context, key SiteKey, forwarder storage.LogForwarder) error {
	if err := o.ClusterAction(key.SiteDomain, storage.KindLogForwarder, teleservices.VerbCreate); err != nil {
		return trace.Wrap(err)
	}
	return o.operator.CreateLogForwarder(ctx, key, forwarder)
}

// UpdateLogForwarder updates an existing log forwarder
func (o *OperatorACL) UpdateLogForwarder(ctx context.Context, key SiteKey, forwarder storage.LogForwarder) error {
	if err := o.ClusterAction(key.SiteDomain, storage.KindLogForwarder, teleservices.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return o.operator.UpdateLogForwarder(ctx, key, forwarder)
}

// DeleteLogForwarder deletes a log forwarder
func (o *OperatorACL) DeleteLogForwarder(ctx context.Context, key SiteKey, forwarderName string) error {
	if err := o.ClusterAction(key.SiteDomain, storage.KindLogForwarder, teleservices.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return o.operator.DeleteLogForwarder(ctx, key, forwarderName)
}

// GetClusterMetrics returns basic CPU/RAM metrics for the specified cluster.
func (o *OperatorACL) GetClusterMetrics(ctx context.Context, req ClusterMetricsRequest) (*ClusterMetricsResponse, error) {
	if err := o.ClusterAction(req.SiteDomain, storage.KindCluster, teleservices.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return o.operator.GetClusterMetrics(ctx, req)
}

func (o *OperatorACL) GetSMTPConfig(key SiteKey) (storage.SMTPConfig, error) {
	if err := o.ClusterAction(key.SiteDomain, storage.KindSMTPConfig, teleservices.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return o.operator.GetSMTPConfig(key)
}

func (o *OperatorACL) UpdateSMTPConfig(ctx context.Context, key SiteKey, config storage.SMTPConfig) error {
	if err := o.ClusterAction(key.SiteDomain, storage.KindSMTPConfig, teleservices.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return o.operator.UpdateSMTPConfig(ctx, key, config)
}

func (o *OperatorACL) DeleteSMTPConfig(ctx context.Context, key SiteKey) error {
	if err := o.ClusterAction(key.SiteDomain, storage.KindSMTPConfig, teleservices.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return o.operator.DeleteSMTPConfig(ctx, key)
}

func (o *OperatorACL) GetAlerts(key SiteKey) ([]storage.Alert, error) {
	if err := o.ClusterAction(key.SiteDomain, storage.KindAlert, teleservices.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}
	return o.operator.GetAlerts(key)
}

func (o *OperatorACL) UpdateAlert(ctx context.Context, key SiteKey, alert storage.Alert) error {
	if err := o.ClusterAction(key.SiteDomain, storage.KindAlert, teleservices.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return o.operator.UpdateAlert(ctx, key, alert)
}

func (o *OperatorACL) DeleteAlert(ctx context.Context, key SiteKey, name string) error {
	if err := o.ClusterAction(key.SiteDomain, storage.KindAlert, teleservices.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return o.operator.DeleteAlert(ctx, key, name)
}

func (o *OperatorACL) GetAlertTargets(key SiteKey) ([]storage.AlertTarget, error) {
	if err := o.ClusterAction(key.SiteDomain, storage.KindAlertTarget, teleservices.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}
	return o.operator.GetAlertTargets(key)
}

func (o *OperatorACL) UpdateAlertTarget(ctx context.Context, key SiteKey, target storage.AlertTarget) error {
	if err := o.ClusterAction(key.SiteDomain, storage.KindAlertTarget, teleservices.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return o.operator.UpdateAlertTarget(ctx, key, target)
}

func (o *OperatorACL) DeleteAlertTarget(ctx context.Context, key SiteKey) error {
	if err := o.ClusterAction(key.SiteDomain, storage.KindAlertTarget, teleservices.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return o.operator.DeleteAlertTarget(ctx, key)
}

// GetClusterEnvironmentVariables retrieves the cluster runtime environment variables
func (o *OperatorACL) GetClusterEnvironmentVariables(key SiteKey) (storage.EnvironmentVariables, error) {
	if err := o.ClusterAction(key.SiteDomain, storage.KindRuntimeEnvironment, teleservices.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}
	return o.operator.GetClusterEnvironmentVariables(key)
}

// UpdateClusterEnvironmentVariables updates the cluster runtime environment variables
// from the specified request
func (o *OperatorACL) UpdateClusterEnvironmentVariables(req UpdateClusterEnvironRequest) error {
	if err := o.ClusterAction(req.ClusterKey.SiteDomain, storage.KindClusterConfiguration, teleservices.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return o.operator.UpdateClusterEnvironmentVariables(req)
}

// GetClusterConfiguration retrieves the cluster configuration
func (o *OperatorACL) GetClusterConfiguration(key SiteKey) (clusterconfig.Interface, error) {
	if err := o.ClusterAction(key.SiteDomain, storage.KindClusterConfiguration, teleservices.VerbList); err != nil {
		return nil, trace.Wrap(err)
	}
	return o.operator.GetClusterConfiguration(key)
}

// UpdateClusterConfiguration updates the cluster configuration from the specified request
func (o *OperatorACL) UpdateClusterConfiguration(req UpdateClusterConfigRequest) error {
	if err := o.ClusterAction(req.ClusterKey.SiteDomain, storage.KindClusterConfiguration, teleservices.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return o.operator.UpdateClusterConfiguration(req)
}

// GetPersistentStorage retrieves cluster persistent storage configuration.
func (o *OperatorACL) GetPersistentStorage(ctx context.Context, key SiteKey) (storage.PersistentStorage, error) {
	if err := o.ClusterAction(key.SiteDomain, storage.KindPersistentStorage, teleservices.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return o.operator.GetPersistentStorage(ctx, key)
}

// UpdatePersistentStorage updates persistent storage configuration.
func (o *OperatorACL) UpdatePersistentStorage(ctx context.Context, req UpdatePersistentStorageRequest) error {
	if err := o.ClusterAction(req.SiteDomain, storage.KindPersistentStorage, teleservices.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return o.operator.UpdatePersistentStorage(ctx, req)
}

func (o *OperatorACL) GetApplicationEndpoints(key SiteKey) ([]Endpoint, error) {
	if err := o.ClusterAction(key.SiteDomain, storage.KindCluster, teleservices.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return o.operator.GetApplicationEndpoints(key)
}

// SignTLSKey signs X509 Public Key with X509 certificate authority of this site
func (o *OperatorACL) SignTLSKey(req TLSSignRequest) (*TLSSignResponse, error) {
	ctx, cluster, err := o.clusterContext(req.SiteDomain)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = o.checker.CheckAccessToRule(ctx, cluster.GetMetadata().Namespace, storage.KindCluster, storage.VerbConnect, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	roles, err := teleservices.FetchRoles(ctx.User.GetRoles(), o.users, ctx.User.GetTraits())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	kubernetesGroups, err := roles.CheckKubeGroups(req.TTL)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	block, _ := pem.Decode(req.CSR)
	if block == nil {
		return nil, trace.BadParameter("failed to parse CSR")
	}
	certRequest, err := x509.ParseCertificateRequest(block.Bytes)
	if err != nil {
		log.Debugf("failed to parse CSR: %v", err)
		return nil, trace.BadParameter("failed to parse CSR")
	}

	switch certRequest.Subject.CommonName {
	case o.username, defaults.KubeForwarderUser:
	default:
		if err := o.currentUserActions(o.username, teleservices.VerbCreate); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	req.Subject = &signer.Subject{
		CN: certRequest.Subject.CommonName,
	}
	for _, group := range kubernetesGroups {
		req.Subject.Names = append(req.Subject.Names, csr.Name{O: group})
	}

	return o.operator.SignTLSKey(req)
}

// SignSSHKey signs SSH Public Key with SSH user certificate authority of this site
func (o *OperatorACL) SignSSHKey(req SSHSignRequest) (*SSHSignResponse, error) {
	if err := o.currentUserActions(req.User, teleservices.VerbCreate); err != nil {
		return nil, trace.Wrap(err)
	}
	return o.operator.SignSSHKey(req)
}

func (o *OperatorACL) GetCurrentUser() (storage.User, error) {
	return nil, trace.BadParameter("not implemented")
}

func (o *OperatorACL) GetCurrentUserInfo() (*UserInfo, error) {
	return &UserInfo{
		User: o.user,
	}, nil
}

func (o *OperatorACL) GetClusterCertificate(key SiteKey, withSecrets bool) (*ClusterCertificate, error) {
	if withSecrets {
		if err := o.ClusterAction(key.SiteDomain, storage.KindTLSKeyPair, teleservices.VerbRead); err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		if err := o.ClusterAction(key.SiteDomain, storage.KindCluster, teleservices.VerbRead); err != nil {
			if err := o.ClusterAction(key.SiteDomain, storage.KindTLSKeyPair, teleservices.VerbRead); err != nil {
				return nil, trace.Wrap(err)
			}
		}
	}
	return o.operator.GetClusterCertificate(key, withSecrets)
}

func (o *OperatorACL) UpdateClusterCertificate(ctx context.Context, req UpdateCertificateRequest) (*ClusterCertificate, error) {
	if err := o.ClusterAction(req.SiteDomain, storage.KindCluster, teleservices.VerbUpdate); err != nil {
		return nil, trace.Wrap(err)
	}
	return o.operator.UpdateClusterCertificate(ctx, req)
}

func (o *OperatorACL) DeleteClusterCertificate(ctx context.Context, key SiteKey) error {
	if err := o.ClusterAction(key.SiteDomain, storage.KindCluster, teleservices.VerbUpdate); err != nil {
		if err := o.ClusterAction(key.SiteDomain, storage.KindTLSKeyPair, teleservices.VerbDelete); err != nil {
			return trace.Wrap(err)
		}
	}
	return o.operator.DeleteClusterCertificate(ctx, key)
}

// StepDown asks the process to pause its leader election heartbeat so it can
// give up its leadership
func (o *OperatorACL) StepDown(key SiteKey) error {
	return o.operator.StepDown(key)
}

// UpsertUser creates or updates a user
func (o *OperatorACL) UpsertUser(ctx context.Context, key SiteKey, user teleservices.User) error {
	if err := o.currentUserActions(user.GetName(), teleservices.VerbCreate, teleservices.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return o.operator.UpsertUser(ctx, key, user)
}

// GetUser returns a user by name
func (o *OperatorACL) GetUser(key SiteKey, name string) (teleservices.User, error) {
	if err := o.ClusterAction(key.SiteDomain, storage.KindCluster, teleservices.VerbRead); err != nil {
		if err := o.currentUserActions(name, teleservices.VerbList, teleservices.VerbRead); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return o.operator.GetUser(key, name)
}

// GetUsers returns all users
func (o *OperatorACL) GetUsers(key SiteKey) ([]teleservices.User, error) {
	if err := o.ClusterAction(key.SiteDomain, storage.KindCluster, teleservices.VerbRead); err != nil {
		if err := o.userActions(teleservices.VerbList, teleservices.VerbRead); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return o.operator.GetUsers(key)
}

// DeleteUser deletes a user by name
func (o *OperatorACL) DeleteUser(ctx context.Context, key SiteKey, name string) error {
	if err := o.userActions(teleservices.VerbDelete); err != nil {
		return trace.Wrap(err)
	}
	return o.operator.DeleteUser(ctx, key, name)
}

// UpsertClusterAuthPreference updates cluster authentication preference
func (o *OperatorACL) UpsertClusterAuthPreference(ctx context.Context, key SiteKey, auth teleservices.AuthPreference) error {
	if err := o.authPreferenceActions(teleservices.VerbCreate, teleservices.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return o.operator.UpsertClusterAuthPreference(ctx, key, auth)
}

// GetClusterAuthPreference returns cluster authentication preference
func (o *OperatorACL) GetClusterAuthPreference(key SiteKey) (teleservices.AuthPreference, error) {
	if err := o.authPreferenceActions(teleservices.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return o.operator.GetClusterAuthPreference(key)
}

// UpsertGithubConnector creates or updates a Github connector
func (o *OperatorACL) UpsertGithubConnector(ctx context.Context, key SiteKey, connector teleservices.GithubConnector) error {
	if err := o.ClusterAction(key.SiteDomain, storage.KindCluster, teleservices.VerbUpdate); err != nil {
		if err := o.AuthConnectorActions(teleservices.KindGithubConnector, teleservices.VerbCreate, teleservices.VerbUpdate); err != nil {
			return trace.Wrap(err)
		}
	}
	return o.operator.UpsertGithubConnector(ctx, key, connector)
}

// GetGithubConnector returns a Github connector by name
//
// Returned connector exclude client secret unless withSecrets is true.
func (o *OperatorACL) GetGithubConnector(key SiteKey, name string, withSecrets bool) (teleservices.GithubConnector, error) {
	if err := o.ClusterAction(key.SiteDomain, storage.KindCluster, teleservices.VerbRead); err != nil {
		if err := o.AuthConnectorActions(teleservices.KindGithubConnector, teleservices.VerbRead); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return o.operator.GetGithubConnector(key, name, withSecrets)
}

// GetGithubConnectors returns all Github connectors
//
// Returned connectors exclude client secret unless withSecrets is true.
func (o *OperatorACL) GetGithubConnectors(key SiteKey, withSecrets bool) ([]teleservices.GithubConnector, error) {
	if err := o.ClusterAction(key.SiteDomain, storage.KindCluster, teleservices.VerbRead); err != nil {
		if err := o.AuthConnectorActions(teleservices.KindGithubConnector, teleservices.VerbList, teleservices.VerbRead); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return o.operator.GetGithubConnectors(key, withSecrets)
}

// DeleteGithubConnector deletes a Github connector by name
func (o *OperatorACL) DeleteGithubConnector(ctx context.Context, key SiteKey, name string) error {
	if err := o.ClusterAction(key.SiteDomain, storage.KindCluster, teleservices.VerbUpdate); err != nil {
		if err := o.AuthConnectorActions(teleservices.KindGithubConnector, teleservices.VerbDelete); err != nil {
			return trace.Wrap(err)
		}
	}
	return o.operator.DeleteGithubConnector(ctx, key, name)
}

// UpsertAuthGateway updates auth gateway configuration.
func (o *OperatorACL) UpsertAuthGateway(ctx context.Context, key SiteKey, gw storage.AuthGateway) error {
	if err := o.ClusterAction(key.SiteDomain, storage.KindCluster, teleservices.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return o.operator.UpsertAuthGateway(ctx, key, gw)
}

// GetAuthGateway returns auth gateway configuration.
func (o *OperatorACL) GetAuthGateway(key SiteKey) (storage.AuthGateway, error) {
	if err := o.ClusterAction(key.SiteDomain, storage.KindCluster, teleservices.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return o.operator.GetAuthGateway(key)
}

// ListReleases returns all currently installed application releases in a cluster.
func (o *OperatorACL) ListReleases(req ListReleasesRequest) ([]storage.Release, error) {
	// TODO: Ideally this method would filter out releases a user does not
	// have access to, however Teleport's resources support only a single
	// namespace (default) for now so it is impossible to, for example,
	// create a resource in a different namespace and configure a role
	// to allow/deny access to everything in a certain namespace.
	//
	// Hence, we're returning all releases based on the broader "cluster"
	// permission here but in the future, when Teleport starts respecting
	// namespaces, it might be worth implementing a more granular ACL.
	if err := o.ClusterAction(req.SiteDomain, storage.KindCluster, teleservices.VerbRead); err != nil {
		return nil, trace.Wrap(err)
	}
	return o.operator.ListReleases(req)
}

// EmitAuditEvent saves the provided event in the audit log.
func (o *OperatorACL) EmitAuditEvent(ctx context.Context, req AuditEventRequest) error {
	if err := o.ClusterAction(req.SiteDomain, storage.KindCluster, teleservices.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}
	return o.operator.EmitAuditEvent(ctx, req)
}

// GetVersion returns the server version information.
func (o *OperatorACL) GetVersion(ctx context.Context) (*modules.Version, error) {
	return o.operator.GetVersion(ctx)
}

// CreateUserInvite creates a new invite token for a user.
func (o *OperatorACL) CreateUserInvite(ctx context.Context, req CreateUserInviteRequest) (*storage.UserToken, error) {
	if err := o.ClusterAction(req.SiteDomain, storage.KindCluster, teleservices.VerbUpdate); err != nil {
		if err := o.ClusterAction(req.SiteDomain, storage.KindInvite, teleservices.VerbCreate); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return o.operator.CreateUserInvite(ctx, req)
}

// GetUserInvites returns all active user invites.
func (o *OperatorACL) GetUserInvites(ctx context.Context, key SiteKey) ([]storage.UserInvite, error) {
	if err := o.ClusterAction(key.SiteDomain, storage.KindCluster, teleservices.VerbRead); err != nil {
		if err := o.ClusterAction(key.SiteDomain, storage.KindInvite, teleservices.VerbList); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return o.operator.GetUserInvites(ctx, key)
}

// DeleteUserInvite deletes the specified user invite.
func (o *OperatorACL) DeleteUserInvite(ctx context.Context, req DeleteUserInviteRequest) error {
	if err := o.ClusterAction(req.SiteDomain, storage.KindCluster, teleservices.VerbUpdate); err != nil {
		if err := o.ClusterAction(req.SiteDomain, storage.KindInvite, teleservices.VerbDelete); err != nil {
			return trace.Wrap(err)
		}
	}
	return o.operator.DeleteUserInvite(ctx, req)
}

// CreateUserReset creates a new reset token for a user.
func (o *OperatorACL) CreateUserReset(ctx context.Context, req CreateUserResetRequest) (*storage.UserToken, error) {
	if err := o.ClusterAction(req.SiteDomain, storage.KindCluster, teleservices.VerbUpdate); err != nil {
		if err := o.Action(teleservices.KindUser, teleservices.VerbUpdate); err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return o.operator.CreateUserReset(ctx, req)
}
