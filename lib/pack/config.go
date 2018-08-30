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
	"encoding/json"
	"io"

	"github.com/gravitational/trace"
)

func WriteConfigPackage(m *Manifest, w io.Writer) error {
	vars := m.Config.EnvVars()
	b, err := json.Marshal(vars)
	if err != nil {
		return trace.Wrap(err)
	}
	cm := Manifest{
		Version: Version,
		Labels: []Label{
			{Name: "type", Value: "orbit/config"},
		},
	}
	return WritePackage(cm, w, []PackageFile{
		{Path: "vars.json", Contents: b},
	})
}

func ReadConfigPackage(r io.Reader) (map[string]string, error) {
	m, files, err := ReadPackage(r)
	if err != nil {
		return nil, err
	}

	if m.Label("type") != "orbit/config" {
		return nil, trace.Errorf(
			"expected label 'type':'orbit/config', got: '%v'", m.Label("type"))
	}

	var bytes []byte
	for _, f := range files {
		if f.Path == "vars.json" {
			bytes = f.Contents
			break
		}
	}
	if bytes == nil {
		return nil, trace.Errorf(
			"expected label 'type':'orbit/config', got: '%v'", m.Label("type"))
	}

	var vals map[string]string
	if err := json.Unmarshal(bytes, &vals); err != nil {
		return nil, trace.Wrap(err, "failed to decode variables")
	}

	return vals, nil
}
