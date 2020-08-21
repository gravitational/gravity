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

package process

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/gravity/lib/app"
	apphandler "github.com/gravitational/gravity/lib/app/handler"
	appservice "github.com/gravitational/gravity/lib/app/service"
	"github.com/gravitational/gravity/lib/autoscale/aws"
	"github.com/gravitational/gravity/lib/blob"
	blobclient "github.com/gravitational/gravity/lib/blob/client"
	blobcluster "github.com/gravitational/gravity/lib/blob/cluster"
	blobfs "github.com/gravitational/gravity/lib/blob/fs"
	blobhandler "github.com/gravitational/gravity/lib/blob/handler"
	"github.com/gravitational/gravity/lib/clients"
	cloudaws "github.com/gravitational/gravity/lib/cloudprovider/aws"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/docker"
	"github.com/gravitational/gravity/lib/helm"
	"github.com/gravitational/gravity/lib/httplib"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/modules"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/ops/monitoring"
	"github.com/gravitational/gravity/lib/ops/opshandler"
	"github.com/gravitational/gravity/lib/ops/opsroute"
	"github.com/gravitational/gravity/lib/ops/opsservice"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/pack/layerpack"
	"github.com/gravitational/gravity/lib/pack/localpack"
	"github.com/gravitational/gravity/lib/pack/webpack"
	"github.com/gravitational/gravity/lib/processconfig"
	"github.com/gravitational/gravity/lib/rpc"
	pb "github.com/gravitational/gravity/lib/rpc/proto"
	rpcserver "github.com/gravitational/gravity/lib/rpc/server"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/storage/keyval"
	"github.com/gravitational/gravity/lib/users"
	"github.com/gravitational/gravity/lib/users/usersservice"
	"github.com/gravitational/gravity/lib/utils"
	web "github.com/gravitational/gravity/lib/webapi"
	"github.com/gravitational/gravity/lib/webapi/ui"

	telelib "github.com/gravitational/teleport/lib"
	teleauth "github.com/gravitational/teleport/lib/auth"
	telecfg "github.com/gravitational/teleport/lib/config"
	teledefaults "github.com/gravitational/teleport/lib/defaults"
	telemodules "github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/service"
	teleservice "github.com/gravitational/teleport/lib/service"
	teleservices "github.com/gravitational/teleport/lib/services"
	teleutils "github.com/gravitational/teleport/lib/utils"
	teleweb "github.com/gravitational/teleport/lib/web"

	"github.com/cloudflare/cfssl/csr"
	"github.com/gravitational/license/authority"
	"github.com/gravitational/roundtrip"
	"github.com/gravitational/teleport"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/kubernetes"
)

type Process struct {
	sync.Once
	sync.Mutex
	service.Supervisor
	logrus.FieldLogger
	context        context.Context
	backend        storage.Backend
	leader         storage.Leader
	packages       pack.PackageService
	cfg            processconfig.Config
	tcfg           telecfg.FileConfig
	identity       users.Identity
	mode           string
	teleportConfig *service.Config
	clusterObjects blob.Objects
	localObjects   blob.Objects
	id             string
	leaderID       string
	applications   app.Applications
	operator       ops.Operator
	reverseTunnel  reversetunnel.Server
	proxy          *teleportProxyService
	client         *kubernetes.Clientset
	// resumeOperationCh relays requests to resume last active cluster operation
	resumeOperationCh chan struct{}
	// clusterServices contains registered cluster services that start when
	// process becomes a leader and stop when leadership is lost
	clusterServices []clusterService
	// cancelServices is the cancel function that stops local cluster services
	cancelServices context.CancelFunc
	agentServer    rpcserver.Server
	agentService   ops.AgentService
	// handlers contains all initialized web handlers
	handlers Handlers
	// rpcCreds holds generated RPC agents credentials
	rpcCreds rpcCredentials
	// authGatewayConfig is the current auth gateway configuration (basically,
	// a config that gets applied on top of teleport's config the process
	// was started with)
	authGatewayConfig storage.AuthGateway
}

// Handlers combines all the process' web and API Handlers
type Handlers struct {
	// Packages is package service web handler
	Packages *webpack.Server
	// Apps is app service web handler
	Apps *apphandler.WebHandler
	// Operator is ops service web handler
	Operator *opshandler.WebHandler
	// Web is web UI handler
	Web *web.WebHandler
	// WebProxy is Teleport web API handler
	WebProxy *teleweb.RewritingHandler
	// WebAPI is web API handler
	WebAPI *web.Handler
	// Proxy is cluster proxy handler
	Proxy *proxyHandler
	// BLOB is object storage service web handler
	BLOB *blobhandler.Server
	// Registry is the Docker registry handler.
	Registry http.Handler
}

// rpcCredentials holds generated RPC agents credentials
type rpcCredentials struct {
	ca     *authority.TLSKeyPair
	client *authority.TLSKeyPair
	server *authority.TLSKeyPair
}

// ServiceStartedEvent defines the payload of the gravity service start event.
// It is used to relay success or failure of service initialization to event listeners
type ServiceStartedEvent struct {
	// Error is set if the service has failed to initialize
	Error error
}

