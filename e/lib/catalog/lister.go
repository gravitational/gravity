// Copyright 2021 Gravitational Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package catalog

import (
	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/catalog"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/schema"

	"github.com/gravitational/trace"
)

type appLister struct {
	apps app.Applications
}

// NewLister returns a lister that uses Ops Center's app service.
func NewLister(env *localenv.LocalEnvironment) (*appLister, error) {
	url, err := env.SelectOpsCenterWithDefault("", defaults.DistributionOpsCenter)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	apps, err := env.AppService(url, localenv.AppConfig{})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &appLister{apps: apps}, nil
}

// List returns application and cluster images from the Ops Center.
func (l *appLister) List(_ bool) (result catalog.ListItems, err error) {
	items, err := l.apps.ListApps(app.ListAppsRequest{
		Repository: defaults.SystemAccountOrg,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	for _, item := range items {
		switch item.Manifest.Kind {
		case schema.KindBundle, schema.KindCluster, schema.KindApplication:
		default:
			continue
		}
		i, err := catalog.NewListItemFromApp(item)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		result = append(result, i)
	}
	return result, nil
}
