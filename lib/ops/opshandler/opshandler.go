/*
Copyright 2018-2019 Gravitational, Inc.

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

package opshandler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/httplib"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/ops/events"
	"github.com/gravitational/gravity/lib/ops/opsclient"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/users"
	"github.com/gravitational/gravity/lib/utils/fields"

	"github.com/gravitational/roundtrip"
	"github.com/gravitational/teleport/lib/auth"
	telehttplib "github.com/gravitational/teleport/lib/httplib"
	teleservices "github.com/gravitational/teleport/lib/services"
	teleutils "github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
)

// WebHandlerConfig is the ops web handler configuration
type WebHandlerConfig struct {
	// Backend is the process backend
	Backend storage.Backend
	// Users is the process users service
	Users users.Identity
	// Operator is the underlying ops service
	Operator ops.Operator
	// Applications is the apps service
	Applications app.Applications
	// Packages is the pack service
	Packages pack.PackageService
	// Authenticator is used to authenticate web requests
	Authenticator users.Authenticator
	// Devmode is whether the process is started in dev mode
	Devmode bool
	// PublicAdvertiseAddr is the process public advertise address
	PublicAdvertiseAddr teleutils.NetAddr
}

// CheckAndSetDefaults validates the config and sets some defaults.
func (c *WebHandlerConfig) CheckAndSetDefaults() error {
	if c.Operator == nil {
		return trace.BadParameter("missing parameter Operator")
	}
	if c.Users == nil {
		return trace.BadParameter("missing parameter Users")
	}
	if c.Applications == nil {
		return trace.BadParameter("missing parameter Applications")
	}
	if c.Packages == nil {
		return trace.BadParameter("missing parameter Packages")
	}
	if c.Authenticator == nil {
		c.Authenticator = users.NewAuthenticatorFromIdentity(c.Users)
	}
	return nil
}

type WebHandler struct {
	httprouter.Router
	cfg        WebHandlerConfig
	middleware *auth.AuthMiddleware
}

// GetConfig returns config web handler was initialized with
func (h *WebHandler) GetConfig() WebHandlerConfig {
	return h.cfg
}

func NewWebHandler(cfg WebHandlerConfig) (*WebHandler, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	h := &WebHandler{
		cfg: cfg,
	}

	// Wrap the router in the authentication middleware which will detect
	// if the client is trying to authenticate using a client certificate,
	// extract user information from it and add it to the request context.
	h.middleware = &auth.AuthMiddleware{
		AccessPoint: users.NewAccessPoint(cfg.Users),
	}
	h.middleware.Wrap(&h.Router)

	h.OPTIONS("/*path", h.options)

	// Report portal status
	h.GET("/portal/v1/status", h.getStatus)

	// Return server version
	h.GET("/portal/v1/version", h.needsAuth(h.getVersion))

	// Applications API
	h.GET("/portal/v1/apps", h.needsAuth(h.getApps))
	h.GET("/portal/v1/gravity", h.needsAuth(h.getGravityBinary))

	// Accounts API
	h.POST("/portal/v1/accounts", h.needsAuth(h.createAccount))
	h.GET("/portal/v1/accounts/:account_id", h.needsAuth(h.getAccount))
	h.GET("/portal/v1/accounts", h.needsAuth(h.getAccounts))

	// Users API
	h.GET("/portal/v1/currentuser", h.needsAuth(h.getCurrentUser))
	h.GET("/portal/v1/currentuserinfo", h.needsAuth(h.getCurrentUserInfo))
	h.POST("/portal/v1/users", h.needsAuth(h.createUser))
	h.DELETE("/portal/v1/users/:user_email", h.needsAuth(h.deleteLocalUser))
	h.PUT("/portal/v1/accounts/:account_id/sites/:site_domain/users/:user_email", h.needsAuth(h.updateUser))

	// API keys API
	h.POST("/portal/v1/apikeys/user/:user_email", h.needsAuth(h.createAPIKey))
	h.GET("/portal/v1/apikeys/user/:user_email", h.needsAuth(h.getAPIKeys))
	h.DELETE("/portal/v1/apikeys/user/:user_email/:api_key", h.needsAuth(h.deleteAPIKey))

	// Invites API
	h.POST("/portal/v1/accounts/:account_id/sites/:site_domain/tokens/userinvites", h.needsAuth(h.createUserInvite))
	h.GET("/portal/v1/accounts/:account_id/sites/:site_domain/tokens/userinvites", h.needsAuth(h.getUserInvites))
	h.DELETE("/portal/v1/accounts/:account_id/sites/:site_domain/tokens/userinvites/:name", h.needsAuth(h.deleteUserInvite))

	// Tokens API
	h.POST("/portal/v1/tokens/install", h.needsAuth(h.createInstallToken))
	h.POST("/portal/v1/accounts/:account_id/sites/:site_domain/tokens/userresets", h.needsAuth(h.resetUser))
	h.POST("/portal/v1/accounts/:account_id/sites/:site_domain/tokens/provision", h.needsAuth(h.createProvisioningToken))
	h.GET("/portal/v1/accounts/:account_id/sites/:site_domain/tokens/expand", h.needsAuth(h.getExpandToken))
	h.GET("/portal/v1/accounts/:account_id/sites/:site_domain/tokens/trustedcluster", h.needsAuth(h.getTrustedClusterToken))

	// Sites API
	h.GET("/portal/v1/localsite", h.needsAuth(h.getLocalSite))
	h.POST("/portal/v1/accounts/:account_id/sites", h.needsAuth(h.createSite))
	h.DELETE("/portal/v1/accounts/:account_id/sites/:site_domain", h.needsAuth(h.deleteSite))
	h.GET("/portal/v1/accounts/:account_id/sites/:site_domain", h.needsAuth(h.getSite))
	h.GET("/portal/v1/accounts/:account_id/sites", h.needsAuth(h.getSites))
	h.GET("/portal/v1/accounts/:account_id/sites/:site_domain/report", h.needsAuth(h.getSiteReport))
	h.POST("/portal/v1/accounts/:account_id/sites/:site_domain/deactivate", h.needsAuth(h.deactivateSite))
	h.POST("/portal/v1/accounts/:account_id/sites/:site_domain/activate", h.needsAuth(h.activateSite))
	h.POST("/portal/v1/accounts/:account_id/sites/:site_domain/complete", h.needsAuth(h.completeFinalInstallStep))
	h.GET("/portal/v1/accounts/:account_id/sites/:site_domain/localuser", h.needsAuth(h.getLocalUser))
	h.PUT("/portal/v1/accounts/:account_id/sites/:site_domain/reset-password", h.needsAuth(h.resetUserPassword))
	h.GET("/portal/v1/accounts/:account_id/sites/:site_domain/agent", h.needsAuth(h.getClusterAgent))
	h.GET("/portal/v1/accounts/:account_id/sites/:site_domain/nodes", h.needsAuth(h.getClusterNodes))

	// Status API
	h.GET("/portal/v1/accounts/:account_id/sites/:site_domain/status", h.needsAuth(h.checkSiteStatus))

	// TODO(klizhetas) refactor this method
	h.GET("/portal/v1/sites/domain/:domain", h.needsAuth(h.getSiteByDomain))
	h.GET("/portal/v1/domains/:domain", h.needsAuth(h.validateDomainName))

	// Leadership API
	h.POST("/portal/v1/accounts/:account_id/sites/:site_domain/stepdown", h.needsAuth(h.stepDown))

	// Sign API
	h.POST("/portal/v1/accounts/:account_id/sites/:site_domain/sign/tls", h.needsAuth(h.signTLSKey))
	h.POST("/portal/v1/accounts/:account_id/sign/ssh", h.needsAuth(h.signSSHKey))

	// Cluster certificate API
	h.GET("/portal/v1/accounts/:account_id/sites/:site_domain/certificate", h.needsAuth(h.getClusterCert))
	h.POST("/portal/v1/accounts/:account_id/sites/:site_domain/certificate", h.needsAuth(h.updateClusterCert))
	h.DELETE("/portal/v1/accounts/:account_id/sites/:site_domain/certificate", h.needsAuth(h.deleteClusterCert))

	// Prechecks API
	h.POST("/portal/v1/accounts/:account_id/sites/:site_domain/prechecks", h.needsAuth(h.validateServers))

	// Site Operations API

	// generate agent instructions - compact form
	h.GET("/t/:token/:server_profile", h.getSiteInstructions)
	// generate agent instructions
	h.GET("/portal/v1/tokens/:token/:server_profile", h.getSiteInstructions)

	h.POST("/portal/v1/accounts/:account_id/sites/:site_domain/operations/install", h.needsAuth(h.createSiteInstallOperation))
	h.PUT("/portal/v1/accounts/:account_id/sites/:site_domain/operations/install/:operation_id", h.needsAuth(h.updateInstallOperation))
	h.GET("/portal/v1/accounts/:account_id/sites/:site_domain/operations/install/:operation_id/agent-report", h.needsAuth(h.getSiteInstallOperationAgentReport))
	h.POST("/portal/v1/accounts/:account_id/sites/:site_domain/operations/install/:operation_id/start", h.needsAuth(h.siteInstallOperationStart))

	// expand - add nodes to the cluster
	h.POST("/portal/v1/accounts/:account_id/sites/:site_domain/operations/expand", h.needsAuth(h.createSiteExpandOperation))
	h.PUT("/portal/v1/accounts/:account_id/sites/:site_domain/operations/expand/:operation_id", h.needsAuth(h.updateExpandOperation))
	h.GET("/portal/v1/accounts/:account_id/sites/:site_domain/operations/expand/:operation_id/agent-report", h.needsAuth(h.getSiteExpandOperationAgentReport))
	h.POST("/portal/v1/accounts/:account_id/sites/:site_domain/operations/expand/:operation_id/start", h.needsAuth(h.siteExpandOperationStart))

	// uninstall - nuke everything
	h.POST("/portal/v1/accounts/:account_id/sites/:site_domain/operations/uninstall", h.needsAuth(h.createSiteUninstallOperation))

	// shrink - remove servers
	h.POST("/portal/v1/accounts/:account_id/sites/:site_domain/operations/shrink", h.needsAuth(h.createSiteShrinkOperation))
	h.POST("/portal/v1/accounts/:account_id/sites/:site_domain/operations/shrink/resume", h.needsAuth(h.resumeShrink))

	// garbage collection
	h.POST("/portal/v1/accounts/:account_id/sites/:site_domain/operations/gc", h.needsAuth(h.createClusterGarbageCollectOperation))

	// cluster reconfiguration
	h.POST("/portal/v1/accounts/:account_id/sites/:site_domain/operations/reconfigure", h.needsAuth(h.createClusterReconfigureOperation))

	// update - update installed application to a new version
	h.POST("/portal/v1/accounts/:account_id/sites/:site_domain/operations/update", h.needsAuth(h.createSiteUpdateOperation))

	// common operations methods
	h.GET("/portal/v1/accounts/:account_id/sites/:site_domain/operations/common", h.needsAuth(h.getSiteOperations))
	// update install/expand operation state
	h.GET("/portal/v1/accounts/:account_id/sites/:site_domain/operations/common/:operation_id", h.needsAuth(h.getSiteOperation))
	h.DELETE("/portal/v1/accounts/:account_id/sites/:site_domain/operations/common/:operation_id", h.needsAuth(h.deleteOperation))
	h.GET("/portal/v1/accounts/:account_id/sites/:site_domain/operations/common/:operation_id/logs", h.needsAuth(h.getSiteOperationLogs))
	h.POST("/portal/v1/accounts/:account_id/sites/:site_domain/operations/common/:operation_id/logs/entry", h.needsAuth(h.createLogEntry))
	h.POST("/portal/v1/accounts/:account_id/sites/:site_domain/operations/common/:operation_id/logs", h.needsAuth(h.streamOperationLogs))
	h.GET("/portal/v1/accounts/:account_id/sites/:site_domain/operations/common/:operation_id/progress", h.needsAuth(h.getSiteOperationProgress))
	h.POST("/portal/v1/accounts/:account_id/sites/:site_domain/operations/common/:operation_id/progress", h.needsAuth(h.createProgressEntry))
	h.GET("/portal/v1/accounts/:account_id/sites/:site_domain/operations/common/:operation_id/crash-report", h.needsAuth(h.getSiteOperationCrashReport))
	h.PUT("/portal/v1/accounts/:account_id/sites/:site_domain/operations/common/:operation_id/complete", h.needsAuth(h.completeSiteOperation))
	h.POST("/portal/v1/accounts/:account_id/sites/:site_domain/operations/common/:operation_id/plan", h.needsAuth(h.createOperationPlan))
	h.POST("/portal/v1/accounts/:account_id/sites/:site_domain/operations/common/:operation_id/plan/changelog", h.needsAuth(h.createOperationPlanChange))
	h.GET("/portal/v1/accounts/:account_id/sites/:site_domain/operations/common/:operation_id/plan", h.needsAuth(h.getOperationPlan))
	h.POST("/portal/v1/accounts/:account_id/sites/:site_domain/operations/common/:operation_id/plan/configure", h.needsAuth(h.configurePackages))

	// log forwarders
	h.GET("/portal/v1/accounts/:account_id/sites/:site_domain/logs/forwarders", h.needsAuth(h.getLogForwarders))

	h.POST("/portal/v1/accounts/:account_id/sites/:site_domain/logs/forwarders", h.needsAuth(h.createLogForwarder))
	h.PUT("/portal/v1/accounts/:account_id/sites/:site_domain/logs/forwarders/:name", h.needsAuth(h.updateLogForwarder))
	h.DELETE("/portal/v1/accounts/:account_id/sites/:site_domain/logs/forwarders/:name", h.needsAuth(h.deleteLogForwarder))

	// smtp
	h.GET("/portal/v1/accounts/:account_id/sites/:site_domain/smtp", h.needsAuth(h.getSMTPConfig))
	h.PUT("/portal/v1/accounts/:account_id/sites/:site_domain/smtp", h.needsAuth(h.updateSMTPConfig))
	h.DELETE("/portal/v1/accounts/:account_id/sites/:site_domain/smtp", h.needsAuth(h.deleteSMTPConfig))

	// monitoring
	h.GET("/portal/v1/accounts/:account_id/sites/:site_domain/monitoring/alerts", h.needsAuth(h.getAlerts))
	h.PUT("/portal/v1/accounts/:account_id/sites/:site_domain/monitoring/alerts/:name", h.needsAuth(h.updateAlert))
	h.DELETE("/portal/v1/accounts/:account_id/sites/:site_domain/monitoring/alerts/:name", h.needsAuth(h.deleteAlert))
	h.GET("/portal/v1/accounts/:account_id/sites/:site_domain/monitoring/alert-targets", h.needsAuth(h.getAlertTargets))
	h.PUT("/portal/v1/accounts/:account_id/sites/:site_domain/monitoring/alert-targets", h.needsAuth(h.updateAlertTarget))
	h.DELETE("/portal/v1/accounts/:account_id/sites/:site_domain/monitoring/alert-targets", h.needsAuth(h.deleteAlertTarget))
	h.GET("/portal/v1/accounts/:account_id/sites/:site_domain/monitoring/metrics", h.needsAuth(h.getClusterMetrics))

	// environment variables
	h.GET("/portal/v1/accounts/:account_id/sites/:site_domain/envars", h.needsAuth(h.getEnvironmentVariables))
	h.PUT("/portal/v1/accounts/:account_id/sites/:site_domain/envars", h.needsAuth(h.updateEnvironmentVariables))
	h.POST("/portal/v1/accounts/:account_id/sites/:site_domain/operations/envars", h.needsAuth(h.createUpdateEnvarsOperation))

	// cluster configuration
	h.GET("/portal/v1/accounts/:account_id/sites/:site_domain/config", h.needsAuth(h.getClusterConfiguration))
	h.PUT("/portal/v1/accounts/:account_id/sites/:site_domain/config", h.needsAuth(h.updateClusterConfig))
	h.POST("/portal/v1/accounts/:account_id/sites/:site_domain/operations/config", h.needsAuth(h.createUpdateConfigOperation))

	// persistent storage configuration
	h.GET("/portal/v1/accounts/:account_id/sites/:site_domain/persistentstorage", h.needsAuth(h.getPersistentStorage))
	h.PUT("/portal/v1/accounts/:account_id/sites/:site_domain/persistentstorage", h.needsAuth(h.updatePersistentStorage))

	// validation
	h.POST("/portal/v1/accounts/:account_id/sites/:site_domain/validation/remoteaccess", h.needsAuth(h.validateRemoteAccess))

	// cluster and application endpoints
	h.GET("/portal/v1/accounts/:account_id/sites/:site_domain/endpoints", h.needsAuth(h.getApplicationEndpoints))

	// app installer
	h.GET("/portal/v1/accounts/:account_id/apps/:repository_id/:package_name/:version/installer", h.needsAuth(h.getAppInstaller))

	// web helpers - special functions for the UI
	h.GET("/portal/v1/webhelpers/accounts/:account_id/sites/:site_domain/operations/last/:operation_type", h.needsAuth(h.getLastOperation))

	// Github connector handlers
	h.POST("/portal/v1/accounts/:account_id/sites/:site_domain/github/connectors",
		h.needsAuth(h.upsertGithubConnector))
	h.GET("/portal/v1/accounts/:account_id/sites/:site_domain/github/connectors/:id",
		h.needsAuth(h.getGithubConnector))
	h.GET("/portal/v1/accounts/:account_id/sites/:site_domain/github/connectors",
		h.needsAuth(h.getGithubConnectors))
	h.DELETE("/portal/v1/accounts/:account_id/sites/:site_domain/github/connectors/:id",
		h.needsAuth(h.deleteGithubConnector))

	// user handlers
	h.POST("/portal/v1/accounts/:account_id/sites/:site_domain/users",
		h.needsAuth(h.upsertUser))
	h.GET("/portal/v1/accounts/:account_id/sites/:site_domain/users/:name",
		h.needsAuth(h.getUser))
	h.GET("/portal/v1/accounts/:account_id/sites/:site_domain/users",
		h.needsAuth(h.getUsers))
	h.DELETE("/portal/v1/accounts/:account_id/sites/:site_domain/users/:name",
		h.needsAuth(h.deleteUser))

	// cluster configuration
	h.POST("/portal/v1/accounts/:account_id/sites/:site_domain/authentication/preference",
		h.needsAuth(h.upsertClusterAuthPreference))
	h.GET("/portal/v1/accounts/:account_id/sites/:site_domain/authentication/preference",
		h.needsAuth(h.getClusterAuthPreference))

	// auth gateway settings
	h.POST("/portal/v1/accounts/:account_id/sites/:site_domain/authgateway",
		h.needsAuth(h.upsertAuthGateway))
	h.GET("/portal/v1/accounts/:account_id/sites/:site_domain/authgateway",
		h.needsAuth(h.getAuthGateway))

	// application releases
	h.GET("/portal/v1/accounts/:account_id/sites/:site_domain/releases",
		h.needsAuth(h.getReleases))

	// audit log events
	h.POST("/portal/v1/accounts/:account_id/sites/:site_domain/events",
		h.needsAuth(h.emitAuditEvent))

	return h, nil
}

// ServeHTTP lets the authentication middleware serve the request before
// passing it through to the router.
func (s *WebHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.middleware.ServeHTTP(w, r)
}

func (h *WebHandler) options(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	roundtrip.ReplyJSON(w, http.StatusOK, map[string]interface{}{"ok": "ok"})
}

/*
   getSiteInstructions is used to retrieve site install instructions

   GET /portal/v1/t/:token/:serverProfile?params
*/
func (h *WebHandler) getSiteInstructions(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	token := p[0].Value
	serverProfile := p[1].Value
	instructions, err := h.cfg.Operator.GetSiteInstructions(token, serverProfile, r.URL.Query())
	if err != nil {
		trace.WriteError(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(instructions)); err != nil {
		log.WithError(err).Warn("Failed to write response.")
	}
}

/*
   getStatus is used by health checkers to validate the status of the portal

   GET /portal/v1/status

   checkers expect the response to be exaclty: {"status": "healthy"}
   otherwise they will alert with the response body
*/
func (h *WebHandler) getStatus(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	roundtrip.ReplyJSON(w, http.StatusOK, map[string]interface{}{"status": "healthy"})
}

/*
   getVersion returns the server version information.

   GET /portal/v1/version
*/
func (h *WebHandler) getVersion(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	version, err := context.Operator.GetVersion(r.Context())
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, version)
	return nil
}

