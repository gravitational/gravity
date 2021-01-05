package service

import (
	"context"
	"fmt"
	"io/ioutil"

	"github.com/gravitational/gravity/e/lib/events"
	"github.com/gravitational/gravity/e/lib/ops"
	"github.com/gravitational/gravity/e/lib/ops/client"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/httplib"
	ossops "github.com/gravitational/gravity/lib/ops"
	libevents "github.com/gravitational/gravity/lib/ops/events"
	"github.com/gravitational/gravity/lib/ops/opsclient"
	"github.com/gravitational/gravity/lib/ops/opsservice"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/users"

	teleservices "github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
)

// UpsertTrustedCluster creates or updates a trusted cluster
func (o *Operator) UpsertTrustedCluster(ctx context.Context, key ossops.SiteKey, cluster storage.TrustedCluster) error {
	o.Infof("UpsertTrustedCluster(%s).", cluster)
	if !o.isInstaller() {
		local, err := o.GetLocalSite(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
		if cluster.GetName() == local.Domain {
			return trace.BadParameter("can't connect to Gravity Hub with the "+
				"same name as this cluster, %v", cluster.GetName())
		}
	}
	existingCluster, err := ops.GetTrustedCluster(key, o)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	if existingCluster != nil {
		if cluster.GetName() != existingCluster.GetName() {
			return trace.AlreadyExists(
				"this cluster is already connected to %v, please delete the "+
					"existing trusted cluster using 'gravity resource rm' "+
					"command before attempting to configure a new one",
				existingCluster.GetName())
		}
		err := existingCluster.CanChangeStateTo(cluster)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	if existingCluster == nil || existingCluster.GetEnabled() != cluster.GetEnabled() {
		err := o.configureAccess(ctx, key, cluster)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	if existingCluster == nil || existingCluster.GetPullUpdates() != cluster.GetPullUpdates() {
		err := o.configureUpdates(ctx, key, cluster)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}

func (o *Operator) configureAccess(ctx context.Context, key ossops.SiteKey, cluster storage.TrustedCluster) error {
	// establish trust between us and Ops Center and create a reverse tunnel
	_, err := o.users().UpsertTrustedCluster(cluster)
	if err != nil {
		return trace.Wrap(err)
	}
	if cluster.GetRegular() {
		if cluster.GetEnabled() {
			libevents.Emit(ctx, o, events.RemoteSupportEnabled, libevents.Fields{
				events.FieldOpsCenter: cluster.GetName(),
			})
		} else {
			libevents.Emit(ctx, o, events.RemoteSupportDisabled, libevents.Fields{
				events.FieldOpsCenter: cluster.GetName(),
			})
		}
	}
	// do not create a local representation of the system cluster
	if cluster.GetSystem() {
		return nil
	}
	// make remote Ops Center create a cluster representing this cluster
	err = o.upsertRemoteCluster(key, cluster)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (o *Operator) configureUpdates(ctx context.Context, key ossops.SiteKey, cluster storage.TrustedCluster) (err error) {
	if cluster.GetPullUpdates() {
		err = o.EnablePeriodicUpdates(ctx, ops.EnablePeriodicUpdatesRequest{
			AccountID:  key.AccountID,
			SiteDomain: key.SiteDomain,
		})
	} else {
		err = o.DisablePeriodicUpdates(ctx, key)
	}
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// DeleteTrustedCluster deletes a trusted cluster specified with req
func (o *Operator) DeleteTrustedCluster(ctx context.Context, req ops.DeleteTrustedClusterRequest) error {
	o.Infof("%s.", req)
	err := req.Check()
	if err != nil {
		return trace.Wrap(err)
	}
	cluster, err := o.getTrustedClusterByName(req.TrustedClusterName)
	if err != nil {
		return trace.Wrap(err)
	}
	if cluster.GetRegular() {
		err = o.DisablePeriodicUpdates(ctx, req.SiteKey())
		if err != nil && !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
	}
	// delete or expire reverse tunnel
	if req.Delay != 0 {
		return trace.Wrap(storage.DisableAccess(o.backend(), cluster.GetName(),
			req.Delay))
	}
	err = o.users().DeleteTrustedCluster(cluster.GetName())
	if err != nil {
		return trace.Wrap(err)
	}
	if cluster.GetRegular() {
		libevents.Emit(ctx, o, events.RemoteSupportDisabled, libevents.Fields{
			events.FieldOpsCenter: cluster.GetName(),
		})
	}
	if cluster.GetSystem() {
		return nil
	}
	err = o.removeRemoteCluster(req.SiteKey(), cluster)
	if err != nil {
		o.Errorf("Failed to remove remote cluster: %v.",
			trace.DebugReport(err))
	}
	return nil
}

// GetTrustedClusters returns a list of configured trusted clusters
func (o *Operator) GetTrustedClusters(key ossops.SiteKey) ([]storage.TrustedCluster, error) {
	clusters, err := o.users().GetTrustedClusters()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var result []storage.TrustedCluster
	for _, cluster := range clusters {
		clusterS, ok := cluster.(storage.TrustedCluster)
		if !ok {
			o.Warnf("Unexpected type %T.", cluster)
			continue
		}
		result = append(result, clusterS)
	}
	return result, nil
}

// GetTrustedCluster returns trusted cluster by name
func (o *Operator) GetTrustedCluster(key ossops.SiteKey, name string) (storage.TrustedCluster, error) {
	teleCluster, err := o.users().GetTrustedCluster(name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	cluster, ok := teleCluster.(storage.TrustedCluster)
	if !ok {
		return nil, trace.BadParameter("unexpected type %T", teleCluster)
	}
	return cluster, nil
}

// upsertRemoteCluster makes a request to the Ops Center represented by the
// provided trusted cluster to create or update a local entry for the cluster
// with the given key
func (o *Operator) upsertRemoteCluster(key ossops.SiteKey, cluster storage.TrustedCluster) error {
	client, err := o.remoteOpsClient(cluster)
	if err != nil {
		return trace.Wrap(err)
	}
	siteCopy, err := copySite(key, o.backend(), o)
	if err != nil {
		return trace.Wrap(err)
	}
	agent, token, err := users.GetOpsCenterAgent(
		cluster.GetName(), key.SiteDomain, o.backend())
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	if trace.IsNotFound(err) {
		agent, token, err = users.CreateOpsCenterAgent(
			cluster.GetName(), key.SiteDomain, o.users())
		if err != nil {
			return trace.Wrap(err)
		}
		o.Debugf("Created Gravity Hub agent %v.", agent.GetName())
	}
	caPackage := opsservice.PlanetCertAuthorityPackage(key.SiteDomain)
	if err != nil {
		return trace.Wrap(err)
	}
	_, reader, err := o.packages().ReadPackage(caPackage)
	if err != nil {
		return trace.Wrap(err)
	}
	defer reader.Close()
	caPackageBytes, err := ioutil.ReadAll(reader)
	if err != nil {
		return trace.Wrap(err)
	}
	req := ops.AcceptRemoteClusterRequest{
		Site: *siteCopy,
		SiteAgent: storage.RemoteAccessUser{
			Email:      agent.GetName(),
			Token:      token.Token,
			SiteDomain: key.SiteDomain,
		},
		HandshakeToken:          cluster.GetToken(),
		TLSCertAuthorityPackage: caPackageBytes,
	}
	if _, err := client.AcceptRemoteCluster(req); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// removeRemoteCluster is an opposite of "upsertRemoteCluster" - it asks the
// remote Ops Center represented by the provided trusted cluster to remove the
// local entry for the cluster with the specified key
func (o *Operator) removeRemoteCluster(key ossops.SiteKey, cluster storage.TrustedCluster) error {
	client, err := o.remoteOpsClient(cluster)
	if err != nil {
		return trace.Wrap(err)
	}
	err = client.RemoveRemoteCluster(ops.RemoveRemoteClusterRequest{
		AccountID:      key.AccountID,
		ClusterName:    key.SiteDomain,
		HandshakeToken: cluster.GetToken(),
	})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func (o *Operator) getTrustedClusterByName(name string) (storage.TrustedCluster, error) {
	cluster, err := o.users().GetTrustedCluster(name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	clusterS, ok := cluster.(storage.TrustedCluster)
	if !ok {
		return nil, trace.BadParameter("unexpected type %T", cluster)
	}
	return clusterS, nil
}

func (o *Operator) remoteOpsClient(cluster teleservices.TrustedCluster) (*client.Client, error) {
	ossClient, err := opsclient.NewBearerClient(
		fmt.Sprintf("https://%v", cluster.GetProxyAddress()),
		cluster.GetToken(),
		opsclient.HTTPClient(httplib.GetClient(o.GetConfig().Devmode)))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return client.New(ossClient), nil
}

func copySite(siteKey ossops.SiteKey, backend storage.Backend, operator ops.Operator) (*ops.SiteCopy, error) {
	site, err := backend.GetSite(siteKey.SiteDomain)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	site.Local = false
	site.License = ""
	installOperation, err := ossops.GetCompletedInstallOperation(siteKey, operator)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	entry, err := backend.GetLastProgressEntry(siteKey.SiteDomain, installOperation.ID)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	entry.Completion = constants.Completed
	entry.State = ossops.ProgressStateCompleted
	return &ops.SiteCopy{
		Site:          *site,
		SiteOperation: (storage.SiteOperation)(*installOperation),
		ProgressEntry: *entry,
	}, nil
}
