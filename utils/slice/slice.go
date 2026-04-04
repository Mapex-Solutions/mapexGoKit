// Package slice provides generic utility functions for slice manipulation.
package slice

// Reverse reverses a slice in place.
// This function modifies the original slice by swapping elements from both ends
// moving towards the center.
//
// Example:
//
//	s := []int{1, 2, 3, 4, 5}
//	slice.Reverse(s)
//	// s is now []int{5, 4, 3, 2, 1}
func Reverse[T any](s []T) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}
