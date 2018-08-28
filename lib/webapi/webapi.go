// Package webapi implements web proxy handler that provides
// various helpers for web UI, so it's OK
// to put UI specific stuff here
package webapi

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"

	"golang.org/x/net/context"

	"github.com/gravitational/gravity/lib/app"
	appsapi "github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/clients"
	"github.com/gravitational/gravity/lib/cloudprovider/aws"
	awsservice "github.com/gravitational/gravity/lib/cloudprovider/aws/service"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/httplib"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/ops/resources"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/systeminfo"
	"github.com/gravitational/gravity/lib/users"
	"github.com/gravitational/gravity/lib/utils"
	"github.com/gravitational/gravity/lib/webapi/ui"

	"github.com/gravitational/form"
	licenseapi "github.com/gravitational/license"
	teleauth "github.com/gravitational/teleport/lib/auth"
	telehttplib "github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/httplib/csrf"
	"github.com/gravitational/teleport/lib/reversetunnel"
	teleservices "github.com/gravitational/teleport/lib/services"
	teleweb "github.com/gravitational/teleport/lib/web"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
)

// Handler is HTTP web API handler
type Handler struct {
	httprouter.Router
	cfg Config
	log.FieldLogger
	plugin Plugin
}

// Config represents web handler configuration parameters
type Config struct {
	// Identity is identity service provided by web api
	Identity users.Identity
	// PrefixURL is a prefix redirect URL for this
	PrefixURL string
	// Auth is a client to authentication service
	Auth teleauth.ClientI
	// WebAuthenticator is used to authenticate web sessions
	WebAuthenticator httplib.Authenticator
	// Operator is the interface to operations service
	Operator ops.Operator
	// Applications is the interface to application management service
	Applications appsapi.Applications
	// Packages is the interface to package management service
	Packages pack.PackageService
	// Providers defines cloud provider-specific functionality
	Providers Providers
	// Tunnel provides access to remote server
	Tunnel reversetunnel.Server
	// Clients provides access to clients for remote clusters such as operator or apps
	Clients *clients.ClusterClients
	// Converter converts objects to UI representation
	Converter ui.Converter
	// Mode is the mode the process is running in
	Mode string
	// Backend is storage backend
	Backend storage.Backend
	// ProxyHost is the address of Teleport proxy
	ProxyHost string
	// ServiceUser specifies the service user to use to
	// create a cluster with for wizard-based installation
	ServiceUser systeminfo.User
}

// Check validates the config
func (c Config) Check() error {
	if c.Identity == nil {
		return trace.BadParameter("missing Identity")
	}
	if c.PrefixURL == "" {
		return trace.BadParameter("missing PrefixURL")
	}
	if c.Auth == nil {
		return trace.BadParameter("missing Auth")
	}
	if c.WebAuthenticator == nil {
		return trace.BadParameter("missing WebAuthenticator")
	}
	if c.Operator == nil {
		return trace.BadParameter("missing Operator")
	}
	if c.Applications == nil {
		return trace.BadParameter("missing Applications")
	}
	if c.Packages == nil {
		return trace.BadParameter("missing Packages")
	}
	if c.Providers == nil {
		return trace.BadParameter("missing Providers")
	}
	if c.Tunnel == nil {
		return trace.BadParameter("missing Tunnel")
	}
	if c.Clients == nil {
		return trace.BadParameter("missing Clients")
	}
	if c.Converter == nil {
		return trace.BadParameter("missing Converter")
	}
	if c.Mode == "" {
		return trace.BadParameter("missing Mode")
	}
	if c.Backend == nil {
		return trace.BadParameter("missing Backend")
	}
	if c.ProxyHost == "" {
		return trace.BadParameter("missing ProxyHost")
	}
	return nil
}

// Plugin allows to customize handler behavior
type Plugin interface {
	// Resources returns resource controller
	Resources(*AuthContext) (resources.Resources, error)
	// CallbackHandler is the OAuth2 provider callback handler
	CallbackHandler(http.ResponseWriter, *http.Request, CallbackParams) error
}

// CallbackParams combines necessary parameters for OAuth2 callback handler
type CallbackParams struct {
	// Username is the name of the authenticated user
	Username string
	// Identity is the external identity of the authenticated user
	Identity teleservices.ExternalIdentity
	// Session is the created web session
	Session teleservices.WebSession
	// Cert is the generated certificate
	Cert []byte
	// HostSigners is a list of signing host public keys trusted by proxy
	HostSigners []teleservices.CertAuthority
	// Type is the original request type
	Type string
	// CreateWebSession indicates sign in via UI
	CreateWebSession bool
	// CSRFToken the original request CSRF token
	CSRFToken string
	// PublicKey is an optional public key to sign in case of successful authentication
	PublicKey []byte
	// ClientRedirectURL is where successfully authenticated client is redirected
	ClientRedirectURL string
}

