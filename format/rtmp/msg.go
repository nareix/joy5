package rtmp

import (
	"fmt"
	"io"
	"io/ioutil"
	"time"

	"github.com/hashicorp/errwrap"

	"github.com/nareix/joy5/format/flv"
	"github.com/nareix/joy5/format/flv/flvio"
	"github.com/nareix/joy5/utils"
	"github.com/nareix/joy5/utils/bits/pio"
)

type message struct {
	timenow     uint32
	timedelta   uint32
	hastimeext  bool
	timeext     uint32
	msgsid      uint32
	msgtypeid   uint8
	msgdatalen  uint32
	msgdataleft uint32
	msghdrtype  uint8
	msgdata     []byte
	sendack     uint32

	malloc func(int) ([]byte, error)

	aggreadp   []byte
	aggfirsttm uint32
	aggidx     int
}

func (c *Conn) nextAggPart(m *message) (got bool, am *message, err error) {
	if m.aggreadp == nil {
		m.aggreadp = m.msgdata
	}
	if len(m.aggreadp) < flvio.TagHeaderLength+4 {
		return
	}

	var tag flvio.Tag
	var datalen int
	if tag, datalen, err = flvio.ParseTagHeader(m.aggreadp); err != nil {
		return
	}
	m.aggreadp = m.aggreadp[flvio.TagHeaderLength:]

	if len(m.aggreadp) < datalen+4 {
		return
	}

	tag.Data = m.aggreadp[:datalen]

	if m.aggidx == 0 {
		m.aggfirsttm = tag.Time
	}

	m.aggreadp = m.aggreadp[datalen+4:]
	m.aggidx++

	am = &message{
		timenow:    m.timenow + tag.Time - m.aggfirsttm,
		msgsid:     tag.StreamId,
		msgtypeid:  tag.Type,
		msgdatalen: uint32(len(tag.Data)),
		malloc:     m.malloc,
	}
	got = true

	if err = am.Start(); err != nil {
		return
	}
	copy(am.msgdata, tag.Data)

	return
}

type command struct {
	name        string
	transid     float64
	obj         flvio.AMFMap
	arr, params []interface{}
}

func (self *message) Start() (err error) {
	self.msgdataleft = self.msgdatalen
	var b []byte

	malloc := self.malloc
	if malloc == nil {
		malloc = func(n int) ([]byte, error) { return make([]byte, n), nil }
	}

	if self.msgdatalen > 4*1024*1024 {
		err = errwrap.Errorf("MsgDataTooBig(%d)", self.msgdatalen)
		return
	}

	if b, err = malloc(int(self.msgdatalen)); err != nil {
		return
	}

	self.msgdata = b
	return
}

func (cs *message) dbgchunk(s string) []interface{} {
	return []interface{}{
		s,
		"msgsid", cs.msgsid, "msgtypeid", cs.msgtypeid,
		"msghdrtype", cs.msghdrtype,
		"got", fmt.Sprintf("%d/%d", cs.msgdatalen-cs.msgdataleft, cs.msgdatalen),
	}
}

func (cs *message) dbgmsg(s string) []interface{} {
	return []interface{}{
		s,
		"msgsid", cs.msgsid, "msgtypeid", cs.msgtypeid,
		"msghdrtype", cs.msghdrtype, "msgdatalen", len(cs.msgdata),
	}
}

const (
	msgtypeidUserControl      = 4
	msgtypeidAck              = 3
	msgtypeidWindowAckSize    = 5
	msgtypeidSetPeerBandwidth = 6
	msgtypeidSetChunkSize     = 1
	msgtypeidCommandMsgAMF0   = 20
	msgtypeidCommandMsgAMF3   = 17
	msgtypeidDataMsgAMF0      = flvio.TAG_AMF0
	msgtypeidDataMsgAMF3      = flvio.TAG_AMF3
	msgtypeidVideoMsg         = flvio.TAG_VIDEO
	msgtypeidAudioMsg         = flvio.TAG_AUDIO
	msgtypeidAggregate        = 22
)

