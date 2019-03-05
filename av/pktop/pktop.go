package pktop

import (
	"github.com/nareix/joy5/codec/aac"
	"github.com/nareix/joy5/codec/h264"
)

type VideoKeyAddExtra struct {
}

type PacketInput struct {
	H264    *h264.Codec
	AAC     *aac.Codec
	readPkt func() (Packet, error)
}

func NewPacketInput(readPkt func() (Packet, error)) *PacketInput {
	return &PacketInput{readPkt: readPkt}
}

type PacketOutput struct {
	lastH264 *h264.Codec
	lastAAC  *aac.Codec
	writePkt func(Packet) error
}

func NewPacketOutput(writePkt func(Packet) error) *PacketOutput {
	return &PacketOutput{writePkt: writePkt}
}

func WriteToNewSlice(marshall func([]byte, *int)) (b []byte) {
	var size int
	marshall(nil, &size)
	b = make([]byte, size)
	var n int
	marshall(b, &n)
	return
}

func (n *PacketOutput) WritePacket(pkt Packet) (err error) {
	switch pkt.Type {
	case H264:
		c := pkt.H264
		if c != nil && (n.lastH264 == nil || !n.lastH264.Equal(*c)) {
			if err = n.writePkt(Packet{
				Type: H264DecoderConfig,
				Data: WriteToNewSlice(c.ToConfig),
			}); err != nil {
				return
			}
			n.lastH264 = c
		}
		return n.writePkt(pkt)

	case AAC:
		if n.lastAAC == nil && pkt.AAC != nil {
			n.lastAAC = pkt.AAC
			wpkt := Packet{
				Type: AACDecoderConfig,
				AAC:  pkt.AAC,
			}
			if err = n.writePkt(wpkt); err != nil {
				return
			}
		}
		return n.writePkt(pkt)
	}
	return
}

func (n *PacketInput) ReadPacket() (pkt Packet, err error) {
	for {
		var rpkt Packet
		if rpkt, err = n.readPkt(); err != nil {
			return
		}
		switch rpkt.Type {
		case H264:
			pkt = rpkt
			allnalus, _ := h264.SplitNALUs(rpkt.Data)
			var spspps [][]byte
			for _, nalu := range allnalus {
				switch h264.NALUType(nalu) {
				case h264.NALU_SPS, h264.NALU_PPS:
					spspps = append(spspps, nalu)
				}
			}
			if len(spspps) > 0 {
				var c *h264.Codec
				if n.H264 != nil {
					c = h264.FromOld(*n.H264)
				} else {
					c = h264.NewCodec()
				}
				for _, b := range spspps {
					c.AddSPSPPS(b)
				}
				if n.H264 == nil || !n.H264.Equal(*c) {
					n.H264 = c
				}
				newnalus := [][]byte{}
				for _, nalu := range allnalus {
					switch h264.NALUType(nalu) {
					case h264.NALU_SPS, h264.NALU_PPS:
						continue
					}
					newnalus = append(newnalus, nalu)
				}
				pkt.Data = h264.JoinNALUsAnnexb(newnalus)
			}
			pkt.H264 = n.H264
			return

		case AAC:
			pkt = rpkt
			if len(rpkt.ExtraData) > 0 && n.AAC == nil {
				c := aac.NewCodecFromMPEG4AudioConfigBytes(rpkt.ExtraData)
				n.AAC = &c
			}
			pkt.AAC = n.AAC
			return

		case AACDecoderConfig:
			c := aac.NewCodecFromMPEG4AudioConfigBytes(rpkt.Data)
			n.AAC = &c

		case H264DecoderConfig:
			if c, _ := h264.FromDecoderConfig(rpkt.Data); c != nil {
				n.H264 = c
			}
		}
	}

	return
}
