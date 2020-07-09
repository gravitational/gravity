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

package webapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gravitational/gravity/lib/clients"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/httplib"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/users"
	"github.com/gravitational/gravity/lib/webapi/ui"

	"github.com/gravitational/teleport"
	telehttplib "github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/httplib/csrf"
	"github.com/gravitational/teleport/lib/service"
	teleweb "github.com/gravitational/teleport/lib/web"
	telewebui "github.com/gravitational/teleport/lib/web/ui"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
)

// WebHandler serves web UI
type WebHandler struct {
	// Router is used to route web requests
	httprouter.Router
	// cfg is the web handler configuration
	cfg WebHandlerConfig
	// FieldLogger allows handler to log messages
	log.FieldLogger
}

// WebHandlerConfig defines a configuration object for the handler
type WebHandlerConfig struct {
	// AssetsDir is the directory containing web assets
	AssetsDir string
	// Mode is the gravity process mode
	Mode string
	// Wizard is whether this process is install wizard
	Wizard bool
	// TeleportConfig is the teleport configuration
	TeleportConfig *service.Config
	// Identity is the cluster user service
	Identity users.Identity
	// Operator is the cluster operator service
	Operator ops.Operator
	// Authenticator is used to authenticate web requests
	Authenticator httplib.Authenticator
	// Forwarder is used to forward web requests to clusters
	Forwarder Forwarder
	// Backend is the cluster backend
	Backend storage.Backend
	// Clients provides access to remote cluster client
	Clients *clients.ClusterClients
}

// NewHandler returns a new instance of NewHandler
func NewHandler(cfg WebHandlerConfig) *WebHandler {
	wh := &WebHandler{
		cfg:         cfg,
		FieldLogger: log.WithField(trace.Component, "webhandler"),
	}

	noLoginRoutes := []string{
		"/web/login",
		"/web/recover/*rest",
		"/web/password_reset",
		"/web/invite/*rest",
		"/web/msg/*rest",
	}
	for _, route := range noLoginRoutes {
		wh.Handle("GET", route, wh.noLogin(wh.defaultHandler))
	}

	// root
	wh.Handle("GET", "/web", wh.needsLogin(wh.rootHandler))
	wh.Handle("GET", "/web/", wh.needsLogin(wh.rootHandler))

	// portal
	wh.Handle("GET", "/web/portal", wh.needsLogin(wh.defaultHandler))
	wh.Handle("GET", "/web/portal/*rest", wh.needsLogin(wh.defaultHandler))

	// installer
	wh.Handle("GET", "/web/installer/new/*rest", wh.needsLogin(wh.installerHandler))
	wh.Handle("GET", "/web/installer/site/:site_domain", wh.needsLogin(wh.installerHandler))
	for _, verb := range []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"} {
		// to handle bandwagon API calls
		wh.Handle(verb, "/web/installer/site/:site_domain/*rest", wh.needsLogin(wh.installerHandler))
	}

	// cluster
	wh.Handle("GET", "/web/site/:site_domain", wh.needsLogin(wh.siteHandler))
	wh.Handle("GET", "/web/site/:site_domain/*rest", wh.needsLogin(wh.siteHandler))

	// grafana
	wh.Handle("GET", grafanaURL+"/*rest", wh.needsLogin(wh.grafanaServeHandler))

	// static files
	ServeStaticFiles(wh, "/web/app/*filepath", http.Dir(filepath.Join(cfg.AssetsDir, "app")))
	wh.Handle("GET", "/web/config.js", wh.noLogin(wh.configHandler))

	// all routes not specified here are handled by the "not found" handler which actually
	// just serves the index page so frontend can figure out how to handle it itself
	wh.NotFound = wh.notFound()

	return wh
}

// ServeStaticFiles serves static files such as js/css/images.
// https://github.com/julienschmidt/httprouter/issues/40
// (this is as is copy of julienschmidt ServerFile method that adds security headers)
func ServeStaticFiles(wh *WebHandler, path string, root http.FileSystem) {
	if len(path) < 10 || path[len(path)-10:] != "/*filepath" {
		panic("path must end with /*filepath in path '" + path + "'")
	}

	fileServer := http.FileServer(root)
	wh.GET(path, func(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
		telehttplib.SetStaticFileHeaders(w.Header())
		// do not list directory files
		url := req.URL.Path
		if url[len(url)-1] == '/' {
			http.NotFound(w, req)
			return
		}

		req.URL.Path = ps.ByName("filepath")
		fileServer.ServeHTTP(w, req)
	})
}

