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
	"os"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/modules"
	"github.com/gravitational/gravity/lib/ops/resources"
	"github.com/gravitational/gravity/lib/ops/resources/gravity"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/tool/common"

	teleservices "github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
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
	control := resources.NewControl(gravityResources)
	err = resources.ForEach(reader, func(resource storage.UnknownResource) error {
		res := teleservices.UnknownResource{ResourceHeader: resource.ResourceHeader, Raw: resource.Raw}
		return trace.Wrap(CreateResource(env, factory, control, res, upsert, user, manual, confirmed))
	})
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
	switch resource.Kind {
	case storage.KindRuntimeEnvironment, storage.KindClusterConfiguration:
	default:
		return trace.Wrap(control.Create(bytes.NewReader(resource.Raw), upsert, user))
	}
	if checkRunningAsRoot() != nil {
		return trace.BadParameter("creating resource %q requires root privileges.\n"+
			"Please run this command as root", resource.Kind)
	}
	updateEnv, err := factory.UpdateEnv()
	if err != nil {
		return trace.Wrap(err)
	}
	defer updateEnv.Close()
	switch resource.Kind {
	case storage.KindRuntimeEnvironment:
		return trace.Wrap(UpdateEnviron(env, updateEnv, resource.Raw, manual, confirmed))
	case storage.KindClusterConfiguration:
		return trace.Wrap(UpdateConfig(env, updateEnv, resource.Raw, manual, confirmed))
	}
	// unreachable
	return trace.BadParameter("unkown resource kind %q", resource.Kind)
}

// RemoveResource deletes resource by name
func RemoveResource(env *localenv.LocalEnvironment, factory LocalEnvironmentFactory, kind string, name string, force bool, user string, manual, confirmed bool) error {
	kind = modules.Get().CanonicalKind(kind)
	switch kind {
	case storage.KindRuntimeEnvironment, storage.KindClusterConfiguration:
	default:
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
	}

	if checkRunningAsRoot() != nil {
		return trace.BadParameter("removing resource %q requires root privileges.\n"+
			"Please run this command as root", kind)
	}
	updateEnv, err := factory.UpdateEnv()
	if err != nil {
		return trace.Wrap(err)
	}
	defer updateEnv.Close()

	switch kind {
	case storage.KindRuntimeEnvironment:
		return trace.Wrap(RemoveEnvars(env, updateEnv, manual, confirmed))
	case storage.KindClusterConfiguration:
		return trace.Wrap(ResetConfig(env, updateEnv, manual, confirmed))
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
