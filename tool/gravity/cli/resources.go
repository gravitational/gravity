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
	"github.com/gravitational/gravity/lib/ops/resources"
	"github.com/gravitational/gravity/lib/ops/resources/gravity"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/tool/common"

	teleservices "github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
	"k8s.io/apimachinery/pkg/util/yaml"
)

// CreateResource updates or inserts one or many resources
func CreateResource(env *localenv.LocalEnvironment, factory LocalEnvironmentFactory, filename string, upsert bool, user string, manual, confirmed bool) error {
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
	for err == nil {
		var raw teleservices.UnknownResource
		err = decoder.Decode(&raw)
		if err != nil {
			break
		}
		switch raw.Kind {
		case storage.KindRuntimeEnvironment:
			if checkRunningAsRoot() != nil {
				return trace.BadParameter("updating cluster runtime environment variables requires root privileges.\n" +
					"Please run this command as root")
			}
			updateEnv, err := factory.UpdateEnv()
			if err != nil {
				return trace.Wrap(err)
			}
			defer updateEnv.Close()
			err = UpdateEnvars(env, updateEnv, raw.Raw, manual, confirmed)
		default:
			err = resources.NewControl(gravityResources).Create(bytes.NewReader(raw.Raw), upsert, user)
		}
	}
	if err == io.EOF {
		err = nil
	}
	return trace.Wrap(err)
}

// RemoveResource deletes resource by name
func RemoveResource(env *localenv.LocalEnvironment, factory LocalEnvironmentFactory, kind string, name string, force bool, user string, manual, confirmed bool) error {
	if kind == storage.KindRuntimeEnvironment {
		if checkRunningAsRoot() != nil {
			return trace.BadParameter("updating environment variables requires root privileges.\n" +
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
