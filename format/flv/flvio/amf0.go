package flvio

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/nareix/joy5/utils/bits/pio"
)

type AMFParseError struct {
	Offset  int
	Message string
	Next    *AMFParseError
	Bytes   []byte
}

func (e *AMFParseError) Error() string {
	if e.Bytes != nil {
		s := []string{}
		for p := e; p != nil; p = p.Next {
			s = append(s, fmt.Sprintf("%s", p.Message))
		}
		return fmt.Sprintf("AMFParseError(%d)", e.Offset) + strings.Join(s, ",") + fmt.Sprintf("Bytes(%x)", e.Bytes)
	}
	return fmt.Sprintf("AMFParseError(%d)", e.Offset)
}

func amfParseErr(message string, b []byte, offset int, err error) error {
	next, _ := err.(*AMFParseError)
	return &AMFParseError{
		Offset:  offset,
		Message: message,
		Next:    next,
	}
}

type AMFKv struct {
	K string
	V interface{}
}
type AMFMap []AMFKv

func (a AMFMap) Get(k string) *AMFKv {
	for i := range a {
		kv := &a[i]
		if kv.K == k {
			return kv
		}
	}
	return nil
}

func (a AMFMap) GetString(k string) (string, bool) {
	v, ok := a.GetV(k)
	if !ok {
		return "", false
	}
	s, typeok := v.(string)
	return s, typeok
}

func (a AMFMap) GetBool(k string) (bool, bool) {
	v, ok := a.GetV(k)
	if !ok {
		return false, false
	}
	b, typeok := v.(bool)
	return b, typeok
}

func (a AMFMap) GetFloat64(k string) (float64, bool) {
	v, ok := a.GetV(k)
	if !ok {
		return 0, false
	}
	f, typeok := v.(float64)
	return f, typeok
}

func (a AMFMap) GetV(k string) (interface{}, bool) {
	kv := a.Get(k)
	if kv == nil {
		return nil, false
	}
	return kv.V, true
}

func (a AMFMap) Del(dk string) AMFMap {
	nm := AMFMap{}
	for _, kv := range a {
		if kv.K != dk {
			nm = append(nm, kv)
		}
	}
	return nm
}

func (a AMFMap) Set(k string, v interface{}) AMFMap {
	kv := a.Get(k)
	if kv == nil {
		return append(a, AMFKv{k, v})
	}
	kv.V = v
	return a
}

func (a AMFMap) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer

	buf.WriteString("{")
	for i, kv := range a {
		if i != 0 {
			buf.WriteString(",")
		}
		key, err := json.Marshal(kv.K)
		if err != nil {
			return nil, err
		}
		buf.Write(key)
		buf.WriteString(":")
		val, err := json.Marshal(kv.V)
		if err != nil {
			return nil, err
		}
		buf.Write(val)
	}

	buf.WriteString("}")
	return buf.Bytes(), nil
}

type AMFArray []interface{}
type AMFECMAArray AMFMap

func readBEFloat64(b []byte, n *int) (f float64, err error) {
	var v uint64
	if v, err = pio.ReadU64BE(b, n); err != nil {
		return
	}
	f = math.Float64frombits(v)
	return
}

func readTime64(b []byte, n *int) (t time.Time, err error) {
	var ts float64
	if ts, err = readBEFloat64(b, n); err != nil {
		return
	}
	t = time.Unix(int64(ts/1000), (int64(ts)%1000)*1000000)
	return
}

func fillBEFloat64(b []byte, n *int, f float64) {
	pio.WriteU64BE(b, n, math.Float64bits(f))
}

func fillAMF0Number(b []byte, n *int, f float64) {
	pio.WriteU8(b, n, numbermarker)
	fillBEFloat64(b, n, f)
}

const (
	numbermarker = iota
	booleanmarker
	stringmarker
	objectmarker
	movieclipmarker
	nullmarker
	undefinedmarker
	referencemarker
	ecmaarraymarker
	objectendmarker
	strictarraymarker
	datemarker
	longstringmarker
	unsupportedmarker
	recordsetmarker
	xmldocumentmarker
	typedobjectmarker
	avmplusobjectmarker
)

func FillAMF0ValMalloc(v interface{}) (b []byte) {
	vals := []interface{}{v}
	n := FillAMF0Vals(nil, vals)
	b = make([]byte, n)
	FillAMF0Vals(b, vals)
	return
}

