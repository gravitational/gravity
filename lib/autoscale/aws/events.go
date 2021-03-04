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
	"strings"
	"time"

	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/utils"
	"github.com/sirupsen/logrus"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/autoscaling"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/sqs"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
)

const (
	// The AWS lifecycle events can only be held open for 100 times the heartbeat timeout which we set to 1 minute
	// for gravity in our terraform. So cancel trying to shrink the node sometime after AWS has terminated the node.
	// TODO: probably should be configurable
	eventTimeout = 110 * time.Minute
	// heartbeatInterval is the interval to send heartbeats to AWS that we're still working on the node and that the
	// ASG should not shutdown the node on us.
	heartbeatInterval = 25 * time.Second
	// monitorShrinkInterval is the interval to monitor the shrink operation in gravity.
	monitorShrinkInterval = 5 * time.Second
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
	// Token is the token to use when interacting with the lifecycle event
	Token string `json:"LifecycleActionToken"`
	// AutoScalingGroupName is the name of the AWS ASG
	AutoScalingGroupName string `json:"AutoScalingGroupName"`
	// LifecycleHookName is the name of the AWS Lifecycle hook
	LifecycleHookName string `json:"LifecycleHookName"`
}

// GetQueueURL returns queue URL associated with this cluster
func (a *Autoscaler) GetQueueURL(ctx context.Context) (string, error) {
	expr, err := regexp.Compile(`[^a-zA-Z0-9\-]`)
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
			a.WithError(err).Warn("Disabling Source/Destination check failed")
			// fallthrough, allow the reconciliation loop to retry later
		}
		if err := a.DeleteEvent(ctx, event); err != nil {
			return trace.Wrap(err)
		}
	case InstanceTerminating:
		go a.removeInstance(ctx, operator, event)

		if err := a.DeleteEvent(ctx, event); err != nil {
			return trace.Wrap(err)
		}
	default:
		a.Debugf("Discarding unsupported event %#v.", event)
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

func (a *Autoscaler) removeInstance(ctx context.Context, operator Operator, event HookEvent) {
	log := a.WithFields(logrus.Fields{
		"instance": event.InstanceID,
		"asg_name": event.AutoScalingGroupName,
	})

	// start sending heartbeats to tell AWS to keep the node alive
	heartbeatCancel := a.startHeartbeatLoop(event)
	defer heartbeatCancel()
	defer a.completeASGLifecycle(event)

	b := utils.NewExponentialBackOff(time.Hour)
	err := utils.RetryWithInterval(ctx, b, func() error {
		cluster, err := operator.GetLocalSite(ctx)
		if err != nil {
			log.WithError(err).Warn("error retrieving site")
			return trace.Wrap(err)
		}

		server, err := ops.FindServerByInstanceID(cluster, event.InstanceID)
		if err != nil {
			log.WithError(err).Warn("server not part of cluster, exiting shrink handling")
			return nil
		}

		opKey, err := operator.CreateSiteShrinkOperation(ctx,
			ops.CreateSiteShrinkOperationRequest{
				AccountID:   cluster.AccountID,
				SiteDomain:  cluster.Domain,
				Servers:     []string{server.Hostname},
				Force:       false,
				NodeRemoved: false,
			})

		// if the node is already offline, fallback to a force removal
		if err != nil && trace.IsBadParameter(err) && strings.Contains(err.Error(), "is offline") {
			a.WithError(err).Warn("node is offline, falling back to force removal")
			opKey, err = operator.CreateSiteShrinkOperation(ctx,
				ops.CreateSiteShrinkOperationRequest{
					AccountID:   cluster.AccountID,
					SiteDomain:  cluster.Domain,
					Servers:     []string{server.Hostname},
					Force:       true,
					NodeRemoved: true,
				})
		}

		if err != nil {
			a.WithError(err).Warn("failed to create shrink operation")
			return trace.Wrap(err)
		}

		a.WithFields(logrus.Fields{
			"instance": event.InstanceID,
			"asg_name": event.AutoScalingGroupName,
			"op_key":   opKey,
			"hostname": server.Hostname,
		}).Info("initiated shrink operation for node")

		a.monitorShrink(ctx, operator, event, opKey)

		return nil
	})

	if err != nil {
		log.WithError(err).Warn("Autoscaler failed to shrink cluster in response to ASG event")
	}
}

