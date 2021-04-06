/*
Copyright 2016 Gravitational, Inc.
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

package utils

import "sort"

// StringSet is a set of unique strings
type StringSet map[string]struct{}

// NewStringSet returns new string set
func NewStringSet() StringSet {
	return make(StringSet)
}

// NewStringSetFromSlice creates stringset from slice
func NewStringSetFromSlice(slice []string) StringSet {
	s := make(StringSet)
	s.AddSlice(slice)
	return s
}

// Add adds a string to set
func (s StringSet) Add(v string) {
	s[v] = struct{}{}
}

// Remove removes string from set
func (s StringSet) Remove(v string) {
	delete(s, v)
}

// Slice converts string into slice
func (s StringSet) Slice() (slice []string) {
	slice = make([]string, 0, len(s))
	for key, _ := range s {
		slice = append(slice, key)
	}
	sort.Strings(slice)
	return slice
}

// AddSlice appends elements of slice to set
func (s StringSet) AddSlice(slice []string) {
	for _, el := range slice {
		s.Add(el)
	}
}

// AddSet joins two sets
func (s StringSet) AddSet(right StringSet) {
	for el, _ := range right {
		s.Add(el)
	}
}

// Has checks whether argument is present in set
func (s StringSet) Has(item string) (exists bool) {
	_, exists = s[item]
	return exists
}