func (h *WebHandler) configHandler(w http.ResponseWriter, r *http.Request, p httprouter.Params, s session) {
	telehttplib.SetWebConfigHeaders(w.Header())
	config := &ui.WebConfig{}
	config.SystemInfo.Wizard = h.cfg.Wizard
	config.Auth = getWebConfigAuthSettings(h.cfg)
	config.Modules.OpsCenter.Features.LicenseGenerator.Enabled = true

	cluster, err := h.cfg.Operator.GetLocalSite(r.Context())
	if err != nil && !trace.IsNotFound(err) {
		log.Errorf("Failed to get local site: %v.", trace.DebugReport(err))
		replyError(w, "failed to get local site", http.StatusInternalServerError)
		return
	}
	if cluster != nil {
		config.SystemInfo.ClusterName = cluster.Domain
		if h.cfg.Mode == constants.ComponentSite {
			config.Routes.DefaultEntry = fmt.Sprintf("/web/site/%v", cluster.Domain)
		}
		manifest := cluster.App.Manifest
		config.User.Logo = manifest.Logo
		// TODO(r0mant): Ideally our manifest would have something like
		// display name but for now provide custom headers for our
		// system images.
		switch manifest.Metadata.Name {
		case defaults.TelekubePackage:
			config.User.Login.HeaderText = defaults.GravityDisplayName
		case defaults.OpsCenterPackage:
			config.User.Login.HeaderText = defaults.GravityHubDisplayName
		default:
			config.User.Login.HeaderText = manifest.Metadata.Name
		}
	}

	if h.cfg.Mode == constants.ComponentSite || h.cfg.Wizard {
		config.Modules.OpsCenter.Features.LicenseGenerator.Enabled = false
	}

	out, err := json.Marshal(config)
	if err != nil {
		log.Errorf("failed to marshal config %v: %v", config, trace.DebugReport(err))
		replyError(w, "failed to marshal config", http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, "var GRV_CONFIG = %v;", string(out))
}

func (h *WebHandler) rootHandler(w http.ResponseWriter, r *http.Request, p httprouter.Params, s session) {
	cluster, err := h.cfg.Operator.GetLocalSite(r.Context())
	if err != nil && !trace.IsNotFound(err) {
		log.Errorf("Failed to get local site: %v.", trace.DebugReport(err))
		replyError(w, "failed to get local site", http.StatusInternalServerError)
		return
	}
	if cluster != nil && h.cfg.Mode == constants.ComponentSite {
		http.Redirect(w, r, fmt.Sprintf("/web/site/%v", cluster.Domain), http.StatusFound)
		return
	}

	h.defaultHandler(w, r, p, s)
}

func (h *WebHandler) defaultHandler(w http.ResponseWriter, r *http.Request, p httprouter.Params, s session) {
	telehttplib.SetIndexHTMLHeaders(w.Header())
	if r.URL.Path == "" || r.URL.Path == "/" {
		// redirect to web app root so it can handle further redirects properly (e.g. redirect
		// to the default route)
		http.Redirect(w, r, "/web", http.StatusFound)
		return
	}

	indexPath := filepath.Join(h.cfg.AssetsDir, "/index.html")
	indexContent, err := ioutil.ReadFile(indexPath)
	if err != nil {
		log.Error(trace.DebugReport(err))
		http.Redirect(w, r, "/web/msg/error/login_failed", http.StatusFound)
		return
	}

	indexPage, err := template.New("index").Parse(string(indexContent))
	if err != nil {
		log.Error(trace.DebugReport(err))
		http.Redirect(w, r, "/web/msg/error/login_failed", http.StatusFound)
		return
	}

	csrfToken, err := csrf.AddCSRFProtection(w, r)
	if err != nil {
		log.Errorf("failed to generate CSRF token %v", err)
	}

	tmplValues := struct {
		Session string
		XCSRF   string
	}{
		XCSRF:   csrfToken,
		Session: s.Session,
	}

	indexPage.Execute(w, tmplValues)
}

// installHandler serves /web/installer/site/<sitename> and its subpaths
func (h *WebHandler) installerHandler(w http.ResponseWriter, r *http.Request, p httprouter.Params, s session) {
	rest := p.ByName("rest")

	if strings.HasPrefix(rest, "/complete") {
		h.completeHandler(w, r, p, s)
		return
	}

	h.defaultHandler(w, r, p, s)
}

// siteHandler serves /web/site/<sitename> and its subpaths
func (h *WebHandler) siteHandler(w http.ResponseWriter, r *http.Request, p httprouter.Params, s session) {
	rest := p.ByName("rest")

	if strings.HasPrefix(rest, "/offline") {
		h.offlineHandler(w, r, p, s)
		return
	}

	siteDomain := p.ByName("site_domain")
	site, err := h.cfg.Operator.GetSiteByDomain(siteDomain)
	if err != nil {
		h.siteNotFoundHandler(w, r, p)
		return
	}

	if strings.HasPrefix(rest, "/uninstall") {
		h.defaultHandler(w, r, p, s)
		return
	}

	if site.State == ops.SiteStateUninstalling {
		redirectToUninstall(w, r, siteDomain)
		return
	}

	// if failed, try checking the last operation to make a redirect to installer or uninstaller
	if site.State == ops.SiteStateFailed {
		lastOp, _, err := ops.GetLastOperation(site.Key(), s.Operator)
		if err != nil && !trace.IsNotFound(err) {
			log.Errorf(trace.DebugReport(err))
		}

		if lastOp != nil && lastOp.Type == ops.OperationUninstall {
			redirectToUninstall(w, r, siteDomain)
			return
		}

		if lastOp != nil && lastOp.Type == ops.OperationInstall {
			http.Redirect(w, r, "/web/installer/site/"+siteDomain, http.StatusFound)
			return
		}
	}

	// the site has completed its final install step
	if site.FinalInstallStepComplete {
		h.defaultHandler(w, r, p, s)
		return
	}

	endpoint := site.App.Manifest.SetupEndpoint()

	// the app does not have a custom install step
	if endpoint == nil {
		h.defaultHandler(w, r, p, s)
		return
	}

	// if the app defines custom install step and the site hasn't completed it yet,
	// always redirect the site back to installer
	http.Redirect(w, r, "/web/installer/site/"+siteDomain, http.StatusFound)
}

// completeHandler serves /web/installer/site/<sitename>/complete
func (h *WebHandler) completeHandler(w http.ResponseWriter, r *http.Request, p httprouter.Params, s session) {
	siteDomain := p.ByName("site_domain")

	// we need the trailing slash so all relative URLs are served from under /complete/...
	if r.URL.Path == "/web/installer/site/"+siteDomain+"/complete" {
		http.Redirect(w, r, r.URL.Path+"/", http.StatusFound)
		return
	}

	site, err := h.cfg.Operator.GetSiteByDomain(siteDomain)
	if err != nil {
		h.siteNotFoundHandler(w, r, p)
		return
	}

	// if the site has completed its final installation step, redirect back to the site page
	if site.FinalInstallStepComplete {
		if r.Method == "POST" || r.Method == "PUT" || r.Method == "DELETE" {
			replyError(w, "The installation has already been completed", http.StatusBadRequest)
			return
		}
		http.Redirect(w, r, "/web/site/"+siteDomain, http.StatusFound)
		return
	}

	endpoint := site.App.Manifest.SetupEndpoint()

	// same if the app does not define the custom installer step (we should not
	// have gotten here in the first place)
	if endpoint == nil {
		http.Redirect(w, r, "/web/site/"+siteDomain, http.StatusFound)
		return
	}

	telehttplib.SetIndexHTMLHeaders(w.Header())
	ctx := context.WithValue(context.TODO(), constants.WebSessionContext, s.ctx.GetWebSession())
	ctx = context.WithValue(ctx, constants.OperatorContext, s.Operator)

	err = h.cfg.Forwarder.ForwardToService(w, r.WithContext(ctx), ForwardRequest{
		ClusterName:      site.Domain,
		ServiceName:      endpoint.ServiceName,
		ServicePort:      endpoint.Port,
		ServiceNamespace: endpoint.Namespace,
		URL:              strings.TrimPrefix(p.ByName("rest"), "/complete"),
	})
	if err != nil {
		replyError(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// offlineHandler serves /web/site/<sitename>/offline
func (h *WebHandler) offlineHandler(w http.ResponseWriter, r *http.Request, p httprouter.Params, s session) {
	siteDomain := p.ByName("site_domain")

	// if the site is indeed offline, the default handler will display the proper "offline" page
	site, err := h.cfg.Operator.GetSiteByDomain(siteDomain)
	if err != nil || site.State == ops.SiteStateOffline || site.State == ops.SiteStateUninstalling {
		h.defaultHandler(w, r, p, s)
		return
	}

	// otherwise, the site is available so redirect to its page /web/site/<sitename>
	http.Redirect(w, r, strings.TrimSuffix(r.URL.Path, p.ByName("rest")), http.StatusFound)
}

// siteNotFoundHandler serves the usecase when we failed to retrieve a site - it may be
// because the site simply does not exist or is offline
func (h *WebHandler) siteNotFoundHandler(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	siteDomain := p.ByName("site_domain")

	// if the site "wasn't found" then it might not exist at all
	_, err := h.cfg.Backend.GetSite(siteDomain)
	if err != nil {
		h.notFound().ServeHTTP(w, r)
		return
	}

	// otherwise it is offline
	redirectOffline(w, r, siteDomain)
}

// replyError replies with JSON-formatted error message and specified error code
func replyError(w http.ResponseWriter, message string, code int) {
	bytes, err := json.Marshal(map[string]string{"message": message})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Error(w, string(bytes), code)
}

// redirectOffline performs a redirect to the site's "offline" page
func redirectOffline(w http.ResponseWriter, r *http.Request, siteDomain string) {
	http.Redirect(w, r, "/web/site/"+siteDomain+"/offline", http.StatusFound)
}

func redirectToUninstall(w http.ResponseWriter, r *http.Request, siteDomain string) {
	http.Redirect(w, r, "/web/site/"+siteDomain+"/uninstall", http.StatusFound)
}

func (h *WebHandler) notFound() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Debugf("Not found handler: %v %v", r.Method, r.URL)
		h.noLogin(h.defaultHandler)(w, r, nil)
	}
}

func (h *WebHandler) noLogin(handle webHandle) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		handle(w, r, p, session{Session: base64.StdEncoding.EncodeToString([]byte("{}"))})
	}
}