// monitorShrink monitors the cluster shrink operation and responds to failures of the shrink operation.
func (a *Autoscaler) monitorShrink(
	ctx context.Context,
	operator Operator,
	event HookEvent,
	opKey *ops.SiteOperationKey) {
	log := a.WithFields(logrus.Fields{
		"instance": event.InstanceID,
		"asg_name": event.AutoScalingGroupName,
		"op_key":   opKey,
	})

	ctx, cancel := context.WithTimeout(ctx, eventTimeout)
	defer cancel()

	ticker := time.NewTicker(monitorShrinkInterval)

	for {
		select {
		case <-ctx.Done():
			log.Warn("Exiting ASG shrink monitor due to reaching max lifetime")
			return
		case <-ticker.C:
			progress, err := operator.GetSiteOperationProgress(*opKey)
			if err != nil {
				log.WithError(err).Warn("failed to retrieve progress entry")
				continue
			}

			if progress.IsCompleted() {
				return
			}

			// if the shrink operation fails, let AWS shutdown the instance by completing the lifecycle event, and then
			// try to run a force shrink on an offline node
			if progress.State == ops.ProgressStateFailed {
				err := a.completeASGLifecycle(event)
				if err != nil {
					// AWS might return errors if we try and complete an ASG lifecycle that is no longer valid due to
					// timeout. So skip the error and continue with the force shrink.
					log.WithError(err).Warn("failed to send AWS ASG completion event")
				}

				err = a.ensureInstanceTerminated(ctx, event)
				if err != nil {
					log.WithError(err).Warn("failed to ensure AWS instance has been terminated")
					continue
				}

				err = a.forceShrink(ctx, operator, event)
				if err != nil {
					log.WithError(err).Warn("failed to force shrink operation")
					continue
				}

				// Once we've started the force shrink operation, exit the monitoring loop. If the force shrink fails
				// this loop isn't really capable of recovering and shouldn't just continually try to shrink the node.
				return
			}
		}
	}
}

func (a *Autoscaler) completeASGLifecycle(event HookEvent) error {
	_, err := a.Config.AutoScaling.CompleteLifecycleAction(&autoscaling.CompleteLifecycleActionInput{
		AutoScalingGroupName:  aws.String(event.AutoScalingGroupName),
		InstanceId:            aws.String(event.InstanceID),
		LifecycleActionToken:  aws.String(event.Token),
		LifecycleHookName:     aws.String(event.LifecycleHookName),
		LifecycleActionResult: aws.String("CONTINUE"),
	})

	a.WithError(err).WithFields(log.Fields{
		"instance": event.InstanceID,
		"asg_name": event.AutoScalingGroupName,
	}).Info("notified AWS of completed uninstall")

	return trace.Wrap(err)
}

func (a *Autoscaler) startHeartbeatLoop(event HookEvent) context.CancelFunc {
	ctx, cancel := context.WithTimeout(context.Background(), eventTimeout)

	go func() {
		ticker := time.NewTicker(heartbeatInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				a.WithFields(log.Fields{
					"instance":   event.InstanceID,
					"asg_name":   event.AutoScalingGroupName,
					"ctx_result": ctx.Err(),
				}).Info("heartbeat loop exiting")

				return
			case <-ticker.C:
				// we're still uninstalling so record a heartbeat with AWS to keep the node alive
				_, err := a.Config.AutoScaling.RecordLifecycleActionHeartbeat(&autoscaling.RecordLifecycleActionHeartbeatInput{
					AutoScalingGroupName: aws.String(event.AutoScalingGroupName),
					InstanceId:           aws.String(event.InstanceID),
					LifecycleActionToken: aws.String(event.Token),
					LifecycleHookName:    aws.String(event.LifecycleHookName),
				})

				a.WithError(err).WithFields(log.Fields{
					"instance": event.InstanceID,
					"asg_name": event.AutoScalingGroupName,
				}).Info("sent heartbeat for lifecycle event")
			}
		}
	}()

	return cancel
}

func (a *Autoscaler) forceShrink(ctx context.Context, operator Operator, event HookEvent) error {
	a.WithFields(log.Fields{
		"instance": event.InstanceID,
		"asg_name": event.AutoScalingGroupName,
	}).Info("running shrink with force set")

	err := a.ensureInstanceTerminated(ctx, event)
	if err != nil {
		return trace.Wrap(err)
	}

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

	return trace.Wrap(err)
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