func msgtypeidString(s uint8) string {
	switch s {
	case msgtypeidUserControl:
		return "UserControl"
	case msgtypeidAck:
		return "Ack"
	case msgtypeidWindowAckSize:
		return "WindowAckSize"
	case msgtypeidSetPeerBandwidth:
		return "SetPeerBandwidth"
	case msgtypeidSetChunkSize:
		return "SetChunkSize"
	case msgtypeidCommandMsgAMF0:
		return "CommandMsgAMF0"
	case msgtypeidCommandMsgAMF3:
		return "CommandMsgAMF3"
	case msgtypeidVideoMsg:
		return "VideoMsg"
	case msgtypeidAudioMsg:
		return "AudioMsg"
	}
	return fmt.Sprint(s)
}

const (
	eventtypeStreamBegin      = 0
	eventtypeSetBufferLength  = 3
	eventtypeStreamIsRecorded = 4
	eventtypePingRequest      = 6
	eventtypePingResponse     = 7
)

const chunkHeader0Length = 16

func fillChunkHeader0MsgDataLen(b []byte, msgdatalen int) {
	pio.PutU24BE(b[4:], uint32(msgdatalen))
}

const ffffff = 0xffffff

func fillChunkHeader0(b []byte, csid uint32, timestamp uint32, msgtypeid uint8, msgsid uint32, msgdatalen int) (n int) {
	//  0                   1                   2                   3
	//  0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
	// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	// |                   timestamp                   |message length |
	// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	// |     message length (cont)     |message type id| msg stream id |
	// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	// |           message stream id (cont)            |
	// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
	//
	//       Figure 9 Chunk Message Header – Type 0

	pio.WriteU8(b, &n, byte(csid)&0x3f)

	if timestamp >= ffffff {
		pio.WriteU24BE(b, &n, ffffff)
	} else {
		pio.WriteU24BE(b, &n, timestamp)
	}

	pio.WriteU24BE(b, &n, uint32(msgdatalen))
	pio.WriteU8(b, &n, msgtypeid)

	pio.WriteU32LE(b, &n, msgsid)

	if timestamp >= ffffff {
		pio.WriteU32BE(b, &n, timestamp)
	}

	return
}

func (self *Conn) fillChunkHeader3(b []byte, csid uint32, timestamp uint32) (n int) {
	pio.WriteU8(b, &n, (byte(csid)&0x3f)|3<<6)
	if timestamp >= ffffff {
		pio.WriteU32BE(b, &n, timestamp)
	}
	return
}

func debugCmdArgs(msg message) []interface{} {
	r := []interface{}{"Type", msgtypeidString(msg.msgtypeid)}
	switch msg.msgtypeid {
	case msgtypeidVideoMsg, msgtypeidAudioMsg:
	case msgtypeidCommandMsgAMF0, msgtypeidCommandMsgAMF3, msgtypeidDataMsgAMF0, msgtypeidDataMsgAMF3:
		amf3 := msg.msgtypeid == msgtypeidCommandMsgAMF3 || msg.msgtypeid == msgtypeidDataMsgAMF3
		arr, _ := flvio.ParseAMFVals(msg.msgdata, amf3)
		r = append(r, []interface{}{"Cmd", utils.JsonString(arr)}...)
		return r
	default:
		r = append(r, []interface{}{"Data", fmt.Sprintf("%x", msg.msgdata)}...)
		return r
	}
	return nil
}

func (self *Conn) readMsg() (msg *message, err error) {
	for {
		if msg, err = self.readChunk(); err != nil {
			return
		}

		if self.readAckSize != 0 && self.ackn-self.lastackn > self.readAckSize {
			if err = self.writeAck(self.ackn); err != nil {
				return
			}
			if err = self.flushWrite(); err != nil {
				return
			}
			self.lastackn = self.ackn
		}

		if msg != nil {
			return
		}
	}
}

func (self *Conn) debugWriteMsg(msg *message) {
	if DebugMsgHeader {
		f := msg.dbgmsg("WriteMsg")
		self.Logger.Info(f...)
	}
	if DebugMsgData {
		self.Logger.Hexdump(msg.msgdata)
	}
	if DebugCmd {
		args := debugCmdArgs(*msg)
		if args != nil {
			self.Logger.Info(append([]interface{}{"WriteCmd"}, args...)...)
		}
	}
}

