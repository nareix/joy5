package main

import (
	"encoding/hex"
	"fmt"
	"net"

	"github.com/nareix/joy5/format"
	"github.com/nareix/joy5/format/flv"
	"github.com/nareix/joy5/format/flv/flvio"
	"github.com/nareix/joy5/format/rtmp"
)

var debugRtmpChunkData = false
var debugRtmpNetEvent = false
var debugRtmpStage = false
var debugRtmpOptsMap = map[string]*bool{
	"chunk": &debugRtmpChunkData,
	"net":   &debugRtmpNetEvent,
	"stage": &debugRtmpStage,
}

var debugFlvHeader = false
var debugFlvOptsMap = map[string]*bool{
	"filehdr": &debugFlvHeader,
}

func handleRtmpClientFlags(c *rtmp.Client) {
	if debugRtmpNetEvent {
		c.LogEvent = func(c *rtmp.Conn, nc net.Conn, e int) {
			es := rtmp.EventString[e]
			fmt.Println("RtmpEvent", nc.LocalAddr(), nc.RemoteAddr(), es)
		}
	}
}

func handleRtmpServerFlags(s *rtmp.Server) {
	if debugRtmpNetEvent {
		s.LogEvent = func(c *rtmp.Conn, nc net.Conn, e int) {
			es := rtmp.EventString[e]
			fmt.Println("RtmpEvent", nc.LocalAddr(), nc.RemoteAddr(), es)
		}
	}
}

func handleRtmpConnFlags(c *rtmp.Conn) {
	if debugRtmpChunkData {
		c.LogChunkDataEvent = func(isRead bool, b []byte) {
			dir := ""
			if isRead {
				dir = "<"
			} else {
				dir = ">"
			}
			fmt.Println(dir, len(b))
			fmt.Print(hex.Dump(b))
		}
	}
	if debugRtmpStage {
		c.LogStageEvent = func(e string, url string) {
			fmt.Println("RtmpStage", e, url)
		}
	}
}

func handleFlvDemuxerFlags(r *flv.Demuxer) {
	if debugFlvHeader {
		r.LogHeaderEvent = func(flags uint8) {
			avflags := ""
			if flags&flvio.FILE_HAS_AUDIO != 0 {
				avflags += "A"
			}
			if flags&flvio.FILE_HAS_VIDEO != 0 {
				avflags += "V"
			}
			fmt.Println("FLVHeader", "AVFlags", avflags)
		}
	}
}

func newFormatOpener() *format.URLOpener {
	return &format.URLOpener{
		OnNewFlvDemuxer: func(r *flv.Demuxer) {
			handleFlvDemuxerFlags(r)
		},
		OnNewRtmpConn: func(c *rtmp.Conn) {
			handleRtmpConnFlags(c)
		},
		OnNewRtmpServer: func(s *rtmp.Server) {
			handleRtmpServerFlags(s)
		},
		OnNewRtmpClient: func(c *rtmp.Client) {
			handleRtmpClientFlags(c)
		},
	}
}
