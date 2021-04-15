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
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	appservice "github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/storage/clusterconfig"
	"github.com/gravitational/gravity/lib/users"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/cenkalti/backoff"
	"github.com/gravitational/license"
	"github.com/gravitational/rigging"
	"github.com/gravitational/trace"
	"github.com/mailgun/timetools"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// site is an internal helper object that implements operations
// on an existing site
type site struct {
	*log.Entry
	// service points to the parent service with settings and backend
	service *Operator

	appService appservice.Applications

	domainName string
	key        ops.SiteKey
	provider   string
	license    string

	// app defines the installation configuration
	app *appservice.Application

	// backendSite is the "storage" representation of the site
	backendSite *storage.Site
	seedConfig  ops.SeedConfig

	// static package assets
	teleportPackage  loc.Locator
	gravityPackage   loc.Locator
	webAssetsPackage loc.Locator
}

func newSite(site *site) (result *site, err error) {
	locator, err := site.app.Manifest.Dependencies.ByName(constants.TeleportPackage)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	site.teleportPackage = *locator

	locator, err = site.app.Manifest.Dependencies.ByName(constants.WebAssetsPackage)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	site.webAssetsPackage = *locator

	locator, err = site.app.Manifest.Dependencies.ByName(constants.GravityPackage)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	site.gravityPackage = *locator

	site.Entry = log.WithFields(log.Fields{
		trace.Component:           constants.ComponentOps,
		constants.FieldSiteDomain: site.domainName,
	})

	return site, nil
}

// cloudProviderName returns cloud provider name as understood
// by kubernetes
func (s *site) cloudProviderName() string {
	switch s.provider {
	case schema.ProviderAWS, schema.ProvisionerAWSTerraform:
		return schema.ProviderAWS
	case schema.ProviderGCE:
		return schema.ProviderGCE
	default:
		return ""
	}
}

func (s *site) gceNodeTags() string {
	return strings.Join(s.backendSite.CloudConfig.GCENodeTags, ",")
}

func (s *site) String() string {
	return fmt.Sprintf("site(domain=%v)", s.domainName)
}

func (s *site) operationLogPath(key ops.SiteOperationKey) string {
	return s.siteDir(key.OperationID, fmt.Sprintf("%v.log", key.OperationID))
}

func (s *site) openFiles(filePaths ...string) ([]io.WriteCloser, error) {
	var files []io.WriteCloser
	for _, filePath := range filePaths {
		file, err := os.OpenFile(
			filePath, os.O_CREATE|os.O_RDWR|os.O_APPEND, defaults.PrivateFileMask)
		if err != nil {
			utils.NewMultiWriteCloser(files...).Close()
			return nil, trace.Wrap(err)
		}
		files = append(files, file)
	}
	return files, nil
}

func (s *site) newOperationRecorder(key ops.SiteOperationKey, additionalLogFiles ...string) (io.WriteCloser, error) {
	writers, err := s.openFiles(additionalLogFiles...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := os.MkdirAll(filepath.Dir(s.operationLogPath(key)), defaults.SharedDirMask); err != nil {
		// to close all the file handles we have just opened
		utils.NewMultiWriteCloser(writers...).Close()
		return nil, trace.Wrap(err)
	}
	f, err := os.OpenFile(
		s.operationLogPath(key), os.O_CREATE|os.O_RDWR|os.O_APPEND, defaults.SharedReadMask)
	if err != nil {
		// to close all the file handles we have just opened
		utils.NewMultiWriteCloser(writers...).Close()
		return nil, trace.Wrap(err)
	}
	writers = append(writers, f)
	return utils.NewMultiWriteCloser(writers...), nil
}

func (s *site) installToken() string {
	return s.backendSite.InstallToken
}

func (s *site) cloudProvider() CloudProvider {
	return s.service.getCloudProvider(s.key)
}

func (s *site) agentUserEmail() string {
	return fmt.Sprintf("agent@%v", s.domainName)
}

func (s *site) agentUser() (storage.User, error) {
	return s.users().GetTelekubeUser(s.agentUserEmail())
}

func (s *site) appPackage() (*loc.Locator, error) {
	site, err := s.backend().GetSite(s.key.SiteDomain)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	loc, err := loc.NewLocator(site.App.Repository, site.App.Name, site.App.Version)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return loc, nil
}

func (s *site) teleport() ops.TeleportProxyService {
	return s.service.cfg.TeleportProxy
}

// deleteSite deletes the cluster entry database entry and cleans up
// the state that is only connected with this cluster
func (s *site) deleteSite() error {
	s.service.deleteCloudProvider(s.key)

	err := s.service.deleteClusterAgents(s.domainName)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := s.packages().DeleteRepository(s.domainName); err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}

	backend := s.service.backend()

	operations, err := backend.GetSiteOperations(s.key.SiteDomain)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	for _, operation := range operations {
		if err = backend.DeleteSiteOperation(s.key.SiteDomain, operation.ID); err != nil {
			return trace.Wrap(err)
		}
	}
	if err = backend.DeleteSite(s.key.SiteDomain); err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}

	// delete teleport certificate authorities for the deleted site
	err = s.teleport().DeleteAuthority(s.domainName)
	if err != nil && !trace.IsNotFound(err) {
		s.service.Warnf("Failed to delete authorities for %q: %v.",
			s.domainName, trace.DebugReport(err))
	}

	// remove the teleport's remote site object (which represents a remote
	// cluster on the main cluster side in a trusted cluster relationship)
	err = s.teleport().DeleteRemoteCluster(s.domainName)
	if err != nil && !trace.IsNotFound(err) {
		s.service.Warnf("Failed to remove remote cluster for %q: %v.",
			s.domainName, trace.DebugReport(err))
	}

	// Delete the application package if it's not used anywhere else
	if err := s.cleanupApplication(); err != nil {
		log.Warnf("Failed to remove application: %v.", err)
	}

	if err := os.RemoveAll(s.siteDir()); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// deleteClusterAgents deletes all agent users for the specified cluster
