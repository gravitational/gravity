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
	"github.com/gravitational/gravity/lib/ops/opsclient"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/roundtrip"
	telehttplib "github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/julienschmidt/httprouter"
)

/* getRetentionPolicies returns a list of retention policies for a site

     GET /portal/v1/accounts/:account_id/sites/:site_domain/monitoring/retention

   Success Response:

     []ops.RetentionPolicy
*/
func (h *WebHandler) getRetentionPolicies(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	policies, err := context.Operator.GetRetentionPolicies(siteKey(p))
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, policies)
	return nil
}

/* updateRetentionPolicy updates retention policies for a site

     PUT /portal/v1/accounts/:account_id/sites/:site_domain/monitoring/retention

   Success Response:

     {
       "message": "retention policy updated"
     }
*/
func (h *WebHandler) updateRetentionPolicy(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	var req ops.UpdateRetentionPolicyRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return trace.Wrap(err)
	}
	err = context.Operator.UpdateRetentionPolicy(req)
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, statusOK("retention policy updated"))
	return nil
}

/* getAlerts returns a list of monitoring alerts for the cluster

     GET /portal/v1/accounts/:account_id/sites/:site_domain/monitoring/alerts

   Success Response:

     []storage.Alert
*/
func (h *WebHandler) getAlerts(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	alerts, err := context.Operator.GetAlerts(siteKey(p))
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, alerts)
	return nil
}

/* updateAlert updates the specified monitoring alert

PUT /portal/v1/accounts/:account_id/sites/:site_domain/monitoring/alerts/:name

   Success Response:

     {
       "message": "alert updated"
     }
*/
func (h *WebHandler) updateAlert(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	var req opsclient.UpsertResourceRawReq
	if err := telehttplib.ReadJSON(r, &req); err != nil {
		return trace.Wrap(err)
	}

	alert, err := storage.UnmarshalAlert(req.Resource)
	if err != nil {
		return trace.Wrap(err)
	}
	if req.TTL != 0 {
		alert.SetTTL(clockwork.NewRealClock(), req.TTL)
	}

	err = context.Operator.UpdateAlert(r.Context(), siteKey(p), alert)
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, statusOK("alert updated"))
	return nil
}

/* deleteAlert deletes a monitoring alert

   DELETE /portal/v1/accounts/:account_id/sites/:site_domain/monitoring/alerts/:name

   Success Response:

     {
       "message": "alert deleted"
     }
*/
func (h *WebHandler) deleteAlert(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	err := context.Operator.DeleteAlert(r.Context(), siteKey(p), p.ByName("name"))
	if err != nil {
		return trace.Wrap(err)
	}

	roundtrip.ReplyJSON(w, http.StatusOK, statusOK("alert deleted"))
	return nil
}

/* getAlertTargets returns a list of monitoring alert targets for the cluster

     GET /portal/v1/accounts/:account_id/sites/:site_domain/monitoring/alert-targets

   Success Response:

     []storage.AlertTarget
*/
func (h *WebHandler) getAlertTargets(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	targets, err := context.Operator.GetAlertTargets(siteKey(p))
	if err != nil {
		return trace.Wrap(err)
	}

	roundtrip.ReplyJSON(w, http.StatusOK, targets)
	return nil
}

/* updateAlertTarget updates cluster monitoring alert target

     PUT /portal/v1/accounts/:account_id/sites/:site_domain/monitoring/alert-targets

   Success Response:

     {
       "message": "alert target updated"
     }
*/
func (h *WebHandler) updateAlertTarget(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	var req opsclient.UpsertResourceRawReq
	if err := telehttplib.ReadJSON(r, &req); err != nil {
		return trace.Wrap(err)
	}

	target, err := storage.UnmarshalAlertTarget(req.Resource)
	if err != nil {
		return trace.Wrap(err)
	}
	if req.TTL != 0 {
		target.SetTTL(clockwork.NewRealClock(), req.TTL)
	}

	err = context.Operator.UpdateAlertTarget(r.Context(), siteKey(p), target)
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, statusOK("alert target updated"))
	return nil
}

/* deleteAlertTarget deletes cluster's monitoring alert target

   DELETE /portal/v1/accounts/:account_id/sites/:site_domain/monitoring/alert-targets

   Success Response:

     {
       "message": "alert target deleted"
     }
*/
func (h *WebHandler) deleteAlertTarget(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *HandlerContext) error {
	err := context.Operator.DeleteAlertTarget(r.Context(), siteKey(p))
	if err != nil {
		return trace.Wrap(err)
	}

	roundtrip.ReplyJSON(w, http.StatusOK, statusOK("alert target deleted"))
	return nil
}