/* getApps returns information about apps available for installation

  GET /portal/v1/apps

Success response:

  [{
    "repository": "gravitational.io",
    "name": "mattermost",
    "version": "1.2.1"
  }]

*/
func (h *WebHandler) getApps(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	repositories, err := h.cfg.Packages.GetRepositories()
	if err != nil {
		return trace.Wrap(err)
	}
	var apps []app.Application
	for _, repository := range repositories {
		batch, err := h.cfg.Applications.ListApps(app.ListAppsRequest{
			Repository: repository,
			Type:       storage.AppUser,
		})
		if err != nil {
			return trace.Wrap(err)
		}
		apps = append(apps, batch...)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, apps)
	return nil
}

/* getGravityBinary exports the cluster's gravity binary.

   GET /portal/v1/gravity
*/
func (h *WebHandler) getGravityBinary(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	cluster, err := context.Operator.GetLocalSite()
	if err != nil {
		return trace.Wrap(err)
	}
	gravityPackage, err := cluster.App.Manifest.Dependencies.ByName(constants.GravityPackage)
	if err != nil {
		return trace.Wrap(err)
	}
	_, reader, err := h.cfg.Packages.ReadPackage(*gravityPackage)
	if err != nil {
		return trace.Wrap(err)
	}
	defer reader.Close()
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment; filename=gravity")
	_, err = io.Copy(w, reader)
	return trace.Wrap(err)
}

