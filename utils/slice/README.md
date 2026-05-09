# slice — Generic slice utilities

Today: a single helper. The package is the home for any future generic slice operation.

> Package name: `slice` (directory: `slice/`).

## Surface

```go
func Reverse[T any](s []T)
```

In-place reversal — swaps `s[i]` with `s[len(s)-1-i]` until they meet. Mutates the caller's slice.

## Usage

```go
xs := []int{1, 2, 3, 4, 5}
slice.Reverse(xs)
// xs == [5 4 3 2 1]
```
