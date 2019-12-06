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

package opshandler

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/ops/opsclient"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/roundtrip"
	telehttplib "github.com/gravitational/teleport/lib/httplib"
	teleservices "github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/julienschmidt/httprouter"
)

/* upsertUser creates or updates the user

     POST /portal/v1/accounts/:account_id/sites/:site_domain/users

   Success Response:

     {
       "message": "user upserted"
     }
*/
func (h *WebHandler) upsertUser(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *HandlerContext) error {
	var req opsclient.UpsertResourceRawReq
	if err := telehttplib.ReadJSON(r, &req); err != nil {
		return trace.Wrap(err)
	}
	user, err := teleservices.GetUserMarshaler().UnmarshalUser(req.Resource)
	if err != nil {
		return trace.Wrap(err)
	}
	if req.TTL != 0 {
		user.SetTTL(clockwork.NewRealClock(), req.TTL)
	}
	err = ctx.Operator.UpsertUser(r.Context(), siteKey(p), user)
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, message("user upserted"))
	return nil
}

func (h *WebHandler) getClusterAuthPreference(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *HandlerContext) error {
	cap, err := ctx.Identity.GetAuthPreference()
	if err != nil {
		return trace.Wrap(err)
	}
	out, err := teleservices.GetAuthPreferenceMarshaler().Marshal(cap)
	return rawMessage(w, out, err)
}

func (h *WebHandler) upsertClusterAuthPreference(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *HandlerContext) error {
	var req opsclient.UpsertResourceRawReq
	if err := telehttplib.ReadJSON(r, &req); err != nil {
		return trace.Wrap(err)
	}

	cap, err := teleservices.GetAuthPreferenceMarshaler().Unmarshal(req.Resource)
	if err != nil {
		return trace.Wrap(err)
	}

	err = ctx.Operator.UpsertClusterAuthPreference(r.Context(), siteKey(p), cap)
	if err != nil {
		return trace.Wrap(err)
	}

	roundtrip.ReplyJSON(w, http.StatusOK, message("cluster authentication preference upserted"))
	return nil
}

/* upsertAuthGateway updates auth gateway settings.

     POST /portal/v1/accounts/:account_id/sites/:site_domain/authgateway

   Success response:

     { "message": "auth gateway preferences updated" }
*/
func (h *WebHandler) upsertAuthGateway(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *HandlerContext) error {
	var req opsclient.UpsertResourceRawReq
	if err := telehttplib.ReadJSON(r, &req); err != nil {
		return trace.Wrap(err)
	}
	gw, err := storage.UnmarshalAuthGateway(req.Resource)
	if err != nil {
		return trace.Wrap(err)
	}
	err = ctx.Operator.UpsertAuthGateway(r.Context(), siteKey(p), gw)
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, message("auth gateway preferences updated"))
	return nil
}

/* getAuthGateway returns the cluster auth gateway settings.

     GET /portal/v1/accounts/:account_id/sites/:site_domain/authgateway

   Success response:

     storage.AuthGateway
*/
func (h *WebHandler) getAuthGateway(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *HandlerContext) error {
	gw, err := ctx.Operator.GetAuthGateway(siteKey(p))
	if err != nil {
		return trace.Wrap(err)
	}
	bytes, err := storage.MarshalAuthGateway(gw)
	return rawMessage(w, bytes, err)
}

/* getReleases returns all currently installed application releases in a cluster.

     GET /portal/v1/accounts/:account_id/sites/:site_domain/releases

   Success response:

     []storage.Release
*/
func (h *WebHandler) getReleases(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *HandlerContext) error {
	err := r.ParseForm()
	if err != nil {
		return trace.Wrap(err)
	}
	includeIcons, err := utils.ParseBoolFlag(r, "include_icons", false)
	if err != nil {
		return trace.Wrap(err)
	}
	releases, err := ctx.Operator.ListReleases(ops.ListReleasesRequest{
		SiteKey:      siteKey(p),
		IncludeIcons: includeIcons,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	items := make([]json.RawMessage, 0, len(releases))
	for _, release := range releases {
		bytes, err := storage.MarshalRelease(release)
		if err != nil {
			return trace.Wrap(err)
		}
		items = append(items, bytes)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, items)
	return nil
}

/* getUser returns user by name

     GET /portal/v1/accounts/:account_id/sites/:site_domain/users/:name

   Success Response:

     teleservices.Role
*/
func (h *WebHandler) getUser(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *HandlerContext) error {
	user, err := ctx.Operator.GetUser(siteKey(p), p.ByName("name"))
	if err != nil {
		return trace.Wrap(err)
	}
	out, err := teleservices.GetUserMarshaler().MarshalUser(user)
	return rawMessage(w, out, err)
}

/* getUsers returns all users

     GET /portal/v1/accounts/:account_id/sites/:site_domain/users

   Success Response:

     []teleservices.User
*/
func (h *WebHandler) getUsers(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *HandlerContext) error {
	users, err := ctx.Operator.GetUsers(siteKey(p))
	if err != nil {
		return trace.Wrap(err)
	}
	items := make([]json.RawMessage, len(users))
	for i, user := range users {
		data, err := teleservices.GetUserMarshaler().MarshalUser(user)
		if err != nil {
			return trace.Wrap(err)
		}
		items[i] = data
	}
	roundtrip.ReplyJSON(w, http.StatusOK, items)
	return nil
}

/* deleteRole deletes user by name

     DELETE /portal/v1/accounts/:account_id/sites/:site_domain/users/:name

   Success Response:

     {
       "message": "user deleted"
     }
*/
func (h *WebHandler) deleteUser(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *HandlerContext) error {
	err := ctx.Operator.DeleteUser(r.Context(), siteKey(p), p.ByName("name"))
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, message("user deleted"))
	return nil
}

/* upsertGithubConnector creates or updates a Github connector

   POST /portal/v1/accounts/:account_id/sites/:site_domain/github/connectors
*/
func (h *WebHandler) upsertGithubConnector(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *HandlerContext) error {
	var req *opsclient.UpsertResourceRawReq
	if err := telehttplib.ReadJSON(r, &req); err != nil {
		return trace.Wrap(err)
	}
	connector, err := teleservices.GetGithubConnectorMarshaler().Unmarshal(req.Resource)
	if err != nil {
		return trace.Wrap(err)
	}
	if req.TTL != 0 {
		connector.SetTTL(clockwork.NewRealClock(), req.TTL)
	}
	err = ctx.Operator.UpsertGithubConnector(r.Context(), siteKey(p), connector)
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, message("upserted Github connector"))
	return nil
}

