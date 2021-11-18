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
	"github.com/gravitational/gravity/lib/install/dispatcher"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/rpc"
	pb "github.com/gravitational/gravity/lib/rpc/proto"
	rpcserver "github.com/gravitational/gravity/lib/rpc/server"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/state"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/systeminfo"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/cenkalti/backoff"
	"github.com/gravitational/trace"
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
		Type:       storage.AppUser,
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

// LoadRPCCredentials loads and validates the contents of the default RPC credentials package
func LoadRPCCredentials(ctx context.Context, packages pack.PackageService) (*rpcserver.Credentials, error) {
	tls, err := loadCredentialsFromPackage(ctx, packages, loc.RPCSecrets)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = rpc.ValidateCredentials(tls, time.Now())
	if err != nil {
		return nil, newInvalidCertError(err)
	}
	creds, err := newRPCCredentials(tls)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return creds, nil
}

// ClientCredentials returns the contents of the default RPC credentials package
func ClientCredentials(packages pack.PackageService) (credentials.TransportCredentials, error) {
	clientCreds, err := rpc.ClientCredentials(packages)
	if err != nil {
		return nil, trace.Wrap(err, "failed to fetch RPC credentials")
	}
	return clientCreds, nil
}

// UpdateOperationState updates the operation data according to the agent report
func UpdateOperationState(operator ops.Operator, operation ops.SiteOperation, report ops.AgentReport) error {
	request, err := GetServerUpdateRequest(operation, report.Servers)
	if err != nil {
		log.WithFields(log.Fields{
			log.ErrorKey: err,
			"report":     report,
		}).Warn("Failed to parse report.")
		return trace.Wrap(err)
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
		ip, _ := utils.SplitHostPort(serverInfo.AdvertiseAddr, "")
		server := storage.Server{
			AdvertiseIP: ip,
			Hostname:    serverInfo.GetHostname(),
			Role:        serverInfo.Role,
			OSInfo:      serverInfo.GetOS(),
			Mounts:      pb.MountsFromProto(serverInfo.Mounts),
			User:        serverInfo.GetUser(),
			Provisioner: op.Provisioner,
			Created:     time.Now().UTC(),
			SELinux:     serverInfo.SELinux,
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
		err = InstallBinaryInto(targetPath, logger, utils.OwnerOption(uid, gid))
		if err == nil {
			break
		}
		logger.WithFields(log.Fields{
			log.ErrorKey:  err,
			"uid":         uid,
			"gid":         gid,
			"target-path": targetPath,
		}).Warn("Failed to install binary.")
	}
	if err != nil {
		return trace.Wrap(err, "failed to install gravity binary in any of %v",
			state.GravityBinPaths)
	}
	return nil
}

// InstallBinaryIntoDefaultLocation installs the gravity binary into one of the default locations
// based on the distribution.
// Returns the path of the binary if installed successfully
func InstallBinaryIntoDefaultLocation(logger log.FieldLogger, opts ...utils.FileOption) (path string, err error) {
	var targetPath string
	for _, targetPath = range state.GravityBinPaths {
		err = InstallBinaryInto(targetPath, logger, opts...)
		if err == nil {
			break
		}
		logger.WithError(err).WithField("target-path", targetPath).Warn("Failed to install binary.")
	}
	if err != nil {
		return "", trace.Wrap(err, "failed to install gravity binary in any of %v",
			state.GravityBinPaths)
	}
	return targetPath, nil
}

// InstallBinaryInto installs this gravity binary into targetPath using given file options.
func InstallBinaryInto(targetPath string, logger log.FieldLogger, opts ...utils.FileOption) error {
	opts = append(opts, utils.PermOption(defaults.SharedExecutableMask))
	dir := filepath.Dir(targetPath)
	if !isRootDir(dir) {
		err := os.MkdirAll(dir, defaults.SharedDirMask)
		if err != nil {
			return trace.ConvertSystemError(err)
		}
	}
	err := utils.CopyFileWithOptions(targetPath, utils.Exe.Path, opts...)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// ExecuteOperation executes the operation specified with machine to completion
func ExecuteOperation(ctx context.Context, machine *fsm.FSM, progress utils.Progress, logger log.FieldLogger) error {
	planErr := machine.ExecutePlan(ctx, progress)
	if planErr != nil {
		logger.WithError(planErr).Warn("Failed to execute plan.")
	}
	err := machine.Complete(ctx, planErr)
	if err != nil {
		logger.WithError(err).Warn("Failed to complete operation.")
	}
	if planErr != nil {
		err = planErr
	}
	return trace.Wrap(err)
}

// Run runs progress loop for the specified operation until the operation
// is complete or context is cancelled.
func (r ProgressPoller) Run(ctx context.Context) error {
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
			if shouldIgnoreProgress(progress, lastProgress) {
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

func shouldIgnoreProgress(current, prev *ops.ProgressEntry) bool {
	return prev != nil && prev.IsEqual(*current) ||
		prev == nil && current.IsFailed()
}

// ProgressPoller is a progress message poller
type ProgressPoller struct {
	log.FieldLogger
	Operator     ops.Operator
	OperationKey ops.SiteOperationKey
	Dispatcher   eventDispatcher
}

// ExecResult describes the result of execution an operation (step).
// An optional completion event can describe the completion outcome to the client
type ExecResult struct {
	// CompletionEvent specifies the optional completion
	// event to send to a client
	CompletionEvent *dispatcher.Event
	// Error specifies the optional execution error
	Error error
}

// FormatAbortError formats the specified error for output by the installer client.
// Output will contain error message for err as well as any error it wraps.
func FormatAbortError(err error) string {
	return trace.UserMessage(err)
}

func isOperationSuccessful(progress ops.ProgressEntry) bool {
	return progress.IsCompleted() && progress.State == ops.OperationStateCompleted
}

type eventDispatcher interface {
	Send(dispatcher.Event)
}

// initOperationPlan initializes a new operation plan for the specified install operation
// in the given operator
func (i *Installer) initOperationPlan(key ops.SiteOperationKey) error {
	clusters, err := i.config.Operator.GetSites(defaults.SystemAccountID)
	if err != nil {
		return trace.Wrap(err)
	}
	if len(clusters) != 1 {
		return trace.BadParameter("expected 1 cluster, got: %v", clusters)
	}
	operation, err := i.config.Operator.GetSiteOperation(key)
	if err != nil {
		return trace.Wrap(err)
	}
	plan, err := i.config.Operator.GetOperationPlan(operation.Key())
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	if plan != nil {
		return trace.AlreadyExists("plan is already initialized")
	}
	plan, err = i.config.Planner.GetOperationPlan(i.config.Operator, clusters[0], *operation)
	if err != nil {
		return trace.Wrap(err)
	}
	err = i.config.Operator.CreateOperationPlan(operation.Key(), *plan)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func newRPCCredentials(tls utils.TLSArchive) (*rpcserver.Credentials, error) {
	caKeyPair, err := tls.GetKeyPair(pb.CA)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	serverKeyPair, err := tls.GetKeyPair(pb.Server)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	serverCreds, err := rpc.ServerCredentialsFromKeyPairs(*serverKeyPair, *caKeyPair)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clientKeyPair, err := tls.GetKeyPair(pb.Client)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clientCreds, err := rpc.ClientCredentialsFromKeyPairs(*clientKeyPair, *caKeyPair)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &rpcserver.Credentials{
		Server: serverCreds,
		Client: clientCreds,
	}, nil
}

func isRootDir(path string) bool {
	return len(path) == 1 && os.IsPathSeparator(path[0])
}

func loadCredentialsFromPackage(ctx context.Context, packages pack.PackageService, loc loc.Locator) (tls utils.TLSArchive, err error) {
	b := utils.NewUnlimitedExponentialBackOff()
	ctx, cancel := defaults.WithTimeout(ctx)
	defer cancel()
	err = utils.RetryWithInterval(ctx, b, func() (err error) {
		tls, err = rpc.CredentialsFromPackage(packages, loc)
		if err != nil && !trace.IsNotFound(err) {
			return &backoff.PermanentError{Err: err}
		}
		return trace.Wrap(err)
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return tls, nil
}

func newInvalidCertError(err error) error {
	return trace.BadParameter("%s. Please make sure that clocks are synchronized between the nodes "+
		"by using ntp, chrony or other time-synchronization programs.",
		trace.UserMessage(err))
}
