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
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gravitational/gravity/lib/blob"
	"github.com/gravitational/gravity/lib/httplib"
	"github.com/gravitational/gravity/lib/users"

	"github.com/gravitational/form"
	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
)

// Config is a config for HTTP handler
type Config struct {
	// Users is identity and access management service
	Users users.Identity
	// Cluster implements cluster-level BLOB storage
	Cluster blob.Objects
	// Local implements local BLOB storage (used by peers to
	// exchange BLOBs)
	Local blob.Objects
}

// Server is HTTP server implementing BLOB storage over HTTP
type Server struct {
	httprouter.Router
	cfg Config
}

// New returns new instance of HTTP  BLOB server
func New(cfg Config) (*Server, error) {
	if cfg.Users == nil {
		return nil, trace.BadParameter("missing parameter Users")
	}
	if cfg.Cluster == nil {
		return nil, trace.BadParameter("missing parameter Cluster")
	}
	if cfg.Local == nil {
		return nil, trace.BadParameter("missing parameter Local")
	}
	h := &Server{
		cfg: cfg,
	}

	handlers := []struct {
		objects blob.Objects
		prefix  string
	}{
		{objects: cfg.Cluster, prefix: "/objects/v1/cluster"},
		{objects: cfg.Local, prefix: "/objects/v1/local"},
	}
	for _, handler := range handlers {
		h.GET(handler.prefix+"/blobs", h.needsAuth(h.getBLOBs, handler.objects))
		h.DELETE(handler.prefix+"/blobs/:hash", h.needsAuth(h.deleteBLOB, handler.objects))
		h.GET(handler.prefix+"/blobs/:hash", h.needsAuth(h.getBLOB, handler.objects))
		h.GET(handler.prefix+"/blobs/:hash/envelope", h.needsAuth(h.getBLOBEnvelope, handler.objects))
		h.HEAD(handler.prefix+"/blobs/:hash", h.needsAuth(h.getBLOB, handler.objects))
		h.POST(handler.prefix+"/blobs", h.needsAuth(h.createBLOB, handler.objects))
	}

	h.NotFound = h.notFound

	return h, nil
}

func (s *Server) notFound(w http.ResponseWriter, r *http.Request) {
	err := trace.NotFound("%v %v is not recognized", r.Method, r.URL.String())
	log.WithError(err).Info(err.Error())
	trace.WriteError(w, trace.Unwrap(err))
}

func (s *Server) getBLOBs(w http.ResponseWriter, r *http.Request, p httprouter.Params, objects blob.Objects) error {
	hashes, err := objects.GetBLOBs()
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, hashes)
	return nil
}

func (s *Server) getBLOBEnvelope(w http.ResponseWriter, r *http.Request, p httprouter.Params, objects blob.Objects) error {
	envelope, err := objects.GetBLOBEnvelope(p.ByName("hash"))
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, envelope)
	return nil
}

func (s *Server) deleteBLOB(w http.ResponseWriter, r *http.Request, p httprouter.Params, objects blob.Objects) error {
	if err := objects.DeleteBLOB(p.ByName("hash")); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (s *Server) getBLOB(w http.ResponseWriter, r *http.Request, p httprouter.Params, objects blob.Objects) error {
	hash := p.ByName("hash")
	fileObject, err := objects.OpenBLOB(hash)
	if err != nil {
		return trace.Wrap(err)
	}
	defer fileObject.Close()

	readSeeker, ok := fileObject.(io.ReadSeeker)
	if !ok {
		return trace.BadParameter("expected read seeker object")
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%v`, hash))
	http.ServeContent(w, r, hash, time.Now(), readSeeker)
	return nil
}

func (s *Server) createBLOB(w http.ResponseWriter, r *http.Request, p httprouter.Params, objects blob.Objects) error {
	var files form.Files

	err := form.Parse(r,
		form.FileSlice("file", &files),
	)
	if err != nil {
		return trace.Wrap(err)
	}

	if len(files) != 1 {
		return trace.BadParameter("expected a single file parameter but got %d", len(files))
	}

	defer func() {
		// don't let the error get lost
		if err := files.Close(); err != nil {
			log.Errorf("failed to close files: %v", trace.DebugReport(err))
		}
	}()

	envelope, err := objects.WriteBLOB(files[0])
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, envelope)
	return nil
}

func (s *Server) needsAuth(fn authHandle, objects blob.Objects) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		log.WithFields(log.Fields{
			"method": r.Method,
		}).Debugf(r.URL.Path)

		authCreds, err := httplib.ParseAuthHeaders(r)
		if err != nil {
			log.WithError(err).Info(err.Error())
			trace.WriteError(w, trace.Unwrap(err))
			return
		}

		user, checker, err := s.cfg.Users.AuthenticateUser(*authCreds)
		if err != nil {
			log.WithError(err).Info("authenticate error")
			// we hide the error from the remote user to avoid giving any hints
			trace.WriteError(
				w, trace.Unwrap(trace.AccessDenied("bad username or password")))
			return
		}

		acl := blob.WithPermissions(objects, s.cfg.Users, user.GetName(), checker)
		if err := fn(w, r, p, acl); err != nil {
			if !trace.IsNotFound(err) && !trace.IsAlreadyExists(err) {
				log.Errorf("handler error: %v", trace.DebugReport(err))
			}
			trace.WriteError(w, trace.Unwrap(err))
		}
	}
}

type authHandle func(
	http.ResponseWriter, *http.Request, httprouter.Params, blob.Objects) error