// NewAPI returns a new instance of web api handler
func NewAPI(cfg Config) (*Handler, error) {
	err := cfg.Check()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	h := &Handler{
		cfg:         cfg,
		FieldLogger: log.WithField(trace.Component, "webapi"),
	}

	// OAuth2 callbacks
	h.GET("/github/callback", telehttplib.MakeHandler(h.githubCallback))

	// Manage existing user invites
	h.GET("/accounts/existing/invites", h.needsAuth(h.getInvites))
	h.DELETE("/accounts/existing/invites/:email", h.needsAuth(h.deleteInvite))

	// Manage existing roles
	h.GET("/accounts/existing/roles", h.needsAuth(h.getRoles))
	h.PUT("/accounts/existing/roles", h.needsAuth(h.updateRole))
	h.POST("/accounts/existing/roles", h.needsAuth(h.createRole))
	h.DELETE("/accounts/existing/roles/:rolename", h.needsAuth(h.deleteRole))

	// Manage existing users
	h.GET("/accounts/existing/users", h.needsAuth(h.getUsers))
	h.PUT("/accounts/existing/users", h.needsAuth(h.updateUser))
	h.DELETE("/accounts/existing/users/:username", h.needsAuth(h.deleteUser))
	h.POST("/accounts/existing/users/password", h.needsAuth(h.updatePassword))
	h.POST("/sites/:domain/invites", h.needsAuth(h.createUserInviteHandle))
	h.POST("/sites/:domain/users/:username/reset", h.needsAuth(h.createUserResetHandle))

	// Manage yaml configs
	h.GET("/sites/:domain/resources/:kind", h.needsAuth(h.getResourceHandler))
	h.PUT("/sites/:domain/resources", h.needsAuth(h.upsertResourceHandler))
	h.POST("/sites/:domain/resources", h.needsAuth(h.upsertResourceHandler))
	h.DELETE("/sites/:domain/resources/:kind/:name", h.needsAuth(h.deleteResourceHandler))

	// Tokens
	h.GET("/tokens/user/:token", telehttplib.MakeHandler(h.getUserToken))
	h.POST("/tokens/invite/done", telehttplib.WithCSRFProtection(h.inviteUserCompleteHandle))
	h.POST("/tokens/reset/done", telehttplib.WithCSRFProtection(h.resetUserCompleteHandle))
	h.POST("/tokens/install", h.needsAuth(h.generateInstallToken))

	// General validation
	h.GET("/domains/:domain_name", h.needsAuth(h.validateDomainName))

	h.GET("/sites/:domain/operations/:operation_id/progress", h.needsAuth(h.getSiteOperationProgress))

	// Operations
	h.GET("/sites/:domain/operations/:operation_id/agent", h.needsAuth(h.agentReport))
	h.POST("/sites/:domain/operations/:operation_id/start", h.needsAuth(h.startOperation))
	h.DELETE("/sites/:domain/operations/:operation_id", h.needsAuth(h.deleteOperation))
	h.GET("/sites/:domain/operations", h.needsAuth(h.getOperations))
	h.POST("/sites/:domain/operations/:operation_id/prechecks", h.needsAuth(h.validateServers))

	// Sites
	h.POST("/sites", h.needsAuth(h.createSite))
	h.POST("/sites/:domain/expand", h.needsAuth(h.expandSite))
	h.POST("/sites/:domain/shrink", h.needsAuth(h.shrinkSite))
	h.GET("/sites/:domain", h.needsAuth(h.getSite))
	h.GET("/sites", h.needsAuth(h.getSites))
	h.GET("/sites/:domain/servers", h.needsAuth(h.getServers))
	h.GET("/sites/:domain/endpoints", h.needsAuth(h.getSiteEndpoints))
	h.GET("/sites/:domain/report", h.needsAuth(h.getSiteReport))
	h.PUT("/sites/:domain", h.needsAuth(h.updateSiteApp))
	h.PUT("/sites/:domain/grafana", h.needsAuth(h.initGrafana))
	h.DELETE("/sites/:domain", h.needsAuth(h.uninstallSite))
	h.GET("/sites/:domain/uninstall", h.needsAuth(h.uninstallStatus))

	// Flavors for installation
	h.GET("/sites/:domain/flavors", h.needsAuth(h.getFlavors))

	// Monitoring
	h.GET("/sites/:domain/monitoring/retention", h.needsAuth(h.getRetentionPolicies))
	h.PUT("/sites/:domain/monitoring/retention", h.needsAuth(h.updateRetentionPolicy))

	// Certificates
	h.GET("/sites/:domain/certificate", h.needsAuth(h.getCertificate))
	h.PUT("/sites/:domain/certificate", h.needsAuth(h.updateCertificate))

	// Cloud Provider-specific endpoints
	h.POST("/provider", h.needsAuth(h.validateProvider))

	// Apps
	h.GET("/sites/:domain/apps", h.needsAuth(h.getApps))
	h.POST("/apps", h.needsAuth(h.uploadApp))
	h.GET("/apps/:repository/:package/:version", h.needsAuth(h.getAppPackage))
	h.GET("/apps/:repository/:package/:version/installer", h.needsAuth(h.getAppInstaller))

	// User
	h.GET("/user/context", h.needsAuth(h.getWebContext))
	h.GET("/user/status", h.needsAuth(h.getUserStatus))

	// Connect to Pod
	h.GET("/sites/:domain/connect", h.needsAuth(h.clusterContainerConnect))

	h.NotFound = telehttplib.MakeStdHandler(h.notFound)

	h.SetPlugin(h)
	return h, nil
}

func (m *Handler) notFound(w http.ResponseWriter, r *http.Request) (interface{}, error) {
	return nil, trace.NotFound("method not found")
}

// SetPlugin sets the handler plugin
func (m *Handler) SetPlugin(plugin Plugin) {
	m.plugin = plugin
}

// GetConfig returns the handler config
func (m *Handler) GetConfig() Config {
	return m.cfg
}

