package util

import (
	"math"
	"strings"
)

// Map takes a slice and a function from the slice's element type to another (or the same) type, applies the function
// to every entry in the original map, and returns the results in the same order.
func Map[In any, Out any](input []In, f func(In) Out) []Out {
	out := make([]Out, len(input))
	for i, in := range input {
		out[i] = f(in)
	}
	return out
}

// FindFirst takes a slice and a function from the slice's element type to bool, and applies the function to each
// entry, stopping either when the function returns true, or there are no more entries to check. If the function returns
// true for an entry, a pointer to that entry is returned. Otherwise, a nil pointer is returned, and 'found' will be
// false.
func FindFirst[T any](input []T, f func(T) bool) (matching *T, found bool) {
	for _, in := range input {
		if f(in) {
			return &in, true
		}
	}
	return nil, false
}

// CopyMap creates a shallow copy of a map.
func CopyMap[K comparable, V any](input map[K]V) map[K]V {
	out := make(map[K]V, len(input))
	for k, v := range input {
		out[k] = v
	}
	return out
}

// MergeMaps takes two maps, a and b, and merges them into one. If a and b contain the same key, the value in b takes
// priority.
func MergeMaps[K comparable, V any](a, b map[K]V) map[K]V {
	out := make(map[K]V, max(len(a), len(b)))
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		out[k] = v
	}
	return out
}

// Join is identical to the builtin strings.Join, but accepts ~string instead of only pure string.
func Join[K ~string](elems []K, sep string) string {
	switch len(elems) {
	case 0:
		return ""
	case 1:
		return string(elems[0])
	}

	var n int
	if len(sep) > 0 {
		if len(sep) >= math.MaxInt/(len(elems)-1) {
			panic("strings: Join output length overflow")
		}
		n += len(sep) * (len(elems) - 1)
	}
	for _, elem := range elems {
		if len(elem) > math.MaxInt-n {
			panic("strings: Join output length overflow")
		}
		n += len(elem)
	}

	var b strings.Builder
	b.Grow(n)
	b.WriteString(string(elems[0]))
	for _, s := range elems[1:] {
		b.WriteString(sep)
		b.WriteString(string(s))
	}
	return b.String()
}
