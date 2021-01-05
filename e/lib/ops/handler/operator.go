package handler

import (
	"encoding/json"
	"net/http"

	"github.com/gravitational/gravity/e/lib/ops"
	"github.com/gravitational/gravity/e/lib/ops/acl"
	ossops "github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/ops/opsclient"
	"github.com/gravitational/gravity/lib/ops/opshandler"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/roundtrip"
	telehttplib "github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	"github.com/sirupsen/logrus"
)

// WebHandler extends the open-source operator web handler
type WebHandler struct {
	// WebHandler is the wrapped open-source operator web handler
	*opshandler.WebHandler
	// Operator is the enterprise operator service
	Operator ops.Operator
}

// NewWebHandler returns extended enterprise ops handler
func NewWebHandler(ossHandler *opshandler.WebHandler, operator ops.Operator) *WebHandler {
	h := &WebHandler{WebHandler: ossHandler, Operator: operator}

	// Ops Center install related API
	h.PUT("/portal/v1/accounts/:account_id/sites/:site_domain/operations/common/:operation_id/register",
		h.needsAuth(h.registerAgent))
	h.POST("/portal/v1/accounts/:account_id/sites/:site_domain/operations/install/:operation_id/copy-cluster",
		h.needsAuth(h.requestClusterCopy))

	// Ops Center endpoints API
	h.GET("/portal/v1/accounts/:account_id/sites/:site_domain/cluster-endpoints",
		h.needsAuth(h.getClusterEndpoints))
	h.PUT("/portal/v1/accounts/:account_id/sites/:site_domain/cluster-endpoints",
		h.needsAuth(h.updateClusterEndpoints))

	// Updates & periodic updates API
	h.GET("/portal/v1/accounts/:account_id/sites/:site_domain/updates",
		h.needsAuth(h.checkForUpdate))
	h.POST("/portal/v1/accounts/:account_id/sites/:site_domain/updates",
		h.needsAuth(h.downloadUpdate))
	h.GET("/portal/v1/accounts/:account_id/sites/:site_domain/periodicupdates",
		h.needsAuth(h.periodicUpdatesStatus))
	h.POST("/portal/v1/accounts/:account_id/sites/:site_domain/periodicupdates/enable",
		h.needsAuth(h.periodicUpdatesEnable))
	h.POST("/portal/v1/accounts/:account_id/sites/:site_domain/periodicupdates/disable",
		h.needsAuth(h.periodicUpdatesDisable))
	h.POST("/portal/v1/accounts/:account_id/sites/:site_domain/periodicupdates/start",
		h.needsAuth(h.periodicUpdatesStart))
	h.POST("/portal/v1/accounts/:account_id/sites/:site_domain/periodicupdates/stop",
		h.needsAuth(h.periodicUpdatesStop))

	// Role management API
	h.POST("/portal/v1/accounts/:account_id/sites/:site_domain/roles",
		h.needsAuth(h.upsertRole))
	h.GET("/portal/v1/accounts/:account_id/sites/:site_domain/roles/:id",
		h.needsAuth(h.getRole))
	h.GET("/portal/v1/accounts/:account_id/sites/:site_domain/roles",
		h.needsAuth(h.getRoles))
	h.DELETE("/portal/v1/accounts/:account_id/sites/:site_domain/roles/:id",
		h.needsAuth(h.deleteRole))

	// OIDC connectors API
	h.POST("/portal/v1/accounts/:account_id/sites/:site_domain/oidc/connectors",
		h.needsAuth(h.upsertOIDCConnector))
	h.GET("/portal/v1/accounts/:account_id/sites/:site_domain/oidc/connectors",
		h.needsAuth(h.getOIDCConnectors))
	h.GET("/portal/v1/accounts/:account_id/sites/:site_domain/oidc/connectors/:id",
		h.needsAuth(h.getOIDCConnector))
	h.DELETE("/portal/v1/accounts/:account_id/sites/:site_domain/oidc/connectors/:id",
		h.needsAuth(h.deleteOIDCConnector))

	// SAML connectors API
	h.POST("/portal/v1/accounts/:account_id/sites/:site_domain/saml/connectors",
		h.needsAuth(h.upsertSAMLConnector))
	h.GET("/portal/v1/accounts/:account_id/sites/:site_domain/saml/connectors",
		h.needsAuth(h.getSAMLConnectors))
	h.GET("/portal/v1/accounts/:account_id/sites/:site_domain/saml/connectors/:id",
		h.needsAuth(h.getSAMLConnector))
	h.DELETE("/portal/v1/accounts/:account_id/sites/:site_domain/saml/connectors/:id",
		h.needsAuth(h.deleteSAMLConnector))

	// Trusted clusters API
	h.POST("/portal/v1/accounts/:account_id/sites/:site_domain/trustedclusters",
		h.needsAuth(h.upsertTrustedCluster))
	h.GET("/portal/v1/accounts/:account_id/sites/:site_domain/trustedclusters/:name",
		h.needsAuth(h.getTrustedCluster))
	h.GET("/portal/v1/accounts/:account_id/sites/:site_domain/trustedclusters",
		h.needsAuth(h.getTrustedClusters))
	h.DELETE("/portal/v1/accounts/:account_id/sites/:site_domain/trustedclusters/:name",
		h.needsAuth(h.deleteTrustedCluster))

	// Remote support API
	h.PUT("/portal/v1/accounts/:account_id/sites/:site_domain/accept",
		h.needsAuth(h.acceptRemoteCluster))
	h.PUT("/portal/v1/accounts/:account_id/sites/:site_domain/remove",
		h.needsAuth(h.removeRemoteCluster))

	// License API
	h.POST("/portal/v1/license/new", h.needsAuth(h.newLicense))
	h.GET("/portal/v1/license/ca", h.needsAuth(h.getLicenseCA))
	h.POST("/portal/v1/accounts/:account_id/sites/:site_domain/license/check",
		h.needsAuth(h.checkSiteLicense))
	h.POST("/portal/v1/accounts/:account_id/sites/:site_domain/license",
		h.needsAuth(h.updateLicense))

	return h
}

