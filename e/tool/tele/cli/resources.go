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

package cli

import (
	"os"

	"github.com/gravitational/gravity/e/lib/ops/resources/tele"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/ops/resources"
	"github.com/gravitational/gravity/tool/common"

	"github.com/gravitational/trace"
)

// createResource updates or inserts resources from the specified file
func createResource(env *localenv.LocalEnvironment, filename string, upsert bool) error {
	operator, err := env.OperatorService("")
	if err != nil {
		return trace.Wrap(err)
	}
	opsURL, err := env.SelectOpsCenter("")
	if err != nil {
		return trace.Wrap(err)
	}
	apps, err := env.AppService(opsURL, localenv.AppConfig{})
	if err != nil {
		return trace.Wrap(err)
	}
	teleResources := tele.New(tele.Config{
		Operator: operator,
		Apps:     apps,
		Silent:   env.Silent,
	})
	reader, err := common.GetReader(filename)
	if err != nil {
		return trace.Wrap(err)
	}
	defer reader.Close()
	req := resources.CreateRequest{
		Upsert: upsert,
	}
	err = resources.NewControl(teleResources).Create(reader, req)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func getResources(env *localenv.LocalEnvironment, kind, name string, format constants.Format) error {
	operator, err := env.OperatorService("")
	if err != nil {
		return trace.Wrap(err)
	}
	opsURL, err := env.SelectOpsCenter("")
	if err != nil {
		return trace.Wrap(err)
	}
	apps, err := env.AppService(opsURL, localenv.AppConfig{})
	if err != nil {
		return trace.Wrap(err)
	}
	teleResources := tele.New(tele.Config{
		Operator: operator,
		Apps:     apps,
		Silent:   env.Silent,
	})
	err = resources.NewControl(teleResources).Get(os.Stdout, kind, name, false, format, "")
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// removeResource deletes the resource specified with kind and name
func removeResource(env *localenv.LocalEnvironment, kind string, name string, force bool) error {
	operator, err := env.OperatorService("")
	if err != nil {
		return trace.Wrap(err)
	}
	opsURL, err := env.SelectOpsCenter("")
	if err != nil {
		return trace.Wrap(err)
	}
	apps, err := env.AppService(opsURL, localenv.AppConfig{})
	if err != nil {
		return trace.Wrap(err)
	}
	teleResources := tele.New(tele.Config{
		Operator: operator,
		Apps:     apps,
		Silent:   env.Silent,
	})
	req := resources.RemoveRequest{
		Kind:  kind,
		Name:  name,
		Force: force,
	}
	err = resources.NewControl(teleResources).Remove(req)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}
