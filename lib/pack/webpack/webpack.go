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

package webpack

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"

	"github.com/gravitational/gravity/lib/httplib"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/users"
	"github.com/gravitational/gravity/lib/utils"
	"github.com/gravitational/gravity/lib/utils/fields"

	"github.com/gravitational/form"
	"github.com/gravitational/roundtrip"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
)

// Config defines package service web handler configuration.
type Config struct {
	// Packages provides access to the package service.
	Packages pack.PackageService
	// Users provides access to the users service.
	Users users.Identity
	// Authenticator is used to authenticate requests.
	Authenticator users.Authenticator
}

// CheckAndSetDefaults validates the request and sets some defaults.
func (c *Config) CheckAndSetDefaults() error {
	if c.Packages == nil {
		return trace.BadParameter("missing parameter Packages")
	}
	if c.Users == nil {
		return trace.BadParameter("missing parameter Users")
	}
	if c.Authenticator == nil {
		c.Authenticator = users.NewAuthenticatorFromIdentity(c.Users)
	}
	return nil
}

type Server struct {
	httprouter.Router
	cfg        Config
	middleware *auth.AuthMiddleware
}

func NewHandler(cfg Config) (*Server, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	h := &Server{
		cfg: cfg,
	}

	// Wrap the router in the authentication middleware which will detect
	// if the client is trying to authenticate using a client certificate,
	// extract user information from it and add it to the request context.
	h.middleware = &auth.AuthMiddleware{
		AccessPoint: users.NewAccessPoint(cfg.Users),
	}
	h.middleware.Wrap(&h.Router)

	h.POST("/pack/v1/repositories", h.needsAuth(h.createRepository))
	h.DELETE("/pack/v1/repositories/:repository", h.needsAuth(h.deleteRepository))
	h.GET("/pack/v1/repositories", h.needsAuth(h.getRepositories))
	h.GET("/pack/v1/repositories/:repository", h.needsAuth(h.getRepository))
	h.POST("/pack/v1/repositories/:repository/packages", h.needsAuth(h.createPackage))
	h.GET("/pack/v1/repositories/:repository/packages", h.needsAuth(h.getPackages))
	h.GET("/pack/v1/repositories/:repository/packages/:package_name/:package_version/file", h.needsAuth(h.getPackageFile))
	h.HEAD("/pack/v1/repositories/:repository/packages/:package_name/:package_version/file", h.needsAuth(h.getPackageFile))
	h.GET("/pack/v1/repositories/:repository/packages/:package_name/:package_version/envelope", h.needsAuth(h.getPackageEnvelope))
	h.POST("/pack/v1/repositories/:repository/packages/:package_name/:package_version", h.needsAuth(h.updatePackageLabels))
	h.DELETE("/pack/v1/repositories/:repository/packages/:package_name/:package_version", h.needsAuth(h.deletePackage))

	return h, nil
}

// ServeHTTP lets the authentication middleware serve the request before
// passing it through to the router.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.middleware.ServeHTTP(w, r)
}

func (s *Server) createRepository(w http.ResponseWriter, r *http.Request, p httprouter.Params, service pack.PackageService) error {
	var repoName string
	var expires time.Time

	err := form.Parse(r,
		form.String("name", &repoName, form.Required()),
		form.Time("expires", &expires, form.Required()),
	)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := service.UpsertRepository(repoName, expires); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (s *Server) getRepository(w http.ResponseWriter, r *http.Request, p httprouter.Params, service pack.PackageService) error {
	repo, err := service.GetRepository(p.ByName("repository"))
	if err != nil {
		return trace.Wrap(err)
	}
	data, err := storage.MarshalRepository(repo)
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, json.RawMessage(data))
	return nil
}

func (s *Server) getRepositories(w http.ResponseWriter, r *http.Request, p httprouter.Params, service pack.PackageService) error {
	repositories, err := service.GetRepositories()
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, repositories)
	return nil
}

