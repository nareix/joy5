package main

import (
	"os"

	"github.com/nareix/joy5/av"
	"github.com/nareix/joy5/codec/h264"
	"github.com/nareix/joy5/format"
	"github.com/nareix/joy5/format/flv"
)

func doMoveH264SeqhdrToKeyFrame(src, dst string) error {
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
	w := flv.NewMuxer(format.NewStreamsWriteSeeker(fw, nil))

	rawf, _ := os.Create("/tmp/a.h264")
	defer rawf.Close()

	var h264seqhdr *h264.Codec

	for {
		pkt, err := r.ReadPacket()
		if err != nil {
			return err
		}

		switch pkt.Type {
		case av.H264DecoderConfig:
			h, err := h264.FromDecoderConfig(pkt.Data)
			if err != nil {
				return err
			}
			h264seqhdr = h
			continue // skip seqhdr

		case av.H264:
			if pkt.IsKeyFrame {
				pktnalus, _ := h264.SplitNALUs(pkt.Data)
				nalus := [][]byte{}
				for _, b := range h264.Map2arr(h264seqhdr.SPS) {
					nalus = append(nalus, b)
				}
				for _, b := range h264.Map2arr(h264seqhdr.PPS) {
					nalus = append(nalus, b)
				}
				nalus = append(nalus, pktnalus...)
				data := h264.JoinNALUsAnnexb(nalus)
				pkt.Data = data
			} else {
				pktnalus, _ := h264.SplitNALUs(pkt.Data)
				data := h264.JoinNALUsAnnexb(pktnalus)
				pkt.Data = data
			}
		}

		if err := w.WritePacket(pkt); err != nil {
			return err
		}
	}
}