/*  inviteUser resets user credentials and returns a user token

    POST /portal/v1/accounts/:account_id/sites/:site_domain/usertokens/resets
*/
func (h *WebHandler) resetUser(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return trace.Wrap(err)
	}
	var req ops.CreateUserResetRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return trace.BadParameter(err.Error())
	}
	resetToken, err := context.Operator.CreateUserReset(r.Context(), req)
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, resetToken)
	return nil
}

/*  createUserInvite creates a new invite token for a user.

    POST /portal/v1/accounts/:account_id/sites/:site_domain/usertokens/invites
*/
func (h *WebHandler) createUserInvite(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return trace.Wrap(err)
	}
	var req ops.CreateUserInviteRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return trace.BadParameter(err.Error())
	}
	inviteToken, err := context.Operator.CreateUserInvite(r.Context(), req)
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, inviteToken)
	return nil
}

/*  getUserInvites returns all active user invites.

    GET /portal/v1/accounts/:account_id/sites/:site_domain/usertokens/invites
*/
func (h *WebHandler) getUserInvites(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	invites, err := context.Operator.GetUserInvites(r.Context(), siteKey(p))
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, invites)
	return nil
}

/*  deleteUserInvite deletes the specified user invite.

    DELETE /portal/v1/accounts/:account_id/sites/:site_domain/usertokens/invites/:name
*/
func (h *WebHandler) deleteUserInvite(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	err := context.Operator.DeleteUserInvite(r.Context(), ops.DeleteUserInviteRequest{
		SiteKey: siteKey(p),
		Name:    p.ByName("name"),
	})
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, statusOK("invite deleted"))
	return nil
}

/* createAccount creates new account entry

   POST /portal/v1/accounts

   {
      "org": "unique org name"
   }

Success response:
  {
     "id": "account-id",
     "org": "unique org name"
  }

*/
func (h *WebHandler) createAccount(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return trace.Wrap(err)
	}
	var req ops.NewAccountRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return trace.BadParameter(err.Error())
	}
	account, err := context.Operator.CreateAccount(req)
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, account)
	return nil
}

/* getAccount retrieves account by ID

   GET /portal/v1/accounts/:account_id

Success response:
  {
     "id": "account-id",
     "org": "unique org name"
  }

*/
func (h *WebHandler) getAccount(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	account, err := context.Operator.GetAccount(p[0].Value)
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, account)
	return nil
}

/* getAccounts returns a list of accounts in the system

   GET /portal/v1/accounts/:account_id

Success response:
  [{
     "id": "account-id",
     "org": "unique org name"
  }]

*/
func (h *WebHandler) getAccounts(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	accounts, err := context.Operator.GetAccounts()
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, accounts)
	return nil
}

/* getCurrentUser returns information about currently logged in user

   GET /portal/v1/currentuser

   Success response:

   {
     "message": "user created"
   }

*/
func (h *WebHandler) getCurrentUser(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	logins, err := context.Checker.CheckLoginDuration(time.Minute)
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, context.User.WebSessionInfo(logins))
	return nil
}

/* getCurrentUserInfo returns information about currently logged in user

   GET /portal/v1/currentuserinfo

   Success response:

   {
     "kubernetes_groups": ["admin", "root"]
   }

*/
func (h *WebHandler) getCurrentUserInfo(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	info, err := context.Operator.GetCurrentUserInfo()
	if err != nil {
		return trace.Wrap(err)
	}
	raw, err := info.ToRaw()
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, raw)
	return nil
}

/* createUser creates a new user

   POST /portal/v1/user

   Success response:

   {
     "message": "user created"
   }

*/
func (h *WebHandler) createUser(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	d := json.NewDecoder(r.Body)
	var req ops.NewUserRequest
	if err := d.Decode(&req); err != nil {
		return trace.BadParameter(err.Error())
	}
	if err := context.Operator.CreateUser(req); err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, statusOK("user created"))
	return nil
}

/* updateUser updates the specified user information.

   PUT /portal/v1/accounts/:account_id/sites/:site_domain/users/:user_email
*/
func (h *WebHandler) updateUser(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	var req ops.UpdateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return trace.BadParameter(err.Error())
	}
	if err := context.Operator.UpdateUser(r.Context(), req); err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, statusOK("user updated"))
	return nil
}

/* deleteUser deletes a user by name

   DELETE /portal/v1/users/:user_name

   Success response:

   {
     "message": "user jenkins deleted"
   }
*/
func (h *WebHandler) deleteLocalUser(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	name := p[0].Value
	err := h.cfg.Users.DeleteUser(name)
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, statusOK(fmt.Sprintf("user %v deleted", name)))
	return nil
}

/* createAPIKey creates a new api key

   POST /portal/v1/users/:user_email/apikeys

   Success response:

   {
     "message": "api key created"
   }

*/
func (h *WebHandler) createAPIKey(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	d := json.NewDecoder(r.Body)
	var req ops.NewAPIKeyRequest
	if err := d.Decode(&req); err != nil {
		return trace.BadParameter(err.Error())
	}

	key, err := context.Operator.CreateAPIKey(r.Context(), req)
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, key)
	return nil
}

/* getAPIKeys returns user API keys

   GET /portal/v1/apikeys/user/:user_email

   Success response:

   [
     {
       "token": "qwe",
       "expires": ...
     },
     ...
   ]
*/
func (h *WebHandler) getAPIKeys(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	userEmail := p.ByName("user_email")
	keys, err := context.Operator.GetAPIKeys(userEmail)
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, keys)
	return nil
}

