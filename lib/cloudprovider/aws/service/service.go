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

package service

import (
	"fmt"

	"github.com/gravitational/gravity/lib/cloudprovider/aws/validation"
	"github.com/gravitational/trace"
	"golang.org/x/net/context"

	awsapi "github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	log "github.com/sirupsen/logrus"
)

// ValidateOutput defines the result of running provider validation
type ValidateOutput struct {
	// VerificationError defines the result of a failing API key verification.
	// It contains a policy document detailing missing permissions in the format
	// acceptable by AWS APIs
	*VerificationError `json:"verify"`
	// Regions lists all available AWS regions
	Regions []*Region `json:"regions"`
}

// FilterRegions removes the regions which are not a part of the provided list from
// this validation result
func (v *ValidateOutput) FilterRegions(regions []string) {
	filtered := make([]*Region, 0, len(regions))
	for _, filter := range regions {
		for _, region := range v.Regions {
			if region.Name == filter {
				filtered = append(filtered, region)
			}
		}
	}
	v.Regions = filtered
}

// VerificationError defines the result of running a permission check
// to a set of AWS resources for the specified credentials
type VerificationError struct {
	// Actions is a list of missing permissions
	Actions []validation.Action `json:"actions"`
}

// Error formats this error as a string so the type implements "error" interface
func (e VerificationError) Error() string {
	return fmt.Sprintf("%v", e.Actions)
}

// VPC defines an AWS VPC
type VPC struct {
	// ID defines a VPC ID
	ID string `json:"vpc_id"`
	// CIDR defines the cidr address block for this VPC
	CIDR string `json:"cidr_block"`
	// Default defines if this VPC is a default one
	Default bool `json:"is_default"`
	// State describes the VPC state: available or pending
	State string `json:"state"`
	// Tags is the tags attached to this VPC
	Tags map[string]string `json:"tags"`
}

// Subnet is our representation of AWS subnet
type Subnet struct {
	// ID is the subnet ID
	ID string `json:"subnet_id"`
	// VPCID is the ID of the VPC the subnet is in
	VPCID string `json:"vpc_id"`
	// CIDR is the subnet CIDR block
	CIDR string `json:"cidr_block"`
	// Tags is the subnet tags
	Tags map[string]string `json:"tags"`
}

// KeyPair defines an AWS key pair reference
type KeyPair struct {
	// Name identifies the key pair
	Name string `json:"name"`
}

// Region defines an AWS EC2 region
type Region struct {
	// Name specifies the region by name
	Name string `json:"name"`
	// Endpoints defines the endpoint for this region
	Endpoint string `json:"endpoint"`
	// VPCs lists the VPCs in this region
	VPCs []VPC `json:"vpcs"`
	// KeyPairs lists the key pairs defined in this region
	KeyPairs []KeyPair `json:"key_pairs"`
}

// Instance defines an AWS instance type
type Instance struct {
	// Name is the name of the instance type
	Name string
	// CPU is the number of cores this instance type has
	CPU int
	// MemoryMiB is the amount of RAM this instance type has
	MemoryMiB int
}

// New returns a new instance of the AWS provider
func New(accessKey, secretKey, sessionToken string) *provider {
	creds := credentials.NewStaticCredentials(accessKey, secretKey, sessionToken)
	return &provider{creds}
}

