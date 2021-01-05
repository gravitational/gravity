package webapi

import (
	"net/http"

	"github.com/gravitational/gravity/e/lib/ops"
	ossops "github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/storage"

	telehttplib "github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
)

// remoteAccessOutput represents cluster's remote support status
type remoteAccessOutput struct {
	// Status indicates whether remote support is enabled, disabled or not configured
	Status string `json:"status"`
}

func createRemoteAccessResponse(cluster storage.TrustedCluster) *remoteAccessOutput {
	var status string
	if cluster == nil {
		status = RemoteAccessNotConfigured
	} else if cluster.GetEnabled() {
		status = RemoteAccessOn
	} else {
		status = RemoteAccessOff
	}
	return &remoteAccessOutput{Status: status}
}

// getRemoteAccess returns remote access status
//
// GET /portalapi/v1/sites/:domain/access
//
// Output:
//
// {
//   "status": "on"
// }
func (m *Handler) getRemoteAccess(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *authContext) (interface{}, error) {
	cluster, err := ops.GetTrustedCluster(ossops.SiteKey{
		AccountID:  ctx.User.GetAccountID(),
		SiteDomain: p.ByName("domain"),
	}, ctx.Operator)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	return createRemoteAccessResponse(cluster), nil
}

// updateRemoteAccessInput is the request to enable/disable remote access
type updateRemoteAccessInput struct {
	// Enabled is whether to enable or disable the access
	Enabled bool `json:"enabled"`
}

// updateRemoteAccess updates remote access status for the specified domain
//
// PUT /portalapi/v1/sites/:domain/access
//
// Input:
//
// {
//   "enabled": true
// }
func (m *Handler) updateRemoteAccess(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *authContext) (interface{}, error) {
	var input updateRemoteAccessInput
	err := telehttplib.ReadJSON(r, &input)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cluster, err := ops.GetTrustedCluster(m.clusterKey(p, ctx), ctx.Operator)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cluster.SetEnabled(input.Enabled)
	err = ctx.Operator.UpsertTrustedCluster(m.clusterKey(p, ctx), cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return createRemoteAccessResponse(cluster), nil
}

// clusterKey returns SiteKey from the request context
func (m *Handler) clusterKey(p httprouter.Params, ctx *authContext) ossops.SiteKey {
	return ossops.SiteKey{
		AccountID:  ctx.User.GetAccountID(),
		SiteDomain: p.ByName("domain"),
	}
}

const (
	// RemoteAccessOn means remote support switch is turned on
	RemoteAccessOn = "on"
	// RemoteAccessOff means remote support switch is turned off
	RemoteAccessOff = "off"
	// RemoteAccessNotConfigured means the cluster is connected to any Ops Center
	RemoteAccessNotConfigured = "n/a"
)
