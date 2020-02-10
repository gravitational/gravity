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

package pack

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/gravitational/gravity/lib/compare"
	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	"github.com/gravitational/gravity/lib/pack"
	"github.com/gravitational/gravity/lib/schema"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/vacuum/prune"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	. "gopkg.in/check.v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestVacuum(t *testing.T) { TestingT(t) }

type S struct{}

var _ = Suite(&S{})

func (*S) SetUpSuite(c *C) {
	if testing.Verbose() {
		log.SetLevel(log.DebugLevel)
	}
}

func (*S) TestDoesnotPruneDirectDependencies(c *C) {
	// setup
	runtimePackage := newPackage("gravitational.io/planet:0.0.1", pack.PurposeLabel, pack.PurposeRuntime)
	app := newAppPackage("gravitational.io/app:0.0.1", storage.AppUser)
	runtimeApp := newAppPackage("gravitational.io/runtime:0.0.1", storage.AppRuntime)
	dependencies := testPackages{
		newPackage("gravitational.io/foo:0.0.1"),
		newPackage("gravitational.io/bar:0.0.1"),
	}

	a, allPackages := newApp(app, runtimeApp, runtimePackage, dependencies...)

	// exercise
	p, err := New(Config{
		App:      a,
		Packages: &dependencies,
	})
	c.Assert(err, IsNil)

	err = p.Prune(context.TODO())
	c.Assert(err, IsNil)

	// verify
	c.Assert(
		byLocator(allPackages),
		compare.SortedSliceEquals,
		byLocator(append([]packageEnvelope{app, runtimeApp, runtimePackage}, dependencies...)),
	)
}

func (*S) TestPrunesOldDependencies(c *C) {
	// setup
	runtimePackage := newPackage("gravitational.io/planet:0.0.1", pack.PurposeLabel, pack.PurposeRuntime)
	app := newAppPackage("gravitational.io/app:0.0.2", storage.AppUser)
	runtimeApp := newAppPackage("gravitational.io/runtime:0.0.1", storage.AppRuntime)
	oldApp := newAppPackage("gravitational.io/app:0.0.1", storage.AppUser)
	dependencies := testPackages{
		newPackage("gravitational.io/foo:0.0.2"),
		newPackage("gravitational.io/bar:0.0.1"),
	}

	a, dependencies := newApp(app, runtimeApp, runtimePackage, dependencies...)
	allPackages := append(dependencies, oldApp, newPackage("gravitational.io/foo:0.0.1"))

	// exercise
	p, err := New(Config{
		App:      a,
		Packages: &allPackages,
	})
	c.Assert(err, IsNil)

	err = p.Prune(context.TODO())
	c.Assert(err, IsNil)

	// verify
	c.Assert(byLocator(allPackages), compare.SortedSliceEquals, byLocator(dependencies))
}

func (*S) TestPrunesOldAppResourcePackages(c *C) {
	// setup
	runtimePackage := newPackage("gravitational.io/planet:0.0.1", pack.PurposeLabel, pack.PurposeRuntime)
	app := newAppPackage("gravitational.io/app:0.0.2", storage.AppUser)
	oldApp := newAppPackage("gravitational.io/app:0.0.1", storage.AppUser)
	appResources := newPackage("gravitational.io/app-resources:0.0.2")
	runtimeApp := newAppPackage("gravitational.io/runtime:0.0.1", storage.AppRuntime)
	oldAppResources := newPackage("gravitational.io/app-resources:0.0.1")
	dependencies := testPackages{
		newPackage("gravitational.io/foo:0.0.2"),
		newPackage("gravitational.io/bar:0.0.1"),
	}

	a, dependencies := newApp(app, runtimeApp, runtimePackage, dependencies...)
	allPackages := append(dependencies, appResources, oldApp, oldAppResources)

	// exercise
	p, err := New(Config{
		Config:   prune.Config{FieldLogger: log.WithField("test", "TestPrunesOldAppResourcePackages")},
		App:      a,
		Packages: &allPackages,
	})
	c.Assert(err, IsNil)

	err = p.Prune(context.TODO())
	c.Assert(err, IsNil)

	// verify
	expected := append(dependencies, appResources)
	c.Assert(byLocator(allPackages), compare.SortedSliceEquals, byLocator(expected),
		Commentf("Should prune old application resource packages"))
}