func FillAMF0ValsMalloc(vals []interface{}) (b []byte) {
	n := FillAMF0Vals(nil, vals)
	b = make([]byte, n)
	FillAMF0Vals(b, vals)
	return
}

func FillAMF0Vals(b []byte, vals []interface{}) (n int) {
	for _, v := range vals {
		if _b, ok := v.([]byte); ok {
			pio.WriteBytes(b, &n, _b)
		} else {
			FillAMF0Val(b, &n, v)
		}
	}
	return
}

type orderpair struct {
	k string
	v interface{}
}

type orderpairs []orderpair

func (o orderpairs) Len() int {
	return len(o)
}

func (o orderpairs) Swap(i, j int) {
	o[i], o[j] = o[j], o[i]
}

func (o orderpairs) Less(i, j int) bool {
	return o[i].k < o[j].k
}

func ordermap(m map[string]interface{}) (p orderpairs) {
	for k, v := range m {
		p = append(p, orderpair{k, v})
	}
	sort.Sort(p)
	return
}

func FillAMF0Val(b []byte, n *int, _val interface{}) {
	switch val := _val.(type) {
	case int8:
		fillAMF0Number(b, n, float64(val))
	case int16:
		fillAMF0Number(b, n, float64(val))
	case int32:
		fillAMF0Number(b, n, float64(val))
	case int64:
		fillAMF0Number(b, n, float64(val))
	case int:
		fillAMF0Number(b, n, float64(val))
	case uint8:
		fillAMF0Number(b, n, float64(val))
	case uint16:
		fillAMF0Number(b, n, float64(val))
	case uint32:
		fillAMF0Number(b, n, float64(val))
	case uint64:
		fillAMF0Number(b, n, float64(val))
	case uint:
		fillAMF0Number(b, n, float64(val))
	case float32:
		fillAMF0Number(b, n, float64(val))
	case float64:
		fillAMF0Number(b, n, float64(val))

	case string:
		u := len(val)
		if u < 65536 {
			pio.WriteU8(b, n, stringmarker)
			pio.WriteU16BE(b, n, uint16(u))
		} else {
			pio.WriteU8(b, n, longstringmarker)
			pio.WriteU32BE(b, n, uint32(u))
		}
		pio.WriteString(b, n, val)

	case AMFECMAArray:
		pio.WriteU8(b, n, ecmaarraymarker)
		pio.WriteU32BE(b, n, uint32(len(val)))
		for _, p := range val {
			pio.WriteString(b, n, p.K)
			FillAMF0Val(b, n, p.V)
		}
		pio.WriteU24BE(b, n, 0x000009)

	case AMFMap:
		pio.WriteU8(b, n, objectmarker)
		for _, p := range val {
			if len(p.K) > 0 {
				pio.WriteU16BE(b, n, uint16(len(p.K)))
				pio.WriteString(b, n, p.K)
				FillAMF0Val(b, n, p.V)
			}
		}
		pio.WriteU24BE(b, n, 0x000009)

	case AMFArray:
		pio.WriteU8(b, n, strictarraymarker)
		pio.WriteU32BE(b, n, uint32(len(val)))
		for _, v := range val {
			FillAMF0Val(b, n, v)
		}

	case time.Time:
		pio.WriteU8(b, n, datemarker)
		u := val.UnixNano()
		f := float64(u / 1000000)
		fillBEFloat64(b, n, f)
		pio.WriteU16BE(b, n, uint16(0))

	case bool:
		pio.WriteU8(b, n, booleanmarker)
		var u uint8
		if val {
			u = 1
		} else {
			u = 0
		}
		pio.WriteU8(b, n, u)

	case nil:
		pio.WriteU8(b, n, nullmarker)
	}

	return
}

func ParseAMF0Val(b []byte, n *int) (val interface{}, err error) {
	val, err = parseAMF0Val(0, b, n)
	return
}

func ParseAMFVals(b []byte, isamf3 bool) (arr []interface{}, err error) {
	var n int
	parse := ParseAMF0Val

	if isamf3 {
		if len(b) < 1 {
			err = amfParseErr("amf3.marker", b, n, nil)
			return
		}
		if b[0] == 0 {
			n++
		} else {
			parse = ParseAMF3Val
		}
	}

	for n < len(b) {
		var v interface{}
		if v, err = parse(b, &n); err != nil {
			return
		}
		arr = append(arr, v)
	}

	return
}

