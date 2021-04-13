/*
Copyright 2018-2019 Gravitational, Inc.

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

package opsservice

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	appservice "github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/checks"
	"github.com/gravitational/gravity/lib/clients"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/httplib"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/modules"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/ops/events"
	"github.com/gravitational/gravity/lib/ops/monitoring"
	"github.com/gravitational/gravity/lib/ops/opsclient"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/rpc/proto"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/users"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/configure/cstrings"
	"github.com/gravitational/teleport/lib/auth"
	teleevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/reversetunnel"
	teleservices "github.com/gravitational/teleport/lib/services"
	teleutils "github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/mailgun/timetools"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
)

// Config holds configuration parameters for operator service
type Config struct {
	// StateDir is for some state is stored locally for now
	StateDir string

	// Backend is a storage backend
	Backend storage.Backend

	// Leader specifies the leader campaign implementation
	Leader storage.Leader

	// Agents service controls install agents that run on the hosts
	Agents *AgentService

	// Clients provides access to clients for remote clusters such as operator or apps
	Clients *clients.ClusterClients

	// Packages service controls release and remote access to software
	// packages
	Packages pack.PackageService

	// Apps service manages application packages
	Apps appservice.Applications

	// TeleportProxyService is a teleport proxy service
	TeleportProxy ops.TeleportProxyService

	// AuthClient is teleport auth server client.
	AuthClient *auth.Client

	// Tunnel is a reverse tunnel server providing access to remote sites
	Tunnel reversetunnel.Server

	// Users service provides access to users
	Users users.Identity

	// Metrics provides interface for cluster metrics collection.
	Metrics monitoring.Metrics

	// Clock is used to mock time in tests
	Clock timetools.TimeProvider

	// Devmode sets/removes some insecure flags acceptable for development
	Devmode bool

	// Local flag indicates whether the process is running in the local gravity site mode
	Local bool

	// Wizard flag indicates whether the process is running in wizard install mode
	Wizard bool

	// Proxy lets this ops center service communicate with other serices
	Proxy ops.Proxy

	// SNIHost if set, sets a base SNI host for APIServer
	SNIHost string

	// SeedConfig defines optional OpsCenter configuration to start with
	SeedConfig ops.SeedConfig

	// ProcessID uniquely identifies gravity process
	ProcessID string

	// PublicAddr is the operator service public advertise address
	PublicAddr teleutils.NetAddr

	// InstallLogFiles is a list of additional install log files
	// to add to install and expand operations for local troubleshooting
	InstallLogFiles []string

	// LogForwarders allows to manage log forwarders via Kubernetes config maps
	LogForwarders LogForwardersControl

	// OpenEBS provides access to managing OpenEBS in the cluster.
	OpenEBS OpenEBSControl

	// Client specifies an optional kubernetes client
	Client *kubernetes.Clientset

	// AuditLog is used to submit events to the audit log
	AuditLog teleevents.IAuditLog
}

// Operator implements Operator interface
type Operator struct {
	cfg Config

	mu sync.Mutex

	// kubeMutex manages access to the client
	kubeMutex sync.Mutex
	// kubeClient is a lazy-loaded kubernetes client
	kubeClient *kubernetes.Clientset

	// providers maps a site key to a cloud provider
	providers map[ops.SiteKey]CloudProvider

	// operationGroups maintains operation group for each site
	operationGroups map[ops.SiteKey]*operationGroup

	// FieldLogger allows this operator to log messages
	log.FieldLogger

	// cachedProvisioningTokenMutex provides a mutex on use of the cached provisioning token.
	cachedProvisioningTokenMutex sync.RWMutex
	// cachedProvisioningToken holds an in memory cache of which token is the cluster provisioning token.
	cachedProvisioningToken string
}

// New creates an instance of the Operator service
func New(cfg Config) (*Operator, error) {
	err := cfg.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	operator := &Operator{
		cfg:             cfg,
		providers:       map[ops.SiteKey]CloudProvider{},
		operationGroups: map[ops.SiteKey]*operationGroup{},
		kubeClient:      cfg.Client,
		FieldLogger:     log.WithField(trace.Component, constants.ComponentOps),
	}
	return operator, nil
}

// NewLocalOperator creates an instance of the operator service
// that is used in a restricted context to allow access to the
// up-to-date APIs (i.e. during update)
func NewLocalOperator(cfg Config) (*Operator, error) {
	err := cfg.CheckRelaxed()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &Operator{
		cfg:             cfg,
		operationGroups: map[ops.SiteKey]*operationGroup{},
		kubeClient:      cfg.Client,
		FieldLogger:     log.WithField(trace.Component, constants.ComponentOps),
	}, nil
}

func (cfg *Config) CheckAndSetDefaults() error {
	if cfg.TeleportProxy == nil {
		return trace.BadParameter("missing TeleportProxy")
	}
	if cfg.Backend == nil {
		return trace.BadParameter("missing Backend")
	}
	if cfg.Agents == nil {
		return trace.BadParameter("missing Agents")
	}
	if cfg.Packages == nil {
		return trace.BadParameter("missing Packages")
	}
	if cfg.Apps == nil {
		return trace.BadParameter("missing Apps")
	}
	if cfg.Users == nil {
		return trace.BadParameter("missing Users")
	}
	if cfg.Proxy == nil {
		return trace.BadParameter("missing Proxy")
	}
	if cfg.AuthClient == nil {
		return trace.BadParameter("missing AuthClient")
	}
	if cfg.ProcessID == "" {
		return trace.BadParameter("missing ProcessID")
	}
	if cfg.Clock == nil {
		cfg.Clock = &timetools.RealTime{}
	}
	if cfg.AuditLog == nil {
		cfg.AuditLog = teleevents.NewDiscardAuditLog()
	}
	return nil
}

func (cfg *Config) CheckRelaxed() error {
	if cfg.Backend == nil {
		return trace.BadParameter("missing Backend")
	}
	if cfg.Packages == nil {
		return trace.BadParameter("missing Packages")
	}
	if cfg.Apps == nil {
		return trace.BadParameter("missing Apps")
	}
	if cfg.Users == nil {
		return trace.BadParameter("missing Users")
	}
	if cfg.StateDir == "" {
		return trace.BadParameter("missing StateDir")
	}
	if cfg.Clock == nil {
		cfg.Clock = &timetools.RealTime{}
	}
	if cfg.AuditLog == nil {
		cfg.AuditLog = teleevents.NewDiscardAuditLog()
	}
	return nil
}

// GetConfig returns config operator was initialized with
func (o *Operator) GetConfig() Config {
	return o.cfg
}

func (o *Operator) siteDir(accountID, siteID string, additional ...string) string {
	path := []string{o.cfg.StateDir}
	if !o.cfg.Local {
		path = append(path, "accounts", accountID, "sites", siteID)
	}
	path = append(path, additional...)
	return filepath.Join(path...)
}

func (o *Operator) backend() storage.Backend {
	return o.cfg.Backend
}

func (o *Operator) leader() storage.Leader {
	return o.cfg.Leader
}

func (o *Operator) packages() pack.PackageService {
	return o.cfg.Packages
}

func (o *Operator) users() users.Identity {
	return o.cfg.Users
}

func (o *Operator) GetAccount(accountID string) (*ops.Account, error) {
	out, err := o.backend().GetAccount(accountID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	a := ops.Account(*out)
	return &a, nil
}

func (o *Operator) CreateAccount(req ops.NewAccountRequest) (*ops.Account, error) {
	out, err := o.backend().CreateAccount(storage.Account{
		ID:  req.ID,
		Org: req.Org,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	a := ops.Account(*out)
	return &a, nil
}

func (o *Operator) GetAccounts() ([]ops.Account, error) {
	accts, err := o.backend().GetAccounts()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	out := make([]ops.Account, len(accts))
	for i, a := range accts {
		out[i] = ops.Account(a)
	}
	return out, nil
}

func (o *Operator) CreateUser(req ops.NewUserRequest) error {
	if err := req.Check(); err != nil {
		return trace.Wrap(err)
	}
	if req.Type == storage.AdminUser {
		return trace.Wrap(o.cfg.Users.CreateAdmin(req.Name, req.Password))
	}
	if req.Type == storage.AgentUser {
		_, err := o.cfg.Users.CreateAgent(storage.NewUser(req.Name, storage.UserSpecV2{}))
		return trace.Wrap(err)
	}
	return trace.BadParameter("the API does not support %v user type", req.Type)
}

func (o *Operator) DeleteLocalUser(email string) error {
	err := o.cfg.Users.DeleteUser(email)
	if err != nil {
		return trace.Wrap(err)
	}
	o.Infof("Deleted user: %v.", email)
	return nil
}

func (o *Operator) CreateAPIKey(ctx context.Context, req ops.NewAPIKeyRequest) (*storage.APIKey, error) {
	key, err := o.cfg.Users.CreateAPIKey(storage.APIKey{
		UserEmail: req.UserEmail,
		Expires:   req.Expires,
		Token:     req.Token,
	}, req.Upsert)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	events.Emit(ctx, o, events.TokenCreated, events.Fields{
		events.FieldOwner: key.UserEmail,
	})
	return key, nil
}

func (o *Operator) GetAPIKeys(userEmail string) ([]storage.APIKey, error) {
	keys, err := o.cfg.Users.GetAPIKeys(userEmail)
	return keys, trace.Wrap(err)
}

func (o *Operator) DeleteAPIKey(ctx context.Context, userEmail, token string) error {
	if err := o.cfg.Users.DeleteAPIKey(userEmail, token); err != nil {
		return trace.Wrap(err)
	}
	events.Emit(ctx, o, events.TokenDeleted, events.Fields{
		events.FieldOwner: userEmail,
	})
	return nil
}

// CreateInstallToken creates a new install token for the specified request.
// If the token already exists, it returns an existing token
func (o *Operator) CreateInstallToken(req ops.NewInstallTokenRequest) (*storage.InstallToken, error) {
	if err := req.Check(); err != nil {
		return nil, trace.Wrap(err)
	}
	var application *loc.Locator
	if req.Application != "" {
		var err error
		application, err = loc.ParseLocator(req.Application)
		if err != nil {
			return nil, trace.Wrap(err, "failed to parse application package reference")
		}
	}
	token, err := o.cfg.Users.CreateInstallToken(
		storage.InstallToken{
			AccountID:   req.AccountID,
			Application: application,
			UserType:    req.UserType,
			UserEmail:   req.UserEmail,
			Token:       req.Token,
		},
	)
	if err != nil && !trace.IsAlreadyExists(err) {
		return nil, trace.Wrap(err)
	}
	if token == nil {
		token, err = o.cfg.Users.GetInstallToken(req.Token)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return token, trace.Wrap(err)
}

func (o *Operator) CreateProvisioningToken(token storage.ProvisioningToken) error {
	err := token.Check()
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = o.users().CreateProvisioningToken(token)
	if err != nil {
		return trace.Wrap(err)
	}
	o.Debugf("Created %s.", token)
	return nil
}

func (o *Operator) GetExpandToken(key ops.SiteKey) (*storage.ProvisioningToken, error) {
	o.cachedProvisioningTokenMutex.RLock()
	cachedToken := o.cachedProvisioningToken
	o.cachedProvisioningTokenMutex.RUnlock()

	if cachedToken != "" {
		// security: make sure to re-retrieve the token from the backend in case it's been updated or removed
		token, err := o.backend().GetProvisioningToken(cachedToken)
		if err != nil && !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}

		if token != nil {
			return token, nil
		}
	}

	tokens, err := o.backend().GetSiteProvisioningTokens(key.SiteDomain)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	for _, token := range tokens {
		// return long-lived join token
		if token.Type == storage.ProvisioningTokenTypeExpand && token.Expires.IsZero() {
			o.cachedProvisioningTokenMutex.Lock()
			o.cachedProvisioningToken = token.Token
			o.cachedProvisioningTokenMutex.Unlock()
			return &token, nil
		}
	}

	return nil, trace.NotFound("join token for %v not found", key.SiteDomain)
}

func (o *Operator) GetTrustedClusterToken(key ops.SiteKey) (storage.Token, error) {
	tokens, err := o.cfg.Users.GetAPIKeys(constants.GatekeeperUser)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(tokens) == 0 {
		return nil, trace.NotFound("trusted cluster token for %v not found",
			key.SiteDomain)
	}
	return storage.NewTokenFromV1(tokens[0]), nil
}

// validateNewSiteRequest makes sure that the provided request is valid
func (o *Operator) validateNewSiteRequest(req *ops.NewSiteRequest) error {
	if req.AppPackage == "" {
		return trace.BadParameter("missing AppPackage")
	}

	switch req.Provider {
	case schema.ProviderOnPrem, schema.ProviderGeneric, schema.ProviderAWS, schema.ProvisionerAWSTerraform, schema.ProviderGCE:
	default:
		if req.Provider == "" {
			return trace.BadParameter("missing Provider")
		}
		return trace.BadParameter(
			"provider %q is not supported", req.Provider)
	}

	if !cstrings.IsValidDomainName(req.DomainName) {
		return trace.BadParameter(
			"domain name should be a valid domain name, got %q", req.DomainName)
	}

	sitePackage, err := loc.ParseLocator(req.AppPackage)
	if err != nil {
		return trace.Wrap(err)
	}

	app, err := o.cfg.Apps.GetApp(*sitePackage)
	if err != nil {
		return trace.Wrap(err)
	}

	switch app.Manifest.Kind {
	case schema.KindBundle, schema.KindCluster:
	default:
		return trace.BadParameter("cannot create cluster with app of type %q", app.Manifest.Kind)
	}

	err = validateLabels(req.Labels)
	if err != nil {
		return trace.Wrap(err)
	}

	serviceUser := req.ServiceUser
	if serviceUser.IsEmpty() {
		req.ServiceUser = storage.DefaultOSUser()
	}

	if req.DNSConfig.IsEmpty() {
		req.DNSConfig = storage.DefaultDNSConfig
	}

	if req.License == "" {
		if app.RequiresLicense() {
			return trace.BadParameter("the app requires a license")
		}
		return nil
	}

	err = ops.VerifyLicense(o.packages(), req.License)
	if err != nil {
		return trace.Wrap(err, "failed to validate provided license")
	}

	return nil
}

func validateLabels(labels map[string]string) error {
	if len(labels) > defaults.MaxSiteLabels {
		return trace.BadParameter(
			"maximum %v site labels are allowed, got: %v", defaults.MaxSiteLabels, len(labels))
	}
	for k, v := range labels {
		if len(k) > defaults.MaxSiteLabelKeyLength {
			return trace.BadParameter(
				"maximum allowed site label key length is %v: %v", defaults.MaxSiteLabelKeyLength, k)
		}
		if len(v) > defaults.MaxSiteLabelValLength {
			return trace.BadParameter(
				"maximum allowed site label value length is %v: %v", defaults.MaxSiteLabelValLength, v)
		}
	}
	return nil
}

func (o *Operator) CreateSite(r ops.NewSiteRequest) (*ops.Site, error) {
	o.Infof("CreateSite(%#v).", r)
	err := o.validateNewSiteRequest(&r)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	opsCenter, err := utils.URLHostname(o.cfg.Packages.PortalURL())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	for _, cluster := range o.cfg.SeedConfig.TrustedClusters {
		// Use the Ops Center configured in seed config
		// See: https://github.com/gravitational/gravity/issues/1350
		if !cluster.GetWizard() {
			opsCenter = cluster.GetName()
			break
		}
	}

	sitePackage, err := loc.ParseLocator(r.AppPackage)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	app, err := o.cfg.Apps.GetApp(*sitePackage)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	dockerConfig := checks.DockerConfigFromSchemaValue(app.Manifest.SystemDocker())
	checks.OverrideDockerConfig(&dockerConfig, r.Docker)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	b := o.backend()

	account, err := b.GetAccount(r.AccountID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// add label "Name" if it wasn't explicitly provided in the request
	labels := r.Labels
	if labels == nil {
		labels = make(map[string]string)
	}
	if _, ok := labels[ops.SiteLabelName]; !ok {
		labels[ops.SiteLabelName] = r.DomainName
	}

	clusterData := &storage.Site{
		AccountID:     account.ID,
		Domain:        r.DomainName,
		Created:       o.cfg.Clock.UtcNow(),
		CreatedBy:     r.Email,
		State:         ops.SiteStateNotInstalled,
		Provider:      r.Provider,
		License:       r.License,
		Labels:        labels,
		Flavor:        r.Flavor,
		DisabledWebUI: r.DisabledWebUI,
		App:           app.PackageEnvelope.ToPackage(),
		Resources:     r.Resources,
		Location:      r.Location,
		ServiceUser:   r.ServiceUser,
		CloudConfig:   r.CloudConfig,
		DNSOverrides:  r.DNSOverrides,
		DNSConfig:     r.DNSConfig,
		ClusterState: storage.ClusterState{
			Docker: dockerConfig,
		},
		InstallToken: r.InstallToken,
	}
	if runtimeLoc := app.Manifest.Base(); runtimeLoc != nil {
		runtimeApp, err := o.cfg.Apps.GetApp(*runtimeLoc)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		clusterData.App.Base = runtimeApp.PackageEnvelope.ToPackagePtr()
	}

	clusterKey := ops.SiteKey{
		AccountID:  clusterData.AccountID,
		SiteDomain: clusterData.Domain,
	}
	clusterData, err = b.CreateSite(*clusterData)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer func() {
		if err == nil {
			return
		}
		if err := o.DeleteSite(clusterKey); err != nil {
			log.WithFields(log.Fields{
				log.ErrorKey: err,
				"cluster":    clusterKey,
			}).Warn("Failed to delete cluster.")
		}
	}()
	st, err := newSite(&site{
		domainName: clusterData.Domain,
		key:        ops.SiteKey{AccountID: account.ID, SiteDomain: clusterData.Domain},
		provider:   clusterData.Provider,
		service:    o,
		appService: o.cfg.Apps,
		app:        app,
		seedConfig: o.cfg.SeedConfig,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err = os.MkdirAll(st.siteDir(), defaults.SharedDirMask); err != nil {
		return nil, trace.Wrap(err)
	}

	agent, err := o.upsertAgentUsers(clusterKey, opsCenter)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = o.initTeleportAuthToken(clusterKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if r.InstallToken != "" {
		_, err = o.cfg.Users.CreateAPIKey(storage.APIKey{
			Token:     r.InstallToken,
			UserEmail: agent.GetName(),
		}, false)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	return convertSite(*clusterData, o.cfg.Apps)
}

// initTeleportAuthToken creates an auth token for teleport nodes.
func (o *Operator) initTeleportAuthToken(key ops.SiteKey) error {
	token, err := users.CryptoRandomToken(defaults.ProvisioningTokenBytes)
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = o.backend().CreateProvisioningToken(storage.ProvisioningToken{
		AccountID:  key.AccountID,
		SiteDomain: key.SiteDomain,
		Token:      token,
		Type:       storage.ProvisioningTokenTypeTeleport,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (o *Operator) upsertAgentUsers(clusterKey ops.SiteKey, opsCenter string) (agent storage.User, err error) {
	if err := o.upsertAdminAgent(clusterKey); err != nil {
		return nil, trace.Wrap(err)
	}
	agent, err = o.upsertAgent(clusterKey, opsCenter)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return agent, nil
}

// upsertAgent creates an agent user for the cluster being installed
func (o *Operator) upsertAgent(clusterKey ops.SiteKey, opsCenter string) (agent storage.User, err error) {
	agent, err = o.cfg.Users.CreateClusterAgent(clusterKey.SiteDomain,
		storage.NewUser(storage.ClusterAgent(clusterKey.SiteDomain), storage.UserSpecV2{
			AccountID: clusterKey.AccountID,
			OpsCenter: opsCenter,
		}))
	if err != nil && !trace.IsAlreadyExists(err) {
		return nil, trace.Wrap(err)
	}
	return agent, nil
}

// upsertAdminAgent creates an admin agent user for the cluster being installed
func (o *Operator) upsertAdminAgent(clusterKey ops.SiteKey) error {
	_, err := o.cfg.Users.CreateClusterAdminAgent(clusterKey.SiteDomain,
		storage.NewUser(storage.ClusterAdminAgent(clusterKey.SiteDomain), storage.UserSpecV2{
			AccountID: clusterKey.AccountID,
		}))
	if err != nil && !trace.IsAlreadyExists(err) {
		return trace.Wrap(err)
	}
	return nil
}

// GetLocalUser returns local gravity site admin
func (o *Operator) GetLocalUser(key ops.SiteKey) (storage.User, error) {
	users, err := o.cfg.Users.GetUsersByAccountID(key.AccountID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	for _, user := range users {
		if user.GetType() == storage.AdminUser {
			return user, nil
		}
	}

	return nil, trace.NotFound("no local user found for: %v", key)
}

// ResetUserPassword resets the user password and returns the new one
func (o *Operator) ResetUserPassword(req ops.ResetUserPasswordRequest) (string, error) {
	password, err := o.cfg.Users.ResetPassword(req.Email)
	return password, trace.Wrap(err)
}

func (o *Operator) GetCurrentUser() (storage.User, error) {
	return nil, trace.BadParameter("not implemented")
}

func (o *Operator) GetCurrentUserInfo() (*ops.UserInfo, error) {
	return nil, trace.BadParameter("not implemented")
}

func (o *Operator) GetClusterAgent(req ops.ClusterAgentRequest) (*storage.LoginEntry, error) {
	entry, err := storage.GetClusterAgentCreds(o.backend(), req.ClusterName,
		req.Admin)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return entry, nil
}

// GetLocalSite returns local cluster record for this Ops Center
func (o *Operator) GetLocalSite(context.Context) (*ops.Site, error) {
	record, err := o.backend().GetLocalSite(defaults.SystemAccountID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cluster, err := convertSite(*record, o.cfg.Apps)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return cluster, nil
}

// GetSiteInstructions returns shell script with instructions
// to execute for particular install agent.
// params are optional URL query parameters that can specify additional
// configuration attributes.
func (o *Operator) GetSiteInstructions(tokenID string, serverProfile string, params url.Values) (string, error) {
	err := validateGetSiteInstructions(tokenID, serverProfile, params)
	if err != nil {
		return "", trace.Wrap(err)
	}

	token, err := o.backend().GetProvisioningToken(tokenID)
	if err != nil {
		return "", trace.Wrap(err)
	}
	s, err := o.openSite(ops.SiteKey{AccountID: token.AccountID, SiteDomain: token.SiteDomain})
	if err != nil {
		return "", trace.Wrap(err)
	}
	instructions, err := s.getJoinInstructions(*token, serverProfile, params)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return instructions, nil
}

var instructionsAllowedRePattern = `^([a-zA-Z0-9\-\.\_]*)$`
var instructionsAllowedRe = regexp.MustCompile(instructionsAllowedRePattern)

// validateGetSiteInstructions validates user provided input into bash script generation for install instructions
// TODO(knisbet) we're going to try and eliminate the bash script generation based on user input, but as a
// workaround make sure we sanitize any inputs to the current function
func validateGetSiteInstructions(tokenID string, serverProfile string, params url.Values) error {
	advertiseAddr := params.Get(schema.AdvertiseAddr)
	if len(advertiseAddr) > 0 && net.ParseIP(advertiseAddr) == nil {
		return trace.Wrap(trace.BadParameter("advertise_addr does not appear to be a valid IP address")).
			AddField("advertise_addr", advertiseAddr)
	}

	if !instructionsAllowedRe.Match([]byte(tokenID)) {
		return trace.BadParameter("Token format validation failed").AddFields(trace.Fields{
			"token":       tokenID,
			"allow_regex": instructionsAllowedRePattern,
		})
	}

	if !instructionsAllowedRe.Match([]byte(serverProfile)) {
		return trace.BadParameter("ServerProfile format validation failed").AddFields(trace.Fields{
			"server_profile": serverProfile,
			"allow_regex":    instructionsAllowedRePattern,
		})
	}

	return nil
}

// SignTLSKey signs X509 Public Key with X509 certificate authority of this site
func (o *Operator) SignTLSKey(req ops.TLSSignRequest) (*ops.TLSSignResponse, error) {
	st, err := o.openSite(req.SiteKey())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	response, err := st.signTLSKey(req)
	return response, trace.Wrap(err)
}

// SignSSHKey signs SSH Public Key with teleport's certificate
func (o *Operator) SignSSHKey(req ops.SSHSignRequest) (*ops.SSHSignResponse, error) {
	if req.TTL <= 0 || req.TTL > constants.MaxInteractiveSessionTTL {
		req.TTL = constants.MaxInteractiveSessionTTL
	}
	sshCert, tlsCert, err := o.cfg.AuthClient.GenerateUserCerts(req.PublicKey, req.User, req.TTL, "")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	authorities, err := o.cfg.AuthClient.GetCertAuthorities(teleservices.HostCA, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ca, err := o.cfg.TeleportProxy.GetCertAuthority(teleservices.CertAuthID{
		Type:       teleservices.HostCA,
		DomainName: o.cfg.TeleportProxy.GetLocalAuthorityDomain(),
	}, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &ops.SSHSignResponse{
		Cert:                   sshCert,
		TrustedHostAuthorities: authorities,
		TLSCert:                tlsCert,
		CACert:                 ca.CertPEM,
	}, nil
}

func (o *Operator) GetSiteOperations(key ops.SiteKey, f ops.OperationsFilter) (ops.SiteOperations, error) {
	_, err := o.openSite(key)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	operations, err := o.backend().GetSiteOperations(key.SiteDomain)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	filtered := f.Filter(ops.SiteOperations(operations))

	return filtered, nil
}

// GetsiteOperation returns the operation information based on it's key
func (o *Operator) GetSiteOperation(key ops.SiteOperationKey) (*ops.SiteOperation, error) {
	site, err := o.openSite(ops.SiteKey{SiteDomain: key.SiteDomain, AccountID: key.AccountID})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	op, err := site.getSiteOperation(key.OperationID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return op, nil
}

// UpdateInstallOperationState updates the state of an install operation
func (o *Operator) UpdateInstallOperationState(key ops.SiteOperationKey, req ops.OperationUpdateRequest) error {
	o.Infof("UpdateInstallOperationState(%#v, %#v).", key, req)
	site, err := o.openSite(key.SiteKey())
	if err != nil {
		return trace.Wrap(err)
	}
	op, err := site.getSiteOperation(key.OperationID)
	if err != nil {
		return trace.Wrap(err)
	}
	if op.Type != ops.OperationInstall {
		return trace.BadParameter("expected %v, got: %v",
			ops.OperationInstall, op)
	}
	return trace.Wrap(site.updateOperationState(op, req))
}

// UpdateExpandOperationState updates the state of an expand operation
func (o *Operator) UpdateExpandOperationState(key ops.SiteOperationKey, req ops.OperationUpdateRequest) error {
	o.Infof("UpdateExpandOperationState(%#v, %#v).", key, req)
	site, err := o.openSite(key.SiteKey())
	if err != nil {
		return trace.Wrap(err)
	}
	op, err := site.getSiteOperation(key.OperationID)
	if err != nil {
		return trace.Wrap(err)
	}
	if op.Type != ops.OperationExpand {
		return trace.BadParameter("expected %v, got: %v",
			ops.OperationExpand, op)
	}
	return trace.Wrap(site.updateOperationState(op, req))
}

// DeleteSiteOperationState removes an unstarted operation and resets site state to active
func (o *Operator) DeleteSiteOperation(key ops.SiteOperationKey) (err error) {
	cluster, err := o.openSite(ops.SiteKey{AccountID: key.AccountID, SiteDomain: key.SiteDomain})
	if err != nil {
		return trace.Wrap(err)
	}

	err = o.backend().DeleteSiteOperation(key.SiteDomain, key.OperationID)
	// restore cluster state to "active"
	if errState := cluster.setSiteState(ops.SiteStateActive); errState != nil {
		log.Warnf("Failed to set cluster %v state to %q: %v.", cluster, ops.SiteStateActive, errState)
	}

	if o.cfg.Agents != nil {
		if err := o.cfg.Agents.StopAgents(context.TODO(), key); err != nil && !trace.IsNotFound(err) {
			log.Warnf("Failed to clean up agents for %v: %v.", key, trace.UserMessage(err))
		}
	}

	return trace.Wrap(err)
}

func (o *Operator) CreateSiteInstallOperation(ctx context.Context, r ops.CreateSiteInstallOperationRequest) (*ops.SiteOperationKey, error) {
	err := r.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	site, err := o.openSite(ops.SiteKey{AccountID: r.AccountID, SiteDomain: r.SiteDomain})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	key, err := site.createInstallOperation(ctx, r)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return key, nil
}

func (o *Operator) CreateSiteExpandOperation(ctx context.Context, r ops.CreateSiteExpandOperationRequest) (*ops.SiteOperationKey, error) {
	err := r.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	site, err := o.openSite(ops.SiteKey{AccountID: r.AccountID, SiteDomain: r.SiteDomain})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	key, err := site.createExpandOperation(ctx, r)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return key, nil
}

func (o *Operator) CreateSiteShrinkOperation(ctx context.Context, r ops.CreateSiteShrinkOperationRequest) (*ops.SiteOperationKey, error) {
	err := r.CheckAndSetDefaults()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	site, err := o.openSite(ops.SiteKey{AccountID: r.AccountID, SiteDomain: r.SiteDomain})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	key, err := site.createShrinkOperation(ctx, r)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return key, nil
}

func (o *Operator) CreateSiteAppUpdateOperation(ctx context.Context, r ops.CreateSiteAppUpdateOperationRequest) (*ops.SiteOperationKey, error) {
	site, err := o.openSite(ops.SiteKey{AccountID: r.AccountID, SiteDomain: r.SiteDomain})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	key, err := site.createUpdateOperation(ctx, r)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return key, nil
}

func (o *Operator) CreateSiteUninstallOperation(ctx context.Context, r ops.CreateSiteUninstallOperationRequest) (*ops.SiteOperationKey, error) {
	site, err := o.openSite(ops.SiteKey{AccountID: r.AccountID, SiteDomain: r.SiteDomain})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if o.cfg.Local {
		// if we're a cluster, create uninstall operation in the Ops Center we're connected to
		return site.requestUninstall(ctx, r)
	}
	key, err := site.createUninstallOperation(ctx, r)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return key, nil
}

// CreateClusterGarbageCollectOperation creates a new garbage collection operation in the cluster
func (o *Operator) CreateClusterGarbageCollectOperation(ctx context.Context, r ops.CreateClusterGarbageCollectOperationRequest) (*ops.SiteOperationKey, error) {
	err := r.Check()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cluster, err := o.openSite(ops.SiteKey{AccountID: r.AccountID, SiteDomain: r.ClusterName})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	key, err := cluster.createGarbageCollectOperation(ctx, r)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return key, nil
}

func (o *Operator) SetOperationState(ctx context.Context, key ops.SiteOperationKey, req ops.SetOperationStateRequest) error {
	o.Infof("%#v", req)
	site, err := o.openSite(key.SiteKey())
	if err != nil {
		return trace.Wrap(err)
	}
	// change the state without "compare" part just to take leverage of
	// the operation group locking to ensure atomicity
	_, err = site.compareAndSwapOperationState(ctx, swap{
		key:        key,
		newOpState: req.State,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	if req.Progress != nil {
		err := o.CreateProgressEntry(key, *req.Progress)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (o *Operator) GetSiteInstallOperationAgentReport(ctx context.Context, key ops.SiteOperationKey) (*ops.AgentReport, error) {
	return o.getSiteOperationAgentReport(ctx, key)
}

func (o *Operator) GetSiteExpandOperationAgentReport(ctx context.Context, key ops.SiteOperationKey) (*ops.AgentReport, error) {
	return o.getSiteOperationAgentReport(ctx, key)
}

func (o *Operator) getSiteOperationAgentReport(ctx context.Context, key ops.SiteOperationKey) (*ops.AgentReport, error) {
	cluster, err := o.openSite(ops.SiteKey{AccountID: key.AccountID, SiteDomain: key.SiteDomain})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	op, err := cluster.getSiteOperation(key.OperationID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	opCtx, err := cluster.newOperationContext(*op)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer opCtx.Close()
	return cluster.agentReport(ctx, opCtx)
}

func (o *Operator) SiteInstallOperationStart(key ops.SiteOperationKey) error {
	site, err := o.openSite(ops.SiteKey{AccountID: key.AccountID, SiteDomain: key.SiteDomain})
	if err != nil {
		return trace.Wrap(err)
	}
	err = site.executeOperation(key, site.installOperationStart)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (o *Operator) SiteExpandOperationStart(key ops.SiteOperationKey) error {
	site, err := o.openSite(ops.SiteKey{AccountID: key.AccountID, SiteDomain: key.SiteDomain})
	if err != nil {
		return trace.Wrap(err)
	}
	err = site.executeOperation(key, site.expandOperationStart)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (o *Operator) GetSiteOperationLogs(key ops.SiteOperationKey) (io.ReadCloser, error) {
	site, err := o.openSite(key.SiteKey())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return site.getOperationLogs(key)
}

// CreateLogEntry appends the provided log entry to the operation's log file
func (o *Operator) CreateLogEntry(key ops.SiteOperationKey, entry ops.LogEntry) error {
	site, err := o.openSite(key.SiteKey())
	if err != nil {
		return trace.Wrap(err)
	}
	return site.createLogEntry(key, entry)
}

func (o *Operator) GetSiteReport(ctx context.Context, req ops.GetClusterReportRequest) (io.ReadCloser, error) {
	cluster, err := o.openSite(req.SiteKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return cluster.getClusterReport(ctx, req.Since)
}

func (o *Operator) GetSiteOperationProgress(key ops.SiteOperationKey) (*ops.ProgressEntry, error) {
	pe, err := o.backend().GetLastProgressEntry(key.SiteDomain, key.OperationID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	progressEntry := ops.ProgressEntry(*pe)
	if progressEntry.Step == 0 {
		progressEntry.Step = progressEntry.Completion / 11
	}
	return &progressEntry, nil
}

func (o *Operator) CreateProgressEntry(key ops.SiteOperationKey, entry ops.ProgressEntry) error {
	_, err := o.backend().CreateProgressEntry(storage.ProgressEntry(entry))
	if err != nil {
		return trace.Wrap(err)
	}
	o.Debugf("Created: %#v.", entry)
	return nil
}

func (o *Operator) DeleteSite(key ops.SiteKey) error {
	st, err := o.openSite(key)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := st.deleteSite(); err != nil {
		return trace.Wrap(err)
	}
	o.Infof("Cluster deleted: %q.", key.String())
	return nil
}

func (o *Operator) GetSiteByDomain(domainName string) (*ops.Site, error) {
	st, err := o.backend().GetSite(domainName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return convertSite(*st, o.cfg.Apps)
}

func (o *Operator) GetSite(key ops.SiteKey) (*ops.Site, error) {
	st, err := o.backend().GetSite(key.SiteDomain)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return convertSite(*st, o.cfg.Apps)
}

func (o *Operator) GetSites(accountID string) ([]ops.Site, error) {
	sts, err := o.backend().GetSites(accountID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sites := make([]ops.Site, len(sts))
	for i, st := range sts {
		s, err := convertSite(st, o.cfg.Apps)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		sites[i] = *s
	}
	return sites, nil
}

// DeactivateSite puts the site in the degraded state and, if requested,
// stops an application.
func (o *Operator) DeactivateSite(req ops.DeactivateSiteRequest) error {
	cluster, err := o.cfg.Backend.GetSite(req.SiteDomain)
	if err != nil {
		return trace.Wrap(err)
	}

	if cluster.State == ops.SiteStateDegraded {
		return nil // nothing to do
	}

	o.Infof("Deactivating cluster %v with reason %q.",
		cluster.Domain, req.Reason)

	cluster.State = ops.SiteStateDegraded
	cluster.Reason = req.Reason

	_, err = o.cfg.Backend.UpdateSite(*cluster)
	if err != nil {
		return trace.Wrap(err)
	}

	if !req.StopApp {
		return nil // nothing to do anymore
	}

	site, err := o.openSite(ops.SiteKey{
		AccountID:  cluster.AccountID,
		SiteDomain: cluster.Domain,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	if site.app.Manifest.HasHook(schema.HookStop) {
		_, _, err = appservice.RunAppHook(context.TODO(), o.cfg.Apps,
			appservice.HookRunRequest{
				Application: cluster.App.Locator(),
				Hook:        schema.HookStop,
				ServiceUser: cluster.ServiceUser,
			})
		return trace.Wrap(err)
	}

	return nil
}

// ActivateSite moves site to the active state and, if requested, starts
// an application.
func (o *Operator) ActivateSite(req ops.ActivateSiteRequest) error {
	cluster, err := o.cfg.Backend.GetSite(req.SiteDomain)
	if err != nil {
		return trace.Wrap(err)
	}

	if cluster.State == ops.SiteStateActive {
		return nil // nothing to do
	}

	o.Infof("Activating cluster %v.", cluster.Domain)

	cluster.State = ops.SiteStateActive
	cluster.Reason = ""

	_, err = o.cfg.Backend.UpdateSite(*cluster)
	if err != nil {
		return trace.Wrap(err)
	}

	if !req.StartApp {
		return nil // nothing to do anymore
	}

	site, err := o.openSite(ops.SiteKey{
		AccountID:  cluster.AccountID,
		SiteDomain: cluster.Domain,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	if site.app.Manifest.HasHook(schema.HookStart) {
		_, _, err = appservice.RunAppHook(context.TODO(), o.cfg.Apps,
			appservice.HookRunRequest{
				Application: cluster.App.Locator(),
				Hook:        schema.HookStart,
				ServiceUser: cluster.ServiceUser,
			})
		return trace.Wrap(err)
	}

	return nil
}

// CompleteFinalInstallStep marks the site as having completed the mandatory last installation step
func (o *Operator) CompleteFinalInstallStep(req ops.CompleteFinalInstallStepRequest) error {
	if err := req.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	o.Debugf("%#v", req)
	// destroy the reverse tunnel connection that the installed cluster is
	// holding to the installer process, otherwise the cluster will keep
	// trying to connect back even after the installer has shut down
	if err := o.removeWizardConnection(req.WizardConnectionTTL); err != nil {
		return trace.Wrap(err)
	}
	// mark cluster install step as completed
	cluster, err := o.cfg.Backend.GetSite(req.SiteDomain)
	if err != nil {
		return trace.Wrap(err)
	}
	cluster.FinalInstallStepComplete = true
	if _, err := o.cfg.Backend.UpdateSite(*cluster); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// removeWizardConnection removes reverse tunnel from this cluster to the
// installer wizard process if there's any
func (o *Operator) removeWizardConnection(delay time.Duration) error {
	cluster, err := storage.GetWizardTrustedCluster(o.backend())
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	if cluster != nil {
		return storage.DisableAccess(o.backend(), cluster.GetName(), delay)
	}
	return nil
}

func (o *Operator) ValidateDomainName(domainName string) error {
	if _, err := o.backend().GetSite(domainName); err != nil {
		if trace.IsNotFound(err) {
			return nil
		}
		return trace.Wrap(err)
	}
	return trace.AlreadyExists("site with domain name %q already exists", domainName)
}

func (o *Operator) ValidateRemoteAccess(req ops.ValidateRemoteAccessRequest) (*ops.ValidateRemoteAccessResponse, error) {
	site, err := o.openSite(req.SiteKey())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return site.validateRemoteAccess(req)
}

// GetAppInstaller returns an installer tarball for application specified with locator
func (o *Operator) GetAppInstaller(req ops.AppInstallerRequest) (io.ReadCloser, error) {
	account, err := o.GetAccount(req.AccountID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	caCert := req.CACert
	if caCert == "" {
		ca, err := pack.ReadCertificateAuthority(o.cfg.Packages)
		if err != nil && !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
		if ca != nil {
			caCert = string(ca.CertPEM)
		}
	}

	var cluster storage.TrustedCluster
	if len(o.cfg.SeedConfig.TrustedClusters) != 0 {
		cluster = o.cfg.SeedConfig.TrustedClusters[0]
	}

	return o.cfg.Apps.GetAppInstaller(appservice.InstallerRequest{
		Account:        (storage.Account)(*account),
		Application:    req.Application,
		TrustedCluster: cluster,
		CACert:         caCert,
		EncryptionKey:  req.EncryptionKey,
	})
}

// GetClusterNodes returns a real-time information about cluster nodes
func (o *Operator) GetClusterNodes(key ops.SiteKey) ([]ops.Node, error) {
	nodes, err := o.backend().GetNodes(defaults.Namespace)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var result []ops.Node
	for _, node := range nodes {
		labels := node.GetAllLabels()
		result = append(result, ops.Node{
			Hostname:     labels[ops.Hostname],
			AdvertiseIP:  labels[ops.AdvertiseIP],
			PublicIP:     labels[defaults.TeleportPublicIPv4Label],
			Profile:      labels[ops.AppRole],
			InstanceType: labels[ops.InstanceType],
			Role:         labels[schema.ServiceLabelRole],
		})
	}
	return result, nil
}

// EmitAuditEvent saves the provided event in the audit log.
func (o *Operator) EmitAuditEvent(ctx context.Context, req ops.AuditEventRequest) error {
	err := req.Check()
	if err != nil {
		return trace.Wrap(err)
	}
	o.Infof("%s.", req)
	if o.cfg.AuditLog != nil {
		err = o.cfg.AuditLog.EmitAuditEvent(req.Event, req.Fields)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

// GetVersion returns the gravity binary version information.
func (o *Operator) GetVersion(ctx context.Context) (*proto.Version, error) {
	version := modules.Get().Version()
	return &version, nil
}

func (o *Operator) openSite(key ops.SiteKey) (*site, error) {
	site, err := o.backend().GetSite(key.SiteDomain)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return o.openSiteInternal(site)
}

func (o *Operator) openSiteInternal(data *storage.Site) (*site, error) {
	sitePackage, err := loc.NewLocator(data.App.Repository, data.App.Name, data.App.Version)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	app, err := o.cfg.Apps.GetApp(*sitePackage)
	if err != nil && !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}

	if trace.IsNotFound(err) {
		log.Error(trace.DebugReport(err))
		app = appservice.Phony
	}

	st, err := newSite(&site{
		service:     o,
		key:         ops.SiteKey{AccountID: data.AccountID, SiteDomain: data.Domain},
		domainName:  data.Domain,
		provider:    data.Provider,
		license:     data.License,
		app:         app,
		appService:  o.cfg.Apps,
		seedConfig:  o.cfg.SeedConfig,
		backendSite: data,
	})

	return st, trace.Wrap(err)
}

// isAWSProvisioner returns true if the provisioner is using AWS
func isAWSProvisioner(provisioner string) bool {
	return provisioner == schema.ProvisionerAWSTerraform || provisioner == schema.ProviderAWS
}

// setCloudProviderFromRequest creates an instance of CloudProvider based on specified
// details.
// variables defines the set of provider-specific details and is extracted from
// the corresponding request.
// Note, that the method might remove certain details from the variables depending
// on the provider.
func (o *Operator) setCloudProviderFromRequest(siteKey ops.SiteKey, provisioner string, variables *storage.OperationVariables) error {
	switch provisioner {
	case schema.ProvisionerAWSTerraform, schema.ProviderAWS:
		accessKey := variables.AWS.AccessKey
		secretKey := variables.AWS.SecretKey
		sessionToken := variables.AWS.SessionToken
		region := variables.AWS.Region
		if region == "" {
			return trace.BadParameter("provide AWS region parameter")
		}
		variables.AWS.AccessKey = ""
		variables.AWS.SecretKey = ""
		variables.AWS.SessionToken = ""
		cloudProvider := &aws{
			accessKey:    accessKey,
			secretKey:    secretKey,
			sessionToken: sessionToken,
			regionName:   region,
			provider:     schema.ProviderAWS,
		}
		o.setCloudProvider(siteKey, cloudProvider)
	}
	return nil
}

func (o *Operator) getCloudProvider(key ops.SiteKey) CloudProvider {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.providers[key]
}

func (o *Operator) setCloudProvider(key ops.SiteKey, cloudProvider CloudProvider) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.providers[key] = cloudProvider
}

func (o *Operator) deleteCloudProvider(key ops.SiteKey) {
	o.mu.Lock()
	defer o.mu.Unlock()
	delete(o.providers, key)
}

func (o *Operator) getOperationGroup(key ops.SiteKey) *operationGroup {
	o.mu.Lock()
	defer o.mu.Unlock()
	if _, ok := o.operationGroups[key]; !ok {
		o.operationGroups[key] = &operationGroup{operator: o, siteKey: key}
	}
	return o.operationGroups[key]
}

// RemoteOpsClient returns remote Ops Center client using the provided trusted
// cluster token for authentication
func (o *Operator) RemoteOpsClient(cluster teleservices.TrustedCluster) (*opsclient.Client, error) {
	client, err := opsclient.NewBearerClient(
		fmt.Sprintf("https://%v", cluster.GetProxyAddress()),
		cluster.GetToken(),
		opsclient.HTTPClient(httplib.GetClient(o.cfg.Devmode)))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return client, nil
}

// Lock locks the operator mutex
func (o *Operator) Lock() {
	o.mu.Lock()
}

// Unlock unlocks the operator mutex
func (o *Operator) Unlock() {
	o.mu.Unlock()
}
