package rtmp

import (
	"fmt"

	"github.com/nareix/joy5/av"
	"github.com/nareix/joy5/format/flv"
	"github.com/nareix/joy5/format/flv/flvio"
)

const (
	StageInit = iota
	StageHandshakeDone
	StageGotPublishOrPlayCommand
	StageCommandDone
	StageDataStart
)

const (
	PrepareReading = iota + 1
	PrepareWriting
)

type Stage int

var StageString = map[Stage]string{
	StageHandshakeDone:           "StageHandshakeDone",
	StageGotPublishOrPlayCommand: "StageGotPublishOrPlayCommand",
	StageCommandDone:             "StageCommandDone",
	StageDataStart:               "StageDataStart",
}

func (s Stage) String() string {
	return StageString[s]
}

func (c *Conn) writeBasicConf() (err error) {
	if err = c.writeWindowAckSize(2500000); err != nil {
		return
	}
	if err = c.writeSetPeerBandwidth(2500000, 2); err != nil {
		return
	}
	if err = c.setAndWriteChunkSize(65536); err != nil {
		return
	}
	return
}

func (c *Conn) writeDataStart() (err error) {
	if c.writing() {
		if !c.Publishing {
			if err = c.writeCommand(5, c.avmsgsid,
				"onStatus", c.lastcmd.transid, nil,
				flvio.AMFMap{
					{K: "level", V: "status"},
					{K: "code", V: "NetStream.Play.PublishNotify"},
					{K: "description", V: "publish notify"},
				},
			); err != nil {
				return
			}

		}
	}

	if err = c.flushWrite(); err != nil {
		return
	}

	c.Stage = StageDataStart
	return
}

func (c *Conn) writePublishOrPlayResult(ok bool, msg string) (err error) {
	if !c.Publishing {
		if !ok {
			if err = c.writeCommand(5, c.avmsgsid,
				"onStatus", c.lastcmd.transid, nil,
				flvio.AMFMap{
					{K: "level", V: "status"},
					{K: "code", V: "NetStream.Play.Failed"},
					{K: "description", V: msg},
				},
			); err != nil {
				return
			}
		} else {
			if err = c.writeStreamIsRecorded(c.avmsgsid); err != nil {
				return
			}
			if err = c.writeStreamBegin(c.avmsgsid); err != nil {
				return
			}

			if err = c.writeCommand(5, c.avmsgsid,
				"onStatus", c.lastcmd.transid, nil,
				flvio.AMFMap{
					{K: "level", V: "status"},
					{K: "code", V: "NetStream.Play.Reset"},
					{K: "description", V: "play reset"},
				}); err != nil {
				return
			}

			if err = c.writeCommand(5, c.avmsgsid,
				"onStatus", c.lastcmd.transid, nil,
				flvio.AMFMap{
					{K: "level", V: "status"},
					{K: "code", V: "NetStream.Play.Start"},
					{K: "description", V: "play start"},
				},
			); err != nil {
				return
			}

			if c.SendSampleAccess {
				if err = c.writeMsg(4, message{
					msgtypeid: msgtypeidDataMsgAMF0,
					msgsid:    c.avmsgsid,
					msgdata:   c.fillAMF0Vals([]interface{}{"|RtmpSampleAccess", true, true}),
				}, nil); err != nil {
					return
				}
			}

			if err = c.writeCommand(5, c.avmsgsid,
				"onStatus", c.lastcmd.transid, nil,
				flvio.AMFMap{
					{K: "level", V: "status"},
					{K: "code", V: "NetStream.Data.Start"},
					{K: "description", V: "data start"},
				},
			); err != nil {
				return
			}
		}
	} else {
		if !ok {
			if err = c.writeCommand(5, c.avmsgsid,
				"onStatus", c.lastcmd.transid, nil,
				flvio.AMFMap{
					{K: "level", V: "status"},
					{K: "code", V: "NetStream.Publish.Failed"},
					{K: "description", V: msg},
				},
			); err != nil {
				return
			}
		} else {
			if err = c.writeCommand(5, c.avmsgsid,
				"onStatus", c.lastcmd.transid, nil,
				flvio.AMFMap{
					{K: "level", V: "status"},
					{K: "code", V: "NetStream.Publish.Start"},
					{K: "description", V: "publish start"},
				},
			); err != nil {
				return
			}
		}
	}

	if err = c.flushWrite(); err != nil {
		return
	}

	c.Stage = StageCommandDone
	return
}

