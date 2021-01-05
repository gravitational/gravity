package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gravitational/gravity/e/lib/ops"
	"github.com/gravitational/gravity/lib/ops/opsclient"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/roundtrip"
	telehttplib "github.com/gravitational/teleport/lib/httplib"
	teleservices "github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/julienschmidt/httprouter"
)

/*  upsertTrustedCluster creates or updates a trusted cluster resource

    POST /portal/v1/accounts/:account_id/sites/:site_domain/trustedclusters
*/
func (h *WebHandler) upsertTrustedCluster(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *handlerContext) error {
	var req opsclient.UpsertResourceRawReq
	if err := telehttplib.ReadJSON(r, &req); err != nil {
		return trace.Wrap(err)
	}
	cluster, err := storage.UnmarshalTrustedCluster(req.Resource)
	if err != nil {
		return trace.Wrap(err)
	}
	if req.TTL != 0 {
		cluster.SetTTL(clockwork.NewRealClock(), req.TTL)
	}
	if err := ctx.Operator.UpsertTrustedCluster(r.Context(), siteKey(p), cluster); err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, message(
		"trusted cluster %v upserted", cluster.GetName()))
	return nil
}

/*  getTrustedCluster looks for a trusted cluster by its name

    GET /portal/v1/accounts/:account_id/sites/:site_domain/trustedclusters/:name
*/
func (h *WebHandler) getTrustedCluster(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *handlerContext) error {
	cluster, err := ctx.Identity.GetTrustedCluster(p.ByName("name"))
	if err != nil {
		return trace.Wrap(err)
	}
	bytes, err := teleservices.GetTrustedClusterMarshaler().Marshal(cluster)
	return trace.Wrap(rawMessage(w, bytes, err))
}

/*  getTrustedClusters returns all configured trusted clusters

    GET /portal/v1/accounts/:account_id/sites/:site_domain/trustedclusters
*/
func (h *WebHandler) getTrustedClusters(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *handlerContext) error {
	clusters, err := ctx.Identity.GetTrustedClusters()
	if err != nil {
		return trace.Wrap(err)
	}
	items := make([]json.RawMessage, len(clusters))
	for i, cluster := range clusters {
		bytes, err := teleservices.GetTrustedClusterMarshaler().Marshal(cluster)
		if err != nil {
			return trace.Wrap(err)
		}
		items[i] = bytes
	}
	roundtrip.ReplyJSON(w, http.StatusOK, items)
	return nil
}

/*  deleteTrustedCluster deletes a trusted cluster by its name

    DELETE /portal/v1/accounts/:account_id/sites/:site_domain/trustedclusters/:name
*/
func (h *WebHandler) deleteTrustedCluster(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *handlerContext) error {
	delay, err := parseDuration(r, "delay")
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	req := ops.DeleteTrustedClusterRequest{
		AccountID:          p.ByName("account_id"),
		ClusterName:        p.ByName("site_domain"),
		TrustedClusterName: p.ByName("name"),
		Delay:              delay,
	}
	if err := ctx.Operator.DeleteTrustedCluster(r.Context(), req); err != nil {
		return trace.Wrap(err)
	}
	roundtrip.ReplyJSON(w, http.StatusOK, message(
		"trusted cluster %v deleted", p.ByName("name")))
	return nil
}

// parseDuration parses the specified query string parameter as duration
func parseDuration(r *http.Request, name string) (time.Duration, error) {
	s := r.URL.Query().Get(name)
	if s == "" {
		return 0, trace.BadParameter("expected a duration parameter %q", name)
	}
	dur, err := time.ParseDuration(s)
	if err != nil {
		return 0, trace.Wrap(err)
	}
	return dur, nil
}