/* registerAgent registers install agent

   PUT /portal/v1/accounts/:account_id/sites/:site_domain/operations/common/:operation_id/register

   Success response: ops.RegisterAgentResponse
*/
func (h *WebHandler) registerAgent(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *handlerContext) error {
	var req ops.RegisterAgentRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return trace.Wrap(err)
	}
	response, err := ctx.Operator.RegisterAgent(req)
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, response)
	return nil
}

/* requestClusterCopy is used by the installer process to retrieve created
   cluster and operation when installing via Ops Center and recreate them
   in its own database

   POST /portal/v1/accounts/:account_id/sites/:site_domain/operations/install/:operation_id/copy-cluster

   Success response: {"status": "ok"}
*/
func (h *WebHandler) requestClusterCopy(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *handlerContext) error {
	var req ops.ClusterCopyRequest
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		return trace.Wrap(err)
	}
	err = ctx.Operator.RequestClusterCopy(req)
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, statusOK("ok"))
	return nil
}

/* getClusterEndpoints returns cluster management endpoints

     GET /portal/v1/accounts/:account_id/sites/:site_domain/cluster-endpoints

   Success Response:

     storage.Endpoints
*/
func (h *WebHandler) getClusterEndpoints(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *handlerContext) error {
	endpoints, err := ctx.Operator.GetClusterEndpoints(siteKey(p))
	if err != nil {
		return trace.Wrap(err)
	}
	bytes, err := storage.MarshalEndpoints(endpoints)
	if err != nil {
		return trace.Wrap(err)
	}
	return rawMessage(w, bytes, err)
}

/* updateClusterEndpoints updates cluster management endpoints

     PUT /portal/v1/accounts/:account_id/sites/:site_domain/cluster-endpoints

   Success Response:

     {"message": "cluster endpoints updated"}
*/
func (h *WebHandler) updateClusterEndpoints(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *handlerContext) error {
	var req *opsclient.UpsertResourceRawReq
	err := telehttplib.ReadJSON(r, &req)
	if err != nil {
		return trace.Wrap(err)
	}
	endpoints, err := storage.UnmarshalEndpoints(req.Resource)
	if err != nil {
		return trace.Wrap(err)
	}
	err = ctx.Operator.UpdateClusterEndpoints(siteKey(p), endpoints)
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, message("cluster endpoints updated"))
	return nil
}

