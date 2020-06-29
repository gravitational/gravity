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
	"encoding/json"
	"regexp"

	"github.com/gravitational/gravity/lib/ops"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/sqs"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

// HookEvent is a lifecycle hook event posted by autoscaling group
type HookEvent struct {
	// QueueURL is a queue this event belongs to
	QueueURL string `json:"-"`
	// ReceiptHandle is SQS receipt handle
	ReceiptHandle string `json:"-"`
	// InstanceID is AWS instance ID
	InstanceID string `json:"EC2InstanceId"`
	// Type is event type
	Type string `json:"LifecycleTransition"`
}

// GetQueueURL returns queue URL associated with this cluster
func (a *Autoscaler) GetQueueURL(ctx context.Context) (string, error) {
	expr, err := regexp.Compile("[^a-zA-Z0-9\\-]")
	if err != nil {
		return "", trace.Wrap(err)
	}
	// safeClusterName is the name that is accepted by SQS naming
	safeClusterName := expr.ReplaceAllString(a.ClusterName, "")
	out, err := a.Queue.GetQueueUrlWithContext(ctx, &sqs.GetQueueUrlInput{
		QueueName: aws.String(safeClusterName),
	})
	if err != nil {
		return "", trace.Wrap(err)
	}
	return aws.StringValue(out.QueueUrl), nil
}

// ProcessEvents listens for events on SQS queue that are sent by the auto scaling
// group lifecycle hooks.
func (a *Autoscaler) ProcessEvents(ctx context.Context, queueURL string, operator Operator) {
	a.WithField("queue", queueURL).Info("Start processing events.")
	for {
		out, err := a.Queue.ReceiveMessageWithContext(ctx, &sqs.ReceiveMessageInput{
			QueueUrl:            aws.String(queueURL),
			MaxNumberOfMessages: aws.Int64(1),
			VisibilityTimeout:   aws.Int64(30),
			WaitTimeSeconds:     aws.Int64(5),
		})
		if err != nil {
			select {
			case <-ctx.Done():
				a.WithField("queue", queueURL).Info("Stop processing events.")
				return
			default:
			}
			a.Errorf("receive message error: %v", trace.DebugReport(err))
			continue
		}
		for _, m := range out.Messages {
			a.Debugf("got message body: %q", aws.StringValue(m.Body))
			hook, err := unmarshalHook(aws.StringValue(m.Body))
			if err != nil {
				a.Errorf("failed to unmarshal hook: %v", trace.DebugReport(err))
			}
			hook.ReceiptHandle = aws.StringValue(m.ReceiptHandle)
			hook.QueueURL = queueURL
			if err := a.processEvent(ctx, operator, *hook); err != nil {
				a.Errorf("failed to process hook: %v", trace.DebugReport(err))
			}
		}
	}
}

func (a *Autoscaler) processEvent(ctx context.Context, operator Operator, event HookEvent) error {
	a.WithField("event", event).Info("Received autoscale event.")
	switch event.Type {
	case InstanceLaunching:
		if err := a.TurnOffSourceDestinationCheck(ctx, event.InstanceID); err != nil {
			return trace.Wrap(err)
		}
		if err := a.DeleteEvent(ctx, event); err != nil {
			return trace.Wrap(err)
		}
	case InstanceTerminating:
		if err := a.ensureInstanceTerminated(ctx, event); err != nil {
			return trace.Wrap(err)
		}
		if err := a.removeInstance(ctx, operator, event); err != nil && !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		if err := a.DeleteEvent(ctx, event); err != nil {
			return trace.Wrap(err)
		}
	default:
		log.Debugf("Discarding unsupported event %#v.", event)
		if err := a.DeleteEvent(ctx, event); err != nil {
			return trace.Wrap(err)
		}
		return trace.BadParameter("unsupported event: %v", event.Type)
	}
	return nil
}

func (a *Autoscaler) ensureInstanceTerminated(ctx context.Context, event HookEvent) error {
	log := a.WithField("instance", event.InstanceID)
	instance, err := a.DescribeInstance(ctx, event.InstanceID)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	if trace.IsNotFound(err) {
		log.Info("Instance is not found.")
		return nil
	}
	if instanceState(*instance) == ec2.InstanceStateNameTerminated {
		log.Info("Instance is already terminated.")
		return nil
	}
	log.Info("Waiting for instance to terminate.")
	if err = a.WaitUntilInstanceTerminated(ctx, event.InstanceID); err != nil {
		return trace.Wrap(err)
	}
	log.Info("Instance has been terminated.")
	return nil
}

func (a *Autoscaler) removeInstance(ctx context.Context, operator Operator, event HookEvent) error {
	cluster, err := operator.GetLocalSite(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	server, err := ops.FindServerByInstanceID(cluster, event.InstanceID)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = operator.CreateSiteShrinkOperation(ctx,
		ops.CreateSiteShrinkOperationRequest{
			AccountID:   cluster.AccountID,
			SiteDomain:  cluster.Domain,
			Servers:     []string{server.Hostname},
			Force:       true,
			NodeRemoved: true,
		})
	if err != nil {
		return trace.Wrap(err)
	}

	a.Debugf("initiated shrink operation for node %v", server.Hostname)
	return nil
}

func mustMarshalHook(e HookEvent) string {
	out, err := json.Marshal(e)
	if err != nil {
		panic(err)
	}
	return string(out)
}

func unmarshalHook(input string) (*HookEvent, error) {
	var out HookEvent
	err := json.Unmarshal([]byte(input), &out)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &out, nil
}
