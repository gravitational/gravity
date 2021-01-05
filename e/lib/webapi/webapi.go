// Copyright 2021 Gravitational Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package webapi

import (
	"net/http"

	"github.com/gravitational/gravity/e/lib/ops"
	"github.com/gravitational/gravity/e/lib/ops/acl"
	"github.com/gravitational/gravity/e/lib/ops/resources/gravity"
	"github.com/gravitational/gravity/lib/ops/resources"
	ossgravity "github.com/gravitational/gravity/lib/ops/resources/gravity"
	"github.com/gravitational/gravity/lib/webapi"

	telehttplib "github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
)

// Handler extends the open-source web API handler
type Handler struct {
	// Handler is the open-source web API handler
	*webapi.Handler
	// Operator is the enterprise operator service
	Operator ops.Operator
}

// NewHandler returns a new enterprise web API handler
func NewHandler(ossHandler *webapi.Handler, operator ops.Operator) *Handler {
	h := &Handler{Handler: ossHandler, Operator: operator}

	// OAuth2 related API
	h.GET("/oidc/callback", telehttplib.MakeHandler(h.oidcCallback))
	h.POST("/saml/callback", telehttplib.MakeHandler(h.samlCallback))
	h.POST("/oidc/login/console", telehttplib.MakeHandler(h.loginConsole))

	// Remote access API
	h.GET("/sites/:domain/access", h.needsAuth(h.getRemoteAccess))
	h.PUT("/sites/:domain/access", h.needsAuth(h.updateRemoteAccess))

	// License API
	h.PUT("/sites/:domain/license", h.needsAuth(h.updateLicense))
	h.POST("/license", h.needsAuth(h.newLicense))
	h.POST("/license/validate", h.needsAuth(h.validateLicense))

	h.SetPlugin(h)
	return h
}

type webAPIHandler func(http.ResponseWriter, *http.Request, httprouter.Params, *authContext) (interface{}, error)

type authContext struct {
	// AuthContext is the wrapped open-source context
	*webapi.AuthContext
	// Operator is the enterprise ACL operator
	Operator *acl.OperatorACL
}

func (h *Handler) needsAuth(fn webAPIHandler) httprouter.Handle {
	return telehttplib.MakeHandler(func(w http.ResponseWriter, r *http.Request, params httprouter.Params) (interface{}, error) {
		ossContext, err := h.GetHandlerContext(w, r)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		// wrap enterprise operator in acl as well
		context := &authContext{
			AuthContext: ossContext,
			Operator:    acl.OperatorWithACL(ossContext.Operator, h.Operator),
		}
		result, err := fn(w, r.WithContext(context.Context), params, context)
		h.Debugf("%v %v %v", r.Method, r.URL.String(), err)
		return result, trace.Wrap(err)
	})
}

// Resources returns the resource controller
func (h *Handler) Resources(ctx *webapi.AuthContext) (resources.Resources, error) {
	ossResources, err := ossgravity.New(ossgravity.Config{
		Operator: ctx.Operator,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return gravity.New(gravity.Config{
		Resources: ossResources,
		Operator:  acl.OperatorWithACL(ctx.Operator, h.Operator),
	})
}
