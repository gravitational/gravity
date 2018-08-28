// Copyright 2015 Kevin Gillette. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package set

import "sort"

func xcopy(data sort.Interface, i, j, k, l int) int {
	for i < k && j < l {
		data.Swap(i, j)
		i, j = i+1, j+1
	}
	return i
}

func slide(data sort.Interface, i, j, n int) {
	xcopy(data, i, j, i+n, j+n)
}

/*
func find(data sort.Interface, x, i, j int) int {
	return sort.Search(j-i, func(y int) bool {
		return !data.Less(x, i+y)
	})
}
*/
