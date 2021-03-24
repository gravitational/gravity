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
	"strings"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/users"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/rigging"
	"github.com/gravitational/trace"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// runClusterProvisionHook runs cluster provisioning hook defined by user
// application manifest
func (s *site) runClusterProvisionHook(ctx *operationContext) error {
	s.reportProgress(ctx, ops.ProgressEntry{
		State:      ops.ProgressStateInProgress,
		Completion: 5,
		Message:    "Running cluster provisioning hook",
	})
	// these are common environment variables valid for all provisioners
	vars := ctx.operation.InstallExpand.Vars
	env := map[string]string{
		constants.EnvTelekubeClusterName: vars.System.ClusterName,
		constants.EnvTelekubeOpsURL:      vars.System.OpsURL,
	}
	if vars.System.Devmode {
		env[constants.EnvTelekubeDevMode] = fmt.Sprintf("%v", vars.System.Devmode)
	}

	job, err := s.app.Manifest.Hooks.ClusterProvision.GetJob()
	if err != nil {
		return trace.Wrap(err)
	}

	// set profiles and roles
	var profiles []string
	for profileName, profile := range ctx.profiles() {
		profiles = append(profiles, profileName)
		env[fmt.Sprintf(constants.EnvTelekubeNodeProfileCountTemplate, profileName)] = fmt.Sprintf("%v", profile.Request.Count)
		env[fmt.Sprintf(constants.EnvTelekubeNodeProfileInstanceTypeTemplate, profileName)] = fmt.Sprintf("%v", profile.Request.InstanceType)
	}
	env[constants.EnvTelekubeNodeProfiles] = strings.Join(profiles, ",")

	request := app.HookRunRequest{
		Application: s.app.Package,
		Hook:        schema.HookClusterProvision,
		Env:         env,
	}
	secretParams := secretParams{
		token: vars.System.Token,
	}
	switch provider := s.cloudProvider().(type) {
	case *aws:
		// set up common parameters
		env[constants.EnvCloudProvider] = schema.ProviderAWS
		// setup parameters for VPC
		env[constants.EnvAWSAMI] = vars.AWS.AMI
		env[constants.EnvAWSRegion] = vars.AWS.Region
		env[constants.EnvAWSVPCID] = vars.AWS.VPCID
		env[constants.EnvAWSKeyName] = vars.AWS.KeyPair
		secretParams.aws = &awsSecrets{
			accessKeyID:     provider.accessKey,
			secretAccessKey: provider.secretKey,
			sessionToken:    provider.sessionToken,
		}
	}
	return s.runIntegrationHook(ctx, job, request, secretParams)
}

// runNodesProvisionHook runs nodes provisioning hook defined by user
// application manifest
func (s *site) runNodesProvisionHook(ctx *operationContext) error {
	s.reportProgress(ctx, ops.ProgressEntry{
		State:      ops.ProgressStateInProgress,
		Completion: 5,
		Message:    "Running nodes provisioning hook",
	})
	// these are common environment variables valid for all provisioners
	vars := ctx.operation.InstallExpand.Vars
	env := map[string]string{
		constants.EnvTelekubeClusterName: vars.System.ClusterName,
		constants.EnvTelekubeOpsURL:      vars.System.OpsURL,
	}
	if vars.System.Devmode {
		env[constants.EnvTelekubeDevMode] = fmt.Sprintf("%v", vars.System.Devmode)
	}
	site, err := s.service.GetSite(s.key)
	if err != nil {
		return trace.Wrap(err)
	}

	job, err := s.app.Manifest.Hooks.NodesProvision.GetJob()
	if err != nil {
		return trace.Wrap(err)
	}

	existingServers := site.ClusterState.ProfileMap()
	// set profiles and roles
	var profiles []string
	for profileName, profile := range ctx.profiles() {
		profiles = append(profiles, profileName)
		// count of servers to be added
		env[fmt.Sprintf(constants.EnvTelekubeNodeProfileAddCountTemplate, profileName)] = fmt.Sprintf("%v", profile.Request.Count)
		// total desired count of the servers
		env[fmt.Sprintf(constants.EnvTelekubeNodeProfileCountTemplate, profileName)] = fmt.Sprintf("%v", profile.Request.Count+len(existingServers[profileName]))
		env[fmt.Sprintf(constants.EnvTelekubeNodeProfileInstanceTypeTemplate, profileName)] = fmt.Sprintf("%v", profile.Request.InstanceType)
	}
	env[constants.EnvTelekubeNodeProfiles] = strings.Join(profiles, ",")

	request := app.HookRunRequest{
		Application: s.app.Package,
		Hook:        schema.HookNodesProvision,
		Env:         env,
	}
	secretParams := secretParams{
		token: vars.System.Token,
	}
	switch provider := s.cloudProvider().(type) {
	case *aws:
		// set up common parameters
		env[constants.EnvCloudProvider] = schema.ProviderAWS
		// setup parameters for VPC
		env[constants.EnvAWSAMI] = vars.AWS.AMI
		env[constants.EnvAWSRegion] = vars.AWS.Region
		env[constants.EnvAWSVPCID] = vars.AWS.VPCID
		env[constants.EnvAWSKeyName] = vars.AWS.KeyPair
		secretParams.aws = &awsSecrets{
			accessKeyID:     provider.accessKey,
			secretAccessKey: provider.secretKey,
			sessionToken:    provider.sessionToken,
		}
	}
	return s.runIntegrationHook(ctx, job, request, secretParams)
}

