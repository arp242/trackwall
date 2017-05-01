package tt

import (
	"testing"
)

func TestEq(t *testing.T) {
	isTests = true

	Eq(t, "x", 1, 1)
	Eq(t, "x", ` `, " ")

	Panic(t, func() { Eq(t, "x", 1, "1") })
	Panic(t, func() { Eq(t, "x", "", []rune{}) })
	Panic(t, func() { Eq(t, "x", "a", 'a') })
}
