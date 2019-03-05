package av

import (
	"time"

	"github.com/nareix/joy5/codec/aac"
	"github.com/nareix/joy5/codec/h264"
)

const (
	H264 = 1 + iota
	AAC
	H264DecoderConfig
	H264SPSPPSNALU
	AACDecoderConfig
)

type Packet struct {
	Type       int
	IsKeyFrame bool
	CTime      time.Duration
	Time       time.Duration
	Data       []byte
	ExtraData  []byte
	AAC        *aac.Codec
	H264       *h264.Codec
}

// StoreBlockUpdate -> GateBlockUpdate -> Snapshot

// RTMP -> SortMerger -> H264KeyAddExtra

type PacketReader interface {
	ReadPacket() (Packet, error)
}

type PacketWriter interface {
	WritePacket(Packet) error
}

type PacketWriteCloser interface {
	PacketWriter
	Close() error
}