func (*S) TestNoopIfDryRun(c *C) {
	// setup
	runtimePackage := newPackage("gravitational.io/planet:0.0.1", pack.PurposeLabel, pack.PurposeRuntime)
	app := newAppPackage("gravitational.io/app:0.0.2", storage.AppUser)
	runtimeApp := newAppPackage("gravitational.io/runtime:0.0.1", storage.AppRuntime)
	oldApp := newAppPackage("gravitational.io/app:0.0.1", storage.AppUser)
	dependencies := testPackages{
		newPackage("gravitational.io/foo:0.0.2"),
		newPackage("gravitational.io/bar:0.0.1"),
	}

	a, dependencies := newApp(app, runtimeApp, runtimePackage, dependencies...)
	allPackages := append(dependencies, oldApp, newPackage("gravitational.io/foo:0.0.1"))

	// exercise
	p, err := New(Config{
		App:      a,
		Packages: &allPackages,
		Config: prune.Config{
			DryRun: true,
		},
	})
	c.Assert(err, IsNil)

	err = p.Prune(context.TODO())
	c.Assert(err, IsNil)

	// verify
	expected := append(dependencies, oldApp, newPackage("gravitational.io/foo:0.0.1"))
	c.Assert(byLocator(allPackages), compare.SortedSliceEquals, byLocator(expected))
}

func (*S) TestPrunesOldPlanetConfiguration(c *C) {
	// setup
	runtimePackage := newPackage("gravitational.io/planet:0.0.3", pack.PurposeLabel, pack.PurposeRuntime)
	oldRuntimePackage := newPackage("gravitational.io/planet:0.0.2", pack.PurposeLabel, pack.PurposeRuntime)
	app := newAppPackage("gravitational.io/app:0.0.2", storage.AppUser)
	runtimeApp := newAppPackage("gravitational.io/runtime:0.0.1", storage.AppRuntime)
	planetConfig := newPackage("cluster/planet-config:0.0.3",
		pack.PurposeLabel, pack.PurposePlanetConfig,
		pack.ConfigLabel, runtimePackage.Locator.ZeroVersion().String(),
	)
	dependencies := testPackages{
		newPackage("gravitational.io/foo:0.0.2"),
		newPackage("gravitational.io/bar:0.0.1"),
	}

	a, dependencies := newApp(app, runtimeApp, runtimePackage, dependencies...)
	oldPlanetConfig := newPackage("cluster/planet-config:0.0.2",
		pack.PurposeLabel, pack.PurposePlanetConfig,
		pack.ConfigLabel, runtimePackage.Locator.ZeroVersion().String(),
	)
	allPackages := append(dependencies, planetConfig, oldPlanetConfig, oldRuntimePackage)

	// exercise
	p, err := New(Config{
		Config: prune.Config{
			FieldLogger: log.WithField("test", "TestPrunesOldPlanetConfiguration"),
		},
		App:      a,
		Packages: &allPackages,
	})
	c.Assert(err, IsNil)

	err = p.Prune(context.TODO())
	c.Assert(err, IsNil)

	// verify
	expected := append(dependencies, planetConfig)
	c.Assert(byLocator(allPackages), compare.SortedSliceEquals, byLocator(expected),
		Commentf("Should prune old planet configuration package"))
}

func (*S) TestPrunesOldPlanetPackages(c *C) {
	// setup
	runtimePackage := newPackage("gravitational.io/planet:0.0.3", pack.PurposeLabel, pack.PurposeRuntime)
	app := newAppPackage("gravitational.io/app:0.0.2", storage.AppUser)
	runtimeApp := newAppPackage("gravitational.io/runtime:0.0.1", storage.AppRuntime)
	oldRuntimePackages := []packageEnvelope{
		newPackage("gravitational.io/planet-master:0.0.2"),
		newPackage("gravitational.io/planet-node:0.0.2"),
	}

	a, dependencies := newApp(app, runtimeApp, runtimePackage)

	allPackages := append(testPackages(dependencies), oldRuntimePackages...)

	// exercise
	p, err := New(Config{
		Config: prune.Config{
			FieldLogger: log.WithField("test", "TestPrunesOldPlanetPackages"),
		},
		App:      a,
		Packages: &allPackages,
	})
	c.Assert(err, IsNil)

	err = p.Prune(context.TODO())
	c.Assert(err, IsNil)

	// verify
	c.Assert(byLocator(allPackages), compare.SortedSliceEquals, byLocator(dependencies),
		Commentf("Should prune old planet packages"))
}

