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

/*
handler introduces new way to access site API:

1. Get access to k8s master

GET  /sites/v1/:account_id/:site_domain/proxy/master/k8s/<k8s-specific endpoints>

for example this request will hit the API

GET  /sites/v1/:account_id/:site_domain/proxy/master/k8s/api/v1/namespaces


2. Get list of servers

GET  /sites/v1/:account_id/:site_id/servers

returns [{
   "advertise-ip": "127.0.0.1",
   "hostname": "bob.example.com",
   "role": "database"
}

(You can later use the server's hostname in Shrink operation)

3. Access any HTTP(S) URL inside the cluster:

GET  /sites/v1/:account_id/:site_domain/proxy/address/:scheme/:host/:port/*rest of the URL

for example this request will hit the http://192.168.1.1:8080

GET  /sites/v1/:account_id/:site_domain/proxy/address/http/192.168.1.1/8080

*/
package process

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/ops/opshandler"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/users"

	web "github.com/gravitational/gravity/lib/webapi"
	"github.com/gravitational/roundtrip"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
)

type proxyHandler struct {
	httprouter.Router
	cfg proxyHandlerConfig
}

type proxyHandlerConfig struct {
	tunnel        reversetunnel.Server
	operator      ops.Operator
	users         users.Identity
	authenticator users.Authenticator
	forwarder     web.Forwarder
	devmode       bool
	backend       storage.Backend
}

func newProxyHandler(cfg proxyHandlerConfig) *proxyHandler {
	ph := &proxyHandler{
		cfg: cfg,
	}
	for _, method := range []string{"GET", "POST", "PUT", "DELETE", "PATCH"} {
		ph.Handle(method, "/sites/v1/:account_id/:domain/proxy/master/*rest", ph.needsAuth(ph.forwardToMaster))
	}
	ph.GET("/sites/v1/:account_id/:domain/servers", ph.needsAuth(ph.getServers))
	ph.OPTIONS("/*path", ph.options)
	ph.NotFound = ph.notFound()
	return ph
}

func (ph *proxyHandler) notFound() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Debugf("Not found handler: %v %v", r.Method, r.URL)
		roundtrip.ReplyJSON(w, http.StatusNotFound, map[string]interface{}{"message": "method not found"})
	}
}

func (ph *proxyHandler) options(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	roundtrip.ReplyJSON(w, http.StatusOK, map[string]interface{}{"ok": "ok"})
}

func (ph *proxyHandler) needsAuth(fn opshandler.ServiceHandle) httprouter.Handle {
	return opshandler.NeedsAuth(ph.cfg.devmode, ph.cfg.backend, ph.cfg.operator, ph.cfg.authenticator, ph.cfg.users, fn)
}

func (ph *proxyHandler) getServers(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *opshandler.HandlerContext) error {
	accountID, siteDomain := p.ByName("account_id"), p.ByName("domain")
	site, err := ctx.Operator.GetSite(ops.SiteKey{
		SiteDomain: siteDomain,
		AccountID:  accountID,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	remoteSite, err := ph.cfg.tunnel.GetSite(site.Domain)
	if err != nil {
		return trace.Wrap(err)
	}

	client, err := remoteSite.GetClient()
	if err != nil {
		return trace.Wrap(err)
	}

	nodes, err := client.GetNodes(defaults.Namespace)
	if err != nil {
		return trace.Wrap(err)
	}

	out := make([]storage.Server, len(nodes))
	for i, s := range nodes {
		labels := s.GetAllLabels()
		out[i] = storage.Server{
			Hostname:    labels[ops.ServerFQDN],
			AdvertiseIP: labels[ops.AdvertiseIP],
			Role:        labels[ops.AppRole],
		}
	}
	roundtrip.ReplyJSON(w, http.StatusOK, out)
	return nil
}

func (ph *proxyHandler) forwardToMaster(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *opshandler.HandlerContext) error {
	accountID, siteDomain := p.ByName("account_id"), p.ByName("domain")
	site, err := ctx.Operator.GetSite(ops.SiteKey{
		SiteDomain: siteDomain,
		AccountID:  accountID,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	url := strings.TrimPrefix(
		r.URL.Path, fmt.Sprintf("/sites/v1/%v/%v/proxy/master", accountID, siteDomain))

	if strings.HasPrefix(url, defaults.APIPrefix) {
		url = url[len(defaults.APIPrefix):]
		err = ph.cfg.forwarder.ForwardToKube(w, r, site.Domain, url)
	} else if strings.HasPrefix(url, defaults.LogServicePrefix) {
		url = filepath.Join("/", defaults.LogServiceAPIVersion, url[len(defaults.LogServicePrefix):])
		err = ph.cfg.forwarder.ForwardToService(w, r, web.ForwardRequest{
			ClusterName: site.Domain,
			ServiceName: defaults.LogServiceName,
			ServicePort: defaults.LogServicePort,
			URL:         url,
		})
	}

	return trace.Wrap(err)
}
