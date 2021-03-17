package main

import (
	"os"

	"github.com/nareix/joy5/av"
	"github.com/nareix/joy5/format/flv"
)

func doSkipGop(src, dst string) error {
	fr, err := os.Open(src)
	if err != nil {
		return err
	}
	defer fr.Close()

	fw, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer fw.Close()

	r := flv.NewDemuxer(fr)
	w := flv.NewMuxer(fw)

	gopnr := 0

	for {
		pkt, err := r.ReadPacket()
		if err != nil {
			return err
		}

		switch pkt.Type {
		case av.H264:
			if pkt.IsKeyFrame {
				gopnr++
			}
			if gopnr > 2 {
				if err := w.WritePacket(pkt); err != nil {
					return err
				}
			}

		default:
			if err := w.WritePacket(pkt); err != nil {
				return err
			}
		}
	}
}
