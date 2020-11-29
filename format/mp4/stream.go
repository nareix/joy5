package mp4

import (
	"fmt"
	"io"
	"reflect"
	"time"

	"github.com/nareix/joy5/av"
	"github.com/nareix/joy5/codec/aac"
	"github.com/nareix/joy5/codec/h264"
	"github.com/nareix/joy5/format/mp4/mp4io"
	"github.com/pkg/errors"
)

const codecTypeAudioBit = 0x1

type Stream struct {
	Codec interface{}

	trackAtom *mp4io.Track
	idx       int

	lastpkt *av.Packet

	timeScale int64
	duration  int64

	muxer   *Muxer
	demuxer *Demuxer

	sample      *mp4io.SampleTable
	sampleIndex int

	sampleOffsetInChunk int64
	syncSampleIndex     int

	dts                    int64
	sttsEntryIndex         int
	sampleIndexInSttsEntry int

	cttsEntryIndex         int
	sampleIndexInCttsEntry int

	chunkGroupIndex    int
	chunkIndex         int
	sampleIndexInChunk int

	sttsEntry *mp4io.TimeToSampleEntry
	cttsEntry *mp4io.CompositionOffsetEntry
}

func CodecType(codec interface{}) (int, error) {
	var cType int
	switch codec.(type) {
	case *h264.Codec:
		cType = av.H264
	case *aac.Codec:
		cType = av.AAC
	default:
		return 0, errors.Errorf("mp4: codec is not supported type=%+v ", reflect.TypeOf(codec))
	}
	return cType, nil
}

func IsAudio(t int) bool {
	return t&codecTypeAudioBit != 0
}

func IsVideo(t int) bool {
	return t&codecTypeAudioBit == 0
}

func timeToTs(tm time.Duration, timeScale int64) int64 {
	return int64(tm * time.Duration(timeScale) / time.Second)
}

func tsToTime(ts int64, timeScale int64) time.Duration {
	return time.Duration(ts) * time.Second / time.Duration(timeScale)
}

func (self *Stream) timeToTs(tm time.Duration) int64 {
	return int64(tm * time.Duration(self.timeScale) / time.Second)
}

func (self *Stream) tsToTime(ts int64) time.Duration {
	return time.Duration(ts) * time.Second / time.Duration(self.timeScale)
}

func (self *Stream) setSampleIndex(index int) (err error) {
	found := false
	start := 0
	self.chunkGroupIndex = 0

	for self.chunkIndex = range self.sample.ChunkOffset.Entries {
		if self.chunkGroupIndex+1 < len(self.sample.SampleToChunk.Entries) &&
			uint32(self.chunkIndex+1) == self.sample.SampleToChunk.Entries[self.chunkGroupIndex+1].FirstChunk {
			self.chunkGroupIndex++
		}
		n := int(self.sample.SampleToChunk.Entries[self.chunkGroupIndex].SamplesPerChunk)
		if index >= start && index < start+n {
			found = true
			self.sampleIndexInChunk = index - start
			break
		}
		start += n
	}
	if !found {
		err = fmt.Errorf("mp4: stream[%d]: cannot locate sample index in chunk", self.idx)
		return
	}

	if self.sample.SampleSize.SampleSize != 0 {
		self.sampleOffsetInChunk = int64(self.sampleIndexInChunk) * int64(self.sample.SampleSize.SampleSize)
	} else {
		if index >= len(self.sample.SampleSize.Entries) {
			err = fmt.Errorf("mp4: stream[%d]: sample index out of range", self.idx)
			return
		}
		self.sampleOffsetInChunk = int64(0)
		for i := index - self.sampleIndexInChunk; i < index; i++ {
			self.sampleOffsetInChunk += int64(self.sample.SampleSize.Entries[i])
		}
	}

	self.dts = int64(0)
	start = 0
	found = false
	self.sttsEntryIndex = 0
	for self.sttsEntryIndex < len(self.sample.TimeToSample.Entries) {
		entry := self.sample.TimeToSample.Entries[self.sttsEntryIndex]
		n := int(entry.Count)
		if index >= start && index < start+n {
			self.sampleIndexInSttsEntry = index - start
			self.dts += int64(index-start) * int64(entry.Duration)
			found = true
			break
		}
		start += n
		self.dts += int64(n) * int64(entry.Duration)
		self.sttsEntryIndex++
	}
	if !found {
		err = fmt.Errorf("mp4: stream[%d]: cannot locate sample index in stts entry", self.idx)
		return
	}

	if self.sample.CompositionOffset != nil && len(self.sample.CompositionOffset.Entries) > 0 {
		start = 0
		found = false
		self.cttsEntryIndex = 0
		for self.cttsEntryIndex < len(self.sample.CompositionOffset.Entries) {
			n := int(self.sample.CompositionOffset.Entries[self.cttsEntryIndex].Count)
			if index >= start && index < start+n {
				self.sampleIndexInCttsEntry = index - start
				found = true
				break
			}
			start += n
			self.cttsEntryIndex++
		}
		if !found {
			err = fmt.Errorf("mp4: stream[%d]: cannot locate sample index in ctts entry", self.idx)
			return
		}
	}

	if self.sample.SyncSample != nil {
		self.syncSampleIndex = 0
		for self.syncSampleIndex < len(self.sample.SyncSample.Entries)-1 {
			if self.sample.SyncSample.Entries[self.syncSampleIndex+1]-1 > uint32(index) {
				break
			}
			self.syncSampleIndex++
		}
	}

	if false {
		fmt.Printf("mp4: stream[%d]: setSampleIndex chunkGroupIndex=%d chunkIndex=%d sampleOffsetInChunk=%d\n",
			self.idx, self.chunkGroupIndex, self.chunkIndex, self.sampleOffsetInChunk)
	}

	self.sampleIndex = index
	return
}