/* deleteAPIKey deletes an api key

   DELETE /portal/v1/apikeys/user/:user_email/:api_key

   Success response:

   {
     "message": "api key deleted"
   }
*/
func (h *WebHandler) deleteAPIKey(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	err := context.Operator.DeleteAPIKey(r.Context(), p[0].Value, p[1].Value)
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, statusOK("api key deleted"))
	return nil
}

/*  createInstallToken generates a one-time token for installation

    POST /portal/v1/accounts/:account_id/tokens/install

    Success response:
    {
      "token": "value",
      "expires": "RFC3339 timestamp",
      "account_id": "account id",
    }
*/
func (h *WebHandler) createInstallToken(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return trace.Wrap(err)
	}
	var req ops.NewInstallTokenRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return trace.BadParameter("cannot unmarshal request object: %v", err)
	}
	token, err := context.Operator.CreateInstallToken(req)
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, token)
	return nil
}

/*  createProvisioningToken creates a new provisioning token

    POST /portal/v1/accounts/:account_id/sites/:site_domain/tokens/provision
*/
func (h *WebHandler) createProvisioningToken(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	d := json.NewDecoder(r.Body)
	var token storage.ProvisioningToken
	if err := d.Decode(&token); err != nil {
		return trace.BadParameter(err.Error())
	}
	if err := context.Operator.CreateProvisioningToken(token); err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, statusOK("token created"))
	return nil
}

/*  getExpandToken returns the site's expand token

    GET /portal/v1/accounts/:account_id/tokens/expand

    Success response:

    storage.ProvisioningToken
*/
func (h *WebHandler) getExpandToken(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	token, err := context.Operator.GetExpandToken(siteKey(p))
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, token)
	return nil
}

/*  getTrustedClusterToken returns the cluster's trusted cluster token

    GET /portal/v1/accounts/:account_id/tokens/trustedcluster

    Success response:

    storage.Token
*/
func (h *WebHandler) getTrustedClusterToken(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	token, err := context.Operator.GetTrustedClusterToken(siteKey(p))
	if err != nil {
		return trace.Wrap(err)
	}
	bytes, err := storage.GetTokenMarshaler().MarshalToken(token)
	return trace.Wrap(rawMessage(w, bytes, err))
}

/* createSite creates a site entry in the portal for a given account

   POST /portal/v1/accounts/<account-id/sites

   {
     "app_package": "gravitational.io/mattermost:1.2.1",
     "provisioner": "onprem",
     "account_id": "account1",
     "email_address": "a@example.com"
     "domain_name": "example.com"
   }

 Success response:

  {
	"id": "site1",
	"account_id": "account1",
	"domain_name": "example.com",
	"state": "state1",
	"provisioner": "onprem",
	"app": {
		"package": {
		   "repository": "gravitational.io",
		   "name": "mattermost",
		   "version": "1.2.1"
		},
		"manifest": {
		   "provisioners": [],
		   "servers": [],
		   "post_install_commands": [],
		},
		"info": {
			endpoints: []
		}
	}
  }
*/
func (h *WebHandler) createSite(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return trace.Wrap(err)
	}
	log.Infof("createSite(%v)", string(data))
	var req ops.NewSiteRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return trace.BadParameter(err.Error())
	}
	req.AccountID = p[0].Value

	appPackage, err := loc.ParseLocator(req.AppPackage)
	if err != nil {
		return trace.Wrap(err)
	}

	userApp, err := h.cfg.Applications.GetApp(*appPackage)
	if err != nil {
		return trace.Wrap(err)
	}

	if err = app.VerifyDependencies(userApp, h.cfg.Applications, h.cfg.Packages); err != nil {
		return trace.Wrap(err)
	}

	site, err := context.Operator.CreateSite(req)
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, site)
	return nil
}

/* deleteSite deletes a site entry, note that it does not uninstall the site

   DELETE /portal/v1/accounts/<account-id>/sites/<site-domain>
*/
func (h *WebHandler) deleteSite(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	err := context.Operator.DeleteSite(siteKey(p))
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, statusOK("site deleted"))
	return nil
}

/*
   getLocalSite returns a site entry by account id and site id

   GET /portal/v1/localsite

Success response:

   {
      ... site contents, see createSite for details
   }
*/
func (h *WebHandler) getLocalSite(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	site, err := context.Operator.GetLocalSite()
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, site)
	return nil
}

/*
   getSite returns a site entry by account id and site id

   GET /portal/v1/accounts/<account-id>/sites/<site-id>

Success response:

   {
      ... site contents, see createSite for details
   }
*/
func (h *WebHandler) getSite(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	site, err := context.Operator.GetSite(siteKey(p))
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, site)
	return nil
}

/*
   getSites returns a list of sites for account

   GET /portal/v1/accounts/<account-id>/sites

Success response:

   [{
      ... site contents, see createSite for details
   }]
*/
func (h *WebHandler) getSites(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	sites, err := context.Operator.GetSites(p[0].Value)
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, sites)
	return nil
}

/*
   getSiteByDomain returns a site entry by domain

   GET /portal/v1/sites/domain/:domain

Success response:

   {
      ... site contents, see createSite for details
   }
*/
func (h *WebHandler) getSiteByDomain(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	site, err := context.Operator.GetSiteByDomain(p[0].Value)
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, site)
	return nil
}

/*  deactivateSite moves site to the "degraded" state and possibly stops the application

    POST /portal/v1/accounts/:account_id/sites/:site_domain/deactivate

    Input: ops.DeactivateSiteRequest

    Success response:
    {
      "message": "site deactivated"
    }
*/
func (h *WebHandler) deactivateSite(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	d := json.NewDecoder(r.Body)
	var req ops.DeactivateSiteRequest
	if err := d.Decode(&req); err != nil {
		return trace.BadParameter(err.Error())
	}
	if err := context.Operator.DeactivateSite(req); err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, statusOK("site deactivated"))
	return nil
}

/*  activateSite moves site to the "active" state and possibly starts the application

    POST /portal/v1/accounts/:account_id/sites/:site_domain/activate

    Input: ops.ActivateSiteRequest

    Success response:
    {
      "message": "site activated"
    }
*/
func (h *WebHandler) activateSite(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	d := json.NewDecoder(r.Body)
	var req ops.ActivateSiteRequest
	if err := d.Decode(&req); err != nil {
		return trace.BadParameter(err.Error())
	}
	if err := context.Operator.ActivateSite(req); err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, statusOK("site activated"))
	return nil
}

/* getSiteReport returns a tarball with collected information about the site

   GET /portal/v1/accounts/:account_id/sites/:site_domain/report
*/
func (h *WebHandler) getSiteReport(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	var since time.Duration
	if val := r.URL.Query().Get("since"); val != "" {
		var err error
		if since, err = time.ParseDuration(val); err != nil {
			return trace.Wrap(err)
		}
	}

	report, err := context.Operator.GetSiteReport(ops.GetClusterReportRequest{
		SiteKey: siteKey(p),
		Since:   since,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	defer report.Close()
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%v", defaults.ReportTarball))
	readSeeker, ok := report.(io.ReadSeeker)
	if ok {
		http.ServeContent(w, r, "report", time.Now(), readSeeker)
		return nil
	}
	_, err = io.Copy(w, report)
	return trace.Wrap(err)
}

/*  completeFinalInstallStep marks the site as having completed the last installation step

    POST /portal/v1/accounts/:account_id/sites/:site_domain/complete

    Input: ops.SiteKey

    Success response:
    {
      "message": "site installer step marked completed"
    }
*/
func (h *WebHandler) completeFinalInstallStep(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	d := json.NewDecoder(r.Body)
	var req ops.CompleteFinalInstallStepRequest
	if err := d.Decode(&req); err != nil {
		return trace.BadParameter(err.Error())
	}
	if err := context.Operator.CompleteFinalInstallStep(req); err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, statusOK("site final install step has been completed"))
	return nil
}

/*  getLocalUser returns the local gravity site user

    GET /portal/v1/accounts/:account_id/sites/:site_domain/localuser

    Input: ops.SiteKey

    Success response: users.User
*/
func (h *WebHandler) getLocalUser(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	user, err := context.Operator.GetLocalUser(siteKey(p))
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, user)
	return nil
}

/*  getClusterAgent returns the agent user for the specified cluster

    GET /portal/v1/accounts/:account_id/sites/:site_domain/agent

    Input: ops.ClusterAgentRequest

    Success response: storage.LoginEntry
*/
func (h *WebHandler) getClusterAgent(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	err := r.ParseForm()
	if err != nil {
		return trace.Wrap(err)
	}
	needAdmin := false
	if r.Form.Get("admin") != "" {
		needAdmin, err = strconv.ParseBool(r.Form.Get("admin"))
		if err != nil {
			return trace.Wrap(err)
		}
	}
	key := siteKey(p)
	entry, err := context.Operator.GetClusterAgent(ops.ClusterAgentRequest{
		AccountID:   key.AccountID,
		ClusterName: key.SiteDomain,
		Admin:       needAdmin,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, entry)
	return nil
}

/*  getClusterNodes returns a real-time information about cluster nodes

    GET /portal/v1/accounts/:account_id/sites/:site_domain/nodes

    Input: ops.SiteKey

    Success response: []ops.Node
*/
func (h *WebHandler) getClusterNodes(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	nodes, err := context.Operator.GetClusterNodes(siteKey(p))
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, nodes)
	return nil
}

