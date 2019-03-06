package flvio

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/nareix/joy5/utils/bits/pio"
)

func TsToTime(ts int64) time.Duration {
	return time.Millisecond * time.Duration(ts)
}

func TimeToTs(tm time.Duration) int64 {
	return int64(tm / time.Millisecond)
}

const (
	TAG_AUDIO = 8
	TAG_VIDEO = 9
	TAG_AMF0  = 18
	TAG_AMF3  = 15
)

func TagTypeString(v uint8) string {
	switch v {
	case TAG_VIDEO:
		return "VIDEO"
	case TAG_AUDIO:
		return "AUDIO"
	case TAG_AMF0:
		return "AMF0"
	case TAG_AMF3:
		return "AMF3"
	}
	return fmt.Sprint(v)
}

func FrameTypeString(v uint8) string {
	switch v {
	case FRAME_INTER:
		return "INTER"
	case FRAME_KEY:
		return "KEY"
	}
	return fmt.Sprint(v)
}

const (
	SOUND_MP3                   = 2
	SOUND_NELLYMOSER_16KHZ_MONO = 4
	SOUND_NELLYMOSER_8KHZ_MONO  = 5
	SOUND_NELLYMOSER            = 6
	SOUND_ALAW                  = 7
	SOUND_MULAW                 = 8
	SOUND_AAC                   = 10
	SOUND_SPEEX                 = 11

	SOUND_5_5Khz = 0
	SOUND_11Khz  = 1
	SOUND_22Khz  = 2
	SOUND_44Khz  = 3

	SOUND_8BIT  = 0
	SOUND_16BIT = 1

	SOUND_MONO   = 0
	SOUND_STEREO = 1

	AAC_SEQHDR = 0
	AAC_RAW    = 1
)

const (
	AVC_SEQHDR = 0
	AVC_NALU   = 1
	AVC_EOS    = 2

	FRAME_KEY   = 1
	FRAME_INTER = 2

	VIDEO_H264 = 7
	VIDEO_H265 = 12
)

type Tag struct {
	Type uint8

	/*
		SoundFormat: UB[4]
		0 = Linear PCM, platform endian
		1 = ADPCM
		2 = MP3
		3 = Linear PCM, little endian
		4 = Nellymoser 16-kHz mono
		5 = Nellymoser 8-kHz mono
		6 = Nellymoser
		7 = G.711 A-law logarithmic PCM
		8 = G.711 mu-law logarithmic PCM
		9 = reserved
		10 = AAC
		11 = Speex
		14 = MP3 8-Khz
		15 = Device-specific sound
		Formats 7, 8, 14, and 15 are reserved for internal use
		AAC is supported in Flash Player 9,0,115,0 and higher.
		Speex is supported in Flash Player 10 and higher.
	*/
	SoundFormat uint8

	/*
		SoundRate: UB[2]
		Sampling rate
		0 = 5.5-kHz For AAC: always 3
		1 = 11-kHz
		2 = 22-kHz
		3 = 44-kHz
	*/
	SoundRate uint8

	/*
		SoundSize: UB[1]
		0 = snd8Bit
		1 = snd16Bit
		Size of each sample.
		This parameter only pertains to uncompressed formats.
		Compressed formats always decode to 16 bits internally
	*/
	SoundSize uint8

	/*
		SoundType: UB[1]
		0 = sndMono
		1 = sndStereo
		Mono or stereo sound For Nellymoser: always 0
		For AAC: always 1
	*/
	SoundType uint8

	/*
		0: AAC sequence header
		1: AAC raw
	*/
	AACPacketType uint8

	/*
		1: keyframe (for AVC, a seekable frame)
		2: inter frame (for AVC, a non- seekable frame)
		3: disposable inter frame (H.263 only)
		4: generated keyframe (reserved for server use only)
		5: video info/command frame
	*/
	FrameType uint8

	/*
		1: JPEG (currently unused)
		2: Sorenson H.263
		3: Screen video
		4: On2 VP6
		5: On2 VP6 with alpha channel
		6: Screen video version 2
		7: AVC
	*/
	VideoFormat uint8

	/*
		0: AVC sequence header
		1: AVC NALU
		2: AVC end of sequence (lower level NALU sequence ender is not required or supported)
	*/
	AVCPacketType uint8

	Time  uint32
	CTime int32

	StreamId uint32

	Header, Data []byte
}

