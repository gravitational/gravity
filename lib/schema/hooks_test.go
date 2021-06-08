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

package schema

import (
	"reflect"

	. "gopkg.in/check.v1"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
)

type HooksSuite struct{}

var _ = Suite(&HooksSuite{})

func (r *HooksSuite) TestEnsureAllHooksAccountedFor(c *C) {
	hooksType := reflect.TypeOf(Hooks{})
	hooksBaseType := reflect.TypeOf(Hook{})
	value := reflect.New(hooksType).Elem()

	var hookNames []string
	for i := 0; i < value.NumField(); i++ {
		field := value.Field(i)
		field.Set(reflect.New(hooksBaseType))
		fieldType := hooksType.Field(i)
		if fieldType.Type.String() == "*schema.Hook" {
			hookNames = append(hookNames, fieldType.Name)
		}
	}
	hooks := value.Interface().(Hooks)

	c.Assert(len(hookNames), Equals, len(hooks.AllHooks()),
		Commentf("missing a hook from this test, or Hooks.AllHooks()"))
	c.Assert(len(hookNames), Equals, len(AllHooks()),
		Commentf("missing a hook from this test, or package level AllHooks() method"))
}

func (r *HooksSuite) TestDecodesHooks(c *C) {
	const manifest = `
apiVersion: bundle.gravitational.io/v2
kind: Bundle
metadata:
  name: test
  resourceVersion: 0.0.1
hooks:
  install:
    job: |
      apiVersion: batch/v1
      kind: Job
      metadata:
        name: expand
      spec:
        template:
          metadata:
            name: expand
          spec:
            containers:
            - name: expand
              image: hook:0.0.1
              command: ["/var/scripts/prepare-env.sh"]
              volumeMounts:
              - mountPath: /var/scripts
                name: scripts
              - mountPath: /var/config
                name: config
            volumes:
            - name: scripts
              hostPath:
                path: /var/scripts
            - name: config
              configMap:
                name: config`
	m, err := ParseManifestYAMLNoValidate([]byte(manifest))
	c.Assert(err, IsNil)

	job := &batchv1.Job{}
	job.Kind = "Job"
	job.APIVersion = "batch/v1"
	job.ObjectMeta.Name = "expand"
	job.Spec.Template.ObjectMeta.Name = "expand"
	job.Spec.Template.Spec.Containers = []v1.Container{
		{
			Name:    "expand",
			Image:   "hook:0.0.1",
			Command: []string{"/var/scripts/prepare-env.sh"},
			VolumeMounts: []v1.VolumeMount{
				{
					Name:      "scripts",
					MountPath: "/var/scripts",
				},
				{
					Name:      "config",
					MountPath: "/var/config",
				},
			},
		},
	}
	job.Spec.Template.Spec.Volumes = []v1.Volume{
		{
			Name: "scripts",
			VolumeSource: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: "/var/scripts",
				},
			},
		},
		{
			Name: "config",
			VolumeSource: v1.VolumeSource{
				ConfigMap: &v1.ConfigMapVolumeSource{
					LocalObjectReference: v1.LocalObjectReference{
						Name: "config",
					},
				},
			},
		},
	}

	installJob, err := m.Hooks.Install.GetJob()
	c.Assert(err, IsNil)
	c.Assert(installJob, DeepEquals, job)
}
