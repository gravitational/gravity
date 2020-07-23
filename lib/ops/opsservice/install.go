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

package opsservice

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"time"

	libapp "github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/checks"
	awsapi "github.com/gravitational/gravity/lib/cloudprovider/aws"
	awssvc "github.com/gravitational/gravity/lib/cloudprovider/aws/service"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/httplib"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/users"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/dustin/go-humanize"
	licenseapi "github.com/gravitational/license"
	"github.com/gravitational/trace"
	"github.com/pborman/uuid"
	log "github.com/sirupsen/logrus"
)

// StreamOperationLogs appends the logs from the provided reader to the
// specified operation (user-facing) log file
func (o *Operator) StreamOperationLogs(key ops.SiteOperationKey, reader io.Reader) error {
	site, err := o.openSite(key.SiteKey())
	if err != nil {
		return trace.Wrap(err)
	}
	writer, err := site.newOperationRecorder(key, o.cfg.InstallLogFiles...)
	if err != nil {
		return trace.Wrap(err)
	}
	defer writer.Close()
	n, err := io.Copy(writer, reader)
	if err != nil {
		return trace.Wrap(err)
	}
	o.Infof("Uploaded operation log %v, written %v.",
		key, humanize.Bytes(uint64(n)))
	return nil
}

// createInstallOperation initiates install operation for a given site
// it makes sure that install operation is the first operation too
func (s *site) createInstallOperation(ctx context.Context, req ops.CreateSiteInstallOperationRequest) (*ops.SiteOperationKey, error) {
	profiles := make(map[string]storage.ServerProfile)
	for _, profile := range s.app.Manifest.NodeProfiles {
		profiles[profile.Name] = storage.ServerProfile{
			Description: profile.Description,
			Labels:      profile.Labels,
			ServiceRole: string(profile.ServiceRole),
			Request:     req.Profiles[profile.Name],
		}
	}
	return s.createInstallExpandOperation(ctx, createInstallExpandOperationRequest{
		Type:        ops.OperationInstall,
		State:       ops.OperationStateInstallInitiated,
		Provisioner: req.Provisioner,
		Vars:        req.Variables,
		Profiles:    profiles,
	})
}

type createInstallExpandOperationRequest struct {
	Type        string
	State       string
	Provisioner string
	Vars        storage.OperationVariables
	Profiles    map[string]storage.ServerProfile
}

