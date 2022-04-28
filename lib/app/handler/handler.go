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

package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/gravitational/gravity/lib/app"
	serviceapi "github.com/gravitational/gravity/lib/app/api"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/helm"
	"github.com/gravitational/gravity/lib/httplib"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/users"
	"github.com/gravitational/gravity/lib/utils"
	"github.com/gravitational/gravity/lib/utils/fields"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/form"
	"github.com/gravitational/roundtrip"
	"github.com/gravitational/teleport/lib/auth"
	telehttplib "github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/websocket"
)

// WebHandlerConfig defines app service web handler configuration.
type WebHandlerConfig struct {
	// Users provides access to the users service.
	Users users.Identity
	// Applications provides access to the application service.
	Applications app.Applications
	// Packages provides access to the package service.
	Packages pack.PackageService
	// Charts provides access to the chart repository.
	Charts helm.Repository
	// Authenticator is used to authenticate requests.
	Authenticator users.Authenticator
}

// CheckAndSetDefaults validates the config and sets some defaults.
func (c *WebHandlerConfig) CheckAndSetDefaults() error {
	if c.Applications == nil {
		return trace.BadParameter("missing parameter Applications")
	}
	if c.Users == nil {
		return trace.BadParameter("missing parameter Users")
	}
	if c.Authenticator == nil {
		c.Authenticator = users.NewAuthenticatorFromIdentity(c.Users)
	}
	return nil
}

type WebHandler struct {
	httprouter.Router
	WebHandlerConfig
	middleware *auth.AuthMiddleware
}

func NewWebHandler(cfg WebHandlerConfig) (*WebHandler, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	h := &WebHandler{
		WebHandlerConfig: cfg,
	}

	// Wrap the router in the authentication middleware which will detect
	// if the client is trying to authenticate using a client certificate,
	// extract user information from it and add it to the request context.
	h.middleware = &auth.AuthMiddleware{
		AccessPoint: users.NewAccessPoint(cfg.Users),
	}
	h.middleware.Wrap(&h.Router)

	h.OPTIONS("/*path", h.options)
	h.GET("/app/v1/applications/:repository_id", h.needsAuth(h.listApps))
	h.GET("/app/v1/applications", h.needsAuth(h.listApps))
	h.GET("/app/v1/applications/", h.needsAuth(h.listApps))
	h.POST("/app/v1/operations/import", h.needsAuth(h.createImportOperation))
	h.GET("/app/v1/operations/import/:operation_id", h.needsAuth(h.getImportedApp))
	h.GET("/app/v1/operations/import/:operation_id/progress", h.needsAuth(h.getOperationProgress))
	h.GET("/app/v1/operations/import/:operation_id/logs", h.needsAuth(h.getOperationLogs))
	h.GET("/app/v1/operations/import/:operation_id/crash-report", h.needsAuth(h.getOperationCrashReport))
	h.POST("/app/v1/operations/export/:repository_id/:package_id/:version", h.needsAuth(h.exportApp))
	h.POST("/app/v1/operations/uninstall/:repository_id/:package_id/:version", h.needsAuth(h.uninstallApp))
	h.GET("/app/v1/applications/:repository_id/:package_id/:version", h.needsAuth(h.getApp))
	h.POST("/app/v1/applications/:repository_id", h.needsAuth(h.createApp))
	h.POST("/app/v1/applications/:repository_id/:package_id/:version/hook/start", h.needsAuth(h.startAppHook))
	h.GET("/app/v1/applications/:repository_id/:package_id/:version/hook/:namespace/:name/wait", h.needsAuth(h.waitAppHook))
	h.GET("/app/v1/applications/:repository_id/:package_id/:version/hook/:namespace/:name/stream", h.needsAuth(h.streamAppHookLogs))
	h.DELETE("/app/v1/applications/:repository_id/:package_id/:version/hook/:namespace/:name", h.needsAuth(h.deleteAppHookJob))

	h.GET("/app/v1/applications/:repository_id/:package_id/:version/status", h.needsAuth(h.getAppStatus))
	h.DELETE("/app/v1/applications/:repository_id/:package_id/:version", h.needsAuth(h.deleteApp))
	h.GET("/app/v1/applications/:repository_id/:package_id/:version/manifest", h.needsAuth(h.getAppManifest))
	h.GET("/app/v1/applications/:repository_id/:package_id/:version/resources", h.needsAuth(h.getAppResources))
	h.GET("/app/v1/applications/:repository_id/:package_id/:version/standalone-installer", h.needsAuth(h.getAppInstaller))
	// this method will allow to pass access token in headers, so web api should submit a form to the following URL
	// the server will reply with the download response
	h.POST("/app/v1/applications/:repository_id/:package_id/:version/standalone-installer", h.needsAuth(h.getAppInstaller))

	// Gravity install URLs
	h.GET("/telekube/install", h.wrap(h.telekubeInstallScript))
	h.GET("/telekube/install/:version", h.wrap(h.telekubeInstallScript))
	h.GET("/telekube/gravity", h.wrap(h.telekubeGravityBinary))
	h.GET("/telekube/bin/:version/:os/:arch/:binary", h.wrap(h.telekubeBinary))

	// Helm charts repository handlers.
	h.GET("/charts/:name", h.needsAuth(h.fetchChart))
	h.GET("/app/v1/charts/:name", h.needsAuth(h.fetchChart)) // Alias for /charts/:name for easier testing.

	return h, nil
}