/*  resetUserPassword resets the user password and returns the new one

    PUT /portal/v1/accounts/:account_id/sites/:site_domain/reset-password

    Input: ops.ResetUserPasswordRequest

    Success response:
    {
      "password": <new password string>
    }
*/
func (h *WebHandler) resetUserPassword(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	d := json.NewDecoder(r.Body)
	var req ops.ResetUserPasswordRequest
	if err := d.Decode(&req); err != nil {
		return trace.BadParameter(err.Error())
	}
	password, err := context.Operator.ResetUserPassword(req)
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, map[string]string{"password": password})
	return nil
}

/*  signTLSKey signs TLS Public Key

    POST /portal/v1/accounts/:account_id/sites/:site_domain/sign/tls
*/
func (h *WebHandler) signTLSKey(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	d := json.NewDecoder(r.Body)
	var req ops.TLSSignRequest
	if err := d.Decode(&req); err != nil {
		return trace.BadParameter(err.Error())
	}
	re, err := context.Operator.SignTLSKey(req)
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, re)
	return nil
}

/*  signSSHKey signs SSH Public Key

    POST /portal/v1/accounts/:account_id/sites/:site_domain/sign/ssh
*/
func (h *WebHandler) signSSHKey(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	d := json.NewDecoder(r.Body)
	var req ops.SSHSignRequest
	if err := d.Decode(&req); err != nil {
		return trace.BadParameter(err.Error())
	}
	re, err := context.Operator.SignSSHKey(req)
	if err != nil {
		return trace.Wrap(err)
	}
	raw, err := re.ToRaw()
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, raw)
	return nil
}

/*  validateServers runs a pre-installation checks for a site

    POST /portal/v1/accounts/:account_id/sites/:site_domain/prechecks
*/
func (h *WebHandler) validateServers(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	d := json.NewDecoder(r.Body)
	var req ops.ValidateServersRequest
	if err := d.Decode(&req); err != nil {
		return trace.BadParameter(err.Error())
	}
	resp, err := context.Operator.ValidateServers(context.Context, req)
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, resp)
	return nil
}

/*  stepDown asks the process to pause leadership

    POST /portal/v1/accounts/:account_id/sites/:site_domain/stepdown
*/
func (h *WebHandler) stepDown(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	err := context.Operator.StepDown(siteKey(p))
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, statusOK("ok"))
	return nil
}

/*  checkSiteStatus checks site status by invoking app status hook

    GET /portal/v1/accounts/:account_id/sites/:site_domain/status

    Success response:
    {
      "message": "ok"
    }
*/
func (h *WebHandler) checkSiteStatus(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	if err := context.Operator.CheckSiteStatus(r.Context(), siteKey(p)); err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, statusOK("ok"))
	return nil
}

/*  validateDomainName checks if the specified domain name has already been allocated

    GET /portal/v1/domains/:domain

    Success response:
    {
      "message": "ok"
    }
*/
func (h *WebHandler) validateDomainName(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	domainName := p[0].Value
	if err := context.Operator.ValidateDomainName(domainName); err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, statusOK("ok"))
	return nil
}

/*  validateRemoteAccess verifies remote access to nodes in the cluster by executing a set of specified
    commands (and returning their results)

    POST /portal/v1/accounts/:account_id/sites/:domain/validation/remoteaccess

    Input: ops.ValidateRemoteAccessRequest
    Output: ops.ValidateRemoteAccessResponse
*/
func (h *WebHandler) validateRemoteAccess(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	var req ops.ValidateRemoteAccessRequest
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&req); err != nil {
		return trace.BadParameter("failed to decode request: %v", err)
	}

	resp, err := context.Operator.ValidateRemoteAccess(req)
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, resp)
	return nil
}

/* getSiteOperations returns a list of operations that were executed for this site

   GET /portal/v1/accounts/:account_id/sites/:site_domain/operations/common

   [{
      "id": "operation id",
      "account_id": "account id",
      "site_id": "site_id",
      "type": "operation type, e.g. 'install_site' or "uninstall_site"",
      "created": "timestamp RFC 3339",
	  "updated": "timestamp RFC 3339",
      "state": "operation state",
	  "servers": "servers involved in this operatoin",
	  "variables": {
         "key": "operation specific variables"
      }
   }]
*/
func (h *WebHandler) getSiteOperations(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	operations, err := context.Operator.GetSiteOperations(siteKey(p))
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, operations)
	return nil
}

/* getSiteOperation returns site operation by it's ID

  GET /portal/v1/accounts/:account_id/sites/:site_domain/operations/common/:operation_id

Success response:

  {
     ...operation, see getSiteOperations for details
  }
*/
func (h *WebHandler) getSiteOperation(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	operation, err := context.Operator.GetSiteOperation(siteOperationKey(p))
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, operation)
	return nil
}

/* deleteOperation removes an unstarted operation

  DELETE /portal/v1/accounts/:account_id/sites/:site_domain/operations/common/:operation_id

Success response:

  {
     "message": "deleted"
  }
*/
func (h *WebHandler) deleteOperation(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	err := context.Operator.DeleteSiteOperation(siteOperationKey(p))
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, statusOK("deleted"))
	return nil
}

/* resumeShrink resumes the interrupted shrink operation

   POST	/portal/v1/accounts/:account_id/sites/:site_domain/operations/shrink/resume


Success response:

{
    "AccountID": "account id",
    "SiteDomain": "site domain",
    "OperationID": "operation id"
}
*/
func (h *WebHandler) resumeShrink(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	key := siteKey(p)
	opKey, err := context.Operator.ResumeShrink(key)
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, opKey)
	return nil
}

/* createSiteInstallOperation creates site install operation. Note that
it does not starts actuall uninstall, but rather creates a record to configure
and track uninstall

   POST	/portal/v1/accounts/:account_id/sites/:site_domain/operations/install

   {
      "account_id": "account id",
      "site_id": "site_id",
      "variables": {
         "key": "operation specific variables"
      }
   }


Success response:

   {
      "account_id": "account id",
      "site_id": "site_id",
      "operation_id": "operation id"
   }
*/
func (h *WebHandler) createSiteInstallOperation(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	d := json.NewDecoder(r.Body)
	var req ops.CreateSiteInstallOperationRequest
	if err := d.Decode(&req); err != nil {
		return trace.BadParameter(err.Error())
	}
	key := siteKey(p)
	req.AccountID = key.AccountID
	req.SiteDomain = key.SiteDomain
	if req.Provisioner == "" {
		site, err := context.Operator.GetSite(key)
		if err != nil {
			return trace.Wrap(err, "failed to fetch a site for key %v", key)
		}
		provisioner, err := schema.GetProvisionerFromProvider(site.Provider)
		if err != nil {
			return trace.Wrap(err)
		}
		req.Provisioner = provisioner
	}
	op, err := context.Operator.CreateSiteInstallOperation(r.Context(), req)
	if err != nil {
		return trace.Wrap(err)
	}
	log.Debugf("got operation: %#v", op)
	roundtrip.ReplyJSON(w, http.StatusOK, op)
	return nil
}

/* updateInstallOperation updates the state of an install operation

   PUT /portal/v1/accounts/:account_id/sites/:site_domain/operations/install/:operation_id
*/
func (h *WebHandler) updateInstallOperation(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	d := json.NewDecoder(r.Body)
	var req ops.OperationUpdateRequest
	if err := d.Decode(&req); err != nil {
		return trace.BadParameter(err.Error())
	}
	opKey := siteOperationKey(p)
	log.Infof("opshandler: updateInstallOperation: req=%#v, key=%v", req, opKey)
	if err := context.Operator.UpdateInstallOperationState(opKey, req); err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, statusOK("install operation updated"))
	return nil
}

/* updateExpandOperation updates the state of an install operation

   PUT /portal/v1/accounts/:account_id/sites/:site_domain/operations/expand/:operation_id
*/
func (h *WebHandler) updateExpandOperation(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	d := json.NewDecoder(r.Body)
	var req ops.OperationUpdateRequest
	if err := d.Decode(&req); err != nil {
		return trace.BadParameter(err.Error())
	}
	opKey := siteOperationKey(p)
	log.Infof("opshandler: updateExpandOperation: req=%#v, key=%v", req, opKey)
	if err := context.Operator.UpdateExpandOperationState(opKey, req); err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, statusOK("expand operation updated"))
	return nil
}

/* completeSiteOperation completes the specified operation with given state

   PUT /portal/v1/accos/:account_id/sites/:site_domain/operations/common/:operation_id/complete

   {
      "account_id": "account id",
      "site_id": "site_id",
      "variables": {
         "key": "operation specific variables"
      }
   }


Success response:

   {
      "status": "operation completed",
   }
*/
func (h *WebHandler) completeSiteOperation(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	d := json.NewDecoder(r.Body)
	var req ops.SetOperationStateRequest
	if err := d.Decode(&req); err != nil {
		return trace.BadParameter(err.Error())
	}
	opKey := siteOperationKey(p)
	err := context.Operator.SetOperationState(opKey, req)
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, statusOK("operation completed"))
	return nil
}

