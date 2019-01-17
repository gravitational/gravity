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

package cli

import (
	"bytes"
	"io"
	"os"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/modules"
	"github.com/gravitational/gravity/lib/ops/resources"
	"github.com/gravitational/gravity/lib/ops/resources/gravity"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/tool/common"

	teleservices "github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
	"k8s.io/apimachinery/pkg/util/yaml"
)

// createResource updates or inserts one or many resources from the specified filename.
// upsert controls whether the resource is expected to exist.
// manual controls whether the operation is created in manual mode if resource creation is implemented
// as a cluster operation.
// confirmed specifies if the user has explicitly approved the operation
func createResource(env *localenv.LocalEnvironment, factory LocalEnvironmentFactory, filename string, upsert bool, user string, manual, confirmed bool) error {
	operator, err := env.SiteOperator()
	if err != nil {
		return trace.Wrap(err)
	}
	gravityResources, err := gravity.New(gravity.Config{
		Operator:    operator,
		CurrentUser: env.CurrentUser(),
		Silent:      env.Silent,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	reader, err := common.GetReader(filename)
	if err != nil {
		return trace.Wrap(err)
	}
	defer reader.Close()
	decoder := yaml.NewYAMLOrJSONDecoder(reader, defaults.DecoderBufferSize)
	control := resources.NewControl(gravityResources)
	for err == nil {
		var resource teleservices.UnknownResource
		err = decoder.Decode(&resource)
		if err != nil {
			break
		}
		err = CreateResource(env, factory, control, resource, upsert, user, manual, confirmed)
	}
	if err == io.EOF {
		err = nil
	}
	return trace.Wrap(err)
}

// CreateResource updates or inserts a single resource
func CreateResource(
	env *localenv.LocalEnvironment,
	factory LocalEnvironmentFactory,
	control *resources.ResourceControl,
	resource teleservices.UnknownResource,
	upsert bool,
	user string,
	manual, confirmed bool,
) error {
	if resource.Kind != storage.KindRuntimeEnvironment {
		return trace.Wrap(control.Create(bytes.NewReader(resource.Raw), upsert, user))
	}
	if checkRunningAsRoot() != nil {
		return trace.BadParameter("updating cluster runtime environment variables requires root privileges.\n" +
			"Please run this command as root")
	}
	updateEnv, err := factory.UpdateEnv()
	if err != nil {
		return trace.Wrap(err)
	}
	defer updateEnv.Close()
	return trace.Wrap(UpdateEnvars(env, updateEnv, resource.Raw, manual, confirmed))
}

// RemoveResource deletes resource by name
func RemoveResource(env *localenv.LocalEnvironment, factory LocalEnvironmentFactory, kind string, name string, force bool, user string, manual, confirmed bool) error {
	if kind := modules.Get().CanonicalKind(kind); kind == storage.KindRuntimeEnvironment {
		if checkRunningAsRoot() != nil {
			return trace.BadParameter("updating cluster runtime environment variables requires root privileges.\n" +
				"Please run this command as root")
		}
		updateEnv, err := factory.UpdateEnv()
		if err != nil {
			return trace.Wrap(err)
		}
		defer updateEnv.Close()
		return trace.Wrap(RemoveEnvars(env, updateEnv, manual, confirmed))
	}
	operator, err := env.SiteOperator()
	if err != nil {
		return trace.Wrap(err)
	}
	gravityResources, err := gravity.New(gravity.Config{
		Operator:    operator,
		CurrentUser: env.CurrentUser(),
		Silent:      env.Silent,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	err = resources.NewControl(gravityResources).Remove(kind, name, force, user)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func getResources(env *localenv.LocalEnvironment, kind string, name string, withSecrets bool, format constants.Format, user string) error {
	operator, err := env.SiteOperator()
	if err != nil {
		return trace.Wrap(err)
	}
	gravityResources, err := gravity.New(gravity.Config{
		Operator:    operator,
		CurrentUser: env.CurrentUser(),
		Silent:      env.Silent,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	err = resources.NewControl(gravityResources).Get(os.Stdout, kind, name, withSecrets, format, user)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}