// ServeHTTP lets the authentication middleware serve the request before
// passing it through to the router.
func (s *WebHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.middleware.ServeHTTP(w, r)
}

/* createAppImportOperation initiates import of an application.

   POST	/app/v1/operations/import

   {
      "source": application_data,
      "request": app.ImportRequest
   }


Success response:

   {
      "id": operation_id,
      "repository": "gravitational.io",
      "package": "mattermost",
      "version": "1.2.1",
      "created": timestamp RFC 3339,
      "updated": timestamp RFC 3339,
      "state": operation_specific_state
   }
*/
func (h *WebHandler) createImportOperation(w http.ResponseWriter,
	req *http.Request, params httprouter.Params,
	context *handlerContext) error {

	var files form.Files
	var request string

	err := form.Parse(req,
		form.FileSlice("source", &files),
		form.String("request", &request),
	)
	if err != nil {
		return trace.Wrap(err)
	}

	if len(files) != 1 {
		return trace.BadParameter("expected a single file but got %d", len(files))
	}

	var importReq app.ImportRequest
	if err = json.Unmarshal([]byte(request), &importReq); err != nil {
		return trace.Wrap(err)
	}
	// Note, the source file from the request is passed on as a io.ReadCloser
	// and will be closed once the import operation is finished
	importReq.Source = files[0]

	operation, err := context.applications.CreateImportOperation(&importReq)
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, operation)
	return nil
}

/* getOperationProgress queries the progress of an active operation

GET /app/v1/operations/import/:operation_id/progress

Success Response:

  {
	"operation_id": operation_id,
	"created": "timestamp RFC 3339",
        "completion": completion_value,
	"state": operation_specific_state,
	"message": "message to display to user"
  }
*/
func (h *WebHandler) getOperationProgress(w http.ResponseWriter,
	req *http.Request, params httprouter.Params,
	context *handlerContext) error {

	op, err := importOperation(params)
	if err != nil {
		return trace.Wrap(err)
	}
	progress, err := context.applications.GetOperationProgress(*op)
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, progress)
	return nil
}

/* getOperationLogs returns logs for this import operation as a websocket stream

GET /app/v1/operations/import/:operation_id/logs

*/
func (h *WebHandler) getOperationLogs(w http.ResponseWriter,
	req *http.Request, params httprouter.Params,
	context *handlerContext) error {

	op, err := importOperation(params)
	if err != nil {
		return trace.Wrap(err)
	}
	reader, err := context.applications.GetOperationLogs(*op)
	if err != nil {
		return trace.Wrap(err)
	}
	ws := &httplib.WebSocketReader{
		Reader: reader,
	}
	defer ws.Close()
	ws.Handler().ServeHTTP(w, req)
	return nil
}

