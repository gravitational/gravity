// Copyright 2015 Kevin Gillette. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package set

import "sort"

// The Cmp type can be used to represent any of the comparison functions,
// such as IsInter.
type Cmp func(data sort.Interface, pivot int) bool

// IsSub returns true only if all elements in the range [0:pivot] are
// also present in the range [pivot:Len].
func IsSub(data sort.Interface, pivot int) bool {
	i, j, k, l := 0, pivot, pivot, data.Len()
	for i < k && j < l {
		switch {
		case data.Less(i, j):
			return false
		case data.Less(j, i):
			j++
		default:
			i, j = i+1, j+1
		}
	}
	return i == k
}

// IsSuper returns true only if all elements in the range [pivot:Len] are
// also present in the range [0:pivot]. IsSuper is especially useful for
// full membership testing.
func IsSuper(data sort.Interface, pivot int) bool {
	i, j, k, l := 0, pivot, pivot, data.Len()
	for i < k && j < l {
		switch {
		case data.Less(i, j):
			i++
		case data.Less(j, i):
			return false
		default:
			i, j = i+1, j+1
		}
	}
	return j == l
}

// IsInter returns true if any element in the range [0:pivot] is also
// present in the range [pivot:Len]. IsInter is especially useful for
// partial membership testing.
func IsInter(data sort.Interface, pivot int) bool {
	i, j, k, l := 0, pivot, pivot, data.Len()
	for i < k && j < l {
		switch {
		case data.Less(i, j):
			i++
		case data.Less(j, i):
			j++
		default:
			return true
		}
	}
	return false
}

// IsEqual returns true if the sets [0:pivot] and [pivot:Len] are equal.
func IsEqual(data sort.Interface, pivot int) bool {
	k, l := pivot, data.Len()
	if k*2 != l {
		return false
	}
	for i := 0; i < k; i++ {
		p, q := k-i-1, l-i-1
		if data.Less(p, q) || data.Less(q, p) {
			return false
		}
	}
	return true
}
