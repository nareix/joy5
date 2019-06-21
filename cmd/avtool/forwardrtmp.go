package main

import (
	"log"
	"net"
	"time"
	"unsafe"

	"github.com/nareix/joy5/av"
	"github.com/nareix/joy5/format"

	"github.com/nareix/joy5/format/rtmp"
)

func doForwardRtmp(listenAddr string) (err error) {
	var lis net.Listener
	if lis, err = net.Listen("tcp", listenAddr); err != nil {
		return
	}

	s := rtmp.NewServer()
	handleRtmpServerFlags(s)

	s.LogEvent = func(c *rtmp.Conn, nc net.Conn, e int) {
		es := rtmp.EventString[e]
		log.Println(unsafe.Pointer(c), nc.LocalAddr(), nc.RemoteAddr(), es)
	}

	s.HandleConn = func(c *rtmp.Conn, nc net.Conn) {
		defer nc.Close()

		if !c.Publishing {
			log.Println(unsafe.Pointer(c), nc.LocalAddr(), nc.RemoteAddr(), "NotPub")
			return
		}

		q := c.URL.Query()
		fwd := q.Get("fwd")
		if fwd == "" {
			log.Println(unsafe.Pointer(c), nc.LocalAddr(), nc.RemoteAddr(), "NoForwardField")
			return
		}
		log.Println(unsafe.Pointer(c), nc.LocalAddr(), nc.RemoteAddr(), "fwd", fwd)

		fo := newFormatOpener()

		var err error
		var w *format.Writer
		if w, err = fo.Create(fwd); err != nil {
			log.Println(unsafe.Pointer(c), nc.LocalAddr(), nc.RemoteAddr(), "DialFailed")
			return
		}
		c2 := w.Rtmp
		nc2 := w.NetConn
		defer nc2.Close()

		log.Println(unsafe.Pointer(c), nc.LocalAddr(), nc.RemoteAddr(), "DialOK", unsafe.Pointer(c2))

		for {
			var pkt av.Packet
			if pkt, err = c.ReadPacket(); err != nil {
				break
			}
			if err = c2.WritePacket(pkt); err != nil {
				break
			}
		}

		log.Println(unsafe.Pointer(c), nc.LocalAddr(), nc.RemoteAddr(), "Closed")
		log.Println(unsafe.Pointer(c2), nc.LocalAddr(), nc.RemoteAddr(), "Closed")
	}

	func() {
		for {
			nc, err := lis.Accept()
			if err != nil {
				time.Sleep(time.Second)
				continue
			}
			go s.HandleNetConn(nc)
		}
	}()
	return
}
