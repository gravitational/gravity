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
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/cloudflare/cfssl/csr"
	"github.com/gravitational/license/authority"
	rt "github.com/gravitational/teleport/lib/reversetunnel"
	teleservices "github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
	"github.com/gravitational/ttlmap"
	log "github.com/sirupsen/logrus"
	"github.com/vulcand/oxy/forward"
)

// Forwarder allows to forward HTTP requests from OpsCenter or gravity site to a service running
// inside deployed k8s cluster
type Forwarder interface {
	// ForwardToKube forwards the request to the authenticated k8s API,
	// Requires operator bound to current user to function properly
	ForwardToKube(w http.ResponseWriter, r *http.Request, siteName, URL string) error
	// ForwardToService forwards the request to a Kubernetes service
	ForwardToService(w http.ResponseWriter, r *http.Request, req ForwardRequest) error
}

// ForwarderConfig is a config for a forwarder
type ForwarderConfig struct {
	// Tunnel is the teleport reverse tunnel
	Tunnel rt.Server
	// User specifies an optional override for Common Name
	// to use when requesting certificates for kubernetes
	User string
}

type forwarder struct {
	ForwarderConfig
	kubeForwarders *ttlmap.TTLMap
	keyPEM         []byte
	sync.Mutex
}

// NewForwarder creates a new forwarder
func NewForwarder(cfg ForwarderConfig) (Forwarder, error) {
	kubeForwarders, err := ttlmap.New(defaults.ClientCacheSize)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	key, err := utils.GeneratePrivateKeyPEM()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &forwarder{
		ForwarderConfig: cfg,
		kubeForwarders:  kubeForwarders,
		keyPEM:          key,
	}, nil
}

func (f *forwarder) getKubeForwarder(ctx *sessionContext) *forward.Forwarder {
	f.Lock()
	defer f.Unlock()
	fwd, ok := f.kubeForwarders.Get(ctx.key())
	if ok {
		return fwd.(*forward.Forwarder)
	}
	return nil
}

// sessionContext contains structured context associated with current
// connection of a user to particular site
type sessionContext struct {
	// operator is ACL bound operator service connected
	// to the roles and permissions of a user
	operator ops.Operator
	// clusterName is a name of the cluster
	clusterName string
	// webSession is a current users web session
	webSession teleservices.WebSession
}

func (s *sessionContext) String() string {
	return fmt.Sprintf("sessionContext(cluster=%v, user=%v, session=%v)", s.clusterName, s.webSession.GetUser(), s.webSession.GetName()[:4])
}

// key is used to cache the whole context
func (s *sessionContext) key() string {
	return fmt.Sprintf("%v.%v.%v", s.clusterName, s.webSession.GetUser(), s.webSession.GetName())
}

func getSessionContext(ctx context.Context, clusterName string) (*sessionContext, error) {
	webSession := ops.SessionFromContext(ctx)
	if webSession == nil {
		return nil, trace.NotFound("missing web session context")
	}
	operator := ops.OperatorFromContext(ctx)
	if operator == nil {
		return nil, trace.NotFound("missing operator context")
	}
	return &sessionContext{operator: operator, webSession: webSession, clusterName: clusterName}, nil
}

