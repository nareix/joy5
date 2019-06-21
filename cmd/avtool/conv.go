package main

import (
	"fmt"
	"io"

	"github.com/nareix/joy5/av"
	"github.com/nareix/joy5/av/pktop"
	"github.com/nareix/joy5/format"
)

var optDontPrintPkt = false
var optNativeRate = false
var optPrintStatSec = false

func doConv(src, dst string) (err error) {
	foR := newFormatOpener()
	foW := newFormatOpener()

	var onPkt func(av.Packet)

	if optPrintStatSec {
		onPkt = startStatSec(foR, foW)
	}

	var fr *format.Reader
	var fw *format.Writer
	var re *pktop.NativeRateLimiter

	if fr, err = foR.Open(src); err != nil {
		return
	}

	canRe := func() bool {
		if optNativeRate {
			return true
		}
		if fw != nil && fw.IsRemote {
			return true
		}
		return false
	}

	for {
		var pkt av.Packet
		if pkt, err = fr.ReadPacket(); err != nil {
			if err == io.EOF {
				err = nil
			}
			return
		}

		pkts := []av.Packet{pkt}

		if fn := onPkt; fn != nil {
			for _, pkt := range pkts {
				fn(pkt)
			}
		}

		if re == nil && canRe() {
			re = pktop.NewNativeRateLimiter()
		}
		if re != nil {
			pkts = re.Do(pkts)
		}

		for _, pkt := range pkts {
			if !optDontPrintPkt {
				fmt.Println(pkt.String())
			}

			if dst != "" && fw == nil {
				if fw, err = foW.Create(dst); err != nil {
					return
				}
			}

			if fw != nil {
				if err = fw.WritePacket(pkt); err != nil {
					return
				}
			}
		}
	}
}
