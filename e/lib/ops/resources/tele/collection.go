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

package tele

import (
	"fmt"
	"io"

	"github.com/gravitational/gravity/lib/app"
	"github.com/gravitational/gravity/lib/storage"
	"github.com/gravitational/gravity/lib/utils"
	"github.com/gravitational/gravity/tool/common"

	"github.com/buger/goterm"
	teleservices "github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
)

type clusterCollection struct {
	clusters []storage.Cluster
}

// Resources returns the resources collection in the generic format
func (c *clusterCollection) Resources() (resources []teleservices.UnknownResource, err error) {
	for _, item := range c.clusters {
		resource, err := utils.ToUnknownResource(item)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		resources = append(resources, *resource)
	}
	return resources, nil
}

// WriteText serializes collection in human-friendly text format
func (c *clusterCollection) WriteText(w io.Writer) error {
	t := goterm.NewTable(0, 10, 5, ' ', 0)
	common.PrintTableHeader(t, []string{"Name", "Status", "Cloud Provider", "Region"})
	for _, cluster := range c.clusters {
		fmt.Fprintf(t, "%v\t%v\t%v\t%v\n",
			cluster.GetName(),
			cluster.GetStatus(),
			cluster.GetProvider(),
			cluster.GetRegion(),
		)
	}
	_, err := io.WriteString(w, t.String())
	return trace.Wrap(err)
}

// WriteJSON serializes collection into JSON format
func (c *clusterCollection) WriteJSON(w io.Writer) error {
	return utils.WriteJSON(c, w)
}

// WriteYAML serializes collection into YAML format
func (c *clusterCollection) WriteYAML(w io.Writer) error {
	return utils.WriteYAML(c, w)
}

func (c *clusterCollection) ToMarshal() interface{} {
	if len(c.clusters) == 1 {
		return c.clusters[0]
	}
	return c.clusters
}

type appCollection []app.Application

// Resources returns the resources collection in the generic format
func (c appCollection) Resources() (resources []teleservices.UnknownResource, err error) {
	// app.Application is not a Resource at the moment
	return nil, trace.NotImplemented("can't convert applications to resources")
}

// WriteText serializes collection in human-friendly text format
func (r appCollection) WriteText(w io.Writer) error {
	t := goterm.NewTable(0, 10, 5, ' ', 0)
	common.PrintTableHeader(t, []string{"Name", "Version"})
	for _, app := range r {
		fmt.Fprintf(t, "%v\t%v\n", app.Package.Name, app.Package.Version)
	}
	_, err := io.WriteString(w, t.String())
	return trace.Wrap(err)
}

// WriteYAML serializes collection into YAML format
func (r appCollection) WriteYAML(w io.Writer) error {
	return utils.WriteYAML(r, w)
}

// WriteJSON serializes collection into JSON format
func (r appCollection) WriteJSON(w io.Writer) error {
	return utils.WriteJSON(r, w)
}

func (r appCollection) ToMarshal() interface{} {
	if len(r) == 1 {
		return r[0]
	}
	return r
}
