package main

/*
#cgo linux LDFLAGS: -ljack

#include <client.h>

*/

import (
	"log"
	"math/rand"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/nareix/joy5/av"
	"github.com/nareix/joy5/format"
	"github.com/nareix/joy5/format/mp4"
	"github.com/nareix/joy5/format/rtsp"
)

func TestRecMP4(t *testing.T) {
	s1 := rand.NewSource(time.Now().UnixNano())
	r1 := rand.New(s1)
	dst := "/home/krasi/Downloads/" + strconv.Itoa(r1.Intn(100)) + ".mp4"
	src := "rtsp://admin:1qazxsw2@ds-2cd1301-i20200709aawre58045175.local:554"

	rtspClt, err := rtsp.DialTimeout(src, 3*time.Second)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := rtspClt.Close(); err != nil {
			log.Fatal(err)
		}
	}()

	os.RemoveAll(dst)

	var clt *rtsp.Client
	clt, err = rtsp.DialTimeout(src, 3*time.Second)
	if err != nil {
		log.Fatal(err)
	}

	var f *os.File
	if f, err = os.Create(dst); err != nil {
		log.Fatal(err)

	}

	var m *mp4.Muxer
	m, err = mp4.NewMuxer(format.NewStreamsWriteSeeker(f, clt))
	if err != nil {
		log.Fatal(err)
	}
	err = m.WriteFileHeader()
	if err != nil {
		log.Fatal(err)
	}

	defer func() {
		if err := m.WriteTrailer(); err != nil {
			log.Fatal(err)
		}
		if err := f.Close(); err != nil {
			log.Fatal(err)
		}
	}()

	for i := 0; i < 200; i++ {
		var pkt av.Packet
		if pkt, err = clt.ReadPacket(); err != nil {
			log.Fatal(err)
		}
		if err = m.WritePacket(pkt); err != nil {
			log.Fatal(err)
		}

	}
}
