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

package clients

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/gravitational/gravity/lib/app"
	appsclient "github.com/gravitational/gravity/lib/app/client"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/httplib"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/ops/opsclient"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/users"
	"github.com/gravitational/license/authority"

	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/trace"
	"github.com/gravitational/ttlmap"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// ClusterClientsConfig contains configuration needed for remote clients
type ClusterClientsConfig struct {
	// Backend is a storage backend
	Backend storage.Backend
	// Tunnel is a reverse tunnel server providing access to remote sites
	Tunnel reversetunnel.Server
}

// ClusterClients provides access to remote clusters' services such as operator or application
type ClusterClients struct {
	ClusterClientsConfig
	sync.RWMutex
	// opsClients is remote operators cache
	opsClients *ttlmap.TTLMap
	// appsClients is remote app services cache
	appsClients *ttlmap.TTLMap
	// kubeClients is remote Kubernetes clients cache
	kubeClients *ttlmap.TTLMap
}

// NewClusterClients returns a new cluster clients interface
func NewClusterClients(conf ClusterClientsConfig) (*ClusterClients, error) {
	if conf.Backend == nil {
		return nil, trace.BadParameter("missing parameter Backend")
	}
	if conf.Tunnel == nil {
		return nil, trace.BadParameter("missing parameter Tunnel")
	}
	opsClients, err := ttlmap.New(defaults.ClientCacheSize)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	appsClients, err := ttlmap.New(defaults.ClientCacheSize)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	kubeClients, err := ttlmap.New(defaults.ClientCacheSize)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &ClusterClients{
		ClusterClientsConfig: conf,
		opsClients:           opsClients,
		appsClients:          appsClients,
		kubeClients:          kubeClients,
	}, nil
}

// OpsClient returns remote operator for the specified cluster
func (r *ClusterClients) OpsClient(clusterName string) (ops.Operator, error) {
	client := r.getOpsClient(clusterName)
	if client != nil {
		return client, nil
	}
	return r.newOpsClient(clusterName)
}

// AppsClient returns remote apps service for the specified cluster
func (r *ClusterClients) AppsClient(clusterName string) (app.Applications, error) {
	client := r.getAppsClient(clusterName)
	if client != nil {
		return client, nil
	}
	return r.newAppsClient(clusterName)
}

// KubeClient returns Kubernetes API client for the specified cluster and user
func (r *ClusterClients) KubeClient(operator ops.Operator, user ops.UserInfo, clusterName string) (*kubernetes.Clientset, error) {
	client := r.getKubeClient(clusterName, user)
	if client != nil {
		return client, nil
	}
	return r.newKubeClient(operator, user, clusterName)
}

func (r *ClusterClients) getOpsClient(clusterName string) ops.Operator {
	r.Lock()
	defer r.Unlock()
	clientI, ok := r.opsClients.Get(clusterName)
	if ok {
		return clientI.(ops.Operator)
	}
	return nil
}

func (r *ClusterClients) getAppsClient(clusterName string) app.Applications {
	r.Lock()
	defer r.Unlock()
	clientI, ok := r.appsClients.Get(clusterName)
	if ok {
		return clientI.(app.Applications)
	}
	return nil
}

func (r *ClusterClients) getKubeClient(clusterName string, user ops.UserInfo) *kubernetes.Clientset {
	r.Lock()
	defer r.Unlock()
	clientI, ok := r.kubeClients.Get(fmt.Sprintf("%v.%v", clusterName, user.User.GetName()))
	if ok {
		return clientI.(*kubernetes.Clientset)
	}
	return nil
}

func (r *ClusterClients) newOpsClient(clusterName string) (ops.Operator, error) {
	info, err := r.clientInfo(clusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	r.Lock()
	defer r.Unlock()

	clientI, ok := r.opsClients.Get(clusterName)
	if ok {
		return clientI.(ops.Operator), nil
	}

	client, err := opsclient.NewAuthenticatedClient(
		info.url.String(), info.key.UserEmail, info.key.Token, opsclient.HTTPClient(info.httpClient))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = r.opsClients.Set(clusterName, client, defaults.ClientCacheTTL)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return client, nil
}

func (r *ClusterClients) newAppsClient(clusterName string) (app.Applications, error) {
	info, err := r.clientInfo(clusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	r.Lock()
	defer r.Unlock()

	clientI, ok := r.appsClients.Get(clusterName)
	if ok {
		return clientI.(app.Applications), nil
	}

	client, err := appsclient.NewAuthenticatedClient(
		info.url.String(), info.key.UserEmail, info.key.Token, appsclient.HTTPClient(info.httpClient))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = r.appsClients.Set(clusterName, client, defaults.ClientCacheTTL)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return client, nil
}

func (r *ClusterClients) newKubeClient(operator ops.Operator, user ops.UserInfo, clusterName string) (*kubernetes.Clientset, error) {
	remoteCluster, err := r.Tunnel.GetSite(clusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	csr, key, err := authority.GenerateCSR(user.ToCSR(), nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	signResponse, err := operator.SignTLSKey(ops.TLSSignRequest{
		AccountID:  defaults.SystemAccountID,
		SiteDomain: clusterName,
		CSR:        csr,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	client, err := kubernetes.NewForConfig(&rest.Config{
		Host: defaults.KubernetesAPIURL,
		TLSClientConfig: rest.TLSClientConfig{
			CAData:   signResponse.CACert,
			CertData: signResponse.Cert,
			KeyData:  key,
		},
		WrapTransport: func(t http.RoundTripper) http.RoundTripper {
			transport := t.(*http.Transport)
			transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
				return remoteCluster.Dial(
					&httplib.Addr{Net: "tcp", Addr: defaults.RemoteClusterDialAddr},
					&httplib.Addr{Net: "tcp", Addr: defaults.KubernetesAPIAddress},
					nil)
			}
			transport.MaxIdleConnsPerHost = defaults.MaxRouterIdleConnsPerHost
			transport.IdleConnTimeout = defaults.ClientCacheTTL
			return transport
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	r.Lock()
	defer r.Unlock()
	err = r.kubeClients.Set(fmt.Sprintf("%v.%v", clusterName, user.User.GetName()),
		client, defaults.ClientCacheTTL)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return client, nil
}

func (r *ClusterClients) clientInfo(siteName string) (*clientInfo, error) {
	remoteSite, err := r.Tunnel.GetSite(siteName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// if this is a tunnel established by an install wizard to this Ops Center,
	// strip the prefix from Teleport site name to get the name of the cluster
	// that is stored in the backend
	clusterName := siteName
	if strings.HasPrefix(siteName, constants.InstallerTunnelPrefix) {
		clusterName = clusterName[len(constants.InstallerTunnelPrefix):]
	}

	key, err := users.GetSiteAgent(clusterName, r.Backend)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var u *url.URL
	// when making a request through the tunnel to install wizard, it makes a
	// call to its own API via loopback interface
	if strings.HasPrefix(siteName, constants.InstallerTunnelPrefix) {
		u, err = url.Parse(defaults.LocalWizardURL)
	} else {
		u, err = url.Parse(defaults.GravityServiceURL)
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &clientInfo{
		key:        key,
		url:        u,
		httpClient: httplib.GetRemoteClient(remoteSite, u),
	}, nil
}

// clientInfo encapsulates a few common parameters needed when creating remote clients
type clientInfo struct {
	key        *storage.APIKey
	url        *url.URL
	httpClient *http.Client
}