/* getImportedApp queries the data for an imported application

GET /app/v1/operations/import/:operation_id

Success Response:

  {
	"package": application_package,
	"manifest": application_manifest,
  }

*/
func (h *WebHandler) getImportedApp(w http.ResponseWriter,
	req *http.Request, params httprouter.Params,
	context *handlerContext) error {

	op, err := importOperation(params)
	if err != nil {
		return trace.Wrap(err)
	}
	app, err := context.applications.GetImportedApplication(*op)
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, app)
	return nil
}

/* getOperationCrashReport returns a file upload with a tarball of a crash dump for this operation

GET /app/v1/operations/import/:operation_id/crash-report

*/
func (h *WebHandler) getOperationCrashReport(w http.ResponseWriter,
	req *http.Request, params httprouter.Params,
	context *handlerContext) error {

	op, err := importOperation(params)
	if err != nil {
		return trace.Wrap(err)
	}
	report, err := context.applications.GetOperationCrashReport(*op)
	if err != nil {
		return trace.Wrap(err)
	}
	w.Header().Set("Content-Disposition", "attachment; filename=crashreport.tar")
	_, err = io.Copy(w, report)
	return err
}

/* getAppManifest returns manifest for the specified application

GET /app/v1/applications/:repository_id/:package_id/:version/manifest

*/
func (h *WebHandler) getAppManifest(w http.ResponseWriter,
	req *http.Request, params httprouter.Params,
	context *handlerContext) error {

	locator, err := appPackage(params)
	if err != nil {
		return trace.Wrap(err)
	}

	reader, err := context.applications.GetAppManifest(*locator)
	if err != nil {
		return trace.Wrap(err)
	}
	defer reader.Close()

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="app.yaml"`)
	_, err = io.Copy(w, reader)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

/* getAppResources returns resources for the specified application

GET /app/v1/applications/:repository_id/:package_id/:version/resources

*/
func (h *WebHandler) getAppResources(w http.ResponseWriter,
	req *http.Request, params httprouter.Params,
	context *handlerContext) error {

	locator, err := appPackage(params)
	if err != nil {
		return trace.Wrap(err)
	}

	reader, err := context.applications.GetAppResources(*locator)
	if err != nil {
		return trace.Wrap(err)
	}
	defer reader.Close()

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="resources.tar"`)
	_, err = io.Copy(w, reader)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

