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

package validation

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/gravitational/trace"
)

var emptyStringValue = aws.String("")

// EC2Probes lists all currently supported EC2 resource probes
var EC2Probes = []ResourceProbe{
	{Action{EC2, "DescribeRegions"}, validateDescribeRegions},
	{Action{EC2, "CreateVpc"}, validateCreateVPC},
	{Action{EC2, "DeleteVpc"}, validateDeleteVPC},
	{Action{EC2, "DescribeNetworkAcls"}, validateDescribeNetworkACLs},
	{Action{EC2, "DescribeVpcAttribute"}, validateDescribeVPCAttribute},
	{Action{EC2, "DescribeVpcs"}, validateDescribeVPCs},
	{Action{EC2, "DescribeVpcClassicLink"}, validateDescribeVPCClassicLink},
	{Action{EC2, "ModifyVpcAttribute"}, validateModifyVPCAttribute},
	{Action{EC2, "CreateTags"}, validateCreateTags},
	{Action{EC2, "DescribeInstances"}, validateDescribeInstances},
	{Action{EC2, "DescribeImages"}, validateDescribeImages},
	{Action{EC2, "DescribeAvailabilityZones"}, validateDescribeAvailabilityZones},
	{Action{EC2, "RunInstances"}, validateRunInstances},
	{Action{EC2, "TerminateInstances"}, validateTerminateInstances},
	{Action{EC2, "StopInstances"}, validateStopInstances},
	{Action{EC2, "StartInstances"}, validateStartInstances},
	{Action{EC2, "ModifyInstanceAttribute"}, validateModifyInstanceAttribute},
	{Action{EC2, "DescribeVolumes"}, validateDescribeVolumes},
	{Action{EC2, "CreateSecurityGroup"}, validateCreateSecurityGroup},
	{Action{EC2, "DeleteSecurityGroup"}, validateDeleteSecurityGroup},
	{Action{EC2, "DescribeSecurityGroups"}, validateDescribeSecurityGroups},
	{Action{EC2, "RevokeSecurityGroupEgress"}, validateRevokeSecurityGroupEgress},
	{Action{EC2, "RevokeSecurityGroupIngress"}, validateRevokeSecurityGroupIngress},
	{Action{EC2, "AuthorizeSecurityGroupEgress"}, validateAuthorizeSecurityGroupEgress},
	{Action{EC2, "AuthorizeSecurityGroupIngress"}, validateAuthorizeSecurityGroupIngress},
	{Action{EC2, "AttachInternetGateway"}, validateAttachInternetGateway},
	{Action{EC2, "CreateInternetGateway"}, validateCreateInternetGateway},
	{Action{EC2, "DeleteInternetGateway"}, validateDeleteInternetGateway},
	{Action{EC2, "DescribeInternetGateways"}, validateDescribeInternetGateways},
	{Action{EC2, "CreateSubnet"}, validateCreateSubnet},
	{Action{EC2, "DeleteSubnet"}, validateDeleteSubnet},
	{Action{EC2, "DescribeSubnets"}, validateDescribeSubnets},
	{Action{EC2, "ModifySubnetAttribute"}, validateModifySubnetAttribute},
	{Action{EC2, "DescribeRouteTables"}, validateDescribeRouteTables},
	{Action{EC2, "CreateRoute"}, validateCreateRoute},
	{Action{EC2, "CreateRouteTable"}, validateCreateRouteTable},
	{Action{EC2, "DeleteRoute"}, validateDeleteRoute},
	{Action{EC2, "DeleteRouteTable"}, validateDeleteRouteTable},
	{Action{EC2, "AssociateRouteTable"}, validateAssociateRouteTable},
	{Action{EC2, "DisassociateRouteTable"}, validateDisassociateRouteTable},
	{Action{EC2, "ReplaceRouteTableAssociation"}, validateReplaceRouteTableAssociation},
	{Action{EC2, "DescribeKeyPairs"}, validateDescribeKeyPairs},
	{Action{EC2, "DetachInternetGateway"}, validateDetachInternetGateway},
}

func validateDescribeRegions(client *clientContext) error {
	request := &ec2.DescribeRegionsInput{
		DryRun: dryRun,
	}
	_, err := client.ec2.DescribeRegions(request)
	return trace.Wrap(err)
}

func validateCreateVPC(client *clientContext) error {
	request := &ec2.CreateVpcInput{
		CidrBlock: aws.String(""),
		DryRun:    dryRun,
	}
	_, err := client.ec2.CreateVpc(request)
	return trace.Wrap(err)
}

