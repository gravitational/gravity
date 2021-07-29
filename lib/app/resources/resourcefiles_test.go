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
	"io/ioutil"
	"os"
	"sort"

	"github.com/gravitational/gravity/lib/compare"

	. "gopkg.in/check.v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type ResourceFilesSuite struct {
}

var _ = Suite(&ResourceFilesSuite{})

func (s *ResourceFilesSuite) TestResourceFiles(c *C) {
	files := prepareResourceFiles(c)
	var rFiles ResourceFiles
	for _, file := range files {
		rFile, err := NewResourceFile(file.Name())
		c.Assert(err, IsNil)
		rFiles = append(rFiles, *rFile)
	}
	sourceImages := sorted([]string{
		"dns-install-hook:0.0.1",
		"image1:1.0.0",
		"image2:2.0.0",
		"image3:3.0.0",
		"image4",
		"image5",
		"k8s-install-hook:0.0.1",
	})
	rewrittenImages := sorted([]string{
		"apiserver:5000/dns-install-hook:0.0.1",
		"apiserver:5000/image1:1.0.0",
		"apiserver:5000/image2:2.0.0",
		"apiserver:5000/image3:3.0.0",
		"apiserver:5000/image4",
		"apiserver:5000/image5",
		"apiserver:5000/k8s-install-hook:0.0.1",
	})

	images, err := rFiles.Images()
	c.Assert(err, IsNil)

	images = sorted(images)
	c.Assert(images, DeepEquals, sourceImages)

	err = rFiles.RewriteImages(func(image string) string {
		return "apiserver:5000/" + image
	})
	c.Assert(err, IsNil)

	images, err = rFiles.Images()
	c.Assert(err, IsNil)
	sort.Strings(images)
	c.Assert(images, DeepEquals, rewrittenImages)

	err = rFiles.Write()
	c.Assert(err, IsNil)

	rFiles = ResourceFiles{}
	for _, file := range files {
		rFile, err := NewResourceFile(file.Name())
		c.Assert(err, IsNil)
		rFiles = append(rFiles, *rFile)
	}

	images, err = rFiles.Images()
	c.Assert(err, IsNil)
	sort.Strings(images)
	c.Assert(images, DeepEquals, rewrittenImages)

	// verify that no resources were dropped
	c.Assert(headers(rFiles), compare.DeepEquals, []runtime.TypeMeta{
		{
			Kind:       "Bundle",
			APIVersion: "bundle.gravitational.io/v2",
		},
		{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		{
			Kind:       "CronTab",
			APIVersion: "stable.example.com/v1",
		},
		{
			Kind:       "CustomResourceDefinition",
			APIVersion: "apiextensions.k8s.io/v1beta1",
		},
		{
			Kind:       "DaemonSet",
			APIVersion: "extensions/v1beta1",
		},
		{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		{
			Kind:       "ReplicationController",
			APIVersion: "v1",
		},
		{
			Kind:       "ReplicationController",
			APIVersion: "v1",
		},
		{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		{
			Kind:       "Service",
			APIVersion: "v1",
		},
		{
			Kind:       "SystemApplication",
			APIVersion: "bundle.gravitational.io/v2",
		},
	})
}

func (s *ResourceFilesSuite) TestForEachObjectInFile(c *C) {
	file, err := ioutil.TempFile("", "test")
	c.Assert(err, IsNil)
	defer os.Remove(file.Name())

	_, err = file.Write([]byte(resources1))
	c.Assert(err, IsNil)

	err = file.Close()
	c.Assert(err, IsNil)

	var resources []string
	err = ForEachObjectInFile(file.Name(), func(object runtime.Object) error {
		resources = append(resources, object.GetObjectKind().GroupVersionKind().Kind)
		return nil
	})
	c.Assert(err, IsNil)
	c.Assert(resources, DeepEquals, []string{
		"ConfigMap",
		"Service",
		"DaemonSet",
		"ReplicationController",
		"Secret",
		"ReplicationController",
	})
}

func prepareResourceFiles(c *C) (files []*os.File) {
	dir := c.MkDir()

	for _, resource := range []string{resources1, resources2, manifest1, manifest2, unrecognizedResource} {
		file, err := ioutil.TempFile(dir, "resourcefilestest")
		c.Assert(err, IsNil)
		_, err = file.Write([]byte(resource))
		c.Assert(err, IsNil)
		files = append(files, file)
	}
	return files
}

// sorted returns s sorted in ascending order
func sorted(s []string) []string {
	sort.Strings(s)
	return s
}

// a pretty complex resources spec that resembles a real-world application
const resources1 = `apiVersion: v1
kind: ConfigMap
metadata:
  name: config
  labels:
    app: test
data:
  config.conf: |-
    KEY=VAL
---
apiVersion: v1
kind: Service
metadata:
  name: service
  labels:
    app: test
  annotations:
    service.beta.kubernetes.io/aws-load-balancer-internal: 0.0.0.0/0
spec:
  type: LoadBalancer
  selector:
    app: test
  ports:
    - name: http
      port: 80
---
apiVersion: extensions/v1beta1
kind: DaemonSet
metadata:
  name: ds
  labels:
    app: test
spec:
  selector:
    matchLabels:
      app: test
  template:
    metadata:
      labels:
        app: test
    spec:
      hostNetwork: true
      containers:
        - name: container1
          image: image1:1.0.0
          imagePullPolicy: Always
          command:
            - ./start.sh
          ports:
            - name: http
              containerPort: 80
          securityContext:
            privileged: true
          volumeMounts:
            - name: data
              mountPath: /data
      nodeSelector:
        app: test
      volumes:
        - name: data
          hostPath:
            path: /data
---
apiVersion: v1
kind: ReplicationController
metadata:
  name: rc1
  labels:
    app: test
spec:
  replicas: 1
  selector:
    app: test
  template:
    metadata:
      labels:
        app: test
    spec:
      hostNetwork: true
      containers:
        - name: container2
          image: image2:2.0.0
          command:
            - ./start.sh
          ports:
            - name: http
              containerPort: 80
          securityContext:
            privileged: true
          volumeMounts:
            - name: data
              mountPath: /data
      nodeSelector:
        app: test
      volumes:
        - name: data
          hostPath:
            path: /data
---
apiVersion: v1
kind: Secret
metadata:
  name: secret
  labels:
    app: test
type: Opaque
data:
  secret: c2VjcmV0Cg==
---
apiVersion: v1
kind: ReplicationController
metadata:
  name: rc2
  labels:
    app: test
spec:
  replicas: 1
  selector:
    app: test
  template:
    metadata:
      labels:
        app: test
    spec:
      hostNetwork: true
      containers:
        - name: container3
          image: image3:3.0.0
          env:
            - name: SECRET
              valueFrom:
                secretKeyRef:
                  name: secret
                  key: secret
          command:
            - ./start.sh
          volumeMounts:
            - name: data
              mountPath: /data
      nodeSelector:
        app: test
      volumes:
        - name: data
          hostPath:
            path: /data`

const resources2 = `apiVersion: v1
kind: Pod
metadata:
  name: pod1
  labels:
    app: test
spec:
  containers:
    - name: image4
      image: image4
---
apiVersion: v1
kind: Pod
metadata:
  name: pod2
  labels:
    app: test
spec:
  containers:
  - name: image5
    image: image5`

const manifest1 = `apiVersion: bundle.gravitational.io/v2
kind: Bundle
metadata:
  repository: gravitational.io
  namespace: kube-system
  name: k8s-aws
  resourceVersion: "1.2.3-1"
hooks:
  install:
    job: |
      apiVersion: batch/v1
      kind: Job
      metadata:
        name: install
      spec:
        template:
          metadata:
            name: install
          spec:
            restartPolicy: OnFailure
            containers:
              - name: hook
                image: k8s-install-hook:0.0.1
                command: ["sleep"]`

const manifest2 = `apiVersion: bundle.gravitational.io/v2
kind: SystemApplication
metadata:
  repository: gravitational.io
  namespace: kube-system
  name: dns-app
  resourceVersion: "0.0.1"
hooks:
  install:
    job: |
      apiVersion: batch/v1
      kind: Job
      metadata:
        name: dns-app-install
      spec:
        template:
          metadata:
            name: dns-app-install
          spec:
            restartPolicy: OnFailure
            containers:
              - name: hook
                image: dns-install-hook:0.0.1
                command: ["sleep"]`
