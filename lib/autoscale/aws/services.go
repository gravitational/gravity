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
	"context"

	gaws "github.com/gravitational/gravity/lib/cloudprovider/aws"
	"github.com/gravitational/gravity/lib/ops"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/aws/aws-sdk-go/service/ssm"
)

// SSM is an interface representing AWS Systems Manager
type SSM interface {
	GetParametersWithContext(aws.Context, *ssm.GetParametersInput, ...request.Option) (*ssm.GetParametersOutput, error)
	PutParameterWithContext(aws.Context, *ssm.PutParameterInput, ...request.Option) (*ssm.PutParameterOutput, error)
}

// SQS is an interface representing AWS Queue Service
type SQS interface {
	DeleteMessageWithContext(aws.Context, *sqs.DeleteMessageInput, ...request.Option) (*sqs.DeleteMessageOutput, error)
	ReceiveMessageWithContext(aws.Context, *sqs.ReceiveMessageInput, ...request.Option) (*sqs.ReceiveMessageOutput, error)
	GetQueueUrlWithContext(aws.Context, *sqs.GetQueueUrlInput, ...request.Option) (*sqs.GetQueueUrlOutput, error)
}

// EC2 is an interface representing AWS Elastic Compute cloud
type EC2 interface {
	ModifyInstanceAttributeWithContext(aws.Context, *ec2.ModifyInstanceAttributeInput, ...request.Option) (*ec2.ModifyInstanceAttributeOutput, error)
	DescribeInstancesWithContext(aws.Context, *ec2.DescribeInstancesInput, ...request.Option) (*ec2.DescribeInstancesOutput, error)
	WaitUntilInstanceTerminatedWithContext(aws.Context, *ec2.DescribeInstancesInput, ...request.WaiterOption) error
}

// Operator is a simplified operator interface to mock in tests
type Operator interface {
	GetLocalSite(context.Context) (*ops.Site, error)
	CreateSiteShrinkOperation(context.Context, ops.CreateSiteShrinkOperationRequest) (*ops.SiteOperationKey, error)
	GetSiteOperationProgress(ops.SiteOperationKey) (*ops.ProgressEntry, error)
}

type NewLocalInstance func() (*gaws.Instance, error)