// runClusterDeprovisionHook runs cluster provisioning hook defined by user
// application manifest
func (s *site) runClusterDeprovisionHook(ctx *operationContext) error {
	s.reportProgress(ctx, ops.ProgressEntry{
		State:      ops.ProgressStateInProgress,
		Completion: 50,
		Message:    "Running cluster deprovisioning hook",
	})
	site, err := s.service.GetSite(s.key)
	if err != nil {
		return trace.Wrap(err)
	}
	// these are common environment variables valid for all deprovisioners
	vars := ctx.operation.Uninstall.Vars
	env := map[string]string{
		constants.EnvTelekubeClusterName: vars.System.ClusterName,
	}

	job, err := s.app.Manifest.Hooks.ClusterDeprovision.GetJob()
	if err != nil {
		return trace.Wrap(err)
	}

	request := app.HookRunRequest{
		Application: s.app.Package,
		Hook:        schema.HookClusterDeprovision,
		Env:         env,
	}
	secretParams := secretParams{
		token: vars.System.Token,
	}

	// set profiles and roles
	var profiles []string
	for profileName, servers := range site.ClusterState.ProfileMap() {
		profiles = append(profiles, profileName)
		env[fmt.Sprintf(constants.EnvTelekubeNodeProfileCountTemplate, profileName)] = fmt.Sprintf("%v", len(servers))
	}
	env[constants.EnvTelekubeNodeProfiles] = strings.Join(profiles, ",")

	switch provider := s.cloudProvider().(type) {
	case *aws:
		// access parameters
		env[constants.EnvCloudProvider] = schema.ProviderAWS
		// setup parameters for VPC
		env[constants.EnvAWSRegion] = vars.AWS.Region
		env[constants.EnvAWSVPCID] = vars.AWS.VPCID
		// pass provider as mounted secrets
		secretParams.aws = &awsSecrets{
			accessKeyID:     provider.accessKey,
			secretAccessKey: provider.secretKey,
			sessionToken:    provider.sessionToken,
		}
	}

	return s.runIntegrationHook(ctx, job, request, secretParams)
}

// runNodesDeprovisionHook runs hook to deprovison nodes
func (s *site) runNodesDeprovisionHook(ctx *operationContext) error {
	s.reportProgress(ctx, ops.ProgressEntry{
		State:      ops.ProgressStateInProgress,
		Completion: 50,
		Message:    "Running nodes deprovisioning hook",
	})
	site, err := s.service.GetSite(s.key)
	if err != nil {
		return trace.Wrap(err)
	}
	// these are common environment variables valid for all deprovisioners
	vars := ctx.operation.Shrink.Vars
	env := map[string]string{
		constants.EnvTelekubeClusterName: vars.System.ClusterName,
	}
	job, err := s.app.Manifest.Hooks.NodesDeprovision.GetJob()
	if err != nil {
		return trace.Wrap(err)
	}
	request := app.HookRunRequest{
		Application: s.app.Package,
		Hook:        schema.HookNodesDeprovision,
		Env:         env,
	}
	secretParams := secretParams{
		token: vars.System.Token,
	}
	// set profiles and roles
	var profiles []string
	for profileName, servers := range site.ClusterState.ProfileMap() {
		profiles = append(profiles, profileName)
		// total desired count of the servers remains the same
		env[fmt.Sprintf(constants.EnvTelekubeNodeProfileCountTemplate, profileName)] = fmt.Sprintf("%v", len(servers))
	}
	env[constants.EnvTelekubeNodeProfiles] = strings.Join(profiles, ",")
	server := ctx.serversToRemove[0]
	switch provider := s.cloudProvider().(type) {
	case *aws:
		env[constants.EnvAWSInstancePrivateIP] = server.AdvertiseIP
		env[constants.EnvAWSInstancePrivateDNS] = server.Nodename
		// access parameters
		env[constants.EnvCloudProvider] = schema.ProviderAWS
		// setup parameters for VPC
		env[constants.EnvAWSRegion] = vars.AWS.Region
		env[constants.EnvAWSVPCID] = vars.AWS.VPCID
		// pass provider as mounted secrets
		secretParams.aws = &awsSecrets{
			accessKeyID:     provider.accessKey,
			secretAccessKey: provider.secretKey,
			sessionToken:    provider.sessionToken,
		}
	}
	err = s.runIntegrationHook(ctx, job, request, secretParams)
	if err != nil {
		ctx.Infof("Integration hook stopped with error: %v", trace.DebugReport(err))
		return trace.Wrap(err)
	}
	return nil
}

