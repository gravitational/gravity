package cli

import (
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
		Upsert:    upsert,
		User:      user,
		Manual:    manual,
		Confirmed: confirmed,
	}
	err = resources.NewControl(gravityResources).Create(reader, req)
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
		Kind:      kind,
		Name:      name,
		Force:     force,
		User:      user,
		Manual:    manual,
		Confirmed: confirmed,
	}
	err = resources.NewControl(gravityResources).Remove(req)
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
	err = resources.NewControl(gravityResources).Get(os.Stdout, kind, name, withSecrets, format, user)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}
