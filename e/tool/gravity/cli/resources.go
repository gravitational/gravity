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
	"context"
	"os"

	"github.com/gravitational/gravity/e/lib/environment"
	"github.com/gravitational/gravity/e/lib/ops/resources/gravity"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/ops/resources"
	ossgravity "github.com/gravitational/gravity/lib/ops/resources/gravity"
	"github.com/gravitational/gravity/tool/common"
	"github.com/gravitational/gravity/tool/gravity/cli"

	"github.com/gravitational/trace"
)

// createResource updates or inserts one or many resources
func createResource(env *environment.Local, factory cli.LocalEnvironmentFactory, filename string, upsert bool, user string, manual, confirmed bool) error {
	operator, err := env.ClusterOperator()
	if err != nil {
		return trace.Wrap(err)
	}
	cluster, err := env.LocalCluster()
	if err != nil {
		return trace.Wrap(err)
	}
	handler := cli.NewDefaultClusterOperationHandler(factory)
	ossResources, err := ossgravity.New(ossgravity.Config{
		Operator:                operator.Client,
		CurrentUser:             env.CurrentUser(),
		Silent:                  env.Silent,
		ClusterOperationHandler: handler,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	gravityResources, err := gravity.New(gravity.Config{
		Resources: ossResources,
		Operator:  operator,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	reader, err := common.GetReader(filename)
	if err != nil {
		return trace.Wrap(err)
	}
	defer reader.Close()
	req := resources.CreateRequest{
		SiteKey:   cluster.Key(),
		Upsert:    upsert,
		Owner:     user,
		Manual:    manual,
		Confirmed: confirmed,
	}
	err = resources.NewControl(gravityResources).Create(context.TODO(), reader, req)
	if err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(err)
}

// removeResource deletes resource by name
func removeResource(env *environment.Local, factory cli.LocalEnvironmentFactory, kind string, name string, force bool, user string, manual, confirmed bool) error {
	operator, err := env.ClusterOperator()
	if err != nil {
		return trace.Wrap(err)
	}
	cluster, err := env.LocalCluster()
	if err != nil {
		return trace.Wrap(err)
	}
	handler := cli.NewDefaultClusterOperationHandler(factory)
	ossResources, err := ossgravity.New(ossgravity.Config{
		Operator:                operator.Client,
		CurrentUser:             env.CurrentUser(),
		Silent:                  env.Silent,
		ClusterOperationHandler: handler,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	gravityResources, err := gravity.New(gravity.Config{
		Resources: ossResources,
		Operator:  operator,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	req := resources.RemoveRequest{
		SiteKey:   cluster.Key(),
		Kind:      kind,
		Name:      name,
		Force:     force,
		Owner:     user,
		Manual:    manual,
		Confirmed: confirmed,
	}
	err = resources.NewControl(gravityResources).Remove(context.TODO(), req)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func getResources(env *environment.Local, kind string, name string, withSecrets bool, format constants.Format, user string) error {
	operator, err := env.ClusterOperator()
	if err != nil {
		return trace.Wrap(err)
	}
	cluster, err := env.LocalCluster()
	if err != nil {
		return trace.Wrap(err)
	}
	ossResources, err := ossgravity.New(ossgravity.Config{
		Operator:    operator.Client,
		CurrentUser: env.CurrentUser(),
		Silent:      env.Silent,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	gravityResources, err := gravity.New(gravity.Config{
		Resources: ossResources,
		Operator:  operator,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	err = resources.NewControl(gravityResources).Get(os.Stdout, resources.ListRequest{
		SiteKey:     cluster.Key(),
		Kind:        kind,
		Name:        name,
		WithSecrets: withSecrets,
		User:        user,
	}, format)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}