func (h *WebHandler) needsLogin(handle webHandle) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		err := func() error {
			session, err := h.authenticate(w, r)
			if err == nil {
				handle(w, r, p, *session)
				return nil
			}
			token, errToken := h.tryLoginWithToken(w, r)
			if errToken != nil {
				log.Warningf("failed to authenticate: %v %v", err, errToken)
				return trace.Wrap(errToken)
			}
			if token == nil {
				return trace.Wrap(err)
			}
			// if the token is already associated with a cluster, it means
			// the installation has been initiated already so redirect the
			// user to the cluster's installer page
			if token.SiteDomain != "" {
				http.Redirect(w, r, "/web/installer/site/"+token.SiteDomain, http.StatusFound)
			} else {
				http.Redirect(w, r, r.URL.Path, http.StatusFound)
			}
			return nil
		}()

		if err != nil {
			if !trace.IsAccessDenied(err) {
				log.Error(trace.DebugReport(err))
			}

			http.Redirect(w, r, "/web/login?redirect_uri="+r.URL.Path, http.StatusFound)
		}
	}
}

func (h *WebHandler) tryLoginWithToken(w http.ResponseWriter, r *http.Request) (*storage.InstallToken, error) {
	tokenID, err := h.getToken(w, r)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if tokenID == "" {
		return nil, nil
	}
	token, err := h.loginWithToken(tokenID, w, r)
	if err != nil {
		return nil, trace.Wrap(err, "failed to login with token %v", tokenID)
	}
	return token, nil
}

