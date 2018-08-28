package webapi

import (
	"net/http"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/httplib"
	"github.com/gravitational/trace"

	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
)

const (
	// grafanaURL is endpoint for grafana reverse proxy
	// ideally we should be getting it from configuration
	grafanaURL = "/web/grafana"
)

// initGrafana prepares the context for grafana reverse proxy and returns proxy endpoint
func (m *Handler) initGrafana(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *AuthContext) (interface{}, error) {
	siteDomain := p[0].Value
	httplib.SetGrafanaHeaders(w.Header(), siteDomain, grafanaURL, false)
	var resData = struct {
		GrafanaURL string `json:"url"`
	}{
		GrafanaURL: grafanaURL,
	}

	return &resData, nil
}

// grafanaHandler forwards requests to Grafana running on site determined by a special cookie
func (h *WebHandler) grafanaServeHandler(w http.ResponseWriter, r *http.Request, p httprouter.Params, s session) {
	err := httplib.VerifySameOrigin(r)
	if err != nil {
		msg := "access denied"
		log.Warningf("%v: %v", msg, trace.DebugReport(err))
		replyError(w, msg, http.StatusForbidden)
		return
	}

	cookie, err := r.Cookie(constants.GrafanaContextCookie)
	if err != nil {
		replyError(w, err.Error(), http.StatusForbidden)
		return
	}

	site, err := h.cfg.Operator.GetSiteByDomain(cookie.Value)
	if err != nil {
		h.siteNotFoundHandler(w, r, p)
		return
	}

	err = h.cfg.Forwarder.ForwardToService(w, r, ForwardRequest{
		ClusterName: site.Domain,
		ServiceName: defaults.GrafanaServiceName,
		ServicePort: defaults.GrafanaServicePort,
		URL:         p.ByName("rest"),
	})

	if err != nil {
		replyError(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