/* getAppInstaller streams a standalone installer tarball to client

GET /app/v1/applications/:repository_id/:package_id/:version/standalone-installer

*/
func (h *WebHandler) getAppInstaller(w http.ResponseWriter,
	req *http.Request, params httprouter.Params,
	context *handlerContext) error {

	locator, err := appPackage(params)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := req.ParseForm(); err != nil {
		return trace.Wrap(err)
	}

	var rawInstallerReq app.InstallerRequestRaw
	requestBytes := req.Form.Get("request")
	err = json.Unmarshal([]byte(requestBytes), &rawInstallerReq)
	if err != nil {
		return trace.Wrap(err, "failed to unmarshal `%s`", requestBytes)
	}

	installerReq, err := rawInstallerReq.ToNative()
	if err != nil {
		return trace.Wrap(err)
	}
	installerReq.Application = *locator

	reader, err := context.applications.GetAppInstaller(*installerReq)
	if err != nil {
		return trace.Wrap(err)
	}
	defer reader.Close()

	installerFilename := fmt.Sprintf("%v-%v-installer.tar.gz",
		installerReq.Application.Name, installerReq.Application.Version)
	w.Header().Set("Content-Type", "application/x-gzip")
	w.Header().Set("Content-Disposition", `attachment; filename="`+installerFilename+`"`)
	_, err = io.Copy(w, reader)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

/* listApps returns a list of applications (optionally for specific repository)

GET /app/v1/applications/:repository_id

Success Response:

  [
	  {
		"package": application_package,
		"manifest": application_manifest,
	  }
  ]
*/
func (h *WebHandler) listApps(w http.ResponseWriter, req *http.Request, params httprouter.Params, context *handlerContext) (err error) {
	var repositories []string
	repository := params.ByName("repository_id")
	if repository == "" {
		repositories, err = h.Packages.GetRepositories()
		if err != nil {
			return trace.Wrap(err)
		}
	} else {
		repositories = append(repositories, repository)
	}
	var appType storage.AppType
	appTypeS := req.FormValue("type")
	if appTypeS == "" {
		appType = storage.AppUser
	} else {
		appType = storage.AppType(appTypeS)
	}
	pattern := req.FormValue("pattern")
	excludeHidden := true // do not display hidden apps in the control panel by default
	if req.FormValue("exclude_hidden") != "" {
		excludeHidden, err = strconv.ParseBool(req.FormValue("exclude_hidden"))
		if err != nil {
			return trace.Wrap(err)
		}
	}
	var apps []app.Application
	for _, repository := range repositories {
		batch, err := context.applications.ListApps(app.ListAppsRequest{
			Repository:    repository,
			Type:          appType,
			ExcludeHidden: excludeHidden,
			Pattern:       pattern,
		})
		if err != nil {
			return trace.Wrap(err)
		}
		apps = append(apps, batch...)
	}

	roundtrip.ReplyJSON(w, http.StatusOK, apps)
	return nil
}

/* getApp queries the data of a specific application

GET /app/v1/applications/:repository_id/:package_id/:version

Success Response:

  {
	"package": application_package,
	"manifest": application_manifest,
  }
*/
func (h *WebHandler) getApp(w http.ResponseWriter, req *http.Request,
	params httprouter.Params, context *handlerContext) error {

	locator, err := appPackage(params)
	if err != nil {
		return trace.Wrap(err)
	}

	app, err := context.applications.GetApp(*locator)
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, app)
	return nil
}

/* exportApp exports containers of an existing application to the specified
docker registry.

POST /app/v1/operations/export/:repository_id/:package_id/:version

  {
	"registryHostPort": registry_address
  }

Success Response:

  {
	"ok": true
  }
*/
func (h *WebHandler) exportApp(w http.ResponseWriter, req *http.Request,
	params httprouter.Params, context *handlerContext) error {

	locator, err := appPackage(params)
	if err != nil {
		return trace.Wrap(err)
	}
	var config serviceapi.ExportConfig
	if err := json.NewDecoder(req.Body).Decode(&config); err != nil {
		return trace.Wrap(err)
	}
	if err = context.applications.ExportApp(app.ExportAppRequest{
		Package:         *locator,
		RegistryAddress: config.RegistryHostPort,
	}); err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, true)
	return nil
}

