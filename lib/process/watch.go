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
	"github.com/gravitational/gravity/lib/processconfig"

	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/trace"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

// startCertificateWatch starts watching for the changes in cluster certificate
// and notifies the process' "certificateCh" when the change happens
func (p *Process) startCertificateWatch(ctx context.Context, client *kubernetes.Clientset) error {
	go func() {
		for {
			err := p.watchCertificate(ctx, client)
			if err != nil {
				p.Errorf("Failed to start certificate watch: %v.", trace.DebugReport(err))
			}
			select {
			case <-time.After(time.Second):
			case <-ctx.Done():
				p.Debug("Certificate watcher stopped.")
				return
			}
		}
	}()
	return nil
}

func (p *Process) watchCertificate(ctx context.Context, client *kubernetes.Clientset) error {
	p.Debug("Restarting certificate watch.")

	watcher, err := client.Core().Secrets(defaults.KubeSystemNamespace).Watch(metav1.ListOptions{
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

// startAuthGatewayWatch launches watcher that monitors config map with
// auth gateway configuration.
func (p *Process) startAuthGatewayWatch(ctx context.Context, client *kubernetes.Clientset) error {
	go func() {
		for {
			err := p.watchAuthGateway(ctx, client)
			if err != nil {
				p.Errorf("Failed to start auth gateway config watch: %v.", trace.DebugReport(err))
			}
			select {
			case <-time.After(time.Second):
			case <-ctx.Done():
				p.Debug("Auth gateway config watcher stopped.")
				return
			}
		}
	}()
	return nil
}

// watchAuthGateway watches changes to the auth gateway config map.
func (p *Process) watchAuthGateway(ctx context.Context, client *kubernetes.Clientset) error {
	p.Debug("Restarting auth gateway config watch.")
	watcher, err := client.Core().ConfigMaps(defaults.KubeSystemNamespace).Watch(metav1.ListOptions{
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
				p.Warningf("Expected ConfigMap, got: %T %v.", event.Object, event.Object)
				continue
			}
			if configMap.Name != constants.AuthGatewayConfigMap {
				p.Debugf("Ignoring ConfigMap change: %v.", configMap.Name)
				continue
			}
			p.Debugf("Detected ConfigMap change: %v.", configMap.Name)
			p.BroadcastEvent(service.Event{
				Name: constants.AuthGatewayConfigUpdatedEvent,
			})
		case <-ctx.Done():
			p.Debug("Stopping auth gateway config watcher.")
			return nil
		}
	}
}

// startWatchingAuthGatewayEvents launches watcher that monitors auth
// gateway configuration change events and appropriately updates
// Teleport configuration.
func (p *Process) startWatchingAuthGatewayEvents(ctx context.Context, client *kubernetes.Clientset) error {
	go func() {
		eventsCh := make(chan service.Event)
		p.WaitForEvent(ctx, constants.AuthGatewayConfigUpdatedEvent, eventsCh)
		p.Infof("Started watching %v events.", constants.AuthGatewayConfigUpdatedEvent)
		for {
			select {
			case event := <-eventsCh:
				if event.Name != constants.AuthGatewayConfigUpdatedEvent {
					p.Warnf("Expected %v event, got: %#v.", constants.AuthGatewayConfigUpdatedEvent, event)
					continue
				}
				p.Infof("Received event: %#v.", event)
				authGatewayConfig, err := p.getAuthGatewayConfig()
				if err != nil {
					p.Errorf(trace.DebugReport(err))
					continue
				}
				if authGatewayConfig.PrincipalsChanged(p.authGatewayConfig) {
					// Teleport principals got updated. Don't restart right
					// away, but update its config so it can regenerate
					// identities for its services.
					p.Infof("Auth gateway principals changed.")
					config, err := p.buildTeleportConfig()
					if err != nil {
						p.Errorf(trace.DebugReport(err))
						continue
					}
					// Replacing principals in config will result in Teleport
					// regenerating identities (asynchonously) and then
					// sending reload event which will be caught below.
					processconfig.ReplacePublicAddrs(p.teleportProcess().Config, config)
				} else if authGatewayConfig.SettingsChanged(p.authGatewayConfig) {
					// Principals didn't change but some of the Teleport
					// settings changed so we can reload right away.
					p.Infof("Auth gateway settings changed.")
					p.BroadcastEvent(service.Event{
						Name: service.TeleportReloadEvent,
					})
				} else {
					// Neither principals nor other settings changed, nothing
					// to do (maybe auth preference changed which is also a
					// part auth gateway resource).
					p.Infof("Auth gateway principals/settings didn't change.")
				}
			case <-ctx.Done():
				p.Infof("Stopped watching %v events.", constants.AuthGatewayConfigUpdatedEvent)
				return
			}
		}
	}()
	return nil
}

// startWatchingReloadEvents launches watcher that listens for reload events
// and restarts the process.
func (p *Process) startWatchingReloadEvents(ctx context.Context, client *kubernetes.Clientset) error {
	go func() {
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
	}()
	return nil
}
