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

package process

import (
	"context"
	"time"

	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	libkube "github.com/gravitational/gravity/lib/kubernetes"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/ops/opsservice"
	"github.com/gravitational/gravity/lib/processconfig"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"

	"github.com/cenkalti/backoff"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/trace"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

// runCertExpirationWatch checks if the default self signed cluster web UI cert is about to expire soon and rotates it
func (p *Process) runCertExpirationWatch(client *kubernetes.Clientset) clusterService {
	return func(ctx context.Context) {
		ticker := time.NewTicker(time.Hour * 24)
		defer ticker.Stop()
		for {
			err := p.replaceCertIfAboutToExpire(ctx, client)
			if err != nil {
				p.WithError(err).Error("Failed to check for cluster web UI certificate expiration or replace it.")
			}

			select {
			case <-ticker.C:
			case <-ctx.Done():
				p.Debug("Cluster web UI certificate expiration watcher stopped.")
				return
			}
		}
	}
}

func (p *Process) replaceCertIfAboutToExpire(ctx context.Context, client *kubernetes.Clientset) error {
	p.Info("Running check for self signed cluster web UI certificate expiration.")

	ticker := backoff.NewTicker(&backoff.ExponentialBackOff{
		InitialInterval: time.Second * 3,
		Multiplier:      1.5,
		MaxInterval:     10 * time.Second,
		MaxElapsedTime:  1 * time.Minute,
		Clock:           backoff.SystemClock,
	})
	defer ticker.Stop()

	for {
		select {
		case tm := <-ticker.C:
			if tm.IsZero() {
				return trace.ConnectionProblem(nil, "timed out waiting while checking "+
					"for cluster web UI certificate expiration")
			}
			clusterCert, _, err := opsservice.GetClusterCertificate(client)
			if err != nil {
				p.WithError(err).Error("Failed to retrieve the cluster web UI certificate from k8s.")
				continue
			}

			cert, err := tlsca.ParseCertificatePEM(clusterCert)
			if err != nil {
				p.WithError(err).Error("Failed to parse the cluster web UI certificate.")
				continue
			}

			if len(cert.Issuer.OrganizationalUnit) == 0 || cert.Issuer.OrganizationalUnit[0] != defaults.SelfSignedCertWebOrg {
				p.Debug("Skipping expiration check for customer provided cluster web UI certificate.")
				return nil
			}

			periodBeforeExpire := time.Now().Add(defaults.CertRenewBeforeExpiry)
			if periodBeforeExpire.After(cert.NotAfter) {
				p.Infof("The cluster web UI certificate with SerialNumber=%v will expire soon."+
					" Replacing it with a new one...", cert.SerialNumber)

				cert, err := utils.GenerateSelfSignedCert([]string{p.cfg.Hostname})
				if err != nil {
					p.WithError(err).Error("Failed to generate self signed cluster web UI certificate.")
					continue
				}

				parsedCert, err := tlsca.ParseCertificatePEM(cert.Cert)
				if err != nil {
					p.WithError(err).Error("Failed to parse the self signed cluster web UI certificate.")
					continue
				}

				err = opsservice.UpdateClusterCertificate(client, ops.UpdateCertificateRequest{
					AccountID:   defaults.SystemAccountID,
					SiteDomain:  defaults.SystemAccountOrg,
					Certificate: cert.Cert,
					PrivateKey:  cert.PrivateKey,
				})
				if err != nil {
					p.WithError(err).Error("Failed to update the self signed cluster web UI certificate.")
					continue
				}

				p.Infof("Successfully rotated the self-signed cluster web UI certificate. "+
					"New cert ExpirationDate:%v, SerialNumber=%v", parsedCert.NotAfter, parsedCert.SerialNumber)
			}

			return nil
		case <-ctx.Done():
			return trace.Wrap(ctx.Err())
		}
	}
}

// runCertificateWatch updates process on p.certificateCh
// when changes to cluster certificates are detected
func (p *Process) runCertificateWatch(client *kubernetes.Clientset) clusterService {
	return func(ctx context.Context) {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			err := p.watchCertificate(ctx, client)
			if err != nil {
				p.Errorf("Failed to start certificate watch: %v.", trace.DebugReport(err))
			}
			select {
			case <-ticker.C:
			case <-ctx.Done():
				p.Debug("Certificate watcher stopped.")
				return
			}
		}
	}
}

func (p *Process) watchCertificate(ctx context.Context, client *kubernetes.Clientset) error {
	p.Debug("Restarting certificate watch.")

	watcher, err := client.CoreV1().Secrets(defaults.KubeSystemNamespace).
		Watch(ctx, metav1.ListOptions{
			FieldSelector: fields.OneTermEqualSelector("metadata.name", constants.ClusterCertificateMap).String(),
		})
	if err != nil {
		return trace.Wrap(err)
	}
	defer watcher.Stop()

	for {
		select {
		case event, ok := <-watcher.ResultChan():
			if !ok {
				p.Debugf("Watcher channel closed: %v.", event)
				return nil
			}

			if event.Type != watch.Modified && event.Type != watch.Deleted {
				p.Debugf("Ignoring event: %v.", event.Type)
				continue
			}

			secret, ok := event.Object.(*v1.Secret)
			if !ok {
				p.Warningf("Expected Secret, got: %T %v.", event.Object, event.Object)
				continue
			}
			if secret.Name != constants.ClusterCertificateMap {
				p.Debugf("Ignoring secret change: %v.", secret.Name)
				continue
			}

			p.Debugf("Detected secret change: %v.", secret.Name)
			p.BroadcastEvent(service.Event{
				Name: constants.ClusterCertificateUpdatedEvent,
			})

		case <-ctx.Done():
			p.Debug("Stopping certificate watcher.")
			return nil
		}
	}
}

