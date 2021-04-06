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

package aws

import (
	"github.com/gravitational/trace"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/credentials/ec2rolecreds"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
)

// Instance defines an AWS instance and provides
// access to basic attributes such as instance id, the name of the node
// and the list of instance tags
type Instance struct {
	// ID is AWS instance id
	ID string
	// NodeName is private DNS name of the instance
	NodeName string
	// Type is instance type
	Type string
	// self references the AWS EC2 instance
	self *ec2.Instance
	// Region is AWS region of the instance
	Region string
	// PrivateIP is a private instance IPv4
	PrivateIP string
	// PublicIP is the instance's assigned public IP
	PublicIP string
}

// IsRunningOnAWS indicates if the current running process appears to be running
// on an AWS instance by checking the availability of the AWS metadata API
func IsRunningOnAWS() (bool, error) {
	session, err := session.NewSession()
	if err != nil {
		return false, trace.Wrap(err)
	}
	metadata := ec2metadata.New(session)
	return metadata.Available(), nil
}

// NewLocalInstance creates a new Instance describing the AWS instance
// we are running on
func NewLocalInstance() (*Instance, error) {
	session, err := session.NewSession()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	metadata := ec2metadata.New(session)
	creds := credentials.NewCredentials(&credentials.ChainProvider{
		VerboseErrors: true,
		Providers: []credentials.Provider{
			&ec2rolecreds.EC2RoleProvider{Client: metadata},
		},
	})

	instanceID, err := metadata.GetMetadata("instance-id")
	if err != nil {
		return nil, trace.Wrap(err, "failed to fetch instance-id from ec2 metadata service")
	}

	zone, err := getAvailabilityZone(metadata)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(zone) == 0 {
		return nil, trace.NotFound("could not determine availability zone")
	}

	regionName := zone[:len(zone)-1]
	ec2 := ec2.New(session, &aws.Config{
		Region:      &regionName,
		Credentials: creds,
		// Enable verbose error logging
		CredentialsChainVerboseErrors: aws.Bool(true),
	})

	ec2instance, err := getInstanceInfo(ec2, instanceID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// The metadata service returns the wrong hostname in case of a private DNS zone,
	// use PrivateDnsName from AWS EC2 API for private DNS zone support.
	// See https://github.com/kubernetes/kubernetes/issues/11543
	privateDNSName := aws.StringValue(ec2instance.PrivateDnsName)
	if privateDNSName == "" {
		return nil, trace.BadParameter("empty private DNS name in instance data")
	}

	instance := &Instance{
		self:      ec2instance,
		ID:        instanceID,
		Type:      aws.StringValue(ec2instance.InstanceType),
		NodeName:  privateDNSName,
		Region:    regionName,
		PrivateIP: aws.StringValue(ec2instance.PrivateIpAddress),
		PublicIP:  aws.StringValue(ec2instance.PublicIpAddress),
	}
	return instance, nil
}

// Tag returns the value of the tag specified with name
// If the tag is not found, an empty string is returned
func (r *Instance) Tag(name string) string {
	for _, tag := range r.self.Tags {
		if tag.Key != nil && *tag.Key == name {
			if tag.Value != nil {
				return *tag.Value
			}
			break
		}
	}
	return ""
}

func getAvailabilityZone(metadata *ec2metadata.EC2Metadata) (string, error) {
	return metadata.GetMetadata("placement/availability-zone")
}

// getInstanceInfo queries the full information about the specified instance from the AWS API
func getInstanceInfo(ec2api *ec2.EC2, instanceID string) (*ec2.Instance, error) {
	request := &ec2.DescribeInstancesInput{
		InstanceIds: []*string{&instanceID},
	}

	instances, err := describeInstances(ec2api, request)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(instances) == 0 {
		return nil, trace.NotFound("no instances found for ID: %v", instanceID)
	}
	if len(instances) > 1 {
		return nil, trace.BadParameter("multiple instances found for ID: %v", instanceID)
	}
	return instances[0], nil
}

func describeInstances(ec2api *ec2.EC2, request *ec2.DescribeInstancesInput) (results []*ec2.Instance, err error) {
	var nextToken *string

	for {
		response, err := ec2api.DescribeInstances(request)
		if err != nil {
			return nil, trace.Wrap(err, "failed to list AWS instances")
		}

		for _, reservation := range response.Reservations {
			results = append(results, reservation.Instances...)
		}

		nextToken = response.NextToken
		if isNilOrEmpty(nextToken) {
			break
		}
		request.NextToken = nextToken
	}

	return results, nil
}

func isNilOrEmpty(s *string) bool {
	return s == nil || *s == ""
}
