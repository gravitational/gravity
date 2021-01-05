package service

import (
	"context"
	"fmt"
	"time"

	"github.com/gravitational/gravity/e/lib/events"
	"github.com/gravitational/gravity/e/lib/ops"
	appclient "github.com/gravitational/gravity/lib/app/client"
	appservice "github.com/gravitational/gravity/lib/app/service"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/docker"
	"github.com/gravitational/gravity/lib/httplib"
	"github.com/gravitational/gravity/lib/loc"
	ossops "github.com/gravitational/gravity/lib/ops"
	libevents "github.com/gravitational/gravity/lib/ops/events"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/pack/webpack"
	"github.com/gravitational/gravity/lib/users"

	"github.com/gravitational/roundtrip"
	teleservices "github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
)

// CheckForUpdates checks with remote Ops Center if there is a newer version
// of the installed application
func (o *Operator) CheckForUpdate(key ossops.SiteKey) (*loc.Locator, error) {
	site, err := o.backend().GetSite(key.SiteDomain)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	cluster, err := ops.GetTrustedCluster(key, o)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	opsPackages, err := o.remotePackagesClient(site.Domain, cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	opsLatest, err := pack.FindLatestPackage(opsPackages, site.App.Locator())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ourLatest, err := pack.FindLatestPackage(o.packages(), site.App.Locator())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	isNewer, err := opsLatest.IsNewerThan(*ourLatest)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if isNewer {
		return opsLatest, nil
	}

	return nil, trace.NotFound(
		"no update for %v installed on %v found", site.App, site.Domain)
}

// DownloadUpdates downloads the provided application version from remote Ops Center
func (o *Operator) DownloadUpdate(ctx context.Context, req ops.DownloadUpdateRequest) error {
	site, err := o.backend().GetSite(req.SiteDomain)
	if err != nil {
		return trace.Wrap(err)
	}

	cluster, err := ops.GetTrustedCluster(req.SiteKey(), o)
	if err != nil {
		return trace.Wrap(err)
	}

	opsPackages, err := o.remotePackagesClient(site.Domain, cluster)
	if err != nil {
		return trace.Wrap(err)
	}

	opsApps, err := o.remoteAppsClient(site.Domain, cluster)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = appservice.PullApp(appservice.AppPullRequest{
		SrcPack: opsPackages,
		SrcApp:  opsApps,
		DstPack: o.packages(),
		DstApp:  o.apps(),
		Package: req.Application,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	imageService, err := docker.NewImageService(docker.RegistryConnectionRequest{
		RegistryAddress: defaults.DockerRegistry,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	err = appservice.SyncApp(ctx,
		appservice.SyncRequest{
			PackService:  o.packages(),
			AppService:   o.apps(),
			ImageService: imageService,
			Package:      req.Application,
		})
	if err != nil {
		return trace.Wrap(err)
	}

	libevents.Emit(ctx, o, events.UpdatesDownloaded, libevents.Fields{
		events.FieldOpsCenter:  cluster.GetName(),
		libevents.FieldName:    req.Application.Name,
		libevents.FieldVersion: req.Application.Version,
	})

	return nil
}

// EnablePeriodicUpdates turns periodic updates for the cluster on or updates the interval
func (o *Operator) EnablePeriodicUpdates(ctx context.Context, req ops.EnablePeriodicUpdatesRequest) error {
	err := req.CheckAndSetDefaults()
	if err != nil {
		return trace.Wrap(err)
	}

	err = o.updatePeriodicUpdatesStatus(req.SiteKey(), true, req.Interval)
	if err != nil {
		return trace.Wrap(err)
	}

	// start the periodic updates goroutine, no-op if already started
	err = o.StartPeriodicUpdates(req.SiteKey())
	if err != nil {
		return trace.Wrap(err)
	}

	cluster, err := ops.GetTrustedCluster(req.SiteKey(), o)
	if err != nil {
		return trace.Wrap(err)
	}

	libevents.Emit(ctx, o, events.UpdatesEnabled, libevents.Fields{
		events.FieldOpsCenter: cluster.GetName(),
		events.FieldInterval:  int(req.Interval.Seconds()),
	})

	return nil
}

// DisablePeriodicUpdates turns periodic updates for the cluster off and
// stops the update fetch loop if it's running
func (o *Operator) DisablePeriodicUpdates(ctx context.Context, key ossops.SiteKey) error {
	err := o.updatePeriodicUpdatesStatus(key, false, 0)
	if err != nil {
		return trace.Wrap(err)
	}

	// stop the periodic updates goroutine, no-op if not running
	err = o.StopPeriodicUpdates(key)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}

	cluster, err := ops.GetTrustedCluster(key, o)
	if err != nil {
		return trace.Wrap(err)
	}

	libevents.Emit(ctx, o, events.UpdatesDisabled, libevents.Fields{
		events.FieldOpsCenter: cluster.GetName(),
	})

	return nil
}

// StartPeriodicUpdates starts periodic updates check
func (o *Operator) StartPeriodicUpdates(key ossops.SiteKey) error {
	cluster, err := ops.GetTrustedCluster(key, o)
	if err != nil {
		return trace.Wrap(err)
	}

	if !cluster.GetPullUpdates() {
		return trace.NotFound("periodic updates are disabled for %v", key.SiteDomain)
	}

	site, err := o.GetSiteByDomain(key.SiteDomain)
	if err != nil {
		return trace.Wrap(err)
	}

	if site.UpdateInterval == 0 {
		return trace.NotFound("periodic updates are disabled for %q", site.Domain)
	}

	o.startService(key, periodicUpdatesService, func(ctx context.Context) {
		o.startPeriodicUpdates(ctx, key)
	})

	return nil
}

// StopPeriodicUpdates stops periodic updates check without disabling it (so they will
// be resumed when the process restarts for example)
func (o *Operator) StopPeriodicUpdates(key ossops.SiteKey) error {
	err := o.stopService(key, periodicUpdatesService)
	if err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}
	return nil
}

// PeriodicUpdatesStatus returns the status of periodic updates for the cluster
func (o *Operator) PeriodicUpdatesStatus(key ossops.SiteKey) (*ops.PeriodicUpdatesStatusResponse, error) {
	cluster, err := ops.GetTrustedCluster(key, o)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	site, err := o.backend().GetSite(key.SiteDomain)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if !cluster.GetPullUpdates() || site.UpdateInterval == 0 {
		return &ops.PeriodicUpdatesStatusResponse{Enabled: false}, nil
	}

	return &ops.PeriodicUpdatesStatusResponse{
		Enabled:   true,
		Interval:  site.UpdateInterval,
		NextCheck: site.NextUpdateCheck,
	}, nil
}

func (o *Operator) startPeriodicUpdates(ctx context.Context, key ossops.SiteKey) {
	o.Infof("Starting periodic updates.")
	ticker := time.NewTicker(defaults.PeriodicUpdatesTickInterval)
	for {
		select {
		case <-ticker.C:
			site, err := o.backend().GetSite(key.SiteDomain)
			if err != nil {
				o.Error(trace.DebugReport(err))
				continue
			}

			// is it time to check for updates?
			if site.NextUpdateCheck.After(o.clock().UtcNow()) {
				// update time hasn't come yet, keep spinning
				continue
			}

			update, err := o.CheckForUpdate(key)
			if err != nil && !trace.IsNotFound(err) {
				o.Error(trace.DebugReport(err))
				continue
			}

			if update == nil {
				o.Infof("No newer versions detected.")
			} else {
				o.Infof("Newer version detected, downloading: %v.", update)
				err = o.DownloadUpdate(ctx, ops.DownloadUpdateRequest{
					AccountID:   key.AccountID,
					SiteDomain:  key.SiteDomain,
					Application: *update,
				})
				if err != nil {
					o.Error(trace.DebugReport(err))
					continue
				}
			}

			site.NextUpdateCheck = o.clock().UtcNow().Add(site.UpdateInterval)
			_, err = o.backend().UpdateSite(*site)
			if err != nil {
				o.Error(trace.DebugReport(err))
			}
		case <-ctx.Done():
			o.Infof("Stopping periodic updates.")
			ticker.Stop()
			return
		}
	}
}

// updatePeriodicUpdatesStatus is a helper that updates appropriate trusted
// cluster and site to enable/disable updates pulling
func (o *Operator) updatePeriodicUpdatesStatus(key ossops.SiteKey, enabled bool, interval time.Duration) error {
	cluster, err := ops.GetTrustedCluster(key, o)
	if err != nil {
		return trace.Wrap(err)
	}

	cluster.SetPullUpdates(enabled)

	if _, err := o.backend().UpsertTrustedCluster(cluster); err != nil {
		return trace.Wrap(err)
	}

	site, err := o.backend().GetSite(key.SiteDomain)
	if err != nil {
		return trace.Wrap(err)
	}
	site.UpdateInterval = interval
	if interval == 0 {
		site.NextUpdateCheck = time.Time{}
	} else {
		site.NextUpdateCheck = o.clock().UtcNow().Add(interval)
	}

	_, err = o.backend().UpdateSite(*site)
	return trace.Wrap(err)
}

// periodicUpdatesService is an identifier for the periodic updates goroutine
const periodicUpdatesService = "periodicupdates"

// remotePackagesClient returns remote Ops Center package service client
func (o *Operator) remotePackagesClient(clusterName string, cluster teleservices.TrustedCluster) (*webpack.Client, error) {
	_, token, err := users.GetOpsCenterAgent(
		cluster.GetName(), clusterName, o.backend())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	client, err := webpack.NewBearerClient(
		fmt.Sprintf("https://%v", cluster.GetProxyAddress()),
		token.Token,
		roundtrip.HTTPClient(httplib.GetClient(o.GetConfig().Devmode)))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return client, nil
}

// remoteAppsClient returns remote Ops Center app service client
func (o *Operator) remoteAppsClient(clusterName string, cluster teleservices.TrustedCluster) (*appclient.Client, error) {
	_, token, err := users.GetOpsCenterAgent(
		cluster.GetName(), clusterName, o.backend())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	client, err := appclient.NewBearerClient(
		fmt.Sprintf("https://%v", cluster.GetProxyAddress()),
		token.Token,
		appclient.HTTPClient(httplib.GetClient(o.GetConfig().Devmode)))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return client, nil
}

// service represents a utility goroutine that can be stopped/started
type service struct {
	ctx    context.Context
	cancel context.CancelFunc
}

// startService starts the provided function as a goroutine and registers it with the operator so
// it can be stopped; does nothing if there's already a goroutine with the provided name running
func (o *Operator) startService(key ossops.SiteKey, name string, fn func(ctx context.Context)) {
	o.Lock()
	defer o.Unlock()
	if _, ok := o.services[key][name]; ok {
		o.Infof("Service %v for %v is already running, nothing to do.",
			name, key)
		return
	}
	ctx, cancel := context.WithCancel(context.TODO())
	go fn(ctx)
	if o.services[key] == nil {
		o.services[key] = make(map[string]service)
	}
	o.services[key][name] = service{ctx: ctx, cancel: cancel}
}

// stopService stops the goroutine registered as a "service" by name
func (o *Operator) stopService(key ossops.SiteKey, name string) error {
	o.Lock()
	defer o.Unlock()
	if _, ok := o.services[key][name]; !ok {
		return trace.NotFound("service %v for %v is not running", name, key)
	}
	svc := o.services[key][name]
	svc.cancel()
	delete(o.services[key], name)
	return nil
}
