package rtsp

import (
	"time"

	"github.com/nareix/joy5/av"
	"github.com/nareix/joy5/format/rtsp/sdp"
)

type Stream struct {
	Codec  interface{}
	Sdp    sdp.Media
	client *Client

	// h264
	fuStarted  bool
	fuBuffer   []byte
	sps        []byte
	pps        []byte
	spsChanged bool
	ppsChanged bool

	gotpkt         bool
	pkt            av.Packet
	timestamp      uint32
	firsttimestamp uint32

	lasttime time.Duration
}
