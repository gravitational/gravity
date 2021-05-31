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

package handler

import (
	"io"
	"net/http"

	"github.com/gravitational/gravity/lib/defaults"
	"github.com/gravitational/gravity/lib/loc"
	helmutils "github.com/gravitational/gravity/lib/utils/helm"

	"github.com/gravitational/trace"
	"github.com/julienschmidt/httprouter"
)

/* getIndexFile returns the Helm chart repository index file.

   GET /charts/index.yaml

   This handler allows the app service to function as a valid Helm chart
   repository because Helm client calls it to update information about
   available charts.
*/
func (h *WebHandler) getIndexFile(w http.ResponseWriter, _ *http.Request, _ httprouter.Params, context *handlerContext) error {
	reader, err := context.applications.FetchIndexFile()
	if err != nil {
		return trace.Wrap(err)
	}
	w.Header().Set("Content-Type", "application/yaml")
	_, err = io.Copy(w, reader)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

/* fetchChart returns Helm chart archive for the specified application.

   GET /charts/:name
   GET /app/v1/charts/:name

   The name parameter is the chart archive filename that is formatted
   as "<name>-<ver>.tgz", for example "alpine-0.1.0.tgz".

   If the name is "index.yaml", then repository index file is returned,
   see "getIndexFile" handler for details.
*/
func (h *WebHandler) fetchChart(w http.ResponseWriter, r *http.Request, p httprouter.Params, context *handlerContext) error {
	name := p.ByName("name")
	if name == "" {
		return trace.BadParameter("empty chart filename")
	}
	if name == "index.yaml" {
		return h.getIndexFile(w, r, p, context)
	}
	chartName, chartVersion, err := helmutils.ParseChartFilename(name)
	if err != nil {
		return trace.Wrap(err)
	}
	locator, err := loc.NewLocator(defaults.SystemAccountOrg, chartName, chartVersion)
	if err != nil {
		return trace.Wrap(err)
	}
	reader, err := context.applications.FetchChart(*locator)
	if err != nil {
		return trace.Wrap(err)
	}
	defer reader.Close()
	w.Header().Set("Content-Type", "application/gzip")
	_, err = io.Copy(w, reader)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}