func (t Tag) DebugFields() []interface{} {
	p := []interface{}{"Type", TagTypeString(t.Type), "Time", t.Time, "Len", len(t.Data)}

	switch t.Type {
	case TAG_VIDEO:
		p = append(p, "FrameType")
		p = append(p, FrameTypeString(t.FrameType))

		p = append(p, "VideoFormat")
		p = append(p, t.VideoFormat)

		switch t.VideoFormat {
		case VIDEO_H264, VIDEO_H265:
			p = append(p, "AVCPacketType")
			p = append(p, t.AVCPacketType)
		}

		if t.CTime != 0 {
			p = append(p, "Ctime")
			p = append(p, t.CTime)
		}

	case TAG_AMF0, TAG_AMF3:
		amf3 := t.Type == TAG_AMF3
		arr, _ := ParseAMFVals(t.Data, amf3)
		arrjs, _ := json.Marshal(arr)
		p = append(p, "Data")
		p = append(p, string(arrjs))
	}

	p = append(p, "Header")
	p = append(p, fmt.Sprintf("%x", t.Header))
	return p
}

func (t Tag) MaxHeaderLen() int {
	return 24
}

func (t *Tag) parseAudioHeader(b []byte) (n int, err error) {
	var flags uint8
	if flags, err = pio.ReadU8(b, &n); err != nil {
		return
	}
	t.SoundFormat = flags >> 4
	t.SoundRate = (flags >> 2) & 0x3
	t.SoundSize = (flags >> 1) & 0x1
	t.SoundType = flags & 0x1

	switch t.SoundFormat {
	case SOUND_AAC:
		if t.AACPacketType, err = pio.ReadU8(b, &n); err != nil {
			return
		}
	}

	return
}

func (t Tag) fillAudioHeader(b []byte) (n int) {
	var flags uint8
	flags |= t.SoundFormat << 4
	flags |= t.SoundRate << 2
	flags |= t.SoundSize << 1
	flags |= t.SoundType
	pio.WriteU8(b, &n, flags)

	switch t.SoundFormat {
	case SOUND_AAC:
		pio.WriteU8(b, &n, t.AACPacketType)
	}

	return
}

func (t *Tag) parseVideoHeader(b []byte) (n int, err error) {
	var flags uint8
	if flags, err = pio.ReadU8(b, &n); err != nil {
		return
	}
	t.FrameType = flags >> 4
	t.VideoFormat = flags & 0xf

	switch t.VideoFormat {
	case VIDEO_H264, VIDEO_H265:
		if t.AVCPacketType, err = pio.ReadU8(b, &n); err != nil {
			return
		}
		var v int32
		if v, err = pio.ReadI24BE(b, &n); err != nil {
			return
		}
		t.CTime = v
	}

	return
}

func (t Tag) fillVideoHeader(b []byte) (n int) {
	pio.WriteU8(b, &n, t.FrameType<<4|t.VideoFormat)

	switch t.VideoFormat {
	case VIDEO_H264, VIDEO_H265:
		pio.WriteU8(b, &n, t.AVCPacketType)
		pio.WriteI24BE(b, &n, int32(t.CTime))
	}
	return
}

func (t Tag) FillHeader(b []byte) (n int) {
	switch t.Type {
	case TAG_AUDIO:
		return t.fillAudioHeader(b)

	case TAG_VIDEO:
		return t.fillVideoHeader(b)
	}

	return
}

