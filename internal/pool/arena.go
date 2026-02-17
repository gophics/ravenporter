package pool

// Arena is a flat slice arena that hands out sub-slices from a single
// backing array. After the initial grow, subsequent Alloc calls are zero-alloc.
type Arena[T any] struct {
	buf []T
}

// NewArena creates an arena pre-sized to hold cap elements.
func NewArena[T any](initCap int) Arena[T] {
	return Arena[T]{buf: make([]T, 0, initCap)}
}

// Alloc returns a zeroed sub-slice of length n from the arena.
func (a *Arena[T]) Alloc(n int) []T {
	start := len(a.buf)
	need := start + n
	if need > cap(a.buf) {
		grown := make([]T, start, need*2) //nolint:mnd // standard doubling
		copy(grown, a.buf)
		a.buf = grown
	}
	a.buf = a.buf[:need]
	return a.buf[start:need:need]
}
