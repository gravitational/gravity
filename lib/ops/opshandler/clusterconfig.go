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
	"github.com/gravitational/gravity/lib/storage/clusterconfig"

	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
)

/* updateClusterConfig updates the cluster configuration

   PUT /portal/v1/accounts/:account_id/sites/:site_domain/config

   {
      "account_id": "account id",
      "site_id": "site_id",
      "config": "<new configuration>"
   }

Success response:

   {
      "message": "cluster configuration updated",
   }
*/
func (h *WebHandler) updateClusterConfig(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	d := json.NewDecoder(r.Body)
	var req ops.UpdateClusterConfigRequest
	if err := d.Decode(&req); err != nil {
		return trace.BadParameter(err.Error())
	}
	req.ClusterKey = siteKey(p)
	err := context.Operator.UpdateClusterConfiguration(req)
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, statusOK("cluster configuration updated"))
	return nil
}

/* createUpdateConfigOperation initiates the operatation of updating cluster configuration

   POST /portal/v1/accounts/:account_id/sites/:site_domain/operations/config

   {
      "account_id": "account id",
      "site_id": "site_id",
      "config": "<new configuration>"
   }

Success response:

   {
      "account_id": "account id",
      "site_id": "site_id",
      "operation_id": "operation id"
   }
*/
func (h *WebHandler) createUpdateConfigOperation(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	d := json.NewDecoder(r.Body)
	var req ops.CreateUpdateConfigOperationRequest
	if err := d.Decode(&req); err != nil {
		return trace.BadParameter(err.Error())
	}
	req.ClusterKey = siteKey(p)
	op, err := context.Operator.CreateUpdateConfigOperation(r.Context(), req)
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, op)
	return nil
}

/* getClusterConfiguration fetches the cluster configuration

     GET /portal/v1/accounts/:account_id/sites/:site_domain/config

   Success Response:

     clusterconfig.Interface
*/
func (h *WebHandler) getClusterConfiguration(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	config, err := context.Operator.GetClusterConfiguration(siteKey(p))
	if err != nil {
		return trace.Wrap(err)
	}
	bytes, err := clusterconfig.Marshal(config)
	return trace.Wrap(rawMessage(w, bytes, err))
}
