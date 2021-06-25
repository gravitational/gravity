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
	"testing"
	"time"

	gaws "github.com/gravitational/gravity/lib/cloudprovider/aws"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/sqs"

	"github.com/gravitational/trace"
	"gopkg.in/check.v1"
)

func TestAutoscaler(t *testing.T) { check.TestingT(t) }

type AutoscalerSuite struct{}

var _ = check.Suite(&AutoscalerSuite{})

// TestInstanceTerminate tests case with terminated instance
func (s *AutoscalerSuite) TestInstanceTerminate(c *check.C) {
	clusterName := "bob"
	instance := &gaws.Instance{
		ID: "instance-1",
	}
	ec := newMockEC2(&ec2.Instance{
		InstanceId: aws.String("instance-1"),
	})
	queue := newMockQueue("queue-1")
	a, err := New(Config{
		ClusterName: clusterName,
		NewLocalInstance: func() (*gaws.Instance, error) {
			return instance, nil
		},
		Queue: queue,
		Cloud: ec,
	})
	c.Assert(err, check.IsNil)
	c.Assert(a, check.NotNil)

	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()
	server := storage.Server{
		InstanceID: "instance-to-delete",
		Hostname:   "instance-to-delete.hostname",
	}
	op := newMockOperator(ops.Site{
		AccountID: "1",
		Domain:    "example.com",
		ClusterState: storage.ClusterState{
			Servers: []storage.Server{server},
		},
	})
	go a.ProcessEvents(ctx, queue.url, op)

	// send terminated event
	msg := &message{
		receipt: "message-1",
		body: mustMarshalHook(HookEvent{
			InstanceID: server.InstanceID,
			Type:       InstanceTerminating,
		}),
	}
	select {
	case <-time.After(time.Second):
		c.Fatalf("timeout")
	case queue.messagesC <- msg:
	}

	// expect the shrink operation to arrive and the message to be deleted
	select {
	case op := <-op.shrinksC:
		c.Assert(op.Servers, check.DeepEquals, []string{server.Hostname})
	case <-time.After(time.Second):
		c.Fatalf("timeout")
	}

	// expect the message to be scheduled for deletion
	select {
	case m := <-queue.deletedC:
		c.Assert(aws.StringValue(m.ReceiptHandle), check.DeepEquals, msg.receipt)
	case <-time.After(time.Second):
		c.Fatalf("timeout")
	}
}

// TestInstanceLaunch tests instance launching
func (s *AutoscalerSuite) TestInstanceLaunching(c *check.C) {
	clusterName := "bob"
	instance := &gaws.Instance{
		ID: "instance-1",
	}
	queue := newMockQueue("queue-1")
	ec := newMockEC2(&ec2.Instance{
		InstanceId: aws.String("instance-1"),
	})
	a, err := New(Config{
		ClusterName: clusterName,
		NewLocalInstance: func() (*gaws.Instance, error) {
			return instance, nil
		},
		Queue: queue,
		Cloud: ec,
	})
	c.Assert(err, check.IsNil)
	c.Assert(a, check.NotNil)

	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()
	op := newMockOperator(ops.Site{
		AccountID:    "1",
		Domain:       "example.com",
		ClusterState: storage.ClusterState{},
	})
	go a.ProcessEvents(ctx, queue.url, op)

	// send launched event
	instanceID := "instance-1"
	msg := &message{
		receipt: "message-1",
		body: mustMarshalHook(HookEvent{
			InstanceID: instanceID,
			Type:       InstanceLaunching,
		}),
	}
	select {
	case <-time.After(time.Second):
		c.Fatalf("timeout")
	case queue.messagesC <- msg:
	}

	// expect the turn off request to arrive
	select {
	case input := <-ec.modifyC:
		c.Assert(aws.StringValue(input.InstanceId), check.DeepEquals, instanceID)
		c.Assert(input.SourceDestCheck.Value, check.DeepEquals, aws.Bool(false))
	case <-time.After(time.Second):
		c.Fatalf("timeout")
	}

	// expect the message to be scheduled for deletion
	select {
	case m := <-queue.deletedC:
		c.Assert(aws.StringValue(m.ReceiptHandle), check.DeepEquals, msg.receipt)
	case <-time.After(time.Second):
		c.Fatalf("timeout")
	}
}

func newMockQueue(url string) *mockQueue {
	return &mockQueue{
		url:       url,
		messagesC: make(chan *message, 10),
		deletedC:  make(chan *sqs.DeleteMessageInput, 10),
	}
}