func (*S) TestDoesnotPruneRequiredLegacyPlanetPackages(c *C) {
	// setup
	runtimePackage := newPackage("gravitational.io/planet-master:0.0.3", pack.PurposeLabel, pack.PurposeRuntime)
	app := newAppPackage("gravitational.io/app:0.0.2", storage.AppUser)
	runtimeApp := newAppPackage("gravitational.io/runtime:0.0.1", storage.AppRuntime)
	a, dependencies := newApp(app, runtimeApp, runtimePackage)
	allPackages := append(testPackages{}, dependencies...)

	// exercise
	p, err := New(Config{
		Config: prune.Config{
			FieldLogger: log.WithField("test", "TestDoesnotPruneRequiredLegacyPlanetPackages"),
		},
		App:      a,
		Packages: &allPackages,
	})
	c.Assert(err, IsNil)

	err = p.Prune(context.TODO())
	c.Assert(err, IsNil)

	// verify
	c.Assert(byLocator(allPackages), compare.SortedSliceEquals, byLocator(dependencies),
		Commentf("Should not prune legacy planet packages that are still in use"))
}

func (*S) TestPrunesOldUpdateRPCCredentials(c *C) {
	// setup
	runtimePackage := newPackage("gravitational.io/planet:0.0.3", pack.PurposeLabel, pack.PurposeRuntime)
	app := newAppPackage("gravitational.io/app:0.0.1", storage.AppUser)
	runtimeApp := newAppPackage("gravitational.io/runtime:0.0.3", storage.AppRuntime)
	rpcSecrets := newPackage(fmt.Sprintf("gravitational.io/%v:0.0.1", defaults.RPCAgentSecretsPackage),
		pack.PurposeLabel, pack.PurposeRPCCredentials,
	)
	dependencies := testPackages{
		newPackage("gravitational.io/foo:0.0.2"),
		newPackage("gravitational.io/bar:0.0.1"),
	}

	a, dependencies := newApp(app, runtimeApp, runtimePackage, dependencies...)
	allPackages := append(dependencies, rpcSecrets,
		// These packages should be pruned as they were used for two previous update attempts
		newPackage(fmt.Sprintf("cluster/%v:0.0.2", defaults.RPCAgentSecretsPackage),
			pack.PurposeLabel, pack.PurposeRPCCredentials),
	)

	// exercise
	p, err := New(Config{
		App:      a,
		Packages: &allPackages,
	})
	c.Assert(err, IsNil)

	err = p.Prune(context.TODO())
	c.Assert(err, IsNil)

	// verify
	// Long-term RPC credentials package should be retained
	expected := append(dependencies, rpcSecrets)
	c.Assert(byLocator(allPackages), compare.SortedSliceEquals, byLocator(expected))
}

func (*S) TestDoesnotPrunePackagesFromClusterRepositories(c *C) {
	// setup
	runtimePackage := newPackage("gravitational.io/planet:0.0.3", pack.PurposeLabel, pack.PurposeRuntime)
	app := newAppPackage("gravitational.io/app:0.0.1", storage.AppUser)
	runtimeApp := newAppPackage("gravitational.io/runtime:0.0.3", storage.AppRuntime)
	dependencies := testPackages{
		newPackage("gravitational.io/foo:0.0.2"),
		newPackage("gravitational.io/bar:0.0.1"),
	}

	a, dependencies := newApp(app, runtimeApp, runtimePackage, dependencies...)
	allPackages := append(dependencies,
		newPackage("cluster/foo:0.0.1"),
		newPackage("cluster/bar:0.0.2"),
	)

	// exercise
	p, err := New(Config{
		App:      a,
		Packages: &allPackages,
	})
	c.Assert(err, IsNil)

	err = p.Prune(context.TODO())
	c.Assert(err, IsNil)

	// verify
	expected := append(dependencies,
		newPackage("cluster/foo:0.0.1"),
		newPackage("cluster/bar:0.0.2"),
	)
	c.Assert(byLocator(allPackages), compare.SortedSliceEquals, byLocator(expected))
}