func validateDeleteVPC(client *clientContext) error {
	request := &ec2.DeleteVpcInput{
		DryRun: dryRun,
		VpcId:  aws.String(""),
	}
	_, err := client.ec2.DeleteVpc(request)
	return trace.Wrap(err)
}

func validateDescribeNetworkACLs(client *clientContext) error {
	request := &ec2.DescribeNetworkAclsInput{
		DryRun: dryRun,
	}
	_, err := client.ec2.DescribeNetworkAcls(request)
	return trace.Wrap(err)
}

func validateDescribeVPCAttribute(client *clientContext) error {
	request := &ec2.DescribeVpcAttributeInput{
		DryRun:    dryRun,
		VpcId:     emptyStringValue,
		Attribute: emptyStringValue,
	}
	_, err := client.ec2.DescribeVpcAttribute(request)
	return trace.Wrap(err)
}

func validateDescribeVPCs(client *clientContext) error {
	request := &ec2.DescribeVpcsInput{
		DryRun: dryRun,
	}
	_, err := client.ec2.DescribeVpcs(request)
	return trace.Wrap(err)
}

func validateDescribeVPCClassicLink(client *clientContext) error {
	request := &ec2.DescribeVpcClassicLinkInput{
		DryRun: dryRun,
	}
	_, err := client.ec2.DescribeVpcClassicLink(request)
	return trace.Wrap(err)
}

func validateModifyVPCAttribute(client *clientContext) error {
	request := &ec2.ModifyVpcAttributeInput{
		VpcId:              dummyValue("vpc"),
		EnableDnsHostnames: &ec2.AttributeBooleanValue{Value: aws.Bool(false)},
	}
	_, err := client.ec2.ModifyVpcAttribute(request)
	return trace.Wrap(err)
}

func validateCreateTags(client *clientContext) error {
	// TODO(knisbet) We've encountered an issue where this API call appears to have started generating 500 errors from amazon
	// disable this check until it can be properly fixed
	// https://github.com/gravitational/gravity/issues/3047
	return nil
	// request := &ec2.CreateTagsInput{
	// 	DryRun:    dryRun,
	// 	Resources: []*string{},
	// 	Tags:      []*ec2.Tag{},
	// }
	// _, err := client.ec2.CreateTags(request)
	// return trace.Wrap(err)
}

func validateDescribeInstances(client *clientContext) error {
	request := &ec2.DescribeInstancesInput{
		DryRun: dryRun,
	}
	_, err := client.ec2.DescribeInstances(request)
	return trace.Wrap(err)
}

func validateDescribeImages(client *clientContext) error {
	request := &ec2.DescribeImagesInput{
		DryRun: dryRun,
	}
	_, err := client.ec2.DescribeImages(request)
	return trace.Wrap(err)
}

func validateDescribeAvailabilityZones(client *clientContext) error {
	request := &ec2.DescribeAvailabilityZonesInput{
		DryRun: dryRun,
	}
	_, err := client.ec2.DescribeAvailabilityZones(request)
	return trace.Wrap(err)
}

func validateRunInstances(client *clientContext) error {
	request := &ec2.RunInstancesInput{
		DryRun:   dryRun,
		ImageId:  dummyValue("ami-"),
		MaxCount: aws.Int64(1),
		MinCount: aws.Int64(1),
	}
	_, err := client.ec2.RunInstances(request)
	return trace.Wrap(err)
}

func validateTerminateInstances(client *clientContext) error {
	instanceID := dummyValueWithLen("i-", len("03453de40a78de6c0"))
	request := &ec2.TerminateInstancesInput{
		DryRun:      dryRun,
		InstanceIds: []*string{instanceID},
	}
	_, err := client.ec2.TerminateInstances(request)
	return trace.Wrap(err)
}

func validateStopInstances(client *clientContext) error {
	instanceID := dummyValueWithLen("i-", len("03453de40a78de6c0"))
	request := &ec2.StopInstancesInput{
		DryRun:      dryRun,
		InstanceIds: []*string{instanceID},
	}
	_, err := client.ec2.StopInstances(request)
	return trace.Wrap(err)
}

func validateStartInstances(client *clientContext) error {
	instanceID := dummyValueWithLen("i-", len("03453de40a78de6c0"))
	request := &ec2.StartInstancesInput{
		DryRun:      dryRun,
		InstanceIds: []*string{instanceID},
	}
	_, err := client.ec2.StartInstances(request)
	return trace.Wrap(err)
}