func (self *Conn) debugReadMsg(msg *message) {
	if DebugMsgHeader {
		f := msg.dbgmsg("ReadMsg")
		self.Logger.Info(f...)
	}
	if DebugMsgData {
		self.Logger.Hexdump(msg.msgdata)
	}
	if DebugCmd {
		args := debugCmdArgs(*msg)
		if args != nil {
			self.Logger.Info(append([]interface{}{"ReadCmd"}, args...)...)
		}
	}
}

func (self *Conn) readMsgHandleEvent() (msg *message, err error) {
	for {
		for {
			if self.aggmsg != nil {
				var got bool
				if got, msg, err = self.nextAggPart(self.aggmsg); err != nil {
					return
				}
				if got {
					break
				} else {
					self.aggmsg = nil
				}
			}
			if msg, err = self.readMsg(); err != nil {
				return
			}
			if msg.msgtypeid == msgtypeidAggregate {
				self.aggmsg = msg
			} else {
				break
			}
		}

		self.debugReadMsg(msg)

		var handled bool
		if handled, err = self.handleEvent(msg); err != nil {
			return
		}
		if handled {
			if err = self.flushWrite(); err != nil {
				return
			}
		} else {
			return
		}
	}
}

func (self *Conn) readChunk() (msg *message, err error) {
	b := self.readbuf
	if _, err = self.wrapRW.Read(b[:1]); err != nil {
		return
	}
	header := b[0]

	var msghdrtype uint8
	var csid uint32

	msghdrtype = header >> 6

	csid = uint32(header) & 0x3f
	switch csid {
	default: // Chunk basic header 1
	case 0: // Chunk basic header 2
		if _, err = self.wrapRW.Read(b[:1]); err != nil {
			return
		}
		csid = uint32(b[0]) + 64
	case 1: // Chunk basic header 3
		if _, err = self.wrapRW.Read(b[:2]); err != nil {
			return
		}
		csid = uint32(pio.U16BE(b)) + 64
	}

	newcs := false
	cs := self.readcsmap[csid]
	if cs == nil {
		cs = &message{}
		self.readcsmap[csid] = cs
		newcs = true
	}
	if len(self.readcsmap) > 16 {
		err = errwrap.Errorf("TooManyCsid")
		return
	}

	var timestamp uint32

	switch msghdrtype {
	case 0:
		//  0                   1                   2                   3
		//  0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
		// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
		// |                   timestamp                   |message length |
		// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
		// |     message length (cont)     |message type id| msg stream id |
		// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
		// |           message stream id (cont)            |
		// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
		//
		//       Figure 9 Chunk Message Header – Type 0
		if cs.msgdataleft != 0 {
			err = errwrap.Errorf("MsgDataLeft(%d)", cs.msgdataleft)
			return
		}
		h := b[:11]
		if _, err = self.wrapRW.Read(h); err != nil {
			return
		}
		timestamp = pio.U24BE(h[0:3])
		cs.msghdrtype = msghdrtype
		cs.msgdatalen = pio.U24BE(h[3:6])
		cs.msgtypeid = h[6]
		cs.msgsid = pio.U32LE(h[7:11])
		if timestamp == ffffff {
			if _, err = self.wrapRW.Read(b[:4]); err != nil {
				return
			}
			timestamp = pio.U32BE(b)
			cs.hastimeext = true
			cs.timeext = timestamp
		} else {
			cs.hastimeext = false
		}
		cs.timenow = timestamp
		if err = cs.Start(); err != nil {
			return
		}

	case 1:
		//  0                   1                   2                   3
		//  0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1
		// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
		// |                timestamp delta                |message length |
		// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
		// |     message length (cont)     |message type id|
		// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
		//
		//       Figure 10 Chunk Message Header – Type 1
		if newcs {
			err = errwrap.Errorf("Type1NoPrevChunk")
			return
		}
		if cs.msgdataleft != 0 {
			err = errwrap.Errorf("MsgDataLeft(%d)", cs.msgdataleft)
			return
		}
		h := b[:7]
		if _, err = self.wrapRW.Read(h); err != nil {
			return
		}
		timestamp = pio.U24BE(h[0:3])
		cs.msghdrtype = msghdrtype
		cs.msgdatalen = pio.U24BE(h[3:6])
		cs.msgtypeid = h[6]
		if timestamp == ffffff {
			if _, err = self.wrapRW.Read(b[:4]); err != nil {
				return
			}
			timestamp = pio.U32BE(b)
			cs.hastimeext = true
			cs.timeext = timestamp
		} else {
			cs.hastimeext = false
		}
		cs.timedelta = timestamp
		cs.timenow += timestamp
		if err = cs.Start(); err != nil {
			return
		}

	case 2:
		//  0                   1                   2
		//  0 1 2 3 4 5 6 7 8 9 0 1 2 3 4 5 6 7 8 9 0 1 2 3
		// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
		// |                timestamp delta                |
		// +-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+
		//
		//       Figure 11 Chunk Message Header – Type 2
		if newcs {
			err = errwrap.Errorf("Type2NoPrevChunk")
			return
		}
		if cs.msgdataleft != 0 {
			err = errwrap.Errorf("MsgDataLeft(%d)", cs.msgdataleft)
			return
		}
		h := b[:3]
		if _, err = self.wrapRW.Read(h); err != nil {
			return
		}
		cs.msghdrtype = msghdrtype
		timestamp = pio.U24BE(h[0:3])
		if timestamp == ffffff {
			if _, err = self.wrapRW.Read(b[:4]); err != nil {
				return
			}
			timestamp = pio.U32BE(b)
			cs.hastimeext = true
			cs.timeext = timestamp
		} else {
			cs.hastimeext = false
		}
		cs.timedelta = timestamp
		cs.timenow += timestamp
		if err = cs.Start(); err != nil {
			return
		}

	case 3:
		if newcs {
			err = errwrap.Errorf("Type3NoPrevChunk")
			return
		}
		if cs.msgdataleft == 0 {
			switch cs.msghdrtype {
			case 0:
				if cs.hastimeext {
					if _, err = self.wrapRW.Read(b[:4]); err != nil {
						return
					}
					timestamp = pio.U32BE(b)
					cs.timenow = timestamp
					cs.timeext = timestamp
				}
			case 1, 2:
				if cs.hastimeext {
					if _, err = self.wrapRW.Read(b[:4]); err != nil {
						return
					}
					timestamp = pio.U32BE(b)
				} else {
					timestamp = cs.timedelta
				}
				cs.timenow += timestamp
			}
			if err = cs.Start(); err != nil {
				return
			}
		} else {
			if cs.hastimeext {
				var b []byte
				if b, err = self.wrapRW.Peek(4); err != nil {
					return
				}
				if pio.U32BE(b) == cs.timeext {
					if _, err = self.wrapRW.Read(b[:4]); err != nil {
						return
					}
				}
			}
		}

	default:
		err = errwrap.Errorf("MsgHdrTypeInvalid(%d)", msghdrtype)
		return
	}

	size := int(cs.msgdataleft)
	if size > self.ReadMaxChunkSize {
		size = self.ReadMaxChunkSize
	}
	off := cs.msgdatalen - cs.msgdataleft
	buf := cs.msgdata[off : int(off)+size]
	if _, err = self.wrapRW.Read(buf); err != nil {
		return
	}
	cs.msgdataleft -= uint32(size)

	if DebugChunkHeader {
		f := cs.dbgchunk("ReadChunk")
		self.Logger.Info(f...)
	}

	if cs.msgdataleft != 0 {
		return
	}

	newmsg := *cs
	msg = &newmsg
	return
}

