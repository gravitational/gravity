// Copyright 2015 Kevin Gillette. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package set

import "sort"

// Ints sorts and deduplicates a slice of ints in place, returning the
// resulting set.
func Ints(data []int) []int {
	sort.Ints(data)
	n := Uniq(sort.IntSlice(data))
	return data[:n]
}

// Float64s sorts and deduplicates a slice of float64s in place, returning
// the resulting set.
func Float64s(data []float64) []float64 {
	sort.Float64s(data)
	n := Uniq(sort.Float64Slice(data))
	return data[:n]
}

// Strings sorts and deduplicates a slice of strings in place, returning
// the resulting set.
func Strings(data []string) []string {
	sort.Strings(data)
	n := Uniq(sort.StringSlice(data))
	return data[:n]
}

// IntsDo applies op to the int sets, s and t, returning the result.
// s and t must already be individually sorted and free of duplicates.
func IntsDo(op Op, s []int, t ...int) []int {
	data := sort.IntSlice(append(s, t...))
	n := op(data, len(s))
	return data[:n]
}

// Float64sDo applies op to the float64 sets, s and t, returning the result.
// s and t must already be individually sorted and free of duplicates.
func Float64sDo(op Op, s []float64, t ...float64) []float64 {
	data := sort.Float64Slice(append(s, t...))
	n := op(data, len(s))
	return data[:n]
}

// StringsDo applies op to the string sets, s and t, returning the result.
// s and t must already be individually sorted and free of duplicates.
func StringsDo(op Op, s []string, t ...string) []string {
	data := sort.StringSlice(append(s, t...))
	n := op(data, len(s))
	return data[:n]
}

// IntsChk compares s and t according to cmp.
func IntsChk(cmp Cmp, s []int, t ...int) bool {
	data := sort.IntSlice(append(s, t...))
	return cmp(data, len(s))
}

// Float64sChk compares s and t according to cmp.
func Float64sChk(cmp Cmp, s []float64, t ...float64) bool {
	data := sort.Float64Slice(append(s, t...))
	return cmp(data, len(s))
}

// StringsChk compares s and t according to cmp.
func StringsChk(cmp Cmp, s []string, t ...string) bool {
	data := sort.StringSlice(append(s, t...))
	return cmp(data, len(s))
}
