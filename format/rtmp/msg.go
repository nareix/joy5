package rtmp

import (
	"fmt"
	"io"
	"io/ioutil"

	"github.com/nareix/joy5/format/flv/flvio"
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

func (m *message) Start() (err error) {
	m.msgdataleft = m.msgdatalen
	var b []byte

	malloc := m.malloc
	if malloc == nil {
		malloc = func(n int) ([]byte, error) { return make([]byte, n), nil }
	}

	if m.msgdatalen > 4*1024*1024 {
		err = fmt.Errorf("MsgDataTooBig(%d)", m.msgdatalen)
		return
	}

	if b, err = malloc(int(m.msgdatalen)); err != nil {
		return
	}

	m.msgdata = b
	return
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

func (c *Conn) fillChunkHeader3(b []byte, csid uint32, timestamp uint32) (n int) {
	pio.WriteU8(b, &n, (byte(csid)&0x3f)|3<<6)
	if timestamp >= ffffff {
		pio.WriteU32BE(b, &n, timestamp)
	}
	return
}

// func debugCmdArgs(msg message) []interface{} {
// 	r := []interface{}{"Type", msgtypeidString(msg.msgtypeid)}
// 	switch msg.msgtypeid {
// 	case msgtypeidVideoMsg, msgtypeidAudioMsg:
// 	case msgtypeidCommandMsgAMF0, msgtypeidCommandMsgAMF3, msgtypeidDataMsgAMF0, msgtypeidDataMsgAMF3:
// 		amf3 := msg.msgtypeid == msgtypeidCommandMsgAMF3 || msg.msgtypeid == msgtypeidDataMsgAMF3
// 		arr, _ := flvio.ParseAMFVals(msg.msgdata, amf3)
// 		r = append(r, []interface{}{"Cmd", utils.JsonString(arr)}...)
// 		return r
// 	default:
// 		r = append(r, []interface{}{"Data", fmt.Sprintf("%x", msg.msgdata)}...)
// 		return r
// 	}
// 	return nil
// }

func (c *Conn) readMsg() (msg *message, err error) {
	for {
		if msg, err = c.readChunk(); err != nil {
			return
		}

		if c.readAckSize != 0 && c.ackn-c.lastackn > c.readAckSize {
			if err = c.writeAck(c.ackn); err != nil {
				return
			}
			if err = c.flushWrite(); err != nil {
				return
			}
			c.lastackn = c.ackn
		}

		if msg != nil {
			return
		}
	}
}

func (c *Conn) debugWriteMsg(msg *message) {
	if fn := c.LogMsgEvent; fn != nil {
		fn(false, *msg)
	}
}

func (c *Conn) debugReadMsg(msg *message) {
	if fn := c.LogMsgEvent; fn != nil {
		fn(true, *msg)
	}
}

func (c *Conn) readMsgHandleEvent() (msg *message, err error) {
	for {
		for {
			if c.aggmsg != nil {
				var got bool
				if got, msg, err = c.nextAggPart(c.aggmsg); err != nil {
					return
				}
				if got {
					break
				} else {
					c.aggmsg = nil
				}
			}
			if msg, err = c.readMsg(); err != nil {
				return
			}
			if msg.msgtypeid == msgtypeidAggregate {
				c.aggmsg = msg
			} else {
				break
			}
		}

		c.debugReadMsg(msg)

		var handled bool
		if handled, err = c.handleEvent(msg); err != nil {
			return
		}
		if handled {
			if err = c.flushWrite(); err != nil {
				return
			}
		} else {
			return
		}
	}
}

func (c *Conn) readChunk() (msg *message, err error) {
	b := c.readbuf
	if _, err = c.wrapRW.Read(b[:1]); err != nil {
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
		if _, err = c.wrapRW.Read(b[:1]); err != nil {
			return
		}
		csid = uint32(b[0]) + 64
	case 1: // Chunk basic header 3
		if _, err = c.wrapRW.Read(b[:2]); err != nil {
			return
		}
		csid = uint32(pio.U16BE(b)) + 64
	}

	newcs := false
	cs := c.readcsmap[csid]
	if cs == nil {
		cs = &message{}
		c.readcsmap[csid] = cs
		newcs = true
	}
	if len(c.readcsmap) > 16 {
		err = fmt.Errorf("TooManyCsid")
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
			err = fmt.Errorf("MsgDataLeft(%d)", cs.msgdataleft)
			return
		}
		h := b[:11]
		if _, err = c.wrapRW.Read(h); err != nil {
			return
		}
		timestamp = pio.U24BE(h[0:3])
		cs.msghdrtype = msghdrtype
		cs.msgdatalen = pio.U24BE(h[3:6])
		cs.msgtypeid = h[6]
		cs.msgsid = pio.U32LE(h[7:11])
		if timestamp == ffffff {
			if _, err = c.wrapRW.Read(b[:4]); err != nil {
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
			err = fmt.Errorf("Type1NoPrevChunk")
			return
		}
		if cs.msgdataleft != 0 {
			err = fmt.Errorf("MsgDataLeft(%d)", cs.msgdataleft)
			return
		}
		h := b[:7]
		if _, err = c.wrapRW.Read(h); err != nil {
			return
		}
		timestamp = pio.U24BE(h[0:3])
		cs.msghdrtype = msghdrtype
		cs.msgdatalen = pio.U24BE(h[3:6])
		cs.msgtypeid = h[6]
		if timestamp == ffffff {
			if _, err = c.wrapRW.Read(b[:4]); err != nil {
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
			err = fmt.Errorf("Type2NoPrevChunk")
			return
		}
		if cs.msgdataleft != 0 {
			err = fmt.Errorf("MsgDataLeft(%d)", cs.msgdataleft)
			return
		}
		h := b[:3]
		if _, err = c.wrapRW.Read(h); err != nil {
			return
		}
		cs.msghdrtype = msghdrtype
		timestamp = pio.U24BE(h[0:3])
		if timestamp == ffffff {
			if _, err = c.wrapRW.Read(b[:4]); err != nil {
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
			err = fmt.Errorf("Type3NoPrevChunk")
			return
		}
		if cs.msgdataleft == 0 {
			switch cs.msghdrtype {
			case 0:
				if cs.hastimeext {
					if _, err = c.wrapRW.Read(b[:4]); err != nil {
						return
					}
					timestamp = pio.U32BE(b)
					cs.timenow = timestamp
					cs.timeext = timestamp
				}
			case 1, 2:
				if cs.hastimeext {
					if _, err = c.wrapRW.Read(b[:4]); err != nil {
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
				if b, err = c.wrapRW.Peek(4); err != nil {
					return
				}
				if pio.U32BE(b) == cs.timeext {
					if _, err = c.wrapRW.Read(b[:4]); err != nil {
						return
					}
				}
			}
		}

	default:
		err = fmt.Errorf("MsgHdrTypeInvalid(%d)", msghdrtype)
		return
	}

	size := int(cs.msgdataleft)
	if size > c.ReadMaxChunkSize {
		size = c.ReadMaxChunkSize
	}
	off := cs.msgdatalen - cs.msgdataleft
	buf := cs.msgdata[off : int(off)+size]
	if _, err = c.wrapRW.Read(buf); err != nil {
		return
	}
	cs.msgdataleft -= uint32(size)

	if fn := c.LogChunkHeaderEvent; fn != nil {
		fn(true, *cs)
	}

	if cs.msgdataleft != 0 {
		return
	}

	newmsg := *cs
	msg = &newmsg
	return
}

func (c *Conn) startPeekReadLoop() {
	if c.writing() {
		go func() {
			io.Copy(ioutil.Discard, c.wrapRW.rw)
			c.closeNotify <- true
		}()
	}
}

func (c *Conn) readCommand() (cmd *command, err error) {
	for {
		var msg *message
		if msg, err = c.readMsgHandleEvent(); err != nil {
			return
		}
		if cmd, err = msg.parseCommand(); err != nil {
			return
		}
		if cmd != nil {
			c.lastcmd = cmd
			return
		}
	}
}

func (c *Conn) ReadTag() (tag flvio.Tag, err error) {
	if tag, err = c.rtmpReadTag(); err != nil {
		return
	}

	if fn := c.LogTagEvent; fn != nil {
		fn(true, tag)
	}
	return
}

func (c *Conn) rtmpReadTag() (tag flvio.Tag, err error) {
	for {
		var msg *message
		if msg, err = c.readMsgHandleEvent(); err != nil {
			return
		}
		var _tag *flvio.Tag
		if _tag, err = msg.parseTag(c.BypassMsgtypeid); err != nil {
			return
		}
		if _tag != nil {
			tag = *_tag
			return
		}
	}
}

func (c *Conn) tmpwbuf(n int) []byte {
	if cap(c.writebuf) < n {
		c.writebuf = make([]byte, 0, n)
	}
	return c.writebuf[:n]
}

func (c *Conn) tmpwbuf2(n int) []byte {
	if cap(c.writebuf2) < n {
		c.writebuf2 = make([]byte, 0, n)
	}
	return c.writebuf2[:n]
}

func (c *Conn) tmprbuf2(n int) []byte {
	if cap(c.readbuf2) < n {
		c.readbuf2 = make([]byte, 0, n)
	}
	return c.readbuf2[:n]
}

func (c *Conn) TmpwbufData(n int) []byte {
	return c.tmpwbuf2(n)
}

func (c *Conn) setAndWriteChunkSize(size int) (err error) {
	c.writeMaxChunkSize = size
	return c.WriteSetChunkSize(size, c.wrapRW.write)
}

func (c *Conn) WriteSetChunkSize(size int, write func([]byte) error) (err error) {
	b := c.tmpwbuf2(4)
	pio.PutU32BE(b[0:4], uint32(size))
	return c.writeEvent2(msgtypeidSetChunkSize, b, write)
}

func (c *Conn) writeAck(seqnum uint32) (err error) {
	b := c.tmpwbuf2(4)
	pio.PutU32BE(b[0:4], seqnum)
	return c.WriteEvent(msgtypeidAck, b)
}

func (c *Conn) writeWindowAckSize(size uint32) (err error) {
	b := c.tmpwbuf2(4)
	pio.PutU32BE(b[0:4], size)
	return c.WriteEvent(msgtypeidWindowAckSize, b)
}

func (c *Conn) writeSetPeerBandwidth(acksize uint32, limittype uint8) (err error) {
	b := c.tmpwbuf2(5)
	pio.PutU32BE(b[0:4], acksize)
	b[4] = limittype
	return c.WriteEvent(msgtypeidSetPeerBandwidth, b)
}

func (c *Conn) writePingResponse(timestamp uint32) (err error) {
	b := c.tmpwbuf2(6)
	pio.PutU16BE(b[0:2], eventtypePingResponse)
	pio.PutU32BE(b[2:6], timestamp)
	return c.WriteEvent(msgtypeidUserControl, b)
}

func (c *Conn) writeStreamIsRecorded(msgsid uint32) (err error) {
	b := c.tmpwbuf2(6)
	pio.PutU16BE(b[0:2], eventtypeStreamIsRecorded)
	pio.PutU32BE(b[2:6], msgsid)
	return c.WriteEvent(msgtypeidUserControl, b)
}

func (c *Conn) writeStreamBegin(msgsid uint32) (err error) {
	b := c.tmpwbuf2(6)
	pio.PutU16BE(b[0:2], eventtypeStreamBegin)
	pio.PutU32BE(b[2:6], msgsid)
	return c.WriteEvent(msgtypeidUserControl, b)
}

func (c *Conn) writeSetBufferLength(msgsid uint32, timestamp uint32) (err error) {
	b := c.tmpwbuf2(10)
	pio.PutU16BE(b[0:2], eventtypeSetBufferLength)
	pio.PutU32BE(b[2:6], msgsid)
	pio.PutU32BE(b[6:10], timestamp)
	return c.WriteEvent(msgtypeidUserControl, b)
}

func (c *Conn) writeCommand(csid, msgsid uint32, args ...interface{}) (err error) {
	return c.writeMsg(csid, message{
		msgtypeid: msgtypeidCommandMsgAMF0,
		msgsid:    msgsid,
		msgdata:   c.fillAMF0Vals(args),
	}, nil)
}

func (c *Conn) fillAMF0Vals(args []interface{}) []byte {
	b := c.tmpwbuf2(flvio.FillAMF0Vals(nil, args))
	flvio.FillAMF0Vals(b, args)
	return b
}

func (c *Conn) writeMsg2(
	csid uint32, msg message,
	fillheader func([]byte) int,
	write func([]byte) error,
	progress func(message),
) (err error) {
	if fillheader == nil {
		fillheader = func(b []byte) int { return 0 }
	}

	b := c.tmpwbuf(chunkHeader0Length + fillheader(nil))
	chdrlen := fillChunkHeader0(b, csid, msg.timenow, msg.msgtypeid, msg.msgsid, 0)
	taghdrlen := fillheader(b[chdrlen:])
	msg.msgdatalen = uint32(taghdrlen + len(msg.msgdata))
	msg.msgdataleft = msg.msgdatalen
	fillChunkHeader0MsgDataLen(b, int(msg.msgdatalen))
	wb := b[:chdrlen+taghdrlen]
	if err = write(wb); err != nil {
		return
	}

	chunkleft := c.writeMaxChunkSize - taghdrlen
	if chunkleft < 0 {
		panic(fmt.Sprintf("TagHdrTooLong(%d,%d)", c.writeMaxChunkSize, taghdrlen))
	}
	msg.msgdataleft -= uint32(taghdrlen)

	i := 0

	for msg.msgdataleft > 0 {
		if i > 0 {
			n := c.fillChunkHeader3(b, csid, msg.timenow)
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
			chunkleft = c.writeMaxChunkSize
		}

		i++

		if progress != nil {
			progress(msg)
		}
	}

	return nil
}

func (c *Conn) writeMsg(csid uint32, msg message, fillheader func([]byte) int) (err error) {
	c.debugWriteMsg(&msg)

	progress := func(msg message) {
		if fn := c.LogChunkHeaderEvent; fn != nil {
			fn(false, msg)
		}
	}

	if err = c.writeMsg2(csid, msg, fillheader, c.wrapRW.write, progress); err != nil {
		return
	}

	return
}

func (c *Conn) WriteTag(tag flvio.Tag) (err error) {
	if c.LogTagEvent != nil {
		c.LogTagEvent(false, tag)
	}

	var csid uint32
	if tag.Type == flvio.TAG_AUDIO || tag.Type == flvio.TAG_AMF0 || tag.Type == flvio.TAG_AMF3 {
		csid = 4
	} else if tag.Type == flvio.TAG_VIDEO {
		csid = 6
	} else {
		csid = 5
	}
	return c.writeMsg(csid, message{
		msgtypeid: uint8(tag.Type),
		msgdata:   tag.Data,
		msgsid:    c.avmsgsid,
		timenow:   tag.Time,
	}, tag.FillHeader)
}

func (c *message) arrToCommand(arr []interface{}) (cmd *command, err error) {
	if len(arr) < 2 {
		err = fmt.Errorf("CmdLenInvalid")
		return
	}

	cmd = &command{arr: arr}
	var ok bool

	if cmd.name, ok = arr[0].(string); !ok {
		err = fmt.Errorf("CmdNameInvalid")
		return
	}
	if cmd.transid, ok = arr[1].(float64); !ok {
		err = fmt.Errorf("CmdTransIdInvalid")
		return
	}

	if len(arr) < 3 {
		return
	}
	cmd.obj, _ = arr[2].(flvio.AMFMap)
	cmd.params = arr[3:]

	return
}

func (c *message) parseCommand() (cmd *command, err error) {
	switch c.msgtypeid {
	case msgtypeidCommandMsgAMF0, msgtypeidCommandMsgAMF3:
		amf3 := c.msgtypeid == msgtypeidCommandMsgAMF3
		var arr []interface{}
		if arr, err = flvio.ParseAMFVals(c.msgdata, amf3); err != nil {
			return
		}
		if cmd, err = c.arrToCommand(arr); err != nil {
			return
		}
	}
	return
}

func (c *message) parseTag(bypass []uint8) (tag *flvio.Tag, err error) {
	for _, id := range bypass {
		if id == c.msgtypeid {
			tag = &flvio.Tag{
				Type: c.msgtypeid,
				Time: c.timenow,
				Data: c.msgdata,
			}
			return
		}
	}

	switch c.msgtypeid {
	case msgtypeidVideoMsg, msgtypeidAudioMsg:
		_tag := flvio.Tag{Type: c.msgtypeid, Time: c.timenow}
		if err = _tag.Parse(c.msgdata); err != nil {
			err = nil
			return
		}
		tag = &_tag
		return

	case msgtypeidDataMsgAMF0, msgtypeidDataMsgAMF3:
		tag = &flvio.Tag{
			Type: c.msgtypeid,
			Time: c.timenow,
			Data: c.msgdata,
		}
		return
	}

	return
}

func (c *Conn) writeEvent2(msgtypeid uint8, b []byte, write func([]byte) error) (err error) {
	return c.writeMsg2(2, message{
		msgtypeid: msgtypeid,
		msgdata:   b,
	}, nil, write, nil)
}

func (c *Conn) WriteEvent(msgtypeid uint8, b []byte) (err error) {
	return c.writeEvent2(msgtypeid, b, c.wrapRW.write)
}

func (c *Conn) handleEvent(msg *message) (handled bool, err error) {
	switch msg.msgtypeid {
	case msgtypeidSetChunkSize:
		var n int
		var v uint32
		if v, err = pio.ReadU32BE(msg.msgdata, &n); err != nil {
			return
		}
		if int(v) < 0 {
			err = fmt.Errorf("SetChunkSizeInvalid(%x)", v)
			return
		}
		handled = true
		c.ReadMaxChunkSize = int(v)
		return

	case msgtypeidWindowAckSize:
		var n int
		var acksize uint32
		if acksize, err = pio.ReadU32BE(msg.msgdata, &n); err != nil {
			return
		}
		handled = true
		c.readAckSize = acksize
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
			err = c.writePingResponse(timestamp)
			return
		}

	default:
		if c.HandleEvent != nil {
			return c.HandleEvent(msg.msgtypeid, msg.msgdata)
		}
	}

	return
}