/* getGithubConnector returns a Github connector by name

   GET /portal/v1/accounts/:account_id/sites/:site_domain/github/connectors/:id
*/
func (h *WebHandler) getGithubConnector(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *HandlerContext) error {
	withSecrets, _, err := telehttplib.ParseBool(r.URL.Query(), constants.WithSecretsParam)
	if err != nil {
		return trace.Wrap(err)
	}
	connector, err := ctx.Operator.GetGithubConnector(siteKey(p), p.ByName("id"), withSecrets)
	if err != nil {
		return trace.Wrap(err)
	}
	out, err := teleservices.GetGithubConnectorMarshaler().Marshal(connector)
	return rawMessage(w, out, err)
}

/* getGithubConnectors returns all Github connectors

   GET /portal/v1/accounts/:account_id/sites/:site_domain/github/connectors
*/
func (h *WebHandler) getGithubConnectors(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *HandlerContext) error {
	withSecrets, _, err := telehttplib.ParseBool(r.URL.Query(), constants.WithSecretsParam)
	if err != nil {
		return trace.Wrap(err)
	}
	connectors, err := ctx.Operator.GetGithubConnectors(siteKey(p), withSecrets)
	if err != nil {
		return trace.Wrap(err)
	}
	items := make([]json.RawMessage, len(connectors))
	for i, connector := range connectors {
		data, err := teleservices.GetGithubConnectorMarshaler().Marshal(connector)
		if err != nil {
			return trace.Wrap(err)
		}
		items[i] = data
	}
	roundtrip.ReplyJSON(w, http.StatusOK, items)
	return nil
}

/* deleteGithubConnector deletes a connector by its name

   DELETE /portal/v1/accounts/:account_id/sites/:site_domain/github/connectors/:id
*/
func (h *WebHandler) deleteGithubConnector(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *HandlerContext) error {
	name := p.ByName("id")
	err := ctx.Operator.DeleteGithubConnector(r.Context(), siteKey(p), name)
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("GitHub connector %q not found", name)
		}
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, message("Github connector deleted"))
	return nil
}

/* getPersistentStorage retrieves cluster persistent storage configuration.

     GET /portal/v1/accounts/:account_id/sites/:site_domain/persistentstorage

   Success response:

     storage.PersistentStorage
*/
func (h *WebHandler) getPersistentStorage(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	ps, err := context.Operator.GetPersistentStorage(r.Context(), siteKey(p))
	if err != nil {
		return trace.Wrap(err)
	}
	bytes, err := storage.MarshalPersistentStorage(ps)
	return rawMessage(w, bytes, err)
}

/* updatePersistentStorage updates persistent storage configuration.

     PUT /portal/v1/accounts/:account_id/sites/:site_domain/persistentstorage

   Success response:

     { "message": "persistent storage configuration updated" }
*/
func (h *WebHandler) updatePersistentStorage(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *HandlerContext) error {
	var req opsclient.UpsertResourceRawReq
	if err := telehttplib.ReadJSON(r, &req); err != nil {
		return trace.Wrap(err)
	}
	ps, err := storage.UnmarshalPersistentStorage(req.Resource)
	if err != nil {
		return trace.Wrap(err)
	}
	err = ctx.Operator.UpdatePersistentStorage(r.Context(), ops.UpdatePersistentStorageRequest{
		SiteKey:  siteKey(p),
		Resource: ps,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, message("persistent storage configuration updated"))
	return nil
}

func rawMessage(w http.ResponseWriter, data []byte, err error) error {
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = w.Write(json.RawMessage(data))
	return err
}

func message(msg string, args ...interface{}) map[string]interface{} {
	return map[string]interface{}{"message": fmt.Sprintf(msg, args...)}
}