/* checkForUpdate checks if a newer version available

   GET /portal/v1/accounts/:account_id/sites/:site_domain/updates
*/
func (h *WebHandler) checkForUpdate(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *handlerContext) error {
	update, err := ctx.Operator.CheckForUpdate(siteKey(p))
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, update)
	return nil
}

/* downloadUpdate downloads a new version to site

   POST /portal/v1/accounts/:account_id/sites/:site_domain/updates

   Input: ops.DownloadUpdateRequest
*/
func (h *WebHandler) downloadUpdate(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *handlerContext) error {
	d := json.NewDecoder(r.Body)
	var req ops.DownloadUpdateRequest
	if err := d.Decode(&req); err != nil {
		return trace.BadParameter(err.Error())
	}
	err := ctx.Operator.DownloadUpdate(req)
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, statusOK("ok"))
	return nil
}

/* periodicUpdatesEnable turns periodic updates on or updates the interval

   POST /portal/v1/accounts/:account_id/sites/:site_domain/periodicupdates/enable

   Input: ops.EnablePeriodicUpdatesRequest
*/
func (h *WebHandler) periodicUpdatesEnable(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *handlerContext) error {
	d := json.NewDecoder(r.Body)
	var req ops.EnablePeriodicUpdatesRequest
	if err := d.Decode(&req); err != nil {
		return trace.BadParameter(err.Error())
	}
	err := ctx.Operator.EnablePeriodicUpdates(req)
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, statusOK("ok"))
	return nil
}

/* periodicUpdatesDisable turns periodic updates off

   POST /portal/v1/accounts/:account_id/sites/:site_domain/periodicupdates/disable

   Input: ops.SiteKey
*/
func (h *WebHandler) periodicUpdatesDisable(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *handlerContext) error {
	err := ctx.Operator.DisablePeriodicUpdates(siteKey(p))
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, statusOK("ok"))
	return nil
}

/* periodicUpdatesStart starts periodic updates if they are enabled

   POST /portal/v1/accounts/:account_id/sites/:site_domain/periodicupdates/start

   Input: ops.SiteKey
*/
func (h *WebHandler) periodicUpdatesStart(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *handlerContext) error {
	err := ctx.Operator.StartPeriodicUpdates(siteKey(p))
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, statusOK("ok"))
	return nil
}

/* periodicUpdatesStop stops periodic updates without disabling them

   POST /portal/v1/accounts/:account_id/sites/:site_domain/periodicupdates/stop

   Input: ops.SiteKey
*/
func (h *WebHandler) periodicUpdatesStop(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *handlerContext) error {
	err := ctx.Operator.StopPeriodicUpdates(siteKey(p))
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, statusOK("ok"))
	return nil
}

/* periodicUpdatesStatus returns the status of periodic updates

   GET /portal/v1/accounts/:account_id/sites/:site_domain/periodicupdates/status

   Input:  ops.SiteKey
   Output: ops.PeriodicUpdateStatusResponse
*/
func (h *WebHandler) periodicUpdatesStatus(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *handlerContext) error {
	status, err := ctx.Operator.PeriodicUpdatesStatus(siteKey(p))
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, status)
	return nil
}

/*  acceptRemoteCluster accepts a request for a new cluster

    PUT /portal/v1/accounts/:account_id/sites/:site_domain/accept
*/
func (h *WebHandler) acceptRemoteCluster(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *handlerContext) error {
	d := json.NewDecoder(r.Body)
	var req ops.AcceptRemoteClusterRequest
	if err := d.Decode(&req); err != nil {
		return trace.BadParameter(err.Error())
	}
	resp, err := context.Operator.AcceptRemoteCluster(req)
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, resp)
	return nil
}