func (h *WebHandler) getToken(w http.ResponseWriter, r *http.Request) (tokenID string, err error) {
	if h.cfg.Wizard {
		// Look up a token by user ID
		token, err := h.cfg.Identity.GetInstallTokenByUser(defaults.WizardUser)
		if err != nil && !trace.IsNotFound(err) {
			return "", trace.Wrap(err, "failed to get an install token for wizard user")
		}
		if token == nil {
			return "", trace.NotFound("install token for wizard user not found")
		}
		tokenID = token.Token
	} else {
		tokenID = r.URL.Query().Get(ops.InstallToken)
	}
	return tokenID, nil
}

func (h *WebHandler) authenticate(w http.ResponseWriter, r *http.Request) (*session, error) {
	ctx, err := h.cfg.Authenticator(w, r, false)
	if err != nil {
		return nil, trace.Wrap(err, "failed to authenticate")
	}

	resp, err := teleweb.NewSessionResponse(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "failed to query session response")
	}

	user, err := h.cfg.Identity.GetTelekubeUser(ctx.GetUser())
	if err != nil {
		log.Errorf("failed to fetch user: %v", trace.DebugReport(err))
		// we hide the error from the remote user to avoid giving any hints
		return nil, trace.AccessDenied("bad username or password")
	}
	checker, err := h.cfg.Identity.GetAccessChecker(user)
	if err != nil {
		log.Errorf("failed to fetch roles: %v", trace.DebugReport(err))
		// we hide the error from the remote user to avoid giving any hints
		return nil, trace.AccessDenied("bad username or password")
	}

	wrappedOperator := ops.OperatorWithACL(h.cfg.Operator, h.cfg.Identity, user, checker)

	bearerToken := *resp
	bearerTokenJSON, err := json.Marshal(bearerToken)
	if err != nil {
		return nil, trace.Wrap(err, "failed to unmarshal session response")
	}

	return &session{
		Session:  base64.StdEncoding.EncodeToString(bearerTokenJSON),
		Username: ctx.GetUser(),
		ctx:      ctx,
		Operator: wrappedOperator,
		UserInfo: ops.UserInfo{
			User: user,
		},
	}, nil
}

