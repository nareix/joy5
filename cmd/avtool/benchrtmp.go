package main

import (
	"fmt"
	"io"
	"net"
	"sync/atomic"
	"time"

	"github.com/nareix/joy5/av"
	"github.com/nareix/joy5/format"
	"github.com/nareix/joy5/format/rtmp"
)

func doBenchRtmp(listenAddr, file string) (err error) {
	fo := newFormatOpener()

	filePkts := []av.Packet{}

	if file != "" {
		var r *format.Reader
		if r, err = fo.Open(file); err != nil {
			return
		}
		for {
			var pkt av.Packet
			if pkt, err = r.ReadPacket(); err != nil {
				if err != io.EOF {
					return
				}
				break
			}
			filePkts = append(filePkts, pkt)
		}
		r.Close()
	}

	var lis net.Listener
	if lis, err = net.Listen("tcp", listenAddr); err != nil {
		return
	}

	s := rtmp.NewServer()
	s.OnNewConn = func(c *rtmp.Conn) {
		handleRtmpConnFlags(c)
	}
	handleRtmpServerFlags(s)

	var pubN, subN int64

	s.HandleConn = func(c *rtmp.Conn, nc net.Conn) {
		defer func() {
			nc.Close()
		}()

		var p *int64
		if c.Publishing {
			p = &pubN
		} else {
			p = &subN
		}
		atomic.AddInt64(p, 1)
		defer atomic.AddInt64(p, -1)

		if c.Publishing {
			for {
				if _, err := c.ReadPacket(); err != nil {
					return
				}
			}
		} else {
			start := time.Now()
			for _, pkt := range filePkts {
				if err := c.WritePacket(pkt); err != nil {
					return
				}
				time.Sleep(pkt.Time - time.Now().Sub(start))
			}
		}
	}

	go func() {
		for range time.Tick(time.Second) {
			fmt.Println("sub", atomic.LoadInt64(&subN), "pub", atomic.LoadInt64(&pubN))
		}
	}()

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
