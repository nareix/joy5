package pio

import (
	"testing"

	"github.com/googollee/go-assert"
)

func TestVecSliceTo(t *testing.T) {
	in := make([][]byte, 5)
	for k := range in {
		in[k] = make([]byte, 30)
	}
	out := make([][]byte, 5)

	n := VecSliceTo(in, nil, 100, 100)
	assert.Equal(t, n, 0)

	n = VecSliceTo(in, out, 100, 120)
	assert.Equal(t, n, 1)

	n = VecSliceTo(in, out, 100, 121)
	assert.Equal(t, n, 2)
}
