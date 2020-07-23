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

package webapi

import (
	"net/http"

	"github.com/gravitational/gravity/lib/httplib"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/ops/opsservice"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"

	telehttplib "github.com/gravitational/teleport/lib/httplib"
	log "github.com/sirupsen/logrus"
)

// startOperation updates the state of the specified operation to reflect the UI progress
// and starts the operation.
//
// POST /portalapi/v1/sites/:domain/operations/:operation_id/start
//
// Input: ops.OperationUpdateRequest
//
// Output:
// {
//   "message": "OK"
// }
func (m *Handler) startOperation(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *AuthContext) (interface{}, error) {
	siteDomain, operationID := p[0].Value, p[1].Value
	cluster, err := context.Operator.GetSiteByDomain(siteDomain)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	operationKey := ops.SiteOperationKey{
		AccountID:   cluster.AccountID,
		SiteDomain:  cluster.Domain,
		OperationID: operationID,
	}
	operation, err := context.Operator.GetSiteOperation(operationKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var req ops.OperationUpdateRequest
	if err = telehttplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}
	log.Infof("startOperation: operation=%v, req=%v", operation, req)

	if operation.Provisioner == schema.ProvisionerOnPrem {
		req.ValidateServers = true
	}

	switch operation.Type {
	case ops.OperationInstall:
		err := context.Operator.UpdateInstallOperationState(operationKey, req)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		go func() {
			err = context.Operator.SiteInstallOperationStart(operationKey)
			if err != nil {
				log.Errorf("Operation %v (%v) failed: %v.", operation.Type,
					operationKey, trace.DebugReport(err))
			}
		}()
	case ops.OperationExpand:
		err := context.Operator.UpdateExpandOperationState(operationKey, req)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		go func() {
			err = context.Operator.SiteExpandOperationStart(operationKey)
			if err != nil {
				log.Errorf("Operation %v (%v) failed: %v.", operation.Type,
					operationKey, trace.DebugReport(err))
			}
		}()
	default:
		return nil, trace.BadParameter("failed to start %v", operation)
	}

	return httplib.OK(), nil
}

// validateServers runs pre-installation checks on the provided servers
//
// POST /portalapi/v1/sites/:domain/operations/:operation_id/prechecks
//
// Input: ops.ValidateServersRequest
//
// Output:
// {
//   "message": "OK"
// }
func (m *Handler) validateServers(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *AuthContext) (interface{}, error) {
	var req ops.ValidateServersRequest

	err := telehttplib.ReadJSON(r, &req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	log.Infof("validateServers: %v", req)

	clusterName, operationID := p.ByName("domain"), p.ByName("operation_id")
	err = opsservice.ValidateServers(ctx.Context, ctx.Operator, ops.ValidateServersRequest{
		AccountID:   ctx.User.GetAccountID(),
		SiteDomain:  clusterName,
		OperationID: operationID,
		Servers:     req.Servers,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return httplib.OK(), nil
}

// getOperations returns a list of operations that were executed for this site
//
// GET /portal/v1/sites/:domain/operations
//
// [{
//    "id": "1dbb12a2-5123-4385-aeb2-876c8dc76319",
//    "account_id": "92afb16b-5123-4385-aeb2-876c8dc76319",
//    "site_id": "5cbb1162-5123-4385-aeb2-876c8dc76319",
//    "type": "operation_install",
//    "created": "timestamp RFC 3339",
//    "updated": "timestamp RFC 3339",
//    "state": "install_initiated",
//    "servers": [],
//    "variables": {
//      "key": "operation specific variables"
//    }
// }]
func (m *Handler) getOperations(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *AuthContext) (interface{}, error) {
	siteDomain := p[0].Value
	siteKey := ops.SiteKey{AccountID: context.User.GetAccountID(), SiteDomain: siteDomain}
	operations, err := context.Operator.GetSiteOperations(siteKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return operations, nil
}

// deleteOperation removes an unstarted operation and places the site into active state
//
// DELETE /portalapi/v1/sites/:domain/operations/:operation_id
//
// Input:
//  operation_id - id of the pending operation
//
// Output:
// {
//   "message": "ok"
// }
func (m *Handler) deleteOperation(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *AuthContext) (interface{}, error) {
	siteDomain, operationID := p[0].Value, p[1].Value
	site, err := ctx.Operator.GetSiteByDomain(siteDomain)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	operationKey := ops.SiteOperationKey{AccountID: ctx.User.GetAccountID(), SiteDomain: site.Domain, OperationID: operationID}
	if err := ctx.Operator.DeleteSiteOperation(operationKey); err != nil {
		return nil, trace.Wrap(err)
	}
	return httplib.OK(), nil
}

func init() {
	operationStates[operationInstall] = operationProgress{
		{Step: 0, Message: "Provisioning Instances"},
		{Step: 1, Message: "Connecting to Instances"},
		{Step: 2, Message: "Verifying Instances"},
		{Step: 3, Message: "Preparing Configuration"},
		{Step: 4, Message: "Installing Dependencies"},
		{Step: 5, Message: "Installing Application"},
		{Step: 6, Message: "Verifying Installation"},
		{Step: 7, Message: "Launching Application"},
		{Step: 8, Message: "Connecting to Application"},
	}
}

type operationType byte

const (
	operationInstall operationType = iota
	operationUninstall
	operationExpand
	operationShrink
	operationUpdate
	operationTypeMax
)

type operationProgress [maxOperationSteps]progressStep

type progressStep struct {
	Step    int    `json:"step"`
	Message string `json:"message"`
}

const maxOperationSteps = 9

type operationStateMatrix [operationTypeMax]operationProgress

var operationStates operationStateMatrix

func (r operationType) String() string {
	out, err := r.MarshalText()
	switch err {
	case nil:
		return string(out)
	default:
		return "<unknown>"
	}
}

func (r *operationType) UnmarshalText(data []byte) error {
	switch string(data) {
	case ops.OperationInstall:
		*r = operationInstall
	case ops.OperationUninstall:
		*r = operationUninstall
	case ops.OperationExpand:
		*r = operationExpand
	case ops.OperationShrink:
		*r = operationShrink
	case ops.OperationUpdate:
		*r = operationUpdate
	default:
		return trace.BadParameter("invalid operation type: %s", data)
	}
	return nil
}

func (r operationType) MarshalText() ([]byte, error) {
	switch r {
	case operationInstall:
		return []byte(ops.OperationInstall), nil
	case operationUninstall:
		return []byte(ops.OperationUninstall), nil
	case operationExpand:
		return []byte(ops.OperationExpand), nil
	case operationShrink:
		return []byte(ops.OperationShrink), nil
	case operationUpdate:
		return []byte(ops.OperationUpdate), nil
	default:
		return nil, trace.BadParameter("invalid operation type: %s", r)
	}
}

func (r operationProgress) Titles() (titles []string) {
	titles = make([]string, 0, len(r))
	for _, step := range r {
		titles = append(titles, step.Message)
	}
	return titles
}