/* createApp creates a new application record.

POST /app/v1/applications/:repository_id/
  {
  	"package": package_data,
  	"labels": map_of_labels
  }

Success Response:

  {
	"package": application_package,
	"manifest": application_manifest
  }
*/
func (h *WebHandler) createApp(w http.ResponseWriter, req *http.Request,
	params httprouter.Params, context *handlerContext) error {

	var files form.Files
	var labelsMap string
	var upsertS string
	var manifestS string
	err := form.Parse(req,
		form.FileSlice("package", &files),
		form.String("labels", &labelsMap),
		form.String("upsert", &upsertS),
		form.String("manifest", &manifestS),
	)
	if err != nil {
		return trace.Wrap(err)
	}

	if len(files) != 1 {
		return trace.BadParameter("expected a single file parameter but got %d", len(files))
	}

	var labels map[string]string
	if err := json.Unmarshal([]byte(labelsMap), &labels); err != nil {
		return trace.Wrap(err)
	}

	defer func() {
		if err := files.Close(); err != nil {
			log.Errorf("failed to close files: %v", err)
		}
	}()

	reader := files[0]
	appPackageName, err := utils.ParseFilename(req, "package")
	if err != nil {
		return trace.Wrap(err, "failed to parse package name")
	}
	locator, err := loc.ParseLocator(appPackageName)
	if err != nil {
		return trace.BadParameter(err.Error())
	}

	upsert, err := strconv.ParseBool(upsertS)
	if err != nil {
		return trace.Wrap(err)
	}

	var application *app.Application
	if upsert {
		application, err = context.applications.UpsertApp(*locator, reader, labels)
	} else {
		if len(manifestS) != 0 {
			application, err = context.applications.CreateAppWithManifest(*locator, []byte(manifestS), reader, labels)
		} else {
			application, err = context.applications.CreateApp(*locator, reader, labels)
		}
	}
	if err != nil {
		return trace.Wrap(err)
	}

	roundtrip.ReplyJSON(w, http.StatusOK, application)
	return nil
}

/* getAppStatus queries the status of a running application.

GET /app/v1/applications/:repository_id/:package_id/:version

Success Response:

  {
	"code": status_code,
	"services": [
		{
			"name": name,
			"nodePort": port_number,
		}
	]
  }
*/
func (h *WebHandler) getAppStatus(w http.ResponseWriter,
	req *http.Request, params httprouter.Params,
	context *handlerContext) error {

	locator, err := appPackage(params)
	if err != nil {
		return trace.Wrap(err)
	}
	status, err := context.applications.StatusApp(*locator)
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, status)
	return nil
}

/* startAppHook starts application hook, it's a non-blocking call

POST /app/v1/applications/:repository_id/:package_id/:version/hook/start

Success Response:

   hook-specific output
*/
func (h *WebHandler) startAppHook(w http.ResponseWriter,
	req *http.Request, params httprouter.Params,
	context *handlerContext) error {

	locator, err := appPackage(params)
	if err != nil {
		return trace.Wrap(err)
	}

	var hookReq app.HookRunRequest
	requestBytes, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return trace.Wrap(err)
	}
	if err = json.Unmarshal(requestBytes, &hookReq); err != nil {
		return trace.Wrap(err)
	}
	hookReq.Application = *locator

	ref, err := context.applications.StartAppHook(req.Context(), hookReq)
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, ref)
	return nil
}

/* waitAppHook blocks until job running app hook fails or succeeds

GET /app/v1/applications/:repository_id/:package_id/:version/hook/:namespace/:name/wait

Success Response:

   hook-specific output
*/
func (h *WebHandler) waitAppHook(w http.ResponseWriter,
	req *http.Request, params httprouter.Params,
	context *handlerContext) error {

	locator, err := appPackage(params)
	if err != nil {
		return trace.Wrap(err)
	}
	hookRef := app.HookRef{
		Application: *locator,
		Name:        params.ByName("name"),
		Namespace:   params.ByName("namespace"),
	}
	err = context.applications.WaitAppHook(req.Context(), hookRef)
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, "job completed")
	return nil
}

/* streamAppHookLogs starts streaming logs for the application hook specified with namespace/name pair

GET /app/v1/applications/:repository_id/:package_id/:version/hook/:namespace/:name/stream

Success Response:

   hook-specific output
*/
func (h *WebHandler) streamAppHookLogs(w http.ResponseWriter,
	req *http.Request, params httprouter.Params,
	context *handlerContext) error {

	locator, err := appPackage(params)
	if err != nil {
		return trace.Wrap(err)
	}
	hookRef := app.HookRef{
		Application: *locator,
		Name:        params.ByName("name"),
		Namespace:   params.ByName("namespace"),
	}
	server := &websocket.Server{
		Handler: func(ws *websocket.Conn) {
			defer ws.Close()
			err = context.applications.StreamAppHookLogs(req.Context(), hookRef, ws)
			if err != nil {
				log.Warningf("application logs stream closed: %v", trace.DebugReport(err))
			}
		},
	}
	server.ServeHTTP(w, req)
	return nil
}

