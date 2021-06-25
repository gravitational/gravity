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

package ops

import (
	"context"
	"os"

	"github.com/gravitational/gravity/lib/cloudprovider/aws"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/ec2rolecreds"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

func getAWSRegion() string {
	return os.Getenv(constants.EnvAWSRegion)
}

func getAWSCreds() (*credentials.Value, error) {
	// first look for creds in the environment variables
	creds, err := credentials.NewEnvCredentials().Get()
	if err == nil {
		log.Debug("Found AWS credentials in environment.")
		return &creds, nil
	}
	// next look in the .aws config file
	creds, err = credentials.NewSharedCredentials("", os.Getenv(constants.EnvAWSProfile)).Get()
	if err == nil {
		log.Debug("Found AWS credentials in config file.")
		return &creds, nil
	}
	// if we're on a ec2 instance, retrieve creds from metadata API
	running, err := aws.IsRunningOnAWS()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if running {
		session, err := session.NewSession()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		creds, err = ec2rolecreds.NewCredentials(session).Get()
		if err == nil {
			log.Debug("Found AWS credentials using ec2 metadata API.")
			return &creds, nil
		}
	}
	// there're no creds anywhere and we're not on AWS
	return nil, trace.NotFound("failed to retrieve AWS credentials, please "+
		"configure them as described in %v or run this command from a ec2 "+
		"instance with access to the metadata API", awsCredsURL)
}

// CreateCluster is a shortcut function to create clusters,
// works for AWS only at the moment. If successful returns key to a started
// install operation.
func CreateCluster(operator Operator, clusterI storage.Cluster) (*SiteOperationKey, error) {
	cluster, ok := clusterI.(*storage.ClusterV2)
	if !ok {
		return nil, trace.BadParameter("unsupported cluster type: %T", clusterI)
	}
	if cluster.Spec.Provider != constants.CloudProviderAWS {
		return nil, trace.BadParameter("creating non-AWS clusters is not supported via tele tool yet")
	}

	cred, err := getAWSCreds()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	userInfo, err := operator.GetCurrentUserInfo()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var vars storage.OperationVariables
	if cluster.Spec.AWS.Region == "" {
		cluster.Spec.AWS.Region = getAWSRegion()
	}
	vars.AWS.Region = cluster.Spec.AWS.Region
	vars.AWS.VPCID = cluster.Spec.AWS.VPC
	vars.AWS.KeyPair = cluster.Spec.AWS.KeyName
	vars.AWS.AccessKey = cred.AccessKeyID
	vars.AWS.SecretKey = cred.SecretAccessKey
	vars.AWS.SessionToken = cred.SessionToken

	updateReq := OperationUpdateRequest{
		Profiles: map[string]storage.ServerProfileRequest{},
	}
	for _, nodeSpec := range cluster.Spec.Nodes {
		_, ok := updateReq.Profiles[nodeSpec.Profile]
		if ok {
			return nil, trace.BadParameter(
				"profile %q appears more than once in cluster spec, specify each profile once when creating a cluster",
				nodeSpec.Profile)
		}
		updateReq.Profiles[nodeSpec.Profile] = storage.ServerProfileRequest{
			InstanceType: nodeSpec.InstanceType,
			Count:        nodeSpec.Count,
		}
	}
	appPackage, err := loc.MakeLocator(cluster.Spec.App)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	req := NewSiteRequest{
		DomainName: cluster.Metadata.Name,
		AppPackage: appPackage.String(),
		AccountID:  defaults.SystemAccountID,
		Email:      userInfo.User.GetName(),
		Provider:   cluster.Spec.Provider,
		Location:   cluster.Spec.AWS.Region,
		Labels:     cluster.Metadata.Labels,
		Resources:  []byte(cluster.GetResources()),
		License:    cluster.GetLicense(),
	}
	site, err := operator.CreateSite(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	opReq := CreateSiteInstallOperationRequest{
		SiteDomain:  site.Domain,
		AccountID:   site.AccountID,
		Provisioner: cluster.Spec.Provider,
		Variables:   vars,
	}
	key, err := operator.CreateSiteInstallOperation(context.TODO(), opReq)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err = operator.UpdateInstallOperationState(*key, updateReq); err != nil {
		return nil, trace.Wrap(err)
	}
	err = operator.SiteInstallOperationStart(*key)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return key, nil
}

// GetClusters returns cluster or list of clusters
func GetClusters(operator Operator, clusterName string) ([]storage.Cluster, error) {
	sites, err := fetchSites(operator, clusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var out []storage.Cluster
	for _, site := range sites {
		op, _, err := GetInstallOperation(site.Key(), operator)
		if err != nil {
			log.Warningf("failed to get install operation for %v: %v", site.Key(), trace.DebugReport(err))
			continue
		}
		cluster := newClusterFromSite(site, op.InstallExpand.Vars)
		out = append(out, cluster)
	}
	return out, nil
}

// RemoveCluster starts cluster removal process, returns operation key
func RemoveCluster(operator Operator, clusterName string) (*SiteOperationKey, error) {
	if clusterName == "" {
		return nil, trace.BadParameter("provide cluster name")
	}
	clusters, err := GetClusters(operator, clusterName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(clusters) == 0 {
		return nil, trace.NotFound("no clusters found")
	}
	return RemoveClusterByCluster(operator, clusters[0])
}

// RemoveClusterByCluster launches uninstall operation for the provided cluster
func RemoveClusterByCluster(operator Operator, cluster storage.Cluster) (*SiteOperationKey, error) {
	creds, err := getAWSCreds()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if cluster.GetProvider() != constants.CloudProviderAWS {
		return nil, trace.BadParameter("deleting non-AWS clusters is not supported via tele tool yet")
	}
	return operator.CreateSiteUninstallOperation(context.TODO(), CreateSiteUninstallOperationRequest{
		AccountID:  defaults.SystemAccountID,
		SiteDomain: cluster.GetName(),
		Force:      true,
		Variables: storage.OperationVariables{
			AWS: storage.AWSVariables{
				Region:       cluster.GetRegion(),
				AccessKey:    creds.AccessKeyID,
				SecretKey:    creds.SecretAccessKey,
				SessionToken: creds.SessionToken,
			},
		},
	})
}

func fetchSites(operator Operator, clusterName string) ([]Site, error) {
	if clusterName == "" {
		return operator.GetSites(defaults.SystemAccountID)
	}
	site, err := operator.GetSite(SiteKey{AccountID: defaults.SystemAccountID, SiteDomain: clusterName})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return []Site{*site}, nil
}

// newClusterFromSite creates cluster resource from Site and install operation variables
func newClusterFromSite(site Site, vars storage.OperationVariables) storage.Cluster {
	cluster := NewClusterFromSite(site)
	if site.Provider == constants.CloudProviderAWS {
		cluster.Spec.AWS = &storage.ClusterAWSProviderSpecV2{
			Region:  vars.AWS.Region,
			VPC:     vars.AWS.VPCID,
			KeyName: vars.AWS.KeyPair,
		}
	}
	return cluster
}

// awsCredsURL is the URL to the AWS article about configuring CLI credentials
const awsCredsURL = "https://docs.aws.amazon.com/cli/latest/userguide/cli-chap-getting-started.html"
