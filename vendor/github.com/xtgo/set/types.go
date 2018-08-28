// Copyright 2015 Kevin Gillette. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package set

import "sort"

type span struct{ i, j int }

type boundspan struct {
	data sort.Interface
	span
}

func (b boundspan) Len() int           { return b.j - b.i }
func (b boundspan) Less(i, j int) bool { return b.data.Less(b.i+i, b.i+j) }
func (b boundspan) Swap(i, j int)      { b.data.Swap(b.i+i, b.i+j) }