func (self *Stream) isSampleValid() bool {
	if self.chunkIndex >= len(self.sample.ChunkOffset.Entries) {
		return false
	}
	if self.chunkGroupIndex >= len(self.sample.SampleToChunk.Entries) {
		return false
	}
	if self.sttsEntryIndex >= len(self.sample.TimeToSample.Entries) {
		return false
	}
	if self.sample.CompositionOffset != nil && len(self.sample.CompositionOffset.Entries) > 0 {
		if self.cttsEntryIndex >= len(self.sample.CompositionOffset.Entries) {
			return false
		}
	}
	if self.sample.SyncSample != nil {
		if self.syncSampleIndex >= len(self.sample.SyncSample.Entries) {
			return false
		}
	}
	if self.sample.SampleSize.SampleSize != 0 {
		if self.sampleIndex >= len(self.sample.SampleSize.Entries) {
			return false
		}
	}
	return true
}

func (self *Stream) incSampleIndex() (duration int64) {
	if false {
		fmt.Printf("incSampleIndex sampleIndex=%d sampleOffsetInChunk=%d sampleIndexInChunk=%d chunkGroupIndex=%d chunkIndex=%d\n",
			self.sampleIndex, self.sampleOffsetInChunk, self.sampleIndexInChunk, self.chunkGroupIndex, self.chunkIndex)
	}

	self.sampleIndexInChunk++
	if uint32(self.sampleIndexInChunk) == self.sample.SampleToChunk.Entries[self.chunkGroupIndex].SamplesPerChunk {
		self.chunkIndex++
		self.sampleIndexInChunk = 0
		self.sampleOffsetInChunk = int64(0)
	} else {
		if self.sample.SampleSize.SampleSize != 0 {
			self.sampleOffsetInChunk += int64(self.sample.SampleSize.SampleSize)
		} else {
			self.sampleOffsetInChunk += int64(self.sample.SampleSize.Entries[self.sampleIndex])
		}
	}

	if self.chunkGroupIndex+1 < len(self.sample.SampleToChunk.Entries) &&
		uint32(self.chunkIndex+1) == self.sample.SampleToChunk.Entries[self.chunkGroupIndex+1].FirstChunk {
		self.chunkGroupIndex++
	}

	sttsEntry := self.sample.TimeToSample.Entries[self.sttsEntryIndex]
	duration = int64(sttsEntry.Duration)
	self.sampleIndexInSttsEntry++
	self.dts += duration
	if uint32(self.sampleIndexInSttsEntry) == sttsEntry.Count {
		self.sampleIndexInSttsEntry = 0
		self.sttsEntryIndex++
	}

	if self.sample.CompositionOffset != nil && len(self.sample.CompositionOffset.Entries) > 0 {
		self.sampleIndexInCttsEntry++
		if uint32(self.sampleIndexInCttsEntry) == self.sample.CompositionOffset.Entries[self.cttsEntryIndex].Count {
			self.sampleIndexInCttsEntry = 0
			self.cttsEntryIndex++
		}
	}

	if self.sample.SyncSample != nil {
		entries := self.sample.SyncSample.Entries
		if self.syncSampleIndex+1 < len(entries) && entries[self.syncSampleIndex+1]-1 == uint32(self.sampleIndex+1) {
			self.syncSampleIndex++
		}
	}

	self.sampleIndex++
	return
}

