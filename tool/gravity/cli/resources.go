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
	"context"
	"os"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/ops/resources"
	"github.com/gravitational/gravity/lib/ops/resources/gravity"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/storage/clusterconfig"
	"github.com/gravitational/gravity/tool/common"

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
	cluster, err := env.LocalCluster()
	if err != nil {
		return trace.Wrap(err)
	}
	clusterHandler := NewDefaultClusterOperationHandler(factory)
	gravityResources, err := gravity.New(gravity.Config{
		Operator:                operator,
		CurrentUser:             env.CurrentUser(),
		Silent:                  env.Silent,
		ClusterOperationHandler: clusterHandler,
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
		req := resources.CreateRequest{
			SiteKey:   cluster.Key(),
			Upsert:    upsert,
			Owner:     user,
			Manual:    manual,
			Confirmed: confirmed,
		}
		return trace.Wrap(control.Create(context.TODO(), bytes.NewReader(resource.Raw), req))
	})
	return trace.Wrap(err)
}

// removeResource deletes resource by name
func removeResource(
	env *localenv.LocalEnvironment,
	factory LocalEnvironmentFactory,
	kind, name string,
	force bool,
	user string,
	manual, confirmed bool,
) error {
	operator, err := env.SiteOperator()
	if err != nil {
		return trace.Wrap(err)
	}
	cluster, err := env.LocalCluster()
	if err != nil {
		return trace.Wrap(err)
	}
	clusterHandler := NewDefaultClusterOperationHandler(factory)
	gravityResources, err := gravity.New(gravity.Config{
		Operator:                operator,
		CurrentUser:             env.CurrentUser(),
		Silent:                  env.Silent,
		ClusterOperationHandler: clusterHandler,
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
	return trace.Wrap(err)

}

func getResources(env *localenv.LocalEnvironment, kind string, name string, withSecrets bool, format constants.Format, user string) error {
	operator, err := env.SiteOperator()
	if err != nil {
		return trace.Wrap(err)
	}
	cluster, err := env.LocalCluster()
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

// NewDefaultClusterOperationHandler creates an instance of the default cluster operation
// handler
func NewDefaultClusterOperationHandler(factory LocalEnvironmentFactory) gravity.ClusterOperationHandler {
	return clusterOperationHandler{
		LocalEnvironmentFactory: factory,
	}
}

// RemoveResource removes the resource specified with req
func (r clusterOperationHandler) RemoveResource(req resources.RemoveRequest) error {
	if checkRunningAsRoot() != nil {
		return trace.BadParameter("removing resource %q requires root privileges.\n"+
			"Please run this command as root", req.Kind)
	}
	localEnv, err := r.NewLocalEnv()
	if err != nil {
		return trace.Wrap(err)
	}
	defer localEnv.Close()
	updateEnv, err := r.NewUpdateEnv()
	if err != nil {
		return trace.Wrap(err)
	}
	defer updateEnv.Close()
	switch req.Kind {
	case storage.KindRuntimeEnvironment:
		env := storage.NewEnvironment(nil)
		return trace.Wrap(updateEnviron(context.TODO(), localEnv, updateEnv, env, req.Manual, req.Confirmed))
	case storage.KindClusterConfiguration:
		return trace.Wrap(resetConfig(context.TODO(), localEnv, updateEnv, req.Manual, req.Confirmed))
	}
	// unreachable
	return trace.BadParameter("unknown resource kind %q", req.Kind)
}

// UpdateResource creates or updates the resource specified with req
func (r clusterOperationHandler) UpdateResource(req resources.CreateRequest) error {
	if checkRunningAsRoot() != nil {
		return trace.BadParameter("creating resource %q requires root privileges.\n"+
			"Please run this command as root", req.Resource.Kind)
	}
	localEnv, err := r.NewLocalEnv()
	if err != nil {
		return trace.Wrap(err)
	}
	defer localEnv.Close()
	updateEnv, err := r.NewUpdateEnv()
	if err != nil {
		return trace.Wrap(err)
	}
	defer updateEnv.Close()
	switch req.Resource.Kind {
	case storage.KindRuntimeEnvironment:
		env, err := storage.UnmarshalEnvironmentVariables(req.Resource.Raw)
		if err != nil {
			return trace.Wrap(err)
		}
		if err := env.CheckAndSetDefaults(); err != nil {
			return trace.Wrap(err)
		}
		return trace.Wrap(updateEnviron(context.TODO(), localEnv, updateEnv,
			env, req.Manual, req.Confirmed))
	case storage.KindClusterConfiguration:
		config, err := clusterconfig.Unmarshal(req.Resource.Raw)
		if err != nil {
			return trace.Wrap(err)
		}
		return trace.Wrap(updateConfig(context.TODO(), localEnv, updateEnv,
			config, req.Manual, req.Confirmed))
	}
	// unreachable
	return trace.BadParameter("unknown resource kind %q", req.Resource.Kind)
}

type clusterOperationHandler struct {
	LocalEnvironmentFactory
}
