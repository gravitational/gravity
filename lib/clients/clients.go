package clients

import (
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

	"github.com/gravitational/roundtrip"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/trace"
	"github.com/gravitational/ttlmap"
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
	sync.Mutex
	// opsClients is remote operators cache
	opsClients *ttlmap.TTLMap
	// appsClients is remote app services cache
	appsClients *ttlmap.TTLMap
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
		return nil, err
	}
	appsClients, err := ttlmap.New(defaults.ClientCacheSize)
	if err != nil {
		return nil, err
	}
	return &ClusterClients{
		ClusterClientsConfig: conf,
		opsClients:           opsClients,
		appsClients:          appsClients,
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
		info.url.String(), info.key.UserEmail, info.key.Token, roundtrip.HTTPClient(info.httpClient))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	r.opsClients.Set(clusterName, client, defaults.ClientCacheTTL)
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
		info.url.String(), info.key.UserEmail, info.key.Token, roundtrip.HTTPClient(info.httpClient))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	r.appsClients.Set(clusterName, client, defaults.ClientCacheTTL)
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
