package main

import (
	"os"

	"github.com/nareix/joy5/av"
	"github.com/nareix/joy5/codec/h264"
	"github.com/nareix/joy5/format/flv"
)

func doAvcc2Annexb(src, dst string) error {
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

	for {
		pkt, err := r.ReadPacket()
		if err != nil {
			return err
		}

		if pkt.Type == av.H264 {
			nalus, _ := h264.SplitNALUs(pkt.Data)
			annexb := h264.JoinNALUsAnnexb(nalus)
			avcc := h264.JoinNALUsAVCC([][]byte{annexb})
			pkt.Data = avcc
		}

		if err := w.WritePacket(pkt); err != nil {
			return err
		}
	}
}