/* createOperationPlan saves the provided operation plan

   POST /portal/v1/accos/:account_id/sites/:site_domain/operations/common/:operation_id/plan

   Success response: {"status": "ok", "message": "plan created"}
*/
func (h *WebHandler) createOperationPlan(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	var plan storage.OperationPlan
	err := json.NewDecoder(r.Body).Decode(&plan)
	if err != nil {
		return trace.Wrap(err)
	}
	err = context.Operator.CreateOperationPlan(siteOperationKey(p), plan)
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, statusOK("plan created"))
	return nil
}

/* createOperationPlanChange creates a new changelog entry for a plan

   POST /portal/v1/accos/:account_id/sites/:site_domain/operations/common/:operation_id/plan/changelog

   Success response: {"status": "ok", "message": "changelog entry created"}
*/
func (h *WebHandler) createOperationPlanChange(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	var change storage.PlanChange
	err := json.NewDecoder(r.Body).Decode(&change)
	if err != nil {
		return trace.Wrap(err)
	}
	err = context.Operator.CreateOperationPlanChange(siteOperationKey(p), change)
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, statusOK("changelog entry created"))
	return nil
}

/* getOperationPlan returns plan for the specified operation

   GET /portal/v1/accos/:account_id/sites/:site_domain/operations/common/:operation_id/plan

   Success response: storage.OperationPlan
*/
func (h *WebHandler) getOperationPlan(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	plan, err := context.Operator.GetOperationPlan(siteOperationKey(p))
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, plan)
	return nil
}

/* configurePackages configures install packages

   POST /portal/v1/accounts/:account_id/sites/:site_domain/operations/common/:operation_id/plan/configure

   Success response: {"status": "ok", "message": "packages configured"}
*/
func (h *WebHandler) configurePackages(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	d := json.NewDecoder(r.Body)
	var req ops.ConfigurePackagesRequest
	if err := d.Decode(&req); err != nil {
		return trace.BadParameter(err.Error())
	}
	req.SiteOperationKey = siteOperationKey(p)
	err := context.Operator.ConfigurePackages(req)
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, statusOK("packages configured"))
	return nil
}

/* getSiteInstallOperationAgentReport returns a set of server parameters such as network interfaces
  collected by agents running on each node as well as download instructions.
  These details are used on the client to let user customize the installation before it commences.

  GET /portal/v1/accounts/:account_id/sites/:site_domain/operations/install/:operation_id/agent-report

Success response:

   {
      "can_continue": true,
      "message": "message to show to user",
      "servers": [{
          "hostname": "localhost",
          "interfaces": [{
             "ipv4_addr": "127.0.0.1",
             "name": "lo"
          }]
      }],
      "instructions": "download instructions to display to user"
   }
*/
func (h *WebHandler) getSiteInstallOperationAgentReport(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	agentReport, err := context.Operator.GetSiteInstallOperationAgentReport(siteOperationKey(p))
	if err != nil {
		return trace.Wrap(err)
	}

	resp, err := agentReport.Transport()
	if err != nil {
		return trace.Wrap(err)
	}

	roundtrip.ReplyJSON(w, http.StatusOK, resp)
	return nil
}

/* siteInstallOperationStart activates actual install operation, note that operation plan has to be set before calling this function
cal

   POST /portal/v1/accounts/:account_id/sites/:site_domain/operations/install/:operation_id/start

Success response:

   {
     "status": "ok"
   }

*/
func (h *WebHandler) siteInstallOperationStart(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	key := siteOperationKey(p)
	go func() {
		err := context.Operator.SiteInstallOperationStart(key)
		if err != nil {
			log.Warningf("site %v install operation failed, error %v", key, err)
		}
	}()
	roundtrip.ReplyJSON(w, http.StatusOK, statusOK("install operation has started"))
	return nil
}

/* createSiteExpandOperation initiates expansion - adding new servers to the cluster
it does not kick off actuall change, but creates a record for tracking

   POST	/portal/v1/accounts/:account_id/sites/:site_domain/operations/expand

   {
      "account_id": "account id",
      "site_id": "site_id",
	  "variables": {
         "key": "operation specific variables"
      },
      "servers": {"master": 1} // servers of the particular profile to add
   }


Success response:

   {
      "account_id": "account id",
      "site_id": "site_id",
      "operation_id": "operation id"
   }
*/
func (h *WebHandler) createSiteExpandOperation(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	d := json.NewDecoder(r.Body)
	var req ops.CreateSiteExpandOperationRequest
	if err := d.Decode(&req); err != nil {
		return trace.BadParameter(err.Error())
	}

	key := siteKey(p)
	req.AccountID = key.AccountID
	req.SiteDomain = key.SiteDomain
	if req.Provisioner == "" {
		installOp, err := ops.GetCompletedInstallOperation(key, context.Operator)
		if err != nil {
			return trace.Wrap(err, "failed to find a completed install operation")
		}
		req.Provisioner = installOp.Provisioner
	}

	op, err := context.Operator.CreateSiteExpandOperation(r.Context(), req)
	if err != nil {
		return trace.Wrap(err)
	}

	log.Infof("got operation: %#v", op)
	roundtrip.ReplyJSON(w, http.StatusOK, op)
	return nil
}

/*getSiteExpandOperationAgent report is used for on prem installations and displays the
information collected by the runtime agents started by user on hosts about server and
their parameters so user can configure it

  GET /portal/v1/accounts/:account_id/sites/:site_domain/operations/expand/:operation_id/agent-report

Success response:

   {
      "can_continue": true,
	  "message": "message to show to user",
	  "servers": [{
          "hostname": "localhost",
          "interfaces": [{
             "ipv4_addr": "127.0.0.1",
             "name": "lo"
           }]
      }],
      "instructions": "download instructions to display to user"
   }
*/
func (h *WebHandler) getSiteExpandOperationAgentReport(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	agentReport, err := context.Operator.GetSiteExpandOperationAgentReport(siteOperationKey(p))
	if err != nil {
		return trace.Wrap(err)
	}

	resp, err := agentReport.Transport()
	if err != nil {
		return trace.Wrap(err)
	}

	roundtrip.ReplyJSON(w, http.StatusOK, resp)
	return nil
}

/* siteExpandOperationStart activates actual expand operation, note that operation plan has to be set before calling this function

   POST /portal/v1/accounts/:account_id/sites/:site_domain/operations/expand/:operation_id/start

Success response:

   {
     "status": "ok"
   }

*/
func (h *WebHandler) siteExpandOperationStart(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	key := siteOperationKey(p)
	go func() {
		err := context.Operator.SiteExpandOperationStart(key)
		if err != nil {
			log.Warningf("site %v expand operation failed, error %v", key, err)
		}
	}()
	roundtrip.ReplyJSON(w, http.StatusOK, statusOK("expand operation has started"))
	return nil
}

/* createSiteUninstallOperation initiates site uninstall operation. Note that
it starts actuall uninstall, and creates a record to configure
and track uninstall

   POST	/portal/v1/accounts/:account_id/sites/:site_domain/operations/uninstall

   {
      "account_id": "account id",
      "site_id": "site_id"
   }


Success response:

   {
      "account_id": "account id",
      "site_id": "site_id",
      "operation_id": "operation id"
   }
*/
func (h *WebHandler) createSiteUninstallOperation(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	d := json.NewDecoder(r.Body)
	var req ops.CreateSiteUninstallOperationRequest
	if err := d.Decode(&req); err != nil {
		return trace.BadParameter(err.Error())
	}
	key := siteKey(p)
	req.AccountID = key.AccountID
	req.SiteDomain = key.SiteDomain
	op, err := context.Operator.CreateSiteUninstallOperation(r.Context(), req)
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, op)
	return nil
}

/*getSiteOperationLogs is a web socket method that returns a stream of logs for this operation

  GET /portal/v1/accounts/:account_id/sites/:site_domain/operations/common/:operation_id/logs

*/
func (h *WebHandler) getSiteOperationLogs(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	return getOpLogs(w, r, siteOperationKey(p), context)
}

/* createLogEntry appends the provided log entry to the operation's log file

   POST /portal/v1/accounts/:account_id/sites/:site_domain/operations/common/:operation_id/logs/entry
*/
func (h *WebHandler) createLogEntry(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	d := json.NewDecoder(r.Body)
	var req ops.LogEntry
	if err := d.Decode(&req); err != nil {
		return trace.BadParameter(err.Error())
	}
	err := context.Operator.CreateLogEntry(siteOperationKey(p), req)
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, statusOK("log entry created"))
	return nil
}

/* streamOperationLogs appends the logs from the provided reader to the
   specified operation (user-facing) log file

   POST /portal/v1/accounts/:account_id/sites/:site_domain/operations/common/:operation_id/logs
*/
func (h *WebHandler) streamOperationLogs(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	err := context.Operator.StreamOperationLogs(siteOperationKey(p), r.Body)
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, statusOK("logs uploaded"))
	return nil
}

