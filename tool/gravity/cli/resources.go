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
	"os"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/ops/resources"
	"github.com/gravitational/gravity/lib/ops/resources/gravity"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/tool/common"

	"github.com/gravitational/trace"
)

// createResource updates or inserts one or many resources
func createResource(env *localenv.LocalEnvironment, filename string, upsert bool, user string) error {
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
	created, err := resources.NewControl(gravityResources).Create(reader, upsert, user)
	if err != nil {
		return trace.Wrap(err)
	}

	for _, resource := range created {
		switch resource.Kind {
		case storage.KindEnvironment:
			err = updateEnvars(env, resource)
			if err != nil {
				return trace.Wrap(err)
			}
		}
		env.Println("created ", resource.Kind)
	}

	return nil
}

// removeResource deletes resource by name
func removeResource(env *localenv.LocalEnvironment, kind string, name string, force bool, user string) error {
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
