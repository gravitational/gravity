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
	"github.com/aws/aws-sdk-go/service/iam"
	"github.com/gravitational/trace"
)

// IAMProbes lists all currently supported IAM resource probes
var IAMProbes = []ResourceProbe{
	{Action{IAM, "AddRoleToInstanceProfile"}, validateAddRoleToInstanceProfile},
	{Action{IAM, "CreateInstanceProfile"}, validateCreateInstanceProfile},
	{Action{IAM, "GetInstanceProfile"}, validateGetInstanceProfile},
	{Action{IAM, "CreateRole"}, validateCreateRole},
	{Action{IAM, "GetRole"}, validateGetRole},
	{Action{IAM, "DeleteRole"}, validateDeleteRole},
	{Action{IAM, "PutRolePolicy"}, validatePutRolePolicy},
	{Action{IAM, "GetRolePolicy"}, validateGetRolePolicy},
	{Action{IAM, "DeleteRolePolicy"}, validateDeleteRolePolicy},
	{Action{IAM, "ListRoles"}, validateListRoles},
	{Action{IAM, "ListInstanceProfiles"}, validateListInstanceProfiles},
	{Action{IAM, "ListInstanceProfilesForRole"}, validateListInstanceProfilesForRole},
	{Action{IAM, "RemoveRoleFromInstanceProfile"}, validateRemoveRoleFromInstanceProfile},
	{Action{IAM, "DeleteInstanceProfile"}, validateDeleteInstanceProfile},
}

func validateCreateInstanceProfile(client *clientContext) (err error) {
	instanceProfileName := dummyValue("profile")
	request := &iam.CreateInstanceProfileInput{
		InstanceProfileName: instanceProfileName,
	}
	_, err = client.iam.CreateInstanceProfile(request)
	if err == nil {
		deleteRequest := &iam.DeleteInstanceProfileInput{
			InstanceProfileName: instanceProfileName,
		}
		_, err = client.iam.DeleteInstanceProfile(deleteRequest)
	}
	return trace.Wrap(err)
}

func validateAddRoleToInstanceProfile(client *clientContext) (err error) {
	instanceProfileName := dummyValue("profile")
	request := &iam.AddRoleToInstanceProfileInput{
		InstanceProfileName: instanceProfileName,
		RoleName:            dummyValue("role"),
	}
	_, err = client.iam.AddRoleToInstanceProfile(request)
	return trace.Wrap(err)
}

func validateGetInstanceProfile(client *clientContext) (err error) {
	request := &iam.GetInstanceProfileInput{
		InstanceProfileName: dummyValue("profile"),
	}
	_, err = client.iam.GetInstanceProfile(request)
	return trace.Wrap(err)
}

func validateGetRolePolicy(client *clientContext) (err error) {
	request := &iam.GetRolePolicyInput{
		PolicyName: dummyValue("policy"),
		RoleName:   dummyValue("role"),
	}
	_, err = client.iam.GetRolePolicy(request)
	return trace.Wrap(err)
}

func validateListRoles(client *clientContext) (err error) {
	request := &iam.ListRolesInput{}
	_, err = client.iam.ListRoles(request)
	return trace.Wrap(err)
}

func validateListInstanceProfiles(client *clientContext) (err error) {
	request := &iam.ListInstanceProfilesInput{}
	_, err = client.iam.ListInstanceProfiles(request)
	return trace.Wrap(err)
}

func validateListInstanceProfilesForRole(client *clientContext) error {
	request := &iam.ListInstanceProfilesForRoleInput{
		RoleName: dummyValue("role"),
	}
	_, err := client.iam.ListInstanceProfilesForRole(request)
	return trace.Wrap(err)
}

func validateRemoveRoleFromInstanceProfile(client *clientContext) (err error) {
	request := &iam.RemoveRoleFromInstanceProfileInput{
		InstanceProfileName: dummyValue("profile"),
		RoleName:            dummyValue("role"),
	}
	_, err = client.iam.RemoveRoleFromInstanceProfile(request)
	return trace.Wrap(err)
}

func validateDeleteInstanceProfile(client *clientContext) (err error) {
	request := &iam.DeleteInstanceProfileInput{
		InstanceProfileName: dummyValue("profile"),
	}
	_, err = client.iam.DeleteInstanceProfile(request)
	return trace.Wrap(err)
}

func validateCreateRole(client *clientContext) error {
	const policy = `{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Principal": {"Service": "ec2.amazonaws.com"},
            "Action": "sts:AssumeRole"
        }
    ]
}`
	request := &iam.CreateRoleInput{
		AssumeRolePolicyDocument: aws.String(policy),
		RoleName:                 dummyValue("role"),
	}
	_, err := client.iam.CreateRole(request)
	if err == nil {
		deleteRequest := &iam.DeleteRoleInput{
			RoleName: request.RoleName,
		}
		_, err = client.iam.DeleteRole(deleteRequest)
	}
	return trace.Wrap(err)
}

func validateGetRole(client *clientContext) error {
	request := &iam.GetRoleInput{
		RoleName: dummyValue("role"),
	}
	_, err := client.iam.GetRole(request)
	return trace.Wrap(err)
}

func validateDeleteRole(client *clientContext) error {
	request := &iam.DeleteRoleInput{
		RoleName: dummyValue("role"),
	}
	_, err := client.iam.DeleteRole(request)
	return trace.Wrap(err)
}

func validatePutRolePolicy(client *clientContext) error {
	request := &iam.PutRolePolicyInput{
		PolicyDocument: dummyValue("document"),
		PolicyName:     dummyValue("policy"),
		RoleName:       dummyValue("role"),
	}
	_, err := client.iam.PutRolePolicy(request)
	return trace.Wrap(err)
}

func validateDeleteRolePolicy(client *clientContext) error {
	request := &iam.DeleteRolePolicyInput{
		PolicyName: dummyValue("policy"),
		RoleName:   dummyValue("role"),
	}
	_, err := client.iam.DeleteRolePolicy(request)
	return trace.Wrap(err)
}
