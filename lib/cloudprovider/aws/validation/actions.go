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
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gravitational/trace"
)

// Context defines an action context (EC2, IAM etc)
type Context byte

const (
	// EC2 action context
	EC2 Context = 0
	// IAM action context
	IAM = 1
)

// Action defines a single AWS context resource action
// Contexts are, for instance, EC2 or IAM
type Action struct {
	Context Context
	Name    string
}

// ParseAction parses the provided string of format "ec2:PermissionsName" into an Action object
func ParseAction(action string) (*Action, error) {
	parts := strings.Split(action, ":")
	if len(parts) != 2 {
		return nil, trace.BadParameter(
			`invalid action format %q, expected "ec2:APIName" or "iam:APIName"`, action)
	}
	var context Context
	switch parts[0] {
	case "ec2":
		context = EC2
	case "iam":
		context = IAM
	default:
		return nil, trace.BadParameter("unsupported AWS API context %q", parts[0])
	}
	return &Action{Context: context, Name: parts[1]}, nil
}

// Actions is a list of resource actions
type Actions []Action

// AsPolicy formats the specified set of actions as a AWS policy file
func (r Actions) AsPolicy(policyVersion string) (string, error) {
	if policyVersion == "" {
		return "", trace.BadParameter("invalid policy version")
	}
	var policy = policy{
		Version: policyVersion,
	}
	rules := map[Context][]Action{}
	for _, action := range r {
		rules[action.Context] = append(rules[action.Context], action)
	}
	for _, actions := range rules {
		policy.Statement = append(policy.Statement, rule{
			Effect:   "Allow",
			Resource: "*",
			Action:   actions,
		})
	}
	jsonBytes, err := json.MarshalIndent(&policy, "  ", "  ")
	return string(jsonBytes), err
}

// String returns a string representation of a Context
func (r Context) String() string {
	switch r {
	case EC2:
		return "ec2"
	case IAM:
		return "iam"
	default:
		return ""
	}
}

// MarshalJSON formats this Action value as JSON
func (r Action) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf(`"%v:%v"`, r.Context, r.Name)), nil
}

// UnmarshalJSON reads an Action value from JSON
func (r *Action) UnmarshalJSON(data []byte) (err error) {
	err = (&r.Context).UnmarshalText(data[1:4])
	r.Name = string(data[4:])
	return trace.Wrap(err)
}

var ec2Context = []byte("ec2")
var iamContext = []byte("iam")

// MarshalText formats a Context value as text
func (r Context) MarshalText() ([]byte, error) {
	switch r {
	case EC2:
		return ec2Context, nil
	case IAM:
		return iamContext, nil
	default:
		return nil, trace.BadParameter("invalid context value: %v", r)
	}
}

// UnmarshalText reads a Context value from text
func (r *Context) UnmarshalText(data []byte) error {
	if bytes.Equal(data, ec2Context) {
		*r = EC2
		return nil
	} else if bytes.Equal(data, iamContext) {
		*r = IAM
		return nil
	}
	return trace.BadParameter("invalid context value: %s", data)
}

type policy struct {
	Version   string
	Statement []rule
}

type rule struct {
	Effect   string
	Action   []Action
	Resource string
}
