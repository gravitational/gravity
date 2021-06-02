/*
Copyright 2020 Gravitational, Inc.
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

package common

import (
	"io"
	"sort"

	"github.com/gravitational/magnet"

	"github.com/olekukonko/tablewriter"
)

// WriteEnvs outputs environment variables that can override build options to w
func WriteEnvs(environ map[string]magnet.EnvVar, w io.Writer) error {
	var result [][]string

	for key, value := range environ {
		if value.Secret {
			result = append(result, []string{key, "<redacted>", "", value.Short})
		} else {
			result = append(result, []string{key, value.Value, value.Default, value.Short})
		}
	}

	sort.SliceStable(result, func(i, j int) bool {
		return result[i][0] < result[j][0]
	})

	table := tablewriter.NewWriter(w)
	table.SetHeader([]string{"Env", "Value", "Default", "Short Description"})
	table.SetBorder(false)
	table.SetAutoWrapText(false)
	table.SetReflowDuringAutoWrap(false)

	table.AppendBulk(result)
	table.Render()

	return nil
}