func (self *Conn) startPeekReadLoop() {
	if self.writing() {
		if DebugStage {
			self.Logger.Info("PeekReadLoopStarted")
		}

		go func() {
			io.Copy(ioutil.Discard, self.RW)
			self.closeNotify <- true
		}()
	}
}

func (self *Conn) readCommand() (cmd *command, err error) {
	for {
		var msg *message
		if msg, err = self.readMsgHandleEvent(); err != nil {
			return
		}
		if cmd, err = msg.parseCommand(); err != nil {
			return
		}
		if cmd != nil {
			self.lastcmd = cmd
			return
		}
	}
}

func (self *Conn) ReadTag() (tag flvio.Tag, err error) {
	if ReadWriteTagSetDeadline {
		if err = self.SetDeadline(time.Now().Add(Timeout)); err != nil {
			return
		}
	}

	if self.isFastRtmp {
		tag, err = self.fastrtmpReadTagHandleEvent()
	} else {
		tag, err = self.rtmpReadTag()
	}
	if err != nil {
		return
	}

	flv.DebugPrintTag("RtmpReadTag", tag)
	return
}

func (self *Conn) rtmpReadTag() (tag flvio.Tag, err error) {
	for {
		var msg *message
		if msg, err = self.readMsgHandleEvent(); err != nil {
			return
		}
		var _tag *flvio.Tag
		if _tag, err = msg.parseTag(self.BypassMsgtypeid); err != nil {
			return
		}
		if _tag != nil {
			tag = *_tag
			return
		}
	}
}

