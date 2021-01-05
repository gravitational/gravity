package service

import (
	"github.com/gravitational/gravity/e/lib/ops"
	"github.com/gravitational/gravity/lib/httplib"
	ossops "github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/ops/opsclient"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/trace"
)

// RegisterAgent is called by install agents to determine who's installer
// and who's joining agent when installing via Ops Center
func (o *Operator) RegisterAgent(req ops.RegisterAgentRequest) (*ops.RegisterAgentResponse, error) {
	o.Infof("%s.", req)
	group, err := o.getInstallGroup(req.SiteOperationKey())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	response, err := group.registerAgent(req)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return response, nil
}

// RequestClusterCopy replicates the cluster specified in the provided request
// and its data from the remote Ops Center
//
// It is used in Ops Center initiated installs when installer process does
// not have the cluster and operation state locally (because the operation
// was created in the Ops Center along with the cluster and all other data).
//
// The following things are replicated: cluster, install operation and its
// progress entry, both admin and regular cluster agents, expand token.
func (o *Operator) RequestClusterCopy(req ops.ClusterCopyRequest) error {
	if !o.GetConfig().Wizard {
		return trace.BadParameter("only installer can request cluster copy")
	}
	o.Infof("Requesting cluster copy: %#v.", req)
	client, err := opsclient.NewBearerClient(req.OpsURL, req.OpsToken,
		opsclient.HTTPClient(httplib.GetClient(true)))
	if err != nil {
		return trace.Wrap(err)
	}
	cluster, err := client.GetSiteByDomain(req.ClusterName)
	if err != nil {
		return trace.Wrap(err)
	}
	operations, err := client.GetSiteOperations(cluster.Key(), ossops.OperationsFilter{})
	if err != nil {
		return trace.Wrap(err)
	}
	if len(operations) == 0 {
		return trace.NotFound("cluster %v does not have operations",
			cluster.Domain)
	}
	regular, err := client.GetClusterAgent(ossops.ClusterAgentRequest{
		AccountID:   req.AccountID,
		ClusterName: req.ClusterName,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	token, err := client.GetExpandToken(cluster.Key())
	if err != nil {
		return trace.Wrap(err)
	}
	// now insert everything we've got
	_, err = o.backend().CreateSite(ossops.ConvertOpsSite(*cluster))
	if err != nil {
		return trace.Wrap(err)
	}
	o.Debugf("Replicated cluster: %v.", cluster)
	for _, op := range operations {
		_, err := o.backend().CreateSiteOperation(op)
		if err != nil {
			return trace.Wrap(err)
		}
		o.Debugf("Replicated operation: %v.", op)
		progress, err := client.GetSiteOperationProgress(ossops.SiteOperationKey{
			AccountID:   op.AccountID,
			SiteDomain:  op.SiteDomain,
			OperationID: op.ID,
		})
		if err != nil {
			return trace.Wrap(err)
		}
		_, err = o.backend().CreateProgressEntry(storage.ProgressEntry(*progress))
		if err != nil {
			return trace.Wrap(err)
		}
		o.Debugf("Replicated progress entry: %v.", progress)
	}
	_, err = o.users().CreateAgentFromLoginEntry(cluster.Domain, *regular, false)
	if err != nil {
		return trace.Wrap(err)
	}
	o.Debugf("Replicated regular agent: %v.", regular)
	_, err = o.users().CreateProvisioningToken(*token)
	if err != nil {
		return trace.Wrap(err)
	}
	o.Debugf("Replicated expand token: %v.", token)
	return nil
}

func (o *Operator) getInstallGroup(key ossops.SiteOperationKey) (*installGroup, error) {
	o.Lock()
	defer o.Unlock()
	if _, ok := o.installGroups[key]; !ok {
		group, err := newInstallGroup(key)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		o.installGroups[key] = group
	}
	return o.installGroups[key], nil
}
