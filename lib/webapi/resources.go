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

package webapi

import (
	"context"
	"net/http"
	"strings"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/httplib"
	"github.com/gravitational/gravity/lib/ops"
	"github.com/gravitational/gravity/lib/ops/resources"
	"github.com/gravitational/gravity/lib/ops/resources/gravity"
	"github.com/gravitational/gravity/lib/utils"
	"github.com/gravitational/gravity/lib/webapi/ui"

	telehttplib "github.com/gravitational/teleport/lib/httplib"
	teleservices "github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"
)

// Resources returns resource controller
func (m *Handler) Resources(ctx *AuthContext) (resources.Resources, error) {
	return gravity.New(gravity.Config{Operator: ctx.Operator})
}

// getResourceHandler is GET handler that returns ConfigItems for requested resource kind
func (m *Handler) getResourceHandler(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *AuthContext) (interface{}, error) {
	kind := p.ByName("kind")
	data, err := m.getResources(clusterKey(ctx, p), kind, ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return makeResponse(data)
}

// upsertResourceHandler is POST|PUT handler that upserts a new resource
func (m *Handler) upsertResourceHandler(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *AuthContext) (interface{}, error) {
	var itemToUpsert ui.ConfigItem
	if err := telehttplib.ReadJSON(r, &itemToUpsert); err != nil {
		return nil, trace.Wrap(err)
	}

	rawRes, err := extractYAMLInfo(itemToUpsert.Content)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = validateKind(rawRes.Kind, itemToUpsert.Kind)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	isNew := r.Method == http.MethodPost
	items, err := m.upsertResource(r.Context(), clusterKey(ctx, p), isNew, *rawRes, ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return makeResponse(items)
}

// deleteResourceHandler is DELETE handler that removes a resource by its kind and name values
func (m *Handler) deleteResourceHandler(w http.ResponseWriter, r *http.Request, p httprouter.Params, ctx *AuthContext) (interface{}, error) {
	resourceKind := p.ByName("kind")
	resourceName := p.ByName("name")
	if err := m.deleteResource(r.Context(), clusterKey(ctx, p), resourceKind, resourceName, ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	return httplib.OK(), nil
}

// deleteResource deletes a resource
func (m *Handler) deleteResource(ctx context.Context, key ops.SiteKey, resourceKind string, resourceName string, authCtx *AuthContext) error {
	controller, err := m.plugin.Resources(authCtx)
	if err != nil {
		return trace.Wrap(err)
	}
	return controller.Remove(ctx, resources.RemoveRequest{
		SiteKey: key,
		Kind:    resourceKind,
		Name:    resourceName,
	})
}

// getResources returns a collection of ConfigItem wrappers that contains resources of requested kind
func (m *Handler) getResources(key ops.SiteKey, kind string, ctx *AuthContext) ([]ui.ConfigItem, error) {
	if kind == "" {
		return nil, trace.BadParameter("missing resource kind")
	}
	cluster, err := ctx.Operator.GetSite(key)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	webCtx, err := ui.NewWebContext(ctx.User, ctx.Identity, *cluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	controller, err := m.plugin.Resources(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	collection, err := controller.GetCollection(resources.ListRequest{
		SiteKey:     key,
		Kind:        kind,
		WithSecrets: webCtx.UserACL.AuthConnectors.Read,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	resources, err := collection.Resources()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	items, err := m.cfg.Converter.ToConfigItems(resources)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return items, nil
}

// upsertResource updates a resource and returns ConfigItem wrapper with the updated resource
func (m *Handler) upsertResource(ctx context.Context, key ops.SiteKey, isNew bool, rawRes teleservices.UnknownResource, authCtx *AuthContext) (interface{}, error) {
	exists, err := m.checkIfResourceExists(key, rawRes, authCtx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if exists && isNew {
		return nil, trace.AlreadyExists("%q already exists",
			rawRes.Metadata.Name)
	}
	if !exists && !isNew {
		return nil, trace.NotFound("cannot find resource with a name %q",
			rawRes.Metadata.Name)
	}
	controller, err := m.plugin.Resources(authCtx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	err = controller.Create(ctx, resources.CreateRequest{
		SiteKey:  key,
		Resource: rawRes,
		Upsert:   true,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	items, err := m.cfg.Converter.ToConfigItems([]teleservices.UnknownResource{rawRes})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return items, nil
}

// extractYAMLInfo extracts resource meta information
func extractYAMLInfo(yaml string) (*teleservices.UnknownResource, error) {
	var unknownRes teleservices.UnknownResource
	reader := strings.NewReader(yaml)
	decoder := kyaml.NewYAMLOrJSONDecoder(reader, defaults.DecoderBufferSize)
	err := decoder.Decode(&unknownRes)
	if err != nil {
		return nil, trace.BadParameter("not a valid resource declaration")
	}

	return &unknownRes, nil
}

// checkIfResourceExists returns true if the specified resource already exists
func (m *Handler) checkIfResourceExists(key ops.SiteKey, rawRes teleservices.UnknownResource, ctx *AuthContext) (bool, error) {
	resourceController, err := m.plugin.Resources(ctx)
	if err != nil {
		return false, trace.Wrap(err)
	}
	_, err = resourceController.GetCollection(resources.ListRequest{
		SiteKey: key,
		Kind:    rawRes.Kind,
		Name:    rawRes.Metadata.Name,
	})
	if err != nil && !trace.IsNotFound(err) {
		return false, trace.Wrap(err)
	}
	return err == nil, nil
}

// validateKind verifies that given resource kind matches its expected value.
func validateKind(actual string, expected string) error {
	if expected == teleservices.KindAuthConnector {
		isSupported := utils.StringInSlice(supportedAuthConnectors, actual)
		if isSupported {
			return nil
		}
	}

	if actual == expected {
		return nil
	}

	return trace.BadParameter("invalid value for kind")
}

// supportedAuthConnectors is a list of SSO providers supported by UI
var supportedAuthConnectors = []string{
	teleservices.KindOIDCConnector,
	teleservices.KindSAMLConnector,
	teleservices.KindGithubConnector,
}
