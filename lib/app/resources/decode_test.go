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

package resources

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/gravitational/gravity/lib/compare"

	. "gopkg.in/check.v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
)

type ResourceCodecSuite struct{}

func TestResourceCodec(t *testing.T) { TestingT(t) }

var _ = Suite(&ResourceCodecSuite{})

func (*ResourceCodecSuite) TestDecodesAndEncodes(c *C) {
	var testCases = []struct {
		resource string
		types    []string
	}{
		{resource: resourcesYAML, types: []string{"Service", "Service", "Pod", "Pod"}},
		{resource: resourcesJSON, types: []string{"Service", "ReplicationController"}},
	}

	for _, testCase := range testCases {
		r := strings.NewReader(testCase.resource)
		resource, err := Decode(r)
		c.Assert(err, IsNil)

		buf := &bytes.Buffer{}
		c.Assert(resource.Encode(buf), IsNil)

		resource, err = Decode(buf)
		c.Assert(err, IsNil)

		var types []string
		for _, o := range resource.Objects {
			types = append(types, o.GetObjectKind().GroupVersionKind().Kind)
		}
		c.Assert(types, DeepEquals, testCase.types)
	}
}

func (*ResourceCodecSuite) TestDecodeUnrecognizedResource(c *C) {
	r := strings.NewReader(unrecognizedResource)

	resources, err := Decode(r)
	c.Assert(err, IsNil)
	c.Assert(headers(ResourceFiles{{Resource: *resources}}), compare.DeepEquals, []runtime.TypeMeta{
		{
			Kind:       "CronTab",
			APIVersion: "stable.example.com/v1",
		},
		{
			Kind:       "CustomResourceDefinition",
			APIVersion: "apiextensions.k8s.io/v1beta1",
		},
	})
}

func (*ResourceCodecSuite) TestEncodesInProperFormat(c *C) {
	var testCases = []struct {
		resource string
		isJSON   bool
	}{
		{resource: resourcesYAML, isJSON: false},
		{resource: resourcesJSON, isJSON: true},
	}
	const bufferSize = 128

	for _, testCase := range testCases {
		r := strings.NewReader(testCase.resource)
		resource, err := Decode(r)
		c.Assert(err, IsNil)

		buf := &bytes.Buffer{}
		c.Assert(resource.Encode(buf), IsNil)

		_, _, isJSON := yaml.GuessJSONStream(buf, bufferSize)
		c.Assert(testCase.isJSON, Equals, isJSON)
	}
}

func headers(files ResourceFiles) (metas []runtime.TypeMeta) {
	for _, file := range files {
		for _, o := range file.Objects {
			gvk := o.GetObjectKind().GroupVersionKind()
			meta := runtime.TypeMeta{
				APIVersion: fmt.Sprintf("%v/%v", gvk.Group, gvk.Version),
				Kind:       gvk.Kind,
			}
			if gvk.Group == "" {
				meta.APIVersion = gvk.Version
			}
			metas = append(metas, meta)
		}
	}
	sort.Slice(metas, func(i, j int) bool {
		return metas[i].Kind < metas[j].Kind
	})
	return metas
}

const resourcesYAML = `
apiVersion: v1
kind: Service
metadata:
  name: postgres
  labels:
    app: mattermost
    role: mattermost-database
spec:
  type: NodePort
  ports:
  - port: 5432
  selector:
    role: mattermost-database
---
apiVersion: v1
kind: Service
metadata:
  name: mattermost
  labels:
    app: mattermost
    role: mattermost-worker
spec:
  type: NodePort
  ports:
  - port: 80
    name: http
  selector:
    role: mattermost-worker
---
apiVersion: v1
kind: Pod
metadata:
  name: mattermost-worker
  labels:
    app: mattermost
    role: mattermost-worker
spec:
  containers:
  - name: mattermost-worker
    image: local.registry:5055/mattermost-worker:1.2.1
    ports:
      - containerPort: 80
  nodeSelector:
    role: worker
---
apiVersion: v1
kind: Pod
metadata:
  name: mattermost-database
  labels:
    app: mattermost
    role: mattermost-database
spec:
  containers:
  - name: mattermost-postgres
    image: local.registry:5055/mattermost-postgres:9.4.4
    volumeMounts:
      - mountPath: /var/lib/postgresql/data
        name: database0
    ports:
      - containerPort: 5432
  nodeSelector:
    role: database
  volumes:
    - name: database0
      hostPath:
        path: /var/database
`

const resourcesJSON = `
{
   "kind":"Service",
   "apiVersion":"v1",
   "metadata":{
     "name":"mock",
     "labels":{
       "app":"mock"
     }
   },
   "spec":{
     "ports": [{
       "protocol": "TCP",
       "port": 99,
       "targetPort": 9949
     }],
     "selector":{
       "app":"mock"
     }
   }
}
{
   "kind":"ReplicationController",
   "apiVersion":"v1",
   "metadata":{
     "name":"mock",
     "labels":{
       "app":"mock"
     }
   },
   "spec":{
     "replicas":1,
     "selector":{
       "app":"mock"
     },
     "template":{
       "metadata":{
         "labels":{
           "app":"mock"
         }
       },
       "spec":{
         "containers":[{
           "name": "mock-container",
           "image": "gcr.io/google-containers/pause:2.0",
           "ports":[{
             "containerPort":9949,
             "protocol":"TCP"
           }]
         }]
       }
     }
   }
}
`

const unrecognizedResource = `apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: crontabs.stable.example.com
spec:
  group: stable.example.com
  versions:
    - name: v1
      served: true
      storage: true
  scope: Namespaced
  names:
    plural: crontabs
    singular: crontab
    kind: CronTab
    shortNames:
    - ct
---
apiVersion: stable.example.com/v1
kind: CronTab
metadata:
  name: my-new-cron-object
spec:
  cronSpec: "* * * * */5"
  image: my-awesome-cron-image
`
