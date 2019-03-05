package pio

import "time"

func PutU8(b []byte, v uint8) {
	b[0] = v
}

func PutI16BE(b []byte, v int16) {
	b[0] = byte(v >> 8)
	b[1] = byte(v)
}

func PutU16BE(b []byte, v uint16) {
	b[0] = byte(v >> 8)
	b[1] = byte(v)
}

func PutI24BE(b []byte, v int32) {
	b[0] = byte(v >> 16)
	b[1] = byte(v >> 8)
	b[2] = byte(v)
}

func PutU24BE(b []byte, v uint32) {
	b[0] = byte(v >> 16)
	b[1] = byte(v >> 8)
	b[2] = byte(v)
}

func PutI32BE(b []byte, v int32) {
	b[0] = byte(v >> 24)
	b[1] = byte(v >> 16)
	b[2] = byte(v >> 8)
	b[3] = byte(v)
}

func PutU32BE(b []byte, v uint32) {
	b[0] = byte(v >> 24)
	b[1] = byte(v >> 16)
	b[2] = byte(v >> 8)
	b[3] = byte(v)
}

func PutU32LE(b []byte, v uint32) {
	b[3] = byte(v >> 24)
	b[2] = byte(v >> 16)
	b[1] = byte(v >> 8)
	b[0] = byte(v)
}

func PutU40BE(b []byte, v uint64) {
	b[0] = byte(v >> 32)
	b[1] = byte(v >> 24)
	b[2] = byte(v >> 16)
	b[3] = byte(v >> 8)
	b[4] = byte(v)
}

func PutU48BE(b []byte, v uint64) {
	b[0] = byte(v >> 40)
	b[1] = byte(v >> 32)
	b[2] = byte(v >> 24)
	b[3] = byte(v >> 16)
	b[4] = byte(v >> 8)
	b[5] = byte(v)
}

func PutU64BE(b []byte, v uint64) {
	b[0] = byte(v >> 56)
	b[1] = byte(v >> 48)
	b[2] = byte(v >> 40)
	b[3] = byte(v >> 32)
	b[4] = byte(v >> 24)
	b[5] = byte(v >> 16)
	b[6] = byte(v >> 8)
	b[7] = byte(v)
}

func PutI64BE(b []byte, v int64) {
	b[0] = byte(v >> 56)
	b[1] = byte(v >> 48)
	b[2] = byte(v >> 40)
	b[3] = byte(v >> 32)
	b[4] = byte(v >> 24)
	b[5] = byte(v >> 16)
	b[6] = byte(v >> 8)
	b[7] = byte(v)
}

func PutTime64(b []byte, t time.Time) {
	var v int64
	if !t.IsZero() {
		v = t.UnixNano()
	}
	PutI64BE(b, v)
}

func WriteU8(b []byte, n *int, v uint8) {
	if b != nil {
		b[*n] = v
	}
	*n += 1
	return
}

func WriteU16BE(b []byte, n *int, v uint16) {
	if b != nil {
		PutU16BE(b[*n:], v)
	}
	*n += 2
	return
}

func WriteU24BE(b []byte, n *int, v uint32) {
	if b != nil {
		PutU24BE(b[*n:], v)
	}
	*n += 3
	return
}

func WriteI24BE(b []byte, n *int, v int32) {
	if b != nil {
		PutI24BE(b[*n:], v)
	}
	*n += 3
	return
}

func WriteU32BE(b []byte, n *int, v uint32) {
	if b != nil {
		PutU32BE(b[*n:], v)
	}
	*n += 4
	return
}

func WriteI32BE(b []byte, n *int, v int32) {
	if b != nil {
		PutI32BE(b[*n:], v)
	}
	*n += 4
	return
}

func WriteU32LE(b []byte, n *int, v uint32) {
	if b != nil {
		PutU32LE(b[*n:], v)
	}
	*n += 4
	return
}

func WriteU64BE(b []byte, n *int, v uint64) {
	if b != nil {
		PutU64BE(b[*n:], v)
	}
	*n += 8
	return
}

func WriteI64BE(b []byte, n *int, v int64) {
	if b != nil {
		PutI64BE(b[*n:], v)
	}
	*n += 8
	return
}

func WriteString(b []byte, n *int, v string) {
	if b != nil {
		copy(b[*n:], v)
	}
	*n += len(v)
	return
}
func WriteBytes(b []byte, n *int, v []byte) {
	if b != nil {
		copy(b[*n:], v)
	}
	*n += len(v)
	return
}