// runIntegrationHook runs integration provisioning hook, sets up all secrets and deletes hook and secret after completing
func (s *site) runIntegrationHook(ctx *operationContext, job *batchv1.Job, request app.HookRunRequest, secretParams secretParams) error {
	pack, err := secret(job.Name, job.Namespace, secretParams)
	if err != nil {
		return trace.Wrap(err)
	}
	request.Volumes = []v1.Volume{*pack.volume}
	request.VolumeMounts = []v1.VolumeMount{*pack.mount}
	request.ServiceUser = s.serviceUser()

	// create secret with credentials and setup volumes
	client, err := s.service.GetKubeClient()
	if err != nil {
		return trace.Wrap(err)
	}
	_, err = client.CoreV1().Secrets(pack.secret.Namespace).
		Create(context.TODO(), pack.secret, metav1.CreateOptions{})
	if err != nil {
		return rigging.ConvertError(err)
	}
	defer func() {
		err := client.CoreV1().Secrets(pack.secret.Namespace).Delete(
			context.TODO(), pack.secret.Name, metav1.DeleteOptions{})
		if err != nil {
			ctx.Warningf("Failed to delete secret: %v", trace.DebugReport(err))
		}
	}()
	ref, err := s.appService.StartAppHook(context.TODO(), request)
	if err != nil {
		ctx.RecordError("Failed to start hook: %v.", err)
		return trace.Wrap(err)
	}
	defer func() {
		err := s.appService.DeleteAppHookJob(context.TODO(), app.DeleteAppHookJobRequest{
			HookRef: *ref,
		})
		if err != nil {
			ctx.Warningf("Failed to delete hook job: %v.", trace.DebugReport(err))
		}
	}()
	go func() {
		err := s.appService.StreamAppHookLogs(context.TODO(), *ref, ctx.recorder)
		if err != nil {
			if err != io.EOF {
				ctx.Warningf("Failed to stream logs: %v.", trace.DebugReport(err))
			}
		}
	}()
	err = utils.Retry(defaults.RetryInterval, defaults.RetryLessAttempts, func() error {
		err = s.appService.WaitAppHook(context.TODO(), *ref)
		if err != nil {
			if !trace.IsConnectionProblem(err) {
				ctx.Infof("Failed to wait for hook completion: %v.", err)
				return utils.Abort(err)
			} else {
				return utils.Continue("got a connection problem, will continue waiting")
			}
		}
		return nil
	})
	if err != nil {
		ctx.RecordError("Hook failed to execute.")
		return trace.Wrap(err)
	}
	ctx.RecordInfo("Hook completed successfully.")
	return nil
}

type secretPack struct {
	secret *v1.Secret
	volume *v1.Volume
	mount  *v1.VolumeMount
}

type secretParams struct {
	token string
	aws   *awsSecrets
}

type awsSecrets struct {
	accessKeyID     string
	secretAccessKey string
	sessionToken    string
}

// secret creates secret with AWS credentials and sets up volume and volume mount
func secret(jobName string, namespace string, params secretParams) (*secretPack, error) {
	suffix, err := users.CryptoRandomToken(3)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%v-%v", jobName, suffix),
			Namespace: namespace,
		},
		StringData: map[string]string{
			"token": params.token,
		},
	}

	if params.aws != nil {
		var tokenLine = ""
		if params.aws.sessionToken != "" {
			tokenLine = fmt.Sprintf("aws_session_token=%v", params.aws.sessionToken)
		}
		secret.StringData["aws-credentials"] = fmt.Sprintf(
			`[default]
aws_access_key_id=%v
aws_secret_access_key=%v
%v
`, params.aws.accessKeyID, params.aws.secretAccessKey, tokenLine)
	}

	volume := &v1.Volume{
		Name: "aws-credentials",
		VolumeSource: v1.VolumeSource{
			Secret: &v1.SecretVolumeSource{
				SecretName: secret.Name,
			},
		},
	}

	mount := &v1.VolumeMount{
		Name:      volume.Name,
		ReadOnly:  true,
		MountPath: constants.TelekubeMountDir,
	}

	return &secretPack{secret: secret, volume: volume, mount: mount}, nil
}