func (c *Conn) readConnect() (err error) {
	var cmd *command

	if cmd, err = c.readCommand(); err != nil {
		return
	}

	if cmd.name != "connect" {
		err = fmt.Errorf("FirstCommandNotConnect")
		return
	}
	if cmd.obj == nil {
		err = fmt.Errorf("ConnectParamsInvalid")
		return
	}

	var ok bool
	var connectpath string

	if connectpath, ok = cmd.obj.GetString("app"); !ok {
		err = fmt.Errorf("ConnectMissingApp")
		return
	}

	var tcurl string
	tcurl, _ = cmd.obj.GetString("tcUrl")
	if tcurl == "" {
		tcurl, _ = cmd.obj.GetString("tcurl")
	}
	c.TcUrl = tcurl

	var pageurl string
	pageurl, _ = cmd.obj.GetString("pageUrl")
	if pageurl == "" {
		pageurl, _ = cmd.obj.GetString("pageurl")
	}
	c.PageUrl = pageurl

	var flashver string
	flashver, _ = cmd.obj.GetString("flashVer")
	if flashver == "" {
		flashver, _ = cmd.obj.GetString("flashver")
	}
	c.FlashVer = flashver

	objectEncoding, _ := cmd.obj.GetFloat64("objectEncoding")

	if err = c.writeBasicConf(); err != nil {
		return
	}

	if err = c.writeCommand(3, 0, "_result", cmd.transid,
		flvio.AMFMap{
			{K: "fmsVer", V: "LNX 9,0,124,2"},
			{K: "capabilities", V: 31},
		},
		flvio.AMFMap{
			{K: "level", V: "status"},
			{K: "code", V: "NetConnection.Connect.Success"},
			{K: "description", V: "Connection succeeded."},
			{K: "objectEncoding", V: objectEncoding},
		},
	); err != nil {
		return
	}

	if err = c.flushWrite(); err != nil {
		return
	}

	for {
		if cmd, err = c.readCommand(); err != nil {
			return
		}

		switch cmd.name {
		case "createStream":
			c.avmsgsid = uint32(1)
			if err = c.writeCommand(3, 0, "_result", cmd.transid, nil, c.avmsgsid); err != nil {
				return
			}
			if err = c.flushWrite(); err != nil {
				return
			}

		case "publish":
			if len(cmd.params) < 1 {
				err = fmt.Errorf("PublishParamsInvalid")
				return
			}
			publishpath, _ := cmd.params[0].(string)

			if c.URL, err = createURL(tcurl, connectpath, publishpath); err != nil {
				return
			}
			c.Publishing = true
			c.Stage = StageGotPublishOrPlayCommand
			return

		case "play":
			if len(cmd.params) < 1 {
				err = fmt.Errorf("PlayParamsInvalid")
				return
			}
			playpath, _ := cmd.params[0].(string)

			if c.URL, err = createURL(tcurl, connectpath, playpath); err != nil {
				return
			}
			c.Publishing = false
			c.Stage = StageGotPublishOrPlayCommand
			return
		}
	}
}

func (c *Conn) checkLevelStatus(cmd *command) (err error) {
	if len(cmd.params) < 1 {
		return fmt.Errorf("NoParams")
	}

	obj, _ := cmd.params[0].(flvio.AMFMap)
	if obj == nil {
		return fmt.Errorf("NoObj")
	}

	level, _ := obj.GetString("level")
	if level != "status" {
		code, _ := obj.GetString("code")
		return fmt.Errorf("CodeInvalid(%s)", code)
	}

	return
}

func (c *Conn) checkCreateStreamResult(cmd *command) (ok bool, avmsgsid uint32) {
	if len(cmd.params) > 0 {
		_avmsgsid, _ := cmd.params[0].(float64)
		avmsgsid = uint32(_avmsgsid)
		ok = true
		return
	}
	return
}

func (c *Conn) writeConnect(path string) (err error) {
	if err = c.writeBasicConf(); err != nil {
		return
	}

	if err = c.writeCommand(3, 0, "connect", 1,
		flvio.AMFMap{
			{K: "app", V: path},
			{K: "flashVer", V: "LNX 9,0,124,2"},
			{K: "tcUrl", V: getTcURL(c.URL)},
			{K: "fpad", V: false},
			{K: "capabilities", V: 15},
			{K: "audioCodecs", V: 4071},
			{K: "videoCodecs", V: 252},
			{K: "videoFunction", V: 1},
		},
	); err != nil {
		return
	}

	if err = c.flushWrite(); err != nil {
		return
	}

	for {
		var cmd *command
		if cmd, err = c.readCommand(); err != nil {
			return
		}

		if cmd.name == "_result" {
			if err = c.checkLevelStatus(cmd); err != nil {
				err = fmt.Errorf("CommandConnectFailed: %s", err)
				return
			}
			break
		}
	}

	return
}

