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

package handler

import (
	"encoding/json"
	"net/http"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/ops/opsclient"

	"github.com/gravitational/roundtrip"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/julienschmidt/httprouter"
)

/* upsertOIDCConnector creates or updates OIDC connector

   POST /portal/v1/accounts/:account_id/sites/:site_domain/oidc/connectors
*/
func (h *WebHandler) upsertOIDCConnector(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *handlerContext) error {
	var req *opsclient.UpsertResourceRawReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return trace.Wrap(err)
	}
	connector, err := services.GetOIDCConnectorMarshaler().UnmarshalOIDCConnector(req.Resource)
	if err != nil {
		return trace.Wrap(err)
	}
	if req.TTL != 0 {
		connector.SetTTL(clockwork.NewRealClock(), req.TTL)
	}
	err = ctx.Identity.UpsertOIDCConnector(connector)
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, message("connector configuration applied"))
	return nil
}

/* getOIDCConnector returns an OIDC connector by name

   GET /portal/v1/accounts/:account_id/sites/:site_domain/oidc/connectors/:id
*/
func (h *WebHandler) getOIDCConnector(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *handlerContext) error {
	withSecrets, _, err := httplib.ParseBool(r.URL.Query(), constants.WithSecretsParam)
	if err != nil {
		return trace.Wrap(err)
	}
	connector, err := ctx.Identity.GetOIDCConnector(p.ByName("id"), withSecrets)
	if err != nil {
		return trace.Wrap(err)
	}
	out, err := services.GetOIDCConnectorMarshaler().MarshalOIDCConnector(connector)
	return rawMessage(w, out, err)
}

/* getOIDCConnectors returns all OIDC connectors

   GET /portal/v1/accounts/:account_id/sites/:site_domain/oidc/connectors
*/
func (h *WebHandler) getOIDCConnectors(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *handlerContext) error {
	withSecrets, _, err := httplib.ParseBool(r.URL.Query(), constants.WithSecretsParam)
	if err != nil {
		return trace.Wrap(err)
	}
	connectors, err := ctx.Identity.GetOIDCConnectors(withSecrets)
	if err != nil {
		return trace.Wrap(err)
	}
	items := make([]json.RawMessage, len(connectors))
	for i, connector := range connectors {
		data, err := services.GetOIDCConnectorMarshaler().MarshalOIDCConnector(connector)
		if err != nil {
			return trace.Wrap(err)
		}
		items[i] = data
	}
	roundtrip.ReplyJSON(w, http.StatusOK, items)
	return nil
}

/* deleteOIDCConnector deletes an OIDC connector by name

   DELETE /portal/v1/accounts/:account_id/sites/:site_domain/oidc/connectors/:id
*/
func (h *WebHandler) deleteOIDCConnector(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *handlerContext) error {
	name := p.ByName("id")
	err := ctx.Identity.DeleteOIDCConnector(name)
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("OIDC connector %q not found", name)
		}
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, message("OIDC connector deleted"))
	return nil
}
