// Copyright 2021 Gravitational Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ops

import (
	"testing"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/storage"

	teledefaults "github.com/gravitational/teleport/lib/defaults"
	"gopkg.in/check.v1"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func TestOps(t *testing.T) { check.TestingT(t) }

type OpsSuite struct{}

var _ = check.Suite(&OpsSuite{})

func (s *OpsSuite) TestServicesFromEndpoints(c *check.C) {
	type testCase struct {
		// publicAdvertiseAddr is the configured advertise addr for user traffic
		publicAdvertiseAddr string
		// agentsAdvertiseAddr is the configured advertise addr for cluster traffic
		agentsAdvertiseAddr string
		// publicService is the expected service for user traffic
		publicService *v1.Service
		// agentsService is the expected service for cluster traffic
		agentsService *v1.Service
		// description is the test case description
		description string
	}
	testCases := []testCase{
		{
			description:         "traffic is not split",
			publicAdvertiseAddr: "ops.example.com:443",
			publicService: makeService(constants.GravityPublicService, []v1.ServicePort{
				{Name: "public", Port: 443, TargetPort: intstr.FromInt(defaults.GravityListenPort)},
				{Name: "sshtunnel", Port: teledefaults.SSHProxyTunnelListenPort},
				{Name: "sshproxy", Port: teledefaults.SSHProxyListenPort},
			}),
			agentsService: makeService(constants.GravityAgentsService, nil),
		},
		{
			description:         "same host, different port",
			publicAdvertiseAddr: "ops.example.com:443",
			agentsAdvertiseAddr: "ops.example.com:444",
			publicService: makeService(constants.GravityPublicService, []v1.ServicePort{
				{Name: "public", Port: 443, TargetPort: intstr.FromInt(defaults.GravityPublicListenPort)},
				{Name: "agents", Port: 444, TargetPort: intstr.FromInt(defaults.GravityListenPort)},
				{Name: "sshtunnel", Port: teledefaults.SSHProxyTunnelListenPort},
				{Name: "sshproxy", Port: teledefaults.SSHProxyListenPort},
			}),
			agentsService: makeService(constants.GravityAgentsService, nil),
		},
		{
			description:         "different host, same port",
			publicAdvertiseAddr: "ops1.example.com:443",
			agentsAdvertiseAddr: "ops2.example.com:443",
			publicService: makeService(constants.GravityPublicService, []v1.ServicePort{
				{Name: "public", Port: 443, TargetPort: intstr.FromInt(defaults.GravityPublicListenPort)},
				{Name: "sshproxy", Port: teledefaults.SSHProxyListenPort},
			}),
			agentsService: makeService(constants.GravityAgentsService, []v1.ServicePort{
				{Name: "agents", Port: 443, TargetPort: intstr.FromInt(defaults.GravityListenPort)},
				{Name: "sshtunnel", Port: teledefaults.SSHProxyTunnelListenPort},
			}),
		},
		{
			description:         "different host, different port",
			publicAdvertiseAddr: "ops1.example.com:443",
			agentsAdvertiseAddr: "ops2.example.com:444",
			publicService: makeService(constants.GravityPublicService, []v1.ServicePort{
				{Name: "public", Port: 443, TargetPort: intstr.FromInt(defaults.GravityPublicListenPort)},
				{Name: "sshproxy", Port: teledefaults.SSHProxyListenPort},
			}),
			agentsService: makeService(constants.GravityAgentsService, []v1.ServicePort{
				{Name: "agents", Port: 444, TargetPort: intstr.FromInt(defaults.GravityListenPort)},
				{Name: "sshtunnel", Port: teledefaults.SSHProxyTunnelListenPort},
			}),
		},
	}
	for _, testCase := range testCases {
		endpoints := storage.NewEndpoints(storage.EndpointsSpecV2{
			PublicAddr: testCase.publicAdvertiseAddr,
			AgentsAddr: testCase.agentsAdvertiseAddr,
		})
		publicService, agentsService, err := ServicesFromEndpoints(endpoints)
		c.Assert(err, check.IsNil,
			check.Commentf("failed test case: %v", testCase.description))
		c.Assert(publicService, check.DeepEquals, testCase.publicService,
			check.Commentf("failed test case: %v", testCase.description))
		c.Assert(agentsService, check.DeepEquals, testCase.agentsService,
			check.Commentf("failed test case: %v", testCase.description))
	}
}

func makeService(name string, ports []v1.ServicePort) *v1.Service {
	return &v1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       constants.KindService,
			APIVersion: constants.ServiceAPIVersion,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: defaults.KubeSystemNamespace,
			Labels: map[string]string{
				defaults.ApplicationLabel: defaults.GravityOpsCenterLabel,
			},
			Annotations: map[string]string{
				constants.AWSLBIdleTimeoutAnnotation: defaults.LBIdleTimeout,
			},
		},
		Spec: v1.ServiceSpec{
			Type:     v1.ServiceTypeLoadBalancer,
			Selector: defaults.GravitySiteSelector,
			Ports:    ports,
		},
	}
}
