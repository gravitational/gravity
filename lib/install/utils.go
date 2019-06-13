/*
Copyright 2019 Gravitational, Inc.

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
package install

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/checks"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/fsm"
	"github.com/gravitational/gravity/lib/install/engine"
	"github.com/gravitational/gravity/lib/install/server/dispatcher"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/process"
	"github.com/gravitational/gravity/lib/rpc"
	pb "github.com/gravitational/gravity/lib/rpc/proto"
	rpcserver "github.com/gravitational/gravity/lib/rpc/server"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/state"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/systeminfo"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/trace"
	"github.com/kardianos/osext"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/credentials"
)

// GetAppPackage finds the user app in the provided service and returns its locator
func GetAppPackage(service app.Applications) (*loc.Locator, error) {
	app, err := GetApp(service)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &app.Package, nil
}

// GetApp finds the user app in the provided service and returns it
func GetApp(service app.Applications) (*app.Application, error) {
	apps, err := service.ListApps(app.ListAppsRequest{
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

	err = InstallBinary(uid, gid, log.StandardLogger())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return user, nil
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

// LoadRPCCredentials returns the contents of the default RPC credentials package
func LoadRPCCredentials(ctx context.Context, packages pack.PackageService) (*rpcserver.Credentials, error) {
	var serverCreds, clientCreds credentials.TransportCredentials
	// FIXME: this will also mask all other possibly terminal failures (file permission
	// issues, etc.) and will keep the command blocked for the whole interval.
	// Get rid of retry or use a better error classification.
	b := utils.NewUnlimitedExponentialBackOff()
	ctx, cancel := defaults.WithTimeout(ctx)
	defer cancel()
	err := utils.RetryWithInterval(ctx, b, func() (err error) {
		serverCreds, clientCreds, err = rpc.CredentialsFromPackage(packages, loc.RPCSecrets)
		return trace.Wrap(err)
	})
	if err != nil {
		return nil, trace.Wrap(err, "failed to unpack RPC credentials")
	}
	return &rpcserver.Credentials{
		Server: serverCreds,
		Client: clientCreds,
	}, nil
}

// ClientCredentials returns the contents of the default RPC credentials package
func ClientCredentials(packages pack.PackageService) (credentials.TransportCredentials, error) {
	clientCreds, err := rpc.ClientCredentialsFromPackage(packages, loc.RPCSecrets)
	if err != nil {
		return nil, trace.Wrap(err, "failed to fetch RPC credentials")
	}
	return clientCreds, nil
}

// UpdateOperationState updates the operation data according to the agent report
func UpdateOperationState(operator ops.Operator, operation ops.SiteOperation, report ops.AgentReport) error {
	request, err := GetServerUpdateRequest(operation, report.Servers)
	if err != nil {
		return trace.Wrap(err, "failed to parse report: %#v", report)
	}
	err = operator.UpdateInstallOperationState(operation.Key(), *request)
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

// InstallBinary places the system binary into the proper binary directory
// depending on the distribution.
// The specified uid/gid pair is used to set user/group permissions on the
// resulting binary
func InstallBinary(uid, gid int, logger log.FieldLogger) (err error) {
	for _, targetPath := range state.GravityBinPaths {
		err = tryInstallBinary(targetPath, uid, gid, logger)
		if err == nil {
			break
		}
	}
	if err != nil {
		return trace.Wrap(err, "failed to install gravity binary in any of %v",
			state.GravityBinPaths)
	}
	return nil
}

// ExecuteOperation executes the operation specified with machine to completion
func ExecuteOperation(ctx context.Context, machine *fsm.FSM, logger log.FieldLogger) error {
	planErr := machine.ExecutePlan(ctx, utils.DiscardProgress)
	if planErr != nil {
		logger.WithError(planErr).Warn("Failed to execute plan.")
	}
	if err := machine.Complete(planErr); err != nil {
		logger.WithError(err).Warn("Failed to complete plan.")
	}
	if planErr != nil {
		return trace.Wrap(planErr)
	}
	return nil
}

// Run runs progress loop for the specified operation until the operation
// is complete or context is cancelled.
func (r ProgressLooper) Run(ctx context.Context) error {
	r.WithField("operation", r.OperationKey.OperationID).Info("Start progress feedback loop.")
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	var lastProgress *ops.ProgressEntry
	for {
		select {
		case <-ticker.C:
			progress, err := r.Operator.GetSiteOperationProgress(r.OperationKey)
			if err != nil {
				r.WithError(err).Warn("Failed to query operation progress.")
				continue
			}
			if lastProgress != nil && lastProgress.IsEqual(*progress) {
				continue
			}
			r.Dispatcher.Send(dispatcher.Event{Progress: progress})
			lastProgress = progress
			if isOperationSuccessful(*progress) {
				return nil
			}
		case <-ctx.Done():
			return nil
		}
	}
}

// ProgressLooper is a progress message poller
type ProgressLooper struct {
	log.FieldLogger
	Operator     ops.Operator
	OperationKey ops.SiteOperationKey
	Dispatcher   eventDispatcher
}

func isOperationSuccessful(progress ops.ProgressEntry) bool {
	return progress.IsCompleted() && progress.State == ops.OperationStateCompleted
}

type eventDispatcher interface {
	Send(dispatcher.Event)
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

func tryInstallBinary(targetPath string, uid, gid int, logger log.FieldLogger) error {
	path, err := osext.Executable()
	if err != nil {
		return trace.Wrap(err, "failed to determine path to binary")
	}
	err = os.MkdirAll(filepath.Dir(targetPath), defaults.SharedDirMask)
	if err != nil {
		return trace.Wrap(err)
	}
	err = utils.CopyFileWithPerms(targetPath, path, defaults.SharedExecutableMask)
	if err != nil {
		return trace.Wrap(err)
	}
	err = os.Chown(targetPath, uid, gid)
	if err != nil {
		return trace.Wrap(trace.ConvertSystemError(err),
			"failed to change ownership on %v", targetPath)
	}
	logger.WithField("path", targetPath).Info("Installed gravity binary.")
	return nil
}

// initOperationPlan initializes a new operation plan for the specified install operation
// in the given operator
func initOperationPlan(operator ops.Operator, planner engine.Planner) error {
	clusters, err := operator.GetSites(defaults.SystemAccountID)
	if err != nil {
		return trace.Wrap(err)
	}
	if len(clusters) != 1 {
		return trace.BadParameter("expected 1 cluster, got: %v", clusters)
	}
	operation, _, err := ops.GetInstallOperation(clusters[0].Key(), operator)
	if err != nil {
		return trace.Wrap(err)
	}
	plan, err := operator.GetOperationPlan(operation.Key())
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	if plan != nil {
		return trace.AlreadyExists("plan is already initialized")
	}
	plan, err = planner.GetOperationPlan(operator, clusters[0], *operation)
	if err != nil {
		return trace.Wrap(err)
	}
	err = operator.CreateOperationPlan(operation.Key(), *plan)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}
