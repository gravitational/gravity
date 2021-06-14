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
	"encoding/json"
	"sort"

	. "gopkg.in/check.v1"
)

type PermissionsSuite struct{}

var _ = Suite(&PermissionsSuite{})

func (r *PermissionsSuite) TestEncodesAsPolicyFile(c *C) {
	const expected = `{
    "Version": "2012-10-17",
    "Statement": [
      {
        "Effect": "Allow",
        "Action": [
          "ec2:CreateVpc",
          "ec2:DeleteVpc",
          "ec2:DescribeVpcs"
        ],
        "Resource": "*"
      },
      {
        "Effect": "Allow",
        "Action": [
          "iam:CreateRole",
          "iam:DeleteRole",
          "iam:PassRole"
        ],
        "Resource": "*"
      }
    ]
  }`

	var expectedPolicy policy
	c.Assert(json.Unmarshal([]byte(expected), &expectedPolicy), IsNil)
	sort.Sort(byContext(expectedPolicy.Statement))

	actions := Actions{
		{EC2, "CreateVpc"},
		{EC2, "DeleteVpc"},
		{EC2, "DescribeVpcs"},
		{IAM, "CreateRole"},
		{IAM, "DeleteRole"},
		{IAM, "PassRole"},
	}
	encodedJSON, err := actions.AsPolicy("2012-10-17")
	c.Assert(err, IsNil)

	var obtainedPolicy policy
	c.Assert(json.Unmarshal([]byte(encodedJSON), &obtainedPolicy), IsNil)
	sort.Sort(byContext(obtainedPolicy.Statement))

	c.Assert(expectedPolicy, DeepEquals, obtainedPolicy)
}

type byContext []rule

func (r byContext) Len() int      { return len(r) }
func (r byContext) Swap(i, j int) { r[i], r[j] = r[j], r[i] }
func (r byContext) Less(i, j int) bool {
	left := r[i].Action
	right := r[j].Action
	return len(left) > 0 && len(right) > 0 && left[0].Context < right[0].Context
}