func (self *Conn) tmpwbuf(n int) []byte {
	if cap(self.writebuf) < n {
		self.writebuf = make([]byte, 0, n)
	}
	return self.writebuf[:n]
}

func (self *Conn) tmpwbuf2(n int) []byte {
	if cap(self.writebuf2) < n {
		self.writebuf2 = make([]byte, 0, n)
	}
	return self.writebuf2[:n]
}

func (self *Conn) tmprbuf2(n int) []byte {
	if cap(self.readbuf2) < n {
		self.readbuf2 = make([]byte, 0, n)
	}
	return self.readbuf2[:n]
}

func (self *Conn) TmpwbufData(n int) []byte {
	return self.tmpwbuf2(n)
}

func (self *Conn) setAndWriteChunkSize(size int) (err error) {
	self.writeMaxChunkSize = size
	return self.WriteSetChunkSize(size, self.wrapRW.write)
}

func (self *Conn) WriteSetChunkSize(size int, write func([]byte) error) (err error) {
	b := self.tmpwbuf2(4)
	pio.PutU32BE(b[0:4], uint32(size))
	return self.writeEvent2(msgtypeidSetChunkSize, b, write)
}

func (self *Conn) writeAck(seqnum uint32) (err error) {
	b := self.tmpwbuf2(4)
	pio.PutU32BE(b[0:4], seqnum)
	return self.WriteEvent(msgtypeidAck, b)
}

func (self *Conn) writeWindowAckSize(size uint32) (err error) {
	b := self.tmpwbuf2(4)
	pio.PutU32BE(b[0:4], size)
	return self.WriteEvent(msgtypeidWindowAckSize, b)
}

func (self *Conn) writeSetPeerBandwidth(acksize uint32, limittype uint8) (err error) {
	b := self.tmpwbuf2(5)
	pio.PutU32BE(b[0:4], acksize)
	b[4] = limittype
	return self.WriteEvent(msgtypeidSetPeerBandwidth, b)
}

func (self *Conn) writePingResponse(timestamp uint32) (err error) {
	b := self.tmpwbuf2(6)
	pio.PutU16BE(b[0:2], eventtypePingResponse)
	pio.PutU32BE(b[2:6], timestamp)
	return self.WriteEvent(msgtypeidUserControl, b)
}

func (self *Conn) writeStreamIsRecorded(msgsid uint32) (err error) {
	b := self.tmpwbuf2(6)
	pio.PutU16BE(b[0:2], eventtypeStreamIsRecorded)
	pio.PutU32BE(b[2:6], msgsid)
	return self.WriteEvent(msgtypeidUserControl, b)
}

func (self *Conn) writeStreamBegin(msgsid uint32) (err error) {
	b := self.tmpwbuf2(6)
	pio.PutU16BE(b[0:2], eventtypeStreamBegin)
	pio.PutU32BE(b[2:6], msgsid)
	return self.WriteEvent(msgtypeidUserControl, b)
}

