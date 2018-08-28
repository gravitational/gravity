package storage

import (
	"encoding/json"
	"testing"

	"github.com/gravitational/gravity/lib/compare"
	"github.com/gravitational/gravity/lib/defaults"

	teleservices "github.com/gravitational/teleport/lib/services"
	teleutils "github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	. "gopkg.in/check.v1"
)

func TestClusterParsing(t *testing.T) { TestingT(t) }

type ClusterSuite struct {
}

var _ = Suite(&ClusterSuite{})

func (s *ClusterSuite) SetUpSuite(c *C) {
	teleutils.InitLoggerForTests()
}

func (s *ClusterSuite) TestClusterParse(c *C) {
	testCases := []struct {
		in      string
		cluster *ClusterV2
		error   error
	}{
		{
			in:    ``,
			error: trace.BadParameter("empty input"),
		},
		{
			in:    `{}`,
			error: trace.BadParameter("failed to validate: name: name is required"),
		},
		{
			in:    `{"kind": "cluster"}`,
			error: trace.BadParameter("failed to validate: name: name is required"),
		},
		{
			in:    `{"kind": "cluster", "version": "v2", "metadata": {"name": "name1"}, "spec": {}}`,
			error: trace.BadParameter("failed to validate: missing properties"),
		},
		{
			in: `kind: cluster
version: v2
metadata:
  name: cluster-name
spec:
  app: telekube:4.14.0
  provider: aws
  aws:
    region: us-west-2
    vpc: vpc-abc123
    keyName: ops
  nodes:
  - profile: database
    count: 2
    instanceType: c3.xlarge
  - profile: leader
    count: 3
    instanceType: m4.xlarge
`,
			cluster: &ClusterV2{
				Kind:    KindCluster,
				Version: teleservices.V2,
				Metadata: teleservices.Metadata{
					Name:      "cluster-name",
					Namespace: defaults.Namespace,
				},
				Spec: ClusterSpecV2{
					App:      "telekube:4.14.0",
					Provider: "aws",
					AWS: &ClusterAWSProviderSpecV2{
						Region:  "us-west-2",
						VPC:     "vpc-abc123",
						KeyName: "ops",
					},
					Nodes: []ClusterNodeSpecV2{
						{
							Profile:      "database",
							Count:        2,
							InstanceType: "c3.xlarge",
						},
						{
							Profile:      "leader",
							Count:        3,
							InstanceType: "m4.xlarge",
						},
					},
				},
			},
		},
	}
	for i, tc := range testCases {
		comment := Commentf("test case %v", i)
		cluster, err := UnmarshalCluster([]byte(tc.in))
		if tc.error != nil {
			c.Assert(err, NotNil, comment)
		} else {
			c.Assert(err, IsNil, comment)
			compare.DeepCompare(c, cluster, tc.cluster)

			out, err := json.Marshal(cluster)
			c.Assert(err, IsNil, comment)

			cluster2, err := UnmarshalCluster(out)
			c.Assert(err, IsNil, comment)
			compare.DeepCompare(c, cluster2, tc.cluster)
		}
	}
}