// New returns and starts a new instance of gravity, either Site or OpsCenter,
// including services like teleport proxy and teleport auth
func New(ctx context.Context, cfg processconfig.Config, tcfg telecfg.FileConfig) (*Process, error) {
	// enable Enterprise version modules
	telemodules.SetModules(&enterpriseModules{})

	err := cfg.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	hook, err := trace.NewUDPHook()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	logrus.AddHook(hook)

	if cfg.Profile.HTTPEndpoint != "" {
		err = StartProfiling(ctx, cfg.Profile.HTTPEndpoint, cfg.Profile.OutputDir)
		if err != nil {
			logrus.Warningf("Failed to setup profiling: %v.", trace.DebugReport(err))
		}
	}

	backend, err := cfg.CreateBackend()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	identity, err := usersservice.New(usersservice.Config{
		Backend: backend,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	objects, err := blobfs.New(blobfs.Config{
		Path: filepath.Join(cfg.DataDir, defaults.PackagesDir),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	processID := cfg.ProcessID()

	blobUser := fmt.Sprintf("%v@%v", processID, constants.BlobUserSuffix)
	blobKey, err := blob.UpsertUser(identity, blobUser)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	peerPool, err := blobclient.NewPool(blobUser, blobKey.Token)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	peerAddr, err := cfg.Pack.PeerAddr()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	clusterObjects, err := blobcluster.New(blobcluster.Config{
		Local:         objects,
		Backend:       backend,
		GetPeer:       peerPool.GetPeer,
		ID:            processID,
		AdvertiseAddr: fmt.Sprintf("https://%v", peerAddr.Addr),
		// TODO: set WriteFactor to the number of controller instances
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clusterObjects.Start()

	packages, err := localpack.New(localpack.Config{
		Backend:     backend,
		DownloadURL: fmt.Sprintf("https://%v", cfg.Pack.GetAddr().Addr),
		UnpackedDir: filepath.Join(cfg.DataDir, defaults.PackagesDir, defaults.UnpackedDir),
		Objects:     clusterObjects,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	process := &Process{
		context:        ctx,
		packages:       packages,
		backend:        backend,
		cfg:            cfg,
		tcfg:           tcfg,
		mode:           cfg.Mode,
		identity:       identity,
		clusterObjects: clusterObjects,
		localObjects:   objects,
		id:             processID,
	}
	if leader, ok := backend.(storage.Leader); ok {
		process.leader = leader
	}

	process.FieldLogger = logrus.WithFields(logrus.Fields{
		trace.Component:     "process",
		constants.FieldMode: cfg.Mode,
	})

	process.Infof("Process ID: %v.", processID)

	process.authGatewayConfig, err = process.getOrInitAuthGatewayConfig()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	process.teleportConfig, err = process.buildTeleportConfig(process.authGatewayConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if cfg.Devmode {
		process.Warn("Enabling Teleport insecure dev mode!")
		telelib.SetInsecureDevMode(true)
	}

	process.Infof("Teleport config: %#v.", process.teleportConfig)
	process.Infof("Gravity config: %#v.", cfg)

	process.removeLegacyIdentities()
	return process, nil
}

// Init initializes the process internal services but does not start them
func (p *Process) Init(ctx context.Context) error {
	if err := p.initAccount(); err != nil {
		return trace.Wrap(err)
	}
	if err := p.initService(ctx); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// Start initializes the process and starts all services
func (p *Process) Start() (err error) {
	p.Supervisor, err = service.NewTeleport(p.teleportConfig)
	if err != nil {
		return trace.Wrap(err)
	}
	p.Supervisor.RegisterFunc("gravity.service", func() (err error) {
		defer p.Supervisor.BroadcastEvent(service.Event{
			Name:    constants.ServiceStartedEvent,
			Payload: &ServiceStartedEvent{Error: err},
		})
		if err = p.Init(p.context); err != nil {
			return trace.Wrap(err)
		}
		if err = p.Serve(); err != nil {
			return trace.Wrap(err)
		}
		return nil
	})
	return p.Supervisor.Start()
}

// TeleportConfig returns the process teleport config
func (p *Process) TeleportConfig() *service.Config {
	return p.teleportConfig
}

// AgentService returns the process agent service
func (p *Process) AgentService() ops.AgentService {
	return p.agentService
}

// AgentServer returns the process RPC server
func (p *Process) AgentServer() rpcserver.Server {
	return p.agentServer
}

// UsersService returns the process identity service
func (p *Process) UsersService() users.Identity {
	return p.identity
}

// Backend returns the process backend
func (p *Process) Backend() storage.Backend {
	return p.backend
}

// Packages returns the process package service
func (p *Process) Packages() pack.PackageService {
	return p.packages
}

// Operator returns the process ops service
func (p *Process) Operator() ops.Operator {
	return p.operator
}

// Handlers returns all process web handlers
func (p *Process) Handlers() *Handlers {
	return &p.handlers
}

// ReverseTunnel returns the process reverse tunnel service
func (p *Process) ReverseTunnel() reversetunnel.Server {
	return p.reverseTunnel
}

// KubeClient returns the process Kubernetes client
func (p *Process) KubeClient() *kubernetes.Clientset {
	return p.client
}

// Context returns the process context
func (p *Process) Context() context.Context {
	return p.context
}

// Config returns the process config
func (p *Process) Config() *processconfig.Config {
	return &p.cfg
}

// StartResumeOperationLoop starts a loop that handles requests to resume
// pending cluster operations
func (p *Process) StartResumeOperationLoop() {
	p.resumeOperationCh = make(chan struct{})
	go p.resumeLastOperationLoop()
}

func (p *Process) getTeleportConfigFromImportState() (*telecfg.FileConfig, error) {
	if p.cfg.ImportDir == "" {
		return nil, nil
	}

	cluster, err := p.backend.GetLocalSite(defaults.SystemAccountID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	importer, err := newImporter(p.cfg.ImportDir)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer importer.Close()

	telecfg, err := importer.getMasterTeleportConfig(cluster.Domain)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return telecfg, nil
}

// ImportState imports site state from the specified import directory into the
// configured backend
func (p *Process) ImportState(importDir string) (err error) {
	done, err := p.backend.GetClusterImportStatus()
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err, "failed to query import state flag")
	}
	if done {
		p.Debug("Cluster state is already imported.")
		return nil
	}

	p.Debugf("Init from %q.", importDir)
	importer, err := newImporter(importDir)
	if err != nil {
		return trace.Wrap(err)
	}
	defer importer.Close()

	err = importer.importState(p.backend, p.packages)
	if err == nil {
		err = utils.Retry(defaults.RetryInterval, defaults.RetryLessAttempts,
			p.backend.SetClusterImported)
	}
	return trace.Wrap(err)
}

// InitRPCCredentials initializes the package with RPC secrets
func (p *Process) InitRPCCredentials() error {
	pkg, err := rpc.InitCredentials(p.packages)
	if err != nil && !trace.IsAlreadyExists(err) {
		return trace.Wrap(err, "failed to init RPC credentials")
	}

	if trace.IsAlreadyExists(err) {
		p.Info("RPC credentials already initialized.")
	} else {
		p.Infof("Initialized RPC credentials: %v.", pkg)
	}
	return nil
}

func (p *Process) setLeader(id string) (oldID string) {
	p.Lock()
	defer p.Unlock()
	oldID = p.leaderID
	p.Infof("setLeader(%v)", id)
	p.leaderID = id
	return oldID
}

func (p *Process) leaderStatus() (string, bool) {
	p.Lock()
	defer p.Unlock()
	return p.leaderID, p.leaderID == p.id
}

func (p *Process) startAutoscale(ctx context.Context) error {
	_, err := cloudaws.NewLocalInstance()
	if err != nil {
		p.Info("Not on AWS, skip autoscaler start.")
		return nil
	}
	p.Info("Starting AWS autoscaler.")
	site, err := p.operator.GetLocalSite()
	if err != nil {
		return trace.Wrap(err)
	}
	client, err := tryGetPrivilegedKubeClient()
	if err != nil {
		return trace.Wrap(err)
	}
	autoscaler, err := aws.New(aws.Config{
		ClusterName: site.Domain,
		Client:      client,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	queueURL, err := autoscaler.GetQueueURL(ctx)
	if err != nil {
		p.Warningf("Failed to get Autoscale Queue URL: %v. Cluster will continue without autoscaling support. Fix the problem and restart the process.", trace.DebugReport(err))
		return nil
	}

	// receive and process events from SQS notification service
	p.RegisterClusterService(func(ctx context.Context) error {
		autoscaler.ProcessEvents(ctx, queueURL, p.operator)
		return nil
	})
	// publish discovery information about this cluster
	p.RegisterClusterService(func(ctx context.Context) error {
		autoscaler.PublishDiscovery(ctx, p.operator)
		return nil
	})
	return nil
}

// startApplicationsSynchronizer starts a service that periodically exports
// Docker images of the cluster's application images to the local Docker
// registry.
//
// TODO There may be a lot of apps, may be worth parallelizing this.
func (p *Process) startApplicationsSynchronizer(ctx context.Context) error {
	p.Info("Starting app images synchronizer.")
	go func() {
		ticker := time.NewTicker(defaults.AppSyncInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				apps, err := p.applications.ListApps(app.ListAppsRequest{
					Repository: defaults.SystemAccountOrg,
				})
				if err != nil {
					p.Errorf("Failed to query applications: %v.",
						trace.DebugReport(err))
					continue
				}
				for _, a := range apps {
					if a.Manifest.Kind == schema.KindApplication {
						p.Infof("Exporting app image %v to registry.", a.Package)
						err = p.applications.ExportApp(app.ExportAppRequest{
							Package:         a.Package,
							RegistryAddress: constants.LocalRegistryAddr,
							CertName:        constants.DockerRegistry,
						})
						if err != nil {
							p.Errorf("Failed to synchronize registry: %v.",
								trace.DebugReport(err))
						}
					}
				}
			case <-ctx.Done():
				p.Info("Stopping app images synchronizer.")
				return
			}
		}
	}()
	return nil
}

// startRegistrySynchronizer starts a goroutine that synchronizes the cluster app
// with the local registry periodically
func (p *Process) startRegistrySynchronizer(ctx context.Context) error {
	p.Info("Starting registry synchronizer.")
	go func() {
		ticker := time.NewTicker(defaults.RegistrySyncInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				cluster, err := p.operator.GetLocalSite()
				if err != nil {
					p.Errorf("Failed to query local cluster: %v.",
						trace.DebugReport(err))
					continue
				}
				err = p.applications.ExportApp(app.ExportAppRequest{
					Package:         cluster.App.Package,
					RegistryAddress: constants.LocalRegistryAddr,
					CertName:        constants.DockerRegistry,
				})
				if err != nil {
					p.Errorf("Failed to synchronize registry: %v.",
						trace.DebugReport(err))
				}
			case <-ctx.Done():
				p.Info("Stopping registry synchronizer.")
				return
			}
		}
	}()
	return nil
}

// startSiteStatusChecker periodically invokes app status hook; should be run in a goroutine
func (p *Process) startSiteStatusChecker(ctx context.Context) error {
	site, err := p.operator.GetLocalSite()
	if err != nil {
		return trace.Wrap(err)
	}

	p.Info("Starting cluster status checker.")
	ticker := time.NewTicker(defaults.SiteStatusCheckInterval)
	for {
		select {
		case <-ticker.C:
			key := ops.SiteKey{
				AccountID:  site.AccountID,
				SiteDomain: site.Domain,
			}
			if err := p.operator.CheckSiteStatus(key); err != nil {
				p.WithError(err).Warn("Cluster status check failed.")
			}
		case <-ctx.Done():
			p.Info("Stopping cluster status checker.")
			ticker.Stop()
			return nil
		}
	}
}

// startElection starts leader election process and watches the changes
func (p *Process) startElection() error {
	// elect gravity site leader - all other sites will remain
	// functional, but will not report OK to readiness probes,
	// making sure that k8s will direct traffic to current leader
	gravityLeaderKey := p.cfg.ETCD.Key + "/leader"
	err := p.leader.AddVoter(p.context, gravityLeaderKey, p.id, defaults.ElectionTerm)
	if err != nil {
		return trace.Wrap(err)
	}

	gravityLeadersC := make(chan string)
	p.leader.AddWatch(gravityLeaderKey, defaults.ElectionTerm/3, gravityLeadersC)
	p.RegisterFunc("gravity.election", func() error {
		p.Infof("Start watching gravity leaders.")
		for leaderID := range gravityLeadersC {
			oldLeaderID := p.setLeader(leaderID)
			p.onSiteLeader(oldLeaderID)
		}
		return nil
	})

	return nil
}

// onSiteLeader executes leader actions when active gravity master is re-elected
func (p *Process) onSiteLeader(oldLeaderID string) {
	var leaderID string
	var isLeader bool
	if leaderID, isLeader = p.leaderStatus(); !isLeader {
		p.Debugf("We are not a leader, the leader is %v.", leaderID)
		p.stopClusterServices()
		return
	}

	if oldLeaderID == leaderID {
		p.Debug("We are still the leader.")
		return
	}

	// Notify that the service became the leader
	p.Supervisor.BroadcastEvent(service.Event{Name: constants.ServiceSelfLeaderEvent})

	// attempt to resume last operation
	select {
	case p.resumeOperationCh <- struct{}{}:
	default:
		p.Warning("Cluster operation already active.")
	}

	// active master runs various services that periodically check
	// the cluster and application status, etc.
	p.startClusterServices(p.clusterServices)
}

// clusterService represents a blocking function that performs some cluster-specific
// periodic action (e.g. runs status hook) and can be stopped by canceling the
// provided context
type clusterService func(context.Context) error

// RegisterClusterService adds the service to the list of registered cluster
// services. Cluster services only run on the leader
func (p *Process) RegisterClusterService(service clusterService) {
	p.clusterServices = append(p.clusterServices, service)
}

// startClusterServices launches services that should be running only on
// active gravity master, like status checker
//
// No-op if the services are already running
func (p *Process) startClusterServices(services []clusterService) error {
	p.Lock()
	defer p.Unlock()

	if p.clusterServicesRunning() {
		return trace.AlreadyExists("cluster services are already running")
	}

	ctx, cancel := context.WithCancel(p.context)
	p.cancelServices = cancel

	for _, service := range services {
		go service(ctx)
	}

	return nil
}

// stopClusterServices stops active master services like cluster status checker
//
// No-op if the services are not running
func (p *Process) stopClusterServices() error {
	p.Lock()
	defer p.Unlock()

	if !p.clusterServicesRunning() {
		return trace.NotFound("cluster services are not running")
	}

	p.cancelServices()
	p.cancelServices = nil
	return nil
}

// clusterServicesRunning returns true if active master services like cluster status
// checks are running within this process
func (p *Process) clusterServicesRunning() bool {
	return p.cancelServices != nil
}

// resumeLastOperationLoop is a long running process that handles requests to resume
// last active cluster operations.
func (p *Process) resumeLastOperationLoop() {
	for {
		select {
		case <-p.resumeOperationCh:
			site, err := p.operator.GetLocalSite()
			if err != nil {
				p.Errorf("Failed to query installed site: %v.", trace.DebugReport(err))
				return
			}
			siteKey := ops.SiteKey{SiteDomain: site.Domain, AccountID: site.AccountID}
			err = p.resumeLastOperation(siteKey)
			if err == nil {
				continue
			}
			if trace.IsNotFound(err) {
				p.Info("No operation to resume found.")
			} else {
				p.Errorf("Failed to resume last operation: %v.", trace.DebugReport(err))
			}
		}
	}
}

// resumeLastOperation attempts to resume last operation in a retry loop.
// Attempts to resume last operation are only made when another operation is active or
// if attempt to resume the operation has failed due to a transient error
func (p *Process) resumeLastOperation(siteKey ops.SiteKey) error {
	// wrap is a circuit-breaker that retries only known transient errors
	wrap := func(err error) error {
		switch {
		case utils.IsClusterUnavailableError(err):
			p.Infof("Etcd cluster unavailable: %v.", err)
			return trace.Wrap(err)
		default:
			return &utils.AbortRetry{err}
		}
	}

	// retry in a loop to account for possible transient failures
	err := utils.Retry(defaults.ResumeRetryInterval, defaults.ResumeRetryAttempts, func() error {
		lastOperation, _, err := ops.GetLastOperation(siteKey, p.operator)
		if err != nil {
			if trace.IsNotFound(err) {
				p.Debugf("No last operation to resume found for %v.", siteKey)
				return nil
			}
			return wrap(err)
		}

		switch lastOperation.Type {
		case ops.OperationShrink:
			_, err = p.operator.ResumeShrink(siteKey)
		default:
			return wrap(trace.NotFound("no resumable operation found: %q", lastOperation.Type))
		}
		if err == nil {
			p.Debugf("Resumed operation on %v.", p.id)
			return nil
		}
		return wrap(err)
	})
	return trace.Wrap(err)
}

func (p *Process) syncRegistry(site *ops.Site, leaderIP string) error {
	p.Infof("Syncing registry.")
	targetRegistry := constants.DockerRegistry
	if leaderIP != "" {
		targetRegistry = fmt.Sprintf("%s:%s", leaderIP, constants.DockerRegistryPort)
	}
	start := time.Now()
	// use the cert name of default registry, but connect via IP without relying on DNS
	err := p.applications.ExportApp(app.ExportAppRequest{
		Package:         site.App.Package,
		RegistryAddress: targetRegistry,
		CertName:        constants.DockerRegistry,
	})
	if err != nil {
		return trace.Wrap(err, "failed syncing registry to %v", targetRegistry)
	}
	p.Infof("Synced registry %v in %v.", targetRegistry, time.Now().Sub(start))
	return nil
}

// ReportReadiness is an HTTP check that reports whether the system is ready.
// This system is ready if it is the active gravity site leader
func (p *Process) ReportReadiness(w http.ResponseWriter, r *http.Request) {
	currentLeaderID, isLeader := p.leaderStatus()
	var statusCode int
	if isLeader {
		statusCode = http.StatusOK
	} else {
		statusCode = http.StatusServiceUnavailable
	}
	roundtrip.ReplyJSON(w, statusCode,
		map[string]string{
			"status":    http.StatusText(statusCode),
			"leader_id": currentLeaderID})
}

// ReportHealth is HTTP check that reports that the system is healthy
// if it can successfully connect to the storage backend
func (p *Process) ReportHealth(w http.ResponseWriter, r *http.Request) {
	log := p.WithField(trace.Component, "healthz")
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	healthCh := make(chan error, 1)
	go func() {
		started := time.Now()
		_, err := p.backend.GetAccounts()

		// TODO(knisbet) This should really be reported through proper metrics collection
		// for now we just log it. On a good system, worse case scenario is about 15ms
		// so give some margin, but log if etcd is slightly on the slow side above 25ms.
		elapsed := time.Now().Sub(started)
		if elapsed > 25*time.Millisecond {
			log.WithField("elapsed", elapsed).Error("Backend is slow.")
		}
		healthCh <- err
	}()

	select {
	case err := <-healthCh:
		if err != nil {
			log.Error(trace.DebugReport(err))
			roundtrip.ReplyJSON(w, http.StatusServiceUnavailable,
				map[string]string{
					"status": "degraded",
					"info":   "backend is in error state",
				})
			return
		}
		roundtrip.ReplyJSON(w, http.StatusOK,
			map[string]string{
				"status": "ok",
				"info":   "service is up and running",
			})

	case <-ctx.Done():
		roundtrip.ReplyJSON(w, http.StatusServiceUnavailable,
			map[string]string{
				"status": "degraded",
				"info":   "backend timed out",
			})
	}
}

// initCertificateAuthority makes sure this OpsCenter has certficate authority and generates
// one if it does not exist yet
func (p *Process) initCertificateAuthority() error {
	exists, err := pack.HasCertificateAuthority(p.packages)
	if err != nil {
		return trace.Wrap(err)
	}

	// the OpsCenter already has a certificate authority
	if exists {
		p.Infof("Certificate authority is already initialized.")
		return nil
	}

	// if not, create it
	certAuthority, err := authority.GenerateSelfSignedCA(csr.CertificateRequest{
		CN: defaults.SystemAccountOrg,
		CA: &csr.CAConfig{
			Expiry: defaults.CACertificateExpiry.String(),
		},
	})
	if err != nil {
		return trace.Wrap(err)
	}

	err = pack.CreateCertificateAuthority(pack.CreateCAParams{
		Packages: p.packages,
		KeyPair:  *certAuthority,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	p.Infof("Initialized certificate authority.")
	return nil
}

func (p *Process) inKubernetes() bool {
	return os.Getenv(constants.EnvPodIP) != ""
}

// WebAdvertiseHost returns the name of the host where public web UI and APIs
// are served.
//
// In the general case it is the same as the API advertise host.
func (p *Process) WebAdvertiseHost() string {
	// we don't need port
	host, _ := utils.SplitHostPort(p.cfg.Pack.GetPublicAddr().Addr, "")
	return host
}

// APIAdvertiseHost returns the hostname advertised to clusters where this
// process serves its API.
func (p *Process) APIAdvertiseHost() string {
	// we don't need port
	host, _ := utils.SplitHostPort(p.cfg.Pack.GetAddr().Addr, "")
	return host
}

func (p *Process) teleportProcess() *teleservice.TeleportProcess {
	return p.Supervisor.(*teleservice.TeleportProcess)
}

func (p *Process) newAuthClient(authServers []teleutils.NetAddr, identity *teleauth.Identity) (*teleauth.Client, error) {
	tlsConfig, err := identity.TLSConfig(p.teleportProcess().Config.CipherSuites)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if p.teleportProcess().Config.ClientTimeout != 0 {
		return teleauth.NewTLSClient(authServers, tlsConfig,
			teleauth.ClientTimeout(p.teleportProcess().Config.ClientTimeout))
	}
	return teleauth.NewTLSClient(authServers, tlsConfig)
}

func (p *Process) initService(ctx context.Context) (err error) {
	eventC := make(chan service.Event)
	p.WaitForEvent(ctx, service.AuthIdentityEvent, eventC)
	event := <-eventC
	p.Infof("Received %v event.", &event)
	conn, ok := (event.Payload).(*service.Connector)
	if !ok {
		return trace.BadParameter("unsupported Connector type: %T", event.Payload)
	}
	p.WaitForEvent(ctx, service.ProxyReverseTunnelReady, eventC)
	event = <-eventC
	p.Infof("Received %v event.", &event)
	reverseTunnel, ok := (event.Payload).(reversetunnel.Server)
	if !ok {
		return trace.BadParameter("ReverseTunnel: unsupported type: %T", event.Payload)
	}
	p.reverseTunnel = reverseTunnel

	p.WaitForEvent(ctx, service.ProxyIdentityEvent, eventC)
	event = <-eventC
	p.Infof("Received %v event.", &event)

	proxyConn, ok := (event.Payload).(*service.Connector)
	if !ok {
		return trace.BadParameter("unsupported Connector type: %T", event.Payload)
	}

	if p.teleportConfig.Proxy.TLSKey == "" {
		if err := initSelfSignedHTTPSCert(p.teleportConfig, p.cfg.Hostname); err != nil {
			return trace.Wrap(err)
		}
	}

	p.handlers.WebProxy, err = teleweb.NewHandler(teleweb.Config{
		Proxy:        reverseTunnel,
		AuthServers:  p.teleportConfig.AuthServers[0],
		DomainName:   p.teleportConfig.Hostname,
		ProxyClient:  proxyConn.Client,
		DisableUI:    true,
		ProxySSHAddr: p.teleportConfig.Proxy.SSHAddr,
		ProxyWebAddr: p.teleportConfig.Proxy.WebAddr,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	authClient, err := p.newAuthClient(p.teleportConfig.AuthServers, conn.ClientIdentity)
	if err != nil {
		return trace.Wrap(err)
	}

	p.identity.SetAuth(authClient)

	proxyConfig, err := p.proxyConfig()
	if err != nil {
		return trace.Wrap(err)
	}

	sshProxyHost := fmt.Sprintf("%v:%v", proxyConfig.host, proxyConfig.sshPort)
	webProxyHost := fmt.Sprintf("%v:%v", proxyConfig.host, proxyConfig.webPort)
	teleportProxy, err := newTeleportProxyService(teleportProxyConfig{
		AuthClient:        authClient,
		ReverseTunnelAddr: proxyConfig.reverseTunnelAddr,
		WebProxyAddr:      webProxyHost,
		SSHProxyAddr:      sshProxyHost,
		// TODO(klizhentas) this means that it will only work if auth server
		// and portal are on the same node, this is a bug
		// to fix that we need to make sure that Auth server provides it's authority
		// hostname via API
		AuthorityDomain: p.teleportConfig.Auth.ClusterName.GetClusterName(),
	})
	if err != nil {
		return trace.Wrap(err)
	}
	p.proxy = teleportProxy

	creds, tlsArchive, err := p.loadRPCCredentials()
	if err != nil {
		return trace.Wrap(err, "failed to load RPC credentials")
	}
	p.rpcCreds = rpcCredentials{
		ca:     tlsArchive[pb.CA],
		client: tlsArchive[pb.Client],
		server: tlsArchive[pb.Server],
	}

	peerStore := opsservice.NewAgentPeerStore(p.backend, p.identity, p.proxy, p.WithField("process", p.id))
	p.agentServer, err = rpcserver.New(rpcserver.Config{
		Credentials: *creds,
		PeerStore:   peerStore,
	}, logrus.StandardLogger())
	if err != nil {
		return trace.Wrap(err)
	}

	// optional mirror with read only packages, used in wizard mode,
	// in this case we create a layered package service, so that writes
	// go to one directory and reads are from the other
	if p.cfg.Pack.ReadDir != "" {
		p.Debugf("Activating read layer: %v.", p.cfg.Pack.ReadDir)
		readBackend, err := keyval.NewBolt(keyval.BoltConfig{
			Path: filepath.Join(p.cfg.Pack.ReadDir, defaults.GravityDBFile),
		})
		if err != nil {
			return trace.Wrap(err)
		}
		objects, err := blobfs.New(blobfs.Config{
			Path: filepath.Join(p.cfg.Pack.ReadDir, defaults.PackagesDir),
		})
		if err != nil {
			return trace.Wrap(err)
		}
		readPackages, err := localpack.New(localpack.Config{
			Backend:     readBackend,
			UnpackedDir: filepath.Join(p.cfg.Pack.ReadDir, defaults.PackagesDir, defaults.UnpackedDir),
			Objects:     objects,
		})
		if err != nil {
			return trace.Wrap(err)
		}

		// use the packages supplied by read only mirror
		p.packages = layerpack.New(readPackages, p.packages)
	}

	seedConfig, err := p.initOpsCenterSeedConfig()
	if err != nil {
		return trace.Wrap(err)
	}
	p.Debugf("%s.", seedConfig)

	p.handlers.Packages, err = webpack.NewHandler(webpack.Config{
		Packages:      p.packages,
		Users:         p.identity,
		Authenticator: p.handlers.WebProxy.GetHandler().AuthenticateRequest,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	var client *kubernetes.Clientset
	if p.inKubernetes() {
		client, err = tryGetPrivilegedKubeClient()
		if err != nil {
			return trace.Wrap(err)
		}
		p.client = client
	} else {
		p.Debug("Not running inside Kubernetes.")
	}

	var charts helm.Repository
	switch p.cfg.Charts.Backend {
	case helm.BackendLocal:
		charts, err = helm.NewRepository(helm.Config{
			Packages: p.packages,
			Backend:  p.backend,
		})
		if err != nil {
			return trace.Wrap(err)
		}
	default:
		return trace.BadParameter("unsupported chart repository backend %q, only %q is currently supported",
			p.cfg.Charts.Backend, helm.BackendLocal)
	}

	applications, err := appservice.New(appservice.Config{
		StateDir:       filepath.Join(p.cfg.DataDir, defaults.ImportDir),
		Backend:        p.backend,
		Packages:       p.packages,
		Devmode:        p.cfg.Devmode,
		Users:          p.identity,
		Charts:         charts,
		CacheResources: true,
		UnpackedDir:    filepath.Join(p.cfg.DataDir, defaults.PackagesDir, defaults.UnpackedDir),
		GetClient:      tryGetPrivilegedKubeClient,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	p.applications = applications

	if p.inKubernetes() {
		p.handlers.Registry, err = docker.NewRegistry(docker.Config{
			Context: ctx,
			Users:   p.identity,
		})
		if err != nil {
			return trace.Wrap(err)
		}
	}

	p.handlers.Apps, err = apphandler.NewWebHandler(apphandler.WebHandlerConfig{
		Users:         p.identity,
		Applications:  applications,
		Packages:      p.packages,
		Charts:        charts,
		Authenticator: p.handlers.WebProxy.GetHandler().AuthenticateRequest,
		Devmode:       p.cfg.Devmode,
	})

	proxy := opsroute.NewClientPool(opsroute.ClientPoolConfig{
		Devmode: p.cfg.Devmode,
		Backend: p.backend,
	})

	var mon monitoring.Monitoring
	if p.inKubernetes() {
		mon, err = monitoring.NewInfluxDB(p.KubeClient())
		if err != nil {
			return trace.Wrap(err)
		}
	}
	var logs opsservice.LogForwardersControl
	if p.inKubernetes() {
		logs = opsservice.NewLogForwardersControl(client)
	}

	agentService := opsservice.NewAgentService(p.agentServer, peerStore,
		p.cfg.Pack.GetAddr().Addr, logrus.StandardLogger())
	p.agentService = agentService

	clusterClients, err := clients.NewClusterClients(clients.ClusterClientsConfig{
		Backend: p.backend,
		Tunnel:  reverseTunnel,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// start operator service and HTTP API
	operator, err := opsservice.New(opsservice.Config{
		Devmode:         p.cfg.Devmode,
		StateDir:        p.cfg.DataDir,
		Backend:         p.backend,
		Leader:          p.leader,
		Agents:          agentService,
		Clients:         clusterClients,
		Packages:        p.packages,
		Apps:            applications,
		Users:           p.identity,
		TeleportProxy:   teleportProxy,
		Tunnel:          reverseTunnel,
		Monitoring:      mon,
		Local:           p.mode == constants.ComponentSite,
		Wizard:          p.mode == constants.ComponentInstaller,
		Proxy:           proxy,
		SNIHost:         seedConfig.SNIHost,
		SeedConfig:      *seedConfig,
		ProcessID:       p.id,
		InstallLogFiles: p.cfg.InstallLogFiles,
		LogForwarders:   logs,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	if p.mode != constants.ComponentSite {
		// in case of ops center, wrap operator in a special router
		// that will route ops requests either to remote site via API
		// or local ops center using local service
		p.operator, err = opsroute.NewRouter(opsroute.RouterConfig{
			Backend: p.backend,
			Local:   operator,
			Clients: clusterClients,
			Wizard:  p.mode == constants.ComponentInstaller,
		})
		if err != nil {
			return trace.Wrap(err)
		}
		err = p.initCertificateAuthority()
		if err != nil {
			return trace.Wrap(err)
		}
	} else {
		p.operator = operator
	}

	p.handlers.Operator, err = opshandler.NewWebHandler(opshandler.WebHandlerConfig{
		Users:               p.identity,
		Operator:            p.operator,
		Applications:        applications,
		Packages:            p.packages,
		Authenticator:       p.handlers.WebProxy.GetHandler().AuthenticateRequest,
		Backend:             p.backend,
		PublicAdvertiseAddr: p.cfg.Pack.GetPublicAddr(),
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// site status checker executes status hook periodically
	p.RegisterClusterService(p.startSiteStatusChecker)

	// a few services that are running only when gravity is started in
	// local site mode
	if p.inKubernetes() {
		if p.leader == nil {
			return trace.BadParameter(
				"cluster requires backend with election capability")
		}

		p.Info("Running inside Kubernetes: starting leader election.")

		if err := p.initClusterCertificate(client); err != nil {
			return trace.Wrap(err)
		}

		if err := p.startCertificateWatch(p.context, client); err != nil {
			return trace.Wrap(err)
		}

		if err := p.startAuthGatewayWatch(p.context, client); err != nil {
			return trace.Wrap(err)
		}

		if err := p.startWatchingReloadEvents(p.context, client); err != nil {
			return trace.Wrap(err)
		}

		if err := p.startRegistrySynchronizer(p.context); err != nil {
			return trace.Wrap(err)
		}

		if err := p.startApplicationsSynchronizer(p.context); err != nil {
			return trace.Wrap(err)
		}
		if err := p.startServiceConfigWatch(p.context, client); err != nil {
			return trace.Wrap(err)
		}

		if err := p.startAutoscale(p.context); err != nil {
			return trace.Wrap(err)
		}

		if err := p.startElection(); err != nil {
			return trace.Wrap(err)
		}
	} else {
		p.Debug("Not running inside Kubernetes.")
	}

	assetsDir := p.cfg.WebAssetsDir
	if assetsDir == "" {
		assetsDir = defaults.GravityWebAssetsDir
	}
	webAssetsPackage, err := pack.FindLatestPackage(
		p.packages, loc.WebAssetsPackageLocator)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	if webAssetsPackage != nil {
		p.Infof("Unpacking assets to %v from %v.", assetsDir, webAssetsPackage)
		if err := pack.Unpack(p.packages, *webAssetsPackage, assetsDir, nil); err != nil {
			return trace.Wrap(err)
		}
	}

	forwarderConfig := web.ForwarderConfig{
		Tunnel: reverseTunnel,
	}

	if p.inKubernetes() {
		serverVersion, err := client.ServerVersion()
		if err != nil {
			return trace.Wrap(err, "failed to query kubernetes server version")
		}

		// Add compatibility to older clusters by overriding the Common Name
		// used in CSR when forwarding requests to api server
		if isLegacyKubeVersion(*serverVersion) {
			forwarderConfig.User = defaults.KubeForwarderUser
		}
	}

	forwarder, err := web.NewForwarder(forwarderConfig)
	if err != nil {
		return trace.Wrap(err)
	}

	p.handlers.Web = web.NewHandler(web.WebHandlerConfig{
		AssetsDir:      assetsDir,
		Mode:           p.mode,
		Wizard:         p.mode == constants.ComponentInstaller,
		TeleportConfig: p.teleportConfig,
		Identity:       p.identity,
		Operator:       p.operator,
		Authenticator:  p.handlers.WebProxy.GetHandler().AuthenticateRequest,
		Forwarder:      forwarder,
		Backend:        p.backend,
		Clients:        clusterClients,
	})

	p.handlers.Proxy = newProxyHandler(proxyHandlerConfig{
		tunnel:        reverseTunnel,
		operator:      p.operator,
		users:         p.identity,
		backend:       p.backend,
		authenticator: p.handlers.WebProxy.GetHandler().AuthenticateRequest,
		forwarder:     forwarder,
		devmode:       p.cfg.Devmode,
	})

	providers := web.NewProviders(applications)

	p.handlers.WebAPI, err = web.NewAPI(web.Config{
		Identity:         p.identity,
		Auth:             authClient,
		PrefixURL:        fmt.Sprintf("https://%v/portalapi/v1", p.cfg.Pack.GetAddr().Addr),
		WebAuthenticator: p.handlers.WebProxy.GetHandler().AuthenticateRequest,
		Applications:     applications,
		Packages:         p.packages,
		Backend:          p.backend,
		Operator:         p.operator,
		Providers:        providers,
		Tunnel:           reverseTunnel,
		Clients:          clusterClients,
		Converter:        ui.NewConverter(),
		Mode:             p.mode,
		ProxyHost:        sshProxyHost,
		ServiceUser:      *p.cfg.ServiceUser,
	})

	if err != nil {
		return trace.Wrap(err)
	}

	p.handlers.BLOB, err = blobhandler.New(blobhandler.Config{
		Users:   p.identity,
		Cluster: p.clusterObjects,
		Local:   p.localObjects,
	})

	if seedConfig.Account != nil {
		account := p.cfg.OpsCenter.SeedConfig.Account
		_, err = p.operator.CreateAccount(ops.NewAccountRequest{
			ID:  account.ID,
			Org: account.Org,
		})
		if err != nil {
			if !trace.IsAlreadyExists(err) {
				return trace.Wrap(err)
			}
		}
	}

	err = p.ensureClusterState()
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// Serve starts serving all process web services
func (p *Process) Serve() error {
	err := p.ServeAPI()
	if err != nil {
		return trace.Wrap(err)
	}
	err = p.ServeHealth()
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// ServeAPI starts serving process API services
func (p *Process) ServeAPI() error {
	err := p.initMux(p.context)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// ServeHealth registers the process health service with the supervisor
func (p *Process) ServeHealth() error {
	healthMux := &httprouter.Router{}
	healthMux.HandlerFunc("GET", "/readyz", p.ReportReadiness)
	healthMux.HandlerFunc("GET", "/healthz", p.ReportHealth)
	p.RegisterFunc("gravity.healthz", func() error {
		p.Infof("Start healthcheck server on %v.", p.cfg.HealthAddr)
		return trace.Wrap(http.ListenAndServe(p.cfg.HealthAddr.Addr, healthMux))
	})
	return nil
}

func tryGetPrivilegedKubeClient() (client *kubernetes.Clientset, err error) {
	_, err = utils.StatFile(constants.PrivilegedKubeconfig)
	if err == nil || !trace.IsNotFound(err) {
		client, _, err = utils.GetKubeClientFromPath(constants.PrivilegedKubeconfig)
	} else {
		client, _, err = utils.GetKubeClient("")
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return client, nil
}

func (p *Process) proxyConfig() (*proxyConfig, error) {
	opsCenterURL, err := url.ParseRequestURI(p.packages.PortalURL())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	opsCenterHostname, _, err := net.SplitHostPort(opsCenterURL.Host)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	_, proxyReverseTunnelPort, err := net.SplitHostPort(p.teleportConfig.Proxy.ReverseTunnelListenAddr.Addr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	proxyReverseTunnelAddr, err := teleutils.ParseAddr(fmt.Sprintf("%v:%v", opsCenterHostname, proxyReverseTunnelPort))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	proxyHost, proxySSHPort, err := net.SplitHostPort(p.teleportConfig.Proxy.SSHAddr.Addr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	_, proxyWebPort, err := net.SplitHostPort(p.teleportConfig.Proxy.WebAddr.Addr)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &proxyConfig{
		host:              proxyHost,
		webPort:           proxyWebPort,
		reverseTunnelAddr: *proxyReverseTunnelAddr,
		sshPort:           proxySSHPort,
	}, nil
}

func (p *Process) initMux(ctx context.Context) error {
	p.Info("Initializing mux.")

	mux := &httprouter.Router{}
	for _, method := range httplib.Methods {
		mux.Handler(method, "/web", p.handlers.Web) // to handle redirect
		mux.Handler(method, "/web/*web", p.handlers.Web)
		mux.Handler(method, "/proxy/*proxy", http.StripPrefix("/proxy", p.handlers.WebProxy))
		mux.Handler(method, "/v1/webapi/*webapi", p.handlers.WebProxy)
		mux.Handler(method, "/portalapi/v1/*portalapi", http.StripPrefix("/portalapi/v1", p.handlers.WebAPI))
		mux.Handler(method, "/sites/*rest", p.handlers.Proxy)
		mux.Handler(method, "/pack/*packages", p.handlers.Packages)
		mux.Handler(method, "/portal/*portal", p.handlers.Operator)
		mux.Handler(method, "/t/*portal", p.handlers.Operator) // shortener for instructions tokens
		mux.Handler(method, "/app/*apps", p.handlers.Apps)
		mux.Handler(method, "/telekube/*rest", p.handlers.Apps)
		mux.Handler(method, "/charts/*rest", p.handlers.Apps)
		mux.Handler(method, "/objects/*rest", p.handlers.BLOB)
		mux.Handler(method, "/v2/*rest", p.handlers.Registry)
		mux.HandlerFunc(method, "/readyz", p.ReportReadiness)
		mux.HandlerFunc(method, "/healthz", p.ReportHealth)
	}
	mux.NotFound = p.handlers.Web.NotFound

	return trace.Wrap(p.ServeLocal(ctx, httplib.GRPCHandlerFunc(
		p.agentServer, mux), p.cfg.Pack.ListenAddr.Addr))
}

// ServeLocal starts serving provided handler mux on the specified address
//
// The listener is restarted when a certificate change event is detected.
func (p *Process) ServeLocal(ctx context.Context, mux http.Handler, addr string) error {
	p.RegisterFunc("gravity.listener", func() error {
		webListener, err := p.startListening(mux, addr)
		if err != nil {
			return trace.Wrap(err)
		}

		eventsCh := make(chan service.Event)
		p.WaitForEvent(ctx, constants.ClusterCertificateUpdatedEvent, eventsCh)

		for {
			select {
			case event := <-eventsCh:
				p.Infof("Got event %q, restarting listener %v.", event, addr)

				err = webListener.Close()
				if err != nil {
					return trace.Wrap(err)
				}

				webListener, err = p.startListening(mux, addr)
				if err != nil {
					return trace.Wrap(err)
				}

			case <-ctx.Done():
				p.Infof("Stopping listener %v.", addr)
				return nil
			}
		}
	})

	return nil
}

// startListening initializes the TLS listener and starts serving on the specified
// address using the provided handler
func (p *Process) startListening(handler http.Handler, addr string) (net.Listener, error) {
	tlsConfig, err := p.getTLSConfig()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	webListener, err := tls.Listen("tcp", addr, tlsConfig)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	go func(listener net.Listener) {
		p.Infof("Serving on %v.", addr)
		err := http.Serve(listener, handler)
		if err != nil && !trace.IsEOF(err) && !utils.IsClosedConnectionError(err) {
			p.Error(trace.DebugReport(err))
		}
	}(webListener)

	return webListener, nil
}

// getTLSConfig returns the TLS config for this process.
//
// In case we're running inside Kubernetes cluster, certificate and key are
// retrieved from the cluster-tls secret. Otherwise (or if that fails) it
// falls back to self-signed certificate and key.
func (p *Process) getTLSConfig() (*tls.Config, error) {
	if p.inKubernetes() {
		config, err := p.tryGetTLSConfig()
		if err == nil {
			return config, nil
		}
		p.Errorf("Failed to load cluster certificate/key pair, falling back "+
			"to self-signed certificate. Make sure that cluster-tls secret "+
			"in kube-system namespace contains proper certificate/key pair. "+
			"The error was: %v.", trace.DebugReport(err))
	}
	cert, err := ioutil.ReadFile(p.teleportConfig.Proxy.TLSCert)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	key, err := ioutil.ReadFile(p.teleportConfig.Proxy.TLSKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	config, err := p.newTLSConfig(cert, key)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return config, nil
}

// tryGetTLSConfig returns certificate/key pair from the cluster-tls secret.
func (p *Process) tryGetTLSConfig() (*tls.Config, error) {
	client, err := tryGetPrivilegedKubeClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = p.initClusterCertificate(client)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cert, key, err := opsservice.GetClusterCertificate(client)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	config, err := p.newTLSConfig(cert, key)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return config, nil
}

// newTLSConfig builds TLS configuration from the provided cert
// and key PEM data
func (p *Process) newTLSConfig(certPEM, keyPEM []byte) (*tls.Config, error) {
	httpCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	grpcCert, err := tls.X509KeyPair(p.rpcCreds.server.CertPEM, p.rpcCreds.server.KeyPEM)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	config := &tls.Config{}

	config.GetCertificate = func(chi *tls.ClientHelloInfo) (*tls.Certificate, error) {
		if chi.ServerName == pb.ServerName {
			return &grpcCert, nil
		}
		return &httpCert, nil
	}

	config.CipherSuites = teleutils.DefaultCipherSuites()

	// Prefer the server ciphers, as curl will use invalid h2 ciphers
	// https://github.com/nghttp2/nghttp2/issues/140
	config.PreferServerCipherSuites = true
	config.MinVersion = tls.VersionTLS12
	config.SessionTicketsDisabled = false
	config.ClientSessionCache = tls.NewLRUClientSessionCache(
		teleutils.DefaultLRUCapacity)
	config.NextProtos = []string{"h2"}

	return config, nil
}

// initClusterCertificate initializes the cluster secret with certificate
// and private key
//
// It is a no-op if the secret already exists.
func (p *Process) initClusterCertificate(client *kubernetes.Clientset) error {
	site, err := p.operator.GetLocalSite()
	if err != nil {
		return trace.Wrap(err)
	}

	cert, key, err := opsservice.GetClusterCertificate(client)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	if len(cert) != 0 && len(key) != 0 {
		p.Info("Cluster certificate is already initialized.")
		return nil
	}

	p.Info("Initializing cluster certificate.")

	certificateData, err := ioutil.ReadFile(p.teleportConfig.Proxy.TLSCert)
	if err != nil {
		return trace.Wrap(err)
	}

	privateKeyData, err := ioutil.ReadFile(p.teleportConfig.Proxy.TLSKey)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = p.operator.UpdateClusterCertificate(ops.UpdateCertificateRequest{
		AccountID:   site.AccountID,
		SiteDomain:  site.Domain,
		Certificate: certificateData,
		PrivateKey:  privateKeyData,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	p.Info("Cluster certificate has been initialized.")
	return nil
}

func (p *Process) initOpsCenterSeedConfig() (*ops.SeedConfig, error) {
	if p.mode == constants.ComponentSite {
		return &ops.SeedConfig{}, nil
	}
	opsCenterURL, err := url.ParseRequestURI(p.packages.PortalURL())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	opsCenterHostname, _, err := net.SplitHostPort(opsCenterURL.Host)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	gatekeeper, err := p.upsertGatekeeperUser(opsCenterHostname)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// determine the trusted cluster name: for a regular Ops Center it's the
	// local cluster name, for a wizard it's name of the cluster being installed
	var clusterName string
	if p.mode == constants.ComponentInstaller {
		clusterName = fmt.Sprintf("%v%v", constants.InstallerTunnelPrefix,
			p.cfg.ClusterName)
	} else {
		local, err := p.backend.GetLocalSite(defaults.SystemAccountID)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		clusterName = local.Domain
	}
	// this trusted cluster represents this Ops Center
	trustedCluster := storage.NewTrustedCluster(clusterName,
		storage.TrustedClusterSpecV2{
			Enabled:              true,
			Token:                gatekeeper.Token,
			ProxyAddress:         opsCenterURL.Host,
			ReverseTunnelAddress: p.proxy.cfg.ReverseTunnelAddr.String(),
			SNIHost:              p.WebAdvertiseHost(),
			Roles:                []string{constants.RoleAdmin},
			PullUpdates:          p.mode != constants.ComponentInstaller,
			Wizard:               p.mode == constants.ComponentInstaller,
		})
	// this is the case when install has seed config,
	// happens when running install not from the opscenter,
	// seed config contains information about remote ops center
	if p.cfg.OpsCenter.SeedConfig != nil {
		// keep the trusted cluster for the wizard Ops Center so it can be
		// cleaned up after the installation
		p.cfg.OpsCenter.SeedConfig.TrustedClusters = append(
			p.cfg.OpsCenter.SeedConfig.TrustedClusters, trustedCluster)
		return p.cfg.OpsCenter.SeedConfig, nil
	}
	return &ops.SeedConfig{
		TrustedClusters: []storage.TrustedCluster{trustedCluster},
		SNIHost:         p.WebAdvertiseHost(),
	}, nil
}

// ensureSystemAccount makes sure that the system account exists in the OpsCenter
func (p *Process) ensureSystemAccount() (*users.Account, error) {
	account, err := p.identity.GetAccount(defaults.SystemAccountID)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	if account != nil {
		p.Debug("System account already exists.")
		return account, nil
	}

	account, err = p.identity.CreateAccount(users.Account{
		ID:  defaults.SystemAccountID,
		Org: defaults.SystemAccountOrg,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	p.Debugf("Created system account: %v.", account)
	return account, nil
}

// createOpsCenterUser creates ops center user that should always exist in the system
func (p *Process) createOpsCenterUser() error {
	_, err := p.identity.GetUser(constants.OpsCenterUser)
	if err == nil {
		return nil
	}
	if !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	role, err := users.NewAdminRole()
	if err != nil {
		return trace.Wrap(err)
	}
	err = p.identity.UpsertUser(
		storage.NewUser(constants.OpsCenterUser, storage.UserSpecV2{
			Type:  storage.AgentUser,
			Roles: []string{role.GetName()},
		}))
	if err != nil {
		// in case if some other process created user just before us
		if !trace.IsAlreadyExists(err) {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (p *Process) initAccount() error {
	account, err := p.ensureSystemAccount()
	if err != nil {
		return trace.Wrap(err)
	}

	// at first, create some system roles
	roles, err := users.GetBuiltinRoles()
	if err != nil {
		return trace.Wrap(err)
	}
	for i := range roles {
		err := p.identity.UpsertRole(roles[i], storage.Forever)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	if err := p.identity.Migrate(); err != nil {
		return trace.Wrap(err)
	}

	if err := p.createOpsCenterUser(); err != nil {
		return trace.Wrap(err)
	}

	// create users defined in static config
	for _, u := range p.cfg.Users {
		identities, err := u.ParsedIdentities()
		if err != nil {
			p.Errorf("%v", trace.DebugReport(err))
			return trace.Wrap(err)
		}
		p.Debugf("Creating user %q(%v, identities=%v) for org %q.", u.Email, u.Type, identities, u.Org)
		for _, role := range u.Roles {
			_, err := p.identity.GetRole(role)
			if err != nil {
				return trace.Wrap(err)
			}
		}
		err = p.identity.UpsertUser(storage.NewUser(u.Email, storage.UserSpecV2{
			AccountOwner:   u.Owner,
			Type:           u.Type,
			Password:       u.Password,
			AccountID:      account.ID,
			Roles:          u.Roles,
			OIDCIdentities: identities,
		}))
		if err != nil {
			if trace.IsAlreadyExists(err) {
				continue
			}
			return trace.Wrap(err)
		}
		// if a user has a pre-configured API key, make sure it exists
		for _, token := range u.Tokens {
			_, err = p.identity.CreateAPIKey(storage.APIKey{
				Token:     token,
				UserEmail: u.Email,
			}, false)
			if err != nil && !trace.IsAlreadyExists(err) {
				return trace.Wrap(err)
			}
		}
	}

	return nil
}

// removeLegacyIdentities removes legacy admin/proxy identities so that new
// ones can be generated upon the first start of new teleport
//
// If they are not removed, this process that includes teleport 3.0 will not
// be able to start after upgrade from older gravity that used teleport 2.4.
//
// TODO Remove after 5.4.0 LTS release.
func (p *Process) removeLegacyIdentities() {
	for _, role := range []teleport.Role{teleport.RoleAdmin, teleport.RoleProxy} {
		for _, ext := range []string{"key", "cert"} {
			os.Remove(filepath.Join(p.teleportConfig.DataDir,
				fmt.Sprintf("%s.%s", strings.ToLower(string(role)), ext)))
		}
	}
}

// ensureClusterState creates cluster state if missing (e.g. when updating
// from an older version)
func (p *Process) ensureClusterState() error {
	site, err := p.backend.GetLocalSite(defaults.SystemAccountID)
	if err != nil {
		if trace.IsNotFound(err) {
			return nil
		}
		return trace.Wrap(err)
	}

	if len(site.ClusterState.Servers) != 0 {
		p.Debug("Cluster state available.")
		return nil
	}

	siteKey := ops.SiteKey{
		AccountID:  site.AccountID,
		SiteDomain: site.Domain,
	}

	operations, err := p.operator.GetSiteOperations(siteKey)
	if err != nil {
		return trace.Wrap(err)
	}

	servers := make(map[string]storage.Server)
	// Replay each operation starting from earliest
	for i := len(operations) - 1; i >= 0; i-- {
		op := operations[i]
		switch op.Type {
		case ops.OperationInstall:
			for _, server := range op.InstallExpand.Servers {
				servers[server.Hostname] = server
			}
		case ops.OperationExpand:
			for _, added := range op.InstallExpand.Servers {
				servers[added.Hostname] = added
			}
		case ops.OperationShrink:
			for _, removed := range op.Shrink.Servers {
				delete(servers, removed.Hostname)
			}
			for _, removed := range op.Shrink.LegacyHostnames {
				delete(servers, removed)
			}
		}
	}

	state := make([]storage.Server, 0, len(servers))
	for _, server := range servers {
		state = append(state, server)
	}
	site.ClusterState.Servers = append(site.ClusterState.Servers, state...)
	_, err = p.backend.UpdateSite(*(*storage.Site)(site))
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (p *Process) upsertGatekeeperUser(opsCenterHostname string) (*users.RemoteAccessUser, error) {
	user, err := p.identity.CreateGatekeeper(users.RemoteAccessUser{
		Email:     constants.GatekeeperUser,
		OpsCenter: opsCenterHostname,
	})
	if err == nil {
		return user, nil
	}
	if err != nil && !trace.IsAlreadyExists(err) {
		return nil, trace.Wrap(err)
	}
	// Return existing user
	keys, err := p.identity.GetAPIKeys(constants.GatekeeperUser)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(keys) == 0 {
		// Instead of failing, create a new API key for the user
		key, err := p.identity.CreateAPIKey(
			storage.APIKey{UserEmail: user.Email}, false)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		keys = append(keys, *key)
	}
	return &users.RemoteAccessUser{
		Email:     constants.GatekeeperUser,
		OpsCenter: opsCenterHostname,
		Token:     keys[0].Token,
	}, nil
}

func (p *Process) getAuthPreference() (teleservices.AuthPreference, error) {
	preference, err := p.backend.GetAuthPreference()
	if err == nil {
		p.Debug("Authentication preference is already initialized.")
		return preference, nil
	}
	if !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	return modules.Get().DefaultAuthPreference(p.cfg.Mode)
}

// loginWithToken logs in the user linked to token specified with tokenID
func (p *Process) loginWithToken(tokenID string, w http.ResponseWriter, r *http.Request) {
	p.Infof("Logging in with token %v.", tokenID)
	fail := func(name string, args ...interface{}) {
		p.Errorf(name, args...)
		http.Redirect(w, r, "/web/msg/error/login_failed", http.StatusFound)
	}
	token, err := p.identity.GetInstallToken(tokenID)
	if err != nil {
		fail("ERROR logging in: %v", err)
		return
	}
	result, err := p.identity.LoginWithInstallToken(tokenID)
	if err != nil {
		fail("ERROR logging in: %v", err)
		return
	}
	if err = teleweb.SetSession(w, result.Email, result.SessionID); err != nil {
		fail("ERROR creating session %v", err)
		return
	}
	// if the token has a site domain, redirect to the site
	if token.SiteDomain != "" {
		var installed *ops.SiteOperation
		siteKey := ops.SiteKey{AccountID: token.AccountID, SiteDomain: token.SiteDomain}
		installed, err = ops.GetCompletedInstallOperation(siteKey, p.operator)
		if err != nil && !trace.IsNotFound(err) {
			fail("ERROR getting install operation %v", err)
			return
		}
		if installed != nil {
			domainPath := fmt.Sprintf("/web/site/%v", token.SiteDomain)
			http.Redirect(w, r, domainPath, http.StatusFound)
		}
	} else {
		// TODO: restrict this to only a (subset of) specific URL
		// like the one to install an application?
		http.Redirect(w, r, r.URL.Path, http.StatusFound)
	}
}

func (p *Process) loadRPCCredentials() (*rpcserver.Credentials, utils.TLSArchive, error) {
	// In case of multi-node install, a gravity-site process may need to
	// fetch a package blob from the leader which may not be fully
	// initialized yet so retry a few times.
	var reader io.ReadCloser
	err := utils.Retry(defaults.RetryInterval, defaults.RetryAttempts, func() (err error) {
		_, reader, err = p.packages.ReadPackage(loc.RPCSecrets)
		if err != nil {
			p.Warnf("Failed to read package %v: %v.", loc.RPCSecrets, trace.Wrap(err))
			return trace.Wrap(err)
		}
		return nil
	})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	defer reader.Close()

	tlsArchive, err := utils.ReadTLSArchive(reader)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	clientCreds, err := rpc.ClientCredentialsFromKeyPairs(
		*tlsArchive[pb.Client], *tlsArchive[pb.CA])
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	serverCreds, err := rpc.ServerCredentialsFromKeyPairs(
		*tlsArchive[pb.Server], *tlsArchive[pb.CA])
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return &rpcserver.Credentials{Client: clientCreds, Server: serverCreds}, tlsArchive, nil
}

// initSelfSignedHTTPSCert generates and self-signs a TLS key+cert pair for HTTPS connection
// to the proxy server.
func initSelfSignedHTTPSCert(cfg *service.Config, hostname string) (err error) {
	keyPath := filepath.Join(cfg.DataDir, teledefaults.SelfSignedKeyPath)
	certPath := filepath.Join(cfg.DataDir, teledefaults.SelfSignedCertPath)

	cfg.Proxy.TLSKey = keyPath
	cfg.Proxy.TLSCert = certPath

	// return the existing pair if they have already been generated
	_, err = tls.LoadX509KeyPair(certPath, keyPath)
	if err == nil {
		return nil
	}
	if !os.IsNotExist(err) {
		return trace.Wrap(err, "error reading certs")
	}

	hosts := []string{cfg.Hostname, "localhost"}
	if hostname != "" {
		hosts = append(hosts, hostname)
	}
	creds, err := teleutils.GenerateSelfSignedCert(hosts)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := ioutil.WriteFile(keyPath, creds.PrivateKey, defaults.PrivateFileMask); err != nil {
		return trace.Wrap(err, "error writing key PEM")
	}
	if err := ioutil.WriteFile(certPath, creds.Cert, defaults.PrivateFileMask); err != nil {
		return trace.Wrap(err, "error writing key PEM")
	}
	return nil
}

func isLegacyKubeVersion(version version.Info) bool {
	return version.Major == constants.KubeLegacyVersion.Major && version.Minor == constants.KubeLegacyVersion.Minor
}

// reverseTunnelsFromTrustedClusters creates reverse tunnels from all enabled
// trusted clusters
func reverseTunnelsFromTrustedClusters(backend storage.Backend) (tunnels []telecfg.ReverseTunnel, err error) {
	clusters, err := backend.GetTrustedClusters()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, cluster := range clusters {
		if !cluster.GetEnabled() {
			continue
		}
		tunnels = append(tunnels, telecfg.ReverseTunnel{
			DomainName: cluster.GetName(),
			Addresses:  []string{cluster.GetReverseTunnelAddress()},
		})
	}
	return tunnels, nil
}

type proxyConfig struct {
	host              string
	webPort           string
	sshPort           string
	reverseTunnelAddr teleutils.NetAddr
}
