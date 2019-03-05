package pio

import (
	"fmt"
	"time"
)

type Error struct {
	N int
}

func (self Error) Error() string {
	return fmt.Sprintf("PIOFailed(%d)", self.N)
}

func U8(b []byte) (i uint8) {
	return b[0]
}

func U16BE(b []byte) (i uint16) {
	i = uint16(b[0])
	i <<= 8
	i |= uint16(b[1])
	return
}

func I16BE(b []byte) (i int16) {
	i = int16(b[0])
	i <<= 8
	i |= int16(b[1])
	return
}

func I24BE(b []byte) (i int32) {
	i = int32(int8(b[0]))
	i <<= 8
	i |= int32(b[1])
	i <<= 8
	i |= int32(b[2])
	return
}

func U24BE(b []byte) (i uint32) {
	i = uint32(b[0])
	i <<= 8
	i |= uint32(b[1])
	i <<= 8
	i |= uint32(b[2])
	return
}

func I32BE(b []byte) (i int32) {
	i = int32(int8(b[0]))
	i <<= 8
	i |= int32(b[1])
	i <<= 8
	i |= int32(b[2])
	i <<= 8
	i |= int32(b[3])
	return
}

func U32LE(b []byte) (i uint32) {
	i = uint32(b[3])
	i <<= 8
	i |= uint32(b[2])
	i <<= 8
	i |= uint32(b[1])
	i <<= 8
	i |= uint32(b[0])
	return
}

func U32BE(b []byte) (i uint32) {
	i = uint32(b[0])
	i <<= 8
	i |= uint32(b[1])
	i <<= 8
	i |= uint32(b[2])
	i <<= 8
	i |= uint32(b[3])
	return
}

func U40BE(b []byte) (i uint64) {
	i = uint64(b[0])
	i <<= 8
	i |= uint64(b[1])
	i <<= 8
	i |= uint64(b[2])
	i <<= 8
	i |= uint64(b[3])
	i <<= 8
	i |= uint64(b[4])
	return
}

func U48BE(b []byte) (i uint64) {
	i = uint64(b[0])
	i <<= 8
	i |= uint64(b[1])
	i <<= 8
	i |= uint64(b[2])
	i <<= 8
	i |= uint64(b[3])
	i <<= 8
	i |= uint64(b[4])
	i <<= 8
	i |= uint64(b[5])
	return
}

func U64BE(b []byte) (i uint64) {
	i = uint64(b[0])
	i <<= 8
	i |= uint64(b[1])
	i <<= 8
	i |= uint64(b[2])
	i <<= 8
	i |= uint64(b[3])
	i <<= 8
	i |= uint64(b[4])
	i <<= 8
	i |= uint64(b[5])
	i <<= 8
	i |= uint64(b[6])
	i <<= 8
	i |= uint64(b[7])
	return
}

func I64BE(b []byte) (i int64) {
	i = int64(int8(b[0]))
	i <<= 8
	i |= int64(b[1])
	i <<= 8
	i |= int64(b[2])
	i <<= 8
	i |= int64(b[3])
	i <<= 8
	i |= int64(b[4])
	i <<= 8
	i |= int64(b[5])
	i <<= 8
	i |= int64(b[6])
	i <<= 8
	i |= int64(b[7])
	return
}

func Time64(b []byte) time.Time {
	v := I64BE(b[0:8])
	if v == 0 {
		return time.Time{}
	}
	const e9 = int64(1e9)
	return time.Unix(v/e9, v%e9)
}

func ReadU8(b []byte, n *int) (v uint8, err error) {
	if len(b) < *n+1 {
		err = Error{N: *n}
		return
	}
	if b != nil {
		v = b[*n]
	}
	*n += 1
	return
}

func ReadU16BE(b []byte, n *int) (v uint16, err error) {
	if len(b) < *n+2 {
		err = Error{N: *n}
		return
	}
	if b != nil {
		v = U16BE(b[*n:])
	}
	*n += 2
	return
}

func ReadI24BE(b []byte, n *int) (v int32, err error) {
	if len(b) < *n+3 {
		err = Error{N: *n}
		return
	}
	if b != nil {
		v = I24BE(b[*n:])
	}
	*n += 3
	return
}

func ReadU24BE(b []byte, n *int) (v uint32, err error) {
	if len(b) < *n+3 {
		err = Error{N: *n}
		return
	}
	if b != nil {
		v = U24BE(b[*n:])
	}
	*n += 3
	return
}

func ReadU32BE(b []byte, n *int) (v uint32, err error) {
	if len(b) < *n+4 {
		err = Error{N: *n}
		return
	}
	if b != nil {
		v = U32BE(b[*n:])
	}
	*n += 4
	return
}

func ReadI32BE(b []byte, n *int) (v int32, err error) {
	if len(b) < *n+4 {
		err = Error{N: *n}
		return
	}
	if b != nil {
		v = I32BE(b[*n:])
	}
	*n += 4
	return
}

func ReadU64BE(b []byte, n *int) (v uint64, err error) {
	if len(b) < *n+8 {
		err = Error{N: *n}
		return
	}
	if b != nil {
		v = U64BE(b[*n:])
	}
	*n += 8
	return
}

func ReadI64BE(b []byte, n *int) (v int64, err error) {
	if len(b) < *n+8 {
		err = Error{N: *n}
		return
	}
	if b != nil {
		v = I64BE(b[*n:])
	}
	*n += 8
	return
}

func ReadBytes(b []byte, n *int, length int) (v []byte, err error) {
	if len(b) < *n+length {
		err = Error{N: *n}
		return
	}
	if b != nil {
		v = b[*n : *n+length]
	}
	*n += length
	return
}

func ReadString(b []byte, n *int, strlen int) (v string, err error) {
	if len(b) < *n+strlen {
		err = Error{N: *n}
		return
	}
	if b != nil {
		v = string(b[*n : *n+strlen])
	}
	*n += strlen
	return
}