func (t *Tag) ParseHeader(b []byte) (n int, err error) {
	switch t.Type {
	case TAG_AUDIO:
		if n, err = t.parseAudioHeader(b); err != nil {
			return
		}

	case TAG_VIDEO:
		if n, err = t.parseVideoHeader(b); err != nil {
			return
		}
	}

	t.Header = b[:n]
	return
}

func (t *Tag) Parse(b []byte) (err error) {
	var n int
	if n, err = t.ParseHeader(b); err != nil {
		return
	}
	t.Data = b[n:]
	return
}

const (
	// TypeFlagsReserved UB[5]
	// TypeFlagsAudio    UB[1] Audio tags are present
	// TypeFlagsReserved UB[1] Must be 0
	// TypeFlagsVideo    UB[1] Video tags are present
	FILE_HAS_AUDIO = 0x4
	FILE_HAS_VIDEO = 0x1
)

func ParseTagHeader(b []byte) (tag Tag, datalen int, err error) {
	tagtype := b[0]
	tag = Tag{Type: tagtype}
	datalen = int(pio.U24BE(b[1:4]))

	var tslo uint32
	var tshi uint8
	tslo = pio.U24BE(b[4:7])
	tshi = b[7]

	tag.Time = tslo | uint32(tshi)<<24
	tag.StreamId = pio.U24BE(b[8:11])
	return
}

func ReadTag(r io.Reader, b []byte, malloc func(int) ([]byte, error)) (tag Tag, err error) {
	if _, err = io.ReadFull(r, b[:TagHeaderLength]); err != nil {
		return
	}
	var datalen int
	if tag, datalen, err = ParseTagHeader(b); err != nil {
		return
	}

	var data []byte
	if data, err = malloc(datalen); err != nil {
		return
	}
	if _, err = io.ReadFull(r, data); err != nil {
		return
	}

	if err = tag.Parse(data); err != nil {
		return
	}

	if _, err = io.ReadFull(r, b[:4]); err != nil {
		return
	}
	return
}

const TagHeaderLength = 11

func FillTagHeader(b []byte, tag Tag, datalen int) {
	b[0] = tag.Type
	pio.PutU24BE(b[1:4], uint32(datalen))
	pio.PutU24BE(b[4:7], uint32(tag.Time&0xffffff))
	b[7] = uint8(tag.Time >> 24)
	pio.PutU24BE(b[8:11], tag.StreamId)
}

const TagTrailerLength = 4

func FillTagTrailer(b []byte, datalen int) {
	pio.PutU32BE(b[0:4], uint32(datalen+TagHeaderLength))
}

func WriteTag(w io.Writer, tag Tag, b []byte) (err error) {
	data := tag.Data

	n := tag.FillHeader(b[TagHeaderLength:])
	datalen := len(data) + n

	FillTagHeader(b, tag, datalen)
	n += TagHeaderLength

	if _, err = w.Write(b[:n]); err != nil {
		return
	}

	if _, err = w.Write(data); err != nil {
		return
	}

	FillTagTrailer(b, datalen)
	if _, err = w.Write(b[:TagTrailerLength]); err != nil {
		return
	}

	return
}

const FileHeaderLength = 13

func FillFileHeader(b []byte, flags uint8) {
	// 'FLV', version 1
	pio.PutU32BE(b[0:4], 0x464c5601)
	b[4] = flags

	// DataOffset: UI32 Offset in bytes from start of file to start of body (that is, size of header)
	// The DataOffset field usually has a value of 9 for FLV version 1.
	pio.PutU32BE(b[5:9], 9)

	// PreviousTagSize0: UI32 Always 0
	pio.PutU32BE(b[9:13], 0)

	return
}

func ParseFileHeader(b []byte) (flags uint8, skip int, err error) {
	flv := pio.U24BE(b[0:3])
	if flv != 0x464c56 { // 'FLV'
		err = fmt.Errorf("CC3Invalid")
		return
	}
	flags = b[4]

	skip = int(pio.U32BE(b[5:9])) - 9
	if skip < 0 {
		err = fmt.Errorf("FileHeaderDataSizeInvalid")
		return
	}

	return
}
