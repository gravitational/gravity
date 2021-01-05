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

	"github.com/gravitational/gravity/lib/ops/opsclient"

	"github.com/gravitational/roundtrip"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/julienschmidt/httprouter"
)

/* upsertRole creates a new role or updates an existing one

   POST /portal/v1/accounts/:account_id/sites/:site_domain/roles
*/
func (h *WebHandler) upsertRole(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *handlerContext) error {
	var req opsclient.UpsertResourceRawReq
	if err := httplib.ReadJSON(r, &req); err != nil {
		return trace.Wrap(err)
	}
	role, err := services.GetRoleMarshaler().UnmarshalRole(req.Resource)
	if err != nil {
		return trace.Wrap(err)
	}
	if req.TTL != 0 {
		role.SetTTL(clockwork.NewRealClock(), req.TTL)
	}
	err = ctx.Identity.UpsertRole(role, req.TTL)
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, message("role upserted"))
	return nil
}

/* getRole returns a role by name

   GET /portal/v1/accounts/:account_id/sites/:site_domain/roles/:id
*/
func (h *WebHandler) getRole(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *handlerContext) error {
	role, err := ctx.Identity.GetRole(p.ByName("id"))
	if err != nil {
		return trace.Wrap(err)
	}
	out, err := services.GetRoleMarshaler().MarshalRole(role)
	return rawMessage(w, out, err)
}

/* getRoles returns all cluster roles

   GET /portal/v1/accounts/:account_id/sites/:site_domain/roles
*/
func (h *WebHandler) getRoles(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *handlerContext) error {
	roles, err := ctx.Identity.GetRoles()
	if err != nil {
		return trace.Wrap(err)
	}
	items := make([]json.RawMessage, len(roles))
	for i, role := range roles {
		data, err := services.GetRoleMarshaler().MarshalRole(role)
		if err != nil {
			return trace.Wrap(err)
		}
		items[i] = data
	}
	roundtrip.ReplyJSON(w, http.StatusOK, items)
	return nil
}

/* deleteRole deletes a role by name

   DELETE /portal/v1/accounts/:account_id/sites/:site_domain/roles/:id
*/
func (h *WebHandler) deleteRole(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *handlerContext) error {
	name := p.ByName("id")
	err := ctx.Identity.DeleteRole(name)
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("role %q not found", name)
		}
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, message("role deleted"))
	return nil
}