/*getSiteOperationCrashReport returns a file upload with a tarball of a crash dump for this operation

  GET /portal/v1/accounts/:account_id/sites/:site_domain/operations/common/:operation_id/crash-report

*/
func (h *WebHandler) getSiteOperationCrashReport(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	report, err := context.Operator.GetSiteReport(ops.GetClusterReportRequest{SiteKey: siteKey(p)})
	if err != nil {
		return trace.Wrap(err)
	}
	w.Header().Set("Content-Disposition", "attachment; filename=crashreport.tar")
	_, err = io.Copy(w, report)
	return err
}

/*getSiteOperationProgress returns a progress report for this operation

  GET /portal/v1/accounts/:account_id/sites/:site_domain/operations/common/:operation_id/progress

Success Response:

  {
	"site_id": "site id",
	"operation_id": "operation id",
	"created": "timestamp RFC 3339",
    "completion": 39,
	"state": "one of 'in_progress', 'failed', or 'completed'",
	"message": "message to display to user"
  }
*/
func (h *WebHandler) getSiteOperationProgress(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	progressEntry, err := context.Operator.GetSiteOperationProgress(siteOperationKey(p))
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, progressEntry)
	return nil
}

/* createProgressEntry creates a new operation progress entry

   POST /portal/v1/accounts/:account_id/sites/:site_domain/operations/common/:operation_id/progress
*/
func (h *WebHandler) createProgressEntry(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	d := json.NewDecoder(r.Body)
	var req ops.ProgressEntry
	if err := d.Decode(&req); err != nil {
		return trace.BadParameter(err.Error())
	}
	err := context.Operator.CreateProgressEntry(siteOperationKey(p), req)
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, statusOK("progress entry created"))
	return nil
}

/*
   getLastOperation returns the last operation of the given type

   GET /portal/v1/webhelpers/accounts/:account_id/sites/:site_domain/operations/last/:operation_type"

Success response:

   {
      ... operation contents, see createSiteOperation for details
   }
*/
func (h *WebHandler) getLastOperation(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	op, err := getLastOperation(siteKey(p), p[2].Value, context)
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, op)
	return nil
}

/* createSiteShrinkOperation removes servers and the data from the cluster

   POST	/portal/v1/accounts/:account_id/sites/:site_domain/operations/shrink

   {
      "account_id": "account id",
      "site_id": "site_id",
      "variables": {
         "key": "operation specific variables"
      },
      servers: ["a.example.com"] // servers to delete
   }


Success response:

   {
      "account_id": "account id",
      "site_id": "site_id",
      "operation_id": "operation id"
   }
*/
func (h *WebHandler) createSiteShrinkOperation(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	d := json.NewDecoder(r.Body)
	var req ops.CreateSiteShrinkOperationRequest
	if err := d.Decode(&req); err != nil {
		return trace.BadParameter(err.Error())
	}
	key := siteKey(p)
	req.AccountID = key.AccountID
	req.SiteDomain = key.SiteDomain
	op, err := context.Operator.CreateSiteShrinkOperation(r.Context(), req)
	if err != nil {
		return trace.Wrap(err)
	}
	log.Infof("got operation: %#v", op)
	roundtrip.ReplyJSON(w, http.StatusOK, op)
	return nil
}

/* createSiteUpdateOperation initiates site update operation

   POST /portal/v1/accounts/:account_id/sites/:site_domain/operations/update

   {
      "account_id": "account id",
      "site_id": "site_id",
      "package": "gravitational.io/mattermost:1.2.3"
   }


Success response:

   {
      "account_id": "account id",
      "site_id": "site_id",
      "operation_id": "operation id"
   }
*/
func (h *WebHandler) createSiteUpdateOperation(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	d := json.NewDecoder(r.Body)
	var req ops.CreateSiteAppUpdateOperationRequest
	if err := d.Decode(&req); err != nil {
		return trace.BadParameter(err.Error())
	}
	key := siteKey(p)
	req.AccountID = key.AccountID
	req.SiteDomain = key.SiteDomain
	op, err := context.Operator.CreateSiteAppUpdateOperation(r.Context(), req)
	if err != nil {
		return trace.Wrap(err)
	}
	log.Debugf("got operation: %#v", op)
	roundtrip.ReplyJSON(w, http.StatusOK, op)
	return nil
}

/* createClusterGarbageCollectOperation creates a new garbage operation for the cluster

   POST	/portal/v1/accounts/:account_id/sites/:site_domain/operations/gc

   {
      "account_id": "account id",
      "site_id": "cluster_name",
   }


Success response:

   {
      "account_id": "account id",
      "site_id": "cluster_name",
      "operation_id": "operation id"
   }
*/
func (h *WebHandler) createClusterGarbageCollectOperation(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	d := json.NewDecoder(r.Body)
	var req ops.CreateClusterGarbageCollectOperationRequest
	if err := d.Decode(&req); err != nil {
		return trace.BadParameter(err.Error())
	}

	key := siteKey(p)
	req.AccountID = key.AccountID
	req.ClusterName = key.SiteDomain
	op, err := context.Operator.CreateClusterGarbageCollectOperation(r.Context(), req)
	if err != nil {
		return trace.Wrap(err)
	}

	log.Infof("got operation: %#v", op)
	roundtrip.ReplyJSON(w, http.StatusOK, op)
	return nil
}

/* createClusterReconfigureOperation creates a new cluster reconfiguration operation.

   POST /portal/v1/accounts/:account_id/sites/:site_domain/operations/reconfigure
*/
func (h *WebHandler) createClusterReconfigureOperation(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	var req ops.CreateClusterReconfigureOperationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return trace.BadParameter(err.Error())
	}
	key, err := context.Operator.CreateClusterReconfigureOperation(r.Context(), req)
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, key)
	return nil
}

/* getLogForwarders returns a list of configured log forwarders

   GET /portal/v1/accounts/:account_id/sites/:site_domain/logs/forwarders

Success response:

   []storage.LogForwarder
*/
func (h *WebHandler) getLogForwarders(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	forwarders, err := context.Operator.GetLogForwarders(siteKey(p))
	if err != nil {
		return trace.Wrap(err)
	}
	items := make([]json.RawMessage, len(forwarders))
	for i, forwarder := range forwarders {
		bytes, err := storage.GetLogForwarderMarshaler().Marshal(forwarder)
		if err != nil {
			return trace.Wrap(err)
		}
		items[i] = bytes
	}
	roundtrip.ReplyJSON(w, http.StatusOK, items)
	return nil
}

/* createLogForwarders creates a new log forwarder

   POST /portal/v1/accounts/:account_id/sites/:site_domain/logs/forwarders
*/
func (h *WebHandler) createLogForwarder(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	var req opsclient.UpsertResourceRawReq
	if err := telehttplib.ReadJSON(r, &req); err != nil {
		return trace.Wrap(err)
	}
	forwarder, err := storage.GetLogForwarderMarshaler().Unmarshal(req.Resource)
	if err != nil {
		return trace.Wrap(err)
	}
	if req.TTL != 0 {
		forwarder.SetTTL(clockwork.NewRealClock(), req.TTL)
	}
	err = context.Operator.CreateLogForwarder(r.Context(), siteKey(p), forwarder)
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, statusOK("log forwarder created"))
	return nil
}

/* updateLogForwarders updates an existing log forwarder

   PUT /portal/v1/accounts/:account_id/sites/:site_domain/logs/forwarders/:name
*/
func (h *WebHandler) updateLogForwarder(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	var req opsclient.UpsertResourceRawReq
	if err := telehttplib.ReadJSON(r, &req); err != nil {
		return trace.Wrap(err)
	}
	forwarder, err := storage.GetLogForwarderMarshaler().Unmarshal(req.Resource)
	if err != nil {
		return trace.Wrap(err)
	}
	if req.TTL != 0 {
		forwarder.SetTTL(clockwork.NewRealClock(), req.TTL)
	}
	err = context.Operator.UpdateLogForwarder(r.Context(), siteKey(p), forwarder)
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, statusOK("log forwarder updated"))
	return nil
}

/* deleteLogForwarders deletes a log forwarder

   DELETE /portal/v1/accounts/:account_id/sites/:site_domain/logs/forwarders/:name
*/
func (h *WebHandler) deleteLogForwarder(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	err := context.Operator.DeleteLogForwarder(r.Context(), siteKey(p), p.ByName("name"))
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, statusOK("log forwarder deleted"))
	return nil
}

/* getSMTPConfig returns the cluster SMTP configuration

     GET /portal/v1/accounts/:account_id/sites/:site_domain/smtp

   Success Response:

     storage.SMTPConfig
*/
func (h *WebHandler) getSMTPConfig(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	config, err := context.Operator.GetSMTPConfig(siteKey(p))
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, config)
	return nil
}

/* updateSMTPConfig updates the cluster SMTP configuration

     PUT /portal/v1/accounts/:account_id/sites/:site_domain/smtp

   Success Response:

     {
       "message": "smtp configuration updated"
     }
*/
func (h *WebHandler) updateSMTPConfig(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	var req opsclient.UpsertResourceRawReq
	if err := telehttplib.ReadJSON(r, &req); err != nil {
		return trace.Wrap(err)
	}

	config, err := storage.UnmarshalSMTPConfig(req.Resource)
	if err != nil {
		return trace.Wrap(err)
	}
	if req.TTL != 0 {
		config.SetTTL(clockwork.NewRealClock(), req.TTL)
	}

	err = context.Operator.UpdateSMTPConfig(r.Context(), siteKey(p), config)
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, statusOK("smtp configuration updated"))
	return nil
}

