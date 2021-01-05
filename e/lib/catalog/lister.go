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
