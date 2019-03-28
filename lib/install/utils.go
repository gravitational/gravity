package install

import (
	"context"
	"fmt"
	"math/rand"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/process"
	"github.com/gravitational/gravity/lib/rpc"
	pb "github.com/gravitational/gravity/lib/rpc/proto"
	"github.com/kardianos/osext"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

func CheckAddr(addr string) error {
	ifaces, err := systeminfo.NetworkInterfaces()
	if err != nil {
		return trace.Wrap(err)
	}
	if len(ifaces) == 0 {
		return trace.NotFound("no network interfaces detected")
	}
	availableAddrs := make([]string, 0, len(ifaces))
	for _, iface := range ifaces {
		if iface.IPv4 == addr {
			return nil
		}
		availableAddrs = append(availableAddrs, iface.IPv4)
	}
	return trace.BadParameter(
		"%v matches none of the available addresses %v",
		addr, strings.Join(availableAddrs, ", "))
}

// GetAppPackage finds the user app in the provided service and returns its locator
func GetAppPackage(service appservice.Applications) (*loc.Locator, error) {
	app, err := GetApp(service)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &app.Package, nil
}

// GetApp finds the user app in the provided service and returns it
func GetApp(service appservice.Applications) (*appservice.Application, error) {
	apps, err := service.ListApps(appservice.ListAppsRequest{
		Repository: defaults.SystemAccountOrg,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(apps) == 0 {
		return nil, trace.NotFound("no application to install")
	}
	installApp := apps[0]
	// deps is a map of packages that appear as dependencies
	deps := make(map[loc.Locator]struct{})
	for _, app := range apps {
		for _, dep := range app.Manifest.Dependencies.Apps {
			deps[dep.Locator] = struct{}{}
		}
		base := app.Manifest.Base()
		if base != nil {
			deps[*base] = struct{}{}
		}
	}
	for _, app := range apps {
		if _, exists := deps[app.Package]; !exists {
			// Use the app that is not a dependency to any other app as one to install
			installApp = app
			break
		}
	}
	return &installApp, nil
}

// GetOrCreateServiceUser returns the user to use for container services.
//
// If the specified ID is empty, a new service user and service group
// (named defaults.ServiceUser/defaults.ServiceGroup) will be created
// with system-allocated IDs.
//
// If the specified ID is not empty, the user is expected to exist.
func GetOrCreateServiceUser(uid, gid string) (user *systeminfo.User, err error) {
	user, err = GetServiceUser(uid)
	if err == nil {
		log.Infof("System user exists: %s.", user)
		return user, nil
	}
	if !trace.IsNotFound(err) {
		return nil, trace.Wrap(err)
	}
	// Create a new user
	user, err = systeminfo.NewUser(defaults.ServiceUser, defaults.ServiceUserGroup, uid, gid)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	log.Infof("Created system user: %s.", user)
	return user, nil
}

// GetServiceUser retrieves the service user by ID.
// If the specified user ID is empty, the function looks up the user by name
// (defaults.ServiceUser).
func GetServiceUser(uid string) (user *systeminfo.User, err error) {
	if uid != "" {
		var id int
		id, err = strconv.Atoi(uid)
		if err != nil {
			return nil, trace.BadParameter("expected a numeric user ID: %v (%v)", uid, err)
		}
		user, err = systeminfo.LookupUserByUID(id)
		if err != nil && !trace.IsNotFound(err) {
			return nil, trace.Wrap(err, "failed to lookup user by ID %v", id)
		}
	} else {
		user, err = systeminfo.LookupUserByName(defaults.ServiceUser)
		if err != nil && !trace.IsNotFound(err) {
			return nil, trace.Wrap(err,
				"failed to lookup user by name %q and none was specified on command line",
				defaults.ServiceUser)
		}
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return user, nil
}

// EnsureServiceUserAndBinary ensures that the specified user exists on host
// (creating it if it does not).
// It also installs the system binary into the proper binary location
// depending on the OS distribution.
// Returns the new or existing service user as a result
func EnsureServiceUserAndBinary(userID, groupID string) (*systeminfo.User, error) {
	uid, err := strconv.Atoi(userID)
	if err != nil {
		return nil, trace.Wrap(err, "invalid numeric user ID %q for cluster", userID)
	}

	gid, err := strconv.Atoi(groupID)
	if err != nil {
		return nil, trace.Wrap(err, "invalid numeric group ID %q for cluster", groupID)
	}

	user, err := GetOrCreateServiceUser(userID, groupID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = installBinary(uid, gid)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return user, nil
}

// ValidateCloudProvider validates the value of the specified cloud provider.
// If no cloud provider has been specified, the provider is autodetected.
func ValidateCloudProvider(cloudProvider string) (provider string, err error) {
	switch cloudProvider {
	case schema.ProviderAWS, schema.ProvisionerAWSTerraform:
		if !cloudaws.IsRunningOnAWS() {
			return "", trace.BadParameter("cloud provider %q was specified "+
				"but the process does not appear to be running on an AWS "+
				"instance", cloudProvider)
		}
		return schema.ProviderAWS, nil
	case schema.ProviderGCE:
		if !gcemeta.OnGCE() {
			return "", trace.BadParameter("cloud provider %q was specified "+
				"but the process does not appear to be running on a GCE "+
				"instance", cloudProvider)
		}
		return schema.ProviderGCE, nil
	case ops.ProviderGeneric, schema.ProvisionerOnPrem:
		return schema.ProviderOnPrem, nil
	case "":
		// Detect cloud provider
		if cloudaws.IsRunningOnAWS() {
			log.Info("Detected AWS cloud provider.")
			return schema.ProviderAWS, nil
		}
		if gcemeta.OnGCE() {
			log.Info("Detected GCE cloud provider.")
			return schema.ProviderGCE, nil
		}
		log.Info("Detected onprem installation.")
		return schema.ProviderOnPrem, nil
	default:
		return "", trace.BadParameter("unsupported cloud provider %q, "+
			"supported are: %v", cloudProvider, schema.SupportedProviders)
	}
}

// FetchCloudMetadata fetches the metadata for the specified cloud provider
func FetchCloudMetadata(cloudProvider string, config *pb.RuntimeConfig) error {
	var docLink string
	switch cloudProvider {
	case schema.ProviderAWS:
		docLink = "https://gravitational.com/gravity/docs/requirements/#aws-iam-policy"
	case schema.ProviderGCE:
		docLink = "https://gravitational.com/gravity/docs/installation/#installing-on-google-compute-engine"
	default:
		return nil
	}
	metadata, err := rpcserver.GetCloudMetadata(cloudProvider)
	if err != nil {
		log.Error(trace.DebugReport(err))
		return trace.BadParameter("integration with the %v cloud provider has been "+
			"turned on but an attempt to fetch the instance metadata failed with "+
			"the following error: %q.\nCheck the documentation to see the required "+
			"instance permissions (%v) or turn off cloud integration by providing "+
			"--cloud-provider=generic flag", strings.ToUpper(cloudProvider), err, docLink)
	}
	config.CloudMetadata = metadata
	return nil
}

func LoadRPCCredentials(ctx context.Context, packages pack.PackageService, log log.FieldLogger) (*rpcserver.Credentials, error) {
	err := exportRPCCredentials(ctx, packages, log)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	serverCreds, clientCreds, err := rpc.Credentials(defaults.RPCAgentSecretsDir)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &rpcserver.Credentials{
		Server: serverCreds,
		Client: clientCreds,
	}, nil
}

func NewAgent(agentURL string, config rpcserver.PeerConfig, log log.FieldLogger) (*rpcserver.PeerServer, error) {
	log.WithField("url", agentURL).Debug("Starting agent.")
	u, err := url.ParseRequestURI(agentURL)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	serverAddr, err := teleutils.ParseAddr("tcp://" + u.Host)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	config.RuntimeConfig.Token = u.Query().Get(httplib.AccessTokenQueryParam)
	agent, err := rpcserver.NewPeer(config, serverAddr.Addr, log)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return agent, nil
}

// UpdateOperationState updates the operation data according to the agent report
func UpdateOperationState(operator ops.Operator, operation ops.SiteOperation, report ops.AgentReport) error {
	request, err := GetServerUpdateRequest(*operation, report.Servers)
	if err != nil {
		return trace.Wrap(err, "failed to parse report: %#v", report)
	}
	err = operator.UpdateInstallOperationState(opKey, *request)
	return trace.Wrap(err)
}

// GetServerUpdateRequest returns a request to update servers in given operation's state
// based on specified list of servers
func GetServerUpdateRequest(op ops.SiteOperation, servers []checks.ServerInfo) (*ops.OperationUpdateRequest, error) {
	req := ops.OperationUpdateRequest{
		Profiles: make(map[string]storage.ServerProfileRequest),
	}
	for _, serverInfo := range servers {
		if serverInfo.AdvertiseAddr == "" {
			return nil, trace.BadParameter("%v has no advertise address", serverInfo)
		}
		if serverInfo.Role == "" {
			return nil, trace.BadParameter("%v has no role", serverInfo)
		}
		var mounts []storage.Mount
		for _, mount := range serverInfo.Mounts {
			mounts = append(mounts, storage.Mount{Name: mount.Name, Source: mount.Source})
		}
		ip, _ := utils.SplitHostPort(serverInfo.AdvertiseAddr, "")
		server := storage.Server{
			AdvertiseIP: ip,
			Hostname:    serverInfo.GetHostname(),
			Role:        serverInfo.Role,
			OSInfo:      serverInfo.GetOS(),
			Mounts:      mounts,
			User:        serverInfo.GetUser(),
			Provisioner: op.Provisioner,
			Created:     time.Now().UTC(),
		}
		if serverInfo.CloudMetadata != nil {
			server.Nodename = serverInfo.CloudMetadata.NodeName
			server.InstanceType = serverInfo.CloudMetadata.InstanceType
			server.InstanceID = serverInfo.CloudMetadata.InstanceId
		}
		req.Servers = append(req.Servers, server)
		profile := req.Profiles[serverInfo.Role]
		profile.Count += 1
		req.Profiles[serverInfo.Role] = profile
	}
	return &req, nil
}

// ServerRequirements computes server requirements based on the selected flavor
func ServerRequirements(flavor schema.Flavor) map[string]storage.ServerProfileRequest {
	result := make(map[string]storage.ServerProfileRequest)
	for _, node := range flavor.Nodes {
		result[node.Profile] = storage.ServerProfileRequest{
			Count: node.Count,
		}
	}
	return result
}

// ExportRPCCredentials exports the RPC agent credentials from the specified package service
// into the default credentials directory
func ExportRPCCredentials(ctx context.Context, packages pack.PackageService, logger log.FieldLogger) error {
	// retry in a loop to account for possible transient errors, for
	// example if the target package service is still starting up.
	// Another case would be if joins are started before an installer process
	// in Ops Center-based workflow, in which case the initial package requests
	// will fail with "bad user name or password" and need to be retried.
	//
	// FIXME: this will also mask all other possibly terminal failures (file permission
	// issues, etc.) and will keep the command blocked for the whole interval.
	// Get rid of retry or use a better error classification.
	b := utils.NewUnlimitedExponentialBackOff()
	ctx = defaults.WithTimeout(ctx)
	err := utils.RetryWithInterval(ctx, b, func() error {
		err := pack.Unpack(packages, loc.RPCSecrets, defaults.RPCAgentSecretsDir, nil)
		return trace.Wrap(err)
	})
	if err != nil {
		return trace.Wrap(err, "failed to unpack RPC credentials")
	}
	logger.Info("RPC credentials unpacked.")
	return nil
}

func generateInstallToken(service ops.Operator, installToken string) (*storage.InstallToken, error) {
	token, err := service.CreateInstallToken(
		ops.NewInstallTokenRequest{
			AccountID: defaults.SystemAccountID,
			UserType:  storage.AdminUser,
			UserEmail: defaults.WizardUser,
			Token:     installToken,
		},
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return token, nil
}

func wait(ctx context.Context, cancel context.CancelFunc, p process.GravityProcess) error {
	errC := make(chan error, 1)
	go func() {
		err := p.Wait()
		cancel()
		errC <- err
	}()
	select {
	case err := <-errC:
		return trace.Wrap(err)
	case <-ctx.Done():
		return trace.Wrap(ctx.Err())
	}
}

// FIXME: move these to top-level
func newCompletedProgressEntry() *ops.ProgressEntry {
	return &ops.ProgressEntry{
		Completion: constants.Completed,
		State:      ops.ProgressStateCompleted,
	}
}

func updateProgress(progress ops.ProgressEntry, send func(Event)) {
	send(Event{Progress: &progress})
	if progress.State == ops.ProgressStateCompleted {
		log.Info("Operation completed.")
	}
	if progress.State == ops.ProgressStateFailed {
		log.Info("Operation failed.")
	}
}

func generateClusterName() string {
	rand.Seed(time.Now().UnixNano())
	return fmt.Sprintf(
		"%v%d",
		strings.Replace(namesgenerator.GetRandomName(0), "_", "", -1),
		rand.Intn(10000))
}
