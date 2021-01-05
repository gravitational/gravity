package cli

import (
	"time"

	"github.com/gravitational/gravity/e/lib/environment"
	"github.com/gravitational/gravity/e/lib/ops"
	"github.com/gravitational/gravity/lib/constants"
	ossops "github.com/gravitational/gravity/lib/ops"

	"github.com/gravitational/trace"
)

func updateDownload(env *environment.Local, every string) error {
	operator, err := env.ClusterOperator()
	if err != nil {
		return trace.Wrap(err)
	}
	cluster, err := operator.GetLocalSite()
	if err != nil {
		return trace.Wrap(err)
	}
	// if "every" flag is provided, only update periodic updates status
	if every != "" {
		return trace.Wrap(setPeriodicUpdates(env, operator, *cluster, every))
	}
	update, err := operator.CheckForUpdate(cluster.Key())
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	if update == nil {
		env.Println("No newer versions found")
		return nil
	}
	env.Printf("New version is available, downloading: %v\n", update)
	err = operator.DownloadUpdate(ops.DownloadUpdateRequest{
		AccountID:   cluster.AccountID,
		SiteDomain:  cluster.Domain,
		Application: *update,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func setPeriodicUpdates(env *environment.Local, operator ops.Operator, cluster ossops.Site, every string) error {
	if every == constants.PeriodicUpdatesOff {
		err := operator.DisablePeriodicUpdates(cluster.Key())
		if err != nil {
			return trace.Wrap(err)
		}
		env.Println("Periodic updates have been turned off")
		return nil
	}
	interval, err := time.ParseDuration(every)
	if err != nil {
		return trace.Wrap(err)
	}
	err = operator.EnablePeriodicUpdates(ops.EnablePeriodicUpdatesRequest{
		AccountID:  cluster.AccountID,
		SiteDomain: cluster.Domain,
		Interval:   interval,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	env.Println("Periodic updates have been turned on")
	return nil
}