// loginWithToken logs in the user linked to token specified with tokenID
func (h *WebHandler) loginWithToken(tokenID string, w http.ResponseWriter, r *http.Request) (*storage.InstallToken, error) {
	log.Debugf("logging in with token %v", tokenID)
	token, err := h.cfg.Identity.GetInstallToken(tokenID)
	if err != nil {
		return nil, trace.Wrap(err, "failed to retrieve install token")
	}
	result, err := h.cfg.Identity.LoginWithInstallToken(tokenID)
	if err != nil {
		return nil, trace.Wrap(err, "failed to login with token %v", tokenID)
	}
	if err = teleweb.SetSession(w, result.Email, result.SessionID); err != nil {
		return nil, trace.Wrap(err, "failed to create a web session")
	}
	return token, nil
}

// getWebConfigAuthConnectors returns a list of webConfigAuthConnector
func getWebConfigAuthSettings(cfg WebHandlerConfig) telewebui.WebConfigAuthSettings {
	authProviders := []telewebui.WebConfigAuthProvider{}
	withSecrets := false

	// get OIDC connectors
	teleOIDCProviders, err := cfg.Identity.GetOIDCConnectors(withSecrets)
	if err == nil {
		for _, item := range teleOIDCProviders {
			authProviders = append(authProviders, ui.NewOIDCAuthProvider(item.GetName(), item.GetDisplay()))
		}

	} else {
		log.Errorf("Failed to get a list of OIDC connectors: %v.", trace.DebugReport(err))
	}

	// get SAML connectors
	teleSAMLProviders, err := cfg.Identity.GetSAMLConnectors(withSecrets)
	if err == nil {
		for _, item := range teleSAMLProviders {
			authProviders = append(authProviders, ui.NewSAMLAuthProvider(item.GetName(), item.GetDisplay()))
		}

	} else {
		log.Errorf("Failed to get a list of SAML connectors: %v.", trace.DebugReport(err))
	}

	// get github connectors
	teleGithubProviders, err := cfg.Identity.GetGithubConnectors(withSecrets)
	if err == nil {
		for _, item := range teleGithubProviders {
			authProviders = append(authProviders, ui.NewGithubAuthProvider(item.GetName(), item.GetDisplay()))
		}

	} else {
		log.Errorf("Failed to get a list of SAML connectors: %v.", trace.DebugReport(err))
	}

	// get cluster auth. second factor
	cap, err := cfg.Identity.GetAuthPreference()
	capSecondFactor := teleport.OTP
	if err != nil {
		log.Errorf("Cannot retrieve authentication preference: %v.", err)
	} else {
		capSecondFactor = cap.GetSecondFactor()
	}

	authSettings := telewebui.WebConfigAuthSettings{
		Providers:    authProviders,
		SecondFactor: capSecondFactor,
	}

	return authSettings
}

type session struct {
	Session  string
	Username string
	Operator ops.Operator
	UserInfo ops.UserInfo
	ctx      *teleweb.SessionContext
}

type webHandle func(w http.ResponseWriter, r *http.Request, p httprouter.Params, s session)
