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

package utils

import "sort"

/*
StringSet is a set of unique strings
*/

type StringSet map[string]struct{}

func NewStringSet() StringSet {
	return make(StringSet)
}

func NewStringSetFromSlice(slice []string) StringSet {
	s := make(StringSet)
	s.AddSlice(slice)
	return s
}

func (s StringSet) Add(v string) {
	s[v] = struct{}{}
}

func (s StringSet) Remove(v string) {
	delete(s, v)
}

func (s StringSet) Slice() (slice []string) {
	slice = make([]string, 0, len(s))
	for key := range s {
		slice = append(slice, key)
	}
	sort.Strings(slice)
	return slice
}

func (s StringSet) AddSlice(slice []string) {
	for _, el := range slice {
		s.Add(el)
	}
}

func (s StringSet) AddSet(right StringSet) {
	for el := range right {
		s.Add(el)
	}
}

func (s StringSet) Has(item string) (exists bool) {
	_, exists = s[item]
	return exists
}

// Diff returns difference between this and provided set.
func (s StringSet) Diff(another StringSet) StringSet {
	diff := NewStringSet()
	for item := range s {
		if !another.Has(item) {
			diff.Add(item)
		}
	}
	for item := range another {
		if !s.Has(item) {
			diff.Add(item)
		}
	}
	return diff
}