func parseAMF0Val(depth int, b []byte, n *int) (val interface{}, err error) {
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
	case numbermarker:
		if val, err = readBEFloat64(b, n); err != nil {
			err = amfParseErr("number", b, *n, err)
			return
		}

	case booleanmarker:
		var v uint8
		if v, err = pio.ReadU8(b, n); err != nil {
			err = amfParseErr("boolean", b, *n, err)
			return
		}
		val = (v != 0)

	case stringmarker:
		var length uint16
		if length, err = pio.ReadU16BE(b, n); err != nil {
			err = amfParseErr("string.length", b, *n, err)
			return
		}
		if val, err = pio.ReadString(b, n, int(length)); err != nil {
			err = amfParseErr("string.body", b, *n, err)
			return
		}

	case objectmarker:
		obj := AMFMap{}
		for {
			var length uint16
			if length, err = pio.ReadU16BE(b, n); err != nil {
				err = amfParseErr("object.key.length", b, *n, err)
				return
			}
			if length == 0 {
				break
			}

			var okey string
			if okey, err = pio.ReadString(b, n, int(length)); err != nil {
				err = amfParseErr("object.key.body", b, *n, err)
				return
			}

			if debug {
				fmt.Println(depth, *n, "object.key", okey)
			}

			var oval interface{}
			if oval, err = parseAMF0Val(depth+1, b, n); err != nil {
				err = amfParseErr("object.val", b, *n, err)
				return
			}

			obj = obj.Set(okey, oval)
		}
		if _, err = pio.ReadU8(b, n); err != nil {
			err = amfParseErr("object.end", b, *n, err)
			return
		}
		val = obj

	case nullmarker:
	case undefinedmarker:

	case ecmaarraymarker:
		if _, err = pio.ReadU32BE(b, n); err != nil {
			err = amfParseErr("array.count", b, *n, err)
			return
		}

		obj := AMFMap{}
		for {
			var length uint16
			if length, err = pio.ReadU16BE(b, n); err != nil {
				err = amfParseErr("array.key.length", b, *n, err)
				return
			}
			if length == 0 {
				break
			}

			var okey string
			if okey, err = pio.ReadString(b, n, int(length)); err != nil {
				err = amfParseErr("array.key.body", b, *n, err)
				return
			}

			var oval interface{}
			if oval, err = parseAMF0Val(depth+1, b, n); err != nil {
				err = amfParseErr("array.val", b, *n, err)
				return
			}

			obj = obj.Set(okey, oval)
		}
		if _, err = pio.ReadU8(b, n); err != nil {
			err = amfParseErr("array.end", b, *n, err)
			return
		}
		val = obj

	case objectendmarker:
		if _, err = pio.ReadU24BE(b, n); err != nil {
			err = amfParseErr("objectend", b, *n, err)
			return
		}

	case strictarraymarker:
		var count uint32
		if count, err = pio.ReadU32BE(b, n); err != nil {
			err = amfParseErr("strictarray.count", b, *n, err)
			return
		}
		if count > uint32(len(b)) {
			err = amfParseErr("strictarray.count.toobig", b, *n, err)
			return
		}
		obj := make(AMFArray, count)
		for i := 0; i < int(count); i++ {
			if obj[i], err = parseAMF0Val(depth+1, b, n); err != nil {
				err = amfParseErr("strictarray.val", b, *n, err)
				return
			}
		}
		val = obj

	case datemarker:
		var t time.Time
		if t, err = readTime64(b, n); err != nil {
			err = amfParseErr("date", b, *n, err)
			return
		}
		if _, err = pio.ReadU16BE(b, n); err != nil {
			err = amfParseErr("date.end", b, *n, err)
			return
		}
		val = t

	case longstringmarker:
		var length uint32
		if length, err = pio.ReadU32BE(b, n); err != nil {
			err = amfParseErr("longstring.length", b, *n, err)
			return
		}
		if length > uint32(len(b)) {
			err = amfParseErr("longstring.length.toobig", b, *n, err)
			return
		}
		if val, err = pio.ReadString(b, n, int(length)); err != nil {
			err = amfParseErr("longstring.body", b, *n, err)
			return
		}

	default:
		err = amfParseErr(fmt.Sprintf("invalidmarker=%d", marker), b, *n, err)
		return
	}

	return
}