func validateModifyInstanceAttribute(client *clientContext) error {
	request := &ec2.ModifyInstanceAttributeInput{
		DryRun:     dryRun,
		InstanceId: emptyStringValue,
	}
	_, err := client.ec2.ModifyInstanceAttribute(request)
	return trace.Wrap(err)
}

func validateDescribeVolumes(client *clientContext) error {
	request := &ec2.DescribeVolumesInput{
		DryRun: dryRun,
	}
	_, err := client.ec2.DescribeVolumes(request)
	return trace.Wrap(err)
}

func validateCreateSecurityGroup(client *clientContext) error {
	request := &ec2.CreateSecurityGroupInput{
		DryRun:      dryRun,
		GroupName:   emptyStringValue,
		Description: emptyStringValue,
	}
	_, err := client.ec2.CreateSecurityGroup(request)
	return trace.Wrap(err)
}

func validateDeleteSecurityGroup(client *clientContext) error {
	groupID := dummyValueWithLen("sg-", len("72034715"))
	request := &ec2.DeleteSecurityGroupInput{
		DryRun:  dryRun,
		GroupId: groupID,
	}
	_, err := client.ec2.DeleteSecurityGroup(request)
	return trace.Wrap(err)
}

func validateDescribeSecurityGroups(client *clientContext) error {
	request := &ec2.DescribeSecurityGroupsInput{
		DryRun: dryRun,
	}
	_, err := client.ec2.DescribeSecurityGroups(request)
	return trace.Wrap(err)
}

func validateRevokeSecurityGroupEgress(client *clientContext) error {
	groupID := dummyValueWithLen("sg-", len("72034715"))
	request := &ec2.RevokeSecurityGroupEgressInput{
		DryRun:        dryRun,
		GroupId:       groupID,
		IpPermissions: []*ec2.IpPermission{},
	}
	_, err := client.ec2.RevokeSecurityGroupEgress(request)
	return trace.Wrap(err)
}

func validateRevokeSecurityGroupIngress(client *clientContext) error {
	groupID := dummyValueWithLen("sg-", len("72034715"))
	request := &ec2.RevokeSecurityGroupIngressInput{
		DryRun:  dryRun,
		GroupId: groupID,
	}
	_, err := client.ec2.RevokeSecurityGroupIngress(request)
	return trace.Wrap(err)
}

func validateAuthorizeSecurityGroupEgress(client *clientContext) error {
	groupID := dummyValueWithLen("sg-", len("72034715"))
	request := &ec2.AuthorizeSecurityGroupEgressInput{
		DryRun:        dryRun,
		GroupId:       groupID,
		IpPermissions: []*ec2.IpPermission{},
	}
	_, err := client.ec2.AuthorizeSecurityGroupEgress(request)
	return trace.Wrap(err)
}

func validateAuthorizeSecurityGroupIngress(client *clientContext) error {
	request := &ec2.AuthorizeSecurityGroupIngressInput{
		DryRun:  dryRun,
		GroupId: dummyValue("sg-"),
	}
	_, err := client.ec2.AuthorizeSecurityGroupIngress(request)
	return trace.Wrap(err)
}

func validateAttachInternetGateway(client *clientContext) error {
	request := &ec2.AttachInternetGatewayInput{
		DryRun:            dryRun,
		InternetGatewayId: emptyStringValue,
		VpcId:             emptyStringValue,
	}
	_, err := client.ec2.AttachInternetGateway(request)
	return trace.Wrap(err)
}

func validateCreateInternetGateway(client *clientContext) error {
	request := &ec2.CreateInternetGatewayInput{
		DryRun: dryRun,
	}
	_, err := client.ec2.CreateInternetGateway(request)
	return trace.Wrap(err)
}

func validateDeleteInternetGateway(client *clientContext) error {
	request := &ec2.DeleteInternetGatewayInput{
		DryRun:            dryRun,
		InternetGatewayId: dummyValue("igw-"),
	}
	_, err := client.ec2.DeleteInternetGateway(request)
	return trace.Wrap(err)
}

func validateDescribeInternetGateways(client *clientContext) error {
	request := &ec2.DescribeInternetGatewaysInput{
		DryRun: dryRun,
	}
	_, err := client.ec2.DescribeInternetGateways(request)
	return trace.Wrap(err)
}

