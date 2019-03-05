package flvio

import (
	"fmt"
	"time"

	"github.com/nareix/joy5/utils/bits/pio"
)

const (
	amf3undefinedmarker = iota
	amf3nullmarker
	amf3falsemarker
	amf3truemarker
	amf3integermarker
	amf3doublemarker
	amf3stringmarker
	amf3xmldocmarker
	amf3datemarker
	amf3arraymarker
	amf3objectmarker
	amf3xmlmarker
	amf3bytearraymarker
	amf3vectorintmarker
	amf3vectoruintmarker
	amf3vectordoublemarker
	amf3vectorobjectmarker
	amf3dictionarymarker
)

func ParseAMF3Val(b []byte, n *int) (val interface{}, err error) {
	val, err = parseAMF3Val(0, b, n)
	return
}

func parseAMF3Val(depth int, b []byte, n *int) (val interface{}, err error) {
	const debug = false
	var marker uint8

	if marker, err = pio.ReadU8(b, n); err != nil {
		err = amfParseErr("marker", b, *n, err)
		return
	}
	if debug {
		fmt.Println(depth, *n, "marker", marker)
	}

	switch marker {
	case amf3undefinedmarker, amf3nullmarker:
		val = nil

	case amf3falsemarker:
		if _, err = pio.ReadU8(b, n); err != nil {
			err = amfParseErr("boolean:false", b, *n, err)
			return
		}
		val = false

	case amf3truemarker:
		if _, err = pio.ReadU8(b, n); err != nil {
			err = amfParseErr("boolean:true", b, *n, err)
			return
		}
		val = true

	case amf3integermarker:
		val, err = readU29(b, n)
		if err != nil {
			err = amfParseErr("integer", b, *n, err)
			return
		}
		val = (val.(int)) << 3
		val = (val.(int)) >> 3

	case amf3doublemarker:
		if val, err = readBEFloat64(b, n); err != nil {
			err = amfParseErr("double", b, *n, err)
			return
		}

	case amf3stringmarker:
		val, err = readString(b, n)
		if err != nil {
			err = amfParseErr("string", b, *n, err)
			return
		}

	case amf3xmldocmarker:
		val, err = readString(b, n)
		if err != nil {
			amfParseErr("xmldoc", b, *n, err)
			return
		}

	case amf3datemarker:
		val, err = readDate(b, n)
		if err != nil {
			err = amfParseErr("date", b, *n, err)
			return
		}

	case amf3arraymarker:
		val, err = readArray(depth, b, n)
		if err != nil {
			err = amfParseErr("array", b, *n, err)
			return
		}

	case amf3objectmarker:
		val, err = readObject(depth, b, n)
		if err != nil {
			err = amfParseErr("object", b, *n, err)
			return
		}

	case amf3xmlmarker:
		val, err = readString(b, n)
		if err != nil {
			err = amfParseErr("xml", b, *n, err)
			return
		}

	case amf3bytearraymarker:
		val, err = readByteArray(b, n)
		if err != nil {
			err = amfParseErr("bytearray", b, *n, err)
			return
		}

	case amf3vectorintmarker, amf3vectoruintmarker, amf3vectordoublemarker, amf3vectorobjectmarker, amf3dictionarymarker:
		err = amfParseErr("support marker", b, *n, err)
		return

	default:
		err = amfParseErr("unknown marker", b, *n, err)
		return
	}

	return
}

func readU29(b []byte, n *int) (val int, err error) {
	for i := 0; i < 4; i++ {
		var v uint8
		if v, err = pio.ReadU8(b, n); err != nil {
			break
		}
		if i == 3 {
			val = val<<8 | int(v)
			return
		}
		val = val<<7 | int(v&0x7f)
		if v&0x80 == 0 {
			return
		}
	}
	err = amfParseErr("u29", b, *n, err)
	return
}

func readString(b []byte, n *int) (val string, err error) {
	l, err := readU29(b, n)
	if err != nil {
		err = amfParseErr("string.u29", b, *n, err)
		return
	}

	if l&1 == 0 {
		err = amfParseErr("not support reference", b, *n, err)
		return
	}
	l = l >> 1

	if val, err = pio.ReadString(b, n, int(l)); err != nil {
		err = amfParseErr("string.body", b, *n, err)
		return
	}
	return
}

func readDate(b []byte, n *int) (val time.Time, err error) {
	l, err := readU29(b, n)
	if err != nil {
		err = amfParseErr("date.u29", b, *n, err)
		return
	}

	if l&1 == 0 {
		err = amfParseErr("not support reference", b, *n, err)
		return
	}

	t, err := readBEFloat64(b, n)
	ts := int64(t)
	val = time.Unix(ts/1000, (ts%1000)*1000000)
	return
}

func readArray(depth int, b []byte, n *int) (val AMFMap, err error) {
	l, err := readU29(b, n)
	if err != nil {
		err = amfParseErr("array.u29", b, *n, err)
		return
	}

	if l&1 == 0 {
		err = amfParseErr("not support reference", b, *n, err)
		return
	}

	l = l >> 1
	var k string
	val = AMFMap{}
	for {
		k, err = readString(b, n)
		if err != nil {
			err = amfParseErr("array.string", b, *n, err)
			return
		}
		if k == "" {
			break
		}
		var v interface{}
		if v, err = parseAMF3Val(depth, b, n); err != nil {
			err = amfParseErr("array.val", b, *n, err)
			return
		}
		val = val.Set(k, v)
	}
	if l == 0 {
		return
	}
	return
}

func readObject(depth int, b []byte, n *int) (val AMFMap, err error) {
	l, err := readU29(b, n)
	if err != nil {
		err = amfParseErr("object.u29", b, *n, err)
		return
	}
	if l&1 == 0 {
		err = amfParseErr("not support reference", b, *n, err)
		return
	}
	l = l >> 1
	if l&1 == 0 {
		err = amfParseErr("not support traits reference", b, *n, err)
		return
	}
	l = l >> 1
	if l&1 == 1 {
		err = amfParseErr("not support traits ext", b, *n, err)
		return
	}
	l = l >> 1
	isDynamic := (l&1 == 1)
	l = l >> 1

	_, err = readString(b, n)
	if err != nil {
		err = amfParseErr("object.string", b, *n, err)
		return
	}

	val = AMFMap{}

	var k string
	if isDynamic {
		for {
			k, err = readString(b, n)
			if err != nil {
				err = amfParseErr("object.string", b, *n, err)
				return
			}
			if k == "" {
				break
			}
			var v interface{}
			if v, err = parseAMF3Val(depth, b, n); err != nil {
				err = amfParseErr("array.val", b, *n, err)
				return
			}
			val = val.Set(k, v)
		}
	} else {
		keys := make([]string, l)
		for i := 0; i < l; i++ {
			keys[i], err = readString(b, n)
			if err != nil {
				return
			}
		}
		for i := 0; i < l; i++ {
			var v interface{}
			if v, err = parseAMF3Val(depth, b, n); err != nil {
				return
			}
			val = val.Set(keys[i], v)
		}
	}
	return
}

func readByteArray(b []byte, n *int) (val []byte, err error) {
	l, err := readU29(b, n)
	if err != nil {
		err = amfParseErr("int", b, *n, err)
		return
	}

	if l&1 == 0 {
		err = amfParseErr("not support reference", b, *n, err)
		return
	}
	l = l >> 1

	if val, err = pio.ReadBytes(b, n, l); err != nil {
		err = amfParseErr("bytearray", b, *n, err)
		return
	}
	return
}