/* deleteAppHookJob deletes app hook job

DLETE /app/v1/applications/:repository_id/:package_id/:version/hook/:namespace/:name

Success Response:

   ok
*/
func (h *WebHandler) deleteAppHookJob(w http.ResponseWriter,
	req *http.Request, params httprouter.Params,
	context *handlerContext) error {

	locator, err := appPackage(params)
	if err != nil {
		return trace.Wrap(err)
	}
	hookRef := app.HookRef{
		Application: *locator,
		Name:        params.ByName("name"),
		Namespace:   params.ByName("namespace"),
	}
	cascade, _, err := telehttplib.ParseBool(req.URL.Query(), "cascade")
	if err != nil {
		return trace.Wrap(err)
	}
	err = context.applications.DeleteAppHookJob(req.Context(), app.DeleteAppHookJobRequest{
		HookRef: hookRef,
		Cascade: cascade,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, "job deleted")
	return nil
}

/* uninstallApp uninstalls a running application.

POST /app/v1/operations/uninstall/:repository_id/:package_id/:version

Success Response:

  {
	"package": application_package,
	"manifest": application_manifest,
  }
*/
func (h *WebHandler) uninstallApp(w http.ResponseWriter, req *http.Request,
	params httprouter.Params, context *handlerContext) error {

	locator, err := appPackage(params)
	if err != nil {
		return trace.Wrap(err)
	}
	app, err := context.applications.UninstallApp(*locator)
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, app)
	return nil
}

/* deleteApp deletes an application record.

DELETE /app/v1/applications/:repository_id/:package_id/:version?force=true

Success Response:

   {
     "ok": true
   }

*/
func (h *WebHandler) deleteApp(w http.ResponseWriter, req *http.Request, params httprouter.Params, context *handlerContext) error {
	locator, err := appPackage(params)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := req.ParseForm(); err != nil {
		return trace.Wrap(err)
	}

	var force bool
	if _, ok := req.Form["force"]; ok {
		force, err = strconv.ParseBool(req.Form.Get("force"))
		if err != nil {
			return trace.Wrap(err)
		}
	}

	deleteReq := app.DeleteRequest{
		Package: *locator,
		Force:   force,
	}

	if err = context.applications.DeleteApp(deleteReq); err != nil {
		return trace.Wrap(err)
	}

	roundtrip.ReplyJSON(w, http.StatusOK, true)
	return nil
}

