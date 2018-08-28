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
	for key, _ := range s {
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
	for el, _ := range right {
		s.Add(el)
	}
}

func (s StringSet) Has(item string) (exists bool) {
	_, exists = s[item]
	return exists
}