func (c *Conn) connectPublish() (err error) {
	connectpath, publishpath := splitPath(c.URL)

	if err = c.writeConnect(connectpath); err != nil {
		return
	}

	transid := 1

	transid++
	if err = c.writeCommand(3, 0, "releaseStream", transid, nil, publishpath); err != nil {
		return
	}

	transid++
	if err = c.writeCommand(3, 0, "FCPublish", transid, nil, publishpath); err != nil {
		return
	}

	transid++
	if err = c.writeCommand(3, 0, "createStream", transid, nil); err != nil {
		return
	}

	if err = c.flushWrite(); err != nil {
		return
	}

	for {
		var cmd *command
		if cmd, err = c.readCommand(); err != nil {
			return
		}
		if cmd.name == "_result" && int(cmd.transid) == transid {
			var ok bool
			if ok, c.avmsgsid = c.checkCreateStreamResult(cmd); !ok {
				err = fmt.Errorf("CreateStreamFailed")
				return
			}
			break
		}
	}

	transid++
	if err = c.writeCommand(4, c.avmsgsid, "publish", transid, nil, publishpath, connectpath); err != nil {
		return
	}

	if err = c.flushWrite(); err != nil {
		return
	}

	var cmd *command
	for {
		if cmd, err = c.readCommand(); err != nil {
			return
		}
		if cmd.name == "onStatus" {
			if err = c.checkLevelStatus(cmd); err != nil {
				err = fmt.Errorf("PublishFailed: %s", err)
				return
			}
			break
		}
	}
	if len(cmd.params) > 0 {
		c.PubPlayOnStatusParams, _ = cmd.params[0].(flvio.AMFMap)
	}

	c.Publishing = true
	c.Stage = StageCommandDone
	return
}

func (c *Conn) connectPlay() (err error) {
	connectpath, playpath := splitPath(c.URL)

	if err = c.writeConnect(connectpath); err != nil {
		return
	}
	if err = c.writeCommand(3, 0, "createStream", 2, nil); err != nil {
		return
	}
	if err = c.writeSetBufferLength(0, 100); err != nil {
		return
	}
	if err = c.flushWrite(); err != nil {
		return
	}

	for {
		var cmd *command
		if cmd, err = c.readCommand(); err != nil {
			return
		}

		if cmd.name == "_result" {
			var ok bool
			if ok, c.avmsgsid = c.checkCreateStreamResult(cmd); !ok {
				err = fmt.Errorf("CreateStreamFailed")
				return
			}
			break
		}
	}

	if err = c.writeCommand(4, c.avmsgsid, "play", 0, nil, playpath); err != nil {
		return
	}
	if err = c.flushWrite(); err != nil {
		return
	}

	var cmd *command
	for {
		if cmd, err = c.readCommand(); err != nil {
			return
		}
		if cmd.name == "onStatus" {
			if err = c.checkLevelStatus(cmd); err != nil {
				err = fmt.Errorf("PlayFailed: %s", err)
				return
			}
			break
		}
	}
	if len(cmd.params) > 0 {
		c.PubPlayOnStatusParams, _ = cmd.params[0].(flvio.AMFMap)
	}

	c.Publishing = false
	c.Stage = StageCommandDone
	return
}

func (c *Conn) ReadPacket() (pkt av.Packet, err error) {
	if err = c.Prepare(StageCommandDone, PrepareReading); err != nil {
		return
	}
	return flv.ReadPacket(c.ReadTag)
}

func (c *Conn) WritePacket(pkt av.Packet) (err error) {
	if err = c.Prepare(StageDataStart, PrepareWriting); err != nil {
		return
	}
	return flv.WritePacket(pkt, c.WriteTag)
}

func (c *Conn) debugStage(flags int, goturl bool) {
	if goturl {
		var event string
		if c.isserver {
			if c.Publishing {
				event = "RtmpServerPublish"
			} else {
				event = "RtmpServerPlay"
			}
		} else {
			if flags == PrepareReading {
				event = "RtmpDialPlay"
			} else {
				event = "RtmpDialPublish"
			}
		}
		if c.LogStageEvent != nil {
			c.LogStageEvent(event, c.URL.String())
		}
	} else {
		event := "Rtmp" + c.Stage.String()
		if c.LogStageEvent != nil {
			c.LogStageEvent(event, "")
		}
	}
}

func (c *Conn) Prepare(stage Stage, flags int) (err error) {
	for c.Stage < stage {
		switch c.Stage {
		case StageInit:
			if c.isserver {
				if err = c.handshakeServer(); err != nil {
					return
				}
			} else {
				if err = c.handshakeClient(); err != nil {
					return
				}
			}
			c.Stage = StageHandshakeDone
			return

		case StageHandshakeDone:
			if c.isserver {
				if err = c.readConnect(); err != nil {
					return
				}
			} else {
				if flags == PrepareReading {
					if err = c.connectPlay(); err != nil {
						return
					}
				} else {
					if err = c.connectPublish(); err != nil {
						return
					}
				}
			}
			c.debugStage(flags, true)

		case StageGotPublishOrPlayCommand:
			if c.PubPlayErr == nil {
				if err = c.writePublishOrPlayResult(true, ""); err != nil {
					return
				}
			} else {
				if err = c.writePublishOrPlayResult(false, c.PubPlayErr.Error()); err != nil {
					return
				}
			}
			c.startPeekReadLoop()
			c.debugStage(flags, false)

		case StageCommandDone:
			if err = c.writeDataStart(); err != nil {
				return
			}
			c.debugStage(flags, false)
		}
	}

	return
}