func (s *site) createInstallExpandOperation(context context.Context, req createInstallExpandOperationRequest) (*ops.SiteOperationKey, error) {
	s.WithField("req", req).Debug("createInstallExpandOperation.")
	operationType := req.Type
	operationInitialState := req.State
	provisioner := req.Provisioner
	variables := req.Vars
	profiles := req.Profiles

	op := &ops.SiteOperation{
		ID:          uuid.New(),
		AccountID:   s.key.AccountID,
		SiteDomain:  s.key.SiteDomain,
		Type:        operationType,
		Created:     s.clock().UtcNow(),
		CreatedBy:   storage.UserFromContext(context),
		Updated:     s.clock().UtcNow(),
		State:       operationInitialState,
		Provisioner: provisioner,
	}

	token, err := s.newProvisioningToken(*op)
	if err != nil && !trace.IsAlreadyExists(err) {
		log.WithError(err).Warn("Failed to create provisioning token.")
		return nil, trace.Wrap(err)
	}
	log.WithField("token", token).Info("Create install operation.")

	ctx, err := s.newOperationContext(*op)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer ctx.Close()

	err = s.updateRequestVars(ctx, &variables, op)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = s.service.setCloudProviderFromRequest(s.key, provisioner, &variables)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if isAWSProvisioner(op.Provisioner) {
		err := s.verifyPermissionsAWS(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	systemVars, err := s.systemVars(*op, variables.System)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	variables.System = *systemVars
	agents := make(map[string]storage.AgentProfile, len(profiles))
	for role := range profiles {
		instructions, err := s.getDownloadInstructions(token, role)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		u, err := url.Parse(fmt.Sprintf("agent://%v/%v", s.service.cfg.Agents.ServerAddr(), role))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		q := u.Query()
		q.Set(httplib.AccessTokenQueryParam, token)
		if s.cloudProviderName() == schema.ProviderAWS {
			q.Set(ops.AgentProvisioner, schema.ProvisionerAWSTerraform)
		}
		u.RawQuery = q.Encode()
		agents[role] = storage.AgentProfile{
			Instructions: instructions,
			AgentURL:     u.String(),
			Token:        token,
		}
	}

	op.InstallExpand = &storage.InstallExpandOperationState{
		Vars:     variables,
		Agents:   agents,
		Profiles: profiles,
		Package:  s.app.Package,
	}

	subnets, err := s.selectSubnets(*op)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	op.InstallExpand.Subnets = *subnets
	ctx.Debugf("selected subnets: %v", subnets)

	key, err := s.getOperationGroup().createSiteOperation(*op)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	s.reportProgress(ctx, ops.ProgressEntry{
		State:   ops.ProgressStateInProgress,
		Message: "Operation has been created",
	})

	return key, nil
}

func (s *site) selectSubnets(operation ops.SiteOperation) (*storage.Subnets, error) {
	// for expand operation, get them from the completed install operation
	if operation.Type == ops.OperationExpand {
		op, err := ops.GetCompletedInstallOperation(s.key, s.service)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return &op.InstallExpand.Subnets, nil
	}

	// we do not control subnets for onprem scenario so just use default subnets
	if !(s.cloudProviderName() == schema.ProviderAWS && operation.Provisioner == schema.ProvisionerAWSTerraform) {
		overlaySubnet := operation.InstallExpand.Vars.OnPrem.PodCIDR
		if overlaySubnet == "" {
			overlaySubnet = defaults.PodSubnet
		}
		serviceSubnet := operation.InstallExpand.Vars.OnPrem.ServiceCIDR
		if serviceSubnet == "" {
			serviceSubnet = defaults.ServiceSubnet
		}
		return &storage.Subnets{
			Overlay: overlaySubnet,
			Service: serviceSubnet,
		}, nil
	}

	// machines on AWS will receive IPs from this subnet
	subnet := operation.GetVars().AWS.VPCCIDR
	if subnet == "" {
		return nil, trace.BadParameter("no subnet CIDR in operation vars: %v", operation)
	}

	// select the overlay subnet non-overlapping with the machines subnet
	overlay, err := utils.SelectSubnet([]string{subnet})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// and select the service subnet non-overlapping with the pod/machine subnets
	service, err := utils.SelectSubnet([]string{subnet, overlay})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &storage.Subnets{
		Overlay: overlay,
		Service: service,
	}, nil
}

// updateRequestVars updates AWS variables from a site create/expand request with defaults.
// In case of an install, if a VPC ID has been specified, a subnet from that VPC is automatically selected.
// In case of an expand, it will also retrieve region/vpc details from a previous install operation.
func (s *site) updateRequestVars(ctx *operationContext, vars *storage.OperationVariables, op *ops.SiteOperation) error {
	// after the site has been installed, all further operations should derive their AWS variables
	// such as region or VPC ID from the install's variables
	if op.Type != ops.OperationInstall {
		installOperation, _, err := ops.GetInstallOperation(s.key, s.service)
		if err != nil {
			return trace.Wrap(err)
		}
		installVars := installOperation.GetVars()
		vars.AWS.Region = installVars.AWS.Region
		if installVars.AWS.VPCID != "" {
			vars.AWS.VPCID = installVars.AWS.VPCID
		}
		if installVars.AWS.KeyPair != "" {
			vars.AWS.KeyPair = installVars.AWS.KeyPair
		}
		if installVars.OnPrem.VxlanPort != 0 {
			vars.OnPrem.VxlanPort = installVars.OnPrem.VxlanPort
		}
	}

	if !isAWSProvisioner(op.Provisioner) {
		return nil
	}

	awsClient := awssvc.New(vars.AWS.AccessKey, vars.AWS.SecretKey, vars.AWS.SessionToken)

	region := vars.AWS.Region
	if mapping, ok := awsapi.Regions[awsapi.RegionName(region)]; ok {
		vars.AWS.AMI = mapping.Image
	}

	// pick a subnet when installing into an existing VPC
	vpcID := vars.AWS.VPCID
	if vpcID != "" && op.Type == ops.OperationInstall {
		vpcBlock, subnetBlocks, err := awsClient.GetCIDRBlocks(region, vpcID)
		if err != nil {
			return trace.Wrap(err)
		}
		ctx.Infof("found subnets for %v/%v (%v): %v", region, vpcID, vpcBlock, subnetBlocks)

		freeSubnet, err := utils.SelectVPCSubnet(vpcBlock, subnetBlocks)
		if err != nil {
			return trace.Wrap(err)
		}
		ctx.Infof("selected /24 subnet for %v/%v: %v", region, vpcID, freeSubnet)
		vars.AWS.SubnetCIDR = freeSubnet

		igwID, err := awsClient.GetInternetGatewayID(region, vpcID)
		if err != nil {
			return trace.Wrap(err)
		}
		ctx.Infof("found internet gateway for %v/%v: %v", region, vpcID, igwID)
		vars.AWS.InternetGatewayID = igwID
	}

	return nil
}

func (s *site) updateOperationState(op *ops.SiteOperation, req ops.OperationUpdateRequest) (err error) {
	ctx, err := s.newOperationContext(*op)
	if err != nil {
		return trace.Wrap(err)
	}
	defer ctx.Close()

	ctx.Infof("updateOperationState(%#v)", req)

	cluster, err := s.backend().GetSite(s.key.SiteDomain)
	if err != nil {
		return trace.Wrap(err)
	}

	var oldStates []string
	var newState string
	switch op.Type {
	case ops.OperationInstall:
		oldStates = []string{
			ops.OperationStateInstallInitiated,
			ops.OperationStateInstallProvisioning,
			ops.OperationStateReady,
			ops.OperationStateInstallPrechecks,
			ops.OperationStateFailed,
		}
		newState = ops.OperationStateInstallPrechecks
	case ops.OperationExpand:
		oldStates = []string{
			ops.OperationStateExpandInitiated,
			ops.OperationStateExpandProvisioning,
			ops.OperationStateReady,
			ops.OperationStateExpandPrechecks,
			ops.OperationStateFailed,
		}
		newState = ops.OperationStateExpandPrechecks
	default:
		return trace.BadParameter("unexpected operation type %v", op.Type)
	}

	op, err = s.compareAndSwapOperationState(swap{
		key:            op.Key(),
		expectedStates: oldStates,
		newOpState:     newState,
	})
	if err != nil {
		if trace.IsCompareFailed(err) {
			log.WithError(err).Warn("Failed to sync operation state.")
			err = trace.BadParameter("internal operation state out of sync")
		}
		return trace.Wrap(err)
	}

	// if prechecks fail, reset the operation state back to "initiated"
	defer func() {
		if err != nil {
			_, casErr := s.compareAndSwapOperationState(swap{
				key:            op.Key(),
				expectedStates: []string{newState},
				newOpState:     oldStates[0],
			})
			if casErr != nil {
				log.WithFields(log.Fields{
					log.ErrorKey: casErr,
					"op":         op.ID,
					"to-state":   oldStates[0],
				}).Warn("Failed to reset operation state.")
			}
		}
	}()

	switch op.Type {
	case ops.OperationInstall:
		err = s.validateInstall(op, &req)
	case ops.OperationExpand:
		err = s.validateExpand(op, &req)
	}
	if err != nil {
		return trace.Wrap(err)
	}

	// check that the requested instance type (if any) is valid for the operation
	err = s.checkInstanceType(ctx, cluster, op, req)
	if err != nil {
		return trace.Wrap(err)
	}

	state := op.InstallExpand

	// update operation state with requested server profiles
	for role, profileRequest := range req.Profiles {
		// find the server profile with the given role in manifest
		profile, err := s.app.Manifest.NodeProfiles.ByName(role)
		if err != nil {
			return trace.Wrap(err)
		}

		// update the operation state with the proper profile
		state.Profiles[role] = storage.ServerProfile{
			Description: profile.Description,
			Labels:      profile.Labels,
			ServiceRole: string(profile.ServiceRole),
			Request: storage.ServerProfileRequest{
				InstanceType: profileRequest.InstanceType,
				Count:        profileRequest.Count,
			},
		}
	}

	infos, err := s.agentService().GetServerInfos(context.TODO(), op.Key())
	if err != nil {
		return trace.Wrap(err)
	}

	if req.ValidateServers {
		validateReq := ops.ValidateServersRequest{
			AccountID:   cluster.AccountID,
			SiteDomain:  cluster.Domain,
			OperationID: op.ID,
			Servers:     req.Servers,
		}
		err = ValidateServers(context.TODO(), s.service, validateReq)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	// check if the customer-provided license is valid for this operation
	err = s.checkLicense(cluster, op, req, infos)
	if err != nil {
		return trace.Wrap(err)
	}

	systemVars, err := s.systemVars(*op, state.Vars.System)
	if err != nil {
		return trace.Wrap(err)
	}
	state.Vars.System = *systemVars

	servers := req.Servers
	if op.Provisioner == schema.ProvisionerOnPrem {
		servers, err = s.configureOnPremServers(ctx, req.Servers, infos)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	op.InstallExpand.Servers = req.Servers
	op.Servers = servers

	_, err = s.updateSiteOperation(op)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (s *site) validateInstall(op *ops.SiteOperation, req *ops.OperationUpdateRequest) error {
	// for onprem installation verify whether provided servers satisfy the selected flavor
	if op.Provisioner == schema.ProvisionerOnPrem {
		err := s.checkOnPremServers(*req)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	err := setClusterRoles(req.Servers, *s.app, 0)
	return trace.Wrap(err)
}

// checkOnPremServers checks that onprem servers in the provided request satisfy profiles in
// the same request
func (s *site) checkOnPremServers(req ops.OperationUpdateRequest) error {
	roleToCount := make(map[string]int)
	for _, server := range req.Servers {
		roleToCount[server.Role] += 1
	}

	// verify that we have exactly the amount of servers of a certain role as dictated by flavor
	for role, profileRequest := range req.Profiles {
		count := roleToCount[role]
		if count < profileRequest.Count {
			return trace.BadParameter(
				"selected flavor needs %v %q nodes, run the above command on %v more node(-s)",
				profileRequest.Count, role, profileRequest.Count-count)
		} else if count > profileRequest.Count {
			return trace.BadParameter(
				"selected flavor needs %v %q nodes, stop agents on %v extra node(-s)",
				profileRequest.Count, role, count-profileRequest.Count)
		}
	}

	for _, server := range req.Servers {
		_, ok := req.Profiles[server.Role]
		// verify that there are no servers with unknown roles
		if !ok {
			return trace.BadParameter("unknown server role %v for %v", server.Role, server)
		}
	}

	return nil
}

// checkInstanceType checks that the requested instance type is valid for the operation
func (s *site) checkInstanceType(ctx *operationContext, site *storage.Site, op *ops.SiteOperation, req ops.OperationUpdateRequest) error {
	// the instance type checks makes sense only for a cloud provider
	if !isAWSProvisioner(op.Provisioner) {
		return nil
	}

	// for both install and expand make sure requested instance types are supported in the site's region
	for _, profileRequest := range req.Profiles {
		if !awsapi.SupportsInstanceType(site.Location, profileRequest.InstanceType) {
			return trace.BadParameter("instance type %v is not supported in %v",
				profileRequest.InstanceType, site.Location)
		}
	}

	// the check for a "fixed" instance type is valid only for expand operation
	if op.Type != ops.OperationExpand {
		return nil
	}

	installOperation, err := ops.GetCompletedInstallOperation(s.key, s.service)
	if err != nil {
		return trace.Wrap(err)
	}

	installState := installOperation.InstallExpand
	if installState == nil {
		return trace.BadParameter("install operation does not have state: %v", installOperation)
	}

	for role, profileRequest := range req.Profiles {
		profile, err := s.app.Manifest.NodeProfiles.ByName(role)
		if err != nil {
			return trace.Wrap(err)
		}

		if profile.ExpandPolicy != schema.ExpandPolicyFixedInstance {
			// the app does not require fixed instance type for this server role
			continue
		}

		installProfile, ok := installState.Profiles[role]
		if !ok {
			// no servers of this profile are provisioned, this should not happen but let's
			// be nice and ignore if it does happen
			ctx.Warningf("no install profile for role %v found: %v, ignoring", role, installState)
			continue
		}

		// let's find out what instance type was provisioned during install
		provisionedType := installProfile.Request.InstanceType
		if provisionedType != profileRequest.InstanceType {
			return trace.BadParameter("must use %v instance type for servers of role '%v'",
				provisionedType, role)
		}
	}

	return nil
}

// checkLicense checks that the license is valid for the current operation
func (s *site) checkLicense(site *storage.Site, op *ops.SiteOperation, req ops.OperationUpdateRequest, infos checks.ServerInfos) error {
	if site.License == "" {
		return nil // nothing to do
	}

	license, err := licenseapi.ParseLicense(site.License)
	if err != nil {
		return trace.Wrap(err, "failed to parse license")
	}

	// for on-prem and AWS scenarios the license is checked a little bit differently
	if op.Provisioner == schema.ProvisionerAWSTerraform {
		err = s.checkLicenseAWS(license, op)
	}
	if op.Provisioner == schema.ProvisionerOnPrem {
		err = s.checkLicenseOnPrem(license, op, req, infos)
	}

	return trace.Wrap(err)
}

func (s *site) checkLicenseAWS(license licenseapi.License, op *ops.SiteOperation) error {
	count, err := s.numExistingServers(op)
	if err != nil {
		return trace.Wrap(err)
	}

	for _, profile := range op.InstallExpand.Profiles {
		count += profile.Request.Count
	}

	// make sure that the license permits the total number of servers
	err = license.GetPayload().CheckCount(count)
	if err != nil {
		return trace.Wrap(err)
	}

	var instanceTypes []string
	for _, profile := range op.InstallExpand.Profiles {
		instanceTypes = append(instanceTypes, profile.Request.InstanceType)
	}

	// make sure the the license permits all requested AWS instance types
	err = license.GetPayload().CheckInstanceTypes(instanceTypes)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (s *site) checkLicenseOnPrem(license licenseapi.License, op *ops.SiteOperation, req ops.OperationUpdateRequest, infos checks.ServerInfos) error {
	count, err := s.numExistingServers(op)
	if err != nil {
		return trace.Wrap(err)
	}

	count += len(req.Servers)

	// make sure that the license permits the total number of servers
	err = license.GetPayload().CheckCount(count)
	if err != nil {
		return trace.Wrap(err)
	}

	// make sure that the license permits on-prem server configurations
	for _, info := range infos {
		err = checkLicenseCPU(license.GetPayload(), info.GetNumCPU())
		if err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// configureOnPremServers configures already active (onprem) servers by querying and storing
// remote system details from the agents into the operation state and configuring the system state
// directory unless it has already been created.
// Returns the list of servers to set as structured operation state.
// Modifies remoteServers with details obtained from corresponding agents in-place.
func (s *site) configureOnPremServers(ctx *operationContext, servers []storage.Server, infos checks.ServerInfos) (updated []storage.Server, err error) {
	updated = make([]storage.Server, 0, len(servers))
	for i, server := range servers {
		info, err := infos.FindByIP(server.AdvertiseIP)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		systemDevice := server.SystemState.Device.Name
		if systemDevice.Path() == "" {
			systemDevice = storage.DeviceName(info.SystemDevice)
		}
		servers[i].SystemState.Device = info.GetDevices().GetByName(systemDevice)
		servers[i].SystemState.StateDir = info.StateDir
		servers[i].User = info.GetUser()
		servers[i].Provisioner = schema.ProvisionerOnPrem
		servers[i].Created = time.Now().UTC()

		if info.CloudMetadata != nil {
			servers[i].Nodename = info.CloudMetadata.NodeName
			servers[i].InstanceType = info.CloudMetadata.InstanceType
			servers[i].InstanceID = info.CloudMetadata.InstanceId
		}

		updated = append(updated, servers[i])
	}

	return updated, nil
}

// addClusterStateServers adds the provided servers to the cluster state
func (s *site) addClusterStateServers(servers []storage.Server) error {
	return trace.Wrap(s.getOperationGroup().addClusterStateServers(servers))
}

// removeClusterStateServers removes servers with the specified hostnames from the cluster state
func (s *site) removeClusterStateServers(hostnames []string) error {
	return trace.Wrap(s.getOperationGroup().removeClusterStateServers(hostnames))
}

// waitForInstaller waits until the remote installer process establishes reverse
// tunnel to this Ops Center and returns its operator
//
// If this process is installer itself, the local operator service is returned
// right away.
func (s *site) waitForInstaller(ctx *operationContext) (ops.Operator, error) {
	if s.service.cfg.Wizard {
		// we are the installer process ourselves
		return s.service, nil
	}
	localCtx, cancel := defaults.WithTimeout(context.TODO())
	defer cancel()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			installer, err := s.service.cfg.Clients.OpsClient(
				constants.InstallerClusterName(s.domainName))
			if err == nil {
				ctx.Infof("Got installer client.")
				return installer, nil
			}
			ctx.Debugf("Failed to get installer client: %v.", err)
		case <-localCtx.Done():
			return nil, trace.LimitExceeded("timeout waiting for installer")
		}
	}
}

// waitForNodes periodically queries the agent report from the remote installer
// process using its provided operator and returns when the report contains
// sufficient number of nodes for the installation
func (s *site) waitForNodes(ctx *operationContext, installer ops.Operator) error {
	localCtx, cancel := defaults.WithTimeout(context.TODO())
	defer cancel()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			report, err := installer.GetSiteInstallOperationAgentReport(ctx.key())
			if err != nil {
				ctx.Warnf("Failed to get agent report: %v.", err)
				continue
			}
			ctx.Infof("Got installer agent report: %v.", report)
			if len(report.Servers) == ctx.getNumServers() {
				ctx.Infof("All agents joined, can continue: %v.", report)
				return nil
			} else {
				ctx.Infof("Not all agents joined yet: %v.", report)
			}
		case <-localCtx.Done():
			return trace.LimitExceeded("timeout waiting for nodes")
		}
	}
}

// waitForOperation periodically queries the operation status and returns
// when the operation completes
func (s *site) waitForOperation(ctx *operationContext) error {
	localCtx, cancel := context.WithTimeout(context.TODO(),
		defaults.InstallApplicationTimeout)
	defer cancel()
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			operation, err := s.service.GetSiteOperation(ctx.key())
			if err != nil {
				ctx.Warnf("Failed to get operation: %v.", err)
				continue
			}
			if !operation.IsFinished() {
				ctx.Infof("Operation %v/%v is still in progress.",
					operation.Type, operation.SiteDomain)
				continue
			}
			ctx.Infof("Operation has finished: %v.", operation)
			return nil
		case <-localCtx.Done():
			return trace.LimitExceeded("timeout waiting for operation")
		}
	}
}

// installOperationStart kicks off actual installation process:
// resource provisioning, package configuration and deployment
func (s *site) installOperationStart(ctx *operationContext) error {
	op, err := s.compareAndSwapOperationState(swap{
		key: ctx.key(),
		expectedStates: []string{
			ops.OperationStateInstallInitiated,
			ops.OperationStateInstallPrechecks,
		},
		newOpState: ops.OperationStateInstallProvisioning,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	if isAWSProvisioner(op.Provisioner) {
		if !s.app.Manifest.HasHook(schema.HookClusterProvision) {
			return trace.BadParameter("%v hook is not defined",
				schema.HookClusterProvision)
		}
		ctx.Info("Using cluster provisioning hook.")
		err := s.runClusterProvisionHook(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
		ctx.RecordInfo("Infrastructure has been successfully provisioned.")
	}

	s.reportProgress(ctx, ops.ProgressEntry{
		State:   ops.ProgressStateInProgress,
		Message: "Waiting for the provisioned nodes to come up",
	})

	installer, err := s.waitForInstaller(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	if isAWSProvisioner(op.Provisioner) {
		err := s.waitForNodes(ctx, installer)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	s.reportProgress(ctx, ops.ProgressEntry{
		State:   ops.ProgressStateInProgress,
		Message: "All servers are up",
	})

	_, err = s.compareAndSwapOperationState(swap{
		key:            ctx.key(),
		expectedStates: []string{ops.OperationStateInstallProvisioning},
		newOpState:     ops.OperationStateInstallDeploying,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// give the installer a green light
	err = installer.SetOperationState(ctx.key(),
		ops.SetOperationStateRequest{
			State: ops.OperationStateReady,
		})
	if err != nil {
		return trace.Wrap(err)
	}

	// wait for it to finish, installer will be reporting progress to us
	err = s.waitForOperation(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func (s *site) newProvisioningToken(operation ops.SiteOperation) (token string, err error) {
	agentUser, err := s.agentUser()
	if err != nil {
		return "", trace.Wrap(err)
	}
	if operation.Type == ops.OperationInstall {
		// For installation, the install token that was specified on the command line
		// (or automatically generated during initialization), becomes both the auth token
		// for the agents and the provisioning token to assign the agent to an operation
		token = s.installToken()
	}
	if token == "" {
		token, err = users.CryptoRandomToken(defaults.ProvisioningTokenBytes)
		if err != nil {
			return "", trace.Wrap(err)
		}
	}
	tokenRequest := storage.ProvisioningToken{
		Token:      token,
		AccountID:  s.key.AccountID,
		SiteDomain: s.key.SiteDomain,
		// Always create an expand token
		Type:        storage.ProvisioningTokenTypeExpand,
		OperationID: operation.ID,
		UserEmail:   agentUser.GetName(),
	}
	if operation.Type == ops.OperationExpand {
		tokenRequest.Expires = s.clock().UtcNow().Add(defaults.ExpandTokenTTL)
	}
	_, err = s.users().CreateProvisioningToken(tokenRequest)
	if err != nil {
		return token, trace.Wrap(err)
	}
	return token, nil
}

// checkLicenseCPU checks if the license supports the provided number of CPUs.
func checkLicenseCPU(p licenseapi.Payload, numCPU uint) error {
	if p.MaxCores != 0 && numCPU > uint(p.MaxCores) {
		return trace.BadParameter(
			"the license allows maximum of %v CPUs, requested: %v", p.MaxCores, numCPU)
	}
	return nil
}

// setClusterRoles assigns cluster roles to servers.
func setClusterRoles(servers []storage.Server, app libapp.Application, masters int) error {
	// count the number of servers designated as master by the node profile
	for _, server := range servers {
		profile, err := app.Manifest.NodeProfiles.ByName(server.Role)
		if err != nil {
			return trace.Wrap(err)
		}

		if profile.ServiceRole == schema.ServiceRoleMaster {
			masters++
		}
	}

	// assign the servers to their rolls
	for i, server := range servers {
		profile, err := app.Manifest.NodeProfiles.ByName(server.Role)
		if err != nil {
			return trace.Wrap(err)
		}
		switch profile.ServiceRole {
		case "":
			if masters < defaults.MaxMasterNodes {
				servers[i].ClusterRole = string(schema.ServiceRoleMaster)
				masters++
			} else {
				servers[i].ClusterRole = string(schema.ServiceRoleNode)
			}
		case schema.ServiceRoleMaster:
			servers[i].ClusterRole = string(schema.ServiceRoleMaster)
			// don't increment masters as this server has already been counted above
		case schema.ServiceRoleNode:
			servers[i].ClusterRole = string(schema.ServiceRoleNode)
		default:
			return trace.BadParameter(
				"unknown cluster role %q for node profile %q",
				profile.ServiceRole, server.Role)
		}
	}
	return nil
}
