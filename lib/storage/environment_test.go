/*
Copyright 2019 Gravitational, Inc.

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

package storage

import (
	"fmt"

	"github.com/gravitational/gravity/lib/compare"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"

	teleservices "github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
	. "gopkg.in/check.v1"
)

type EnvS struct{}

var _ = Suite(&EnvS{})

func (*EnvS) TestCheckAndSetDefaults(c *C) {
	testCases := []struct {
		vars    string
		error   bool
		comment string
	}{
		{
			vars:    `{"http_proxy": "proxy.example.com"}`,
			error:   true,
			comment: "http proxy missing scheme",
		},
		{
			vars:    `{"http_proxy": "http://proxy.example.com", "https_proxy": "proxy2.example.com"}`,
			error:   true,
			comment: "https proxy missing scheme",
		},
		{
			vars:    `{"https_proxy": "http://proxy2.example.com"}`,
			error:   true,
			comment: "https proxy has incorrect scheme",
		},
		{
			vars:    `{"http_proxy": "http://proxy.example.com", "https_proxy": "https://proxy2.example.com"}`,
			error:   false,
			comment: "http and https proxies are ok",
		},
		{
			vars:    `{"http_proxy": "", "https_proxy": ""}`,
			error:   false,
			comment: "empty http and https proxies are ok",
		},
		{
			vars:    `{}`,
			error:   false,
			comment: "empty resource is ok",
		},
	}
	for _, test := range testCases {
		data := fmt.Sprintf(`{"kind": "runtimeenvironment", "metadata": {"name": "foo"}, "version": "v1", "spec": {"data": %v}}`, test.vars)
		env, err := UnmarshalEnvironmentVariables([]byte(data))
		c.Assert(err, IsNil)

		err = env.CheckAndSetDefaults()
		if test.error {
			c.Assert(err, NotNil, Commentf(test.comment))
		} else {
			c.Assert(err, IsNil, Commentf(test.comment))
		}
	}
}

func (*EnvS) TestParsesEnvironment(c *C) {
	testCases := []struct {
		in      string
		env     *EnvironmentV1
		error   error
		comment string
	}{
		{
			in:      `{}`,
			error:   trace.BadParameter("failed to validate: name: name is required"),
			comment: "fails for empty json",
		},
		{
			in:      `{"kind": "runtimeenvironment"}`,
			error:   trace.BadParameter("failed to validate: name: name is required"),
			comment: "invalid with missing required fields",
		},
		{
			in: `{"kind": "runtimeenvironment", "metadata": {"name": "foo"}, "version": "v1", "spec": {"data": {"foo": "bar"}}}`,
			env: &EnvironmentV1{
				Kind:    KindRuntimeEnvironment,
				Version: "v1",
				Metadata: teleservices.Metadata{
					Name:      constants.ClusterEnvironmentMap,
					Namespace: defaults.KubeSystemNamespace,
				},
				Spec: EnvironmentSpec{
					KeyValues: map[string]string{
						"foo": "bar",
					},
				},
			},
			comment: "overrides metadata.name and metadata.namespace",
		},
		{
			in: `{"kind": "runtimeenvironment", "metadata": {"name": "foo"}, "version": "v1", "spec": {"data": null}}`,
			env: &EnvironmentV1{
				Kind:    KindRuntimeEnvironment,
				Version: "v1",
				Metadata: teleservices.Metadata{
					Name:      constants.ClusterEnvironmentMap,
					Namespace: defaults.KubeSystemNamespace,
				},
			},
			comment: "missing (empty) spec is ok",
		},
		{
			in: `kind: runtimeenvironment
version: v1
spec:
  data:
    foo: bar
    HTTP_PROXY: "example.com:8081"
`,
			env: &EnvironmentV1{
				Kind:    KindRuntimeEnvironment,
				Version: "v1",
				Metadata: teleservices.Metadata{
					Name:      constants.ClusterEnvironmentMap,
					Namespace: defaults.KubeSystemNamespace,
				},
				Spec: EnvironmentSpec{
					KeyValues: map[string]string{
						"foo":        "bar",
						"HTTP_PROXY": "example.com:8081",
					},
				},
			},
			comment: "parses the correct spec",
		},
	}
	for _, tc := range testCases {
		comment := Commentf(tc.comment)
		env, err := UnmarshalEnvironmentVariables([]byte(tc.in))
		if tc.error != nil {
			c.Assert(err, FitsTypeOf, tc.error, comment)
			continue
		}
		c.Assert(err, IsNil, comment)
		c.Assert(env, compare.DeepEquals, tc.env, comment)

		bytes, err := MarshalEnvironment(env)
		c.Assert(err, IsNil, comment)

		env2, err := UnmarshalEnvironmentVariables([]byte(bytes))
		c.Assert(err, IsNil, comment)
		c.Assert(env2, compare.DeepEquals, env, comment)
	}
}
