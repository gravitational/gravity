// Copyright 2015 Kevin Gillette. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package set

import "sort"

// Pivots transforms set-relative sizes into data-absolute pivots. Pivots is
// mostly only useful in conjunction with Apply. The sizes slice sizes may
// be modified by the call.
func Pivots(sizes ...int) []int {
	n := 0
	for i, l := range sizes {
		n += l
		sizes[i] = n
	}
	return sizes
}

// Apply concurrently applies op to all the sets terminated by pivots.
// pivots must contain one higher than the final index in each set, with the
// final element of pivots being equal to data.Len(); this deviates from the
// pivot semantics of other functions (which treat pivot as a delimiter) in
// order to make initializing the pivots slice simpler.
//
// data.Swap and data.Less are assumed to be concurrent-safe. Only
// associative operations should be used (Diff is not associative); see the
// Apply (Diff) example for a workaround. The result of applying SymDiff
// will contain elements that exist in an odd number of sets.
//
// The implementation runs op concurrently on pairs of neighbor sets
// in-place; when any pair has been merged, the resulting set is re-paired
// with one of its neighbor sets and the process repeats until only one set
// remains. The process is adaptive (large sets will not prevent small pairs
// from being processed), and strives for data-locality (only adjacent
// neighbors are paired and data shifts toward the zero index).
func Apply(op Op, data sort.Interface, pivots []int) (size int) {
	switch len(pivots) {
	case 0:
		return 0
	case 1:
		return pivots[0]
	case 2:
		return op(data, pivots[0])
	}

	spans := make([]span, 0, len(pivots)+1)

	// convert pivots into spans (index intervals that represent sets)
	i := 0
	for _, j := range pivots {
		spans = append(spans, span{i, j})
		i = j
	}

	n := len(spans) // original number of spans
	m := n / 2      // original number of span pairs (rounded down)

	// true if the span is being used
	inuse := make([]bool, n)

	ch := make(chan span, m)

	// reverse iterate over every other span, starting with the last;
	// concurrent algo (further below) will pick available pairs operate on
	for i := range spans[:m] {
		i = len(spans) - 1 - i*2
		ch <- spans[i]
	}

	for s := range ch {
		if len(spans) == 1 {
			if s.i != 0 {
				panic("impossible final span")
			}
			// this was the last operation
			return s.j
		}

		// locate the span we received (match on start of span only)
		i := sort.Search(len(spans), func(i int) bool { return spans[i].i >= s.i })

		// store the result (this may change field j but not field i)
		spans[i] = s

		// mark the span as available for use
		inuse[i] = false

		// check the immediate neighbors for availability (prefer left)
		j, k := i-1, i+1
		switch {
		case j >= 0 && !inuse[j]:
			i, j = j, i
		case k < len(spans) && !inuse[k]:
			j = k
		default:
			// nothing to do right now. wait for something else to finish
			continue
		}

		s, t := spans[i], spans[j]

		go func(s, t span) {
			// sizes of the respective sets
			k, l := s.j-s.i, t.j-t.i

			// shift the right-hand span to be adjacent to the left
			slide(data, s.j, t.i, l)

			// prepare a view of the data (abs -> rel indices)
			b := boundspan{data, span{s.i, s.j + l}}

			// store result of op, adjusting for view (rel -> abs)
			s.j = s.i + op(b, k)

			// send the result back to the coordinating goroutine
			ch <- s
		}(s, t)

		// account for the spawn merging that will occur
		s.j += t.j - t.i

		k = j + 1

		// shrink the lists to account for the merger
		spans = append(append(spans[:i], s), spans[k:]...)

		// (and the merged span is now in use as well)
		inuse = append(append(inuse[:i], true), inuse[k:]...)
	}
	panic("unreachable")
}