func (o *Operator) deleteClusterAgents(clusterName string) error {
	users, err := o.backend().GetSiteUsers(clusterName)
	if err != nil {
		return trace.Wrap(err)
	}
	errors := make([]error, len(users))
	for i, user := range users {
		if user.GetType() != storage.AgentUser {
			continue
		}
		err := o.DeleteLocalUser(user.GetName())
		if err != nil {
			errors[i] = trace.Wrap(err)
		}
	}
	return trace.NewAggregate(errors...)
}

func (s *site) cleanupApplication() error {
	if purpose, exists := s.app.PackageEnvelope.RuntimeLabels[pack.PurposeLabel]; !exists || purpose != pack.PurposeMetadata {
		// With no metadata label, do not attempt to delete the application package
		return nil
	}

	var errors []error
	req := appservice.DeleteRequest{Package: s.app.Package}
	if err := s.appService.DeleteApp(req); err != nil {
		errors = append(errors, err)
	}
	if s.backendSite.App.Base != nil {
		req = appservice.DeleteRequest{Package: s.backendSite.App.Base.Locator()}
		err := s.appService.DeleteApp(req)
		if err != nil {
			errors = append(errors, err)
		}
	}

	return trace.NewAggregate(errors...)
}

func (s *site) siteDir(additional ...string) string {
	return s.service.siteDir(s.key.AccountID, s.key.SiteDomain, additional...)
}

func (s *site) users() users.Identity {
	return s.service.cfg.Users
}

func (s *site) backend() storage.Backend {
	return s.service.backend()
}

func (s *site) leader() storage.Leader {
	return s.service.leader()
}

func (s *site) agentService() ops.AgentService {
	return s.service.cfg.Agents
}

func (s *site) agentRunner(ctx *operationContext) *agentRunner {
	return &agentRunner{
		ctx:          ctx,
		AgentService: s.agentService(),
	}
}

func (s *site) packages() pack.PackageService {
	return s.service.cfg.Packages
}

func (s *site) apps() appservice.Applications {
	return s.service.cfg.Apps
}

func (s *site) clock() timetools.TimeProvider {
	return s.service.cfg.Clock
}

func (s *site) getOperationGroup() *operationGroup {
	return s.service.getOperationGroup(s.key)
}

// site specific package repository that is only accessible to this site
func (s *site) siteRepoName() string {
	return s.domainName
}

