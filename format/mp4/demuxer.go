package mp4

import (
	"fmt"
	"io"
	"time"

	"github.com/nareix/joy5/av"
	"github.com/nareix/joy5/codec/aac"
	"github.com/nareix/joy5/codec/h264"
	"github.com/nareix/joy5/format/mp4/mp4io"
)

type Demuxer struct {
	r         io.ReadSeeker
	streams   []*Stream
	movieAtom *mp4io.Movie
}

func NewDemuxer(r io.ReadSeeker) *Demuxer {
	return &Demuxer{
		r: r,
	}
}

func (self *Demuxer) Streams() (streams []interface{}, err error) {
	if err = self.probe(); err != nil {
		return
	}
	for _, stream := range self.streams {
		streams = append(streams, stream.Codec)
	}
	return
}

func (self *Demuxer) readat(pos int64, b []byte) (err error) {
	if _, err = self.r.Seek(pos, 0); err != nil {
		return
	}
	if _, err = io.ReadFull(self.r, b); err != nil {
		return
	}
	return
}

func (self *Demuxer) probe() (err error) {
	if self.movieAtom != nil {
		return
	}

	var moov *mp4io.Movie
	var atoms []mp4io.Atom

	if atoms, err = mp4io.ReadFileAtoms(self.r); err != nil {
		return
	}
	if _, err = self.r.Seek(0, 0); err != nil {
		return
	}

	for _, atom := range atoms {
		if atom.Tag() == mp4io.MOOV {
			moov = atom.(*mp4io.Movie)
		}
	}

	if moov == nil {
		err = fmt.Errorf("mp4: 'moov' atom not found")
		return
	}

	self.streams = []*Stream{}
	for i, atrack := range moov.Tracks {
		stream := &Stream{
			trackAtom: atrack,
			demuxer:   self,
			idx:       i,
		}
		if atrack.Media != nil && atrack.Media.Info != nil && atrack.Media.Info.Sample != nil {
			stream.sample = atrack.Media.Info.Sample
			stream.timeScale = int64(atrack.Media.Header.TimeScale)
		} else {
			err = fmt.Errorf("mp4: sample table not found")
			return
		}

		if avc1 := atrack.GetAVC1Conf(); avc1 != nil {
			if stream.Codec, err = h264.FromDecoderConfig(avc1.Data); err != nil {
				return
			}
			self.streams = append(self.streams, stream)
		} else if esds := atrack.GetElemStreamDesc(); esds != nil {
			if stream.Codec, err = aac.FromMPEG4AudioConfigBytes(esds.DecConfig); err != nil {
				return
			}
			self.streams = append(self.streams, stream)
		}
	}

	self.movieAtom = moov
	return
}

func (self *Demuxer) ReadPacket() (pkt av.Packet, err error) {
	if err = self.probe(); err != nil {
		return
	}

	var chosen *Stream
	for _, stream := range self.streams {
		if chosen == nil || stream.tsToTime(stream.dts) < chosen.tsToTime(chosen.dts) {
			chosen = stream
		}
	}
	if false {
		fmt.Printf("ReadPacket: chosen index=%v time=%v\n", chosen.idx, chosen.tsToTime(chosen.dts))
	}
	tm := chosen.tsToTime(chosen.dts)
	if pkt, err = chosen.readPacket(); err != nil {
		return
	}
	pkt.Time = tm
	return
}

func (self *Demuxer) CurrentTime() (tm time.Duration) {
	if len(self.streams) > 0 {
		stream := self.streams[0]
		tm = stream.tsToTime(stream.dts)
	}
	return
}

func (self *Demuxer) SeekToTime(tm time.Duration) (err error) {
	for sType, stream := range self.streams {
		if IsVideo(sType) {
			if err = stream.seekToTime(tm); err != nil {
				return
			}
			tm = stream.tsToTime(stream.dts)
			break
		}
	}

	for sType, stream := range self.streams {
		if !IsVideo(sType) {
			if err = stream.seekToTime(tm); err != nil {
				return
			}
		}
	}

	return
}
