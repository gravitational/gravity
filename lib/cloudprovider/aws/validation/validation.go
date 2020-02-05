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
	"net/http"
	"strings"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/trace"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/iam"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
)

// Validate validates the specified AWS API key has access to the specified set of
// resources.
// Returns the list of actions this account does not have access to.
func Validate(accessKey, secretKey, sessionToken, regionName string, probes Probes, ctx context.Context) (actions Actions, err error) {
	creds := credentials.NewStaticCredentials(accessKey, secretKey, sessionToken)
	return ValidateWithCreds(creds, regionName, probes, ctx)
}

// ValidateWithCreds is an overload of Validate accepting specified credentials object.
func ValidateWithCreds(creds *credentials.Credentials, regionName string, probes Probes, ctx context.Context) (actions Actions, err error) {
	session, err := session.NewSession()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	config := &aws.Config{
		Credentials: creds,
		Region:      aws.String(regionName),
	}

	clientCtx := &clientContext{
		ec2: ec2.New(session, config),
		iam: iam.New(session, config),
	}

	actions, err = validateWithContext(clientCtx, probes, resourceValidatorFunc(validateResource), ctx)
	return actions, trace.Wrap(err)
}

func validateWithContext(clientCtx *clientContext, probes Probes, validator resourceValidator, ctx context.Context) (actions Actions, err error) {
	var errors []error

	if len(probes) < 1 {
		return actions, nil
	}

	log.Infof("Running validation probes...")

	// before running checks on all permissions, quickly try to check the first one
	// to make sure the provided keys are valid at all
	if _, err = validator.Do(clientCtx, probes[0]); err != nil {
		return nil, trace.Wrap(err)
	}

	type result struct {
		ok    bool
		err   error
		probe ResourceProbe
	}

	semaphoreC := make(chan struct{}, defaults.MaxValidationConcurrency)
	resultC := make(chan result, len(probes))

	for _, probe := range probes {
		select {
		case semaphoreC <- struct{}{}:
			go func(probe ResourceProbe) {
				// TODO: make direct use of request.Request objects
				// wrapped into ctxhttp.Do calls to use context effectively
				ok, err := validator.Do(clientCtx, probe)
				resultC <- result{ok, trace.Wrap(err), probe}
				<-semaphoreC
			}(probe)
		case <-ctx.Done():
			break
		}
	}

	for range probes {
		select {
		case r := <-resultC:
			if r.err == nil {
				if !r.ok {
					log.Infof("Permission is missing: %s.", r.probe.Action)

					actions = append(actions, r.probe.Action)
					if dependencies, ok := ActionDependencies[r.probe.Action]; ok {
						actions = append(actions, dependencies...)
					}
				}
			} else {
				errors = append(errors, r.err)
				err = trace.Unwrap(r.err)
				if awsErr, ok := err.(awserr.Error); ok {
					log.Infof("Probe failed for %s: %v (Code=%v, Message=%v).",
						r.probe.Action, err, awsErr.Code(), awsErr.Message())
				}
			}
		case <-ctx.Done():
			break
		}
	}
	close(semaphoreC)
	close(resultC)

	return actions, trace.NewAggregate(errors...)
}

var AllProbes Probes

// ActionDependencies assigns an action a set of dependent action permissions.
//
// For instance, as a permission, `iam:PassRole` cannot be verified with API -
// instead, if the `iam:AddRoleToInstanceProfile` action is used, the PassRole
// permission is implicitly required.
var ActionDependencies = map[Action][]Action{
	// Adding a role to instance profile implies enabled PassRole permission
	{IAM, "AddRoleToInstanceProfile"}: {{IAM, "PassRole"}},
}

type Probes []ResourceProbe

func init() {
	AllProbes = append(AllProbes, EC2Probes...)
	AllProbes = append(AllProbes, IAMProbes...)
}

// resourceValidator abstract the action of validating access to an AWS resource
// It exists for testing purposes
type resourceValidator interface {
	Do(client *clientContext, probe ResourceProbe) (bool, error)
}

// resourceValidatorFunc validates the resource described by probe
// It implements the resourceValidator interface
type resourceValidatorFunc func(client *clientContext, probe ResourceProbe) (bool, error)

// Do validates access to the resource specified with probe using client for API
func (r resourceValidatorFunc) Do(client *clientContext, probe ResourceProbe) (bool, error) {
	ok, err := r(client, probe)
	return ok, trace.Wrap(err)
}

// validateResource validates access to the resource specified with probe using
// client for API
func validateResource(client *clientContext, probe ResourceProbe) (bool, error) {
	if err := probe.probe(client); err != nil {
		srcErr := trace.Unwrap(err)

		switch awsErr := srcErr.(type) {
		case awserr.RequestFailure:
			switch awsErr.StatusCode() {
			case http.StatusForbidden:
				return false, nil
			case http.StatusUnauthorized:
				return false, trace.Wrap(err, "invalid AWS credentials")
			case http.StatusNotFound:
				return true, nil
			}
		}
		if awsErr, ok := srcErr.(awserr.Error); ok {
			// Not all operations support DryRun flag
			// The success of such operation is distinguished
			// based on the error code:
			// NotFound stands for success, while AccessDenied is an error
			if strings.HasSuffix(awsErr.Code(), ".NotFound") ||
				// FIXME: this needs more work - ignoring for now
				strings.HasSuffix(awsErr.Code(), ".Malformed") {
				return true, nil
			}
			// An error of type DryRunOperation means a successful operation
			// but with the DryRun flag set to True
			if awsErr.Code() == "DryRunOperation" {
				return true, nil
			}
		}
		return false, trace.Wrap(err)
	}
	return true, nil
}

// ResourceProbe defines an AWS resource probe context
type ResourceProbe struct {
	Action
	probe func(clientContext *clientContext) error
}

// clientContext groups clients for different AWS contexts (EC2, IAM, etc)
type clientContext struct {
	ec2 *ec2.EC2
	iam *iam.IAM
}