func (s *site) systemVars(op ops.SiteOperation, variables storage.SystemVariables) (*storage.SystemVariables, error) {
	token, err := s.users().GetOperationProvisioningToken(op.SiteDomain, op.ID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	url := strings.Join([]string{s.packages().PortalURL(), "t"}, "/")
	result := variables
	result.ClusterName = op.SiteDomain
	result.OpsURL = url
	result.Token = token.Token
	result.Devmode = s.service.cfg.Devmode || s.service.cfg.Local
	return &result, nil
}

func (s *site) setSiteState(state string) error {
	site, err := s.backend().GetSite(s.key.SiteDomain)
	if err != nil {
		return trace.Wrap(err)
	}
	site.State = state
	_, err = s.backend().UpdateSite(*site)
	return trace.Wrap(err)
}

func (s *site) executeOperation(key ops.SiteOperationKey, fn func(ctx *operationContext) error) error {
	op, err := s.getSiteOperation(key.OperationID)
	if err != nil {
		return trace.Wrap(err)
	}
	ctx, err := s.newOperationContext(*op)
	if err != nil {
		return trace.Wrap(err)
	}
	go func() {
		if err := s.executeOperationWithContext(ctx, op, fn); err != nil {
			s.WithFields(log.Fields{
				log.ErrorKey: err,
				"operation":  op,
			}).Warn("Failed to execute operation.")
		}
	}()
	return nil
}

func (s *site) executeOperationWithContext(ctx *operationContext, op *ops.SiteOperation, fn func(ctx *operationContext) error) error {
	defer ctx.Close()

	opErr := fn(ctx)

	if opErr == nil {
		return nil
	}

	ctx.WithError(opErr).Error("Operation failure.")

	// change the state without "compare" part just to take leverage of
	// the operation group locking to ensure atomicity
	_, err := s.compareAndSwapOperationState(context.TODO(), swap{
		key:        ctx.key(),
		newOpState: ops.OperationStateFailed,
	})
	if err != nil {
		ctx.WithError(err).Error("Failed to compare and swap operation state.")
	}

	s.reportProgress(ctx, ops.ProgressEntry{
		State:      ops.ProgressStateFailed,
		Completion: constants.Completed,
		Message:    opErr.Error(),
	})
	return trace.Wrap(opErr)
}

//nolint:unused
type transformFn func(reader io.Reader) (io.ReadCloser, error)

//nolint:unused
func (s *site) copyFile(src, dst string, transform transformFn) error {
	s.Infof("copyFile(src=%v, dst=%v)", src, dst)
	file, err := os.Open(src)
	if err != nil {

		return trace.Wrap(err)
	}
	defer file.Close()
	err = s.copyFileFromStream(file, dst, transform)
	if err != nil {
		return trace.Wrap(err)
	}
	info, err := os.Stat(src)
	if err != nil {
		defer os.Remove(dst)
		return trace.Wrap(err)
	}
	err = os.Chmod(dst, info.Mode())
	if err != nil {
		defer os.Remove(dst)
		return trace.Wrap(err)
	}
	return nil
}

//nolint:unused
func (s *site) copyFileFromStream(stream io.ReadCloser, dst string, transform transformFn) (err error) {
	if transform != nil {
		stream, err = transform(stream)
		if err != nil {
			return trace.Wrap(err)
		}
		defer stream.Close()
	}
	w, err := os.Create(dst)
	if err != nil {
		return trace.Wrap(err)
	}
	defer w.Close()
	if _, err = io.Copy(w, stream); err != nil {
		defer os.Remove(dst)
		return trace.Wrap(err)
	}
	return nil
}

func (s *site) compareAndSwapOperationState(ctx context.Context, swap swap) (*ops.SiteOperation, error) {
	return s.getOperationGroup().compareAndSwapOperationState(ctx, swap)
}

func (s *site) setOperationState(key ops.SiteOperationKey, state string) (*ops.SiteOperation, error) {
	operation, err := s.getSiteOperation(key.OperationID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	operation.State = state
	operation, err = s.updateSiteOperation(operation)
	return operation, trace.Wrap(err)
}

func (s *site) createSiteOperation(o *ops.SiteOperation) (*ops.SiteOperation, error) {
	out, err := s.backend().CreateSiteOperation((storage.SiteOperation)(*o))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Create initial progress entry
	progressEntry := storage.ProgressEntry{
		SiteDomain:  out.SiteDomain,
		OperationID: out.ID,
		Created:     s.clock().UtcNow(),
	}

	_, err = s.backend().CreateProgressEntry(progressEntry)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return (*ops.SiteOperation)(out), nil
}

func (s *site) updateSiteOperation(o *ops.SiteOperation) (*ops.SiteOperation, error) {
	out, err := s.backend().UpdateSiteOperation((storage.SiteOperation)(*o))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return (*ops.SiteOperation)(out), nil
}

func (s site) dockerConfig() storage.DockerConfig {
	return s.backendSite.ClusterState.Docker
}

func (s site) servers() []storage.Server {
	return s.backendSite.ClusterState.Servers
}

func (s site) dnsConfig() storage.DNSConfig {
	if s.backendSite.DNSConfig.IsEmpty() {
		return storage.DefaultDNSConfig
	}
	return s.backendSite.DNSConfig
}

func (s site) serviceUser() storage.OSUser {
	if !s.backendSite.ServiceUser.IsEmpty() {
		return s.backendSite.ServiceUser
	}
	return storage.DefaultOSUser()
}

func (s site) uid() string {
	if !s.backendSite.ServiceUser.IsEmpty() {
		return s.backendSite.ServiceUser.UID
	}
	return defaults.ServiceUserID
}

func (s site) gid() string {
	if !s.backendSite.ServiceUser.IsEmpty() {
		return s.backendSite.ServiceUser.GID
	}
	return defaults.ServiceUserID
}

func (s *site) getClusterConfiguration() (*clusterconfig.Resource, error) {
	client, err := s.service.GetKubeClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	configmap, err := client.CoreV1().ConfigMaps(defaults.KubeSystemNamespace).
		Get(context.TODO(), constants.ClusterConfigurationMap, metav1.GetOptions{})
	err = rigging.ConvertError(err)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	var spec string
	if configmap != nil {
		spec = configmap.Data["spec"]
	}
	var config *clusterconfig.Resource
	if spec != "" {
		config, err = clusterconfig.Unmarshal([]byte(spec))
		if err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		config = clusterconfig.NewEmpty()
	}
	if err := s.setClusterConfigDefaults(config); err != nil {
		return nil, trace.Wrap(err)
	}
	return config, nil
}

func (s *site) setClusterConfigDefaults(config *clusterconfig.Resource) error {
	if config.Spec.Global.CloudProvider == "" {
		config.Spec.Global.CloudProvider = s.provider
	}
	installOp, _, err := ops.GetInstallOperation(s.key, s.service)
	if err != nil {
		return trace.Wrap(err)
	}
	if installOp == nil {
		return trace.NotFound("no install operation found for cluster %q", s.key.SiteDomain)
	}
	if config.Spec.Global.PodCIDR == "" {
		config.Spec.Global.PodCIDR = installOp.InstallExpand.Vars.OnPrem.PodCIDR
	}
	if config.Spec.Global.ServiceCIDR == "" {
		config.Spec.Global.ServiceCIDR = installOp.InstallExpand.Vars.OnPrem.ServiceCIDR
	}
	return nil
}

func (s *site) getClusterEnvironmentVariables() (env storage.EnvironmentVariables, err error) {
	b := backoff.NewExponentialBackOff()
	b.MaxElapsedTime = defaults.APIWaitTimeout
	err = utils.RetryTransient(context.TODO(), b, func() error {
		env, err = s.service.GetClusterEnvironmentVariables(s.key)
		return trace.Wrap(err)
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return env, nil
}

func convertSite(in storage.Site, apps appservice.Applications) (*ops.Site, error) {
	loc, err := loc.NewLocator(in.App.Repository, in.App.Name, in.App.Version)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	app, err := apps.GetApp(*loc)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	if trace.IsNotFound(err) {
		log.Warnf("Failed to open application %v: %v.", loc, trace.DebugReport(err))
		app = appservice.Phony
	}

	serviceUser := in.ServiceUser
	if serviceUser.IsEmpty() {
		serviceUser = storage.DefaultOSUser()
	}

	site := &ops.Site{
		Domain:    in.Domain,
		State:     in.State,
		Reason:    in.Reason,
		AccountID: in.AccountID,
		Created:   in.Created,
		CreatedBy: in.CreatedBy,
		Local:     in.Local,
		App: ops.Application{
			Manifest:        app.Manifest,
			Package:         app.Package,
			PackageEnvelope: app.PackageEnvelope,
		},
		Resources:                in.Resources,
		Provider:                 in.Provider,
		Labels:                   in.Labels,
		FinalInstallStepComplete: in.FinalInstallStepComplete,
		Location:                 in.Location,
		Flavor:                   in.Flavor,
		UpdateInterval:           in.UpdateInterval,
		NextUpdateCheck:          in.NextUpdateCheck,
		ClusterState:             in.ClusterState,
		ServiceUser:              serviceUser,
		CloudConfig:              in.CloudConfig,
		DNSOverrides:             in.DNSOverrides,
		DNSConfig:                in.DNSConfig,
		InstallToken:             in.InstallToken,
	}
	if in.License != "" {
		parsed, err := license.ParseLicense(in.License)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		payload := parsed.GetPayload()
		payload.EncryptionKey = nil // do not display encryption key if it's present
		site.License = &ops.License{
			Raw:     in.License,
			Payload: payload,
		}
	}
	return site, nil
}
