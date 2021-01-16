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

package cli

import (
	"context"
	"os"
	"strings"

	"github.com/gravitational/gravity/e/lib/ops/resources/tele"
	"github.com/gravitational/gravity/lib/constants"
	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/modules"
	"github.com/gravitational/gravity/lib/ops/resources"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/tool/common"

	"github.com/gravitational/trace"
)

func init() {
	modules.SetResources(&teleResources{})
}

// createResource updates or inserts resources from the specified file
func createResource(env *localenv.LocalEnvironment, filename string, upsert bool) error {
	operator, err := env.OperatorService("")
	if err != nil {
		return trace.Wrap(err)
	}
	opsURL, err := env.SelectOpsCenter("")
	if err != nil {
		return trace.Wrap(err)
	}
	apps, err := env.AppService(opsURL, localenv.AppConfig{})
	if err != nil {
		return trace.Wrap(err)
	}
	teleResources := tele.New(tele.Config{
		Operator: operator,
		Apps:     apps,
		Silent:   env.Silent,
	})
	reader, err := common.GetReader(filename)
	if err != nil {
		return trace.Wrap(err)
	}
	defer reader.Close()
	req := resources.CreateRequest{
		Upsert: upsert,
	}
	err = resources.NewControl(teleResources).Create(context.TODO(), reader, req)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func getResources(env *localenv.LocalEnvironment, kind, name string, format constants.Format) error {
	operator, err := env.OperatorService("")
	if err != nil {
		return trace.Wrap(err)
	}
	opsURL, err := env.SelectOpsCenter("")
	if err != nil {
		return trace.Wrap(err)
	}
	apps, err := env.AppService(opsURL, localenv.AppConfig{})
	if err != nil {
		return trace.Wrap(err)
	}
	teleResources := tele.New(tele.Config{
		Operator: operator,
		Apps:     apps,
		Silent:   env.Silent,
	})
	err = resources.NewControl(teleResources).Get(os.Stdout, resources.ListRequest{
		Kind:        kind,
		Name:        name,
		WithSecrets: false,
	}, format)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// removeResource deletes the resource specified with kind and name
func removeResource(env *localenv.LocalEnvironment, kind string, name string, force bool) error {
	operator, err := env.OperatorService("")
	if err != nil {
		return trace.Wrap(err)
	}
	opsURL, err := env.SelectOpsCenter("")
	if err != nil {
		return trace.Wrap(err)
	}
	apps, err := env.AppService(opsURL, localenv.AppConfig{})
	if err != nil {
		return trace.Wrap(err)
	}
	teleResources := tele.New(tele.Config{
		Operator: operator,
		Apps:     apps,
		Silent:   env.Silent,
	})
	req := resources.RemoveRequest{
		Kind:  kind,
		Name:  name,
		Force: force,
	}
	err = resources.NewControl(teleResources).Remove(context.TODO(), req)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// SupportedResources returns a list of resources that can be created/viewed
func (*teleResources) SupportedResources() []string {
	return supportedResources
}

// SupportedResourcesToRemove returns a list of resources that can be removed
func (*teleResources) SupportedResourcesToRemove() []string {
	return supportedResources
}

// CanonicalKind translates the specified kind to canonical form.
// Returns the kind unmodified if the kind is unknown
func (*teleResources) CanonicalKind(kind string) string {
	switch strings.ToLower(kind) {
	case storage.KindApp, "apps":
		return storage.KindApp
	case storage.KindCluster, "clusters":
		return storage.KindCluster
	default:
		return kind
	}
}

type teleResources struct{}

// SupportedResources is a list of resources supported by
// "tele resource create/get/rm" subcommands
var supportedResources = []string{
	storage.KindCluster,
	storage.KindApp,
}