// runAuthGatewayWatch monitors config map with auth gateway configuration
// and updates Teleport configuration appropriately.
func (p *Process) runAuthGatewayWatch(client *kubernetes.Clientset) clusterService {
	return func(ctx context.Context) {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			err := p.watchAuthGateway(ctx, client)
			if err != nil {
				p.WithError(err).Warn("Failed to start auth gateway config watch.")
			}
			select {
			case <-ticker.C:
			case <-ctx.Done():
				p.Debug("Auth gateway config watcher stopped.")
				return
			}
		}
	}
}

// watchAuthGateway observes changes to the auth gateway config map and
// updates Teleport configuration appropriately.
func (p *Process) watchAuthGateway(ctx context.Context, client *kubernetes.Clientset) error {
	p.Debug("Restarting auth gateway config watch.")
	watcher, err := client.CoreV1().ConfigMaps(defaults.KubeSystemNamespace).
		Watch(ctx, metav1.ListOptions{
			FieldSelector: fields.OneTermEqualSelector("metadata.name", constants.AuthGatewayConfigMap).String(),
		})
	if err != nil {
		return trace.Wrap(err)
	}
	defer watcher.Stop()
	for {
		select {
		case event, ok := <-watcher.ResultChan():
			if !ok {
				p.Debugf("Watcher channel closed: %v.", event)
				return nil
			}
			if event.Type != watch.Modified && event.Type != watch.Deleted {
				p.Debugf("Ignoring event: %v.", event.Type)
				continue
			}
			configMap, ok := event.Object.(*v1.ConfigMap)
			if !ok {
				p.Warningf("Expected ConfigMap, got: %[1]T %[1]v.", event.Object)
				continue
			}
			if configMap.Name != constants.AuthGatewayConfigMap {
				p.Debugf("Ignoring ConfigMap change: %v.", configMap.Name)
				continue
			}
			p.Infof("Detected ConfigMap change: %v.", configMap.Name)
			authGatewayConfig, err := p.getAuthGatewayConfig()
			if err != nil {
				p.Errorf("Failed to retrieve auth gateway config: %v.",
					trace.DebugReport(err))
				return trace.Wrap(err)
			}
			err = p.reloadAuthGatewayConfig(authGatewayConfig)
			if err != nil {
				p.Errorf("Failed to reload auth gateway config: %v.",
					trace.DebugReport(err))
				continue
			}
		case <-ctx.Done():
			p.Debug("Stopping auth gateway config watcher.")
			return nil
		}
	}
}

// reloadAuthGatewayConfig compares the provided auth gateway configuration
// with the configuration the process is currently started with and makes a
// decision on whether the configuration should be updated and/or the process
// restarted in order for the changes to take effect.
func (p *Process) reloadAuthGatewayConfig(authGatewayConfig storage.AuthGateway) error {
	if authGatewayConfig.PrincipalsChanged(p.authGatewayConfig) {
		// Teleport principals got updated. Don't restart right
		// away, but update its config so it can regenerate
		// identities for its services.
		p.Info("Auth gateway principals changed.")
		config, err := p.buildTeleportConfig(authGatewayConfig)
		if err != nil {
			return trace.Wrap(err)
		}
		// Replacing principals in config will result in Teleport
		// regenerating identities (asynchonously) and then
		// sending reload event which will be caught below.
		processconfig.ReplacePublicAddrs(p.TeleportProcess.Config, config)
	} else if authGatewayConfig.SettingsChanged(p.authGatewayConfig) {
		// Principals didn't change but some of the Teleport
		// settings changed so we can reload right away.
		p.Info("Auth gateway settings changed.")
		p.BroadcastEvent(service.Event{
			Name: service.TeleportReloadEvent,
		})
	} else {
		// Neither principals nor other settings changed, nothing
		// to do (maybe auth preference changed which is also a
		// part of auth gateway resource).
		p.Info("Auth gateway principals/settings didn't change.")
	}
	// Update gateway config information on the process so we can compare
	// with it if/when next change happens.
	p.authGatewayConfig = authGatewayConfig
	return nil
}

// runReloadEventsWatch watches reload events and restarts the process.
func (p *Process) runReloadEventsWatch(client *kubernetes.Clientset) clusterService {
	return func(ctx context.Context) {
		eventsCh := make(chan service.Event)
		p.WaitForEvent(ctx, service.TeleportReloadEvent, eventsCh)
		p.Infof("Started watching %v events.", service.TeleportReloadEvent)
		for {
			select {
			case event := <-eventsCh:
				if event.Name != service.TeleportReloadEvent {
					p.Warnf("Expected %v event, got: %#v.", service.TeleportReloadEvent, event)
					continue
				}
				p.Infof("Received event: %#v.", event)
				err := libkube.DeleteSelf(client, p.FieldLogger)
				if err != nil {
					p.Errorf("Failed to restart the pod: %v.", trace.DebugReport(err))
					continue
				}
			case <-ctx.Done():
				p.Infof("Stopped watching %v events.", service.TeleportReloadEvent)
				return
			}
		}
	}
}