func (self *Conn) writeSetBufferLength(msgsid uint32, timestamp uint32) (err error) {
	b := self.tmpwbuf2(10)
	pio.PutU16BE(b[0:2], eventtypeSetBufferLength)
	pio.PutU32BE(b[2:6], msgsid)
	pio.PutU32BE(b[6:10], timestamp)
	return self.WriteEvent(msgtypeidUserControl, b)
}

func (self *Conn) writeCommand(csid, msgsid uint32, args ...interface{}) (err error) {
	return self.writeMsg(csid, message{
		msgtypeid: msgtypeidCommandMsgAMF0,
		msgsid:    msgsid,
		msgdata:   self.fillAMF0Vals(args),
	}, nil)
}

func (self *Conn) fillAMF0Vals(args []interface{}) []byte {
	b := self.tmpwbuf2(flvio.FillAMF0Vals(nil, args))
	flvio.FillAMF0Vals(b, args)
	return b
}

func (self *Conn) writeMsg2(
	csid uint32, msg message,
	fillheader func([]byte) int,
	write func([]byte) error,
	progress func(message),
) (err error) {
	if fillheader == nil {
		fillheader = func(b []byte) int { return 0 }
	}

	b := self.tmpwbuf(chunkHeader0Length + fillheader(nil))
	chdrlen := fillChunkHeader0(b, csid, msg.timenow, msg.msgtypeid, msg.msgsid, 0)
	taghdrlen := fillheader(b[chdrlen:])
	msg.msgdatalen = uint32(taghdrlen + len(msg.msgdata))
	msg.msgdataleft = msg.msgdatalen
	fillChunkHeader0MsgDataLen(b, int(msg.msgdatalen))
	wb := b[:chdrlen+taghdrlen]
	if err = write(wb); err != nil {
		return
	}

	chunkleft := self.writeMaxChunkSize - taghdrlen
	if chunkleft < 0 {
		panic(fmt.Sprintf("TagHdrTooLong(%d,%d)", self.writeMaxChunkSize, taghdrlen))
	}
	msg.msgdataleft -= uint32(taghdrlen)

	i := 0

	for msg.msgdataleft > 0 {
		if i > 0 {
			n := self.fillChunkHeader3(b, csid, msg.timenow)
			if err = write(b[:n]); err != nil {
				return
			}
		}

		n := int(msg.msgdataleft)
		if n > chunkleft {
			n = chunkleft
		}

		start := int(msg.msgdatalen-msg.msgdataleft) - taghdrlen
		if err = write(msg.msgdata[start : start+n]); err != nil {
			return
		}
		chunkleft -= n
		msg.msgdataleft -= uint32(n)

		if chunkleft == 0 {
			chunkleft = self.writeMaxChunkSize
		}

		i++

		if progress != nil {
			progress(msg)
		}
	}

	return nil
}

func (self *Conn) writeMsg(csid uint32, msg message, fillheader func([]byte) int) (err error) {
	self.debugWriteMsg(&msg)

	progress := func(msg message) {
		if DebugChunkHeader {
			f := msg.dbgchunk("WriteChunk")
			self.Logger.Info(f...)
		}
	}

	if err = self.writeMsg2(csid, msg, fillheader, self.wrapRW.write, progress); err != nil {
		return
	}

	return
}

func (self *Conn) WriteTag(tag flvio.Tag) (err error) {
	if ReadWriteTagSetDeadline {
		if err = self.SetDeadline(time.Now().Add(Timeout)); err != nil {
			return
		}
	}

	flv.DebugPrintTag("RtmpWriteTag", tag)

	if self.isFastRtmp {
		return self.fastrtmpWriteTag(tag)
	}

	var csid uint32
	if tag.Type == flvio.TAG_AUDIO || tag.Type == flvio.TAG_AMF0 || tag.Type == flvio.TAG_AMF3 {
		csid = 4
	} else if tag.Type == flvio.TAG_VIDEO {
		csid = 6
	} else {
		csid = 5
	}
	return self.writeMsg(csid, message{
		msgtypeid: uint8(tag.Type),
		msgdata:   tag.Data,
		msgsid:    self.avmsgsid,
		timenow:   tag.Time,
	}, tag.FillHeader)
}

