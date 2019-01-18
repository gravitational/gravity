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

package helm

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"

	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/manifest"
	"k8s.io/helm/pkg/proto/hapi/chart"
	"k8s.io/helm/pkg/renderutil"
	"k8s.io/helm/pkg/timeconv"

	"github.com/gravitational/trace"
)

// RenderParameters defines parameters to render Helm template.
type RenderParameters struct {
	// Path is a chart path.
	Path string
	// Values is a list of YAML files with values.
	Values []string
	// Set is a list of values set on the CLI.
	Set []string
}

// RenderHelm renders templates of a provided Helm chart.
func Render(p RenderParameters) ([]byte, error) {
	rawVals, err := vals(p.Values, p.Set, nil, nil, "", "", "")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	config := &chart.Config{
		Raw:    string(rawVals),
		Values: map[string]*chart.Value{},
	}
	ch, err := chartutil.Load(p.Path)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	options := renderutil.Options{
		ReleaseOptions: chartutil.ReleaseOptions{
			Time: timeconv.Now(),
		},
	}
	renderedTemplates, err := renderutil.Render(ch, config, options)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	var b bytes.Buffer
	for _, m := range manifest.SplitManifests(renderedTemplates) {
		filename := filepath.Base(m.Name)
		// Render only Kubernetes resources skipping internal Helm
		// files and files that begin with underscore which are not
		// expected to output a Kubernetes spec.
		if filename == "NOTES.txt" || strings.HasPrefix(filename, "_") {
			continue
		}
		b.WriteString(fmt.Sprintf("---\n%v\n", m.Content))
	}
	return b.Bytes(), nil
}
