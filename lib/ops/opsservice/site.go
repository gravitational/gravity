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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	appservice "github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/gravity/lib/users"
	"github.com/gravitational/license"
	"github.com/gravitational/trace"
	"github.com/mailgun/timetools"
	log "github.com/sirupsen/logrus"
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
	// resources is additional runtime k8s resources injected during
	// installation process
	resources []byte

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

func (s *site) hasResources() bool {
	return len(s.resources) != 0
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

func (s *site) loadProvisionerState(state interface{}) error {
	s.Infof("loadProvisionerState")
	st, err := s.backend().GetSite(s.key.SiteDomain)

	if err != nil {
		return trace.Wrap(err)
	}
	if st.ProvisionerState == nil {
		return trace.NotFound("no provisioner state found")
	}
	return trace.Wrap(json.Unmarshal(st.ProvisionerState, state))
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

func (s *site) agentUserAndKey() (storage.User, *storage.APIKey, error) {
	u, err := s.agentUser()
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	keys, err := s.users().GetAPIKeys(u.GetName())
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	if len(keys) == 0 {
		return nil, nil, trace.NotFound("no api keys found for user(%v)", u.GetName())
	}
	return u, &keys[0], nil
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

	// remove the reverse tunnel site
	err = s.service.cfg.Tunnel.RemoveSite(s.domainName)
	if err != nil {
		s.service.Warnf("Failed to remove reverse tunnel for %q: %v.",
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
	return &agentRunner{ctx, s.agentService()}
}

func (s *site) packages() pack.PackageService {
	return s.service.cfg.Packages
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
	return &storage.SystemVariables{
		ClusterName: op.SiteDomain,
		OpsURL:      url,
		Token:       token.Token,
		Devmode:     s.service.cfg.Devmode || s.service.cfg.Local,
		Docker:      variables.Docker,
	}, nil
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

func (s *site) updateSiteApp(appPackage string) error {
	loc, err := loc.ParseLocator(appPackage)
	if err != nil {
		return trace.Wrap(err)
	}

	site, err := s.backend().GetSite(s.key.SiteDomain)
	if err != nil {
		return trace.Wrap(err)
	}

	envelope, err := s.service.cfg.Packages.ReadPackageEnvelope(*loc)
	if err != nil {
		return trace.Wrap(err)
	}

	site.App = envelope.ToPackage()
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
	go s.executeOperationWithContext(ctx, op, fn)
	return nil
}

func (s *site) executeOperationWithContext(ctx *operationContext, op *ops.SiteOperation, fn func(ctx *operationContext) error) {
	defer ctx.Close()

	opErr := fn(ctx)

	if opErr == nil {
		return
	}

	ctx.Errorf("operation failure: %v", trace.DebugReport(opErr))

	// change the state without "compare" part just to take leverage of
	// the operation group locking to ensure atomicity
	_, err := s.compareAndSwapOperationState(swap{
		key:        ctx.key(),
		newOpState: ops.OperationStateFailed,
	})
	if err != nil {
		ctx.Errorf("failed to compare and swap operation state: %v", trace.DebugReport(err))
	}

	s.reportProgress(ctx, ops.ProgressEntry{
		State:      ops.ProgressStateFailed,
		Completion: constants.Completed,
		Message:    opErr.Error(),
	})
}

type transformFn func(reader io.Reader) (io.ReadCloser, error)

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

func (s *site) copyFileFromString(data, dst string, transform transformFn) error {
	s.Debugf("rendering \n%s\n to %v", data, dst)
	reader := strings.NewReader(data)
	return s.copyFileFromStream(ioutil.NopCloser(reader), dst, transform)
}

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

func (s *site) copyDir(src, dst string, t transformFn) error {
	s.Infof("copyDir(src=%v, dst=%v)", src, dst)
	info, err := os.Stat(src)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := os.MkdirAll(dst, info.Mode()); err != nil {
		return trace.Wrap(err)
	}
	dir, err := os.Open(src)
	if err != nil {
		return trace.Wrap(err)
	}
	defer dir.Close()

	fileinfos, err := dir.Readdir(-1)
	if err != nil {
		return trace.Wrap(err)
	}

	for _, f := range fileinfos {
		fsrc := filepath.Join(src, f.Name())
		fdst := filepath.Join(dst, f.Name())
		if f.IsDir() {
			if err := s.copyDir(fsrc, fdst, t); err != nil {
				return trace.Wrap(err)
			}
		} else {
			if err := s.copyFile(fsrc, fdst, t); err != nil {
				return trace.Wrap(err)
			}
		}
	}
	return nil
}

func (s *site) render(data []byte, server map[string]interface{}, ctx *operationContext) (io.Reader, error) {
	t := template.New("tpl")
	t, err := t.Parse(string(data))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	site, err := s.backend().GetSite(s.domainName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	variables, err := ctx.operation.GetVars().ToMap()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	context := map[string]interface{}{
		"variables":   variables,
		"server":      server,
		"site_labels": site.Labels,
		"networking":  s.getNetworkType(ctx),
	}
	buf := &bytes.Buffer{}
	if err := t.Execute(buf, context); err != nil {
		return nil, trace.Wrap(err)
	}
	return buf, nil
}

func (s *site) getNetworkType(ctx *operationContext) string {
	return s.app.Manifest.GetNetworkType(s.provider, ctx.operation.Provisioner)
}

func (s *site) renderString(data []byte, server map[string]interface{}, ctx *operationContext) (string, error) {
	r, err := s.render(data, server, ctx)
	if err != nil {
		return "", trace.Wrap(err)
	}
	out, err := ioutil.ReadAll(r)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return string(out), nil
}

func (s *site) compareAndSwapOperationState(swap swap) (*ops.SiteOperation, error) {
	return s.getOperationGroup().compareAndSwapOperationState(swap)
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
		Resources: in.Resources,
		Provider:  in.Provider,
		Labels:    in.Labels,
		FinalInstallStepComplete: in.FinalInstallStepComplete,
		Location:                 in.Location,
		UpdateInterval:           in.UpdateInterval,
		NextUpdateCheck:          in.NextUpdateCheck,
		ClusterState:             in.ClusterState,
		ServiceUser:              serviceUser,
		CloudConfig:              in.CloudConfig,
		DNSOverrides:             in.DNSOverrides,
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
