// Copyright 2015 Kevin Gillette. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package set implements type-safe, non-allocating algorithms that operate
// on ordered sets.
//
// Most functions take a data parameter of type sort.Interface and a pivot
// parameter of type int; data represents two sets covering the ranges
// [0:pivot] and [pivot:Len], each of which is expected to be sorted and
// free of duplicates. sort.Sort may be used for sorting, and Uniq may be
// used to filter away duplicates.
//
// All mutating functions swap elements as necessary from the two input sets
// to form a single output set, returning its size: the output set will be
// in the range [0:size], and will be in sorted order and free of
// duplicates. Elements which were moved into the range [size:Len] will have
// undefined order and may contain duplicates.
//
// All pivots must be in the range [0:Len]. A panic may occur when invalid
// pivots are passed into any of the functions.
//
// Convenience functions exist for slices of int, float64, and string
// element types, and also serve as examples for implementing utility
// functions for other types.
//
// Elements will be considered equal if `!Less(i,j) && !Less(j,i)`. An
// implication of this is that NaN values are equal to each other.
package set

// BUG(extemporalgenome): All ops should use binary search when runs are detected