func (self *Stream) sampleCount() int {
	if self.sample.SampleSize.SampleSize == 0 {
		chunkGroupIndex := 0
		count := 0
		for chunkIndex := range self.sample.ChunkOffset.Entries {
			n := int(self.sample.SampleToChunk.Entries[chunkGroupIndex].SamplesPerChunk)
			count += n
			if chunkGroupIndex+1 < len(self.sample.SampleToChunk.Entries) &&
				uint32(chunkIndex+1) == self.sample.SampleToChunk.Entries[chunkGroupIndex+1].FirstChunk {
				chunkGroupIndex++
			}
		}
		return count
	} else {
		return len(self.sample.SampleSize.Entries)
	}
}

func (self *Stream) readPacket() (pkt av.Packet, err error) {
	if !self.isSampleValid() {
		err = io.EOF
		return
	}
	//fmt.Println("readPacket", self.sampleIndex)

	chunkOffset := self.sample.ChunkOffset.Entries[self.chunkIndex]
	sampleSize := uint32(0)
	if self.sample.SampleSize.SampleSize != 0 {
		sampleSize = self.sample.SampleSize.SampleSize
	} else {
		sampleSize = self.sample.SampleSize.Entries[self.sampleIndex]
	}

	sampleOffset := int64(chunkOffset) + self.sampleOffsetInChunk
	pkt.Data = make([]byte, sampleSize)
	if err = self.demuxer.readat(sampleOffset, pkt.Data); err != nil {
		return
	}

	if self.sample.SyncSample != nil {
		if self.sample.SyncSample.Entries[self.syncSampleIndex]-1 == uint32(self.sampleIndex) {
			pkt.IsKeyFrame = true
		}
	}

	//println("pts/dts", self.ptsEntryIndex, self.dtsEntryIndex)
	if self.sample.CompositionOffset != nil && len(self.sample.CompositionOffset.Entries) > 0 {
		cts := int64(self.sample.CompositionOffset.Entries[self.cttsEntryIndex].Offset)
		pkt.CTime = self.tsToTime(cts)
	}

	self.incSampleIndex()

	return
}

func (self *Stream) seekToTime(tm time.Duration) (err error) {
	index := self.timeToSampleIndex(tm)
	if err = self.setSampleIndex(index); err != nil {
		return
	}
	if false {
		fmt.Printf("stream[%d]: seekToTime index=%v time=%v cur=%v\n", self.idx, index, tm, self.tsToTime(self.dts))
	}
	return
}

func (self *Stream) timeToSampleIndex(tm time.Duration) int {
	targetTs := self.timeToTs(tm)
	targetIndex := 0

	startTs := int64(0)
	endTs := int64(0)
	startIndex := 0
	endIndex := 0
	found := false
	for _, entry := range self.sample.TimeToSample.Entries {
		endTs = startTs + int64(entry.Count*entry.Duration)
		endIndex = startIndex + int(entry.Count)
		if targetTs >= startTs && targetTs < endTs {
			targetIndex = startIndex + int((targetTs-startTs)/int64(entry.Duration))
			found = true
		}
		startTs = endTs
		startIndex = endIndex
	}
	if !found {
		if targetTs < 0 {
			targetIndex = 0
		} else {
			targetIndex = endIndex - 1
		}
	}

	if self.sample.SyncSample != nil {
		entries := self.sample.SyncSample.Entries
		for i := len(entries) - 1; i >= 0; i-- {
			if entries[i]-1 < uint32(targetIndex) {
				targetIndex = int(entries[i] - 1)
				break
			}
		}
	}

	return targetIndex
}
