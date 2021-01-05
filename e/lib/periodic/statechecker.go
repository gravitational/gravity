package periodic

import (
	"time"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/storage"

	"github.com/gravitational/teleport"
	rt "github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
)

type StateCheckerConfig struct {
	Backend  storage.Backend
	Operator ops.Operator
	Packages pack.PackageService
	Tunnel   rt.Server
	Context  context.Context
}

// StartStateChecker launches a goroutine that monitors installed sites and transitions them
// to offline state and back based on whether they maintain reverse tunnel to OpsCenter or not
//
// This service is supposed to be run only if gravity is started in "opscenter" mode.
func StartStateChecker(conf StateCheckerConfig) {
	checker := stateChecker{
		conf:        conf,
		FieldLogger: logrus.WithField(trace.Component, "statechecker"),
	}
	go checker.start()
}

type stateChecker struct {
	conf StateCheckerConfig
	// FieldLogger is used for logging
	logrus.FieldLogger
}

// start periodically iterates over all installed sites and, if needed, updates their states
// appropriately depending on their "remote support" status
func (c *stateChecker) start() {
	ticker := time.NewTicker(defaults.OfflineCheckInterval)
	for {
		select {
		case <-ticker.C:
			sites, err := c.conf.Backend.GetAllSites()
			if err != nil {
				c.Errorf("Failed to get clusters: %v.", trace.DebugReport(err))
				continue
			}
			for _, site := range sites {
				if err = c.checkSite(site); err != nil {
					c.Errorf("Failed to check cluster: %v.", trace.DebugReport(err))
				}
			}
		case <-c.conf.Context.Done():
			c.Info("Stopping state checker.")
			ticker.Stop()
			return
		}

	}
}

// checkSite determines whether the provided site maintains a reverse tunnel with the
// OpsCenter and either moves the site to "offline" state or updates the site's local
// state to match the remote site state
func (c *stateChecker) checkSite(site storage.Site) error {
	operations, err := c.conf.Backend.GetSiteOperations(site.Domain)
	if err != nil {
		return trace.Wrap(err)
	}

	// do not mess with states of sites that have an active operation in progress inside OpsCenter
	// this can happen for example during initial installation
	for _, op := range operations {
		if op.State != ops.OperationStateCompleted && op.State != ops.OperationStateFailed {
			return nil
		}
	}

	// also do not touch failed sites
	if site.State == ops.SiteStateFailed {
		return nil
	}

	// if the site is online, sync its state with the remote site, otherwise mark it offline
	if !c.isOnline(site.Domain) {
		return trace.Wrap(c.markOffline(site))
	}

	return trace.Wrap(c.syncSite(site))
}

// markOffline updates the state of the provided local site "offline" in the database
func (c *stateChecker) markOffline(localSite storage.Site) error {
	if localSite.State == ops.SiteStateOffline { // no-op
		return nil
	}

	c.Infof("Cluster %v became offline (was: %v).", localSite.Domain, localSite.State)

	localSite.State = ops.SiteStateOffline
	_, err := c.conf.Backend.UpdateSite(localSite)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// syncSite updates the state and app package of the provided local site to match those
// of the remote site
func (c *stateChecker) syncSite(localCluster storage.Site) error {
	remoteCluster, err := c.conf.Operator.GetSite(ops.SiteKey{
		AccountID:  localCluster.AccountID,
		SiteDomain: localCluster.Domain,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	changed := false

	remoteState := remoteCluster.State
	if localCluster.State != remoteState {
		c.Infof("Cluster %v became %v (was: %v).", localCluster.Domain,
			remoteState, localCluster.State)

		changed = true
		localCluster.State = remoteState
	}

	remoteApp := remoteCluster.App
	if !localCluster.App.Locator().IsEqualTo(remoteApp.Package) {
		c.Infof("Cluster %v app changed to %v (was: %v).", localCluster.Domain,
			remoteApp.Package, localCluster.App.Locator())

		changed = true
		localCluster.App = remoteApp.PackageEnvelope.ToPackage()
	}

	remoteNodes := remoteCluster.ClusterState.Servers
	if !localCluster.ClusterState.Servers.IsEqualTo(remoteNodes) {
		c.Infof("Cluster %v nodes changed to %s (was: %s).", localCluster.Domain,
			remoteNodes, localCluster.ClusterState.Servers)

		changed = true
		localCluster.ClusterState.Servers = remoteNodes
	}

	if changed {
		_, err = c.conf.Backend.UpdateSite(localCluster)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	return nil
}

// isOnline checks whether there is an active reverse tunnel for the site with the provided name
func (c *stateChecker) isOnline(name string) bool {
	remoteSites := c.conf.Tunnel.GetSites()
	for _, site := range remoteSites {
		if site.GetName() == name {
			return site.GetStatus() == teleport.RemoteClusterStatusOnline
		}
	}
	return false
}
