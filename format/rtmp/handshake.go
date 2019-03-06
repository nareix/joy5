package rtmp

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"

	"github.com/nareix/joy5/utils/bits/pio"
)

var (
	hsClientFullKey = []byte{
		'G', 'e', 'n', 'u', 'i', 'n', 'e', ' ', 'A', 'd', 'o', 'b', 'e', ' ',
		'F', 'l', 'a', 's', 'h', ' ', 'P', 'l', 'a', 'y', 'e', 'r', ' ',
		'0', '0', '1',
		0xF0, 0xEE, 0xC2, 0x4A, 0x80, 0x68, 0xBE, 0xE8, 0x2E, 0x00, 0xD0, 0xD1,
		0x02, 0x9E, 0x7E, 0x57, 0x6E, 0xEC, 0x5D, 0x2D, 0x29, 0x80, 0x6F, 0xAB,
		0x93, 0xB8, 0xE6, 0x36, 0xCF, 0xEB, 0x31, 0xAE,
	}
	hsServerFullKey = []byte{
		'G', 'e', 'n', 'u', 'i', 'n', 'e', ' ', 'A', 'd', 'o', 'b', 'e', ' ',
		'F', 'l', 'a', 's', 'h', ' ', 'M', 'e', 'd', 'i', 'a', ' ',
		'S', 'e', 'r', 'v', 'e', 'r', ' ',
		'0', '0', '1',
		0xF0, 0xEE, 0xC2, 0x4A, 0x80, 0x68, 0xBE, 0xE8, 0x2E, 0x00, 0xD0, 0xD1,
		0x02, 0x9E, 0x7E, 0x57, 0x6E, 0xEC, 0x5D, 0x2D, 0x29, 0x80, 0x6F, 0xAB,
		0x93, 0xB8, 0xE6, 0x36, 0xCF, 0xEB, 0x31, 0xAE,
	}
	hsClientPartialKey = hsClientFullKey[:30]
	hsServerPartialKey = hsServerFullKey[:36]
)

func hsMakeDigest(key []byte, src []byte, gap int) (dst []byte) {
	h := hmac.New(sha256.New, key)
	if gap <= 0 {
		h.Write(src)
	} else {
		h.Write(src[:gap])
		h.Write(src[gap+32:])
	}
	return h.Sum(nil)
}

func hsCalcDigestPos(p []byte, base int) (pos int) {
	for i := 0; i < 4; i++ {
		pos += int(p[base+i])
	}
	pos = (pos % 728) + base + 4
	return
}

func hsFindDigest(p []byte, key []byte, base int) int {
	gap := hsCalcDigestPos(p, base)
	digest := hsMakeDigest(key, p, gap)
	if bytes.Compare(p[gap:gap+32], digest) != 0 {
		return -1
	}
	return gap
}

func hsParse1(p []byte, peerkey []byte, key []byte) (ok bool, digest []byte) {
	var pos int
	if pos = hsFindDigest(p, peerkey, 772); pos == -1 {
		if pos = hsFindDigest(p, peerkey, 8); pos == -1 {
			return
		}
	}
	ok = true
	digest = hsMakeDigest(key, p[pos:pos+32], -1)
	return
}

func hsCreate01(p []byte, time uint32, ver uint32, key []byte) {
	p[0] = 3
	p1 := p[1:]
	rand.Read(p1[8:])
	pio.PutU32BE(p1[0:4], time)
	pio.PutU32BE(p1[4:8], ver)
	gap := hsCalcDigestPos(p1, 8)
	digest := hsMakeDigest(key, p1, gap)
	copy(p1[gap:], digest)
}

func hsCreate2(p []byte, key []byte) {
	rand.Read(p)
	gap := len(p) - 32
	digest := hsMakeDigest(key, p, gap)
	copy(p[gap:], digest)
}

// complex handshake for connecting fms
func (c *Conn) handshakeClient() (err error) {
	var C0C1 [1 + 1536]byte

	// C0 version
	C0C1[0] = 0x03

	// 4 bytes 0
	copy(C0C1[1:5], []byte{0x00, 0x00, 0x00, 0x00})

	// 4 bytes version 9 0 124 2
	copy(C0C1[5:9], []byte{0x09, 0x00, 0x7c, 0x02})

	rand.Read(C0C1[1+8:])
	gap := hsCalcDigestPos(C0C1[1:], 8)
	digest := hsMakeDigest(hsClientPartialKey, C0C1[1:], gap)
	copy(C0C1[gap+1:], digest)

	if _, err = c.wrapRW.rw.Write(C0C1[:]); err != nil {
		return
	}
	if err = c.flushWrite(); err != nil {
		return
	}

	var S0S1S2 [1536*2 + 1]byte
	var C2 [1536]byte
	if _, err = io.ReadFull(c.wrapRW.rw, S0S1S2[:]); err != nil {
		return
	}

	ok, digest2 := hsParse1(S0S1S2[1:1536+1], hsServerPartialKey, hsClientFullKey)
	if !ok {
		// fallback to simple handshake
		copy(C2[:], S0S1S2[1:1536+1])
	} else {
		hsCreate2(C2[:], digest2)
	}

	if _, err = c.wrapRW.rw.Write(C2[:]); err != nil {
		return
	}

	c.Stage = StageHandshakeDone
	return nil
}

func HsServerParseC1(C1 []byte) (bool, []byte) {
	return hsParse1(C1, hsClientPartialKey, hsServerFullKey)
}

func (c *Conn) handshakeServer() (err error) {
	var random [(1 + 1536*2) * 2]byte

	C0C1C2 := random[:1536*2+1]
	C0 := C0C1C2[:1]
	C1 := C0C1C2[1 : 1536+1]
	C2 := C0C1C2[1536+1:]

	S0S1S2 := random[1536*2+1:]
	S0 := S0S1S2[:1]
	S1 := S0S1S2[1 : 1536+1]
	S0S1 := S0S1S2[:1536+1]
	S2 := S0S1S2[1536+1:]

	// < C0
	if _, err = io.ReadFull(c.wrapRW.rw, C0); err != nil {
		return
	}

	if C0[0] != 3 {
		err = fmt.Errorf("VersionInvalid(%d)", C0[0])
		return
	}

	// < C1
	if _, err = io.ReadFull(c.wrapRW.rw, C1); err != nil {
		return
	}

	S0[0] = 3

	clitime := pio.U32BE(C1[0:4])
	srvtime := clitime
	srvver := uint32(0x0d0e0a0d)

	var ok bool
	var digest []byte
	if ok, digest = hsParse1(C1, hsClientPartialKey, hsServerFullKey); ok {
		hsCreate01(S0S1, srvtime, srvver, hsServerPartialKey)
		hsCreate2(S2, digest)
	} else {
		copy(S1, C2)
		copy(S2, C1)
	}

	// > S0S1S2
	if _, err = c.wrapRW.rw.Write(S0S1S2); err != nil {
		return
	}
	if err = c.flushWrite(); err != nil {
		return
	}

	// < C2
	if _, err = io.ReadFull(c.wrapRW.rw, C2); err != nil {
		return
	}

	c.Stage = StageHandshakeDone
	return
}