func (*S) TestDoesnotPrunePackagesFromRemoteClusters(c *C) {
	// setup
	runtimePackage := newPackage("gravitational.io/planet:0.0.3", pack.PurposeLabel, pack.PurposeRuntime)
	app := newAppPackage("gravitational.io/app:0.0.1", storage.AppUser)
	runtimeApp := newAppPackage("gravitational.io/runtime:0.0.3", storage.AppRuntime)
	dependencies := testPackages{
		newPackage("gravitational.io/foo:0.0.2"),
		newPackage("gravitational.io/bar:0.0.1"),
	}
	a, dependencies := newApp(app, runtimeApp, runtimePackage, dependencies...)

	remoteRuntimePackage := newPackage("gravitational.io/planet:0.0.2", pack.PurposeLabel, pack.PurposeRuntime)
	remoteApp := newAppPackage("gravitational.io/remote-app:0.0.3", storage.AppUser)
	remoteRuntimeApp := newAppPackage("gravitational.io/runtime:0.0.2", storage.AppRuntime)
	remote, remoteDependencies := newApp(remoteApp, remoteRuntimeApp, remoteRuntimePackage)

	allPackages := append(dependencies,
		newPackage("cluster/foo:0.0.1"),
		newPackage("cluster/bar:0.0.2"),
	)
	allPackages = append(allPackages, remoteDependencies...)

	apps := []storage.Application{*remote}

	// exercise
	p, err := New(Config{
		App:      a,
		Apps:     apps,
		Packages: &allPackages,
	})
	c.Assert(err, IsNil)

	err = p.Prune(context.TODO())
	c.Assert(err, IsNil)

	// verify
	expected := append(dependencies,
		newPackage("cluster/foo:0.0.1"),
		newPackage("cluster/bar:0.0.2"),
	)
	expected = append(expected, remoteDependencies...)
	c.Assert(byLocator(allPackages), compare.SortedSliceEquals, byLocator(expected))
}

func (*S) TestPrunesOldRuntimeAppPackage(c *C) {
	// setup
	runtimePackage := newPackage("gravitational.io/planet:0.0.3", pack.PurposeLabel, pack.PurposeRuntime)
	app := newAppPackage("gravitational.io/app:0.0.1", storage.AppUser)
	runtimeApp := newAppPackage("gravitational.io/runtime:0.0.3", storage.AppRuntime)
	oldRuntimeApp := newAppPackage("gravitational.io/runtime:0.0.2", storage.AppRuntime)
	a, dependencies := newApp(app, runtimeApp, runtimePackage)
	allPackages := append(testPackages(dependencies), oldRuntimeApp)

	// exercise
	p, err := New(Config{
		App:      a,
		Packages: &allPackages,
	})
	c.Assert(err, IsNil)

	err = p.Prune(context.TODO())
	c.Assert(err, IsNil)

	// verify
	c.Assert(byLocator(allPackages), compare.SortedSliceEquals, byLocator(dependencies))
}

func (*S) TestDoesnotPruneUnrelatedPackages(c *C) {
	// setup
	runtimePackage := newPackage("gravitational.io/planet:0.0.1", pack.PurposeLabel, pack.PurposeRuntime)
	app := newAppPackage("gravitational.io/app:0.0.1", storage.AppUser)
	runtimeApp := newAppPackage("gravitational.io/runtime:0.0.1", storage.AppRuntime)
	anotherApp := newAppPackage("gravitational.io/app2:0.0.2", storage.AppUser)
	a, dependencies := newApp(app, runtimeApp, runtimePackage)
	allPackages := append(testPackages(dependencies), anotherApp)

	// exercise
	p, err := New(Config{
		App:      a,
		Packages: &allPackages,
	})
	c.Assert(err, IsNil)

	err = p.Prune(context.TODO())
	c.Assert(err, IsNil)

	// verify
	expected := append(dependencies, anotherApp)
	c.Assert(byLocator(allPackages), compare.SortedSliceEquals, byLocator(expected))
}