type mockEC2 struct {
	modifyC  chan *ec2.ModifyInstanceAttributeInput
	instance *ec2.Instance
}

func newMockEC2(instance *ec2.Instance) *mockEC2 {
	return &mockEC2{
		modifyC:  make(chan *ec2.ModifyInstanceAttributeInput, 10),
		instance: instance,
	}
}

func (m *mockEC2) ModifyInstanceAttributeWithContext(ctx aws.Context, input *ec2.ModifyInstanceAttributeInput, opts ...request.Option) (*ec2.ModifyInstanceAttributeOutput, error) {
	select {
	case m.modifyC <- input:
		return &ec2.ModifyInstanceAttributeOutput{}, nil
	case <-ctx.Done():
		return nil, trace.ConnectionProblem(nil, "context is terminating")
	default:
		return nil, trace.BadParameter("blocked on channel send")
	}
}

func (m *mockEC2) DescribeInstancesWithContext(ctx aws.Context, input *ec2.DescribeInstancesInput, opts ...request.Option) (*ec2.DescribeInstancesOutput, error) {
	return &ec2.DescribeInstancesOutput{
		Reservations: []*ec2.Reservation{
			{
				Instances: []*ec2.Instance{m.instance},
			},
		},
	}, nil
}

func (m *mockEC2) WaitUntilInstanceTerminatedWithContext(ctx aws.Context, input *ec2.DescribeInstancesInput, opts ...request.WaiterOption) error {
	return nil
}

type message struct {
	receipt string
	body    string
}
type mockQueue struct {
	url       string
	messagesC chan *message
	deletedC  chan *sqs.DeleteMessageInput
}

func (q *mockQueue) DeleteMessageWithContext(ctx aws.Context, i *sqs.DeleteMessageInput, opts ...request.Option) (*sqs.DeleteMessageOutput, error) {
	select {
	case q.deletedC <- i:
		return &sqs.DeleteMessageOutput{}, nil
	case <-ctx.Done():
		return nil, trace.ConnectionProblem(nil, "context is terminating")
	default:
		return nil, trace.BadParameter("blocked on send in DeleteMessageWithContext")
	}
}

func (q *mockQueue) ReceiveMessageWithContext(ctx aws.Context, i *sqs.ReceiveMessageInput, opts ...request.Option) (*sqs.ReceiveMessageOutput, error) {
	select {
	case e := <-q.messagesC:
		return &sqs.ReceiveMessageOutput{
			Messages: []*sqs.Message{
				{
					Body:          aws.String(e.body),
					ReceiptHandle: aws.String(e.receipt),
				},
			},
		}, nil
	case <-time.After(time.Second * time.Duration(aws.Int64Value(i.WaitTimeSeconds))):
		return &sqs.ReceiveMessageOutput{}, nil
	case <-ctx.Done():
		return nil, trace.ConnectionProblem(nil, "context is terminating")
	}
}

//nolint:revive,stylecheck // implements external contract
func (q *mockQueue) GetQueueUrlWithContext(ctx aws.Context, i *sqs.GetQueueUrlInput, opts ...request.Option) (*sqs.GetQueueUrlOutput, error) {
	return &sqs.GetQueueUrlOutput{
		QueueUrl: aws.String(q.url),
	}, nil
}

func newMockOperator(site ops.Site) *mockOperator {
	return &mockOperator{
		site:     site,
		shrinksC: make(chan *ops.CreateSiteShrinkOperationRequest, 100),
	}
}

type mockOperator struct {
	site     ops.Site
	shrinksC chan *ops.CreateSiteShrinkOperationRequest
}

func (o *mockOperator) GetLocalSite(context.Context) (*ops.Site, error) {
	return &o.site, nil
}

func (o *mockOperator) CreateSiteShrinkOperation(ctx context.Context, req ops.CreateSiteShrinkOperationRequest) (*ops.SiteOperationKey, error) {
	select {
	case o.shrinksC <- &req:
	default:
		return nil, trace.BadParameter("blocked on channel: %v", len(o.shrinksC))
	}
	return &ops.SiteOperationKey{
		AccountID:   o.site.AccountID,
		SiteDomain:  o.site.Domain,
		OperationID: "op-1",
	}, nil
}

func (o *mockOperator) GetSiteOperationProgress(key ops.SiteOperationKey) (*ops.ProgressEntry, error) {
	return nil, nil
}
