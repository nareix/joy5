package main

import (
	"fmt"
	"net"
	"sync/atomic"
	"time"

	"github.com/nareix/joy5/av"
	"github.com/nareix/joy5/format"
)

type StatConn struct {
	net.Conn
	OnRead  func(int)
	OnWrite func(int)
}

func (sc *StatConn) Read(b []byte) (int, error) {
	n, err := sc.Conn.Read(b)
	if err == nil {
		if fn := sc.OnRead; fn != nil {
			fn(n)
		}
	}
	return n, err
}

func (sc *StatConn) Write(b []byte) (int, error) {
	n, err := sc.Conn.Write(b)
	if err == nil {
		if fn := sc.OnWrite; fn != nil {
			fn(n)
		}
	}
	return n, err
}

func startStatSec(or, ow *format.URLOpener) (onPkt func(av.Packet)) {
	type Stat struct {
		tx, rx int64
		name   string
	}
	chstat := make(chan *Stat)
	var input, output int
	newstatconn := func(oldnc net.Conn, name string) net.Conn {
		s := &Stat{name: name}
		nc := &StatConn{
			Conn:    oldnc,
			OnRead:  func(n int) { atomic.AddInt64(&s.rx, int64(n)) },
			OnWrite: func(n int) { atomic.AddInt64(&s.tx, int64(n)) },
		}
		chstat <- s
		return nc
	}
	or.ReplaceRawConn = func(oldnc net.Conn) net.Conn {
		nc := newstatconn(oldnc, fmt.Sprint("input", input))
		input++
		return nc
	}
	ow.ReplaceRawConn = func(oldnc net.Conn) net.Conn {
		nc := newstatconn(oldnc, fmt.Sprint("output", output))
		output++
		return nc
	}
	var statPkt struct {
		n int64
	}
	onPkt = func(pkt av.Packet) {
		atomic.AddInt64(&statPkt.n, 1)
	}
	go func() {
		ss := []*Stat{}
		for {
			select {
			case s := <-chstat:
				ss = append(ss, s)
			case <-time.After(time.Second):
				pkts := atomic.SwapInt64(&statPkt.n, 0)
				fmt.Println("StatSec", "pkts", pkts)
				for _, s := range ss {
					tx := atomic.SwapInt64(&s.tx, 0)
					rx := atomic.SwapInt64(&s.rx, 0)
					fmt.Println("StatSec", s.name, "tx", tx, "rx", rx)
				}
			}
		}
	}()
	return
}