func (self *message) arrToCommand(arr []interface{}) (cmd *command, err error) {
	if len(arr) < 2 {
		err = errwrap.Errorf("CmdLenInvalid")
		return
	}

	cmd = &command{arr: arr}
	var ok bool

	if cmd.name, ok = arr[0].(string); !ok {
		err = errwrap.Errorf("CmdNameInvalid")
		return
	}
	if cmd.transid, ok = arr[1].(float64); !ok {
		err = errwrap.Errorf("CmdTransIdInvalid")
		return
	}

	if len(arr) < 3 {
		return
	}
	cmd.obj, _ = arr[2].(flvio.AMFMap)
	cmd.params = arr[3:]

	return
}

func (self *message) parseCommand() (cmd *command, err error) {
	switch self.msgtypeid {
	case msgtypeidCommandMsgAMF0, msgtypeidCommandMsgAMF3:
		amf3 := self.msgtypeid == msgtypeidCommandMsgAMF3
		var arr []interface{}
		if arr, err = flvio.ParseAMFVals(self.msgdata, amf3); err != nil {
			return
		}
		if cmd, err = self.arrToCommand(arr); err != nil {
			return
		}
	}
	return
}

func (self *message) parseTag(bypass []uint8) (tag *flvio.Tag, err error) {
	for _, id := range bypass {
		if id == self.msgtypeid {
			tag = &flvio.Tag{
				Type: self.msgtypeid,
				Time: self.timenow,
				Data: self.msgdata,
			}
			return
		}
	}

	switch self.msgtypeid {
	case msgtypeidVideoMsg, msgtypeidAudioMsg:
		_tag := flvio.Tag{Type: self.msgtypeid, Time: self.timenow}
		if err = _tag.Parse(self.msgdata); err != nil {
			err = nil
			return
		}
		tag = &_tag
		return

	case msgtypeidDataMsgAMF0, msgtypeidDataMsgAMF3:
		tag = &flvio.Tag{
			Type: self.msgtypeid,
			Time: self.timenow,
			Data: self.msgdata,
		}
		return

	case flvio.TAG_PRIV:
		_tag := flvio.Tag{Type: self.msgtypeid, Time: self.timenow}
		if err = _tag.Parse(self.msgdata); err != nil {
			err = nil
			return
		}
		tag = &_tag
		return
	}

	return
}

func (self *Conn) writeEvent2(msgtypeid uint8, b []byte, write func([]byte) error) (err error) {
	return self.writeMsg2(2, message{
		msgtypeid: msgtypeid,
		msgdata:   b,
	}, nil, write, nil)
}

func (self *Conn) WriteEvent(msgtypeid uint8, b []byte) (err error) {
	if self.isFastRtmp {
		return self.fastrtmpWriteEvent(msgtypeid, b)
	}
	return self.writeEvent2(msgtypeid, b, self.wrapRW.write)
}

func (self *Conn) handleEvent(msg *message) (handled bool, err error) {
	switch msg.msgtypeid {
	case msgtypeidSetChunkSize:
		var n int
		var v uint32
		if v, err = pio.ReadU32BE(msg.msgdata, &n); err != nil {
			return
		}
		if int(v) < 0 {
			err = errwrap.Errorf("SetChunkSizeInvalid(%x)", v)
			return
		}
		handled = true
		self.ReadMaxChunkSize = int(v)
		return

	case msgtypeidWindowAckSize:
		var n int
		var acksize uint32
		if acksize, err = pio.ReadU32BE(msg.msgdata, &n); err != nil {
			return
		}
		handled = true
		self.readAckSize = acksize
		return

	case msgtypeidUserControl:
		var n int
		var eventtype uint16
		if eventtype, err = pio.ReadU16BE(msg.msgdata, &n); err != nil {
			return
		}

		switch eventtype {
		case eventtypePingRequest:
			var timestamp uint32
			if timestamp, err = pio.ReadU32BE(msg.msgdata, &n); err != nil {
				return
			}
			handled = true
			err = self.writePingResponse(timestamp)
			return
		}

	default:
		if self.HandleEvent != nil {
			return self.HandleEvent(msg.msgtypeid, msg.msgdata)
		}
	}

	return
}