func (s *Server) deleteRepository(w http.ResponseWriter, r *http.Request, p httprouter.Params, service pack.PackageService) error {
	if err := service.DeleteRepository(p.ByName("repository")); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (s *Server) deletePackage(w http.ResponseWriter, r *http.Request, p httprouter.Params, service pack.PackageService) error {
	loc, err := loc.NewLocator(p.ByName("repository"), p.ByName("package_name"), p.ByName("package_version"))
	if err != nil {
		return trace.BadParameter(err.Error())
	}
	if err := service.DeletePackage(*loc); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (s *Server) getPackages(w http.ResponseWriter, r *http.Request, p httprouter.Params, service pack.PackageService) error {
	repoName := p.ByName("repository")
	log.Infof("getPackages(%v)", repoName)

	packages, err := service.GetPackages(repoName)

	if err != nil {
		return trace.Wrap(err)
	}

	roundtrip.ReplyJSON(w, http.StatusOK, packages)
	return nil
}

func (s *Server) getPackageEnvelope(w http.ResponseWriter, r *http.Request, p httprouter.Params, service pack.PackageService) error {
	loc, err := loc.NewLocator(p.ByName("repository"), p.ByName("package_name"), p.ByName("package_version"))
	if err != nil {
		return trace.BadParameter(err.Error())
	}
	envelope, err := service.ReadPackageEnvelope(*loc)
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, envelope)
	return nil
}

func (s *Server) getPackageFile(w http.ResponseWriter, r *http.Request, p httprouter.Params, service pack.PackageService) error {
	loc, err := loc.NewLocator(p.ByName("repository"), p.ByName("package_name"), p.ByName("package_version"))
	if err != nil {
		return trace.BadParameter(err.Error())
	}

	_, fileObject, err := service.ReadPackage(*loc)
	if err != nil {
		return trace.Wrap(err)
	}
	defer fileObject.Close()

	readSeeker, ok := fileObject.(io.ReadSeeker)
	if !ok {
		return trace.BadParameter("expected read seeker object")
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%v`, loc.String()))
	http.ServeContent(w, r, loc.String(), time.Now(), readSeeker)
	return nil
}

func (s *Server) createPackage(w http.ResponseWriter, r *http.Request, p httprouter.Params, service pack.PackageService) error {
	var files form.Files
	var labelsMap string
	var upsertS string
	var hiddenS string
	var packageType string
	var manifest string

	err := form.Parse(r,
		form.FileSlice("package", &files),
		form.String("labels", &labelsMap),
		form.String("upsert", &upsertS),
		form.String("hidden", &hiddenS),
		form.String("type", &packageType),
		form.String("manifest", &manifest),
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

	var upsert bool
	if upsertS != "" {
		upsert, err = strconv.ParseBool(upsertS)
		if err != nil {
			return trace.BadParameter("upsert should be either 'true' or 'false', got %v", upsertS)
		}
	}

	var hidden bool
	if hiddenS != "" {
		hidden, err = strconv.ParseBool(hiddenS)
		if err != nil {
			return trace.BadParameter("hidden should be either 'true' or 'false', got %v", hiddenS)
		}
	}

	defer func() {
		// don't let get the error get lost
		if err := files.Close(); err != nil {
			log.Errorf("failed to close files: %v", err)
		}
	}()

	packageName, err := utils.ParseFilename(r, "package")
	if err != nil {
		return trace.Wrap(err, "failed to parse package name")
	}
	loc, err := loc.ParseLocator(packageName)
	if err != nil {
		return trace.BadParameter(err.Error())
	}

	// configure package attributes
	opts := []pack.PackageOption{pack.WithLabels(labels), pack.WithHidden(hidden)}
	if manifest != "" {
		opts = append(opts, pack.WithManifest(packageType, []byte(manifest)))
	}

	var envelope *pack.PackageEnvelope
	if upsert {
		envelope, err = service.UpsertPackage(*loc, files[0], opts...)
	} else {
		envelope, err = service.CreatePackage(*loc, files[0], opts...)
	}

	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, envelope)
	return nil
}

func (s *Server) updatePackageLabels(w http.ResponseWriter, r *http.Request, p httprouter.Params, service pack.PackageService) error {
	loc, err := loc.NewLocator(p.ByName("repository"), p.ByName("package_name"), p.ByName("package_version"))
	if err != nil {
		return trace.BadParameter(err.Error())
	}
	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return trace.Wrap(err)
	}
	log.Infof("updatePackageLabels(%v)", string(data))
	var req labels
	if err := json.Unmarshal(data, &req); err != nil {
		return trace.BadParameter(err.Error())
	}
	err = service.UpdatePackageLabels(*loc, req.AddLabels, req.RemoveLabels)
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, map[string]string{"status": "ok", "message": "labels updated"})
	return nil
}

func (s *Server) needsAuth(fn authHandle) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		logger := log.WithFields(fields.FromRequest(r))

		authResult, err := s.cfg.Authenticator.Authenticate(w, r)
		if err != nil {
			logger.WithError(err).Warn("Authentication error.")
			httplib.WriteError(w, trace.AccessDenied("bad username or password"), r.Header) // Hide the actual error.
			return
		}

		// create a ACL aware wrapper packages service
		// and pass it to the handlers, so every action will be automatically
		// checked against current user
		service := pack.PackagesWithACL(s.cfg.Packages, s.cfg.Users, authResult.User, authResult.Checker)
		if err := fn(w, r, p, service); err != nil {
			if trace.IsAccessDenied(err) {
				logger.WithError(err).Warn("Access denied.")
			} else if !trace.IsNotFound(err) && !trace.IsAlreadyExists(err) {
				logger.WithError(err).Error("Handler error.")
			}
			httplib.WriteError(w, err, r.Header)
		}
	}
}

type authHandle func(
	http.ResponseWriter, *http.Request, httprouter.Params, pack.PackageService) error

type labels struct {
	AddLabels    map[string]string `json:"add_labels"`
	RemoveLabels []string          `json:"remove_labels"`
}
