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
