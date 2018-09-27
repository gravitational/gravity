/*
Copyright 2017 Gravitational, Inc.

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

package monitoring

import (
	"context"

	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/gravitational/satellite/agent/health"
	"github.com/gravitational/trace"
)

// NewAWSHasProfileChecker returns a new checker, that checks that the instance
// has a node profile assigned to it.
// TODO(knisbet): look into enhancing this to check the contents of the profile
// for missing permissions. However, for now this exists just as a basic check
// for instances that accidently lose their profile assignment.
func NewAWSHasProfileChecker() health.Checker {
	return &awsHasProfileChecker{}
}

type awsHasProfileChecker struct{}

// Name returns this checker name
// Implements health.Checker
func (*awsHasProfileChecker) Name() string {
	return awsHasProfileCheckerID
}

// Check will check the metadata API to see if an IAM profile is assigned to the node
// Implements health.Checker
func (*awsHasProfileChecker) Check(ctx context.Context, reporter health.Reporter) {
	session, err := session.NewSession()
	if err != nil {
		reporter.Add(NewProbeFromErr(awsHasProfileCheckerID, "failed to create session", trace.Wrap(err)))
		return
	}
	metadata := ec2metadata.New(session)

	_, err = metadata.IAMInfo()
	if err != nil {
		reporter.Add(NewProbeFromErr(awsHasProfileCheckerID, "failed to determine node IAM profile", trace.Wrap(err)))
		return
	}
	reporter.Add(NewSuccessProbe(awsHasProfileCheckerID))
}

// IsRunningOnAWS attempts to use the AWS metadata API to determine if the
// currently running node is an AWS node or not
func IsRunningOnAWS() bool {
	session := session.New()
	metadata := ec2metadata.New(session)
	return metadata.Available()
}

const (
	awsHasProfileCheckerID = "aws"
)
