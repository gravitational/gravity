package aws

import (
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
	GetParameterWithContext(aws.Context, *ssm.GetParameterInput, ...request.Option) (*ssm.GetParameterOutput, error)
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
}

// Operator is a simplified operator interface to mock in tests
type Operator interface {
	GetLocalSite() (*ops.Site, error)
	CreateSiteShrinkOperation(ops.CreateSiteShrinkOperationRequest) (*ops.SiteOperationKey, error)
}

type NewLocalInstance func() (*gaws.Instance, error)
