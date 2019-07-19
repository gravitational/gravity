/*
Copyright 2019 Gravitational, Inc.

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

package status

import (
	"context"
	"strings"

	httptransport "github.com/go-openapi/runtime/client"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/httplib"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/utils"
	"github.com/gravitational/trace"
	alertmanager "github.com/prometheus/alertmanager/api/v2/client"
	"github.com/prometheus/alertmanager/api/v2/client/alert"
	"github.com/prometheus/alertmanager/api/v2/models"
)

const (
	alertname    = "alertname"
	watchdog     = "Watchdog"
	job          = "job"
	satellite    = "satellite"
	rolloutStuck = "KubeDaemonSetRolloutStuck"
	daemonset    = "daemonset"
	message      = "message"
	firing       = "firing"
	severity     = "severity"
	critical     = "critical"
)

// FromAlertManager collects alerts from the prometheus alertmanager deployed to the cluster
func FromAlertManager(ctx context.Context, cluster ops.Site) ([]*models.GettableAlert, error) {
	client, err := httplib.GetPlanetClient(httplib.WithLocalResolver(cluster.DNSConfig.Addr()))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	transport := httptransport.NewWithClient(defaults.AlertmanagerServiceAddr, "/api/v2",
		[]string{"http"}, client)

	am := alertmanager.New(transport, nil)
	getOk, err := am.Alert.GetAlerts(alert.NewGetAlertsParams().WithContext(ctx))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return filterAlerts(getOk.Payload), nil
}

// filterAlerts prevents expected alerts from being returned
func filterAlerts(alerts []*models.GettableAlert) []*models.GettableAlert {
	filtered := []*models.GettableAlert{}
	hasWatchdog := false

	for _, alert := range alerts {
		if alert.Labels[alertname] == watchdog {
			hasWatchdog = true
			// filter the Watchdog alert, which should constantly be firing
			continue
		}

		// filter satellite alerts, because they're already collected directly from satellite
		if job, ok := alert.Labels[job]; ok && job == satellite {
			continue
		}

		// Prometheus detects the gravity-site election process as an error since only one pod is ever ready
		// alertname: KubeDaemonSetRolloutStuck
		// daemonset: gravity-site
		// message: Only 33.33333333333333% of the desired Pods of DaemonSet kube-system/gravity-site are scheduled...
		if name, ok := alert.Labels[alertname]; ok && name == rolloutStuck {
			if ds, ok := alert.Labels[daemonset]; ok && ds == constants.GravityServiceName {
				if msg, ok := alert.Annotations[message]; ok && strings.Contains(msg, "33.333333") {
					continue
				}
			}
		}

		filtered = append(filtered, alert)
	}

	// if we didn't find the watchdog alert, it indicates there is a problem with the alerting system, acting as a
	// sort of deadman switch
	if !hasWatchdog {
		filtered = append(filtered, &models.GettableAlert{
			Status: &models.AlertStatus{
				State: utils.StringPtr(firing),
			},
			Annotations: models.LabelSet{
				message: "Alertmanager watchdog failed",
			},
			Alert: models.Alert{
				Labels: models.LabelSet{
					alertname: "WatchdogDown",
					severity:  critical,
				},
			},
		})
	}
	return filtered
}