/*  removeRemoteCluster handles the request to remove a cluster entry in the
    Ops Center that remote cluster sends when removing a trusted cluster
    for this Ops Center

    PUT /portal/v1/accounts/:account_id/sites/:site_domain/remove
*/
func (h *WebHandler) removeRemoteCluster(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *handlerContext) error {
	d := json.NewDecoder(r.Body)
	var req ops.RemoveRemoteClusterRequest
	if err := d.Decode(&req); err != nil {
		return trace.BadParameter(err.Error())
	}
	err := context.Operator.RemoveRemoteCluster(req)
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, statusOK("ok"))
	return nil
}

/*  newLicense generates a new license

    POST /portal/v1/license/new

    Success response:
    {
      "license": <license string>
    }
*/
func (h *WebHandler) newLicense(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *handlerContext) error {
	d := json.NewDecoder(r.Body)
	var req ops.NewLicenseRequest
	if err := d.Decode(&req); err != nil {
		return trace.BadParameter(err.Error())
	}
	license, err := context.Operator.NewLicense(req)
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, struct {
		License string `json:"license"`
	}{License: license})
	return nil
}

/*  checkSiteLicense verifies the license installed on the site

    POST /portal/v1/accounts/:account_id/sites/:site_domain/license/check

    Success response:
    {
      "message": "ok"
    }
*/
func (h *WebHandler) checkSiteLicense(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *handlerContext) error {
	if err := context.Operator.CheckSiteLicense(siteKey(p)); err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, statusOK("ok"))
	return nil
}

/*  updateLicense updates site's license

    POST /portal/v1/accounts/:account_id/sites/:site_domain/license

    Input: ops.UpdateLicenseRequest

    Success response:
    {
      "message": "license updated"
    }
*/
func (h *WebHandler) updateLicense(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *handlerContext) error {
	d := json.NewDecoder(r.Body)
	var req ops.UpdateLicenseRequest
	if err := d.Decode(&req); err != nil {
		return trace.BadParameter(err.Error())
	}
	if err := context.Operator.UpdateLicense(req); err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, statusOK("license updated"))
	return nil
}

/*  getLicenseCA returns CA certificate Ops Center uses to sign licenses

    GET /portal/v1/license/ca

    Input: ops.SiteKey

    Success response: { "certificate": []byte("") }
*/
func (h *WebHandler) getLicenseCA(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *handlerContext) error {
	certificate, err := context.Operator.GetLicenseCA()
	if err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, map[string][]byte{
		"certificate": certificate,
	})
	return nil
}

func (h *WebHandler) needsAuth(fn serviceHandle) httprouter.Handle {
	handler := func(w http.ResponseWriter, r *http.Request, params httprouter.Params) error {
		setAntiXSSHeaders(w.Header())
		ossContext, err := opshandler.GetHandlerContext(w, r, h.GetConfig().Backend,
			h.GetConfig().Operator, h.GetConfig().Authenticator, h.GetConfig().Users)
		if err != nil {
			return trace.Wrap(err)
		}
		// wrap enterprise operator in acl as well
		operatorACL, ok := ossContext.Operator.(*ossops.OperatorACL)
		if !ok {
			return trace.BadParameter("unexpected type: %T", ossContext.Operator)
		}
		context := &handlerContext{
			HandlerContext: ossContext,
			Operator:       acl.OperatorWithACL(operatorACL, h.Operator),
		}
		err = fn(w, r.WithContext(context.Context), params, context)
		if err != nil {
			return trace.Wrap(err)
		}
		return nil
	}
	return func(w http.ResponseWriter, r *http.Request, params httprouter.Params) {
		err := handler(w, r, params)
		if err != nil {
			if trace.IsAccessDenied(err) {
				logrus.Debugf("Access denied for %v %v: %v.", r.Method, r.URL.Path,
					trace.DebugReport(err))
			}
			trace.WriteError(w, err)
		}
	}
}

// setAntiXSSHeaders sets HTTP headers against XSS attacks
func setAntiXSSHeaders(h http.Header) {
	telehttplib.SetNoSniff(h)
	telehttplib.SetSameOriginIFrame(h)
}

type serviceHandle func(http.ResponseWriter, *http.Request, httprouter.Params,
	*handlerContext) error

type handlerContext struct {
	// HandlerContext is the wrapped open-source ops handler context
	*opshandler.HandlerContext
	// Operator is the enterprise operator
	Operator ops.Operator
}
