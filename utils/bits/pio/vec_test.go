package pio

import (
	"testing"
)

func TestVecSliceTo(t *testing.T) {
	in := make([][]byte, 5)
	for k := range in {
		in[k] = make([]byte, 30)
	}
	out := make([][]byte, 5)

	n := VecSliceTo(in, nil, 100, 100)
	if n != 0 {
		t.Fail()
	}

	n = VecSliceTo(in, out, 100, 120)
	if n != 1 {
		t.Fail()
	}

	n = VecSliceTo(in, out, 100, 121)
	if n != 2 {
		t.Fail()
	}
}
