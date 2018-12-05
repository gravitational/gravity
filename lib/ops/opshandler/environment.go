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
	"net/http"

	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/ops/opsclient"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/roundtrip"
	telehttplib "github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
)

/* getEnvironment returns the cluster environment

     GET /portal/v1/accounts/:account_id/sites/:site_domain/environment

   Success Response:

     storage.Environment
*/
func (h *WebHandler) getEnvironment(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	env, err := context.Operator.GetClusterEnvironment(siteKey(p))
	if err != nil {
		return trace.Wrap(err)
	}
	bytes, err := storage.MarshalEnvironment(env)
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, bytes)
	return nil
}

/* updateEnvironment updates the cluster environment

     PUT /portal/v1/accounts/:account_id/sites/:site_domain/environment

   Success Response:

     {
       "message": "environment updated"
     }
*/
func (h *WebHandler) updateEnvironment(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	var req opsclient.UpsertResourceRawReq
	if err := telehttplib.ReadJSON(r, &req); err != nil {
		return trace.Wrap(err)
	}
	env, err := storage.UnmarshalEnvironment(req.Resource)
	if err != nil {
		return trace.Wrap(err)
	}
	key := siteKey(p)
	err = context.Operator.UpdateClusterEnvironment(ops.UpdateClusterEnvironmentRequest{
		Key: key,
		Env: env,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, statusOK("environment updated"))
	return nil
}
