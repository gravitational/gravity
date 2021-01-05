package cli

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/gravitational/gravity/e/lib/environment"
	"github.com/gravitational/gravity/e/lib/ops"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/trace"
)

// updateRemoteAccess enables or disables remote access
func updateRemoteAccess(env *environment.Local, enabled bool) error {
	operator, err := env.ClusterOperator()
	if err != nil {
		return trace.Wrap(err)
	}
	site, err := operator.GetLocalSite(context.TODO())
	if err != nil {
		return trace.Wrap(err)
	}
	cluster, err := ops.GetTrustedCluster(site.Key(), operator)
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("this cluster does not have remote support " +
				"configured yet, please create a trusted cluster resource to " +
				"be able to manage remote support via gravity tunnel command")
		}
		return trace.Wrap(err)
	}
	cluster.SetEnabled(enabled)
	err = operator.UpsertTrustedCluster(context.TODO(), site.Key(), cluster)
	if err != nil {
		return trace.Wrap(err)
	}
	printRemoteAccessStatus(cluster)
	return nil
}

// remoteAccessStatus prints status of remote access
func remoteAccessStatus(env *environment.Local) error {
	operator, err := env.ClusterOperator()
	if err != nil {
		return trace.Wrap(err)
	}
	site, err := operator.GetLocalSite(context.TODO())
	if err != nil {
		return trace.Wrap(err)
	}
	cluster, err := ops.GetTrustedCluster(site.Key(), operator)
	if err != nil {
		if trace.IsNotFound(err) {
			return trace.NotFound("this cluster does not have remote support " +
				"configured yet, please create a trusted cluster resource to " +
				"be able to manage remote support via gravity tunnel command")
		}
		return trace.Wrap(err)
	}
	printRemoteAccessStatus(cluster)
	return nil
}

func printRemoteAccessStatus(cluster storage.TrustedCluster) {
	status := "enabled"
	if !cluster.GetEnabled() {
		status = "disabled"
	}
	w := new(tabwriter.Writer)
	w.Init(os.Stdout, 0, 8, 1, '\t', 0)
	fmt.Fprintf(w, "Gravity Hub\tStatus\n")
	fmt.Fprintf(w, "%v\t%v\n", cluster.GetName(), status)
	w.Flush()
}