// Validate runs permission validation against the given set of actions (resources)
// and obtains basic cloud provider metadata.
//nolint::revive // ctx will be moved to front in a separate PR
func (r *provider) Validate(probes validation.Probes, policyVersion string, ctx context.Context) (*ValidateOutput, error) {
	// FIXME: assuming a default region for the permissions check as the region
	// is not specified in input.
	// The permissions check does not really require the region, but
	// the queries it executes are region-based. This should not be a problem,
	// since we're not quering actual data, but verifying that the access is
	// at all given - regardless of the region.
	actions, err := validation.ValidateWithCreds(r.creds, *defaultRegion, probes, ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(actions) > 0 {
		return nil, VerificationError{Actions: actions}
	}

	session, err := session.NewSession()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	regions, err := describeRegions(session, r.creds, ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	result := make([]*Region, 0, len(regions))
	for _, region := range regions {
		result = append(result, region)
	}

	if err = r.describeVPCs(session, regions, ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	if err = r.describeKeyPairs(session, regions, ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	return &ValidateOutput{
		Regions: result,
	}, nil
}

// GetAvailabilityZones returns a list of available availability zones for the specified region
func (r *provider) GetAvailabilityZones(region string) ([]string, error) {
	session, err := session.NewSession()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	conn := ec2.New(session, &awsapi.Config{
		Credentials: r.creds,
		Region:      &region,
	})

	output, err := conn.DescribeAvailabilityZones(&ec2.DescribeAvailabilityZonesInput{
		Filters: []*ec2.Filter{
			{
				Name:   awsapi.String("region-name"),
				Values: []*string{&region},
			},
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var result []string
	for _, az := range output.AvailabilityZones {
		if awsapi.StringValue(az.State) == ec2.AvailabilityZoneStateAvailable {
			result = append(result, awsapi.StringValue(az.ZoneName))
		}
	}

	return result, nil
}

// GetInternetGatewayID returns ID of the internet gateway attached to the specified VPC
func (r *provider) GetInternetGatewayID(region, vpcID string) (string, error) {
	session, err := session.NewSession()
	if err != nil {
		return "", trace.Wrap(err)
	}
	conn := ec2.New(session, &awsapi.Config{
		Credentials: r.creds,
		Region:      &region,
	})
	out, err := conn.DescribeInternetGateways(&ec2.DescribeInternetGatewaysInput{
		Filters: []*ec2.Filter{{
			Name:   awsapi.String("attachment.vpc-id"),
			Values: awsapi.StringSlice([]string{vpcID})},
		}})
	if err != nil {
		return "", trace.Wrap(err)
	}
	if len(out.InternetGateways) == 0 {
		return "", trace.NotFound("VPC %v/%v does not have attached internet gateways",
			region, vpcID)
	}
	return awsapi.StringValue(out.InternetGateways[0].InternetGatewayId), nil
}

// FindVPCByTag returns the first VPC in region matching the provided tag
func (r *provider) FindVPCByTag(region, key, value string) (*VPC, error) {
	session, err := session.NewSession()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	conn := ec2.New(session, &awsapi.Config{
		Credentials: r.creds,
		Region:      &region,
	})
	out, err := conn.DescribeVpcs(&ec2.DescribeVpcsInput{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, vpc := range out.Vpcs {
		for _, tag := range vpc.Tags {
			if awsapi.StringValue(tag.Key) == key && awsapi.StringValue(tag.Value) == value {
				return &VPC{
					ID:      awsapi.StringValue(vpc.VpcId),
					CIDR:    awsapi.StringValue(vpc.CidrBlock),
					Default: awsapi.BoolValue(vpc.IsDefault),
					State:   awsapi.StringValue(vpc.State),
				}, nil
			}
		}
	}
	return nil, trace.NotFound("no VPC matching tag %v=%v in %v", key, value, region)
}

// GetSubnets returns a list of all subnets found in the specified VPC
func (r *provider) GetSubnets(region, vpcID string) ([]Subnet, error) {
	session, err := session.NewSession()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	conn := ec2.New(session, &awsapi.Config{
		Credentials: r.creds,
		Region:      &region,
	})
	out, err := conn.DescribeSubnets(&ec2.DescribeSubnetsInput{
		Filters: []*ec2.Filter{{
			Name:   awsapi.String("vpc-id"),
			Values: awsapi.StringSlice([]string{vpcID})},
		}})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	subnets := make([]Subnet, 0, len(out.Subnets))
	for _, s := range out.Subnets {
		tags := make(map[string]string)
		for _, t := range s.Tags {
			tags[awsapi.StringValue(t.Key)] = awsapi.StringValue(t.Value)
		}
		subnets = append(subnets, Subnet{
			ID:    awsapi.StringValue(s.SubnetId),
			VPCID: awsapi.StringValue(s.VpcId),
			CIDR:  awsapi.StringValue(s.CidrBlock),
			Tags:  tags,
		})
	}
	return subnets, nil
}

// GetCIDRBlocks returns CIDR blocks for the specified VPC and all its subnets
func (r *provider) GetCIDRBlocks(region, vpcID string) (vpcBlock string, subnetBlocks []string, err error) {
	session, err := session.NewSession()
	if err != nil {
		return "", nil, trace.Wrap(err)
	}
	conn := ec2.New(session, &awsapi.Config{
		Credentials: r.creds,
		Region:      &region,
	})
	vpcOut, err := conn.DescribeVpcs(&ec2.DescribeVpcsInput{
		Filters: []*ec2.Filter{{
			Name:   awsapi.String("vpc-id"),
			Values: awsapi.StringSlice([]string{vpcID})},
		}})
	if err != nil {
		return "", nil, trace.Wrap(err)
	}
	if len(vpcOut.Vpcs) == 0 {
		return "", nil, trace.NotFound("VPC %v/%v not found", region, vpcID)
	}
	vpcBlock = awsapi.StringValue(vpcOut.Vpcs[0].CidrBlock)
	subnetOut, err := conn.DescribeSubnets(&ec2.DescribeSubnetsInput{
		Filters: []*ec2.Filter{{
			Name:   awsapi.String("vpc-id"),
			Values: awsapi.StringSlice([]string{vpcID})},
		}})
	if err != nil {
		return "", nil, trace.Wrap(err)
	}
	for _, subnet := range subnetOut.Subnets {
		subnetBlocks = append(subnetBlocks, awsapi.StringValue(subnet.CidrBlock))
	}
	return vpcBlock, subnetBlocks, nil
}

func (r *provider) describeVPCs(session *session.Session, regions map[string]*Region, ctx context.Context) error {
	type result struct {
		region string
		vpcs   []VPC
		err    error
	}

	describeVPCs := func(region string, ctx context.Context, resultC chan<- *result) {
		conn := ec2.New(session, &awsapi.Config{
			Credentials: r.creds,
			Region:      &region,
		})

		resp, err := conn.DescribeVpcs(&ec2.DescribeVpcsInput{})
		if err != nil {
			resultC <- &result{err: err}
			return
		}
		var vpcs []VPC
		for _, vpc := range resp.Vpcs {
			tags := make(map[string]string)
			for _, tag := range vpc.Tags {
				tags[awsapi.StringValue(tag.Key)] = awsapi.StringValue(tag.Value)
			}
			item := VPC{
				CIDR:    awsapi.StringValue(vpc.CidrBlock),
				ID:      awsapi.StringValue(vpc.VpcId),
				Default: awsapi.BoolValue(vpc.IsDefault),
				State:   awsapi.StringValue(vpc.State),
				Tags:    tags,
			}
			vpcs = append(vpcs, item)
			log.Debugf("VPC: %v", item)
		}
		resultC <- &result{vpcs: vpcs, region: region}
	}

	resultC := make(chan *result)
	for regionName := range regions {
		go describeVPCs(regionName, ctx, resultC)
	}

	var errors []error
	for range regions {
		select {
		case result := <-resultC:
			if result.err != nil {
				errors = append(errors, result.err)
			} else {
				regions[result.region].VPCs = append(regions[result.region].VPCs, result.vpcs...)
			}
		case <-ctx.Done():
			break
		}
	}
	close(resultC)
	if len(errors) > 0 {
		return trace.NewAggregate(errors...)
	}
	return nil
}

func (r *provider) describeKeyPairs(session *session.Session, regions map[string]*Region, ctx context.Context) error {
	type result struct {
		region   string
		keyPairs []KeyPair
		err      error
	}
	describeKeyPairs := func(region string, ctx context.Context, resultC chan<- *result) {
		conn := ec2.New(session, &awsapi.Config{
			Credentials: r.creds,
			Region:      &region,
		})

		resp, err := conn.DescribeKeyPairs(&ec2.DescribeKeyPairsInput{})
		if err != nil {
			resultC <- &result{err: err}
			return
		}
		var keyPairs []KeyPair
		for _, keyPair := range resp.KeyPairs {
			item := KeyPair{Name: awsapi.StringValue(keyPair.KeyName)}
			keyPairs = append(keyPairs, item)
			log.Debugf("KeyPair: %v", item)
		}
		resultC <- &result{keyPairs: keyPairs, region: region}
	}

	resultC := make(chan *result)
	for regionName := range regions {
		go describeKeyPairs(regionName, ctx, resultC)
	}

	var errors []error
	for range regions {
		select {
		case result := <-resultC:
			if result.err != nil {
				errors = append(errors, result.err)
			} else {
				regions[result.region].KeyPairs = append(regions[result.region].KeyPairs, result.keyPairs...)
			}
		case <-ctx.Done():
			break
		}
	}
	close(resultC)
	if len(errors) > 0 {
		return trace.NewAggregate(errors...)
	}
	return nil
}

func describeRegions(session *session.Session, creds *credentials.Credentials, ctx context.Context) (regions map[string]*Region, err error) {
	conn := ec2.New(session, &awsapi.Config{
		Credentials: creds,
		Region:      defaultRegion,
	})

	resp, err := conn.DescribeRegions(&ec2.DescribeRegionsInput{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	regions = make(map[string]*Region, len(resp.Regions))
	for _, region := range resp.Regions {
		regionName := awsapi.StringValue(region.RegionName)
		regions[regionName] = &Region{Name: regionName, Endpoint: awsapi.StringValue(region.Endpoint)}
	}
	return regions, nil
}

type provider struct {
	creds *credentials.Credentials
}

// defaultRegion defines an EC2 region to use for API calls where a region is not determining the results
// (i.e. querying all regions or making a resource probing call where region is irrelevant)
var defaultRegion = awsapi.String("us-east-1")

var EC2InstanceTypes = []Instance{
	{Name: "t2.nano", CPU: 1, MemoryMiB: 500},
	{Name: "t2.micro", CPU: 1, MemoryMiB: 1000},
	{Name: "t2.small", CPU: 1, MemoryMiB: 2000},
	{Name: "t2.medium", CPU: 2, MemoryMiB: 4000},
	{Name: "t2.large", CPU: 2, MemoryMiB: 8000},
	{Name: "m3.medium", CPU: 1, MemoryMiB: 3750},
	{Name: "m3.large", CPU: 2, MemoryMiB: 7500},
	{Name: "m3.xlarge", CPU: 4, MemoryMiB: 15000},
	{Name: "m3.2xlarge", CPU: 8, MemoryMiB: 30000},
	{Name: "m4.large", CPU: 2, MemoryMiB: 8000},
	{Name: "m4.xlarge", CPU: 4, MemoryMiB: 16000},
	{Name: "m4.2xlarge", CPU: 8, MemoryMiB: 32000},
	{Name: "m4.4xlarge", CPU: 16, MemoryMiB: 64000},
	{Name: "m4.10xlarge", CPU: 40, MemoryMiB: 160000},
	{Name: "c3.large", CPU: 2, MemoryMiB: 3750},
	{Name: "c3.xlarge", CPU: 4, MemoryMiB: 7500},
	{Name: "c3.2xlarge", CPU: 8, MemoryMiB: 15000},
	{Name: "c3.4xlarge", CPU: 16, MemoryMiB: 30000},
	{Name: "c3.8xlarge", CPU: 32, MemoryMiB: 60000},
	{Name: "c4.large", CPU: 2, MemoryMiB: 3750},
	{Name: "c4.xlarge", CPU: 4, MemoryMiB: 7500},
	{Name: "c4.2xlarge", CPU: 8, MemoryMiB: 15000},
	{Name: "c4.4xlarge", CPU: 16, MemoryMiB: 30000},
	{Name: "c4.8xlarge", CPU: 36, MemoryMiB: 60000},
	{Name: "x1.32xlarge", CPU: 128, MemoryMiB: 1952000},
	{Name: "g2.2xlarge", CPU: 8, MemoryMiB: 15000},
	{Name: "g2.8xlarge", CPU: 32, MemoryMiB: 60000},
	{Name: "r3.large", CPU: 2, MemoryMiB: 15250},
	{Name: "r3.xlarge", CPU: 4, MemoryMiB: 30500},
	{Name: "r3.2xlarge", CPU: 8, MemoryMiB: 61000},
	{Name: "r3.4xlarge", CPU: 16, MemoryMiB: 122000},
	{Name: "r3.8xlarge", CPU: 32, MemoryMiB: 244000},
	{Name: "i2.xlarge", CPU: 4, MemoryMiB: 30500},
	{Name: "i2.2xlarge", CPU: 8, MemoryMiB: 61000},
	{Name: "i2.4xlarge", CPU: 16, MemoryMiB: 122000},
	{Name: "i2.8xlarge", CPU: 32, MemoryMiB: 244000},
	{Name: "d2.xlarge", CPU: 4, MemoryMiB: 30500},
	{Name: "d2.2xlarge", CPU: 8, MemoryMiB: 61000},
	{Name: "d2.4xlarge", CPU: 16, MemoryMiB: 122000},
	{Name: "d2.8xlarge", CPU: 36, MemoryMiB: 244000},
}