func (f *forwarder) newKubeForwarder(ctx *sessionContext) (*forward.Forwarder, error) {
	remoteCluster, err := f.getRemoteSite(ctx.clusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	info, err := ctx.operator.GetCurrentUserInfo()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	req := csr.CertificateRequest{
		CN: info.User.GetName(),
	}
	if f.User != "" {
		req.CN = f.User
	}

	for _, group := range info.KubernetesGroups {
		req.Names = append(req.Names, csr.Name{O: group})
	}

	csr, key, err := authority.GenerateCSR(req, f.keyPEM)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	response, err := ctx.operator.SignTLSKey(ops.TLSSignRequest{
		AccountID:  defaults.SystemAccountID,
		SiteDomain: ctx.clusterName,
		CSR:        csr,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cert, err := tls.X509KeyPair(response.Cert, key)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pool := x509.NewCertPool()
	ok := pool.AppendCertsFromPEM(response.CACert)
	if !ok {
		return nil, trace.BadParameter("failed to append certs from PEM")
	}

	//nolint:gosec // TODO: set MinVersion
	tlsConfig := &tls.Config{
		RootCAs:      pool,
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
	}
	tlsConfig.BuildNameToCertificate()

	// this will work because proxies are listening on active master
	// nodes, and masters all have insecure local HTTP API on loopback
	targetAddr := fmt.Sprintf("%v:%v", constants.APIServerDomainName, defaults.APIServerSecurePort)

	remoteDialer := func(_, _ string) (net.Conn, error) {
		conn, err := remoteCluster.Dial(
			NewTCPAddr(defaults.RemoteClusterDialAddr), NewTCPAddr(targetAddr), nil)
		return conn, trace.Wrap(err)
	}

	fwd, err := forward.New(
		forward.RoundTripper(&http.Transport{
			Dial:                remoteDialer,
			TLSClientConfig:     tlsConfig,
			MaxIdleConnsPerHost: defaults.MaxRouterIdleConnsPerHost,
			// IdleConnTimeout defines the maximum amount of time before idle connections
			// are closed. Leaving this unset will lead to connections open forever and
			// will cause memory leaks in a long running process
			IdleConnTimeout: defaults.ClientCacheTTL,
		}),
		forward.WebsocketDial(remoteDialer),
		forward.Logger(log.StandardLogger()),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	f.Lock()
	defer f.Unlock()

	fwdI, ok := f.kubeForwarders.Get(ctx.key())
	if ok {
		return fwdI.(*forward.Forwarder), nil
	}

	ttl := ctx.webSession.GetExpiryTime().Sub(time.Now().UTC())
	if ttl <= time.Second {
		return nil, trace.NotFound("session has expired")
	}
	if err := f.kubeForwarders.Set(ctx.key(), fwd, ttl); err != nil {
		return nil, trace.Wrap(err)
	}
	return fwd, nil
}

func (f *forwarder) getOrCreateKubeForwarder(ctx context.Context, clusterName string) (*forward.Forwarder, error) {
	sessionContext, err := getSessionContext(ctx, clusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	client := f.getKubeForwarder(sessionContext)
	if client != nil {
		log.Debugf("return existing forwarder for %v", sessionContext)
		return client, nil
	}
	log.Debugf("created new forwarder for %v", sessionContext)
	return f.newKubeForwarder(sessionContext)
}

// ForwardToKube forwards the request to the k8s TLS API
func (f *forwarder) ForwardToKube(w http.ResponseWriter, r *http.Request, clusterName, URL string) error {
	fwd, err := f.getOrCreateKubeForwarder(r.Context(), clusterName)
	if err != nil {
		return trace.Wrap(err)
	}

	// add origin headers so the service consuming the request on the other site
	// is aware of where it came from
	r.Header.Add("X-Forwarded-Proto", "https")
	r.Header.Add("X-Forwarded-Host", r.Host)
	r.Header.Add("X-Forwarded-Path", r.URL.Path)

	targetAddr := fmt.Sprintf("%v:%v", constants.APIServerDomainName, defaults.APIServerSecurePort)
	r.URL.Scheme = "https"
	r.URL.Host = targetAddr
	r.URL.Path = URL
	r.RequestURI = r.URL.Path + "?" + r.URL.RawQuery

	fwd.ServeHTTP(w, r)
	return nil
}

// ForwardRequest encapsulates parameters for request forwarding
type ForwardRequest struct {
	// ClusterName is the name of the cluster to forward the request to
	ClusterName string
	// ServiceName is the name of the service to forward the request to
	ServiceName string
	// ServicePort is the service port
	ServicePort int
	// ServiceNamespace is the namespace where the service resides
	ServiceNamespace string
	// URL is the request URL
	URL string
}

// ForwardToService forwards the request to a Kubernetes service
func (f *forwarder) ForwardToService(w http.ResponseWriter, r *http.Request, req ForwardRequest) error {
	remoteSite, err := f.getRemoteSite(req.ClusterName)
	if err != nil {
		return trace.Wrap(err)
	}

	ns := req.ServiceNamespace
	if ns == "" {
		ns = defaults.KubeSystemNamespace
	}

	port := req.ServicePort
	if port == 0 {
		port = 80
	}

	addr := fmt.Sprintf(defaults.ServiceAddr, req.ServiceName, ns)
	addr += fmt.Sprintf(":%v", port)
	return trace.Wrap(f.forward(w, r, remoteSite, addr, req.URL))
}

// NewTCPAddr creates an instance of ADDR
func NewTCPAddr(a string) net.Addr {
	return &addr{net: "tcp", addr: a}
}

func (f *forwarder) forward(w http.ResponseWriter, r *http.Request, remoteSite rt.RemoteSite, targetAddr, URL string) error {
	remoteDialer := func(_, _ string) (net.Conn, error) {
		conn, err := remoteSite.Dial(
			NewTCPAddr(defaults.RemoteClusterDialAddr), NewTCPAddr(targetAddr), nil)
		return conn, trace.Wrap(err)
	}

	fwd, err := forward.New(
		forward.RoundTripper(&http.Transport{
			Dial: remoteDialer,
		}),
		forward.WebsocketDial(remoteDialer),
		forward.Logger(log.StandardLogger()),
	)
	if err != nil {
		return trace.Wrap(err)
	}

	// add origin headers so the service consuming the request on the other site
	// is aware of where it came from
	r.Header.Add("X-Forwarded-Proto", "https")
	r.Header.Add("X-Forwarded-Host", r.Host)
	r.Header.Add("X-Forwarded-Path", r.URL.Path)

	r.URL.Scheme = "http"
	r.URL.Host = targetAddr
	r.URL.Path = URL
	r.RequestURI = r.URL.Path + "?" + r.URL.RawQuery

	log.Debugf("forwarding to cluster %q: %#v", remoteSite.GetName(), r)
	fwd.ServeHTTP(w, r)

	return nil
}

func (f *forwarder) getRemoteSite(siteName string) (rt.RemoteSite, error) {
	for _, site := range f.Tunnel.GetSites() {
		if site.GetName() == siteName {
			return site, nil
		}
	}
	return nil, trace.NotFound("site not found: %v", siteName)
}

type addr struct {
	net  string
	addr string
}

func (a *addr) Network() string {
	return a.net
}

func (a *addr) String() string {
	return a.addr
}
