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
	"net/http"

	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
)

/* createUpdateEnvarsOperation initiates the operatation of updating cluster environment variables

   POST /portal/v1/accounts/:account_id/sites/:site_domain/operations/envars

   {
      "account_id": "account id",
      "site_id": "site_id",
      "env": "<new enviornment>"
   }


Success response:

   {
      "account_id": "account id",
      "site_id": "site_id",
      "operation_id": "operation id"
   }
*/
func (h *WebHandler) createUpdateEnvarsOperation(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	d := json.NewDecoder(r.Body)
	var req ops.CreateUpdateEnvarsOperationRequest
	if err := d.Decode(&req); err != nil {
		return trace.BadParameter(err.Error())
	}
	req.SiteKey = siteKey(p)
	op, err := context.Operator.CreateUpdateEnvarsOperation(req)
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, op)
	return nil
}

/* getEnvironmentVariables fetches the cluster environment variables

     GET /portal/v1/accounts/:account_id/sites/:site_domain/envars

   Success Response:

     storage.Environment
*/
func (h *WebHandler) getEnvironmentVariables(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	env, err := context.Operator.GetClusterEnvironmentVariables(siteKey(p))
	if err != nil {
		return trace.Wrap(err)
	}
	bytes, err := storage.MarshalEnvironment(env)
	return trace.Wrap(rawMessage(w, bytes, err))
}
