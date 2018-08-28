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
	encodedJson, err := actions.AsPolicy("2012-10-17")
	c.Assert(err, IsNil)

	var obtainedPolicy policy
	c.Assert(json.Unmarshal([]byte(encodedJson), &obtainedPolicy), IsNil)
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
