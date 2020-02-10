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

package cli

import (
	"context"
	"io"
	"os"

	"github.com/gravitational/gravity/lib/localenv"
	"github.com/gravitational/gravity/lib/report"
	"github.com/gravitational/trace"
)

// systemReport collects system diagnostics and outputs them as a (optionally compressed) tarball
// to the specified writer w.
// filters defines the specific diagnostics to collect, if unspecified - all diagnostics will be collected.
func systemReport(env *localenv.LocalEnvironment, filters []string, compressed bool, outputPath string) error {
	var w io.Writer = os.Stdout
	if outputPath != "" {
		f, err := os.Create(outputPath)
		if err != nil {
			return trace.ConvertSystemError(err)
		}
		defer f.Close()
		w = f
	}
	config := report.Config{
		Filters:    filters,
		Compressed: compressed,
		Packages:   env.Packages,
	}
	return report.Collect(context.TODO(), config, w)
}
