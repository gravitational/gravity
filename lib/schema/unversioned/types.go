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

package unversioned

// MultiSourceValue defines a type with multiple value sources
type MultiSourceValue struct {
	// Env is the name of environment variable to read value from
	Env string `json:"env,omitempty"`
	// Path is the path to the file to read value from
	Path string `json:"path,omitempty"`
	// Value is the literal value
	Value string `json:"value,omitempty"`
}

// IsEmpty determines if this multi-source value is empty
func (v MultiSourceValue) IsEmpty() bool {
	return v.Env == "" && v.Path == "" && v.Value == ""
}

// Set sets the literal value for the multi-source value
func (v *MultiSourceValue) Set(value string) {
	v.Env = ""
	v.Path = ""
	v.Value = value
}
