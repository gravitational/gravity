package process

import (
	"context"
	"time"

	"github.com/gravitational/gravity/e/lib/defaults"
	"github.com/gravitational/gravity/e/lib/ops"
	"github.com/gravitational/gravity/e/lib/ops/service"
	ossops "github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
)

var mlog = logrus.WithField(trace.Component, "migrate")

// runMigrations performs necessary data migrations upon process startup
func (p *Process) runMigrations(client *kubernetes.Clientset) error {
	if err := migrateLicense(client); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// migrateLicense converts the license installed as a config map to a secret
func migrateLicense(client *kubernetes.Clientset) error {
	licenseData, err := service.GetLicenseFromConfigMap(client)
	if err != nil {
		if !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		return nil // no license config map, nothing to do
	}
	mlog.Info("Migrating license from ConfigMap to Secret.")
	err = service.InstallLicenseSecret(client, licenseData)
	if err != nil {
		return trace.Wrap(err)
	}
	err = service.DeleteLicenseConfigMap(client)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// maybeReconnectTrustedClusters reestablishes connections to trusted clusters if needed
func (p *Process) maybeReconnectTrustedClusters(ctx context.Context) {
	if !p.waitForOperator(ctx) {
		return
	}
	_ = utils.RetryWithInterval(ctx, utils.NewUnlimitedExponentialBackOff(), func() error {
		localCluster, err := p.operator.GetLocalSite(ctx)
		if err != nil {
			return trace.Wrap(err, "failed to get local cluster")
		}
		trustedClusters, err := p.operator.GetTrustedClusters(localCluster.Key())
		if err != nil {
			return trace.Wrap(err, "failed to get list of trusted clusters")
		}
		for _, tc := range trustedClusters {
			needReconnect, err := p.needReconnectTrustedCluster(tc)
			if err != nil {
				return trace.Wrap(err)
			}
			if needReconnect {
				// launch reconnection in a separate goroutine so it does
				// not block anything and can be retried indefinitely
				// in case the Ops Center is unreachable
				go p.reconnectTrustedCluster(ctx, localCluster, tc)
			}
		}
		return nil
	})
}

// needReconnectTrustedCluster returns true if the cluster where this process
// is running should reestablish connection to the provided trusted cluster
func (p *Process) needReconnectTrustedCluster(tc storage.TrustedCluster) (bool, error) {
	// only reestablish connection to real Ops Center
	if tc.GetWizard() || tc.GetSystem() {
		mlog.Debugf("Do not reconnect system or wizard trusted cluster %q.", tc.GetName())
		return false, nil
	}
	// check the trusted cluster's certificate authorities - if any of them
	// are missing TLS key pairs, it means that this trusted cluster connection
	// was established before upgrade to teleport 3.0 so the clusters need to
	// exchange their certificate authorities again
	hostAuth, err := p.Backend().GetCertAuthority(services.CertAuthID{
		Type:       services.HostCA,
		DomainName: tc.GetName(),
	}, false)
	if err != nil {
		return false, trace.Wrap(err)
	}
	userAuth, err := p.Backend().GetCertAuthority(services.CertAuthID{
		Type:       services.UserCA,
		DomainName: tc.GetName(),
	}, false)
	if err != nil {
		return false, trace.Wrap(err)
	}
	if len(hostAuth.GetTLSKeyPairs()) != 0 && len(userAuth.GetTLSKeyPairs()) != 0 {
		mlog.Debugf("Don't need to reconnect trusted cluster %q.", tc.GetName())
		return false, nil
	}
	mlog.Debugf("Need to reconnect trusted cluster %q.", tc.GetName())
	return true, nil
}

func (p *Process) reconnectTrustedCluster(ctx context.Context, local *ossops.Site, tc storage.TrustedCluster) {
	utils.RetryWithInterval(ctx, utils.NewUnlimitedExponentialBackOff(), func() error {
		mlog.Infof("Trying to reconnect trusted cluster %q.", tc.GetName())
		err := p.operator.DeleteTrustedCluster(ctx, ops.DeleteTrustedClusterRequest{
			AccountID:          local.AccountID,
			ClusterName:        local.Domain,
			TrustedClusterName: tc.GetName(),
		})
		if err != nil && !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}
		// reconnect the cluster after a small delay to give Ops Center
		// a chance to clean it up properly on its side
		timer := time.NewTimer(defaults.TrustedClusterReconnectInterval)
		select {
		case <-timer.C:
			err = p.operator.UpsertTrustedCluster(ctx, local.Key(), tc)
			if err != nil {
				return trace.Wrap(err)
			}
		}
		mlog.Infof("Reconnected trusted cluster %q.", tc.GetName())
		return nil
	})
}