func validateCreateSubnet(client *clientContext) error {
	request := &ec2.CreateSubnetInput{
		DryRun:    dryRun,
		CidrBlock: emptyStringValue,
		VpcId:     emptyStringValue,
	}
	_, err := client.ec2.CreateSubnet(request)
	return trace.Wrap(err)
}

func validateDeleteSubnet(client *clientContext) error {
	request := &ec2.DeleteSubnetInput{
		DryRun:   dryRun,
		SubnetId: emptyStringValue,
	}
	_, err := client.ec2.DeleteSubnet(request)
	return trace.Wrap(err)
}

func validateDescribeSubnets(client *clientContext) error {
	request := &ec2.DescribeSubnetsInput{
		DryRun: dryRun,
	}
	_, err := client.ec2.DescribeSubnets(request)
	return trace.Wrap(err)
}

func validateModifySubnetAttribute(client *clientContext) error {
	request := &ec2.ModifySubnetAttributeInput{
		SubnetId:            dummyValue("subnet"),
		MapPublicIpOnLaunch: &ec2.AttributeBooleanValue{Value: aws.Bool(false)},
	}
	_, err := client.ec2.ModifySubnetAttribute(request)
	return trace.Wrap(err)
}

func validateDescribeRouteTables(client *clientContext) error {
	request := &ec2.DescribeRouteTablesInput{
		DryRun: dryRun,
	}
	_, err := client.ec2.DescribeRouteTables(request)
	return trace.Wrap(err)
}

func validateCreateRoute(client *clientContext) error {
	request := &ec2.CreateRouteInput{
		DryRun:               dryRun,
		DestinationCidrBlock: emptyStringValue,
		RouteTableId:         emptyStringValue,
	}
	_, err := client.ec2.CreateRoute(request)
	return trace.Wrap(err)
}

func validateCreateRouteTable(client *clientContext) error {
	request := &ec2.CreateRouteTableInput{
		DryRun: dryRun,
		VpcId:  emptyStringValue,
	}
	_, err := client.ec2.CreateRouteTable(request)
	return trace.Wrap(err)
}

func validateDeleteRoute(client *clientContext) error {
	routeTableId := dummyValueWithLen("rtb-", len("6e9f0b0b"))
	request := &ec2.DeleteRouteInput{
		DryRun:               dryRun,
		DestinationCidrBlock: aws.String("10.10.0.0/16"),
		RouteTableId:         routeTableId,
	}
	_, err := client.ec2.DeleteRoute(request)
	return trace.Wrap(err)
}

func validateDeleteRouteTable(client *clientContext) error {
	routeTableId := dummyValueWithLen("rtb-", len("6e9f0b0b"))
	request := &ec2.DeleteRouteTableInput{
		DryRun:       dryRun,
		RouteTableId: routeTableId,
	}
	_, err := client.ec2.DeleteRouteTable(request)
	return trace.Wrap(err)
}

func validateAssociateRouteTable(client *clientContext) error {
	request := &ec2.AssociateRouteTableInput{
		DryRun:       dryRun,
		RouteTableId: emptyStringValue,
		SubnetId:     emptyStringValue,
	}
	_, err := client.ec2.AssociateRouteTable(request)
	return trace.Wrap(err)
}

func validateDisassociateRouteTable(client *clientContext) error {
	request := &ec2.DisassociateRouteTableInput{
		DryRun:        dryRun,
		AssociationId: emptyStringValue,
	}
	_, err := client.ec2.DisassociateRouteTable(request)
	return trace.Wrap(err)
}

func validateReplaceRouteTableAssociation(client *clientContext) error {
	request := &ec2.ReplaceRouteTableAssociationInput{
		DryRun:        dryRun,
		AssociationId: emptyStringValue,
		RouteTableId:  emptyStringValue,
	}
	_, err := client.ec2.ReplaceRouteTableAssociation(request)
	return trace.Wrap(err)
}

func validateDescribeKeyPairs(client *clientContext) error {
	request := &ec2.DescribeKeyPairsInput{
		DryRun: dryRun,
	}
	_, err := client.ec2.DescribeKeyPairs(request)
	return trace.Wrap(err)
}

func validateDetachInternetGateway(client *clientContext) error {
	request := &ec2.DetachInternetGatewayInput{
		DryRun:            dryRun,
		VpcId:             emptyStringValue,
		InternetGatewayId: emptyStringValue,
	}
	_, err := client.ec2.DetachInternetGateway(request)
	return trace.Wrap(err)
}
