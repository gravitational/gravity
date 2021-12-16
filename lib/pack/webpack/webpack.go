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

	"github.com/gravitational/form"
	"github.com/gravitational/roundtrip"
	teleservices "github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
)

type Config struct {
	Packages      pack.PackageService
	Users         users.Identity
	Authenticator httplib.Authenticator
}

type Server struct {
	httprouter.Router
	cfg        Config
	fileServer http.Handler
}

func NewHandler(cfg Config) (*Server, error) {
	if cfg.Packages == nil {
		return nil, trace.BadParameter("missing parameter Packages")
	}
	if cfg.Users == nil {
		return nil, trace.BadParameter("missing parameter Users")
	}
	h := &Server{
		cfg: cfg,
	}

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
		log.WithFields(log.Fields{
			"method": r.Method,
		}).Debugf(r.URL.Path)

		authCreds, err := httplib.ParseAuthHeaders(r)
		if err != nil {
			trace.WriteError(w, err)
			return
		}
		cookie, err := r.Cookie("session")
		hasCookie := err == nil && cookie != nil && cookie.Value != ""
		var user storage.User
		var checker teleservices.AccessChecker
		if !hasCookie {
			user, checker, err = s.cfg.Users.AuthenticateUser(*authCreds)
			if err != nil {
				log.Debugf("authenticate error: %v", trace.DebugReport(err))
				// we hide the error from the remote user to avoid giving any hints
				trace.WriteError(
					w, trace.AccessDenied("bad username or password"))
				return
			}
		} else {
			if s.cfg.Authenticator == nil {
				log.Debugf("web sessions are not supported: %v", err)
				// we hide the error from the remote user to avoid giving any hints
				trace.WriteError(
					w, trace.AccessDenied("web sessions are not supported"))
				return
			}
			session, err := s.cfg.Authenticator(w, r, true)
			if err != nil {
				log.Debugf("authenticate error: %v", err)
				// we hide the error from the remote user to avoid giving any hints
				trace.WriteError(
					w, trace.AccessDenied("bad username or password"))
				return
			}
			user, err := s.cfg.Users.GetTelekubeUser(session.GetUser())
			if err != nil {
				log.Debugf("authenticate error: %v", err)
				// we hide the error from the remote user to avoid giving any hints
				trace.WriteError(
					w, trace.AccessDenied("bad username or password"))
				return
			}
			checker, err = s.cfg.Users.GetAccessChecker(user)
			if err != nil {
				log.Errorf("failed to fetch roles %v", trace.DebugReport(err))
				trace.WriteError(
					w, trace.BadParameter("internal server error"))
				return
			}
		}

		// create a ACL aware wrapper packages service
		// and pass it to the handlers, so every action will be automatically
		// checked against current user
		service := pack.PackagesWithACL(s.cfg.Packages, s.cfg.Users, user, checker)
		if err := fn(w, r, p, service); err != nil {
			if trace.IsAccessDenied(err) {
				log.Debugf("access denied: %v", err)
			} else if !trace.IsNotFound(err) && !trace.IsAlreadyExists(err) {
				log.Errorf("handler error: %v", trace.DebugReport(err))
			}
			trace.WriteError(w, err)
		}
	}
}

type authHandle func(
	http.ResponseWriter, *http.Request, httprouter.Params, pack.PackageService) error

type authContext struct {
	UserName  string
	AccountID string
	SiteID    string
}

type labels struct {
	AddLabels    map[string]string `json:"add_labels"`
	RemoveLabels []string          `json:"remove_labels"`
}