/* telekubeInstallScript returns a bash script for installing telekube tools of provided version on a target machine

   GET /telekube/install
   GET /telekube/install/:version
*/
func (h *WebHandler) telekubeInstallScript(w http.ResponseWriter, r *http.Request, p httprouter.Params) error {
	ver := p.ByName("version")
	if ver == "" {
		ver = constants.LatestVersion
	}

	// Security: sanitize semver input
	semver, err := semver.NewVersion(ver)
	if err != nil {
		return trace.Wrap(err)
	}
	err = utils.SanitizeSemver(*semver)
	if err != nil {
		return trace.Wrap(err)
	}

	tfVersion, err := getTerraformVersion(ver, h.Packages)
	if err != nil {
		return trace.Wrap(err)
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)

	err = telekubeInstallScriptTemplate.Execute(w, map[string]string{
		"version":   ver,
		"tfVersion": tfVersion,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func getTerraformVersion(binaryVersion string, packages pack.PackageService) (tfVersion string, err error) {
	// Check whether the terraform provider is available
	// Note: The terraform provider for older releases may not be published, so the following code tries to detect
	// whether the terraform provider has been published for the requested release
	//
	// This works by attempting to locate the package, and resolving the version in the tfVersion variable:
	// NotFound  -> ""          - Don't try and install if it doesn't exist for the specified version
	// <version> -> "<version>" - If a specific version is requested, install that version
	// latest    -> "<version>" - If latest is specified, resolve that to the latest available version.
	tfVersion = constants.LatestVersion
	if binaryVersion != constants.LatestVersion {
		_, err := semver.NewVersion(binaryVersion)
		if err != nil {
			return "", trace.BadParameter("the provided version is not valid: %v", binaryVersion)
		}
		tfVersion = binaryVersion
	}

	// hard code our module lookup based on linux/x86_64, if it exists, we assume it exists
	// for other requested architectures/os, which won't be detected until later
	name := strings.Join([]string{constants.TerraformGravityPackage, "linux", "x86_64"}, "_")
	locator, err := loc.NewLocator(defaults.SystemAccountOrg, name, tfVersion)
	if err != nil {
		return "", trace.Wrap(err)
	}

	envelope, err := packages.ReadPackageEnvelope(*locator)
	if err != nil {
		if !trace.IsNotFound(err) {
			return "", trace.Wrap(err)
		}
		tfVersion = ""
	} else {
		tfVersion = envelope.Locator.Version
	}

	return tfVersion, nil
}

/* telekubeGravityBinary returns latest gravity binary available on this cluster

   GET /telekube/gravity
*/
func (h *WebHandler) telekubeGravityBinary(w http.ResponseWriter, r *http.Request, p httprouter.Params) error {

	locator, err := loc.NewLocator(defaults.SystemAccountOrg, constants.GravityPackage, pack.LatestVersion)
	if err != nil {
		return trace.Wrap(err)
	}

	_, reader, err := h.Packages.ReadPackage(*locator)
	if err != nil {
		return trace.Wrap(err)
	}
	defer reader.Close()

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%v"`, constants.GravityPackage))

	seeker, ok := reader.(io.ReadSeeker)
	if ok {
		http.ServeContent(w, r, constants.GravityPackage, time.Now(), seeker)
		return nil
	}

	_, err = io.Copy(w, reader)
	return trace.Wrap(err)
}

/* telekubeBinary returns one of gravity, tele or tsh binaries of requested version

   GET /telekube/bin/:version/:os/:arch/:binary
*/
func (h *WebHandler) telekubeBinary(w http.ResponseWriter, r *http.Request, p httprouter.Params) error {
	ver := p.ByName("version")
	if ver == "" {
		return trace.BadParameter("'version' parameter is required")
	}
	if ver == constants.LatestVersion {
		ver = pack.LatestVersion
	}

	os := p.ByName("os")
	if os == "" {
		return trace.BadParameter("'os' parameter is required")
	}

	arch := p.ByName("arch")
	if arch == "" {
		return trace.BadParameter("'arch' parameter is required")
	}

	bin := p.ByName("binary")
	if bin == "" {
		return trace.BadParameter("'binary' parameter is required")
	}

	// telekube binaries have names which include their OS and architecture: tele_linux_x86_64
	name := strings.Join([]string{bin, os, arch}, "_")

	locator, err := loc.NewLocator(defaults.SystemAccountOrg, name, ver)
	if err != nil {
		return trace.Wrap(err)
	}

	_, reader, err := h.Packages.ReadPackage(*locator)
	if err != nil {
		return trace.Wrap(err)
	}
	defer reader.Close()

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%v"`, bin))

	seeker, ok := reader.(io.ReadSeeker)
	if ok {
		http.ServeContent(w, r, bin, time.Now(), seeker)
		return nil
	}

	_, err = io.Copy(w, reader)
	return trace.Wrap(err)
}

func (h *WebHandler) options(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	roundtrip.ReplyJSON(w, http.StatusOK, map[string]interface{}{"ok": "ok"})
}

func (h *WebHandler) wrap(fn func(w http.ResponseWriter, r *http.Request, p httprouter.Params) error) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		if err := fn(w, r, p); err != nil {
			log.WithFields(fields.FromRequest(r)).WithError(err).Info("Handler error.")
			trace.WriteError(w, trace.Unwrap(err))
		}
	}
}

func (h *WebHandler) needsAuth(fn serviceHandler) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
		logger := log.WithFields(fields.FromRequest(r))

		authResult, err := h.Authenticator.Authenticate(w, r)
		if err != nil {
			logger.WithError(err).Warn("Authentication error.")
			trace.WriteError(w, trace.Unwrap(trace.AccessDenied("bad username or password"))) // Hide the actual error.
			return
		}

		apps := app.ApplicationsWithACL(h.Applications, h.Users, authResult.User, authResult.Checker)
		context := &handlerContext{
			applications: apps,
			user:         authResult.User,
		}

		if err := fn(w, r, params, context); err != nil {
			if !trace.IsNotFound(err) && !trace.IsAlreadyExists(err) {
				logger.WithError(err).Error("Handler error.")
			} else {
				logger.WithError(err).Debug("Handler error.")
			}
			trace.WriteError(w, trace.Unwrap(err))
		}
	}
}