/* deleteSMTPConfig deletes the cluster SMTP configuration

   DELETE /portal/v1/accounts/:account_id/sites/:site_domain/smtp

   Success Response:

     {
       "message": "smtp configuration deleted"
     }
*/
func (h *WebHandler) deleteSMTPConfig(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	err := context.Operator.DeleteSMTPConfig(r.Context(), siteKey(p))
	if err != nil {
		return trace.Wrap(err)
	}

	roundtrip.ReplyJSON(w, http.StatusOK, statusOK("smtp configuration deleted"))
	return nil
}

/* getApplicationEndpoints returns application endpoints for a deployed cluster

     GET /portal/v1/accounts/:account_id/sites/:site_domain/endpoints

   Success Response:

     []ops.Endpoint
*/
func (h *WebHandler) getApplicationEndpoints(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	endpoints, err := context.Operator.GetApplicationEndpoints(siteKey(p))
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, endpoints)
	return nil
}

/* getAppInstaller returns a standalone installer for the specified application

GET /portal/v1/accounts/:account_id/apps/:repository_id/:package_name/:version/installer

   Success Response:

     binary stream with application tarball
*/
func (h *WebHandler) getAppInstaller(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	accountID := p.ByName("account_id")
	locator, err := loc.NewLocator(p.ByName("repository_id"), p.ByName("package_name"), p.ByName("version"))
	if err != nil {
		return trace.Wrap(err)
	}

	if err := r.ParseForm(); err != nil {
		return trace.Wrap(err)
	}

	var installerReq ops.AppInstallerRequest
	requestBytes := r.Form.Get("request")

	err = json.Unmarshal([]byte(requestBytes), &installerReq)
	if err != nil {
		return trace.Wrap(err, "failed to unmarshal `%s`", requestBytes)
	}
	installerReq.AccountID = accountID

	reader, err := context.Operator.GetAppInstaller(installerReq)
	if err != nil {
		return trace.Wrap(err)
	}
	defer reader.Close()

	installerFilename := fmt.Sprintf("%v-%v-installer.tar.gz", locator.Name, locator.Version)
	w.Header().Set("Content-Type", "application/x-gzip")
	w.Header().Set("Content-Disposition", `attachment; filename="`+installerFilename+`"`)
	_, err = io.Copy(w, reader)
	return trace.Wrap(err)
}

/* getClusterCert returns the cluster certificate

     GET /portal/v1/accounts/:account_id/sites/:site_domain/certificate

   Success Response:

     ops.ClusterCertificate
*/
func (h *WebHandler) getClusterCert(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	var withSecrets bool
	var err error
	if r.URL.Query().Get(constants.WithSecretsParam) != "" {
		withSecrets, _, err = telehttplib.ParseBool(r.URL.Query(), constants.WithSecretsParam)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	cert, err := context.Operator.GetClusterCertificate(siteKey(p), withSecrets)
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, cert)
	return nil
}

/* updateClusterCert updates the cluster certificate

     POST /portal/v1/accounts/:account_id/sites/:site_domain/certificate

   Success Response:

     ops.ClusterCertificate
*/
func (h *WebHandler) updateClusterCert(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	var req ops.UpdateCertificateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return trace.BadParameter(err.Error())
	}
	cert, err := context.Operator.UpdateClusterCertificate(r.Context(), req)
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, cert)
	return nil
}

/* deleteClusterCert deletes the cluster certificate

     DELETE /portal/v1/accounts/:account_id/sites/:site_domain/certificate

   Success Response:

    200 certificate deleted
*/
func (h *WebHandler) deleteClusterCert(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	err := context.Operator.DeleteClusterCertificate(r.Context(), siteKey(p))
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, message("certificate deleted"))
	return nil
}

/* emitAuditEvent saves the provided event in the audit log.

     POST /portal/v1/accounts/:account_id/sites/:site_domain/events

   Success response:

     { "message": "audit log event saved" }
*/
func (h *WebHandler) emitAuditEvent(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	var req ops.AuditEventRequest
	err := telehttplib.ReadJSON(r, &req)
	if err != nil {
		return trace.Wrap(err)
	}
	events.Emit(r.Context(), context.Operator, req.Event, events.Fields(req.Fields))
	roundtrip.ReplyJSON(w, http.StatusOK, message("audit log event saved"))
	return nil
}

func (s *WebHandler) needsAuth(fn ServiceHandle) httprouter.Handle {
	return NeedsAuth(s.cfg.Devmode, s.cfg.Backend, s.cfg.Operator, s.cfg.Authenticator, s.cfg.Users, fn)
}

// GetHandlerContext authenticates the user that made the request and returns
// the appropriate handler context
func GetHandlerContext(w http.ResponseWriter, r *http.Request, backend storage.Backend, operator ops.Operator, authenticator users.Authenticator, usersService users.Identity) (*HandlerContext, error) {
	logger := log.WithFields(fields.FromRequest(r))

	authResult, err := authenticator.Authenticate(w, r)
	if err != nil {
		logger.WithError(err).Warn("Authentication error.")
		return nil, trace.AccessDenied("bad username or password") // Hide the actual error.
	}

	// Enrich the request context with additional auth info.
	ctx := r.Context()
	ctx = context.WithValue(ctx, constants.UserContext, authResult.User.GetName())
	if authResult.Session != nil {
		ctx = context.WithValue(ctx, constants.WebSessionContext, authResult.Session.GetWebSession())
	}

	// create a permission aware wrapper packages service
	// and pass it to the handlers, so every action will be automatically
	// checked against current user
	wrappedOperator := ops.OperatorWithACL(operator, usersService, authResult.User, authResult.Checker)
	wrappedIdentity := users.IdentityWithACL(backend, usersService, authResult.User, authResult.Checker)
	if err != nil {
		logger.WithError(err).Error("Failed to init identity service.")
		return nil, trace.BadParameter("internal server error")
	}
	// enrich context with operator bound to current user
	ctx = context.WithValue(ctx, constants.OperatorContext, wrappedOperator)
	handlerContext := &HandlerContext{
		Operator: wrappedOperator,
		User:     authResult.User,
		Checker:  authResult.Checker,
		Identity: wrappedIdentity,
		Context:  ctx,
	}
	return handlerContext, nil
}

// NeedsAuth is authentication wrapper for ops handlers
func NeedsAuth(devmode bool, backend storage.Backend, operator ops.Operator, authenticator users.Authenticator, usersService users.Identity, fn ServiceHandle) httprouter.Handle {
	handler := func(w http.ResponseWriter, r *http.Request, params httprouter.Params) error {
		handlerContext, err := GetHandlerContext(w, r, backend, operator, authenticator, usersService)
		if err != nil {
			return trace.Wrap(err)
		}
		err = fn(w, r.WithContext(handlerContext.Context), params, handlerContext)
		if err != nil {
			return trace.Wrap(err)
		}
		return nil
	}
	return func(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
		err := handler(w, r, params)
		if err != nil {
			if trace.IsAccessDenied(err) {
				log.WithFields(fields.FromRequest(r)).WithError(err).Warn("Access denied.")
			}
			trace.WriteError(w, err)
		}
	}
}

func getLastOperation(key ops.SiteKey, operationType string, context *HandlerContext) (*ops.SiteOperation, error) {
	operations, err := context.Operator.GetSiteOperations(key)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var lastOp *storage.SiteOperation
	for i, op := range operations {
		if op.Type == operationType {
			lastOp = &operations[i]
			break
		}
	}
	if lastOp == nil {
		return nil, trace.NotFound("no operations of type %v found", operationType)
	}
	return (*ops.SiteOperation)(lastOp), nil
}

func getOpLogs(w http.ResponseWriter, r *http.Request, key ops.SiteOperationKey, context *HandlerContext) error {
	reader, err := context.Operator.GetSiteOperationLogs(key)
	if err != nil {
		return trace.Wrap(err)
	}
	ws := &httplib.WebSocketReader{
		Reader: reader,
	}
	defer ws.Close()
	ws.Handler().ServeHTTP(w, r)
	return nil
}

// ServiceHandle defines an ops handler function type
type ServiceHandle func(http.ResponseWriter, *http.Request, httprouter.Params,
	*HandlerContext) error

// HandlerContext is the request context that gets passed into each handler function
type HandlerContext struct {
	// Operator is the operator service
	Operator ops.Operator
	// SiteKey is the request cluster key
	SiteKey ops.SiteKey
	// User is the authenticated user
	User storage.User
	// Identity is the users service
	Identity users.Identity
	// Checker is used for ACL checks
	Checker teleservices.AccessChecker
	// Context is the request context
	Context context.Context
}

func statusOK(message string) interface{} {
	return map[string]string{"status": "ok", "message": message}
}

func siteOperationKey(p httprouter.Params) ops.SiteOperationKey {
	return ops.SiteOperationKey{
		AccountID:   p[0].Value,
		SiteDomain:  p[1].Value,
		OperationID: p[2].Value,
	}
}

func siteKey(p httprouter.Params) ops.SiteKey {
	return ops.SiteKey{
		AccountID:  p[0].Value,
		SiteDomain: p[1].Value,
	}
}
