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
	"testing"

	"golang.org/x/net/context"

	. "gopkg.in/check.v1"
)

func TestValidation(t *testing.T) { TestingT(t) }

type ValidationSuite struct{}

var _ = Suite(&ValidationSuite{})

func (r *ValidationSuite) TestAddsDependencies(c *C) {
	validator := fakeValidator{}
	client := &clientContext{}
	probes := Probes{
		{Action{IAM, "AddRoleToInstanceProfile"}, validateAddRoleToInstanceProfile},
	}
	actions, err := validateWithContext(context.TODO(), client, probes, validator)
	c.Assert(err, IsNil)
	c.Assert(actions, DeepEquals, Actions{
		{IAM, "AddRoleToInstanceProfile"},
		{IAM, "PassRole"},
	})
}

func (r *ValidationSuite) TestEmptyStatementAlwaysValidates(c *C) {
	validator := fakeValidator{}
	client := &clientContext{}
	var probes Probes
	actions, err := validateWithContext(context.TODO(), client, probes, validator)
	c.Assert(err, IsNil)
	c.Assert(actions, IsNil)
}

type fakeValidator struct{}

func (r fakeValidator) Do(client *clientContext, probe ResourceProbe) (bool, error) {
	return false, nil
}