// CallbackHandler is the generic OAuth2 provider callback handler
func (m *Handler) CallbackHandler(w http.ResponseWriter, r *http.Request, p CallbackParams) error {
	if p.CreateWebSession {
		err := csrf.VerifyToken(p.CSRFToken, r)
		if err != nil {
			m.Warnf("Failed to verify CSRF token: %v.", err)
			return trace.AccessDenied("access denied")
		}
		m.Info("Redirecting to web browser.")
		err = teleweb.SetSession(w, p.Username, p.Session.GetName())
		if err != nil {
			return trace.Wrap(err)
		}
		telehttplib.SafeRedirect(w, r, p.ClientRedirectURL)
		return nil
	}
	if len(p.PublicKey) == 0 {
		return trace.BadParameter("not a web or console login request")
	}
	m.Info("Redirecting to console login.")
	redirectURL, err := teleweb.ConstructSSHResponse(teleweb.AuthParams{
		Username:          p.Username,
		Identity:          p.Identity,
		Session:           p.Session,
		Cert:              p.Cert,
		HostSigners:       p.HostSigners,
		ClientRedirectURL: p.ClientRedirectURL,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	http.Redirect(w, r, redirectURL.String(), http.StatusFound)
	return nil
}

// githubCallback handles the callback from Github during OAuth2 authentication
// flow
//
//   GET /github/callback
//
func (m *Handler) githubCallback(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	result, err := m.cfg.Auth.ValidateGithubAuthCallback(r.URL.Query())
	if err != nil {
		m.Warnf("Error validating callback: %v.", err)
		http.Redirect(w, r, "/web/msg/error/login_failed", http.StatusFound)
		return nil, nil
	}
	m.Infof("Callback: %v %v %v.", result.Username, result.Identity, result.Req.Type)
	return nil, m.plugin.CallbackHandler(w, r, CallbackParams{
		Username:          result.Username,
		Identity:          result.Identity,
		Session:           result.Session,
		Cert:              result.Cert,
		HostSigners:       result.HostSigners,
		Type:              result.Req.Type,
		CreateWebSession:  result.Req.CreateWebSession,
		CSRFToken:         result.Req.CSRFToken,
		PublicKey:         result.Req.PublicKey,
		ClientRedirectURL: result.Req.ClientRedirectURL,
	})
}

func (m *Handler) getUserStatus(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *AuthContext) (interface{}, error) {
	return httplib.OK(), nil
}

// getUserToken returns information about signup token
//
// GET /portalapi/v1/tokens/secret/<token-id>
//
// {
//     "token": "token-id",
//     "invite_email": "invite email",
//     "expires": "RFC3339 expiration timestamp",
//     "type": "token type", // "account_email" if user invited via email or "account_oidc" if it's OIDC invite
//     "account_id": "account-id", // is set if this token is adding user to existing account
//     "username": "username", // is set if this is password recovery token
//     "qr_code": "base64", // base64 encoded qr code bytes
//     "connector_id": "google" // OpenID Connect connector ID
// }
func (m *Handler) getUserToken(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	userToken, err := m.cfg.Identity.GetUserToken(p[0].Value)
	if err != nil {
		log.Errorf("failed to fetch user token: %v", trace.DebugReport(err))
		// we hide the error from the remote user to avoid giving any hints
		return nil, trace.AccessDenied("bad or expired token")
	}
	return userToken, nil
}

type inviteUserReq struct {
	Name  string   `json:"name"`
	Roles []string `json:"roles"`
}

// createUserInviteHandle creates user invite and returns a user token
//
// GET /portalapi/v1/sites/:domain/invites
//
func (m *Handler) createUserInviteHandle(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *AuthContext) (interface{}, error) {
	var req *inviteUserReq
	if err := telehttplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	invite := storage.UserInvite{
		CreatedBy: ctx.User.GetName(),
		Name:      req.Name,
		Roles:     req.Roles,
	}

	userToken, err := ctx.Identity.CreateInviteToken(m.cfg.PrefixURL, invite)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return userToken, nil
}

// createUserResetHandle resets user credentials and returns a user token
//
// GET /portalapi/v1/sites/:domain/users/:username/reset
//
func (m *Handler) createUserResetHandle(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *AuthContext) (interface{}, error) {
	userToken, err := ctx.Identity.CreateResetToken(
		m.cfg.PrefixURL,
		p[1].Value,
		defaults.UserResetTokenTTL)

	if err != nil {
		return nil, trace.Wrap(err)
	}

	return userToken, nil
}

// getInvites returns current active user invites
//
// GET /portalapi/v1/accounts/existing/invites
//
func (m *Handler) getInvites(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *AuthContext) (interface{}, error) {
	invites, err := ctx.Identity.GetUserInvites(ctx.User.GetAccountID())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return invites, nil
}

// getUserACL returns current user access list
//
// GET /portalapi/v1/accounts/existing/access
//
func (m *Handler) getWebContext(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *AuthContext) (interface{}, error) {
	userCtx, err := ui.NewWebContext(ctx.User, ctx.Identity)
	if err != nil {
		return nil, trace.Wrap(err, "Unable to retrieve user information")
	}

	return userCtx, nil
}

// deleteInvite deletes user invite
//
// DELETE /portalapi/v1/accounts/existing/invites/:email
//
// It deletes user invite and all associated tokens
//
func (m *Handler) deleteInvite(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *AuthContext) (interface{}, error) {
	err := ctx.Identity.DeleteUserInvite(ctx.User.GetAccountID(), p[0].Value)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return httplib.OK(), nil
}

// getUsers returns all users and user invites
//
// GET /portalapi/v1/accounts/existing/users
//
// [{"name": "username", "type": "admin", "account_id": "account_id", "site_id": "site_id", "allowed_logins": ["admin"], "identities": [], "email":""}]
//
// To resend the invite simply call inviteUser again with the same email
//
func (m *Handler) getUsers(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *AuthContext) (interface{}, error) {
	users, err := ctx.Identity.GetUsersByAccountID(ctx.User.GetAccountID())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	invites, err := ctx.Identity.GetUserInvites(ctx.User.GetAccountID())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	uiUsers := []ui.User{}

	for _, item := range users {
		if !ui.IsHiddenUserType(item.GetType()) {
			uiUsers = append(uiUsers, ui.NewUserByStorageUser(item))
		}
	}

	for _, item := range invites {
		uiUsers = append(uiUsers, ui.NewUserByInvite(item))
	}

	return uiUsers, nil
}

// deleteRole deletes role
//
// DELETE /portalapi/v1/accounts/existing/roles/:rolename
//
func (m *Handler) deleteRole(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *AuthContext) (interface{}, error) {
	roleName := p.ByName("rolename")
	if err := ctx.Identity.DeleteRole(roleName); err != nil {
		return nil, trace.Wrap(err)
	}

	return httplib.OK(), nil
}

// updateRole updates given role
//
// PUT /portalapi/v1/accounts/existing/roles/:rolename
//
func (m *Handler) updateRole(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *AuthContext) (interface{}, error) {

	return httplib.OK(), nil
}

// createRole creates new role
//
// POST /portalapi/v1/accounts/existing/roles/:rolename
//
func (m *Handler) createRole(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *AuthContext) (interface{}, error) {

	return httplib.OK(), nil
}

func (m *Handler) getRoles(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *AuthContext) (interface{}, error) {

	return nil, nil
}

// updateUser updates existing user
//
// PUT /portalapi/v1/accounts/existing/roles
//
// Input : { "id": "admin@gravitational.com", "name": "", "email": "admin@gravitational.com", "roles": [ "@teleadmin" ], "created": "0001-01-01T00:00:00Z", "status": "active", "owner": true }
//
func (m *Handler) updateUser(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *AuthContext) (interface{}, error) {
	uiUser := ui.User{}
	if err := telehttplib.ReadJSON(r, &uiUser); err != nil {
		return nil, trace.Wrap(err)
	}

	updateReq := storage.UpdateUserReq{
		Roles:    &uiUser.Roles,
		FullName: &uiUser.Name,
	}

	if err := ctx.Identity.UpdateUser(uiUser.Email, updateReq); err != nil {
		return nil, trace.Wrap(err)
	}

	return httplib.OK(), nil
}

// deleteUser deletes user
//
// DELETE /portalapi/v1/accounts/existing/users/:username
//
// It deletes user invite and all associated tokens
//
func (m *Handler) deleteUser(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *AuthContext) (interface{}, error) {
	err := ctx.Identity.DeleteUser(p[0].Value)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return httplib.OK(), nil
}

type updatePasswordReq struct {
	OldPassword users.Password `json:"old_password"`
	NewPassword users.Password `json:"new_password"`
}

// updatePassword
//
// POST /portalapi/v1/accounts/existing/users/password
//
// {"old_password": "base64 encoded old password", "new_password": "base64 encoded new password"}
//
// It changes user's password to the new password if the old password matches
//
func (m *Handler) updatePassword(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *AuthContext) (interface{}, error) {
	var req *updatePasswordReq
	if err := telehttplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	err := ctx.Identity.UpdatePassword(ctx.User.GetName(), req.OldPassword, req.NewPassword)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return httplib.OK(), nil
}

type authenticatedHandler func(
	http.ResponseWriter, *http.Request, httprouter.Params, *AuthContext) (interface{}, error)

type AuthContext struct {
	// User is a current user
	User storage.User
	// Checkers is access checker
	Checker teleservices.AccessChecker
	// Operator is the interface to operations service
	Operator *ops.OperatorACL
	// Applications is the interface to application management service
	Applications appsapi.Applications
	// Packages is the interface to package management service
	Packages pack.PackageService
	// Identity is identity service
	Identity users.Identity
	// SessionContext is a current session context
	SessionContext *teleweb.SessionContext
}

// GetHandlerContext authenticates the session user and returns an appropriate
// handler context
func (m *Handler) GetHandlerContext(w http.ResponseWriter, r *http.Request) (*AuthContext, error) {
	authCreds, err := httplib.ParseAuthHeaders(r)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if authCreds.Type == httplib.AuthBasic {
		return nil, trace.AccessDenied("method not supported")
	}
	session, err := m.cfg.WebAuthenticator(w, r, true)
	if err != nil {
		return nil, trace.AccessDenied("bad username or password")
	}
	user, err := m.cfg.Identity.GetTelekubeUser(session.GetUser())
	if err != nil {
		return nil, trace.AccessDenied("bad username or password")
	}
	checker, err := m.cfg.Identity.GetAccessChecker(user)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &AuthContext{
		User:           user,
		Operator:       ops.OperatorWithACL(m.cfg.Operator, m.cfg.Identity, user, checker),
		Applications:   app.ApplicationsWithACL(m.cfg.Applications, m.cfg.Identity, user, checker),
		Packages:       pack.PackagesWithACL(m.cfg.Packages, m.cfg.Identity, user, checker),
		Identity:       users.IdentityWithACL(m.cfg.Backend, m.cfg.Identity, user, checker),
		Checker:        checker,
		SessionContext: session,
	}, nil
}

func (m *Handler) needsAuth(fn authenticatedHandler) httprouter.Handle {
	return telehttplib.MakeHandler(func(w http.ResponseWriter, r *http.Request, params httprouter.Params) (interface{}, error) {
		context, err := m.GetHandlerContext(w, r)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		result, err := fn(w, r, params, context)
		log.Debugf("%v %v %v", r.Method, r.URL.String(), err)
		return result, trace.Wrap(err)
	})
}

// recoverPasswordComplete finalizes password recovery process
//
// POST /portalapi/v1/recoveries/start
//
// {"password": "base64 password value", "hotp_value": "one time token", "secret_token": "secret recovery token"}
//
// It sets the password and logs the user in
//
func (m *Handler) resetUserCompleteHandle(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	var req users.UserTokenCompleteRequest
	if err := telehttplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := req.Password.Check(); err != nil {
		return nil, trace.Wrap(err)
	}

	websession, err := m.cfg.Identity.ResetUserWithToken(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = teleweb.SetSession(w, websession.GetUser(), websession.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return httplib.OK(), nil
}

func (m *Handler) inviteUserCompleteHandle(w http.ResponseWriter, r *http.Request, p httprouter.Params) (interface{}, error) {
	var req users.UserTokenCompleteRequest
	if err := telehttplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := req.Password.Check(); err != nil {
		return nil, trace.Wrap(err)
	}

	websession, err := m.cfg.Identity.CreateUserWithToken(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := teleweb.SetSession(w, websession.GetUser(), websession.GetName()); err != nil {
		return nil, trace.Wrap(err)
	}
	return httplib.OK(), nil
}

// validateDomainName ensures that the specified domain name is unique
//
// GET /portalapi/v1/sites/:domain_name
//
// Input: /portalapi/v1/sites/example.com
//
// Output:
// {
//   "suggestions": [
//     "example1.com",
//     "my_example.com",
//   ]
// }
//
// It verifies that the provided domain name is unique, otherwise, a list of possible alternatives
// is generated.

func (m *Handler) validateDomainName(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *AuthContext) (interface{}, error) {
	domainName := p[0].Value
	suggestions := []string{}
	if err := ctx.Operator.ValidateDomainName(domainName); err != nil {
		if trace.IsAlreadyExists(err) {
			// TODO: have ValidateDomainName generate alternatives
			suggestions = []string{""}
		} else {
			return nil, trace.Wrap(err)
		}
	}

	return suggestions, nil
}

// validateProvider validates the specified provider and returns basic metadata upon success
//
// POST /portalapi/v1/provider/
//
// Input:
// {
//   "provider": "aws",
//   "variables": {
//     "access_key": "foo",
//     "secret_key": "bar"
//   },
//   "application": "gravitaitonal.io/qux:1.2.3"
// }
//
// Output:
//
// {
//   "aws": {
//     "verify": {
//       "policy": <...>
//     },
//     "vpcs": [
//       {"id": "vpc-e2aaff87", "cidr_block": "172.31.0.0/16", "region": "us-east-1"}
//     ],
//     "regions": [
//       {"name": "us-east-1", "endpoint": "ec2.us-east-1.amazonaws.com"},
//       {"name": "us-west-1", "endpoint": "ec2.us-west-1.amazonaws.com"}
//     ]
//   }
// }
//
// It verifies that the provided credentials are sufficient for install operations and returns
// basic metadata describing the provider
func (m *Handler) validateProvider(w http.ResponseWriter, r *http.Request, p httprouter.Params, authCtx *AuthContext) (interface{}, error) {
	var req ValidateInput
	if err := telehttplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	result, err := m.cfg.Providers.Validate(&req, context.TODO())
	if err != nil {
		if _, ok := trace.Unwrap(err).(awsservice.VerificationError); ok {
			w.WriteHeader(http.StatusForbidden)
		}
		return nil, trace.Wrap(err)
	}

	if schema.IsAWSProvider(req.Provider) && result.AWS != nil {
		app, err := authCtx.Applications.GetApp(loc.Locator(req.Application))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		supportedRegions := app.Manifest.Providers.AWS.Regions
		if len(supportedRegions) != 0 {
			result.AWS.FilterRegions(supportedRegions)
		}
	}

	return result, nil
}

// getSiteOperationProgress returns a progress report for this operation
//
// GET /sites/:domain/portalapi/v1/operations/:operation_id/progress
//
// Output:
//
// {
// 	"site_id": "site id",
// 	"operation_id": "operation id",
// 	"created": "timestamp RFC 3339",
// 	"completion": 39,
// 	"state": "one of 'in_progress', 'failed', or 'completed'",
// 	"message": "message to display to user"
// }
//

func (m *Handler) getSiteOperationProgress(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *AuthContext) (interface{}, error) {
	siteDomain, operationID := p[0].Value, p[1].Value
	site, err := context.Operator.GetSiteByDomain(siteDomain)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	opKey := ops.SiteOperationKey{
		AccountID:   site.AccountID,
		SiteDomain:  site.Domain,
		OperationID: operationID,
	}

	progressEntry, err := context.Operator.GetSiteOperationProgress(opKey)

	if err != nil {
		return nil, trace.Wrap(err)
	}

	return progressEntry, nil
}

// agentReport provides update on the specified active operation
//
// GET /sites/:domain/portalapi/v1/operations/:operation_id/agent
//
// Input: operation_id
//
// Output:
// {
//   "can_continue": false,
//   "message": "waiting for servers to come up",
//   "servers": [...<list of already active (known) servers>...],
//   "instructions": <provisioner-specific installation instructions>,
// }
//
// It provides an update on the active operation: list of already active servers
// as well as provider-specific data.
func (m *Handler) agentReport(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *AuthContext) (interface{}, error) {
	siteDomain, operationID := p[0].Value, p[1].Value
	site, err := context.Operator.GetSiteByDomain(siteDomain)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	opKey := ops.SiteOperationKey{
		AccountID:   site.AccountID,
		SiteDomain:  site.Domain,
		OperationID: operationID,
	}
	app, err := context.Applications.GetApp(site.App.Package)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	op, err := context.Operator.GetSiteOperation(opKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var agentReport *ops.AgentReport
	switch op.Type {
	case ops.OperationInstall:
		agentReport, err = context.Operator.GetSiteInstallOperationAgentReport(opKey)
	case ops.OperationExpand:
		agentReport, err = context.Operator.GetSiteExpandOperationAgentReport(opKey)
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}
	servers := make([]serverInfo, 0, len(agentReport.Servers))
	for _, server := range agentReport.Servers {
		profile, err := app.Manifest.NodeProfiles.ByName(server.Role)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		var mounts []storage.Mount
		for _, m := range profile.Mounts() {
			if m.Hidden {
				continue
			}
			mounts = append(mounts, storage.Mount{
				Name:            m.Name,
				Source:          m.Path,
				Destination:     m.TargetPath,
				CreateIfMissing: utils.BoolValue(m.CreateIfMissing),
				SkipIfMissing:   utils.BoolValue(m.SkipIfMissing),
			})
		}
		for i := range mounts {
			for _, m := range server.Mounts {
				if mounts[i].Name == m.Name {
					mounts[i].Source = m.Source
				}
			}
		}
		servers = append(servers, serverInfo{
			Hostname:   server.GetHostname(),
			Interfaces: server.GetNetworkInterfaces(),
			Devices:    server.GetDevices(),
			Role:       server.Role,
			OSInfo:     server.GetOS(),
			Mounts:     mounts,
		})
	}

	return &agentInfo{
		Message: agentReport.Message,
		Servers: servers,
		// TODO: docker configuration needs to be per-node profile
		Docker: app.Manifest.SystemDocker(),
	}, nil
}

type agentInfo struct {
	// Message a user-friendly message that describes the
	// agent cluster state - e.g. whether all agents have connected
	Message string `json:"message"`
	// Servers describes the connected remote agents
	Servers []serverInfo `json:"servers"`
	// Docker describes docker configuration from the application
	// manifest
	Docker schema.Docker `json:"docker"`
}

type serverInfo struct {
	// Hostname is a server hostname
	Hostname string `json:"hostname"`
	// Interfaces lists all network interfaces from host
	Interfaces map[string]storage.NetworkInterface `json:"interfaces"`
	// Devices lists the disks/partitions on the host
	Devices storage.Devices `json:"devices"`
	// Role defines the application-specific server role
	Role string `json:"role"`
	// OSInfo identifies the host operating system
	OSInfo storage.OSInfo `json:"os"`
	// Mounts lists mount overrides
	Mounts []storage.Mount `json:"mounts"`
}

type siteCreateInput struct {
	// AppPackage defines the application package for install
	AppPackage string `json:"app_package"`
	// DomainName is a name that uniquely identifies the installation
	DomainName string `json:"domain_name"`
	// Provider defines cloud-provider specific settings
	Provider cloudProvider `json:"provider"`
	// License is the license to install on site
	License string `json:"license"`
	// Labels is a custom key/value metadata to attach to a new site
	Labels map[string]string `json:"labels"`
}

type cloudProvider struct {
	// Provisioner sets the provisioner to use for this site
	// Provisioner defaults to the name of the provider with the exception
	// of the case of bare-metal installation - when it is `OnPrem`
	Provisioner string `json:"provisioner"`
	// AWS defines AWS-specific configuration
	AWS *awsProvider `json:"aws"`
	// GCE defines GCE-specific configuration
	GCE *gceProvider `json:"gce"`
	// OnPrem defines the attributes of the bare-metal provider
	OnPrem *onPremProvider `json:"onprem"`
}

type awsProvider struct {
	// AccessKey sets the access key ID part of the AWS API key
	AccessKey string `json:"access_key"`
	// SecretKey sets the secret key part of the AWS API key
	SecretKey string `json:"secret_key"`
	// Region defines an AWS region
	Region string `json:"region"`
	// VPCID identifies the VPC to install into
	// If unspecified, a new VPC will be created
	VPCID string `json:"vpc_id"`
	// KeyPair defines a key pair for SSH access to the provisioned node(s)
	KeyPair string `json:"key_pair"`
}

type gceProvider struct {
	// NodeTags lists additional VM instance tags to add
	NodeTags []string `json:"node_tags"`
}

type onPremProvider struct {
	// PodCIDR is the CIDR range for pod subnet
	PodCIDR string `json:"pod_cidr"`
	// ServiceCIDR is the CIDR range for service subnet
	ServiceCIDR string `json:"service_cidr"`
}

type siteCreateOutput struct {
	// SiteDomain identifies the site as a result of the create operation
	SiteDomain string `json:"site_domain"`
}

// createSite configures an install operation for the specified site.
//
// POST /portalapi/v1/sites/
//
// Input:
// {
//   "app_package": "example.com/foo:1.2.3"
//   "provisioner": "aws_terraform",
//   "domain_name": "example.com",
//   "provider": {
//     "provisioner": 'AWS',
//     "aws": {
//       "access_key": "AADGHJ56gfjy_0j",
//       "secret_key": "dhjkfsdAZDGhh1a9fjy_0j19f3",
//       "region": "us-east-1",
//       "vpc_id": "vpc-124abd7a"
//     }
//   }
// }
//
// Output:
// {
//   "site_id": "344238abcd7"
// }
func (m *Handler) createSite(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *AuthContext) (interface{}, error) {
	var input siteCreateInput
	if err := telehttplib.ReadJSON(r, &input); err != nil {
		return nil, trace.Wrap(err)
	}

	req := ops.NewSiteRequest{
		AppPackage: input.AppPackage,
		AccountID:  context.User.GetAccountID(),
		Email:      context.User.GetName(),
		DomainName: input.DomainName,
		License:    input.License,
		Labels:     input.Labels,
	}

	var vars storage.OperationVariables
	provisioner := input.Provider.Provisioner
	switch {
	case input.Provider.AWS != nil:
		req.Provider = schema.ProviderAWS
		if provisioner == "" {
			provisioner = schema.ProvisionerAWSTerraform
		}
		vars.AWS.Region = input.Provider.AWS.Region
		vars.AWS.AccessKey = input.Provider.AWS.AccessKey
		vars.AWS.SecretKey = input.Provider.AWS.SecretKey
		vars.AWS.VPCID = input.Provider.AWS.VPCID
		vars.AWS.KeyPair = input.Provider.AWS.KeyPair
		req.Location = input.Provider.AWS.Region
	case input.Provider.GCE != nil:
		req.Provider = schema.ProviderGCE
		req.CloudConfig.GCENodeTags = input.Provider.GCE.NodeTags
	case input.Provider.OnPrem != nil:
		req.Provider = schema.ProviderOnPrem
		if provisioner == "" {
			provisioner = schema.ProvisionerOnPrem
		}
		vars.OnPrem.PodCIDR = input.Provider.OnPrem.PodCIDR
		vars.OnPrem.ServiceCIDR = input.Provider.OnPrem.ServiceCIDR
	default:
		return nil, trace.BadParameter("provider unspecified in request")
	}

	if req.ServiceUser.IsEmpty() {
		req.ServiceUser = storage.OSUser{
			Name: m.cfg.ServiceUser.Name,
			UID:  strconv.Itoa(m.cfg.ServiceUser.UID),
			GID:  strconv.Itoa(m.cfg.ServiceUser.GID),
		}
	}
	site, err := context.Operator.CreateSite(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	opReq := ops.CreateSiteInstallOperationRequest{
		SiteDomain:  site.Domain,
		AccountID:   site.AccountID,
		Provisioner: provisioner,
		Variables:   vars,
	}
	key, err := context.Operator.CreateSiteInstallOperation(opReq)
	if err != nil {
		siteKey := site.Key()
		errDelete := context.Operator.DeleteSite(siteKey)
		if errDelete != nil {
			log.Errorf("failed to delete site %v: %v", siteKey, trace.DebugReport(errDelete))
		}
		return nil, trace.Wrap(err)
	}
	return &siteCreateOutput{key.SiteDomain}, nil
}

type siteExpandInput struct {
	// ServerProfile defines the server profile to expand with
	ServerProfile string `json:"profile"`
	// Provider defines cloud-provider specific settings
	Provider cloudProvider `json:"provider"`
}

type siteExpandOutput struct {
	// Operation defines the active expand operation
	Operation ops.SiteOperation `json:"operation"`
}

// expandSite configures an expand operation for the specified site.
//
// POST /portalapi/v1/sites/:site_id/expand
//
// Input:
// {
//   provider: {
//     provisioner: 'aws_terraform',
//     profile: 'database',
//     aws: {
//       access_key: "AADGHJ56gfjy_0j",
//       secret_key: "dhjkfsdAZDGhh1a9fjy_0j19f3",
//     }
//   }
// }
//
// Output:
// {
//   "operation_id": "13423abcf576234d"
// }
//
func (m *Handler) expandSite(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *AuthContext) (interface{}, error) {
	siteDomain := p[0].Value
	site, err := context.Operator.GetSite(ops.SiteKey{
		SiteDomain: siteDomain,
		AccountID:  context.User.GetAccountID(),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var input siteExpandInput
	if err := telehttplib.ReadJSON(r, &input); err != nil {
		return nil, trace.Wrap(err)
	}

	var vars storage.OperationVariables
	provisioner := input.Provider.Provisioner
	switch {
	case input.Provider.AWS != nil:
		if provisioner == "" {
			provisioner = schema.ProvisionerAWSTerraform
		}
		if provisioner == schema.ProvisionerOnPrem {
			break
		}
		vars.AWS.AccessKey = input.Provider.AWS.AccessKey
		vars.AWS.SecretKey = input.Provider.AWS.SecretKey
	case input.Provider.OnPrem != nil:
		if provisioner == "" {
			provisioner = schema.ProvisionerOnPrem
		}
	}

	opReq := ops.CreateSiteExpandOperationRequest{
		SiteDomain:  site.Domain,
		AccountID:   site.AccountID,
		Provisioner: provisioner,
		Variables:   vars,
	}
	if input.ServerProfile != "" {
		opReq.Servers = map[string]int{input.ServerProfile: 1}
	}
	var operationKey *ops.SiteOperationKey
	if operationKey, err = context.Operator.CreateSiteExpandOperation(opReq); err != nil {
		return nil, trace.Wrap(err)
	}
	var operation *ops.SiteOperation
	if operation, err = context.Operator.GetSiteOperation(*operationKey); err != nil {
		return nil, trace.Wrap(err)
	}
	return &siteExpandOutput{*operation}, nil
}

type siteShrinkInput struct {
	// Servers is a list of server hostnames to delete
	Servers []string `json:"servers"`
	// Provider defines cloud-provider specific settings
	Provider cloudProvider `json:"provider"`
}

type siteShrinkOutput struct {
	// Operation is the launched shrink operation
	Operation ops.SiteOperation `json:"operation"`
}

// shrinkSite launches shrink operation for the specified site
//
// POST /portalapi/v1/sites/:site_id/shrink
//
// Input:
// {
//   servers: ["hostname"],
//   provider: {
//     aws: {
//       access_key: "AADGHJ56gfjy_0j",
//       secret_key: "dhjkfsdAZDGhh1a9fjy_0j19f3"
//     }
//   }
// }
//
// Output:
// {
//   "operation": ops.SiteOperation
// }
//
func (m *Handler) shrinkSite(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *AuthContext) (interface{}, error) {
	var input siteShrinkInput
	if err := telehttplib.ReadJSON(r, &input); err != nil {
		return nil, trace.Wrap(err)
	}

	var vars storage.OperationVariables
	if input.Provider.AWS != nil {
		vars.AWS.AccessKey = input.Provider.AWS.AccessKey
		vars.AWS.SecretKey = input.Provider.AWS.SecretKey
	}

	key, err := context.Operator.CreateSiteShrinkOperation(ops.CreateSiteShrinkOperationRequest{
		AccountID:   context.User.GetAccountID(),
		SiteDomain:  p.ByName("domain"),
		Variables:   vars,
		Servers:     input.Servers,
		Provisioner: input.Provider.Provisioner,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	operation, err := context.Operator.GetSiteOperation(*key)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return siteShrinkOutput{Operation: *operation}, nil
}

// getSite retrieves details on the specified site
//
// GET /portalapi/v1/sites/:domain
//
// Input: site_id
//
// Output:
// {
//   "id": "344238abcd7"
//   "created": "2016-05-14 13:00:05"
//   "domain_name": "example.com"
//   "account_id": "1ab238a8cd5"
//   "state": "active"
//   "provisioner": "aws_terraform"
//   "app": {"package": "gravitational.io/test:1.0.0", "manifest": <...application manifest...>}
// }
func (m *Handler) getSite(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *AuthContext) (interface{}, error) {
	siteDomain := p[0].Value
	site, err := context.Operator.GetSite(ops.SiteKey{
		SiteDomain: siteDomain,
		AccountID:  context.User.GetAccountID(),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return site, nil
}

// getApps retrieves the list of site app's packages of all available versions
//
// GET /portalapi/v1/sites/:domain/apps
//
// Input: site_id
//
// Output:
// [{
//   "manifest": {},
//   "package": "repository/package:version"
// }]
func (m *Handler) getApps(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *AuthContext) (interface{}, error) {
	siteDomain := p[0].Value
	site, err := context.Operator.GetSite(ops.SiteKey{
		SiteDomain: siteDomain,
		AccountID:  context.User.GetAccountID(),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var appService app.Applications
	if m.cfg.Mode == constants.ComponentSite {
		// when running in cluster mode, use local apps service
		appService = context.Applications
	} else {
		// when running in Ops Center mode, use remote cluster apps client
		appService, err = m.cfg.Clients.AppsClient(site.Domain)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	apps, err := appService.ListApps(appsapi.ListAppsRequest{
		Repository: site.App.Package.Repository,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var out []appsapi.Application
	for _, app := range apps {
		if app.Package.Name == site.App.Package.Name {
			out = append(out, app)
		}
	}

	return out, nil
}

// uploadApp uploads a new application package
//
// POST /portalapi/v1/apps
//
// Input: application package
//
// Output:
// {
//    "manifest": "application manifest",
//    "package": "application package",
// }
func (m *Handler) uploadApp(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *AuthContext) (interface{}, error) {
	var files form.Files
	if err := form.Parse(r, form.FileSlice("source", &files)); err != nil {
		return nil, trace.Wrap(err)
	}
	defer files.Close()
	if len(files) != 1 {
		return nil, trace.BadParameter("expected a single file but got %v", len(files))
	}

	progressC := make(chan *appsapi.ProgressEntry)
	errorC := make(chan error, 1)
	req := &appsapi.ImportRequest{
		Source:    files[0],
		Email:     context.User.GetName(),
		ProgressC: progressC,
		ErrorC:    errorC,
	}
	op, err := context.Applications.CreateImportOperation(req)

	for _ = range progressC {
	}

	if err = <-errorC; err != nil {
		return nil, trace.Wrap(err)
	}

	app, err := context.Applications.GetImportedApplication(*op)
	return app, trace.Wrap(err)
}

// getCertificate returns information about the cluster certificate
//
// GET /portalapi/v1/sites/:domain/certificate
//
// Output:
//
//   `certificateOutput` struct
//
func (m *Handler) getCertificate(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *AuthContext) (interface{}, error) {
	cert, err := context.Operator.GetClusterCertificate(ops.SiteKey{
		AccountID:  context.User.GetAccountID(),
		SiteDomain: p[0].Value,
	}, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	info, err := utils.ParseCertificate(cert.Certificate)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return info, nil
}

// updateCertificate updates the cluster certificate
//
// PUT /portalapi/v1/sites/:domain/certificate
//
// Input:
//
//   file `certificate` with certificate
//   file `private_key` with private key
//   (optional) file `intermediate` with intermediate certificates
//
// Output:
//
//   `certificateOutput` struct
//
func (m *Handler) updateCertificate(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *AuthContext) (interface{}, error) {
	certificate, err := readFile(r, "certificate")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	privateKey, err := readFile(r, "private_key")
	if err != nil {
		return nil, trace.Wrap(err)
	}

	intermediate, err := readFile(r, "intermediate")
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}

	cert, err := context.Operator.UpdateClusterCertificate(ops.UpdateCertificateRequest{
		AccountID:    context.User.GetAccountID(),
		SiteDomain:   p[0].Value,
		Certificate:  certificate,
		PrivateKey:   privateKey,
		Intermediate: intermediate,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	info, err := utils.ParseCertificate(cert.Certificate)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return info, nil
}

// readFile reads the file by the provided name from the request and
// returns its content
func readFile(r *http.Request, name string) ([]byte, error) {
	var files form.Files
	err := form.Parse(r, form.FileSlice(name, &files))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer files.Close()
	if len(files) == 0 {
		return nil, trace.NotFound("file %q is not provided", name)
	}
	if len(files) != 1 {
		return nil, trace.BadParameter("expected 1 file %q, got %v", name, len(files))
	}
	data, err := ioutil.ReadAll(files[0])
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return data, nil
}

// getSites retrieves details of all sites for the specified account
//
// GET /portalapi/v1/sites
//
// Input: site_id
//
// Output:
// [{
//   "id": "344238abcd7"
//   "created": "2016-05-14 13:00:05"
//   "domain_name": "example.com"
//   "account_id": "1ab238a8cd5"
//   "state": "active"
//   "provisioner": "aws_terraform"
//   "app": {"package": "gravitational.io/test:1.0.0", "manifest": <...application manifest...>}
// }]
func (m *Handler) getSites(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *AuthContext) (interface{}, error) {
	sites, err := context.Operator.GetSites(context.User.GetAccountID())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return sites, nil
}

type siteUpdateInput struct {
	Package string `json:"package"`
}

type siteUpdateOutput struct {
	OperationID string `json:"operation_id"`
}

// updateSiteApp starts the operation of updating the application installed on the site
//
// PUT /portalapi/v1/sites/:site_id
//
// Input: site_id, package
//
// Output:
// {
//   "operation_id": "123456789"
// }
func (m *Handler) updateSiteApp(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *AuthContext) (interface{}, error) {
	var input siteUpdateInput
	if err := telehttplib.ReadJSON(r, &input); err != nil {
		return nil, trace.Wrap(err)
	}
	req := ops.CreateSiteAppUpdateOperationRequest{
		AccountID:  context.User.GetAccountID(),
		SiteDomain: p[0].Value,
		App:        input.Package,
	}
	log.Infof("got site update operation request: %v", req)
	op, err := context.Operator.CreateSiteAppUpdateOperation(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &siteUpdateOutput{op.OperationID}, nil
}

type uninstallSiteInput struct {
	// Variables contains operation specific parameters, e.g. AWS keys
	Variables struct {
		// AccessKey is AWS API access key
		AccessKey string `json:"access_key"`
		// SecretKey is AWS API secret key
		SecretKey string `json:"secret_key"`
	} `json:"variables"`
	// Force ignores the errors during application uninstallation
	Force bool `json:"force"`
	// Remove removes the site entry from the database but does not touch
	// provisioned servers or running application
	Remove bool `json:"remove_only"`
}

// uninstallSite uninstalls resources and deletes state of the specified site
//
// DELETE /portalapi/v1/sites/:domain
//
// Input:
//
//   uninstallSiteInput
//
// Output:
//
//   {
//     "message": "ok"
//   }
//
// The operation proceeds even in case of operational errors and always succeeds.
func (m *Handler) uninstallSite(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *AuthContext) (interface{}, error) {
	var input uninstallSiteInput
	if err := telehttplib.ReadJSON(r, &input); err != nil {
		return nil, trace.Wrap(err)
	}

	// if we're asked only to remove the site from OpsCenter, do not launch uninstall operation
	if input.Remove {
		log.Infof("removing site %v from OpsCenter", p.ByName("domain"))
		err := context.Operator.DeleteSite(ops.SiteKey{
			AccountID:  context.User.GetAccountID(),
			SiteDomain: p.ByName("domain"),
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		return httplib.OK(), nil
	}

	opKey, err := context.Operator.CreateSiteUninstallOperation(ops.CreateSiteUninstallOperationRequest{
		AccountID:  context.User.GetAccountID(),
		SiteDomain: p.ByName("domain"),
		Force:      input.Force,
		Variables: storage.OperationVariables{
			AWS: storage.AWSVariables{
				AccessKey: input.Variables.AccessKey,
				SecretKey: input.Variables.SecretKey,
			},
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	go monitorUninstallProgress(context.Operator, *opKey)
	return httplib.OK(), nil
}

func (m *Handler) uninstallStatus(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *AuthContext) (interface{}, error) {
	status, err := ui.GetUninstallStatus(context.User.GetAccountID(), p.ByName("domain"), context.Operator)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return status, nil
}

func monitorUninstallProgress(operator ops.Operator, opKey ops.SiteOperationKey) {
	var progress *ops.ProgressEntry
	var err error
	for {
		time.Sleep(defaults.ProgressPollTimeout)
		progress, err = operator.GetSiteOperationProgress(opKey)
		if err != nil {
			break
		}
		switch progress.State {
		case ops.ProgressStateCompleted:
		case ops.ProgressStateFailed:
			err = trace.Errorf(progress.Message)
		default:
			continue
		}
		break
	}
	if err != nil {
		log.Errorf("failed to wait on uninstall operation: %v", err)
	}
}

// getEndpoints returns a list of endpoints of the application installed on site
//
//   GET /portalapi/v1/sites/:domain/endpoints
//
// Input:
//
//   none
//
// Output:
//
//   []ops.Endpoint
func (m *Handler) getSiteEndpoints(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *AuthContext) (interface{}, error) {
	endpoints, err := context.Operator.GetApplicationEndpoints(ops.SiteKey{
		AccountID:  context.User.GetAccountID(),
		SiteDomain: p.ByName("domain"),
	})
	return endpoints, trace.Wrap(err)
}

// getSiteReport returns a tarball with collected information about the site
//
//   GET /portalapi/v1/sites/:domain/report
//
// Input:
//
//   none
//
// Output:
//
//   report.tar
func (m *Handler) getSiteReport(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *AuthContext) (interface{}, error) {
	reader, err := context.Operator.GetSiteReport(ops.SiteKey{
		AccountID:  context.User.GetAccountID(),
		SiteDomain: p.ByName("domain"),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer reader.Close()

	w.Header().Set("Content-Type", "application/x-gzip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%v", defaults.ReportTarball))

	_, err = io.Copy(w, reader)
	return nil, trace.Wrap(err)
}

// getServers obtains the list of server nodes for the specified site
//
// GET /portalapi/v1/sites/:domain/servers
//
// Input:
//   site_id
//
// Output:
// [{
//   "hostname": "foo.example.com",
//   "ip": "192.176.178.31",
//   "role": "database",
//   "display_role": "database server",
//   "instance_type": "c3.2xlarge"
// }]
//
func (m *Handler) getServers(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *AuthContext) (interface{}, error) {
	siteDomain := p.ByName("domain")
	site, err := context.Operator.GetSite(ops.SiteKey{
		SiteDomain: siteDomain,
		AccountID:  context.User.GetAccountID(),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	remoteSite, err := m.cfg.Tunnel.GetSite(site.Domain)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	client, err := remoteSite.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	nodes, err := client.GetNodes(defaults.Namespace)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	servers := make([]serverListItem, 0, len(nodes))
	for _, node := range nodes {
		labels := node.GetAllLabels()
		servers = append(servers, serverListItem{
			Server: storage.Server{
				Hostname:    labels[ops.Hostname],
				AdvertiseIP: labels[ops.AdvertiseIP],
				Role:        labels[ops.AppRole],
			},
			DisplayRole:  labels[schema.DisplayRole],
			InstanceType: labels[ops.InstanceType],
			PublicIPv4:   labels[defaults.TeleportPublicIPv4Label],
			ServerID:     node.GetName(),
		})
	}
	return servers, nil
}

type serverListItem struct {
	storage.Server
	DisplayRole  string `json:"display_role"`
	InstanceType string `json:"instance_type"`
	PublicIPv4   string `json:"public_ipv4"`
	// ServerID matches the name of the server from metadata
	// and is equivalent to server ID from the V1 schema
	ServerID string `json:"id"`
}

// getFlavors returns a list of flavors for the installation.
//
// GET /portalapi/v1/sites/:domain/flavors
//
// Output:
// {
//   "title": "Flavors title",
//   "items": [
//     {
//       "name": "Flavor 1",
//        ...
//     },
//     {
//       "name": "Flavor 2",
//        ...
//     },
//   ]
// }
func (m *Handler) getFlavors(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *AuthContext) (interface{}, error) {
	siteDomain := p[0].Value
	site, err := context.Operator.GetSiteByDomain(siteDomain)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	app, err := context.Applications.GetApp(site.App.Package)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var licensePayload *licenseapi.Payload
	if site.License != nil {
		licensePayload = &site.License.Payload
	}

	return getSliderOptions(site, app, licensePayload), nil
}

type sliderOptions struct {
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Default     string         `json:"default"`
	Items       []sliderOption `json:"items"`
}

type sliderOption struct {
	Name        string                `json:"name"`
	Description string                `json:"description"`
	Profiles    map[string]sliderItem `json:"profiles"`
}

type sliderItem struct {
	Count         int      `json:"count"`
	InstanceTypes []string `json:"instance_types"`
}

// getSliderOptions returns a list of app flavors that satisfy the provided license.
func getSliderOptions(site *ops.Site, app *appsapi.Application, license *licenseapi.Payload) sliderOptions {
	// if the app does not have flavors, return an empty list right away
	if len(app.Manifest.Installer.Flavors.Items) == 0 {
		return sliderOptions{}
	}

	options := sliderOptions{
		Title:       app.Manifest.Installer.Flavors.Prompt,
		Description: app.Manifest.Installer.Flavors.Description,
		Default:     app.Manifest.Installer.Flavors.Default,
	}

	// before returning flavors, filter out instance types not supported in the site's region
	defer func() {
		for i, option := range options.Items {
			for j, profile := range option.Profiles {
				options.Items[i].Profiles[j] = sliderItem{
					Count:         profile.Count,
					InstanceTypes: aws.SupportedInstanceTypes(site.Location, profile.InstanceTypes),
				}
			}
		}
	}()

	allInstanceTypes := make(map[string][]string)
	for _, profile := range app.Manifest.NodeProfiles {
		allInstanceTypes[profile.Name] = profile.Providers.AWS.InstanceTypes
	}

	// if the app does not require license, return all flavors
	if app.Manifest.License == nil || !app.Manifest.License.Enabled {
		for _, flavor := range app.Manifest.Installer.Flavors.Items {
			options.Items = append(
				options.Items, flavorToSliderOption(flavor, allInstanceTypes))
		}
		return options
	}

	allowedInstanceTypes := make(map[string][]string)
	for name, types := range allInstanceTypes {
		allowedInstanceTypes[name] = license.FilterInstanceTypes(types)
	}

	// if license is present, pick only matching flavors
FlavorsLoop:
	for _, flavor := range app.Manifest.Installer.Flavors.Items {
		// calculate how many nodes in total this flavor has
		totalCount := 0
		for _, node := range flavor.Nodes {
			totalCount += node.Count
		}

		// make sure the license allows this many nodes
		err := license.CheckCount(totalCount)
		if err != nil {
			continue
		}

		// make sure the license allows at least some instance types for the flavor
		if site.IsAWS() {
			for _, node := range flavor.Nodes {
				if len(allowedInstanceTypes[node.Profile]) == 0 {
					continue FlavorsLoop
				}
			}
		}

		// the flavor satisfies all the criterias, add it to the resulting list
		options.Items = append(
			options.Items, flavorToSliderOption(flavor, allowedInstanceTypes))
	}

	return options
}

func flavorToSliderOption(flavor schema.Flavor, instanceTypes map[string][]string) sliderOption {
	option := sliderOption{
		Name:        flavor.Name,
		Description: flavor.Description,
		Profiles:    make(map[string]sliderItem),
	}
	for _, node := range flavor.Nodes {
		option.Profiles[node.Profile] = sliderItem{
			Count:         node.Count,
			InstanceTypes: instanceTypes[node.Profile],
		}
	}
	return option
}

/* getAppPackage streams the contents of the specified application package

GET /portalapi/v1/apps/:repository_id/:package/:version

*/
func (m *Handler) getAppPackage(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *AuthContext) (interface{}, error) {
	locator, err := loc.NewLocator(p.ByName("repository"), p.ByName("package"), p.ByName("version"))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	_, reader, err := context.Packages.ReadPackage(*locator)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer reader.Close()

	readSeeker, ok := reader.(io.ReadSeeker)
	if !ok {
		return nil, trace.BadParameter("expected read seeker object")
	}
	w.Header().Set("Content-Type", "application/x-tar")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%v.tar`, locator.String()))
	http.ServeContent(w, r, locator.String(), time.Now(), readSeeker)
	return nil, nil
}

/* getAppInstaller generates a tarball with a standlone installer for application
   package specified with repository_name/package_name/version and returns a binary byte stream
   of its contents

GET /portalapi/v1/apps/:repository_id/:package/:version/installer

*/
func (m *Handler) getAppInstaller(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *AuthContext) (interface{}, error) {
	locator, err := loc.NewLocator(p.ByName("repository"), p.ByName("package"), p.ByName("version"))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	reader, err := context.Operator.GetAppInstaller(ops.AppInstallerRequest{
		AccountID:   context.User.GetAccountID(),
		Application: *locator,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer reader.Close()

	w.Header().Set("Content-Type", "application/x-gzip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(
		`attachment; filename="%v-installer.tar.gz"`, locator.String()))
	_, err = io.Copy(w, reader)
	return nil, trace.Wrap(err)
}

// getRetentionPolicies returns a list of configured retention policies for a site
//
//   GET /sites/:domain/monitoring/retention
//
// Input:
//
//   -
//
// Output:
//
//   []ops.RetentionPolicy
func (m *Handler) getRetentionPolicies(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *AuthContext) (interface{}, error) {
	return context.Operator.GetRetentionPolicies(ops.SiteKey{
		AccountID:  context.User.GetAccountID(),
		SiteDomain: p.ByName("domain"),
	})
}

// updateRetentionPolicy updates site's retention policies
//
//   PUT /sites/:domain/monitoring/retention
//
// Input:
//
//   []updateRetentionInput
//
// Output:
//
//   {"message": "OK"}
func (m *Handler) updateRetentionPolicy(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *AuthContext) (interface{}, error) {
	var inputs []updateRetentionInput
	err := telehttplib.ReadJSON(r, &inputs)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, input := range inputs {
		err = context.Operator.UpdateRetentionPolicy(ops.UpdateRetentionPolicyRequest{
			AccountID:  context.User.GetAccountID(),
			SiteDomain: p.ByName("domain"),
			Name:       input.Name,
			Duration:   input.Duration,
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return httplib.OK(), nil
}

// updateRetentionInput is the input for "update retention policy" API call
type updateRetentionInput struct {
	// Name is the retention policy name
	Name string `json:"name"`
	// Duration is the new policy duration
	Duration time.Duration `json:"duration"`
}

type webAPIResponse struct {
	Items interface{} `json:"items"`
}

// makeResponse takes a collection of objects and returns API response object
func makeResponse(items interface{}) (interface{}, error) {
	return webAPIResponse{Items: items}, nil
}