func newApp(app, runtimeApp, runtimePackage packageEnvelope, dependencies ...packageEnvelope) (*storage.Application, []packageEnvelope) {
	m := schema.Manifest{
		Header: schema.Header{
			TypeMeta: metav1.TypeMeta{
				Kind:       schema.KindBundle,
				APIVersion: schema.APIVersionV2,
			},
			Metadata: schema.Metadata{
				Name:            app.Locator.Name,
				ResourceVersion: app.Locator.Version,
			},
		},
		SystemOptions: &schema.SystemOptions{
			Runtime: &schema.Runtime{
				Locator: runtimeApp.Locator,
			},
			Dependencies: schema.SystemDependencies{
				Runtime: &schema.Dependency{
					Locator: runtimePackage.Locator,
				},
			},
		},
	}
	for _, dep := range dependencies {
		if dep.Type == "" {
			m.Dependencies.Packages = append(m.Dependencies.Packages, schema.Dependency{Locator: dep.Locator})
		} else {
			m.Dependencies.Apps = append(m.Dependencies.Apps, schema.Dependency{Locator: dep.Locator})
		}
	}
	return &storage.Application{
		Locator:  app.Locator,
		Manifest: m,
	}, append(dependencies, app, runtimeApp, runtimePackage)
}

func newPackage(pkg string, labels ...string) packageEnvelope {
	return packageEnvelope{
		Locator:       loc.MustParseLocator(pkg),
		RuntimeLabels: asLabels(labels...),
	}
}

func newAppPackage(pkg string, typ_ storage.AppType, labels ...string) packageEnvelope {
	return packageEnvelope{
		Locator:       loc.MustParseLocator(pkg),
		Type:          string(typ_),
		RuntimeLabels: asLabels(labels...),
	}
}

func (r testPackages) GetRepositories() (repositories []string, err error) {
	rs := make(map[string]struct{}, len(r))
	for _, envelope := range r {
		if _, exists := rs[envelope.Locator.Repository]; !exists {
			rs[envelope.Locator.Repository] = struct{}{}
		}
	}
	for repository := range rs {
		repositories = append(repositories, repository)
	}
	return repositories, nil
}

func (r testPackages) GetPackages(repository string) (envelopes []pack.PackageEnvelope, err error) {
	for _, envelope := range r {
		if envelope.Locator.Repository == repository {
			envelopes = append(envelopes, pack.PackageEnvelope(envelope))
		}
	}
	return envelopes, nil
}

func (r testPackages) ReadPackageEnvelope(loc loc.Locator) (*pack.PackageEnvelope, error) {
	for _, envelope := range r {
		if envelope.Locator.IsEqualTo(loc) {
			return (*pack.PackageEnvelope)(&envelope), nil
		}
	}
	return nil, trace.NotFound("no package %v found", loc)
}

func (r *testPackages) DeletePackage(loc loc.Locator) error {
	for i := range *r {
		if (*r)[i].Locator.IsEqualTo(loc) {
			*r = append((*r)[:i], (*r)[i+1:]...)
			break
		}
	}
	return nil
}

func asLabels(labels ...string) map[string]string {
	if len(labels) == 0 {
		return nil
	}

	result := make(map[string]string)
	labelsTo(result, labels...)
	return result
}

func labelsTo(result map[string]string, labels ...string) {
	var label, value string
	for len(labels) > 1 {
		label, value, labels = labels[0], labels[1], labels[2:]
		result[label] = value
	}
	if len(labels) == 1 {
		result[labels[0]] = ""
	}
}

type testPackages []packageEnvelope

func (r packageEnvelope) String() string {
	return r.Locator.String()
}

type packageEnvelope pack.PackageEnvelope

func (r byLocator) GoString() string {
	var result []string
	for _, env := range r {
		result = append(result, env.Locator.String())
	}
	return strings.Join(result, ",")
}

func (r byLocator) Len() int { return len(r) }
func (r byLocator) Swap(i, j int) {
	r[i], r[j] = r[j], r[i]
}
func (r byLocator) Less(i, j int) bool {
	return r[i].Locator.String() < r[j].Locator.String()
}

type byLocator []packageEnvelope
