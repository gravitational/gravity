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
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/storage/clusterconfig"

	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/trace"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

// startCertificateWatch starts watching for the changes in cluster certificate
// and notifies the process' "certificateCh" when the change happens
func (p *Process) startCertificateWatch(ctx context.Context, client *kubernetes.Clientset) error {
	go func() {
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
// auth gateway configuration and updates Teleport configuration
// appropriately.
func (p *Process) startAuthGatewayWatch(ctx context.Context, client *kubernetes.Clientset) error {
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			err := p.watchAuthGateway(ctx, client)
			if err != nil {
				p.Errorf("Failed to start auth gateway config watch: %v.", trace.DebugReport(err))
			}
			select {
			case <-ticker.C:
			case <-ctx.Done():
				p.Debug("Auth gateway config watcher stopped.")
				return
			}
		}
	}()
	return nil
}

// watchAuthGateway observes changes to the auth gateway config map and
// updates Teleport configuration appropriately.
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
		processconfig.ReplacePublicAddrs(p.teleportProcess().Config, config)
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

// startServiceConfigWatch watches the clusterconfiguration configmap and updates
// the gravity-site service configurations if the gravityControllerService has
// been modified.
func (p *Process) startServiceConfigWatch(ctx context.Context, client *kubernetes.Clientset) error {
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			err := p.watchServiceConfig(ctx, client)
			if err != nil {
				p.Errorf("Failed to start service config watch: %v.", trace.DebugReport(err))
			}
			select {
			case <-ticker.C:
			case <-ctx.Done():
				p.Debug("Service config watcher stopped.")
				return
			}
		}

	}()
	return nil
}

func (p *Process) watchServiceConfig(ctx context.Context, client *kubernetes.Clientset) error {
	p.Debug("Restarting service config watch.")

	watcher, err := client.CoreV1().ConfigMaps(defaults.KubeSystemNamespace).Watch(metav1.ListOptions{
		FieldSelector: fields.OneTermEqualSelector("metadata.name", constants.ClusterConfigurationMap).String(),
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
			if err := updateServiceConfiguration(client); err != nil {
				p.Debug("Failed to update service config: %v.", trace.DebugReport(err))
			}
		case <-ctx.Done():
			p.Debug("Stopping certificate watcher.")
			return nil
		}
	}

}

// getServiceConfiguration returns the current gravityControllerService configuration
// defined in the clusterconfiguration configmap.
func getServiceConfiguration(client *kubernetes.Clientset) (*clusterconfig.GravityControllerService, error) {
	configmap, err := client.CoreV1().ConfigMaps(defaults.KubeSystemNamespace).Get(constants.ClusterConfigurationMap, metav1.GetOptions{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	spec := configmap.Data["spec"]
	if spec == "" {
		return nil, trace.NotFound("clutserconfiguration spec is empty")
	}

	clusterConfig, err := clusterconfig.Unmarshal([]byte(spec))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return clusterConfig.GetGravityControllerServiceConfig(), nil
}

// updateServiceConfiguration updates the gravity-site service configuration.
func updateServiceConfiguration(client *kubernetes.Clientset) error {
	config, err := getServiceConfiguration(client)
	if err != nil {
		return trace.Wrap(err)
	}

	services := client.CoreV1().Services(defaults.KubeSystemNamespace)

	svc, err := services.Get(constants.GravityServiceName, metav1.GetOptions{})
	if err != nil {
		return trace.Wrap(err)
	}

	// Set default service type and annotations.
	svcType := v1.ServiceType(clusterconfig.LoadBalancer)
	annotations := map[string]string{
		clusterconfig.AWSIdleTimeoutKey: clusterconfig.AWSLoadBalancerIdleTimeout,
		clusterconfig.AWSInternalKey:    clusterconfig.AWSLoadBalancerInternal,
	}

	if !config.IsEmpty() {
		if config.Type != "" {
			svcType = v1.ServiceType(config.Type)
		}
		for key, val := range config.Annotations {
			annotations[key] = val
		}
	}

	// shouldUpdate indicates that a change has been made to the service
	// configuration and should be updated.
	var shouldUpdate bool

	if svc.Spec.Type != svcType {
		svc.Spec.Type = svcType
		shouldUpdate = true
	}

	if len(svc.Annotations) != len(annotations) {
		svc.Annotations = annotations
		shouldUpdate = true
	} else {
		for key, updatedVal := range annotations {
			existingVal, exists := svc.Annotations[key]
			if !exists || existingVal != updatedVal {
				svc.Annotations = annotations
				shouldUpdate = true
				break
			}
		}
	}

	if !shouldUpdate {
		return nil
	}

	if _, err := services.Update(svc); err != nil {
		return trace.Wrap(err)
	}

	return nil
}
