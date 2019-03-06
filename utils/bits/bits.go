package bits

import (
	"fmt"
	"io"
)

type Reader struct {
	R    io.Reader
	n    int
	bits uint64
}

func (r *Reader) ReadBits64(n int) (bits uint64, err error) {
	if r.n < n {
		var b [8]byte
		var got int
		want := (n - r.n + 7) / 8
		if got, err = r.R.Read(b[:want]); err != nil {
			return
		}
		if got < want {
			err = fmt.Errorf("bits: EOF")
			return
		}
		for i := 0; i < got; i++ {
			r.bits <<= 8
			r.bits |= uint64(b[i])
		}
		r.n += got * 8
	}
	bits = r.bits >> uint(r.n-n)
	r.bits ^= bits << uint(r.n-n)
	r.n -= n
	return
}

func (r *Reader) ReadBits(n int) (bits uint, err error) {
	var bits64 uint64
	if bits64, err = r.ReadBits64(n); err != nil {
		return
	}
	bits = uint(bits64)
	return
}

func (r *Reader) Read(p []byte) (n int, err error) {
	for n < len(p) {
		want := 8
		if len(p)-n < want {
			want = len(p) - n
		}
		var bits uint64
		if bits, err = r.ReadBits64(want * 8); err != nil {
			break
		}
		for i := 0; i < want; i++ {
			p[n+i] = byte(bits >> uint((want-i-1)*8))
		}
		n += want
	}
	return
}

type Writer struct {
	W    io.Writer
	n    int
	bits uint64
}

func (w *Writer) WriteBits64(bits uint64, n int) (err error) {
	if w.n+n > 64 {
		move := uint(64 - w.n)
		mask := bits >> move
		w.bits = (w.bits << move) | mask
		w.n = 64
		if err = w.FlushBits(); err != nil {
			return
		}
		n -= int(move)
		bits ^= (mask << move)
	}
	w.bits = (w.bits << uint(n)) | bits
	w.n += n
	return
}

func (w *Writer) WriteBits(bits uint, n int) (err error) {
	return w.WriteBits64(uint64(bits), n)
}

func (w *Writer) Write(p []byte) (n int, err error) {
	for n < len(p) {
		if err = w.WriteBits64(uint64(p[n]), 8); err != nil {
			return
		}
		n++
	}
	return
}

func (w *Writer) FlushBits() (err error) {
	if w.n > 0 {
		var b [8]byte
		bits := w.bits
		if w.n%8 != 0 {
			bits <<= uint(8 - (w.n % 8))
		}
		want := (w.n + 7) / 8
		for i := 0; i < want; i++ {
			b[i] = byte(bits >> uint((want-i-1)*8))
		}
		if _, err = w.W.Write(b[:want]); err != nil {
			return
		}
		w.n = 0
	}
	return
}