func importOperation(params httprouter.Params) (*storage.AppOperation, error) {
	operationID := params[0].Value
	return &storage.AppOperation{
		ID: operationID,
	}, nil
}

func appPackage(params httprouter.Params) (*loc.Locator, error) {
	repository := params[0].Value
	packageName := params[1].Value
	packageVersion := params[2].Value
	locator, err := loc.NewLocator(repository, packageName, packageVersion)

	if err != nil {
		return nil, trace.Wrap(err)
	}
	return locator, nil
}

type serviceHandler func(http.ResponseWriter, *http.Request, httprouter.Params, *handlerContext) error

type handlerContext struct {
	applications app.Applications
	user         storage.User
}

var (
	// telekubeInstallScriptTemplate is a bash script template to download and install telekube binaries
	telekubeInstallScriptTemplate = template.Must(template.New("telekube").Parse(`#!/bin/bash
#
# This script downloads the {{.version}} build of telekube
# and installs it on the target machine
#

OS=$(uname)
if [[ "$OS" == 'Linux' ]]; then
    OS='linux'
elif [[ "$OS" == 'Darwin' ]]; then
    OS='darwin'
else
    echo "Platform $OS is not supported"
    exit 1
fi

ARCH=$(uname -m)
if [[ ! $ARCH == 'x86_64' ]]; then
    echo "Architecture $ARCH is not supported"
    exit 1
fi

URL=https://get.gravitational.io/telekube/bin/{{.version}}/$OS/$ARCH

for BINARY in tele gravity tsh; do
    echo "Downloading $BINARY..."
    rm -f $BINARY
    curl -sOfL $URL/$BINARY
    if [ $? -ne 0 ]; then
        echo -e "Failed downloading $BINARY of version {{.version}}. Is the URL correct?\n$URL/$BINARY"
        exit 1
    fi
    chmod +x $BINARY
    sudo install -m 0755 $BINARY /usr/local/bin/$BINARY
    rm $BINARY

    echo "Done! Try running '$BINARY version'"
done

{{ if .tfVersion }}
mkdir -p ${HOME}/.terraform.d/plugins
TF_URL=https://get.gravitational.io/telekube/bin/{{.tfVersion}}/$OS/$ARCH
for BINARY in terraform-provider-gravity terraform-provider-gravityenterprise; do
    echo "Downloading $BINARY..."
    rm -f $BINARY
    curl -sOfL $TF_URL/$BINARY
    if [ $? -ne 0 ]; then
        echo -e "Failed downloading $BINARY of version {{.tfVersion}}. Is the URL correct?\n$URL/$BINARY"
        exit 1
    fi
    chmod +x $BINARY
    sudo install -m 0755 $BINARY ${HOME}/.terraform.d/plugins/$BINARY_{{.tfVersion}}
    rm $BINARY

    echo "Done! Terraform provider '$BINARY' is now available."
done
{{ end }}
`))
)
